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

func TestDayNumberForBooking(t *testing.T) {
	cases := []struct {
		name      string
		trip      dbgen.Trip
		startTime pgtype.Timestamptz
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
			name:      "no booking start time falls back to day 1",
			trip:      mkTrip("2026-09-10", ""),
			startTime: mkStart(""),
			want:      1,
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
			name:      "invalid timezone falls back to UTC",
			trip:      mkTrip("2026-09-10", "Not/AZone"),
			startTime: mkStart("2026-09-12T02:00:00Z"),
			want:      3,
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
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := dayNumberForBooking(tc.trip, tc.startTime); got != tc.want {
				t.Errorf("dayNumberForBooking() = %d, want %d", got, tc.want)
			}
		})
	}
}
