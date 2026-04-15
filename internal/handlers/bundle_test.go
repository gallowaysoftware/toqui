package handlers

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui-backend/internal/trip"
)

func TestParseTripIDFromBundlePath(t *testing.T) {
	validID := uuid.New()
	tests := []struct {
		name   string
		path   string
		wantID uuid.UUID
		wantOK bool
	}{
		{
			name:   "valid path",
			path:   "/api/trips/" + validID.String() + "/bundle",
			wantID: validID,
			wantOK: true,
		},
		{
			name:   "valid path with trailing slash",
			path:   "/api/trips/" + validID.String() + "/bundle/",
			wantID: validID,
			wantOK: true,
		},
		{
			name:   "wrong suffix",
			path:   "/api/trips/" + validID.String() + "/export/ical",
			wantID: uuid.Nil,
			wantOK: false,
		},
		{
			name:   "missing trip ID",
			path:   "/api/trips//bundle",
			wantID: uuid.Nil,
			wantOK: false,
		},
		{
			name:   "invalid UUID",
			path:   "/api/trips/not-a-uuid/bundle",
			wantID: uuid.Nil,
			wantOK: false,
		},
		{
			name:   "wrong prefix",
			path:   "/other/trips/" + validID.String() + "/bundle",
			wantID: uuid.Nil,
			wantOK: false,
		},
		{
			name:   "too short",
			path:   "/api/trips",
			wantID: uuid.Nil,
			wantOK: false,
		},
		{
			name:   "empty",
			path:   "",
			wantID: uuid.Nil,
			wantOK: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotID, gotOK := parseTripIDFromBundlePath(tc.path)
			if gotOK != tc.wantOK {
				t.Errorf("parseTripIDFromBundlePath(%q) ok = %v, want %v", tc.path, gotOK, tc.wantOK)
			}
			if gotID != tc.wantID {
				t.Errorf("parseTripIDFromBundlePath(%q) id = %v, want %v", tc.path, gotID, tc.wantID)
			}
		})
	}
}

func TestBuildBundleTripInfo(t *testing.T) {
	tripID := uuid.New()
	userID := uuid.New()
	now := time.Now()
	budget := int64(250000)

	dbTrip := &dbgen.Trip{
		ID:                   tripID,
		UserID:               userID,
		Title:                "Greece Adventure",
		Status:               "active",
		Description:          pgtype.Text{String: "Island hopping in the Aegean", Valid: true},
		StartDate:            pgtype.Date{Time: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), Valid: true},
		EndDate:              pgtype.Date{Time: time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC), Valid: true},
		DestinationCountry:   pgtype.Text{String: "GR", Valid: true},
		DestinationCountries: []string{"GR", "TR"},
		BudgetCents:          pgtype.Int8{Int64: budget, Valid: true},
		Currency:             pgtype.Text{String: "EUR", Valid: true},
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	themes := []string{"romance", "food"}

	info := buildBundleTripInfo(dbTrip, themes)

	if info.ID != tripID.String() {
		t.Errorf("ID = %q, want %q", info.ID, tripID.String())
	}
	if info.Title != "Greece Adventure" {
		t.Errorf("Title = %q, want %q", info.Title, "Greece Adventure")
	}
	if info.Status != "active" {
		t.Errorf("Status = %q, want %q", info.Status, "active")
	}
	if info.Description != "Island hopping in the Aegean" {
		t.Errorf("Description = %q, want %q", info.Description, "Island hopping in the Aegean")
	}
	if info.StartDate != "2026-06-01" {
		t.Errorf("StartDate = %q, want %q", info.StartDate, "2026-06-01")
	}
	if info.EndDate != "2026-06-14" {
		t.Errorf("EndDate = %q, want %q", info.EndDate, "2026-06-14")
	}
	if info.DestinationCountry != "GR" {
		t.Errorf("DestinationCountry = %q, want %q", info.DestinationCountry, "GR")
	}
	if len(info.DestinationCountries) != 2 || info.DestinationCountries[0] != "GR" || info.DestinationCountries[1] != "TR" {
		t.Errorf("DestinationCountries = %v, want [GR TR]", info.DestinationCountries)
	}
	if len(info.Themes) != 2 || info.Themes[0] != "romance" {
		t.Errorf("Themes = %v, want [romance food]", info.Themes)
	}
	if info.BudgetCents == nil || *info.BudgetCents != budget {
		t.Errorf("BudgetCents = %v, want %d", info.BudgetCents, budget)
	}
	if info.Currency != "EUR" {
		t.Errorf("Currency = %q, want %q", info.Currency, "EUR")
	}
}

func TestBuildBundleTripInfo_Minimal(t *testing.T) {
	dbTrip := &dbgen.Trip{
		ID:     uuid.New(),
		UserID: uuid.New(),
		Title:  "Quick Trip",
		Status: "planning",
	}

	info := buildBundleTripInfo(dbTrip, nil)

	if info.Title != "Quick Trip" {
		t.Errorf("Title = %q, want %q", info.Title, "Quick Trip")
	}
	if info.Description != "" {
		t.Errorf("Description should be empty, got %q", info.Description)
	}
	if info.StartDate != "" {
		t.Errorf("StartDate should be empty, got %q", info.StartDate)
	}
	if info.BudgetCents != nil {
		t.Errorf("BudgetCents should be nil, got %v", info.BudgetCents)
	}
}

