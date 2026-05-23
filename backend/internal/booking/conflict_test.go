package booking

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/gallowaysoftware/toqui/backend/internal/dbgen"
)

// helper: build a flight booking
func mkFlight(t *testing.T, dep, arr string, depart, land time.Time) dbgen.Booking {
	t.Helper()
	return dbgen.Booking{
		ID:                uuid.New(),
		Type:              "flight",
		DepartureLocation: pgtype.Text{String: dep, Valid: dep != ""},
		ArrivalLocation:   pgtype.Text{String: arr, Valid: arr != ""},
		StartTime:         pgtype.Timestamptz{Time: depart, Valid: !depart.IsZero()},
		EndTime:           pgtype.Timestamptz{Time: land, Valid: !land.IsZero()},
	}
}

func mkHotel(t *testing.T, checkIn, checkOut time.Time) dbgen.Booking {
	t.Helper()
	return dbgen.Booking{
		ID:        uuid.New(),
		Type:      "hotel",
		StartTime: pgtype.Timestamptz{Time: checkIn, Valid: !checkIn.IsZero()},
		EndTime:   pgtype.Timestamptz{Time: checkOut, Valid: !checkOut.IsZero()},
	}
}

func mkCar(t *testing.T, pickup, dropoff time.Time) dbgen.Booking {
	t.Helper()
	return dbgen.Booking{
		ID:        uuid.New(),
		Type:      "car_rental",
		StartTime: pgtype.Timestamptz{Time: pickup, Valid: !pickup.IsZero()},
		EndTime:   pgtype.Timestamptz{Time: dropoff, Valid: !dropoff.IsZero()},
	}
}

// hasConflict returns true if any conflict in the slice has the given Type.
func hasConflict(conflicts []Conflict, typ string) bool {
	for _, c := range conflicts {
		if c.Type == typ {
			return true
		}
	}
	return false
}

func TestDetectConflicts_TightLayover(t *testing.T) {
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	// JFK→FRA lands at 12:00, FRA→ATH departs at 12:45 (45-min layover)
	a := mkFlight(t, "JFK", "FRA", base.Add(-7*time.Hour), base)
	b := mkFlight(t, "FRA", "ATH", base.Add(45*time.Minute), base.Add(45*time.Minute+3*time.Hour))

	got := DetectConflicts([]dbgen.Booking{a, b})
	if !hasConflict(got, ConflictTightLayover) {
		t.Errorf("expected tight_layover conflict, got %+v", got)
	}
}

func TestDetectConflicts_LayoverOverThreshold(t *testing.T) {
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	// 3-hour layover at FRA — should NOT flag
	a := mkFlight(t, "JFK", "FRA", base.Add(-7*time.Hour), base)
	b := mkFlight(t, "FRA", "ATH", base.Add(3*time.Hour), base.Add(3*time.Hour+3*time.Hour))

	got := DetectConflicts([]dbgen.Booking{a, b})
	if hasConflict(got, ConflictTightLayover) {
		t.Errorf("did not expect tight_layover for 3-hour layover, got %+v", got)
	}
}

func TestDetectConflicts_LayoverDifferentAirports(t *testing.T) {
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	// Lands at FRA, next departs from MUC — different airports, not a layover
	a := mkFlight(t, "JFK", "FRA", base.Add(-7*time.Hour), base)
	b := mkFlight(t, "MUC", "ATH", base.Add(45*time.Minute), base.Add(45*time.Minute+3*time.Hour))

	got := DetectConflicts([]dbgen.Booking{a, b})
	if hasConflict(got, ConflictTightLayover) {
		t.Errorf("did not expect tight_layover for different airports, got %+v", got)
	}
}

func TestDetectConflicts_LayoverCaseInsensitive(t *testing.T) {
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	// Same airport but different cases — should still flag
	a := mkFlight(t, "JFK", "fra", base.Add(-7*time.Hour), base)
	b := mkFlight(t, "FRA ", "ATH", base.Add(45*time.Minute), base.Add(45*time.Minute+3*time.Hour))

	got := DetectConflicts([]dbgen.Booking{a, b})
	if !hasConflict(got, ConflictTightLayover) {
		t.Errorf("expected tight_layover (case-insensitive match), got %+v", got)
	}
}

