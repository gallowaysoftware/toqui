package affiliate

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strings"
	"testing"
)

func TestNewLinkBuilder(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{
		SkyscannerID:       "sky123",
		BookingComID:       "book456",
		GetYourGuideID:     "gyg789",
		ViatorID:           "vtr101",
		DiscoverCarsID:     "dc202",
		SafetyWingID:       "sw303",
		ExpediaPublisherID: "exp404",
	})
	if b.skyscannerID != "sky123" {
		t.Errorf("expected skyscannerID %q, got %q", "sky123", b.skyscannerID)
	}
	if b.bookingComID != "book456" {
		t.Errorf("expected bookingComID %q, got %q", "book456", b.bookingComID)
	}
	if b.getYourGuideID != "gyg789" {
		t.Errorf("expected getYourGuideID %q, got %q", "gyg789", b.getYourGuideID)
	}
	if b.viatorID != "vtr101" {
		t.Errorf("expected viatorID %q, got %q", "vtr101", b.viatorID)
	}
	if b.discoverCarsID != "dc202" {
		t.Errorf("expected discoverCarsID %q, got %q", "dc202", b.discoverCarsID)
	}
	if b.safetyWingID != "sw303" {
		t.Errorf("expected safetyWingID %q, got %q", "sw303", b.safetyWingID)
	}
	if b.expediaPublisherID != "exp404" {
		t.Errorf("expected expediaPublisherID %q, got %q", "exp404", b.expediaPublisherID)
	}
}

func TestFlightSearchURL_WithAffiliateID(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{SkyscannerID: "sky123"})
	u := b.FlightSearchURL("JFK", "PRG", "2026-06-15")

	if !strings.Contains(u, "skyscanner.com/transport/flights/JFK/PRG/2026-06-15") {
		t.Errorf("unexpected URL structure: %s", u)
	}
	if !strings.Contains(u, "associateid=sky123") {
		t.Errorf("expected affiliate ID in URL: %s", u)
	}
}

func TestFlightSearchURL_WithoutAffiliateID(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{})
	u := b.FlightSearchURL("JFK", "PRG", "2026-06-15")

	if !strings.Contains(u, "skyscanner.com/transport/flights/JFK/PRG/2026-06-15") {
		t.Errorf("unexpected URL structure: %s", u)
	}
	if strings.Contains(u, "associateid") {
		t.Errorf("should not contain affiliate param when ID is empty: %s", u)
	}
}

func TestHotelSearchURL_WithAffiliateID(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{BookingComID: "book456"})
	u := b.HotelSearchURL("", "Prague", "2026-06-15", "2026-06-20")

	if !strings.Contains(u, "booking.com/searchresults.html") {
		t.Errorf("unexpected URL structure: %s", u)
	}
	if !strings.Contains(u, "ss=Prague") {
		t.Errorf("expected city in URL: %s", u)
	}
	if !strings.Contains(u, "checkin=2026-06-15") {
		t.Errorf("expected checkin date in URL: %s", u)
	}
	if !strings.Contains(u, "checkout=2026-06-20") {
		t.Errorf("expected checkout date in URL: %s", u)
	}
	if !strings.Contains(u, "aid=book456") {
		t.Errorf("expected affiliate ID in URL: %s", u)
	}
}

func TestHotelSearchURL_WithoutAffiliateID(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{})
	u := b.HotelSearchURL("", "Prague", "2026-06-15", "2026-06-20")

	if !strings.Contains(u, "booking.com/searchresults.html") {
		t.Errorf("unexpected URL structure: %s", u)
	}
	if strings.Contains(u, "aid=") {
		t.Errorf("should not contain affiliate param when ID is empty: %s", u)
	}
}

func TestHotelSearchURL_NoDates(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{BookingComID: "book456"})
	u := b.HotelSearchURL("", "Tokyo", "", "")

	if !strings.Contains(u, "ss=Tokyo") {
		t.Errorf("expected city in URL: %s", u)
	}
	if strings.Contains(u, "checkin=") {
		t.Errorf("should not contain checkin when empty: %s", u)
	}
	if strings.Contains(u, "checkout=") {
		t.Errorf("should not contain checkout when empty: %s", u)
	}
}

func TestHotelSearchURL_WithPropertyName(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{BookingComID: "book456"})
	u := b.HotelSearchURL("St. Regis Vommuli", "Maldives", "", "")

	// Property name should take precedence over city.
	if !strings.Contains(u, "ss=St.+Regis+Vommuli") {
		t.Errorf("expected property name in ss param, got: %s", u)
	}
	if strings.Contains(u, "ss=Maldives") {
		t.Errorf("city should not appear when property name is set: %s", u)
	}
}