func TestBuildBundleItinerary(t *testing.T) {
	itemID1 := uuid.New()
	itemID2 := uuid.New()
	itemID3 := uuid.New()

	// Build metadata with day_summary and day_date.
	md1, _ := json.Marshal(map[string]string{
		"day_summary": "Arrive in Athens",
		"day_date":    "2026-06-01",
	})
	md2, _ := json.Marshal(map[string]string{
		"day_summary": "Santorini",
		"day_date":    "2026-06-02",
	})

	startTime := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	endTime := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

	items := []dbgen.ItineraryItem{
		{
			ID:                 itemID1,
			DayNumber:          pgtype.Int4{Int32: 1, Valid: true},
			OrderInDay:         pgtype.Int4{Int32: 1, Valid: true},
			Title:              pgtype.Text{String: "Visit Acropolis", Valid: true},
			Type:               pgtype.Text{String: "activity", Valid: true},
			Description:        pgtype.Text{String: "Ancient ruins tour", Valid: true},
			StartTime:          pgtype.Timestamptz{Time: startTime, Valid: true},
			EndTime:            pgtype.Timestamptz{Time: endTime, Valid: true},
			Metadata:           md1,
			EstimatedCostCents: pgtype.Int8{Int64: 2000, Valid: true},
			CostCurrency:       pgtype.Text{String: "EUR", Valid: true},
		},
		{
			ID:         itemID2,
			DayNumber:  pgtype.Int4{Int32: 1, Valid: true},
			OrderInDay: pgtype.Int4{Int32: 2, Valid: true},
			Title:      pgtype.Text{String: "Lunch at Plaka", Valid: true},
			Type:       pgtype.Text{String: "food", Valid: true},
			Metadata:   md1,
		},
		{
			ID:         itemID3,
			DayNumber:  pgtype.Int4{Int32: 2, Valid: true},
			OrderInDay: pgtype.Int4{Int32: 1, Valid: true},
			Title:      pgtype.Text{String: "Ferry to Santorini", Valid: true},
			Type:       pgtype.Text{String: "transport", Valid: true},
			Metadata:   md2,
		},
	}

	coordsMap := map[uuid.UUID]trip.ItineraryItemCoords{
		itemID1: {ID: itemID1, Latitude: 37.9715, Longitude: 23.7267},
	}

	days := buildBundleItinerary(items, coordsMap)

	if len(days) != 2 {
		t.Fatalf("expected 2 days, got %d", len(days))
	}

	// Day 1
	day1 := days[0]
	if day1.DayNumber != 1 {
		t.Errorf("day1.DayNumber = %d, want 1", day1.DayNumber)
	}
	if day1.Summary != "Arrive in Athens" {
		t.Errorf("day1.Summary = %q, want %q", day1.Summary, "Arrive in Athens")
	}
	if day1.Date != "2026-06-01" {
		t.Errorf("day1.Date = %q, want %q", day1.Date, "2026-06-01")
	}
	if len(day1.Items) != 2 {
		t.Fatalf("day1 should have 2 items, got %d", len(day1.Items))
	}

	// First item has coordinates
	if day1.Items[0].Latitude != 37.9715 {
		t.Errorf("item1.Latitude = %f, want 37.9715", day1.Items[0].Latitude)
	}
	if day1.Items[0].EstimatedCostCents == nil || *day1.Items[0].EstimatedCostCents != 2000 {
		t.Errorf("item1.EstimatedCostCents = %v, want 2000", day1.Items[0].EstimatedCostCents)
	}
	if day1.Items[0].CostCurrency != "EUR" {
		t.Errorf("item1.CostCurrency = %q, want %q", day1.Items[0].CostCurrency, "EUR")
	}

	// Day 2
	day2 := days[1]
	if day2.DayNumber != 2 {
		t.Errorf("day2.DayNumber = %d, want 2", day2.DayNumber)
	}
	if len(day2.Items) != 1 {
		t.Fatalf("day2 should have 1 item, got %d", len(day2.Items))
	}
}

func TestBuildBundleItinerary_Empty(t *testing.T) {
	days := buildBundleItinerary(nil, nil)
	if len(days) != 0 {
		t.Errorf("expected 0 days for nil items, got %d", len(days))
	}
}

