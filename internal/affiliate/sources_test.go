package affiliate

import (
	"strings"
	"testing"
)

// --- FlightSources ---

func TestFlightSources_OrderingAffiliateFirst(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{SkyscannerID: "sky123"})
	got := b.FlightSources("JFK", "PRG", "2026-06-15", "", false)

	if len(got) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(got))
	}
	if got[0].Partner != PartnerSkyscanner {
		t.Errorf("expected first source = Skyscanner, got %q", got[0].Partner)
	}
	if !got[0].IsAffiliate {
		t.Error("Skyscanner source should be marked IsAffiliate=true when ID is configured")
	}
	if got[1].Partner != PartnerGoogle {
		t.Errorf("expected second source = Google, got %q", got[1].Partner)
	}
	if got[1].IsAffiliate {
		t.Error("Google Flights source must never be marked IsAffiliate")
	}
}

func TestFlightSources_NoAffiliateID_StillAffiliatePartner(t *testing.T) {
	// Without a configured Skyscanner associateid, the Skyscanner source
	// still appears (the URL is usable) and MUST still be marked
	// IsAffiliate=true. The field is a static property of the partner
	// domain — skyscanner.com is a commercial flight aggregator regardless
	// of whether our tracking ID is plumbed in for this environment.
	//
	// Regression pin for the PR #332 follow-up: the old code conflated
	// "has tracking ID" with "is an affiliate partner". That made Pro-tier
	// selection pick un-ID'd skyscanner/booking/getyourguide URLs as the
	// "first independent source" and paired them with the IndependentDisclosure
	// copy — an affirmative misleading claim. For the "is our tracking ID
	// wired up?" concept, use LinkBuilder.HasPartner instead.
	b := NewLinkBuilder(LinkBuilderConfig{})
	got := b.FlightSources("JFK", "PRG", "2026-06-15", "", false)

	if len(got) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(got))
	}
	if got[0].Partner != PartnerSkyscanner {
		t.Errorf("expected first source = Skyscanner, got %q", got[0].Partner)
	}
	if !got[0].IsAffiliate {
		t.Error("Skyscanner source must be IsAffiliate=true regardless of whether the tracking ID is configured — it's a static partner-domain property")
	}
	// The URL must still not carry the tracking ID when it's not configured.
	if strings.Contains(got[0].URL, "associateid") {
		t.Errorf("URL should not contain associateid when no ID is configured: %s", got[0].URL)
	}
}

func TestFlightSources_GoogleFlightsURL(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{SkyscannerID: "sky123"})
	got := b.FlightSources("JFK", "PRG", "2026-06-15", "", false)

	gf := got[1]
	if !strings.HasPrefix(gf.URL, "https://www.google.com/travel/flights?q=") {
		t.Errorf("Google Flights URL malformed: %s", gf.URL)
	}
	if !strings.Contains(gf.URL, "Flights+from+JFK+to+PRG") {
		t.Errorf("Google Flights URL should encode the query: %s", gf.URL)
	}
	if !strings.Contains(gf.URL, "2026-06-15") {
		t.Errorf("Google Flights URL should include the date: %s", gf.URL)
	}
}

func TestFlightSources_GoogleFlightsAnytimeOmitsDate(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{})
	got := b.FlightSources("JFK", "PRG", "anytime", "", false)
	gf := got[1]
	if strings.Contains(gf.URL, "anytime") {
		t.Errorf("Google Flights URL should not include the literal 'anytime' sentinel: %s", gf.URL)
	}
}

func TestFlightSources_SubIDOnlyOnAffiliate(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{SkyscannerID: "sky123"})
	got := b.FlightSources("JFK", "PRG", "2026-06-15", "trip-hash-123", false)

	if !strings.Contains(got[0].URL, "utm_content=trip-hash-123") {
		t.Errorf("affiliate URL should carry sub-ID: %s", got[0].URL)
	}
	if strings.Contains(got[1].URL, "utm_content") {
		t.Errorf("Google Flights URL must never carry a sub-ID: %s", got[1].URL)
	}
}

