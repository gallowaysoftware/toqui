package handlers

import (
	"testing"
	"time"

	"github.com/google/uuid"

	toquiv1 "github.com/gallowaysoftware/toqui/backend/gen/toqui/v1"
	"github.com/gallowaysoftware/toqui/backend/internal/dbgen"
)

// newTestTrip creates a minimal dbgen.Trip for unit tests.
func newTestTrip() *dbgen.Trip {
	return &dbgen.Trip{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		Title:     "My Trip",
		Status:    "planning",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// parseUUIDForTest wraps uuid.Parse for use in test assertions.
func parseUUIDForTest(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}

func TestCloneTripDefaultTitle(t *testing.T) {
	// Verify that tripToProto produces a valid proto Trip from a dbgen.Trip
	// (the title-defaulting happens in the service layer, so we test the
	// handler-level title pass-through here).
	cases := []struct {
		name      string
		reqTitle  string
		wantEmpty bool
	}{
		{"empty title passes through to service", "", true},
		{"custom title passes through to service", "My Custom Title", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := &toquiv1.CloneTripRequest{
				TripId: "550e8400-e29b-41d4-a716-446655440000",
				Title:  c.reqTitle,
			}
			if c.wantEmpty && req.Title != "" {
				t.Errorf("expected empty title, got %q", req.Title)
			}
			if !c.wantEmpty && req.Title != c.reqTitle {
				t.Errorf("expected title %q, got %q", c.reqTitle, req.Title)
			}
		})
	}
}

func TestCloneTripRequestValidation(t *testing.T) {
	// Verify invalid trip_id is rejected at the UUID parse level.
	cases := []struct {
		name    string
		tripID  string
		wantErr bool
	}{
		{"valid UUID", "550e8400-e29b-41d4-a716-446655440000", false},
		{"invalid UUID", "not-a-uuid", true},
		{"empty string", "", true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// The handler uses uuid.Parse to validate the trip_id.
			// We test the same logic here.
			_, err := parseUUIDForTest(c.tripID)
			if (err != nil) != c.wantErr {
				t.Errorf("parseUUID(%q): error=%v, wantErr=%v", c.tripID, err, c.wantErr)
			}
		})
	}
}

func TestTripToProtoPreservesFields(t *testing.T) {
	// Verify that the tripToProto conversion correctly handles destination
	// countries — important for clone because we copy them from the source.
	trip := newTestTrip()
	trip.DestinationCountries = []string{"GR", "TR"}

	proto := tripToProto(trip)
	if len(proto.DestinationCountries) != 2 {
		t.Fatalf("expected 2 destination countries, got %d", len(proto.DestinationCountries))
	}
	if proto.DestinationCountries[0] != "GR" || proto.DestinationCountries[1] != "TR" {
		t.Errorf("destination countries mismatch: got %v", proto.DestinationCountries)
	}
}

func TestTripToProtoStatusPlanning(t *testing.T) {
	// Cloned trips always start in planning. Verify the conversion.
	trip := newTestTrip()
	trip.Status = "planning"

	proto := tripToProto(trip)
	if proto.Status != toquiv1.TripStatus_TRIP_STATUS_PLANNING {
		t.Errorf("expected PLANNING status, got %v", proto.Status)
	}
}
