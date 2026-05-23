package booking

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/gallowaysoftware/toqui/backend/internal/dbgen"
)

func bk(typ string) dbgen.Booking {
	return dbgen.Booking{ID: uuid.New(), Type: typ}
}

func mkTrip(t *testing.T, startDate, endDate time.Time) dbgen.Trip {
	t.Helper()
	return dbgen.Trip{
		ID:        uuid.New(),
		StartDate: pgtype.Date{Time: startDate, Valid: !startDate.IsZero()},
		EndDate:   pgtype.Date{Time: endDate, Valid: !endDate.IsZero()},
	}
}

func TestAnalyzeCoverage_EmptyBookings(t *testing.T) {
	trip := mkTrip(t, time.Now(), time.Now().Add(72*time.Hour))
	got := AnalyzeCoverage(trip, nil, 0)
	if got != nil {
		t.Errorf("expected nil for empty bookings, got %+v", got)
	}
	got = AnalyzeCoverage(trip, []dbgen.Booking{}, 0)
	if got != nil {
		t.Errorf("expected nil for empty bookings slice, got %+v", got)
	}
}

func TestAnalyzeCoverage_FlightNoAccommodation(t *testing.T) {
	trip := mkTrip(t, time.Now(), time.Now().Add(72*time.Hour))
	got := AnalyzeCoverage(trip, []dbgen.Booking{bk("flight")}, 0)
	if got == nil {
		t.Fatalf("expected no_accommodation gap, got nil")
	}
	if got.Type != GapNoAccommodation {
		t.Errorf("expected no_accommodation, got %+v", got)
	}
	if got.Priority != 1 {
		t.Errorf("expected priority 1 for no_accommodation, got %d", got.Priority)
	}
}

func TestAnalyzeCoverage_AccommodationButNoReturnFlight(t *testing.T) {
	trip := mkTrip(t, time.Now(), time.Now().Add(72*time.Hour))
	got := AnalyzeCoverage(trip, []dbgen.Booking{bk("flight"), bk("hotel")}, 0)
	if got == nil {
		t.Fatalf("expected no_return_transport gap, got nil")
	}
	if got.Type != GapNoReturnTransport {
		t.Errorf("expected no_return_transport, got %+v", got)
	}
	if got.Priority != 2 {
		t.Errorf("expected priority 2 for no_return_transport, got %d", got.Priority)
	}
}

func TestAnalyzeCoverage_RoundTripNoGroundTransport(t *testing.T) {
	trip := mkTrip(t, time.Now(), time.Now().Add(72*time.Hour))
	bookings := []dbgen.Booking{bk("flight"), bk("flight"), bk("hotel")}
	got := AnalyzeCoverage(trip, bookings, 0)
	if got == nil {
		t.Fatalf("expected no_ground_transport gap, got nil")
	}
	if got.Type != GapNoGroundTransport {
		t.Errorf("expected no_ground_transport, got %+v", got)
	}
	if got.Priority != 3 {
		t.Errorf("expected priority 3, got %d", got.Priority)
	}
}

// TestAnalyzeCoverage_AccommodationOnlyNoFlightSilent regresses the
// no_return_transport false positive: a user with only non-flight
// transport (e.g. accommodation + train) was being told to "find a
// return flight" even though they have no flights at all.
func TestAnalyzeCoverage_AccommodationOnlyNoFlightSilent(t *testing.T) {
	trip := mkTrip(t, time.Now(), time.Now().Add(72*time.Hour))
	bookings := []dbgen.Booking{bk("hotel"), bk("train")}
	got := AnalyzeCoverage(trip, bookings, 1)
	if got != nil && got.Type == GapNoReturnTransport {
		t.Errorf("did not expect no_return_transport hint when user has no flights, got %+v", got)
	}
}

// TestAnalyzeCoverage_HotelAndCarOnlyNoFlightSilent — the road-trip
// case. User booked accommodation and a car, no flights. Old code fired
// no_return_transport because hasAccommodation && countFlights<2.
func TestAnalyzeCoverage_HotelAndCarOnlyNoFlightSilent(t *testing.T) {
	trip := mkTrip(t, time.Now(), time.Now().Add(120*time.Hour))
	bookings := []dbgen.Booking{bk("hotel"), bk("car_rental")}
	got := AnalyzeCoverage(trip, bookings, 1)
	if got != nil && got.Type == GapNoReturnTransport {
		t.Errorf("did not expect no_return_transport on a road trip, got %+v", got)
	}
}

