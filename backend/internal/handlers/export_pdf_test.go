package handlers

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
)

func TestGeneratePDF_StartsWithMagic(t *testing.T) {
	trip := &dbgen.Trip{
		ID:    uuid.New(),
		Title: "Test Trip",
	}

	pdfBytes, err := GeneratePDF(trip, nil, nil)
	if err != nil {
		t.Fatalf("GeneratePDF failed: %v", err)
	}

	if len(pdfBytes) < 5 {
		t.Fatal("PDF output too short")
	}
	if string(pdfBytes[:5]) != "%PDF-" {
		t.Errorf("expected PDF to start with %%PDF-, got %q", string(pdfBytes[:5]))
	}
}

func TestGeneratePDF_EmptyItinerary(t *testing.T) {
	trip := &dbgen.Trip{
		ID:    uuid.New(),
		Title: "Empty Trip",
	}

	pdfBytes, err := GeneratePDF(trip, nil, nil)
	if err != nil {
		t.Fatalf("GeneratePDF failed: %v", err)
	}

	// Valid PDF with magic header
	if string(pdfBytes[:5]) != "%PDF-" {
		t.Errorf("expected PDF magic header, got %q", string(pdfBytes[:5]))
	}

	// A minimal PDF with just a header and footer should be reasonably small
	// but still valid (at least a few hundred bytes for PDF structure)
	if len(pdfBytes) < 100 {
		t.Errorf("PDF seems too small (%d bytes), may be malformed", len(pdfBytes))
	}
}

func TestGeneratePDF_MultiDayTrip(t *testing.T) {
	tripID := uuid.New()
	tripStart := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	tripEnd := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)

	trip := &dbgen.Trip{
		ID:                   tripID,
		Title:                "Japan Adventure",
		Description:          pgtype.Text{String: "Exploring Tokyo and Kyoto", Valid: true},
		Status:               "planning",
		StartDate:            pgtype.Date{Time: tripStart, Valid: true},
		EndDate:              pgtype.Date{Time: tripEnd, Valid: true},
		DestinationCountries: []string{"JP"},
	}

	items := []dbgen.ItineraryItem{
		{
			ID:          uuid.New(),
			TripID:      tripID,
			Title:       pgtype.Text{String: "Visit Meiji Shrine", Valid: true},
			Description: pgtype.Text{String: "Historic Shinto shrine", Valid: true},
			Type:        pgtype.Text{String: "sightseeing", Valid: true},
			DayNumber:   pgtype.Int4{Int32: 1, Valid: true},
			StartTime:   pgtype.Timestamptz{Time: time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC), Valid: true},
			EndTime:     pgtype.Timestamptz{Time: time.Date(2026, 10, 1, 11, 0, 0, 0, time.UTC), Valid: true},
		},
		{
			ID:        uuid.New(),
			TripID:    tripID,
			Title:     pgtype.Text{String: "Lunch at Tsukiji", Valid: true},
			Type:      pgtype.Text{String: "food", Valid: true},
			DayNumber: pgtype.Int4{Int32: 1, Valid: true},
		},
		{
			ID:          uuid.New(),
			TripID:      tripID,
			Title:       pgtype.Text{String: "Fushimi Inari Taisha", Valid: true},
			Description: pgtype.Text{String: "Thousands of vermilion torii gates", Valid: true},
			Type:        pgtype.Text{String: "sightseeing", Valid: true},
			DayNumber:   pgtype.Int4{Int32: 3, Valid: true},
		},
	}

	pdfBytes, err := GeneratePDF(trip, items, nil)
	if err != nil {
		t.Fatalf("GeneratePDF failed: %v", err)
	}

	if string(pdfBytes[:5]) != "%PDF-" {
		t.Error("expected valid PDF header")
	}

	// Multi-day trip with items should produce a larger PDF than empty
	emptyTrip := &dbgen.Trip{ID: uuid.New(), Title: "Empty"}
	emptyPDF, _ := GeneratePDF(emptyTrip, nil, nil)
	if len(pdfBytes) <= len(emptyPDF) {
		t.Error("PDF with itinerary items should be larger than empty PDF")
	}
}

