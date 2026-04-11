package handlers

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
)

func TestGenerateICS_EmptyItinerary(t *testing.T) {
	trip := &dbgen.Trip{
		ID:    uuid.New(),
		Title: "Empty Trip",
	}

	ics := GenerateICS(trip, nil, nil)

	if !strings.Contains(ics, "BEGIN:VCALENDAR") {
		t.Error("missing VCALENDAR begin")
	}
	if !strings.Contains(ics, "END:VCALENDAR") {
		t.Error("missing VCALENDAR end")
	}
	if !strings.Contains(ics, "X-WR-CALNAME:Empty Trip") {
		t.Error("missing calendar name")
	}
	if !strings.Contains(ics, "PRODID:-//Toqui//Trip Export//EN") {
		t.Error("missing PRODID")
	}
	if !strings.Contains(ics, "VERSION:2.0") {
		t.Error("missing VERSION")
	}
	if strings.Contains(ics, "BEGIN:VEVENT") {
		t.Error("should not have any events for empty itinerary")
	}
}

func TestGenerateICS_ItineraryWithTimes(t *testing.T) {
	tripID := uuid.New()
	itemID := uuid.New()
	startTime := time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)
	endTime := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)

	trip := &dbgen.Trip{
		ID:    tripID,
		Title: "Japan Trip",
	}

	items := []dbgen.ItineraryItem{
		{
			ID:     itemID,
			TripID: tripID,
			Title:  pgtype.Text{String: "Visit Kinkaku-ji Temple", Valid: true},
			Description: pgtype.Text{
				String: "Golden Pavilion temple visit",
				Valid:  true,
			},
			StartTime: pgtype.Timestamptz{Time: startTime, Valid: true},
			EndTime:   pgtype.Timestamptz{Time: endTime, Valid: true},
		},
	}

	ics := GenerateICS(trip, items, nil)

	if !strings.Contains(ics, "BEGIN:VEVENT") {
		t.Fatal("missing VEVENT")
	}
	if !strings.Contains(ics, "UID:"+itemID.String()+"@toqui.travel") {
		t.Error("missing or incorrect UID")
	}
	if !strings.Contains(ics, "DTSTART:20261005T090000Z") {
		t.Error("missing or incorrect DTSTART")
	}
	if !strings.Contains(ics, "DTEND:20261005T120000Z") {
		t.Error("missing or incorrect DTEND")
	}
	if !strings.Contains(ics, "SUMMARY:Visit Kinkaku-ji Temple") {
		t.Error("missing or incorrect SUMMARY")
	}
	if !strings.Contains(ics, "DESCRIPTION:Golden Pavilion temple visit") {
		t.Error("missing or incorrect DESCRIPTION")
	}
}

func TestGenerateICS_ItineraryWithoutTimes_UseDayNumber(t *testing.T) {
	tripID := uuid.New()
	itemID := uuid.New()
	tripStart := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

	trip := &dbgen.Trip{
		ID:        tripID,
		Title:     "Greece Trip",
		StartDate: pgtype.Date{Time: tripStart, Valid: true},
	}

	items := []dbgen.ItineraryItem{
		{
			ID:        itemID,
			TripID:    tripID,
			Title:     pgtype.Text{String: "Explore Athens", Valid: true},
			DayNumber: pgtype.Int4{Int32: 3, Valid: true},
		},
	}

	ics := GenerateICS(trip, items, nil)

	// Day 3 of a trip starting Oct 1 = Oct 3 (1-based: day 1 = Oct 1, day 2 = Oct 2, day 3 = Oct 3)
	if !strings.Contains(ics, "DTSTART;VALUE=DATE:20261003") {
		t.Error("expected all-day event on Oct 3 for day_number=3")
	}
	if !strings.Contains(ics, "DTEND;VALUE=DATE:20261004") {
		t.Error("expected DTEND on Oct 4 for all-day event")
	}
}

func TestGenerateICS_ItineraryWithoutTimes_NoStartDate(t *testing.T) {
	tripID := uuid.New()
	itemID := uuid.New()

	trip := &dbgen.Trip{
		ID:    tripID,
		Title: "Somewhere Trip",
	}

	items := []dbgen.ItineraryItem{
		{
			ID:     itemID,
			TripID: tripID,
			Title:  pgtype.Text{String: "Do Something", Valid: true},
		},
	}

	ics := GenerateICS(trip, items, nil)

	// Should still produce a VEVENT with fallback dates
	if !strings.Contains(ics, "BEGIN:VEVENT") {
		t.Error("expected a VEVENT even without dates")
	}
	if !strings.Contains(ics, "DTSTART;VALUE=DATE:") {
		t.Error("expected all-day fallback event")
	}
}

