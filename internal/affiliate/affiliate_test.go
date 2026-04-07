package affiliate

import (
	"strings"
	"testing"
)

func TestNewLinkBuilder(t *testing.T) {
	b := NewLinkBuilder(LinkBuilderConfig{
		SkyscannerID:   "sky123",
		BookingComID:   "book456",
		GetYourGuideID: "gyg789",
		ViatorID:       "vtr101",
		DiscoverCarsID: "dc202",
		SafetyWingID:   "sw303",
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