func TestDetectConflicts_HotelBeforeFlightLands(t *testing.T) {
	base := time.Date(2026, 6, 1, 18, 0, 0, 0, time.UTC)
	// Inbound lands at 18:00, hotel check-in at 16:00 — flag.
	// (Two flights so the detector has a credible inbound anchor — see
	// TestDetectConflicts_HotelBeforeFlightLands_SingleFlightNoFlag for
	// the single-flight regression.)
	inbound := mkFlight(t, "JFK", "CDG", base.Add(-8*time.Hour), base)
	outbound := mkFlight(t, "CDG", "JFK", base.Add(120*time.Hour), base.Add(128*time.Hour))
	hotel := mkHotel(t, base.Add(-2*time.Hour), base.Add(48*time.Hour))

	got := DetectConflicts([]dbgen.Booking{inbound, outbound, hotel})
	if !hasConflict(got, ConflictHotelBeforeArrival) {
		t.Errorf("expected hotel_before_arrival, got %+v", got)
	}
}

func TestDetectConflicts_HotelAfterFlightLands(t *testing.T) {
	base := time.Date(2026, 6, 1, 18, 0, 0, 0, time.UTC)
	// Inbound lands at 18:00, hotel check-in at 19:00 — fine.
	inbound := mkFlight(t, "JFK", "CDG", base.Add(-8*time.Hour), base)
	outbound := mkFlight(t, "CDG", "JFK", base.Add(120*time.Hour), base.Add(128*time.Hour))
	hotel := mkHotel(t, base.Add(time.Hour), base.Add(48*time.Hour))

	got := DetectConflicts([]dbgen.Booking{inbound, outbound, hotel})
	if hasConflict(got, ConflictHotelBeforeArrival) {
		t.Errorf("did not expect hotel_before_arrival, got %+v", got)
	}
}

// TestDetectConflicts_HotelBeforeFlightLands_SingleFlightNoFlag is the
// regression for the return-only-import false positive: a user who only
// forwarded their homebound flight will have hotel check-ins days before
// it. We must not flag every legitimate hotel as "before arrival" because
// we can't tell inbound from outbound with only one flight.
func TestDetectConflicts_HotelBeforeFlightLands_SingleFlightNoFlag(t *testing.T) {
	base := time.Date(2026, 6, 1, 18, 0, 0, 0, time.UTC)
	// Single flight + hotel check-in 5 days before flight start.
	// Could be inbound (would be a real conflict) or return-only
	// (would be a false positive). Detector stays quiet.
	flight := mkFlight(t, "JFK", "CDG", base.Add(-8*time.Hour), base)
	hotel := mkHotel(t, base.Add(-120*time.Hour), base.Add(-72*time.Hour))

	got := DetectConflicts([]dbgen.Booking{flight, hotel})
	if hasConflict(got, ConflictHotelBeforeArrival) {
		t.Errorf("did not expect hotel_before_arrival with a single flight, got %+v", got)
	}
}

func TestDetectConflicts_CarReturnAfterOutboundFlight(t *testing.T) {
	base := time.Date(2026, 6, 5, 14, 0, 0, 0, time.UTC)
	outbound := mkFlight(t, "CDG", "JFK", base, base.Add(8*time.Hour))
	inbound := mkFlight(t, "JFK", "CDG", base.Add(-96*time.Hour), base.Add(-88*time.Hour))
	// Car returned at 15:00, flight departs at 14:00 — error.
	car := mkCar(t, base.Add(-72*time.Hour), base.Add(time.Hour))

	got := DetectConflicts([]dbgen.Booking{inbound, outbound, car})
	if !hasConflict(got, ConflictCarReturnMismatch) {
		t.Errorf("expected car_return_mismatch (after flight), got %+v", got)
	}
}

func TestDetectConflicts_CarReturnTooCloseToFlight(t *testing.T) {
	base := time.Date(2026, 6, 5, 14, 0, 0, 0, time.UTC)
	inbound := mkFlight(t, "JFK", "CDG", base.Add(-96*time.Hour), base.Add(-88*time.Hour))
	outbound := mkFlight(t, "CDG", "JFK", base, base.Add(8*time.Hour))
	// Car returned 15 min before outbound flight (within 30-min buffer) — warn.
	car := mkCar(t, base.Add(-72*time.Hour), base.Add(-15*time.Minute))

	got := DetectConflicts([]dbgen.Booking{inbound, outbound, car})
	if !hasConflict(got, ConflictCarReturnMismatch) {
		t.Errorf("expected car_return_mismatch (within buffer), got %+v", got)
	}
}