func TestGenerateICS_Bookings(t *testing.T) {
	tripID := uuid.New()
	bookingID := uuid.New()
	startTime := time.Date(2026, 10, 10, 14, 30, 0, 0, time.UTC)
	endTime := time.Date(2026, 10, 10, 16, 0, 0, 0, time.UTC)

	trip := &dbgen.Trip{
		ID:    tripID,
		Title: "Barcelona Trip",
	}

	bookings := []dbgen.Booking{
		{
			ID:               bookingID,
			UserID:           uuid.New(),
			TripID:           pgtype.UUID{Bytes: tripID, Valid: true},
			Type:             "activity",
			Title:            "Sagrada Familia Tour",
			StartTime:        pgtype.Timestamptz{Time: startTime, Valid: true},
			EndTime:          pgtype.Timestamptz{Time: endTime, Valid: true},
			Provider:         pgtype.Text{String: "GetYourGuide", Valid: true},
			ConfirmationCode: pgtype.Text{String: "SGF-12345", Valid: true},
			Address:          pgtype.Text{String: "Carrer de Mallorca 401, Barcelona", Valid: true},
		},
	}

	ics := GenerateICS(trip, nil, bookings)

	if !strings.Contains(ics, "BEGIN:VEVENT") {
		t.Fatal("missing VEVENT for booking")
	}
	if !strings.Contains(ics, "UID:"+bookingID.String()+"@toqui.travel") {
		t.Error("missing booking UID")
	}
	if !strings.Contains(ics, "DTSTART:20261010T143000Z") {
		t.Error("missing booking DTSTART")
	}
	if !strings.Contains(ics, "DTEND:20261010T160000Z") {
		t.Error("missing booking DTEND")
	}
	if !strings.Contains(ics, "SUMMARY:[Activity] Sagrada Familia Tour") {
		t.Error("missing or incorrect booking SUMMARY")
	}
	if !strings.Contains(ics, "Provider: GetYourGuide") {
		t.Error("missing provider in DESCRIPTION")
	}
	if !strings.Contains(ics, "Confirmation: SGF-12345") {
		t.Error("missing confirmation code in DESCRIPTION")
	}
	if !strings.Contains(ics, "LOCATION:Carrer de Mallorca 401\\, Barcelona") {
		t.Error("missing or incorrectly escaped LOCATION")
	}
}

func TestGenerateICS_BookingWithFlightLocations(t *testing.T) {
	tripID := uuid.New()

	trip := &dbgen.Trip{
		ID:    tripID,
		Title: "Flight Trip",
	}

	bookings := []dbgen.Booking{
		{
			ID:                uuid.New(),
			UserID:            uuid.New(),
			TripID:            pgtype.UUID{Bytes: tripID, Valid: true},
			Type:              "flight",
			Title:             "NYC to Barcelona",
			StartTime:         pgtype.Timestamptz{Time: time.Date(2026, 10, 5, 8, 0, 0, 0, time.UTC), Valid: true},
			DepartureLocation: pgtype.Text{String: "JFK Airport", Valid: true},
			ArrivalLocation:   pgtype.Text{String: "Barcelona El Prat", Valid: true},
		},
	}

	ics := GenerateICS(trip, nil, bookings)

	if !strings.Contains(ics, "LOCATION:JFK Airport -> Barcelona El Prat") {
		t.Error("missing departure -> arrival LOCATION for flight booking")
	}
}

func TestGenerateICS_BookingWithoutEndTime(t *testing.T) {
	tripID := uuid.New()
	startTime := time.Date(2026, 10, 5, 14, 0, 0, 0, time.UTC)

	trip := &dbgen.Trip{
		ID:    tripID,
		Title: "Short Trip",
	}

	bookings := []dbgen.Booking{
		{
			ID:        uuid.New(),
			UserID:    uuid.New(),
			TripID:    pgtype.UUID{Bytes: tripID, Valid: true},
			Type:      "activity",
			Title:     "Quick Tour",
			StartTime: pgtype.Timestamptz{Time: startTime, Valid: true},
		},
	}

	ics := GenerateICS(trip, nil, bookings)

	// Should default to 1 hour duration
	if !strings.Contains(ics, "DTEND:20261005T150000Z") {
		t.Error("expected DTEND 1 hour after start for booking without end time")
	}
}