func TestBuildBundleBookings(t *testing.T) {
	bookingID := uuid.New()
	start := time.Date(2026, 6, 1, 14, 30, 0, 0, time.UTC)
	end := time.Date(2026, 6, 1, 17, 0, 0, 0, time.UTC)

	bookings := []dbgen.Booking{
		{
			ID:               bookingID,
			Type:             "flight",
			Title:            "Athens to Santorini",
			Provider:         pgtype.Text{String: "Aegean Airlines", Valid: true},
			ConfirmationCode: pgtype.Text{String: "ABC123", Valid: true},
			StartTime:        pgtype.Timestamptz{Time: start, Valid: true},
			EndTime:          pgtype.Timestamptz{Time: end, Valid: true},
			Address:          pgtype.Text{String: "Athens International Airport", Valid: true},
			DetailsJson:      []byte(`{"flight_number":"A3 354"}`),
		},
		{
			ID:    uuid.New(),
			Type:  "hotel",
			Title: "Santorini Sunset Hotel",
		},
	}

	result := buildBundleBookings(bookings)

	if len(result) != 2 {
		t.Fatalf("expected 2 bookings, got %d", len(result))
	}

	b1 := result[0]
	if b1.ID != bookingID.String() {
		t.Errorf("b1.ID = %q, want %q", b1.ID, bookingID.String())
	}
	if b1.Type != "flight" {
		t.Errorf("b1.Type = %q, want %q", b1.Type, "flight")
	}
	if b1.ConfirmationCode != "ABC123" {
		t.Errorf("b1.ConfirmationCode = %q, want %q", b1.ConfirmationCode, "ABC123")
	}
	if b1.Provider != "Aegean Airlines" {
		t.Errorf("b1.Provider = %q, want %q", b1.Provider, "Aegean Airlines")
	}
	if b1.StartTime != "2026-06-01T14:30:00Z" {
		t.Errorf("b1.StartTime = %q, want %q", b1.StartTime, "2026-06-01T14:30:00Z")
	}
	if b1.DetailsJSON != `{"flight_number":"A3 354"}` {
		t.Errorf("b1.DetailsJSON = %q", b1.DetailsJSON)
	}

	// Minimal booking
	b2 := result[1]
	if b2.Title != "Santorini Sunset Hotel" {
		t.Errorf("b2.Title = %q, want %q", b2.Title, "Santorini Sunset Hotel")
	}
	if b2.ConfirmationCode != "" {
		t.Errorf("b2.ConfirmationCode should be empty, got %q", b2.ConfirmationCode)
	}
}

func TestBuildBundleBookings_Empty(t *testing.T) {
	result := buildBundleBookings(nil)
	if len(result) != 0 {
		t.Errorf("expected 0 bookings for nil input, got %d", len(result))
	}
}

func TestMatchGuides(t *testing.T) {
	handler := &BundleHandler{
		guides: NewGuidesHandler("https://app.toqui.travel"),
	}

	// Japan should match tokyo-food and japan-adventure
	guides := handler.matchGuides([]string{"JP"})
	if len(guides) != 2 {
		t.Fatalf("expected 2 guides for JP, got %d", len(guides))
	}
	slugs := make(map[string]bool)
	for _, g := range guides {
		slugs[g.Slug] = true
	}
	if !slugs["tokyo-food"] {
		t.Error("expected tokyo-food guide for JP")
	}
	if !slugs["japan-adventure"] {
		t.Error("expected japan-adventure guide for JP")
	}

	// Multi-country
	guides = handler.matchGuides([]string{"GR", "TR"})
	slugs = make(map[string]bool)
	for _, g := range guides {
		slugs[g.Slug] = true
	}
	if !slugs["greece-romance"] {
		t.Error("expected greece-romance guide for GR")
	}
	if !slugs["istanbul-culture"] {
		t.Error("expected istanbul-culture guide for TR")
	}

	// No country
	guides = handler.matchGuides(nil)
	if len(guides) != 0 {
		t.Errorf("expected 0 guides for nil countries, got %d", len(guides))
	}

	// Unknown country
	guides = handler.matchGuides([]string{"XX"})
	if len(guides) != 0 {
		t.Errorf("expected 0 guides for unknown country XX, got %d", len(guides))
	}
}

func TestMatchGuides_CaseInsensitive(t *testing.T) {
	handler := &BundleHandler{
		guides: NewGuidesHandler("https://app.toqui.travel"),
	}

	// Lowercase country code should still match
	guides := handler.matchGuides([]string{"jp"})
	if len(guides) != 2 {
		t.Errorf("expected 2 guides for lowercase jp, got %d", len(guides))
	}
}

func TestBundleResponseJSON(t *testing.T) {
	budget := int64(100000)
	resp := bundleResponse{
		BundleVersion: "2026-06-01T00:00:00Z",
		Modified:      true,
		Trip: &bundleTripInfo{
			ID:          uuid.New().String(),
			Title:       "Test Trip",
			Status:      "active",
			BudgetCents: &budget,
			Currency:    "USD",
		},
		Itinerary: []bundleDay{
			{DayNumber: 1, Items: []bundleDayItem{{ID: uuid.New().String(), Title: "Item 1"}}},
		},
		Bookings:     []bundleBooking{},
		ChatMessages: []bundleChatMessage{},
		Guides:       []bundleGuide{},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal bundle response: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal bundle response: %v", err)
	}

	if decoded["bundle_version"] != "2026-06-01T00:00:00Z" {
		t.Errorf("bundle_version = %v", decoded["bundle_version"])
	}
	if decoded["modified"] != true {
		t.Errorf("modified = %v", decoded["modified"])
	}
	if decoded["trip"] == nil {
		t.Error("trip should not be nil")
	}
}
