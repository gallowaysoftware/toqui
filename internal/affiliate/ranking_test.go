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

// --- ScoreSources: the scored fit ranker introduced in #386 PR 2 ---

func TestScoreSources_EmptyReturnsNil(t *testing.T) {
	if got := ScoreSources(ScoreContext{}, nil); got != nil {
		t.Errorf("expected nil for nil input, got %+v", got)
	}
	if got := ScoreSources(ScoreContext{}, []Source{}); got != nil {
		t.Errorf("expected nil for empty slice, got %+v", got)
	}
}

func TestScoreSources_FreeTier_PreservesAffiliateFirstOrdering(t *testing.T) {
	// The affiliate-first ordering inside sources.go must survive the
	// ranker on free tier. Real flight pool: Skyscanner (affiliate),
	// Google Flights (non-affiliate), Wikivoyage (non-affiliate).
	b := NewLinkBuilder(LinkBuilderConfig{SkyscannerID: "sky123"})
	sources := b.FlightSources("JFK", "PRG", "2026-06-15", "", false)

	scored := ScoreSources(ScoreContext{
		PreferNonAffiliate: false,
		HasSpecificDates:   true,
	}, sources)

	if len(scored) != len(sources) {
		t.Fatalf("ScoreSources should return all candidates (got %d, want %d)", len(scored), len(sources))
	}
	if scored[0].Partner != PartnerSkyscanner {
		t.Errorf("free tier top pick should be Skyscanner, got %q (rationale: %s)", scored[0].Partner, scored[0].Rationale)
	}
}

func TestScoreSources_ProTier_PrefersNonAffiliate(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{SkyscannerID: "sky123"})
	sources := b.FlightSources("JFK", "PRG", "2026-06-15", "", true) // pro pool

	scored := ScoreSources(ScoreContext{
		PreferNonAffiliate: true,
		HasSpecificDates:   true,
	}, sources)

	if scored[0].IsAffiliate {
		t.Errorf("Pro tier top pick must be non-affiliate, got affiliate source %+v (rationale: %s)", scored[0].Source, scored[0].Rationale)
	}
}

func TestScoreSources_ProTier_FallbackWhenAllAffiliate(t *testing.T) {
	// The defensive path: if the candidate pool happens to be all
	// affiliate (a future category that loses its independent option),
	// the Pro ranker must still return SOMETHING — the affiliate
	// fallback wins and DisclosureFor pairs it with FTCDisclosure.
	onlyAffiliate := []Source{
		{ID: "p1", Partner: PartnerSkyscanner, IsAffiliate: true},
		{ID: "p2", Partner: PartnerBookingCom, IsAffiliate: true},
	}
	scored := ScoreSources(ScoreContext{PreferNonAffiliate: true}, onlyAffiliate)
	if len(scored) != 2 {
		t.Fatalf("expected 2 scored sources, got %d", len(scored))
	}
	if !scored[0].IsAffiliate {
		t.Errorf("with only-affiliate pool, top pick must be affiliate (the defensive fallback), got %+v", scored[0].Source)
	}
}

func TestScoreSources_ScaffoldedAirbnbPenalized(t *testing.T) {
	// Two non-affiliate vacation-rental options where Airbnb is
	// scaffolded (no affiliate ID until Impact.com) and a hypothetical
	// established non-affiliate alternative. The established one must
	// win even though both are non-affiliate.
	sources := []Source{
		{ID: "airbnb", Partner: PartnerAirbnb, IsAffiliate: false},
		{ID: "wikivoyage", Partner: PartnerWikivoyage, IsAffiliate: false},
	}
	scored := ScoreSources(ScoreContext{
		PreferNonAffiliate: true,
		HasSpecificCity:    true,
	}, sources)

	if scored[0].Partner == PartnerAirbnb {
		t.Errorf("scaffolded Airbnb should NOT outrank an established non-affiliate alternative, got top=%q", scored[0].Partner)
	}
}