// --- HotelSources ---

func TestHotelSources_OrderingAffiliateFirst(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{BookingComID: "book456"})
	got := b.HotelSources("", "Prague", "2026-06-15", "2026-06-20", "", false)

	if len(got) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(got))
	}
	if got[0].Partner != PartnerBookingCom {
		t.Errorf("expected first source = BookingCom, got %q", got[0].Partner)
	}
	if !got[0].IsAffiliate {
		t.Error("Booking.com source should be IsAffiliate=true when ID configured")
	}
	if got[1].Partner != PartnerGoogle {
		t.Errorf("expected second source = Google, got %q", got[1].Partner)
	}
	if got[1].IsAffiliate {
		t.Error("Google Maps hotel source must never be IsAffiliate")
	}
}

func TestHotelSources_PropertyNameUsesGoogleSearch(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{})
	got := b.HotelSources("Park Hyatt Tokyo", "Tokyo", "", "", "", false)
	gh := got[1]

	if !strings.HasPrefix(gh.URL, "https://www.google.com/search?q=") {
		t.Errorf("when propertyName is set, independent source should be a Google search URL: %s", gh.URL)
	}
	if !strings.Contains(gh.URL, "Park+Hyatt+Tokyo+hotel") {
		t.Errorf("Google search URL should include property name + 'hotel': %s", gh.URL)
	}
}

func TestHotelSources_NoPropertyUsesGoogleMaps(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{})
	got := b.HotelSources("", "Reykjavik", "", "", "", false)
	gh := got[1]

	if !strings.HasPrefix(gh.URL, "https://www.google.com/maps/search/") {
		t.Errorf("without property name, independent source should be a Google Maps search: %s", gh.URL)
	}
	if !strings.Contains(gh.URL, "hotels%20in%20Reykjavik") {
		t.Errorf("Google Maps URL should include 'hotels in <city>': %s", gh.URL)
	}
}

// --- ActivitySources ---

func TestActivitySources_WithCity_IncludesWikivoyage(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{GetYourGuideID: "gyg789"})
	got := b.ActivitySources("walking tour", "Prague", "", false)

	if len(got) != 3 {
		t.Fatalf("expected 3 sources (gyg, google, wikivoyage), got %d", len(got))
	}
	if got[0].Partner != PartnerGetYourGuide || !got[0].IsAffiliate {
		t.Errorf("expected affiliate first = GetYourGuide, got %q (affiliate=%v)", got[0].Partner, got[0].IsAffiliate)
	}
	if got[1].Partner != PartnerGoogle || got[1].IsAffiliate {
		t.Errorf("expected second = Google (independent), got %q (affiliate=%v)", got[1].Partner, got[1].IsAffiliate)
	}
	if got[2].Partner != PartnerWikivoyage || got[2].IsAffiliate {
		t.Errorf("expected third = Wikivoyage (independent), got %q (affiliate=%v)", got[2].Partner, got[2].IsAffiliate)
	}
}

func TestActivitySources_NoCity_OmitsWikivoyage(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{GetYourGuideID: "gyg789"})
	got := b.ActivitySources("walking tour", "", "", false)

	if len(got) != 2 {
		t.Fatalf("expected 2 sources without city, got %d", len(got))
	}
	for _, s := range got {
		if s.Partner == PartnerWikivoyage {
			t.Error("Wikivoyage source should not appear when city is empty")
		}
	}
}

func TestActivitySources_WikivoyageURLEncoding(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{})
	got := b.ActivitySources("anything", "New York City", "", false)

	var wv Source
	for _, s := range got {
		if s.Partner == PartnerWikivoyage {
			wv = s
		}
	}
	if wv.URL == "" {
		t.Fatal("expected Wikivoyage source for city='New York City'")
	}
	// Wikivoyage uses underscores for spaces in page titles
	if !strings.Contains(wv.URL, "New_York_City") {
		t.Errorf("Wikivoyage URL should use underscores in page slug: %s", wv.URL)
	}
}