func TestHotelSearchURL_PropertyNameEmptyFallsBackToCity(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{})
	u := b.HotelSearchURL("", "Lisbon", "", "")
	if !strings.Contains(u, "ss=Lisbon") {
		t.Errorf("expected city fallback in URL: %s", u)
	}
}

func TestActivityURL_WithPartnerID(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{GetYourGuideID: "gyg789"})
	u := b.ActivityURL("walking tour Prague")

	if !strings.Contains(u, "getyourguide.com/s/") {
		t.Errorf("unexpected URL structure: %s", u)
	}
	if !strings.Contains(u, "q=walking+tour+Prague") {
		t.Errorf("expected query in URL: %s", u)
	}
	if !strings.Contains(u, "partner_id=gyg789") {
		t.Errorf("expected partner ID in URL: %s", u)
	}
}

func TestActivityURL_WithoutPartnerID(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{})
	u := b.ActivityURL("cooking class Tokyo")

	if !strings.Contains(u, "getyourguide.com/s/") {
		t.Errorf("unexpected URL structure: %s", u)
	}
	if strings.Contains(u, "partner_id") {
		t.Errorf("should not contain partner_id when empty: %s", u)
	}
}

func TestViatorActivityURL_WithPartnerID(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{ViatorID: "vtr101"})
	u := b.ViatorActivityURL("food tour Rome")

	if !strings.Contains(u, "viator.com/search/") {
		t.Errorf("unexpected URL structure: %s", u)
	}
	if !strings.Contains(u, "pid=vtr101") {
		t.Errorf("expected partner ID in URL: %s", u)
	}
}

func TestViatorActivityURL_WithoutPartnerID(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{})
	u := b.ViatorActivityURL("snorkeling Bali")

	if !strings.Contains(u, "viator.com/search/") {
		t.Errorf("unexpected URL structure: %s", u)
	}
	if strings.Contains(u, "pid=") {
		t.Errorf("should not contain pid when empty: %s", u)
	}
}

func TestCarRentalURL_WithAffiliateID(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{DiscoverCarsID: "dc202"})
	u := b.CarRentalURL("Lisbon", "2026-07-01", "2026-07-10")

	if !strings.Contains(u, "discovercars.com") {
		t.Errorf("unexpected URL structure: %s", u)
	}
	if !strings.Contains(u, "location=Lisbon") {
		t.Errorf("expected location in URL: %s", u)
	}
	if !strings.Contains(u, "pickup_date=2026-07-01") {
		t.Errorf("expected pickup date in URL: %s", u)
	}
	if !strings.Contains(u, "dropoff_date=2026-07-10") {
		t.Errorf("expected dropoff date in URL: %s", u)
	}
	if !strings.Contains(u, "a_aid=dc202") {
		t.Errorf("expected affiliate ID in URL: %s", u)
	}
}

func TestCarRentalURL_WithoutAffiliateID(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{})
	u := b.CarRentalURL("Lisbon", "2026-07-01", "2026-07-10")

	if !strings.Contains(u, "discovercars.com") {
		t.Errorf("unexpected URL structure: %s", u)
	}
	if strings.Contains(u, "a_aid=") {
		t.Errorf("should not contain affiliate param when ID is empty: %s", u)
	}
}

func TestCarRentalURL_NoDates(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{DiscoverCarsID: "dc202"})
	u := b.CarRentalURL("Tokyo", "", "")

	if !strings.Contains(u, "location=Tokyo") {
		t.Errorf("expected location in URL: %s", u)
	}
	if strings.Contains(u, "pickup_date=") {
		t.Errorf("should not contain pickup_date when empty: %s", u)
	}
	if strings.Contains(u, "dropoff_date=") {
		t.Errorf("should not contain dropoff_date when empty: %s", u)
	}
}

func TestTravelInsuranceURL_WithReferenceID(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{SafetyWingID: "sw303"})
	u := b.TravelInsuranceURL("Japan")

	if !strings.Contains(u, "safetywing.com/nomad-insurance") {
		t.Errorf("unexpected URL structure: %s", u)
	}
	if !strings.Contains(u, "referenceID=sw303") {
		t.Errorf("expected reference ID in URL: %s", u)
	}
}

func TestTravelInsuranceURL_WithoutReferenceID(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{})
	u := b.TravelInsuranceURL("Japan")

	if !strings.Contains(u, "safetywing.com/nomad-insurance") {
		t.Errorf("unexpected URL structure: %s", u)
	}
	if strings.Contains(u, "referenceID") {
		t.Errorf("should not contain referenceID when empty: %s", u)
	}
}