func TestGeneratePDF_WithBookings(t *testing.T) {
	tripID := uuid.New()

	trip := &dbgen.Trip{
		ID:    tripID,
		Title: "Barcelona Trip",
	}

	bookings := []dbgen.Booking{
		{
			ID:               uuid.New(),
			UserID:           uuid.New(),
			TripID:           pgtype.UUID{Bytes: tripID, Valid: true},
			Type:             "flight",
			Title:            "NYC to Barcelona",
			Provider:         pgtype.Text{String: "Delta Airlines", Valid: true},
			ConfirmationCode: pgtype.Text{String: "DL-98765", Valid: true},
			StartTime:        pgtype.Timestamptz{Time: time.Date(2026, 10, 5, 8, 0, 0, 0, time.UTC), Valid: true},
			EndTime:          pgtype.Timestamptz{Time: time.Date(2026, 10, 5, 20, 0, 0, 0, time.UTC), Valid: true},
		},
		{
			ID:               uuid.New(),
			UserID:           uuid.New(),
			TripID:           pgtype.UUID{Bytes: tripID, Valid: true},
			Type:             "hotel",
			Title:            "Hotel Arts Barcelona",
			Provider:         pgtype.Text{String: "Booking.com", Valid: true},
			ConfirmationCode: pgtype.Text{String: "BK-12345", Valid: true},
			StartTime:        pgtype.Timestamptz{Time: time.Date(2026, 10, 5, 15, 0, 0, 0, time.UTC), Valid: true},
			EndTime:          pgtype.Timestamptz{Time: time.Date(2026, 10, 10, 11, 0, 0, 0, time.UTC), Valid: true},
		},
	}

	pdfBytes, err := GeneratePDF(trip, nil, bookings)
	if err != nil {
		t.Fatalf("GeneratePDF failed: %v", err)
	}

	if string(pdfBytes[:5]) != "%PDF-" {
		t.Error("expected valid PDF header")
	}

	// PDF with bookings should be larger than empty
	emptyTrip := &dbgen.Trip{ID: uuid.New(), Title: "Empty"}
	emptyPDF, _ := GeneratePDF(emptyTrip, nil, nil)
	if len(pdfBytes) <= len(emptyPDF) {
		t.Error("PDF with bookings should be larger than empty PDF")
	}
}

func TestGeneratePDF_WithEstimatedCost(t *testing.T) {
	tripID := uuid.New()

	trip := &dbgen.Trip{
		ID:    tripID,
		Title: "Cost Test Trip",
	}

	itemsWithCost := []dbgen.ItineraryItem{
		{
			ID:                 uuid.New(),
			TripID:             tripID,
			Title:              pgtype.Text{String: "Fancy Dinner", Valid: true},
			DayNumber:          pgtype.Int4{Int32: 1, Valid: true},
			EstimatedCostCents: pgtype.Int8{Int64: 7500, Valid: true},
			CostCurrency:       pgtype.Text{String: "EUR", Valid: true},
		},
		{
			ID:                 uuid.New(),
			TripID:             tripID,
			Title:              pgtype.Text{String: "Museum Visit", Valid: true},
			DayNumber:          pgtype.Int4{Int32: 1, Valid: true},
			EstimatedCostCents: pgtype.Int8{Int64: 2000, Valid: true},
			// No currency set — should default to USD
		},
	}

	itemsNoCost := []dbgen.ItineraryItem{
		{
			ID:        uuid.New(),
			TripID:    tripID,
			Title:     pgtype.Text{String: "Free Walking Tour", Valid: true},
			DayNumber: pgtype.Int4{Int32: 1, Valid: true},
			// No cost
		},
	}

	pdfWithCost, err := GeneratePDF(trip, itemsWithCost, nil)
	if err != nil {
		t.Fatalf("GeneratePDF with cost failed: %v", err)
	}

	pdfNoCost, err := GeneratePDF(trip, itemsNoCost, nil)
	if err != nil {
		t.Fatalf("GeneratePDF without cost failed: %v", err)
	}

	// PDF with cost info should be larger than without
	// (cost line adds extra text content)
	if len(pdfWithCost) <= len(pdfNoCost) {
		t.Error("PDF with estimated costs should be larger than PDF without costs")
	}
}

func TestGeneratePDF_WithDatesAndCountries(t *testing.T) {
	tripWithMeta := &dbgen.Trip{
		ID:                   uuid.New(),
		Title:                "Multi-Country Trip",
		StartDate:            pgtype.Date{Time: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), Valid: true},
		EndDate:              pgtype.Date{Time: time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC), Valid: true},
		DestinationCountries: []string{"FR", "IT", "ES"},
		Status:               "planning",
	}

	tripNoMeta := &dbgen.Trip{
		ID:    uuid.New(),
		Title: "No Meta Trip",
	}

	pdfWithMeta, err := GeneratePDF(tripWithMeta, nil, nil)
	if err != nil {
		t.Fatalf("GeneratePDF with metadata failed: %v", err)
	}

	pdfNoMeta, err := GeneratePDF(tripNoMeta, nil, nil)
	if err != nil {
		t.Fatalf("GeneratePDF without metadata failed: %v", err)
	}

	// PDF with dates/countries should be larger
	if len(pdfWithMeta) <= len(pdfNoMeta) {
		t.Error("PDF with dates and countries should be larger than PDF without")
	}
}

func TestGeneratePDF_WithDescription(t *testing.T) {
	tripWithDesc := &dbgen.Trip{
		ID:          uuid.New(),
		Title:       "Described Trip",
		Description: pgtype.Text{String: "A wonderful adventure through multiple countries", Valid: true},
	}

	tripNoDesc := &dbgen.Trip{
		ID:    uuid.New(),
		Title: "No Desc Trip",
	}

	pdfWithDesc, err := GeneratePDF(tripWithDesc, nil, nil)
	if err != nil {
		t.Fatalf("GeneratePDF with description failed: %v", err)
	}

	pdfNoDesc, err := GeneratePDF(tripNoDesc, nil, nil)
	if err != nil {
		t.Fatalf("GeneratePDF without description failed: %v", err)
	}

	if len(pdfWithDesc) <= len(pdfNoDesc) {
		t.Error("PDF with description should be larger than PDF without")
	}
}