// --- CarRentalSources ---

func TestCarRentalSources_OrderingAffiliateFirst(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{DiscoverCarsID: "dc202"})
	got := b.CarRentalSources("Lisbon", "2026-07-01", "2026-07-10", false)

	if len(got) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(got))
	}
	if got[0].Partner != PartnerDiscoverCars || !got[0].IsAffiliate {
		t.Errorf("expected first = DiscoverCars (affiliate), got %q (affiliate=%v)", got[0].Partner, got[0].IsAffiliate)
	}
	if got[1].Partner != PartnerGoogle || got[1].IsAffiliate {
		t.Errorf("expected second = Google (independent), got %q (affiliate=%v)", got[1].Partner, got[1].IsAffiliate)
	}
	if !strings.Contains(got[1].URL, "car%20rental%20near%20Lisbon") {
		t.Errorf("Google Maps URL should include 'car rental near <location>': %s", got[1].URL)
	}
}

// --- InsuranceSources ---

func TestInsuranceSources_HasIndependentFallback(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{SafetyWingID: "sw303"})
	got := b.InsuranceSources("Japan", false)

	if len(got) != 2 {
		t.Fatalf("expected 2 sources (safetywing, google), got %d", len(got))
	}
	if got[0].Partner != PartnerSafetyWing || !got[0].IsAffiliate {
		t.Errorf("expected first = SafetyWing (affiliate), got %q (affiliate=%v)", got[0].Partner, got[0].IsAffiliate)
	}
	if got[1].Partner != PartnerGoogle || got[1].IsAffiliate {
		t.Errorf("expected second = Google search (independent), got %q (affiliate=%v)", got[1].Partner, got[1].IsAffiliate)
	}
	if !strings.Contains(got[1].URL, "travel+insurance+for+Japan") {
		t.Errorf("Google insurance URL should include 'travel insurance for <dest>': %s", got[1].URL)
	}
}

// --- PR #332 follow-up regression coverage ---

// TestSources_IsAffiliateIsStaticPerPartner pins the contract that every
// source builder reports IsAffiliate based purely on the Partner identity,
// never on whether the tracking ID is plumbed through for this environment.
//
// This is the unit-level mirror of the R-11 / R-20 agentic-test findings:
// both personas (Pro tier, local env, partner IDs unset) caught the tool
// returning affiliate URLs labelled with IndependentDisclosure. Root cause
// was `IsAffiliate: b.xxxID != ""` in every builder. If anyone ever
// reintroduces that pattern, this test fails at compile-speed before any
// user is misled.
func TestSources_IsAffiliateIsStaticPerPartner(t *testing.T) {
	// Empty LinkBuilderConfig — mimics a local dev env, or a staging env
	// where a partner's ID env var accidentally went unset. The previous
	// code silently flipped IsAffiliate to false here, with user-visible
	// consequences.
	b := NewLinkBuilder(LinkBuilderConfig{})

	cases := []struct {
		name    string
		sources []Source
	}{
		{"FlightSources", b.FlightSources("JFK", "PRG", "2026-06-15", "", false)},
		{"HotelSources", b.HotelSources("", "Prague", "2026-06-15", "2026-06-20", "", false)},
		{"ActivitySources", b.ActivitySources("walking tour", "Prague", "", false)},
		{"CarRentalSources", b.CarRentalSources("Lisbon", "2026-07-01", "2026-07-10", false)},
		{"InsuranceSources", b.InsuranceSources("Japan", false)},
	}

	// Partners that are always commission-earning commercial domains.
	// If you add a new affiliate partner, add it here and verify the
	// builder marks it IsAffiliate=true unconditionally.
	affiliatePartners := map[Partner]bool{
		PartnerSkyscanner:   true,
		PartnerBookingCom:   true,
		PartnerGetYourGuide: true,
		PartnerViator:       true,
		PartnerDiscoverCars: true,
		PartnerSafetyWing:   true,
	}
	// Partners that are genuinely independent (commission-free).
	independentPartners := map[Partner]bool{
		PartnerGoogle:      true,
		PartnerWikivoyage:  true,
		PartnerOfficialGov: true,
	}

	for _, tc := range cases {
		for _, s := range tc.sources {
			switch {
			case affiliatePartners[s.Partner]:
				if !s.IsAffiliate {
					t.Errorf("%s: partner %q is a commission-earning domain but IsAffiliate=false (the exact bug R-11 and R-20 caught)", tc.name, s.Partner)
				}
			case independentPartners[s.Partner]:
				if s.IsAffiliate {
					t.Errorf("%s: partner %q is an independent source but IsAffiliate=true (would cause us to under-disclose to Pro users)", tc.name, s.Partner)
				}
			default:
				t.Errorf("%s: source %q has unclassified partner %q — add it to affiliatePartners or independentPartners above", tc.name, s.ID, s.Partner)
			}
		}
	}
}