func TestDetectConflicts_CarReturnComfortablyBeforeFlight(t *testing.T) {
	base := time.Date(2026, 6, 5, 14, 0, 0, 0, time.UTC)
	inbound := mkFlight(t, "JFK", "CDG", base.Add(-96*time.Hour), base.Add(-88*time.Hour))
	outbound := mkFlight(t, "CDG", "JFK", base, base.Add(8*time.Hour))
	// Car returned 3 hours before outbound flight — fine.
	car := mkCar(t, base.Add(-72*time.Hour), base.Add(-3*time.Hour))

	got := DetectConflicts([]dbgen.Booking{inbound, outbound, car})
	if hasConflict(got, ConflictCarReturnMismatch) {
		t.Errorf("did not expect car_return_mismatch with 3h buffer, got %+v", got)
	}
}

// TestDetectConflicts_CarReturnMismatch_SingleFlightNoFlag is the
// regression for the inbound-only-import false positive: a user with a
// single inbound flight and a car rental for the rest of the trip would
// have the car ending after the (in-the-past) inbound start, which the
// old code reported as an error-severity car_return_mismatch.
func TestDetectConflicts_CarReturnMismatch_SingleFlightNoFlag(t *testing.T) {
	base := time.Date(2026, 6, 1, 14, 0, 0, 0, time.UTC)
	// Inbound flight on day 1, car picked up on day 1, returned on day 8.
	// "Latest" flight is the inbound — without a return flight to anchor
	// the outbound, the detector must stay quiet.
	inbound := mkFlight(t, "JFK", "CDG", base.Add(-8*time.Hour), base)
	car := mkCar(t, base.Add(time.Hour), base.Add(168*time.Hour))

	got := DetectConflicts([]dbgen.Booking{inbound, car})
	if hasConflict(got, ConflictCarReturnMismatch) {
		t.Errorf("did not expect car_return_mismatch with a single flight, got %+v", got)
	}
}

func TestDetectConflicts_MissingReturnFlight(t *testing.T) {
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	flight := mkFlight(t, "JFK", "CDG", base.Add(-8*time.Hour), base)

	got := DetectConflicts([]dbgen.Booking{flight})
	if !hasConflict(got, ConflictMissingReturnFlight) {
		t.Errorf("expected missing_return_flight on lone flight, got %+v", got)
	}
}

func TestDetectConflicts_RoundTripNoMissingFlightWarning(t *testing.T) {
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	out := mkFlight(t, "JFK", "CDG", base.Add(-8*time.Hour), base)
	ret := mkFlight(t, "CDG", "JFK", base.Add(120*time.Hour), base.Add(128*time.Hour))

	got := DetectConflicts([]dbgen.Booking{out, ret})
	if hasConflict(got, ConflictMissingReturnFlight) {
		t.Errorf("did not expect missing_return_flight on round-trip, got %+v", got)
	}
}

func TestDetectConflicts_EmptyBookings(t *testing.T) {
	got := DetectConflicts(nil)
	if got != nil {
		t.Errorf("expected nil for empty input, got %+v", got)
	}
	got = DetectConflicts([]dbgen.Booking{})
	if got != nil {
		t.Errorf("expected nil for empty slice, got %+v", got)
	}
}

func TestDetectConflicts_SkipsBookingsWithMissingTimes(t *testing.T) {
	// Booking with no times should not panic and should not produce a conflict.
	b := dbgen.Booking{
		ID:   uuid.New(),
		Type: "flight",
	}
	got := DetectConflicts([]dbgen.Booking{b})
	for _, c := range got {
		if c.Type == ConflictTightLayover || c.Type == ConflictHotelBeforeArrival || c.Type == ConflictCarReturnMismatch {
			t.Errorf("did not expect time-based conflict on time-less booking, got %+v", c)
		}
	}
}

