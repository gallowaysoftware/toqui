package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gallowaysoftware/toqui/backend/internal/auth"
	"github.com/gallowaysoftware/toqui/backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui/backend/internal/trip"
)

// ICalExportHandler handles iCal/ICS export of trip itineraries.
type ICalExportHandler struct {
	tripSvc *trip.Service
	authSvc *auth.Service
	queries *dbgen.Queries
}

// NewICalExportHandler creates a new ICalExportHandler.
func NewICalExportHandler(tripSvc *trip.Service, authSvc *auth.Service, pool *pgxpool.Pool) *ICalExportHandler {
	return &ICalExportHandler{
		tripSvc: tripSvc,
		authSvc: authSvc,
		queries: dbgen.New(pool),
	}
}

// HandleExportICal handles GET /api/trips/{id}/export/ical.
// Returns an ICS file containing all itinerary items and bookings as VEVENTs.
func (h *ICalExportHandler) HandleExportICal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := authenticateRESTRequest(r, h.authSvc)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Extract trip ID from URL path: /api/trips/{id}/export/ical
	tripID, ok := parseTripIDFromExportPath(r.URL.Path)
	if !ok {
		http.Error(w, "invalid trip ID", http.StatusBadRequest)
		return
	}

	// Load trip (verifies ownership)
	t, err := h.tripSvc.GetByID(r.Context(), userID, tripID)
	if err != nil {
		slog.Warn("ical export: trip not found or not owned", "trip_id", tripID, "user_id", userID, "error", err)
		http.Error(w, "trip not found", http.StatusNotFound)
		return
	}

	// Load itinerary items
	items, err := h.tripSvc.GetItinerary(r.Context(), tripID)
	if err != nil {
		slog.Error("ical export: failed to load itinerary", "trip_id", tripID, "error", err)
		http.Error(w, "failed to load itinerary", http.StatusInternalServerError)
		return
	}

	// Load bookings
	bookings, err := h.queries.ListBookingsByTrip(r.Context(), dbgen.ListBookingsByTripParams{
		TripID: uuidToPgtype(tripID),
		UserID: userID,
	})
	if err != nil {
		slog.Error("ical export: failed to load bookings", "trip_id", tripID, "error", err)
		http.Error(w, "failed to load bookings", http.StatusInternalServerError)
		return
	}

	// Generate ICS content
	ics := GenerateICS(t, items, bookings)

	// Sanitize title for filename
	filename := sanitizeFilename(t.Title)

	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="trip-%s.ics"`, filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(ics))
}

// parseTripIDFromExportPath extracts the trip UUID from a path like
// /api/trips/{id}/export/ical or /api/trips/{id}/export/pdf.
func parseTripIDFromExportPath(path string) (uuid.UUID, bool) {
	// Expected: /api/trips/<uuid>/export/{format}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	// parts: [api, trips, <uuid>, export, ical|pdf]
	if len(parts) < 5 || parts[0] != "api" || parts[1] != "trips" || parts[3] != "export" {
		return uuid.Nil, false
	}
	// Validate the export format
	switch parts[4] {
	case "ical", "pdf":
		// valid
	default:
		return uuid.Nil, false
	}
	id, err := uuid.Parse(parts[2])
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

// GenerateICS produces an RFC 5545 iCalendar document from a trip's itinerary
// items and bookings. Each item and booking becomes a VEVENT.
func GenerateICS(t *dbgen.Trip, items []dbgen.ItineraryItem, bookings []dbgen.Booking) string {
	var b strings.Builder

	calName := t.Title
	if calName == "" {
		calName = "Toqui Trip"
	}

	b.WriteString("BEGIN:VCALENDAR\r\n")
	b.WriteString("VERSION:2.0\r\n")
	b.WriteString("PRODID:-//Toqui//Trip Export//EN\r\n")
	b.WriteString("CALSCALE:GREGORIAN\r\n")
	b.WriteString("METHOD:PUBLISH\r\n")
	writeICSLine(&b, "X-WR-CALNAME", calName)

	// Itinerary items
	for _, item := range items {
		writeItineraryEvent(&b, t, &item)
	}

	// Bookings (skip those already linked to itinerary items to avoid duplicates)
	linkedBookingIDs := make(map[uuid.UUID]bool)
	for _, item := range items {
		if item.BookingID.Valid {
			linkedBookingIDs[item.BookingID.Bytes] = true
		}
	}
	for _, bk := range bookings {
		if linkedBookingIDs[bk.ID] {
			continue
		}
		writeBookingEvent(&b, t, &bk)
	}

	b.WriteString("END:VCALENDAR\r\n")
	return b.String()
}

// writeItineraryEvent writes a VEVENT for an itinerary item.
func writeItineraryEvent(b *strings.Builder, t *dbgen.Trip, item *dbgen.ItineraryItem) {
	b.WriteString("BEGIN:VEVENT\r\n")
	writeICSLine(b, "UID", item.ID.String()+"@toqui.travel")

	dtStart, dtEnd, allDay := resolveItineraryTimes(t, item)

	if allDay {
		// VALUE=DATE format: YYYYMMDD (no time component)
		fmt.Fprintf(b, "DTSTART;VALUE=DATE:%s\r\n", dtStart.Format("20060102"))
		fmt.Fprintf(b, "DTEND;VALUE=DATE:%s\r\n", dtEnd.Format("20060102"))
	} else {
		fmt.Fprintf(b, "DTSTART:%s\r\n", formatICSDateTime(dtStart))
		fmt.Fprintf(b, "DTEND:%s\r\n", formatICSDateTime(dtEnd))
	}

	if item.Title.Valid && item.Title.String != "" {
		writeICSLine(b, "SUMMARY", item.Title.String)
	}
	if item.Description.Valid && item.Description.String != "" {
		writeICSLine(b, "DESCRIPTION", item.Description.String)
	}

	b.WriteString("END:VEVENT\r\n")
}

// writeBookingEvent writes a VEVENT for a booking.
func writeBookingEvent(b *strings.Builder, t *dbgen.Trip, bk *dbgen.Booking) {
	b.WriteString("BEGIN:VEVENT\r\n")
	writeICSLine(b, "UID", bk.ID.String()+"@toqui.travel")

	if bk.StartTime.Valid {
		fmt.Fprintf(b, "DTSTART:%s\r\n", formatICSDateTime(bk.StartTime.Time))
		if bk.EndTime.Valid {
			fmt.Fprintf(b, "DTEND:%s\r\n", formatICSDateTime(bk.EndTime.Time))
		} else {
			// Default to 1 hour duration for bookings without end time
			fmt.Fprintf(b, "DTEND:%s\r\n", formatICSDateTime(bk.StartTime.Time.Add(time.Hour)))
		}
	} else if t.StartDate.Valid {
		// No start time: use trip start date if available, otherwise skip times
		fmt.Fprintf(b, "DTSTART;VALUE=DATE:%s\r\n", t.StartDate.Time.Format("20060102"))
		fmt.Fprintf(b, "DTEND;VALUE=DATE:%s\r\n", t.StartDate.Time.AddDate(0, 0, 1).Format("20060102"))
	}

	summary := bk.Title
	if bk.Type != "" && bk.Type != "other" {
		typeName := strings.ToUpper(bk.Type[:1]) + bk.Type[1:]
		summary = fmt.Sprintf("[%s] %s", typeName, bk.Title)
	}
	writeICSLine(b, "SUMMARY", summary)

	// Build description from available booking details
	var descParts []string
	if bk.Provider.Valid && bk.Provider.String != "" {
		descParts = append(descParts, "Provider: "+bk.Provider.String)
	}
	if bk.ConfirmationCode.Valid && bk.ConfirmationCode.String != "" {
		descParts = append(descParts, "Confirmation: "+bk.ConfirmationCode.String)
	}
	if len(descParts) > 0 {
		writeICSLine(b, "DESCRIPTION", strings.Join(descParts, "\\n"))
	}

	// Location from address or departure/arrival
	if bk.Address.Valid && bk.Address.String != "" {
		writeICSLine(b, "LOCATION", bk.Address.String)
	} else if bk.DepartureLocation.Valid && bk.DepartureLocation.String != "" {
		loc := bk.DepartureLocation.String
		if bk.ArrivalLocation.Valid && bk.ArrivalLocation.String != "" {
			loc += " -> " + bk.ArrivalLocation.String
		}
		writeICSLine(b, "LOCATION", loc)
	}

	b.WriteString("END:VEVENT\r\n")
}

// resolveItineraryTimes determines start/end times for an itinerary item.
// Returns the times and whether this is an all-day event.
func resolveItineraryTimes(t *dbgen.Trip, item *dbgen.ItineraryItem) (start, end time.Time, allDay bool) {
	if item.StartTime.Valid {
		start = item.StartTime.Time
		if item.EndTime.Valid {
			end = item.EndTime.Time
		} else {
			// Default to 1 hour
			end = start.Add(time.Hour)
		}
		return start, end, false
	}

	// No explicit start_time: compute from trip start_date + day_number
	if t.StartDate.Valid && item.DayNumber.Valid && item.DayNumber.Int32 > 0 {
		// day_number is 1-based: day 1 = trip start date
		dayOffset := int(item.DayNumber.Int32) - 1
		start = t.StartDate.Time.AddDate(0, 0, dayOffset)
		end = start.AddDate(0, 0, 1) // all-day event spans next day
		return start, end, true
	}

	// Fallback: use trip start date as an all-day event
	if t.StartDate.Valid {
		start = t.StartDate.Time
		end = start.AddDate(0, 0, 1)
		return start, end, true
	}

	// Absolute fallback: use current date
	now := time.Now().UTC().Truncate(24 * time.Hour)
	return now, now.AddDate(0, 0, 1), true
}

// formatICSDateTime formats a time in iCalendar UTC datetime format.
func formatICSDateTime(t time.Time) string {
	return t.UTC().Format("20060102T150405Z")
}

// writeICSLine writes a property line with proper folding per RFC 5545.
// Lines exceeding 75 octets are folded with CRLF + space.
func writeICSLine(b *strings.Builder, property, value string) {
	// Escape special characters per RFC 5545
	value = escapeICSValue(value)

	line := property + ":" + value

	// RFC 5545 requires lines be no longer than 75 octets.
	// We fold at 73 to account for the CRLF.
	const maxLen = 73
	for len(line) > maxLen {
		b.WriteString(line[:maxLen])
		b.WriteString("\r\n ")
		line = line[maxLen:]
	}
	b.WriteString(line)
	b.WriteString("\r\n")
}

// escapeICSValue escapes special characters in an ICS property value.
func escapeICSValue(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, ";", "\\;")
	s = strings.ReplaceAll(s, ",", "\\,")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "")
	return s
}

// sanitizeFilename removes or replaces characters unsafe for filenames.
func sanitizeFilename(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "export"
	}
	// Replace spaces and common separators with hyphens
	s = strings.ReplaceAll(s, " ", "-")
	// Remove anything that's not alphanumeric, hyphen, or underscore
	re := regexp.MustCompile(`[^a-zA-Z0-9\-_]`)
	s = re.ReplaceAllString(s, "")
	// Collapse multiple hyphens
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-")
	if s == "" {
		return "export"
	}
	// Truncate to reasonable length
	if len(s) > 64 {
		s = s[:64]
	}
	return s
}

// uuidToPgtype converts a google/uuid to pgtype.UUID.
func uuidToPgtype(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}