// TestProTierSelection_EmptyConfig_GetsIndependentSource is the integration
// test that would have caught the R-11 / R-20 regression pre-merge. It
// wires the full chain — source builder, SelectForPreference(true),
// DisclosureFor — against a LinkBuilder with no affiliate IDs configured,
// which is the exact scenario the agentic-test harness runs in.
//
// For a Pro user (preferNonAffiliate=true), the selected source must be
// an independent partner (Google or Wikivoyage), the URL must not point
// at a commercial aggregator domain, and the disclosure must be
// IndependentDisclosure. If this test fails, Pro users are being told
// "Toqui earns no commission" on links that go straight to booking.com,
// getyourguide.com, or skyscanner.com — a consumer-protection hazard.
func TestProTierSelection_EmptyConfig_GetsIndependentSource(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{}) // no affiliate IDs — local dev scenario

	cases := []struct {
		name           string
		sources        []Source
		wantPartner    Partner
		forbiddenHosts []string
	}{
		{
			name:           "flight",
			sources:        b.FlightSources("JFK", "PRG", "2026-06-15", "", false),
			wantPartner:    PartnerGoogle,
			forbiddenHosts: []string{"skyscanner.com"},
		},
		{
			name:           "hotel (city-only)",
			sources:        b.HotelSources("", "Prague", "2026-06-15", "2026-06-20", "", false),
			wantPartner:    PartnerGoogle,
			forbiddenHosts: []string{"booking.com"},
		},
		{
			name:           "hotel (property name)",
			sources:        b.HotelSources("Park Hyatt Tokyo", "Tokyo", "", "", "", false),
			wantPartner:    PartnerGoogle,
			forbiddenHosts: []string{"booking.com"},
		},
		{
			name:           "activity",
			sources:        b.ActivitySources("walking tour", "Prague", "", false),
			wantPartner:    PartnerGoogle, // Google Maps is preferred over Wikivoyage (earlier in slice)
			forbiddenHosts: []string{"getyourguide.com", "viator.com"},
		},
		{
			name:           "car rental",
			sources:        b.CarRentalSources("Lisbon", "2026-07-01", "2026-07-10", false),
			wantPartner:    PartnerGoogle,
			forbiddenHosts: []string{"discovercars.com"},
		},
		{
			name:           "insurance",
			sources:        b.InsuranceSources("Japan", false),
			wantPartner:    PartnerGoogle,
			forbiddenHosts: []string{"safetywing.com"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			selected := SelectForPreference(true, tc.sources)
			if selected.Partner != tc.wantPartner {
				t.Errorf("Pro selection should be %q, got %q (URL: %s)", tc.wantPartner, selected.Partner, selected.URL)
			}
			if selected.IsAffiliate {
				t.Errorf("Pro selection must be IsAffiliate=false, got %+v", selected)
			}
			for _, host := range tc.forbiddenHosts {
				if strings.Contains(selected.URL, host) {
					t.Errorf("Pro selection URL must not point at commercial aggregator %q: %s", host, selected.URL)
				}
			}
			if disc := DisclosureFor(selected, true); disc != IndependentDisclosure {
				t.Errorf("Pro selection must pair with IndependentDisclosure, got %q", disc)
			}
		})
	}
}

