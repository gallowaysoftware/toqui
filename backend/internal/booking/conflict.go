package booking

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/gallowaysoftware/toqui/backend/internal/dbgen"
)

// Conflict describes a scheduling problem detected between two or more
// bookings on the same trip. The detector is intentionally conservative —
// it only flags clear, time-based contradictions. Anything fuzzier is
// surfaced via CoverageGap (gap detection) instead.
type Conflict struct {
	// Type is a stable machine-readable identifier so the frontend can
	// pick an icon, copy, or call to action without parsing Message.
	Type string `json:"type"`

	// Severity is "warning" (looks risky, user should double-check) or
	// "error" (almost certainly a problem the user has to act on).
	Severity string `json:"severity"`

	// Message is a short human-readable summary safe to show as-is. It
	// must NOT contain destination names, hotel names, dates, or any
	// other travel content beyond what the conflict type implies — see
	// CLAUDE.md privacy rules.
	Message string `json:"message"`

	// BookingIDs lists the bookings involved in the conflict, in stable
	// order so frontend tests can rely on it.
	BookingIDs []string `json:"booking_ids"`
}

const (
	ConflictTightLayover        = "tight_layover"
	ConflictHotelBeforeArrival  = "hotel_before_arrival"
	ConflictCarReturnMismatch   = "car_return_mismatch"
	ConflictMissingReturnFlight = "missing_return_flight"

	// minLayoverMinutes is the threshold below which two flights with
	// matching connection airports trigger a tight-layover warning. 90
	// minutes is conservative — IATA's MCT (minimum connection time) for
	// most international transfers is 60–75 minutes; below that domestic
	// travelers often miss bags and gates.
	minLayoverMinutes = 90

	// carReturnBufferMinutes is the slack we want between car rental
	// return and an outbound flight departure. Below this, the user
	// almost certainly misses the flight or eats penalty fees on the
	// rental.
	carReturnBufferMinutes = 30
)

// DetectConflicts scans a slice of bookings (typically all bookings for a
// single trip) and returns every conflict it can detect. The function is
// pure — it makes no DB calls and accepts no clock — and is safe to call
// from any context including hot paths and tests.
//
// The detector is conservative: bookings missing critical fields (e.g. a
// flight without start/end times or airports) are simply skipped rather
// than reported as conflicts. The goal is "no false positives" because
// a noisy detector trains users to ignore alerts.
func DetectConflicts(bookings []dbgen.Booking) []Conflict {
	if len(bookings) < 1 {
		return nil
	}

	flights := make([]dbgen.Booking, 0, len(bookings))
	hotels := make([]dbgen.Booking, 0, len(bookings))
	cars := make([]dbgen.Booking, 0, len(bookings))
	for _, b := range bookings {
		switch b.Type {
		case "flight":
			flights = append(flights, b)
		case "hotel", "vacation_rental", "hostel":
			hotels = append(hotels, b)
		case "car_rental":
			cars = append(cars, b)
		}
	}

	var out []Conflict
	out = append(out, detectTightLayovers(flights)...)
	out = append(out, detectHotelBeforeArrival(flights, hotels)...)
	out = append(out, detectCarReturnMismatches(flights, cars)...)
	if c := detectMissingReturnFlight(flights); c != nil {
		out = append(out, *c)
	}
	return out
}

// detectTightLayovers finds ordered pairs of flights (a, b) where flight a's
// arrival airport equals flight b's departure airport AND flight b departs
// less than minLayoverMinutes after flight a lands. We enumerate every
// (i, j) pair (both directions, i != j) so that flights with identical
// end_times still produce conflicts in either direction, and so that the
// detector is order-independent regardless of how callers pass bookings.
func detectTightLayovers(flights []dbgen.Booking) []Conflict {
	if len(flights) < 2 {
		return nil
	}
	var out []Conflict
	for i := 0; i < len(flights); i++ {
		a := flights[i]
		aArrival := normalizeAirport(a.ArrivalLocation)
		aEnd := endTime(a)
		if aArrival == "" || aEnd.IsZero() || a.ID == uuid.Nil {
			continue
		}
		for j := 0; j < len(flights); j++ {
			if i == j {
				continue
			}
			b := flights[j]
			bDeparture := normalizeAirport(b.DepartureLocation)
			bStart := startTime(b)
			if bDeparture == "" || bStart.IsZero() || b.ID == uuid.Nil {
				continue
			}
			if aArrival != bDeparture {
				continue
			}
			delta := bStart.Sub(aEnd)
			if delta < 0 || delta >= minLayoverMinutes*time.Minute {
				// Negative deltas are handled by the symmetric (j, i)
				// iteration. Deltas at or beyond the threshold are fine.
				continue
			}
			out = append(out, Conflict{
				Type:       ConflictTightLayover,
				Severity:   "warning",
				Message:    fmt.Sprintf("Layover under %d minutes between two flights at the same airport.", minLayoverMinutes),
				BookingIDs: []string{a.ID.String(), b.ID.String()},
			})
		}
	}
	return out
}

