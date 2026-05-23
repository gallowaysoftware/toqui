package booking

import (
	"time"

	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
)

// CoverageGap describes one missing piece of a trip plan that the AI
// should proactively prompt the user about. The analyzer returns at most
// ONE gap — the highest-priority issue — to avoid drowning the user in
// suggestions after every booking import.
type CoverageGap struct {
	// Type is a stable machine-readable identifier so the frontend can
	// pick an icon, copy, or a tool to invoke (e.g. "search hotels").
	Type string `json:"type"`

	// Priority is 1..4 (1 = highest). The analyzer returns the gap with
	// the lowest priority number when multiple are present.
	Priority int `json:"priority"`

	// SuggestionHint is a short user-facing string the AI can paraphrase
	// when prompting the user. Must NOT contain destination names,
	// hotel names, dates, or any other travel content beyond the
	// generic "you have X but no Y" framing.
	SuggestionHint string `json:"suggestion_hint"`
}

const (
	GapNoAccommodation   = "no_accommodation"
	GapNoReturnTransport = "no_return_transport"
	GapNoGroundTransport = "no_ground_transport"
	GapSparseItinerary   = "sparse_itinerary"
)

// AnalyzeCoverage examines a trip's bookings (and a count of itinerary
// items) and returns the single highest-priority gap, or nil if the
// trip looks well-covered or there's not enough data to suggest
// anything yet.
//
// Pure function — no DB calls, no clock. Caller is responsible for
// providing a snapshot of trip + bookings + itinerary count.
//
// Detectors run in priority order; the first one to fire wins. New
// detectors should be inserted at the priority level matching their
// "how blocking is this for the trip" weight.
func AnalyzeCoverage(trip dbgen.Trip, bookings []dbgen.Booking, itineraryItemCount int) *CoverageGap {
	// A brand-new trip with nothing booked has nothing to suggest yet —
	// telling the user "you have nothing booked!" right after they open
	// the app is noise, not signal. Only suggest once the user has
	// started building the trip.
	if len(bookings) == 0 {
		return nil
	}

	hasFlight := false
	hasAccommodation := false
	hasGroundTransport := false
	for _, b := range bookings {
		switch b.Type {
		case "flight":
			hasFlight = true
		case "hotel", "vacation_rental", "hostel":
			hasAccommodation = true
		case "car_rental", "transfer", "train", "bus":
			hasGroundTransport = true
		}
	}

	// Priority 1: has flights but no place to sleep. Most blocking.
	if hasFlight && !hasAccommodation {
		return &CoverageGap{
			Type:           GapNoAccommodation,
			Priority:       1,
			SuggestionHint: "You have flights booked but no accommodation — want me to find some places to stay?",
		}
	}

	// Priority 2: has accommodation AND at least one flight, but only
	// one. Common after a one-way flight import; the user usually wants
	// the loop closed. Gating on hasFlight (not just hasAccommodation)
	// avoids the false positive where a user with accommodation + train
	// gets told "find a return flight" — they may not be flying at all.
	// Detecting missing return transport for non-flight modes is a
	// follow-up; we'd rather stay quiet than nag with the wrong hint.
	if hasFlight && hasAccommodation && countFlights(bookings) < 2 {
		return &CoverageGap{
			Type:           GapNoReturnTransport,
			Priority:       2,
			SuggestionHint: "You have accommodation booked but no return flight — want me to look for one?",
		}
	}

	// Priority 3: has flight + accommodation but no ground transport.
	// Soft suggestion — many destinations don't need a car. Only fire
	// if the user has both anchors in place.
	if hasFlight && hasAccommodation && !hasGroundTransport {
		return &CoverageGap{
			Type:           GapNoGroundTransport,
			Priority:       3,
			SuggestionHint: "You have flights and accommodation but nothing for getting around — want me to suggest car rentals or transfers?",
		}
	}

	// Priority 4: trip has time on the calendar but the itinerary is
	// sparse. Lowest priority — feels like nagging if fired too early.
	if days := tripDays(trip); days >= 3 {
		if itineraryItemCount < days/2 {
			return &CoverageGap{
				Type:           GapSparseItinerary,
				Priority:       4,
				SuggestionHint: "Several days of your trip don't have anything planned yet — want me to suggest some activities?",
			}
		}
	}

	return nil
}

// countFlights returns the number of flight bookings.
func countFlights(bookings []dbgen.Booking) int {
	n := 0
	for _, b := range bookings {
		if b.Type == "flight" {
			n++
		}
	}
	return n
}

// tripDays returns the number of days the trip covers (end_date -
// start_date + 1) or 0 if dates aren't set. We extract the calendar
// (Y, M, D) components and rebuild both ends at midnight UTC before
// subtracting, so DST transitions, leap-second windows, or a future
// driver change that hands us a non-UTC pgtype.Date.Time can't shift
// the answer by ±1 day.
func tripDays(trip dbgen.Trip) int {
	if !trip.StartDate.Valid || !trip.EndDate.Valid {
		return 0
	}
	sy, sm, sd := trip.StartDate.Time.Date()
	ey, em, ed := trip.EndDate.Time.Date()
	start := time.Date(sy, sm, sd, 0, 0, 0, 0, time.UTC)
	end := time.Date(ey, em, ed, 0, 0, 0, 0, time.UTC)
	if end.Before(start) {
		return 0
	}
	// +1 because a trip from June 1 to June 1 is 1 day, not 0.
	return int(end.Sub(start)/(24*time.Hour)) + 1
}