// TestFreeTierSelection_EmptyConfig_StillHonorsFTC pins the dual of the
// Pro-tier test: for a free-tier user, the selected source must be the
// affiliate partner (even when un-ID'd — sources[0]) and carry
// FTCDisclosure, never IndependentDisclosure. This stops a "fix" to
// IsAffiliate from accidentally breaking free-tier commission attribution
// or disclosure.
func TestFreeTierSelection_EmptyConfig_StillHonorsFTC(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{})

	cases := []struct {
		name        string
		sources     []Source
		wantPartner Partner
	}{
		{"flight", b.FlightSources("JFK", "PRG", "2026-06-15", "", false), PartnerSkyscanner},
		{"hotel", b.HotelSources("", "Prague", "2026-06-15", "2026-06-20", "", false), PartnerBookingCom},
		{"activity", b.ActivitySources("walking tour", "Prague", "", false), PartnerGetYourGuide},
		{"car rental", b.CarRentalSources("Lisbon", "2026-07-01", "2026-07-10", false), PartnerDiscoverCars},
		{"insurance", b.InsuranceSources("Japan", false), PartnerSafetyWing},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			selected := SelectForPreference(false, tc.sources)
			if selected.Partner != tc.wantPartner {
				t.Errorf("free selection should be %q, got %q", tc.wantPartner, selected.Partner)
			}
			if !selected.IsAffiliate {
				t.Errorf("free selection must be IsAffiliate=true, got %+v", selected)
			}
			if disc := DisclosureFor(selected, false); disc != FTCDisclosure {
				t.Errorf("free selection must pair with FTCDisclosure, got %q", disc)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Pro-tier source-pool widening (#386)
//
// When includePro=true, each per-category builder appends additional
// candidate sources that the free-tier pool deliberately excludes. The
// invariants we pin:
//
//   - Free pool is a strict subset of Pro pool (same prefix, Pro just
//     appends). A future bug that reorders or replaces sources between
//     tiers gets caught here.
//   - Every Pro-only source is IsAffiliate=false. The marketing claim
//     ("widens beyond affiliate partners") would be falsified if any
//     Pro addition was a commission source. If/when we sign an
//     Airbnb / Squaremouth affiliate partnership, IsAffiliate flips to
//     true on that specific source and these tests must be updated
//     deliberately.
//   - Each Pro source has a non-empty URL — pin against a regression
//     where a builder appends a Source{} zero value.
// ---------------------------------------------------------------------------

func TestProTier_FreePoolIsSubsetOfProPool(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{})

	cases := []struct {
		name string
		free []Source
		pro  []Source
	}{
		{"flights",
			b.FlightSources("JFK", "PRG", "2026-06-15", "", false),
			b.FlightSources("JFK", "PRG", "2026-06-15", "", true)},
		{"hotels",
			b.HotelSources("", "Prague", "2026-06-15", "2026-06-20", "", false),
			b.HotelSources("", "Prague", "2026-06-15", "2026-06-20", "", true)},
		{"activities",
			b.ActivitySources("walking tour", "Prague", "", false),
			b.ActivitySources("walking tour", "Prague", "", true)},
		{"car rental",
			b.CarRentalSources("Lisbon", "2026-07-01", "2026-07-10", false),
			b.CarRentalSources("Lisbon", "2026-07-01", "2026-07-10", true)},
		{"insurance",
			b.InsuranceSources("Japan", false),
			b.InsuranceSources("Japan", true)},
		{"vacation rental",
			b.VacationRentalSources("Lisbon", "", "", "", false),
			b.VacationRentalSources("Lisbon", "", "", "", true)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.pro) <= len(tc.free) {
				t.Errorf("Pro pool (%d) must strictly extend free pool (%d) — got no widening",
					len(tc.pro), len(tc.free))
			}
			for i, free := range tc.free {
				if i >= len(tc.pro) || tc.pro[i].ID != free.ID {
					t.Errorf("Pro pool[%d] ID = %q, want free pool[%d] ID = %q (Pro must append, not reorder)",
						i, tc.pro[i].ID, i, free.ID)
				}
			}
		})
	}
}

