package handlers

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/gallowaysoftware/toqui/backend/internal/dbgen"
)

func mkTrip(startDate string, timezone string) dbgen.Trip {
	var t dbgen.Trip
	if startDate != "" {
		d, err := time.Parse("2006-01-02", startDate)
		if err != nil {
			panic(err)
		}
		t.StartDate = pgtype.Date{Time: d, Valid: true}
	}
	if timezone != "" {
		t.Timezone = pgtype.Text{String: timezone, Valid: true}
	}
	return t
}

func mkStart(rfc3339 string) pgtype.Timestamptz {
	if rfc3339 == "" {
		return pgtype.Timestamptz{}
	}
	ts, err := time.Parse(time.RFC3339, rfc3339)
	if err != nil {
		panic(err)
	}
	return pgtype.Timestamptz{Time: ts, Valid: true}
}

func mkTZ(tz string) pgtype.Text {
	if tz == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: tz, Valid: true}
}

func TestDayNumberForBooking(t *testing.T) {
	cases := []struct {
		name      string
		trip      dbgen.Trip
		startTime pgtype.Timestamptz
		bookingTZ pgtype.Text
		want      int32
	}{
		{
			name:      "first day",
			trip:      mkTrip("2026-09-10", ""),
			startTime: mkStart("2026-09-10T14:00:00Z"),
			want:      1,
		},
		{
			name:      "third day",
			trip:      mkTrip("2026-09-10", ""),
			startTime: mkStart("2026-09-12T09:30:00Z"),
			want:      3,
		},
		{
			name:      "no trip start date falls back to day 1",
			trip:      mkTrip("", ""),
			startTime: mkStart("2026-09-12T09:30:00Z"),
			want:      1,
		},
		{
			// No parsed start time → the "Unscheduled" (day 0) bucket
			// that itineraryToProto renders, not an asserted day.
			name:      "no booking start time stays unscheduled",
			trip:      mkTrip("2026-09-10", ""),
			startTime: mkStart(""),
			want:      0,
		},
		{
			name:      "booking before trip start clamps to day 1",
			trip:      mkTrip("2026-09-10", ""),
			startTime: mkStart("2026-09-08T10:00:00Z"),
			want:      1,
		},
		{
			// 23:30 UTC on Sep 11 is already Sep 12 in Tokyo (UTC+9):
			// without the trip timezone this would land on day 2.
			name:      "trip timezone shifts the calendar date forward",
			trip:      mkTrip("2026-09-10", "Asia/Tokyo"),
			startTime: mkStart("2026-09-11T23:30:00Z"),
			want:      3,
		},
		{
			// 02:00 UTC on Sep 12 is still Sep 11 in Vancouver (UTC-7):
			// without the trip timezone this would land on day 3.
			name:      "trip timezone shifts the calendar date backward",
			trip:      mkTrip("2026-09-10", "America/Vancouver"),
			startTime: mkStart("2026-09-12T02:00:00Z"),
			want:      2,
		},
		{
			// The booking's own timezone wins over the trip's: a flight
			// leaving Toronto at 20:00 EDT (= next morning JST) belongs
			// on the Toronto calendar day printed on the ticket, even
			// for a Tokyo trip.
			name:      "booking timezone wins over trip timezone",
			trip:      mkTrip("2026-09-10", "Asia/Tokyo"),
			startTime: mkStart("2026-09-11T00:00:00Z"), // Sep 10 20:00 EDT, Sep 11 09:00 JST
			bookingTZ: mkTZ("America/Toronto"),
			want:      1,
		},
		{
			name:      "invalid timezone falls back to UTC",
			trip:      mkTrip("2026-09-10", "Not/AZone"),
			startTime: mkStart("2026-09-12T02:00:00Z"),
			want:      3,
		},
		{
			// Invalid booking tz falls through to the trip tz, not UTC.
			name:      "invalid booking timezone falls back to trip timezone",
			trip:      mkTrip("2026-09-10", "America/Vancouver"),
			startTime: mkStart("2026-09-12T02:00:00Z"),
			bookingTZ: mkTZ("Not/AZone"),
			want:      2,
		},
		{
			// Offsets in the booking timestamp itself must not matter —
			// only the instant does. 2026-09-12T08:00+09:00 is
			// 2026-09-11T23:00Z, i.e. day 2 in UTC.
			name:      "booking timestamp offset is normalised",
			trip:      mkTrip("2026-09-10", ""),
			startTime: mkStart("2026-09-12T08:00:00+09:00"),
			want:      2,
		},
		{
			// Exactly local midnight belongs to the day that starts.
			name:      "booking at local midnight in trip timezone",
			trip:      mkTrip("2026-09-10", "Asia/Tokyo"),
			startTime: mkStart("2026-09-12T00:00:00+09:00"),
			want:      3,
		},
		{
			// DST fall-back (America/Vancouver leaves DST 2026-11-01):
			// day math must count calendar days, not 24h blocks — the
			// 25-hour day between start and booking must not skew it.
			name:      "trip spanning a DST transition counts calendar days",
			trip:      mkTrip("2026-10-31", "America/Vancouver"),
			startTime: mkStart("2026-11-02T10:00:00-08:00"),
			want:      3,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := dayNumberForBooking(tc.trip, tc.startTime, tc.bookingTZ); got != tc.want {
				t.Errorf("dayNumberForBooking() = %d, want %d", got, tc.want)
			}
		})
	}
}