// detectHotelBeforeArrival finds hotels whose check-in time precedes the
// earliest inbound flight's landing time. This is a soft warning — many
// hotels allow early check-in — but it's surprising and worth flagging.
//
// We only flag hotels whose check-in is BEFORE the earliest flight ends
// (not before all flights end), because users with multi-leg trips often
// have the hotel land in the middle of the flight schedule.
//
// Requires at least two flights so we have a credible "inbound" anchor.
// With a single flight we can't tell whether it's the inbound or the
// return leg — and treating a return-only import as the inbound would
// flag every legitimate hotel on the trip.
func detectHotelBeforeArrival(flights, hotels []dbgen.Booking) []Conflict {
	if len(flights) < 2 || len(hotels) == 0 {
		return nil
	}
	// Earliest flight arrival.
	earliest := time.Time{}
	for _, f := range flights {
		end := endTime(f)
		if end.IsZero() {
			continue
		}
		if earliest.IsZero() || end.Before(earliest) {
			earliest = end
		}
	}
	if earliest.IsZero() {
		return nil
	}

	var out []Conflict
	for _, h := range hotels {
		start := startTime(h)
		if start.IsZero() || h.ID == uuid.Nil {
			continue
		}
		if start.Before(earliest) {
			out = append(out, Conflict{
				Type:       ConflictHotelBeforeArrival,
				Severity:   "warning",
				Message:    "Accommodation check-in is before your inbound flight lands.",
				BookingIDs: []string{h.ID.String()},
			})
		}
	}
	return out
}

// detectCarReturnMismatches finds car rentals whose return time is at or
// after the user's outbound (return-leg) flight departure, or within
// carReturnBufferMinutes of it. The "outbound" flight is the latest
// flight on the trip — no separate marker is needed because if a flight
// is later than the car return, the user expects to drive to the airport.
//
// Requires at least two flights so the "latest" flight is a credible
// outbound anchor. With a single flight we can't tell inbound from
// outbound: an inbound-only one-way (user lands and rents a car for the
// whole stay) would flag every car rental as ending after the "outbound"
// flight, which is just the inbound flight in the past.
func detectCarReturnMismatches(flights, cars []dbgen.Booking) []Conflict {
	if len(flights) < 2 || len(cars) == 0 {
		return nil
	}
	// Latest flight departure — that's the "going home" flight.
	latest := time.Time{}
	for _, f := range flights {
		start := startTime(f)
		if start.IsZero() {
			continue
		}
		if start.After(latest) {
			latest = start
		}
	}
	if latest.IsZero() {
		return nil
	}

	var out []Conflict
	for _, c := range cars {
		end := endTime(c)
		if end.IsZero() || c.ID == uuid.Nil {
			continue
		}
		// If the car is returned BEFORE the trip's latest flight, only
		// flag if the gap is suspiciously small (within the buffer).
		// If the car is returned AFTER the flight departs — that's a
		// hard problem.
		if end.After(latest) {
			out = append(out, Conflict{
				Type:       ConflictCarReturnMismatch,
				Severity:   "error",
				Message:    "Car rental ends after your outbound flight departs.",
				BookingIDs: []string{c.ID.String()},
			})
			continue
		}
		gap := latest.Sub(end)
		if gap < carReturnBufferMinutes*time.Minute {
			out = append(out, Conflict{
				Type:       ConflictCarReturnMismatch,
				Severity:   "warning",
				Message:    fmt.Sprintf("Car rental ends within %d minutes of your outbound flight departure.", carReturnBufferMinutes),
				BookingIDs: []string{c.ID.String()},
			})
		}
	}
	return out
}

// detectMissingReturnFlight returns a single soft warning if the user has
// exactly one flight booking. This usually means they paid for the
// outbound but haven't yet booked the return — worth nudging.
//
// Two-flight trips are treated as complete; if both flights are outbound
// this catches it weakly via tight_layover or other checks, and if a
// user is genuinely on a one-way the warning is easy to dismiss.
func detectMissingReturnFlight(flights []dbgen.Booking) *Conflict {
	if len(flights) != 1 {
		return nil
	}
	if flights[0].ID == uuid.Nil {
		return nil
	}
	return &Conflict{
		Type:       ConflictMissingReturnFlight,
		Severity:   "warning",
		Message:    "You have an outbound flight but no return leg booked yet.",
		BookingIDs: []string{flights[0].ID.String()},
	}
}

// startTime returns the booking's start_time as a time.Time, or zero if
// not set.
func startTime(b dbgen.Booking) time.Time {
	if !b.StartTime.Valid {
		return time.Time{}
	}
	return b.StartTime.Time
}

// endTime returns the booking's end_time as a time.Time, or zero if not
// set.
func endTime(b dbgen.Booking) time.Time {
	if !b.EndTime.Valid {
		return time.Time{}
	}
	return b.EndTime.Time
}

// normalizeAirport strips whitespace and uppercases an airport/location
// code so "lhr" and " LHR " both compare equal to "LHR". Invalid inputs
// return "" (the caller treats those as unmatchable).
func normalizeAirport(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return strings.ToUpper(strings.TrimSpace(t.String))
}