func TestProTier_AllProAdditionsAreNonAffiliate(t *testing.T) {
	// The marketing claim is "widens beyond affiliate partners". Pin
	// that every Pro-only addition is IsAffiliate=false. If/when an
	// Airbnb / Squaremouth affiliate partnership is signed, flip the
	// specific source's IsAffiliate=true and update this test
	// deliberately — the invariant change is the signal.
	b := NewLinkBuilder(LinkBuilderConfig{})

	cases := []struct {
		name string
		free []Source
		pro  []Source
	}{
		{"flights",
			b.FlightSources("JFK", "PRG", "2026-06-15", "", false),
			b.FlightSources("JFK", "PRG", "2026-06-15", "", true)},
		{"hotels",
			b.HotelSources("", "Prague", "2026-06-15", "2026-06-20", "", false),
			b.HotelSources("", "Prague", "2026-06-15", "2026-06-20", "", true)},
		{"activities (with city)",
			b.ActivitySources("walking tour", "Prague", "", false),
			b.ActivitySources("walking tour", "Prague", "", true)},
		{"car rental",
			b.CarRentalSources("Lisbon", "2026-07-01", "2026-07-10", false),
			b.CarRentalSources("Lisbon", "2026-07-01", "2026-07-10", true)},
		{"insurance",
			b.InsuranceSources("Japan", false),
			b.InsuranceSources("Japan", true)},
		{"vacation rental",
			b.VacationRentalSources("Lisbon", "", "", "", false),
			b.VacationRentalSources("Lisbon", "", "", "", true)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			additions := tc.pro[len(tc.free):]
			if len(additions) == 0 {
				t.Skip("no Pro additions for this category yet")
			}
			for _, src := range additions {
				if src.IsAffiliate {
					t.Errorf("Pro addition %q (%s) is IsAffiliate=true — violates 'widens beyond affiliate partners' marketing claim",
						src.ID, src.Partner)
				}
				if src.URL == "" {
					t.Errorf("Pro addition %q has empty URL — likely a Source{} zero value bug", src.ID)
				}
				if src.Title == "" {
					t.Errorf("Pro addition %q has empty Title — would render as an empty card in chat", src.ID)
				}
			}
		})
	}
}

func TestProTier_FlightSources_AddsITAMatrixAndMomondo(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{})
	pro := b.FlightSources("JFK", "PRG", "2026-06-15", "", true)

	// Find the additions by ID — we don't pin position because future
	// builders may reorder.
	ids := make(map[string]Source, len(pro))
	for _, s := range pro {
		ids[s.ID] = s
	}

	if _, ok := ids["ita_matrix"]; !ok {
		t.Errorf("Pro flight pool should include ita_matrix; got IDs: %v", mapsKeys(ids))
	}
	if _, ok := ids["momondo"]; !ok {
		t.Errorf("Pro flight pool should include momondo; got IDs: %v", mapsKeys(ids))
	}
	// Pin the URL shapes — these are part of the contract with the
	// upstream services. A typo or ordering change here breaks the
	// integration silently.
	if !strings.Contains(ids["ita_matrix"].URL, "matrix.itasoftware.com") {
		t.Errorf("ita_matrix URL = %q, want matrix.itasoftware.com host", ids["ita_matrix"].URL)
	}
	if !strings.Contains(ids["momondo"].URL, "momondo.com") {
		t.Errorf("momondo URL = %q, want momondo.com host", ids["momondo"].URL)
	}
}