func TestGenerateICS_LinkedBookingNotDuplicated(t *testing.T) {
	tripID := uuid.New()
	bookingID := uuid.New()
	itemID := uuid.New()
	startTime := time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)

	trip := &dbgen.Trip{
		ID:    tripID,
		Title: "No Dupes Trip",
	}

	items := []dbgen.ItineraryItem{
		{
			ID:        itemID,
			TripID:    tripID,
			Title:     pgtype.Text{String: "Hotel Check-in", Valid: true},
			StartTime: pgtype.Timestamptz{Time: startTime, Valid: true},
			BookingID: pgtype.UUID{Bytes: bookingID, Valid: true},
		},
	}

	bookings := []dbgen.Booking{
		{
			ID:        bookingID,
			UserID:    uuid.New(),
			TripID:    pgtype.UUID{Bytes: tripID, Valid: true},
			Type:      "hotel",
			Title:     "Hotel Arts Barcelona",
			StartTime: pgtype.Timestamptz{Time: startTime, Valid: true},
		},
	}

	ics := GenerateICS(trip, items, bookings)

	// Should only have ONE VEVENT (from the itinerary item), not two
	count := strings.Count(ics, "BEGIN:VEVENT")
	if count != 1 {
		t.Errorf("expected 1 VEVENT (linked booking should be deduplicated), got %d", count)
	}
}

func TestGenerateICS_MixedItemsAndBookings(t *testing.T) {
	tripID := uuid.New()
	tripStart := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

	trip := &dbgen.Trip{
		ID:        tripID,
		Title:     "Mixed Trip",
		StartDate: pgtype.Date{Time: tripStart, Valid: true},
	}

	items := []dbgen.ItineraryItem{
		{
			ID:        uuid.New(),
			TripID:    tripID,
			Title:     pgtype.Text{String: "Morning Walk", Valid: true},
			DayNumber: pgtype.Int4{Int32: 1, Valid: true},
		},
		{
			ID:     uuid.New(),
			TripID: tripID,
			Title:  pgtype.Text{String: "Afternoon Tour", Valid: true},
			StartTime: pgtype.Timestamptz{
				Time:  time.Date(2026, 10, 1, 14, 0, 0, 0, time.UTC),
				Valid: true,
			},
			EndTime: pgtype.Timestamptz{
				Time:  time.Date(2026, 10, 1, 17, 0, 0, 0, time.UTC),
				Valid: true,
			},
		},
	}

	bookings := []dbgen.Booking{
		{
			ID:     uuid.New(),
			UserID: uuid.New(),
			TripID: pgtype.UUID{Bytes: tripID, Valid: true},
			Type:   "hotel",
			Title:  "Hotel Check-in",
			StartTime: pgtype.Timestamptz{
				Time:  time.Date(2026, 10, 1, 15, 0, 0, 0, time.UTC),
				Valid: true,
			},
		},
	}

	ics := GenerateICS(trip, items, bookings)

	count := strings.Count(ics, "BEGIN:VEVENT")
	if count != 3 {
		t.Errorf("expected 3 VEVENTs (2 items + 1 booking), got %d", count)
	}
}

