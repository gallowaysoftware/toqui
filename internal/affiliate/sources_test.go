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

func TestFlightSources_NoAffiliateID_StillReturnsSkyscanner(t *testing.T) {
	// Without a configured affiliate ID, the Skyscanner source still appears
	// (it's a usable URL) but IsAffiliate must be false — there's no commission
	// to disclose.
	b := NewLinkBuilder(LinkBuilderConfig{})
	got := b.FlightSources("JFK", "PRG", "2026-06-15", "")

	if len(got) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(got))
	}
	if got[0].Partner != PartnerSkyscanner {
		t.Errorf("expected first source = Skyscanner, got %q", got[0].Partner)
	}
	if got[0].IsAffiliate {
		t.Error("Skyscanner source must be IsAffiliate=false when no ID is configured")
	}
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