func TestAnalyzeCoverage_FullyCoveredShortTrip(t *testing.T) {
	// 3-day trip: 2 flights + hotel + car. Half of 3 = 1, so even 1
	// itinerary item should clear the sparse threshold.
	trip := mkTrip(t, time.Now(), time.Now().Add(48*time.Hour))
	bookings := []dbgen.Booking{bk("flight"), bk("flight"), bk("hotel"), bk("car_rental")}
	got := AnalyzeCoverage(trip, bookings, 2)
	if got != nil {
		t.Errorf("expected nil for fully-covered trip, got %+v", got)
	}
}

func TestAnalyzeCoverage_SparseItinerary(t *testing.T) {
	// 7-day trip, all transport/lodging covered, but only 1 itinerary
	// item planned — sparse.
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC)
	trip := mkTrip(t, start, end)
	bookings := []dbgen.Booking{bk("flight"), bk("flight"), bk("hotel"), bk("car_rental")}
	got := AnalyzeCoverage(trip, bookings, 1)
	if got == nil || got.Type != GapSparseItinerary {
		t.Errorf("expected sparse_itinerary, got %+v", got)
	}
}

func TestAnalyzeCoverage_NotSparseEnough(t *testing.T) {
	// 4-day trip, all transport/lodging covered, 2 itinerary items —
	// half the trip is planned, not sparse.
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC)
	trip := mkTrip(t, start, end)
	bookings := []dbgen.Booking{bk("flight"), bk("flight"), bk("hotel"), bk("car_rental")}
	got := AnalyzeCoverage(trip, bookings, 2)
	if got != nil {
		t.Errorf("expected nil for half-planned trip, got %+v", got)
	}
}

func TestAnalyzeCoverage_ShortTripSkipsSparseCheck(t *testing.T) {
	// 2-day trip — too short for the sparse-itinerary check to fire.
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)
	trip := mkTrip(t, start, end)
	bookings := []dbgen.Booking{bk("flight"), bk("flight"), bk("hotel"), bk("car_rental")}
	got := AnalyzeCoverage(trip, bookings, 0)
	if got != nil {
		t.Errorf("expected nil for short well-covered trip, got %+v", got)
	}
}

func TestAnalyzeCoverage_PrioritizesNoAccommodationOverGroundTransport(t *testing.T) {
	// Both gaps would fire — but no_accommodation (priority 1) wins.
	trip := mkTrip(t, time.Now(), time.Now().Add(72*time.Hour))
	bookings := []dbgen.Booking{bk("flight")}
	got := AnalyzeCoverage(trip, bookings, 0)
	if got == nil || got.Type != GapNoAccommodation {
		t.Errorf("expected priority-1 no_accommodation to win, got %+v", got)
	}
}

func TestAnalyzeCoverage_VacationRentalCountsAsAccommodation(t *testing.T) {
	trip := mkTrip(t, time.Now(), time.Now().Add(72*time.Hour))
	bookings := []dbgen.Booking{bk("flight"), bk("vacation_rental")}
	got := AnalyzeCoverage(trip, bookings, 0)
	if got == nil || got.Type != GapNoReturnTransport {
		t.Errorf("expected no_return_transport (vacation_rental counts as accommodation), got %+v", got)
	}
}

func TestAnalyzeCoverage_TrainCountsAsGroundTransport(t *testing.T) {
	// Round-trip flight + hotel + train → covered, no gap.
	trip := mkTrip(t, time.Now(), time.Now().Add(48*time.Hour))
	bookings := []dbgen.Booking{bk("flight"), bk("flight"), bk("hotel"), bk("train")}
	got := AnalyzeCoverage(trip, bookings, 1)
	if got != nil {
		t.Errorf("expected nil (train satisfies ground transport), got %+v", got)
	}
}

func TestAnalyzeCoverage_TripWithoutDatesSkipsSparseCheck(t *testing.T) {
	// Trip with no dates → tripDays returns 0 → sparse check is skipped.
	trip := dbgen.Trip{ID: uuid.New()}
	bookings := []dbgen.Booking{bk("flight"), bk("flight"), bk("hotel"), bk("car_rental")}
	got := AnalyzeCoverage(trip, bookings, 0)
	if got != nil {
		t.Errorf("expected nil for date-less covered trip, got %+v", got)
	}
}