func TestEscapeICSValue(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple text", "simple text"},
		{"text with, comma", "text with\\, comma"},
		{"text with; semicolon", "text with\\; semicolon"},
		{"line one\nline two", "line one\\nline two"},
		{"backslash\\here", "backslash\\\\here"},
		{"combo,;\n\\", "combo\\,\\;\\n\\\\"},
	}

	for _, tt := range tests {
		got := escapeICSValue(tt.input)
		if got != tt.expected {
			t.Errorf("escapeICSValue(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"My Trip", "My-Trip"},
		{"", "export"},
		{"Japan 2026!", "Japan-2026"},
		{"Hello/World<>Test", "HelloWorldTest"},
		{"---leading---trailing---", "leading-trailing"},
		{"   spaces   ", "spaces"},
	}

	for _, tt := range tests {
		got := sanitizeFilename(tt.input)
		if got != tt.expected {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestParseTripIDFromExportPath(t *testing.T) {
	validID := uuid.New()

	tests := []struct {
		path   string
		wantOK bool
		wantID uuid.UUID
	}{
		{"/api/trips/" + validID.String() + "/export/ical", true, validID},
		{"/api/trips/not-a-uuid/export/ical", false, uuid.Nil},
		{"/api/trips/", false, uuid.Nil},
		{"/api/trips/" + validID.String() + "/invite", false, uuid.Nil},
		{"/something/else", false, uuid.Nil},
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

func TestWriteICSLine_Folding(t *testing.T) {
	var b strings.Builder
	// Create a value that when combined with property will exceed 73 chars
	longValue := strings.Repeat("A", 100)
	writeICSLine(&b, "SUMMARY", longValue)

	result := b.String()
	// Should contain a fold (CRLF + space)
	if !strings.Contains(result, "\r\n ") {
		t.Error("expected line folding for long property value")
	}
	// The unfolded content should equal the original
	unfolded := strings.ReplaceAll(result, "\r\n ", "")
	unfolded = strings.TrimSuffix(unfolded, "\r\n")
	if unfolded != "SUMMARY:"+longValue {
		t.Errorf("unfolded content mismatch: got %q", unfolded)
	}
}

func TestFormatICSDateTime(t *testing.T) {
	tm := time.Date(2026, 10, 5, 9, 30, 15, 0, time.UTC)
	got := formatICSDateTime(tm)
	want := "20261005T093015Z"
	if got != want {
		t.Errorf("formatICSDateTime() = %q, want %q", got, want)
	}
}

func TestFormatICSDateTime_NonUTC(t *testing.T) {
	// Should convert to UTC
	loc := time.FixedZone("EST", -5*3600)
	tm := time.Date(2026, 10, 5, 9, 0, 0, 0, loc)
	got := formatICSDateTime(tm)
	want := "20261005T140000Z" // 9 AM EST = 2 PM UTC
	if got != want {
		t.Errorf("formatICSDateTime() = %q, want %q", got, want)
	}
}

func TestGenerateICS_SpecialCharactersInTitle(t *testing.T) {
	trip := &dbgen.Trip{
		ID:    uuid.New(),
		Title: "Trip, with; special\nchars",
	}

	items := []dbgen.ItineraryItem{
		{
			ID:     uuid.New(),
			TripID: trip.ID,
			Title:  pgtype.Text{String: "Dinner at Café & Bar, Downtown", Valid: true},
			StartTime: pgtype.Timestamptz{
				Time:  time.Date(2026, 10, 5, 19, 0, 0, 0, time.UTC),
				Valid: true,
			},
		},
	}

	ics := GenerateICS(trip, items, nil)

	// Calendar name should have special chars escaped
	if !strings.Contains(ics, "X-WR-CALNAME:Trip\\, with\\; special\\nchars") {
		t.Error("calendar name should have special characters escaped")
	}
	// Item title should have comma escaped
	if !strings.Contains(ics, "SUMMARY:Dinner at Caf") {
		t.Error("expected escaped summary in output")
	}
}

func TestGenerateICS_BookingTypeOther(t *testing.T) {
	tripID := uuid.New()

	trip := &dbgen.Trip{
		ID:    tripID,
		Title: "Other Booking Trip",
	}

	bookings := []dbgen.Booking{
		{
			ID:     uuid.New(),
			UserID: uuid.New(),
			TripID: pgtype.UUID{Bytes: tripID, Valid: true},
			Type:   "other",
			Title:  "Generic Booking",
			StartTime: pgtype.Timestamptz{
				Time:  time.Date(2026, 10, 5, 10, 0, 0, 0, time.UTC),
				Valid: true,
			},
		},
	}

	ics := GenerateICS(trip, nil, bookings)

	// Type "other" should not add a prefix
	if !strings.Contains(ics, "SUMMARY:Generic Booking") {
		t.Error("booking with type 'other' should not have type prefix in summary")
	}
	if strings.Contains(ics, "[Other]") {
		t.Error("type 'other' should not be shown as prefix")
	}
}

func TestGenerateICS_CRLFLineEndings(t *testing.T) {
	trip := &dbgen.Trip{
		ID:    uuid.New(),
		Title: "CRLF Test",
	}

	ics := GenerateICS(trip, nil, nil)

	// All lines should end with \r\n per RFC 5545
	lines := strings.Split(ics, "\r\n")
	if len(lines) < 5 {
		t.Fatal("expected at least 5 lines in minimal ICS output")
	}

	// The raw string should not contain bare \n (without preceding \r)
	cleaned := strings.ReplaceAll(ics, "\r\n", "")
	if strings.Contains(cleaned, "\n") {
		t.Error("found bare \\n without \\r - ICS requires CRLF line endings")
	}
}