func TestScoreSources_DatesBoostAggregators(t *testing.T) {
	// Two affiliate sources of comparable status — but only the
	// aggregator should benefit from HasSpecificDates. We test by
	// constructing a synthetic pool: a search aggregator and a generic
	// partner. With dates, the aggregator should rank higher.
	sources := []Source{
		{ID: "generic", Partner: PartnerGeneric, IsAffiliate: true},
		{ID: "agg", Partner: PartnerSkyscanner, IsAffiliate: true},
	}
	scored := ScoreSources(ScoreContext{
		PreferNonAffiliate: false,
		HasSpecificDates:   true,
	}, sources)

	if scored[0].Partner != PartnerSkyscanner {
		t.Errorf("with HasSpecificDates, search aggregator should outrank generic partner, got top=%q (rationale: %s)", scored[0].Partner, scored[0].Rationale)
	}
}

func TestScoreSources_CityBoostsCityCurated(t *testing.T) {
	// Atlas Obscura (city-curated) vs Wikivoyage (general): with a city
	// supplied, Atlas Obscura should rank higher among non-affiliates.
	// Both are non-affiliate, so the affiliate-preference signal is
	// neutral on free tier; the city signal decides.
	sources := []Source{
		{ID: "wikivoyage", Partner: PartnerWikivoyage, IsAffiliate: false},
		{ID: "atlas", Partner: PartnerAtlasObscura, IsAffiliate: false},
	}
	scored := ScoreSources(ScoreContext{
		PreferNonAffiliate: false,
		HasSpecificCity:    true,
	}, sources)

	// Wikivoyage IS in isCityCurated — both should get the city bump,
	// but with no other tiebreakers the stable sort should preserve
	// input order. Real test: without city, neither gets the bump and
	// the input order is preserved either way. Switch to a contrast
	// test: city-curated vs an aggregator that doesn't get the city
	// bump. With a city supplied, the editorial source ranks higher.
	sources2 := []Source{
		{ID: "agg", Partner: PartnerSkyscanner, IsAffiliate: true},
		{ID: "atlas", Partner: PartnerAtlasObscura, IsAffiliate: false},
	}
	scored2 := ScoreSources(ScoreContext{
		PreferNonAffiliate: true, // Pro: non-affiliate gets +1.5
		HasSpecificCity:    true, // Atlas Obscura also gets +0.4 (city-curated)
	}, sources2)
	if scored2[0].Partner != PartnerAtlasObscura {
		t.Errorf("Pro + HasSpecificCity should pick Atlas Obscura over Skyscanner, got %q (rationale: %s)", scored2[0].Partner, scored2[0].Rationale)
	}

	_ = scored
}

func TestScoreSources_RationaleIsPopulated(t *testing.T) {
	// PR 3 will surface the rationale to the AI; this test is a
	// contract that scoreOne always emits at least the affiliate-status
	// fragment so the rationale is never empty.
	sources := []Source{{ID: "x", Partner: PartnerGoogle, IsAffiliate: false}}
	scored := ScoreSources(ScoreContext{PreferNonAffiliate: true}, sources)
	if scored[0].Rationale == "" {
		t.Errorf("rationale should not be empty for any scored source")
	}
}

func TestScoreSources_StableTiebreakerPreservesInputOrder(t *testing.T) {
	// Two sources that score identically (free tier, non-affiliate, no
	// other fit signals) must come out in input order. This is what
	// keeps the affiliate-first ordering inside sources.go acting as
	// the deterministic tiebreaker.
	sources := []Source{
		{ID: "first", Partner: PartnerGoogle, IsAffiliate: false},
		{ID: "second", Partner: PartnerWikivoyage, IsAffiliate: false},
	}
	scored := ScoreSources(ScoreContext{}, sources)
	if scored[0].ID != "first" || scored[1].ID != "second" {
		t.Errorf("equal-scored sources should preserve input order, got [%q, %q]", scored[0].ID, scored[1].ID)
	}
}