// TestDetectConflicts_TightLayover_BreakBugRegression covers the case
// where the previous implementation sorted flights by arrival time and
// broke the inner loop on the first non-tight pair. Because the inner
// loop iterated by arrival rather than departure, the early break could
// skip a real tight-layover from a later-arriving flight that departs
// immediately after an earlier landing. This test would have failed
// against that implementation.
func TestDetectConflicts_TightLayover_BreakBugRegression(t *testing.T) {
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	// A lands at FRA at 12:00.
	a := mkFlight(t, "JFK", "FRA", base.Add(-7*time.Hour), base)
	// B is a different flight that conflicts with nothing — it lands at
	// JFK at 12:30, sorted second by arrival. The old code would break
	// the inner loop here on a non-matching airport check still happens
	// to land first, but the *delta-based* break is the bug surface.
	b := mkFlight(t, "MAD", "JFK", base.Add(-1*time.Hour), base.Add(30*time.Minute))
	// C is the smoking gun: sorted last by arrival (lands at LHR at
	// 14:00, departing FRA at 12:15) — under a 90-min layover. The old
	// code's break-on-delta-≥-90 fired against B (delta = 13:00 − 12:00
	// = 60min for B's irrelevant departure) before reaching C.
	c := mkFlight(t, "FRA", "LHR", base.Add(15*time.Minute), base.Add(2*time.Hour))

	got := DetectConflicts([]dbgen.Booking{a, b, c})
	if !hasConflict(got, ConflictTightLayover) {
		t.Errorf("expected tight_layover (A→C via FRA, 15min) despite B sorted between, got %+v", got)
	}
}

// TestDetectConflicts_TightLayover_EqualEndTimes covers the symmetric-
// pair gap when two flights have identical end_times. The previous
// stable-sort + j>i iteration could miss the pair entirely because
// neither flight was unambiguously "first to land". Iterating both
// directions catches it.
func TestDetectConflicts_TightLayover_EqualEndTimes(t *testing.T) {
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	// Both flights end at the exact same instant. A lands at FRA, B
	// departs FRA 30 min later.
	a := mkFlight(t, "JFK", "FRA", base.Add(-7*time.Hour), base)
	b := mkFlight(t, "FRA", "ATH", base.Add(30*time.Minute), base)

	got := DetectConflicts([]dbgen.Booking{a, b})
	if !hasConflict(got, ConflictTightLayover) {
		t.Errorf("expected tight_layover for equal-end_times pair, got %+v", got)
	}
}

// TestDetectConflicts_MissingReturnFlight_SkipsZeroUUID guards against
// emitting a conflict whose BookingIDs is the zero UUID — that would
// surface "00000000-..." in the frontend and make any downstream
// click-through useless.
func TestDetectConflicts_MissingReturnFlight_SkipsZeroUUID(t *testing.T) {
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	// Flight with a deliberately-zero ID (uuid.Nil).
	flight := dbgen.Booking{
		// ID left at zero value: uuid.Nil
		Type:              "flight",
		DepartureLocation: pgtype.Text{String: "JFK", Valid: true},
		ArrivalLocation:   pgtype.Text{String: "CDG", Valid: true},
		StartTime:         pgtype.Timestamptz{Time: base.Add(-8 * time.Hour), Valid: true},
		EndTime:           pgtype.Timestamptz{Time: base, Valid: true},
	}
	got := DetectConflicts([]dbgen.Booking{flight})
	if hasConflict(got, ConflictMissingReturnFlight) {
		t.Errorf("did not expect missing_return_flight with zero-UUID booking, got %+v", got)
	}
}

func TestDetectConflicts_BookingIDsPopulated(t *testing.T) {
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	a := mkFlight(t, "JFK", "FRA", base.Add(-7*time.Hour), base)
	b := mkFlight(t, "FRA", "ATH", base.Add(45*time.Minute), base.Add(45*time.Minute+3*time.Hour))

	got := DetectConflicts([]dbgen.Booking{a, b})
	for _, c := range got {
		if c.Type != ConflictTightLayover {
			continue
		}
		if len(c.BookingIDs) != 2 {
			t.Errorf("expected 2 booking IDs in tight_layover, got %v", c.BookingIDs)
		}
	}
}
