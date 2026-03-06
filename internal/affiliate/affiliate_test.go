package affiliate

import (
	"strings"
	"testing"
)

func TestNewLinkBuilder(t *testing.T) {
	b := NewLinkBuilder("sky123", "book456", "gyg789")
	if b.skyscannerID != "sky123" {
		t.Errorf("expected skyscannerID %q, got %q", "sky123", b.skyscannerID)
	}
	if b.bookingComID != "book456" {
		t.Errorf("expected bookingComID %q, got %q", "book456", b.bookingComID)
	}
	if b.getYourGuideID != "gyg789" {
		t.Errorf("expected getYourGuideID %q, got %q", "gyg789", b.getYourGuideID)
	}
}

func TestFlightSearchURL_WithAffiliateID(t *testing.T) {
	b := NewLinkBuilder("sky123", "", "")
	u := b.FlightSearchURL("JFK", "PRG", "2026-06-15")

	if !strings.Contains(u, "skyscanner.com/transport/flights/JFK/PRG/2026-06-15") {
		t.Errorf("unexpected URL structure: %s", u)
	}
	if !strings.Contains(u, "associateid=sky123") {
		t.Errorf("expected affiliate ID in URL: %s", u)
	}
}

func TestFlightSearchURL_WithoutAffiliateID(t *testing.T) {
	b := NewLinkBuilder("", "", "")
	u := b.FlightSearchURL("JFK", "PRG", "2026-06-15")

	if !strings.Contains(u, "skyscanner.com/transport/flights/JFK/PRG/2026-06-15") {
		t.Errorf("unexpected URL structure: %s", u)
	}
	if strings.Contains(u, "associateid") {
		t.Errorf("should not contain affiliate param when ID is empty: %s", u)
	}
}

func TestHotelSearchURL_WithAffiliateID(t *testing.T) {
	b := NewLinkBuilder("", "book456", "")
	u := b.HotelSearchURL("Prague", "2026-06-15", "2026-06-20")

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
	b := NewLinkBuilder("", "", "")
	u := b.HotelSearchURL("Prague", "2026-06-15", "2026-06-20")

	if !strings.Contains(u, "booking.com/searchresults.html") {
		t.Errorf("unexpected URL structure: %s", u)
	}
	if strings.Contains(u, "aid=") {
		t.Errorf("should not contain affiliate param when ID is empty: %s", u)
	}
}

func TestHotelSearchURL_NoDates(t *testing.T) {
	b := NewLinkBuilder("", "book456", "")
	u := b.HotelSearchURL("Tokyo", "", "")

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

func TestActivityURL_WithPartnerID(t *testing.T) {
	b := NewLinkBuilder("", "", "gyg789")
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
	b := NewLinkBuilder("", "", "")
	u := b.ActivityURL("cooking class Tokyo")

	if !strings.Contains(u, "getyourguide.com/s/") {
		t.Errorf("unexpected URL structure: %s", u)
	}
	if strings.Contains(u, "partner_id") {
		t.Errorf("should not contain partner_id when empty: %s", u)
	}
}

func TestHasPartner(t *testing.T) {
	b := NewLinkBuilder("sky123", "", "gyg789")

	tests := []struct {
		partner Partner
		want    bool
	}{
		{PartnerSkyscanner, true},
		{PartnerBookingCom, false},
		{PartnerGetYourGuide, true},
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
