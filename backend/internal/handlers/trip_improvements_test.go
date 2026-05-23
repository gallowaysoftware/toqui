package handlers

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
)

// TestTripToProto_NotesField verifies that the notes field round-trips
// through tripToProto correctly.
func TestTripToProto_NotesField(t *testing.T) {
	trip := newTestTrip()
	trip.Notes = pgtype.Text{String: "Remember to book airport transfer", Valid: true}

	proto := tripToProto(trip)
	if proto.Notes != "Remember to book airport transfer" {
		t.Errorf("Notes = %q, want %q", proto.Notes, "Remember to book airport transfer")
	}
}

func TestTripToProto_NotesEmpty(t *testing.T) {
	trip := newTestTrip()
	// Notes not set (default zero value)

	proto := tripToProto(trip)
	if proto.Notes != "" {
		t.Errorf("Notes = %q, want empty", proto.Notes)
	}
}

// TestTripToProto_CoverImageURL verifies cover image URL field.
func TestTripToProto_CoverImageURL(t *testing.T) {
	trip := newTestTrip()
	trip.CoverImageUrl = pgtype.Text{String: "https://images.toqui.travel/greece.jpg", Valid: true}

	proto := tripToProto(trip)
	if proto.CoverImageUrl != "https://images.toqui.travel/greece.jpg" {
		t.Errorf("CoverImageUrl = %q, want %q", proto.CoverImageUrl, "https://images.toqui.travel/greece.jpg")
	}
}

func TestTripToProto_CoverImageURLEmpty(t *testing.T) {
	trip := newTestTrip()

	proto := tripToProto(trip)
	if proto.CoverImageUrl != "" {
		t.Errorf("CoverImageUrl = %q, want empty", proto.CoverImageUrl)
	}
}

// TestTripToProto_Timezone verifies timezone field.
func TestTripToProto_Timezone(t *testing.T) {
	trip := newTestTrip()
	trip.Timezone = pgtype.Text{String: "Europe/Athens", Valid: true}

	proto := tripToProto(trip)
	if proto.Timezone != "Europe/Athens" {
		t.Errorf("Timezone = %q, want %q", proto.Timezone, "Europe/Athens")
	}
}

func TestTripToProto_TimezoneEmpty(t *testing.T) {
	trip := newTestTrip()

	proto := tripToProto(trip)
	if proto.Timezone != "" {
		t.Errorf("Timezone = %q, want empty", proto.Timezone)
	}
}

// TestTripToProto_IsTemplate verifies the template flag.
func TestTripToProto_IsTemplate(t *testing.T) {
	trip := newTestTrip()
	trip.IsTemplate = true

	proto := tripToProto(trip)
	if !proto.IsTemplate {
		t.Error("IsTemplate = false, want true")
	}
}

func TestTripToProto_NotTemplate(t *testing.T) {
	trip := newTestTrip()
	trip.IsTemplate = false

	proto := tripToProto(trip)
	if proto.IsTemplate {
		t.Error("IsTemplate = true, want false")
	}
}

// TestTripToProto_AllNewFields verifies all new fields together.
func TestTripToProto_AllNewFields(t *testing.T) {
	trip := newTestTrip()
	trip.Notes = pgtype.Text{String: "Pack sunscreen", Valid: true}
	trip.CoverImageUrl = pgtype.Text{String: "https://images.toqui.travel/santorini.jpg", Valid: true}
	trip.Timezone = pgtype.Text{String: "Europe/Athens", Valid: true}
	trip.IsTemplate = true
	trip.BudgetCents = pgtype.Int8{Int64: 300000, Valid: true}
	trip.Currency = pgtype.Text{String: "EUR", Valid: true}

	proto := tripToProto(trip)

	if proto.Notes != "Pack sunscreen" {
		t.Errorf("Notes = %q, want %q", proto.Notes, "Pack sunscreen")
	}
	if proto.CoverImageUrl != "https://images.toqui.travel/santorini.jpg" {
		t.Errorf("CoverImageUrl = %q, want %q", proto.CoverImageUrl, "https://images.toqui.travel/santorini.jpg")
	}
	if proto.Timezone != "Europe/Athens" {
		t.Errorf("Timezone = %q, want %q", proto.Timezone, "Europe/Athens")
	}
	if !proto.IsTemplate {
		t.Error("IsTemplate = false, want true")
	}
	if proto.BudgetCents == nil || *proto.BudgetCents != 300000 {
		t.Errorf("BudgetCents = %v, want 300000", proto.BudgetCents)
	}
	if proto.Currency != "EUR" {
		t.Errorf("Currency = %q, want %q", proto.Currency, "EUR")
	}
}

// TestTripToProtoWithThemes_NewFields verifies that tripToProtoWithThemes
// also carries the new fields (since it delegates to tripToProto).
func TestTripToProtoWithThemes_NewFields(t *testing.T) {
	trip := newTestTrip()
	trip.Notes = pgtype.Text{String: "Bring adapter", Valid: true}
	trip.Timezone = pgtype.Text{String: "Asia/Tokyo", Valid: true}
	trip.CoverImageUrl = pgtype.Text{String: "https://img.example.com/jp.jpg", Valid: true}

	themes := []string{"food", "history"}
	proto := tripToProtoWithThemes(trip, themes)

	if proto.Notes != "Bring adapter" {
		t.Errorf("Notes = %q, want %q", proto.Notes, "Bring adapter")
	}
	if proto.Timezone != "Asia/Tokyo" {
		t.Errorf("Timezone = %q, want %q", proto.Timezone, "Asia/Tokyo")
	}
	if proto.CoverImageUrl != "https://img.example.com/jp.jpg" {
		t.Errorf("CoverImageUrl = %q, want %q", proto.CoverImageUrl, "https://img.example.com/jp.jpg")
	}
	if len(proto.Themes) != 2 {
		t.Errorf("Themes length = %d, want 2", len(proto.Themes))
	}
}

// TestItineraryItemToProto_AllFields verifies that the itinerary item
// conversion used by the ReorderItineraryItem RPC handles all fields.
func TestItineraryItemToProto_AllFields(t *testing.T) {
	cost := int64(2500)
	items := []dbgen.ItineraryItem{
		{
			DayNumber:          pgtype.Int4{Int32: 2, Valid: true},
			OrderInDay:         pgtype.Int4{Int32: 3, Valid: true},
			Title:              pgtype.Text{String: "Acropolis Tour", Valid: true},
			Type:               pgtype.Text{String: "sightseeing", Valid: true},
			Description:        pgtype.Text{String: "Guided tour of the Acropolis", Valid: true},
			EstimatedCostCents: pgtype.Int8{Int64: cost, Valid: true},
			CostCurrency:       pgtype.Text{String: "EUR", Valid: true},
		},
	}

	itin := itineraryToProto("test-trip", items, nil)
	if len(itin.Days) != 1 {
		t.Fatalf("expected 1 day, got %d", len(itin.Days))
	}
	if len(itin.Days[0].Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(itin.Days[0].Items))
	}

	item := itin.Days[0].Items[0]
	if item.Title != "Acropolis Tour" {
		t.Errorf("Title = %q, want %q", item.Title, "Acropolis Tour")
	}
	if item.OrderInDay != 3 {
		t.Errorf("OrderInDay = %d, want 3", item.OrderInDay)
	}
	if item.EstimatedCostCents == nil || *item.EstimatedCostCents != 2500 {
		t.Errorf("EstimatedCostCents = %v, want 2500", item.EstimatedCostCents)
	}
}
