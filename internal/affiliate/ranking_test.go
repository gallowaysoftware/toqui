package affiliate

import (
	"testing"
)

func TestSelectForPreference_EmptyReturnsZero(t *testing.T) {
	got := SelectForPreference(false, nil)
	if got != (Source{}) {
		t.Errorf("expected zero Source for nil input, got %+v", got)
	}
	got = SelectForPreference(true, []Source{})
	if got != (Source{}) {
		t.Errorf("expected zero Source for empty slice, got %+v", got)
	}
}

func TestSelectForPreference_FreeTier_TakesFirst(t *testing.T) {
	sources := []Source{
		{ID: "a", IsAffiliate: true},
		{ID: "b", IsAffiliate: false},
	}
	got := SelectForPreference(false, sources)
	if got.ID != "a" {
		t.Errorf("free tier should take sources[0]=a, got %q", got.ID)
	}
}

func TestSelectForPreference_PreferNonAffiliate_PicksFirstNonAffiliate(t *testing.T) {
	sources := []Source{
		{ID: "affiliate", IsAffiliate: true},
		{ID: "indep1", IsAffiliate: false},
		{ID: "indep2", IsAffiliate: false},
	}
	got := SelectForPreference(true, sources)
	if got.ID != "indep1" {
		t.Errorf("preferNonAffiliate should pick first IsAffiliate=false source (indep1), got %q", got.ID)
	}
}

func TestSelectForPreference_PreferNonAffiliate_FallsBackToAffiliate(t *testing.T) {
	// Insurance-style category: only an affiliate option exists.
	sources := []Source{
		{ID: "safetywing", IsAffiliate: true},
	}
	got := SelectForPreference(true, sources)
	if got.ID != "safetywing" {
		t.Errorf("preferNonAffiliate should fall back to affiliate when no indep source exists, got %q", got.ID)
	}
}

func TestSelectForPreference_PreferNonAffiliate_AllAffiliate_ReturnsFirst(t *testing.T) {
	sources := []Source{
		{ID: "p1", IsAffiliate: true},
		{ID: "p2", IsAffiliate: true},
	}
	got := SelectForPreference(true, sources)
	if got.ID != "p1" {
		t.Errorf("with all-affiliate sources, preferNonAffiliate should fall back to sources[0]=p1, got %q", got.ID)
	}
}

func TestSelectForPreference_PreferNonAffiliate_FirstSourceIsIndependent(t *testing.T) {
	// If sources[0] happens to already be non-affiliate, it should be picked.
	sources := []Source{
		{ID: "indep", IsAffiliate: false},
		{ID: "affiliate", IsAffiliate: true},
	}
	got := SelectForPreference(true, sources)
	if got.ID != "indep" {
		t.Errorf("expected sources[0]=indep when it's already non-affiliate, got %q", got.ID)
	}
}

// --- DisclosureFor ---

func TestDisclosureFor_Independent(t *testing.T) {
	got := DisclosureFor(Source{IsAffiliate: false}, false)
	if got != IndependentDisclosure {
		t.Errorf("non-affiliate source should always get IndependentDisclosure, got %q", got)
	}
	got = DisclosureFor(Source{IsAffiliate: false}, true)
	if got != IndependentDisclosure {
		t.Errorf("non-affiliate source should get IndependentDisclosure regardless of preference flag, got %q", got)
	}
}

// THE #190 LB-4 regression test. Pro-tier caller, affiliate source
// (either because the category only had an affiliate candidate, or
// a future bug routed a Pro user to one): the disclosure MUST be
// FTCDisclosure, not ProDisclosure and definitely not the softened
// "no commission" IndependentDisclosure text. Deriving the label from
// user tier instead of Source.IsAffiliate was the root cause of the
// r11/R-20 agentic reports.
func TestDisclosureFor_AffiliatePro_ReturnsFTC(t *testing.T) {
	got := DisclosureFor(Source{IsAffiliate: true}, true)
	if got != FTCDisclosure {
		t.Errorf("affiliate source + Pro-tier caller must get FTCDisclosure (not ProDisclosure, not IndependentDisclosure), got %q", got)
	}
}