func TestHasPartner(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{
		SkyscannerID:   "sky123",
		GetYourGuideID: "gyg789",
		DiscoverCarsID: "dc202",
	})

	tests := []struct {
		partner Partner
		want    bool
	}{
		{PartnerSkyscanner, true},
		{PartnerBookingCom, false},
		{PartnerGetYourGuide, true},
		{PartnerViator, false},
		{PartnerDiscoverCars, true},
		{PartnerSafetyWing, false},
		{PartnerGeneric, false},
	}

	for _, tt := range tests {
		got := b.HasPartner(tt.partner)
		if got != tt.want {
			t.Errorf("HasPartner(%q) = %v, want %v", tt.partner, got, tt.want)
		}
	}
}

func TestPartnerForCategory(t *testing.T) {
	tests := []struct {
		category string
		want     Partner
	}{
		{"flight", PartnerSkyscanner},
		{"hotel", PartnerBookingCom},
		{"activity", PartnerGetYourGuide},
		{"car_rental", PartnerDiscoverCars},
		{"insurance", PartnerSafetyWing},
		{"restaurant", PartnerGeneric},
		{"unknown", PartnerGeneric},
		{"", PartnerGeneric},
	}

	for _, tt := range tests {
		got := PartnerForCategory(tt.category)
		if got != tt.want {
			t.Errorf("PartnerForCategory(%q) = %q, want %q", tt.category, got, tt.want)
		}
	}
}

func TestFTCDisclosure(t *testing.T) {
	if FTCDisclosure == "" {
		t.Error("FTCDisclosure should not be empty")
	}
}

// --- HashTripID tests ---

func TestHashTripID_SHA256(t *testing.T) {
	tripID := "550e8400-e29b-41d4-a716-446655440000"
	got := HashTripID(tripID)

	// Verify length: 12 hex chars
	if len(got) != 12 {
		t.Errorf("HashTripID(%q) length = %d, want 12", tripID, len(got))
	}

	// Verify it matches our expected SHA-256 computation (first 6 bytes)
	h := sha256.Sum256([]byte(tripID))
	expected := hex.EncodeToString(h[:6])
	if got != expected {
		t.Errorf("HashTripID(%q) = %q, want %q", tripID, got, expected)
	}
}

func TestHashTripID_Empty(t *testing.T) {
	got := HashTripID("")
	if got != "" {
		t.Errorf("HashTripID(\"\") = %q, want empty string", got)
	}
}

func TestHashTripID_Deterministic(t *testing.T) {
	id := "some-trip-uuid-12345"
	first := HashTripID(id)
	for i := 0; i < 100; i++ {
		if HashTripID(id) != first {
			t.Fatal("HashTripID is not deterministic")
		}
	}
}

func TestHashTripID_DifferentInputsDifferentOutputs(t *testing.T) {
	ids := []string{
		"trip-1",
		"trip-2",
		"550e8400-e29b-41d4-a716-446655440000",
		"660e8400-e29b-41d4-a716-446655440000",
	}
	seen := make(map[string]string)
	for _, id := range ids {
		h := HashTripID(id)
		if prev, ok := seen[h]; ok {
			t.Errorf("collision: HashTripID(%q) == HashTripID(%q) == %q", id, prev, h)
		}
		seen[h] = id
	}
}

// --- Sub-ID parameter tests ---

func TestFlightSearchURL_WithSubID(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{SkyscannerID: "sky123"})
	u := b.FlightSearchURL("JFK", "PRG", "2026-06-15", "abc123def456")

	if !strings.Contains(u, "associateid=sky123") {
		t.Errorf("expected affiliate ID in URL: %s", u)
	}
	if !strings.Contains(u, "utm_content=abc123def456") {
		t.Errorf("expected utm_content sub-ID in URL: %s", u)
	}
}

func TestFlightSearchURL_SubIDOmittedWithoutAffiliateID(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{})
	u := b.FlightSearchURL("JFK", "PRG", "2026-06-15", "abc123def456")

	if strings.Contains(u, "utm_content") {
		t.Errorf("should not contain utm_content when affiliate ID is empty: %s", u)
	}
}

func TestFlightSearchURL_EmptySubID(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{SkyscannerID: "sky123"})
	u := b.FlightSearchURL("JFK", "PRG", "2026-06-15", "")

	if strings.Contains(u, "utm_content") {
		t.Errorf("should not contain utm_content when sub-ID is empty: %s", u)
	}
}