func TestProTier_HotelSources_AddsHotellook(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{})
	pro := b.HotelSources("", "Prague", "2026-06-15", "2026-06-20", "", true)
	for _, s := range pro {
		if s.ID == "hotellook" {
			if !strings.Contains(s.URL, "hotellook.com") {
				t.Errorf("hotellook URL = %q, want hotellook.com host", s.URL)
			}
			return
		}
	}
	t.Error("Pro hotel pool should include hotellook")
}

func TestProTier_ActivitySources_AddsAtlasObscuraAndTimeOut(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{})
	pro := b.ActivitySources("walking tour", "Prague", "", true)

	ids := make(map[string]Source, len(pro))
	for _, s := range pro {
		ids[s.ID] = s
	}
	if _, ok := ids["atlas_obscura"]; !ok {
		t.Errorf("Pro activity pool should include atlas_obscura; got: %v", mapsKeys(ids))
	}
	if _, ok := ids["timeout"]; !ok {
		t.Errorf("Pro activity pool with city should include timeout; got: %v", mapsKeys(ids))
	}
}

func TestProTier_ActivitySources_TimeOutOmittedWhenCityMissing(t *testing.T) {
	// Time Out's URL slug pattern is the city name. Without a city we
	// can't construct a meaningful URL, so the source must be skipped
	// rather than landing the user on a broken page.
	b := NewLinkBuilder(LinkBuilderConfig{})
	pro := b.ActivitySources("walking tour", "", "", true)
	for _, s := range pro {
		if s.ID == "timeout" {
			t.Errorf("timeout source should be omitted when city is empty, got URL %q", s.URL)
		}
	}
}

func TestProTier_CarRentalSources_AddsTuroAndAutoEurope(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{})
	pro := b.CarRentalSources("Lisbon", "2026-07-01", "2026-07-10", true)

	ids := make(map[string]Source, len(pro))
	for _, s := range pro {
		ids[s.ID] = s
	}
	if _, ok := ids["turo"]; !ok {
		t.Errorf("Pro car-rental pool should include turo; got: %v", mapsKeys(ids))
	}
	if _, ok := ids["auto_europe"]; !ok {
		t.Errorf("Pro car-rental pool should include auto_europe; got: %v", mapsKeys(ids))
	}
}

func TestProTier_InsuranceSources_AddsSquaremouthAndInsureMyTrip(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{})
	pro := b.InsuranceSources("Japan", true)

	ids := make(map[string]Source, len(pro))
	for _, s := range pro {
		ids[s.ID] = s
	}
	if _, ok := ids["squaremouth"]; !ok {
		t.Errorf("Pro insurance pool should include squaremouth; got: %v", mapsKeys(ids))
	}
	if _, ok := ids["insuremytrip"]; !ok {
		t.Errorf("Pro insurance pool should include insuremytrip; got: %v", mapsKeys(ids))
	}
}

func TestProTier_VacationRentalSources_AddsAirbnb(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{})
	pro := b.VacationRentalSources("Lisbon", "", "", "", true)
	for _, s := range pro {
		if s.ID == "airbnb" {
			// Verifies the explicit comment in the Pro-tier code: Airbnb
			// is currently scaffolded WITHOUT an affiliate ID. If an
			// Impact.com partnership lands, IsAffiliate flips to true
			// and this test must be updated.
			if s.IsAffiliate {
				t.Errorf("airbnb source should be non-affiliate until Impact.com partnership is signed; got IsAffiliate=true")
			}
			if !strings.Contains(s.URL, "airbnb.com") {
				t.Errorf("airbnb URL = %q, want airbnb.com host", s.URL)
			}
			return
		}
	}
	t.Error("Pro vacation-rental pool should include airbnb")
}

// mapsKeys is a tiny test helper — pulled out so the test bodies stay
// focused on the assertion. Drop once Go's std map-keys helper is
// available everywhere we run.
func mapsKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