func TestDisclosureFor_AffiliateFree(t *testing.T) {
	got := DisclosureFor(Source{IsAffiliate: true}, false)
	if got != FTCDisclosure {
		t.Errorf("affiliate source + free tier should get FTCDisclosure, got %q", got)
	}
}

// --- Integration: round-trip through SelectForPreference + DisclosureFor ---

func TestSelectAndDisclose_ProInsurance_PicksGoogleSearchAndIndependent(t *testing.T) {
	// Today every category in sources.go exposes at least one non-affiliate
	// candidate; for insurance that's a plain Google search. So Pro users
	// requesting insurance get the Google search source and an
	// IndependentDisclosure — NOT the SafetyWing affiliate fallback. This
	// test pins that behaviour: if InsuranceSources is ever changed to drop
	// the Google fallback, this test will fail loudly (rather than silently
	// flipping to a ProDisclosure path via a t.Logf no-op).
	b := NewLinkBuilder(LinkBuilderConfig{SafetyWingID: "sw303"})
	sources := b.InsuranceSources("Japan", false)
	selected := SelectForPreference(true, sources)
	disc := DisclosureFor(selected, true)

	if selected.IsAffiliate {
		t.Fatalf("Pro insurance selection must today be the non-affiliate Google search, got affiliate source %+v (if InsuranceSources lost its Google fallback, see TestSelectAndDisclose_ProInsuranceAffiliateFallback for the defensive path)", selected)
	}
	if selected.Partner != PartnerGoogle {
		t.Errorf("Pro insurance non-affiliate source should be PartnerGoogle, got %q", selected.Partner)
	}
	if disc != IndependentDisclosure {
		t.Errorf("Pro insurance independent path must use IndependentDisclosure, got %q", disc)
	}
}

func TestSelectAndDisclose_ProInsuranceAffiliateFallback(t *testing.T) {
	// Defensive coverage: if a future change removes the Google search
	// fallback from InsuranceSources (or any other category that loses its
	// independent option), SelectForPreference must fall back to the
	// affiliate candidate and DisclosureFor must produce FTCDisclosure —
	// the SAME label free-tier users see on affiliate URLs. Softening it
	// for Pro was the #190 LB-4 bug: a Pro user looking at a
	// commission-earning URL deserves the same straightforward partner-
	// link disclosure as everyone else.
	onlyAffiliate := []Source{
		{
			ID:          "safetywing",
			Partner:     PartnerSafetyWing,
			IsAffiliate: true,
			URL:         "https://safetywing.com/nomad-insurance?referenceID=sw303",
		},
	}
	selected := SelectForPreference(true, onlyAffiliate)
	if !selected.IsAffiliate {
		t.Fatalf("expected affiliate fallback when no independent option exists, got %+v", selected)
	}
	if disc := DisclosureFor(selected, true); disc != FTCDisclosure {
		t.Errorf("affiliate fallback for Pro must use FTCDisclosure (not ProDisclosure), got %q", disc)
	}
}

func TestSelectAndDisclose_ProFlight_PicksGoogleAndIndependent(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{SkyscannerID: "sky123"})
	sources := b.FlightSources("JFK", "PRG", "2026-06-15", "", false)
	selected := SelectForPreference(true, sources)
	if selected.Partner != PartnerGoogle {
		t.Errorf("Pro flight should select Google, got %q", selected.Partner)
	}
	if DisclosureFor(selected, true) != IndependentDisclosure {
		t.Errorf("Pro flight should get IndependentDisclosure")
	}
}

func TestSelectAndDisclose_FreeFlight_PicksSkyscannerAndFTC(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{SkyscannerID: "sky123"})
	sources := b.FlightSources("JFK", "PRG", "2026-06-15", "", false)
	selected := SelectForPreference(false, sources)
	if selected.Partner != PartnerSkyscanner {
		t.Errorf("Free flight should select Skyscanner, got %q", selected.Partner)
	}
	if DisclosureFor(selected, false) != FTCDisclosure {
		t.Errorf("Free flight should get FTCDisclosure")
	}
}