func TestFlightSearchURL_NoSubIDArg(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{SkyscannerID: "sky123"})
	u := b.FlightSearchURL("JFK", "PRG", "2026-06-15")

	if strings.Contains(u, "utm_content") {
		t.Errorf("should not contain utm_content when no sub-ID arg: %s", u)
	}
	if !strings.Contains(u, "associateid=sky123") {
		t.Errorf("expected affiliate ID in URL: %s", u)
	}
}

func TestHotelSearchURL_WithSubID(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{BookingComID: "book456"})
	u := b.HotelSearchURL("", "Prague", "2026-06-15", "2026-06-20", "abc123def456")

	if !strings.Contains(u, "aid=book456") {
		t.Errorf("expected affiliate ID in URL: %s", u)
	}
	if !strings.Contains(u, "label=abc123def456") {
		t.Errorf("expected label sub-ID in URL: %s", u)
	}
}

func TestHotelSearchURL_SubIDOmittedWithoutAffiliateID(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{})
	u := b.HotelSearchURL("", "Prague", "2026-06-15", "2026-06-20", "abc123def456")

	if strings.Contains(u, "label=") {
		t.Errorf("should not contain label when affiliate ID is empty: %s", u)
	}
}

func TestActivityURL_WithSubID(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{GetYourGuideID: "gyg789"})
	u := b.ActivityURL("walking tour Prague", "abc123def456")

	if !strings.Contains(u, "partner_id=gyg789") {
		t.Errorf("expected partner ID in URL: %s", u)
	}
	if !strings.Contains(u, "cmp=abc123def456") {
		t.Errorf("expected cmp sub-ID in URL: %s", u)
	}
}

func TestActivityURL_SubIDOmittedWithoutPartnerID(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{})
	u := b.ActivityURL("walking tour Prague", "abc123def456")

	if strings.Contains(u, "cmp=") {
		t.Errorf("should not contain cmp when partner ID is empty: %s", u)
	}
}

func TestViatorActivityURL_WithSubID(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{ViatorID: "vtr101"})
	u := b.ViatorActivityURL("food tour Rome", "abc123def456")

	if !strings.Contains(u, "pid=vtr101") {
		t.Errorf("expected partner ID in URL: %s", u)
	}
	if !strings.Contains(u, "cmp=abc123def456") {
		t.Errorf("expected cmp sub-ID in URL: %s", u)
	}
}

func TestViatorActivityURL_SubIDOmittedWithoutPartnerID(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{})
	u := b.ViatorActivityURL("food tour Rome", "abc123def456")

	if strings.Contains(u, "cmp=") {
		t.Errorf("should not contain cmp when partner ID is empty: %s", u)
	}
}

func TestVacationRentalURL_WithPublisherID(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{ExpediaPublisherID: "1011l428984"})
	u := b.VacationRentalURL("Kyoto", "2026-06-10", "2026-06-17", "trip123")

	// Wrapped in Partnerize
	if !strings.HasPrefix(u, "https://prf.hn/click/camref:1011l428984/") {
		t.Errorf("expected Partnerize prefix with camref, got: %s", u)
	}
	// pubref (subID) is present
	if !strings.Contains(u, "/pubref:trip123/") {
		t.Errorf("expected /pubref:trip123/ segment, got: %s", u)
	}
	// Destination URL is percent-encoded inside the outer link
	if !strings.Contains(u, "destination:https%3A%2F%2Fwww.vrbo.com%2Fsearch") {
		t.Errorf("expected encoded VRBO destination, got: %s", u)
	}
	// The encoded destination preserves the search params
	if !strings.Contains(u, "q%3DKyoto") {
		t.Errorf("expected encoded q=Kyoto in destination, got: %s", u)
	}
	if !strings.Contains(u, "d1%3D2026-06-10") {
		t.Errorf("expected encoded d1=checkin in destination, got: %s", u)
	}
}

func TestVacationRentalURL_WithoutPublisherID_FallsThrough(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{})
	u := b.VacationRentalURL("Kyoto", "2026-06-10", "2026-06-17", "trip123")

	// No publisher ID → return the raw VRBO URL unwrapped.
	if !strings.HasPrefix(u, "https://www.vrbo.com/search") {
		t.Errorf("expected unwrapped VRBO URL, got: %s", u)
	}
	if strings.Contains(u, "prf.hn") {
		t.Errorf("should not wrap in prf.hn when publisher ID is empty: %s", u)
	}
}

func TestVacationRentalURL_OmitsPubrefWhenNoSubID(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{ExpediaPublisherID: "1011l428984"})
	u := b.VacationRentalURL("Kyoto", "", "", "")

	if strings.Contains(u, "/pubref:") {
		t.Errorf("should not include /pubref: segment when tripIDHash is empty: %s", u)
	}
}