func TestGeneratePDF_NoError_LargeTrip(t *testing.T) {
	tripID := uuid.New()
	trip := &dbgen.Trip{
		ID:                   tripID,
		Title:                "Big Trip",
		Description:          pgtype.Text{String: "A very detailed trip", Valid: true},
		StartDate:            pgtype.Date{Time: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), Valid: true},
		EndDate:              pgtype.Date{Time: time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC), Valid: true},
		DestinationCountries: []string{"JP", "KR", "TW"},
		Status:               "planning",
	}

	// Create 14 days of items (2 per day)
	var items []dbgen.ItineraryItem
	for day := int32(1); day <= 14; day++ {
		items = append(items, dbgen.ItineraryItem{
			ID:                 uuid.New(),
			TripID:             tripID,
			Title:              pgtype.Text{String: "Morning Activity", Valid: true},
			Description:        pgtype.Text{String: "Some morning thing to do on this day", Valid: true},
			Type:               pgtype.Text{String: "activity", Valid: true},
			DayNumber:          pgtype.Int4{Int32: day, Valid: true},
			EstimatedCostCents: pgtype.Int8{Int64: 5000, Valid: true},
			CostCurrency:       pgtype.Text{String: "JPY", Valid: true},
		}, dbgen.ItineraryItem{
			ID:        uuid.New(),
			TripID:    tripID,
			Title:     pgtype.Text{String: "Afternoon Activity", Valid: true},
			Type:      pgtype.Text{String: "sightseeing", Valid: true},
			DayNumber: pgtype.Int4{Int32: day, Valid: true},
		})
	}

	bookings := []dbgen.Booking{
		{
			ID:               uuid.New(),
			UserID:           uuid.New(),
			TripID:           pgtype.UUID{Bytes: tripID, Valid: true},
			Type:             "flight",
			Title:            "Outbound Flight",
			Provider:         pgtype.Text{String: "ANA", Valid: true},
			ConfirmationCode: pgtype.Text{String: "ANA-001", Valid: true},
			StartTime:        pgtype.Timestamptz{Time: time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC), Valid: true},
		},
		{
			ID:               uuid.New(),
			UserID:           uuid.New(),
			TripID:           pgtype.UUID{Bytes: tripID, Valid: true},
			Type:             "hotel",
			Title:            "Tokyo Hotel",
			Provider:         pgtype.Text{String: "Hotels.com", Valid: true},
			ConfirmationCode: pgtype.Text{String: "HTL-999", Valid: true},
			StartTime:        pgtype.Timestamptz{Time: time.Date(2026, 6, 1, 15, 0, 0, 0, time.UTC), Valid: true},
			EndTime:          pgtype.Timestamptz{Time: time.Date(2026, 6, 7, 11, 0, 0, 0, time.UTC), Valid: true},
		},
	}

	pdfBytes, err := GeneratePDF(trip, items, bookings)
	if err != nil {
		t.Fatalf("GeneratePDF failed for large trip: %v", err)
	}

	if string(pdfBytes[:5]) != "%PDF-" {
		t.Error("expected valid PDF header for large trip")
	}

	// A 14-day trip should produce a multi-page PDF
	if len(pdfBytes) < 1000 {
		t.Errorf("large trip PDF seems too small (%d bytes)", len(pdfBytes))
	}
}

func TestGeneratePDF_SafeString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple text", "simple text"},
		{"ASCII only 123", "ASCII only 123"},
		{"Tokyo \u6771\u4eac", "Tokyo ??"},
		{"caf\u00e9", "caf\u00e9"}, // e with accent is < 256, should pass through
		{"", ""},
	}

	for _, tt := range tests {
		got := safeString(tt.input)
		if got != tt.expected {
			t.Errorf("safeString(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestParseTripIDFromExportPath_PDF(t *testing.T) {
	validID := uuid.New()

	tests := []struct {
		path   string
		wantOK bool
		wantID uuid.UUID
	}{
		{"/api/trips/" + validID.String() + "/export/pdf", true, validID},
		{"/api/trips/" + validID.String() + "/export/ical", true, validID},
		{"/api/trips/not-a-uuid/export/pdf", false, uuid.Nil},
		{"/api/trips/" + validID.String() + "/export/csv", false, uuid.Nil},
		{"/api/trips/" + validID.String() + "/export/", false, uuid.Nil},
		{"/api/trips/", false, uuid.Nil},
	}

	for _, tt := range tests {
		gotID, gotOK := parseTripIDFromExportPath(tt.path)
		if gotOK != tt.wantOK {
			t.Errorf("parseTripIDFromExportPath(%q) ok = %v, want %v", tt.path, gotOK, tt.wantOK)
		}
		if gotOK && gotID != tt.wantID {
			t.Errorf("parseTripIDFromExportPath(%q) id = %v, want %v", tt.path, gotID, tt.wantID)
		}
	}
}
