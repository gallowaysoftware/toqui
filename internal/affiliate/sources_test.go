package affiliate

import (
	"strings"
	"testing"
)

// --- FlightSources ---

func TestFlightSources_OrderingAffiliateFirst(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{SkyscannerID: "sky123"})
	got := b.FlightSources("JFK", "PRG", "2026-06-15", "")

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
	got := b.FlightSources("JFK", "PRG", "2026-06-15", "")

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
	got := b.FlightSources("JFK", "PRG", "2026-06-15", "")

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
	got := b.FlightSources("JFK", "PRG", "anytime", "")
	gf := got[1]
	if strings.Contains(gf.URL, "anytime") {
		t.Errorf("Google Flights URL should not include the literal 'anytime' sentinel: %s", gf.URL)
	}
}

func TestFlightSources_SubIDOnlyOnAffiliate(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{SkyscannerID: "sky123"})
	got := b.FlightSources("JFK", "PRG", "2026-06-15", "trip-hash-123")

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
	got := b.HotelSources("", "Prague", "2026-06-15", "2026-06-20", "")

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
	got := b.HotelSources("Park Hyatt Tokyo", "Tokyo", "", "", "")
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
	got := b.HotelSources("", "Reykjavik", "", "", "")
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
	got := b.ActivitySources("walking tour", "Prague", "")

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
	got := b.ActivitySources("walking tour", "", "")

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
	got := b.ActivitySources("anything", "New York City", "")

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
	got := b.CarRentalSources("Lisbon", "2026-07-01", "2026-07-10")

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
	got := b.InsuranceSources("Japan")

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
		{"FlightSources", b.FlightSources("JFK", "PRG", "2026-06-15", "")},
		{"HotelSources", b.HotelSources("", "Prague", "2026-06-15", "2026-06-20", "")},
		{"ActivitySources", b.ActivitySources("walking tour", "Prague", "")},
		{"CarRentalSources", b.CarRentalSources("Lisbon", "2026-07-01", "2026-07-10")},
		{"InsuranceSources", b.InsuranceSources("Japan")},
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
			sources:        b.FlightSources("JFK", "PRG", "2026-06-15", ""),
			wantPartner:    PartnerGoogle,
			forbiddenHosts: []string{"skyscanner.com"},
		},
		{
			name:           "hotel (city-only)",
			sources:        b.HotelSources("", "Prague", "2026-06-15", "2026-06-20", ""),
			wantPartner:    PartnerGoogle,
			forbiddenHosts: []string{"booking.com"},
		},
		{
			name:           "hotel (property name)",
			sources:        b.HotelSources("Park Hyatt Tokyo", "Tokyo", "", "", ""),
			wantPartner:    PartnerGoogle,
			forbiddenHosts: []string{"booking.com"},
		},
		{
			name:           "activity",
			sources:        b.ActivitySources("walking tour", "Prague", ""),
			wantPartner:    PartnerGoogle, // Google Maps is preferred over Wikivoyage (earlier in slice)
			forbiddenHosts: []string{"getyourguide.com", "viator.com"},
		},
		{
			name:           "car rental",
			sources:        b.CarRentalSources("Lisbon", "2026-07-01", "2026-07-10"),
			wantPartner:    PartnerGoogle,
			forbiddenHosts: []string{"discovercars.com"},
		},
		{
			name:           "insurance",
			sources:        b.InsuranceSources("Japan"),
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
		{"flight", b.FlightSources("JFK", "PRG", "2026-06-15", ""), PartnerSkyscanner},
		{"hotel", b.HotelSources("", "Prague", "2026-06-15", "2026-06-20", ""), PartnerBookingCom},
		{"activity", b.ActivitySources("walking tour", "Prague", ""), PartnerGetYourGuide},
		{"car rental", b.CarRentalSources("Lisbon", "2026-07-01", "2026-07-10"), PartnerDiscoverCars},
		{"insurance", b.InsuranceSources("Japan"), PartnerSafetyWing},
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