func TestVacationRentalURL_NonASCIICity(t *testing.T) {
	// Non-ASCII city names (Japanese, Arabic, Cyrillic) must survive
	// the double-encoding round-trip: VRBO's q param gets URL-encoded
	// into its own query string, then the whole destination URL gets
	// QueryEscape'd again when wrapped by Partnerize. If either layer
	// corrupts the bytes (e.g. PathEscape leaving %E6 intact but
	// eating the %), the Partnerize tracker hands VRBO a broken
	// search and the user lands on an empty page.
	b := NewLinkBuilder(LinkBuilderConfig{ExpediaPublisherID: "1011l428984"})
	u := b.VacationRentalURL("東京", "2026-08-01", "2026-08-07", "trip-ja")

	// The Tokyo bytes (東京) percent-encode to %E6%9D%B1%E4%BA%AC. After
	// the outer QueryEscape those %-signs become %25, so we expect the
	// double-encoded sequence in the final URL.
	if !strings.Contains(u, "%25E6%259D%25B1%25E4%25BA%25AC") {
		t.Errorf("expected double-encoded Tokyo bytes, got: %s", u)
	}
	// Verify round-trip: decode twice and we should recover the
	// destination URL with the Japanese chars in the q param.
	decoded1, err := url.QueryUnescape(strings.TrimPrefix(u, "https://prf.hn/click/camref:1011l428984/pubref:trip-ja/destination:"))
	if err != nil {
		t.Fatalf("outer QueryUnescape: %v", err)
	}
	if !strings.Contains(decoded1, "q=%E6%9D%B1%E4%BA%AC") {
		t.Errorf("outer decode should reveal VRBO URL with encoded Tokyo, got: %s", decoded1)
	}
	parsed, err := url.Parse(decoded1)
	if err != nil {
		t.Fatalf("parse destination: %v", err)
	}
	if got := parsed.Query().Get("q"); got != "東京" {
		t.Errorf("round-tripped q param: want %q, got %q", "東京", got)
	}
}

func TestVacationRentalSources_EmptyCityProducesCleanTitle(t *testing.T) {
	// Earlier revision produced "Vacation rentals in  (VRBO)" — two
	// spaces — when city was empty. Adversarial review W5. Titles
	// should degrade to a city-less form without a double-space.
	b := NewLinkBuilder(LinkBuilderConfig{ExpediaPublisherID: "1011l428984"})
	sources := b.VacationRentalSources("", "", "", "", false)
	for _, s := range sources {
		if strings.Contains(s.Title, "  ") {
			t.Errorf("source title contains double-space for empty city: %q (id=%s)", s.Title, s.ID)
		}
	}
}

func TestExpediaHotelURL_WithPublisherID(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{ExpediaPublisherID: "1011l428984"})
	u := b.ExpediaHotelURL("Lisbon", "2026-09-01", "2026-09-05", "trip-abc")

	if !strings.HasPrefix(u, "https://prf.hn/click/camref:1011l428984/") {
		t.Errorf("expected Partnerize prefix, got: %s", u)
	}
	if !strings.Contains(u, "destination:https%3A%2F%2Fwww.expedia.com%2FHotel-Search") {
		t.Errorf("expected encoded Expedia destination, got: %s", u)
	}
	if !strings.Contains(u, "destination%3DLisbon") {
		t.Errorf("expected encoded destination=Lisbon, got: %s", u)
	}
}

func TestHasPartner_ExpediaGroup(t *testing.T) {
	withID := NewLinkBuilder(LinkBuilderConfig{ExpediaPublisherID: "1011l428984"})
	if !withID.HasPartner(PartnerExpedia) {
		t.Error("expected HasPartner(Expedia) to be true when ID configured")
	}
	if !withID.HasPartner(PartnerVRBO) {
		t.Error("expected HasPartner(VRBO) to be true when ID configured — both share one camref")
	}

	without := NewLinkBuilder(LinkBuilderConfig{})
	if without.HasPartner(PartnerExpedia) || without.HasPartner(PartnerVRBO) {
		t.Error("expected HasPartner(Expedia/VRBO) to be false when no ID configured")
	}
}

func TestPartnerForCategory_VacationRental(t *testing.T) {
	if got := PartnerForCategory("vacation_rental"); got != PartnerVRBO {
		t.Errorf("PartnerForCategory(vacation_rental) = %v, want %v", got, PartnerVRBO)
	}
	// Sanity: the hotel category is unchanged by this PR.
	if got := PartnerForCategory("hotel"); got != PartnerBookingCom {
		t.Errorf("PartnerForCategory(hotel) = %v, want %v (unchanged)", got, PartnerBookingCom)
	}
}
