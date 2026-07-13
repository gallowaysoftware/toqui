//go:build integration

package integration

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgtype"

	toquiv1 "github.com/gallowaysoftware/toqui/backend/gen/toqui/v1"
	"github.com/gallowaysoftware/toqui/backend/internal/ai"
	"github.com/gallowaysoftware/toqui/backend/internal/auth"
	"github.com/gallowaysoftware/toqui/backend/internal/booking"
	"github.com/gallowaysoftware/toqui/backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui/backend/internal/handlers"
)

// cannedAIProvider returns a fixed parsed-booking JSON blob for any
// ChatStream call — enough to drive the real ingest pipeline end to end
// without a live model.
type cannedAIProvider struct{ json string }

func (p *cannedAIProvider) ChatStream(_ context.Context, _ *ai.ChatRequest) (<-chan ai.Event, error) {
	ch := make(chan ai.Event, 2)
	ch <- ai.Event{Type: ai.EventTextDelta, Text: p.json}
	close(ch)
	return ch, nil
}

func (p *cannedAIProvider) Name() string { return "canned" }

// TestIngestEmail_EndToEnd drives the IngestEmail RPC through the real
// pipeline: MIME parse → trip resolution (subject match) → AI parse (canned)
// → booking insert with source="email" → itinerary auto-link.
func TestIngestEmail_EndToEnd(t *testing.T) {
	env := NewTestEnv(t)
	env.CleanDB(t)
	ctx := context.Background()
	queries := dbgen.New(env.Pool)

	user, err := queries.UpsertUserByGoogleID(ctx, dbgen.UpsertUserByGoogleIDParams{
		GoogleID: pgtype.Text{String: "g_email_import", Valid: true},
		Email:    "traveler@example.com",
		Name:     pgtype.Text{String: "Traveler", Valid: true},
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Two trips: the subject names "Barcelona Trip", which must win over
	// the more-recently-created "Lisbon Trip".
	bcn, err := queries.CreateTrip(ctx, dbgen.CreateTripParams{UserID: user.ID, Title: "Barcelona Trip"})
	if err != nil {
		t.Fatalf("create barcelona: %v", err)
	}
	if _, err := queries.CreateTrip(ctx, dbgen.CreateTripParams{UserID: user.ID, Title: "Lisbon Trip"}); err != nil {
		t.Fatalf("create lisbon: %v", err)
	}

	parsedJSON := `{"type":"hotel","confirmation_code":"HTL-9988","provider":"Hotel Arts","title":"Hotel Arts Barcelona","start_time":"2027-05-02T15:00:00Z","address":"Barcelona"}`
	svc := booking.NewService(env.Pool, &cannedAIProvider{json: parsedJSON})
	h := handlers.NewBookingHandler(svc, queries)

	rawEmail := "From: Hotel Arts <res@hotelarts.com>\r\n" +
		"Subject: Your Barcelona Trip reservation is confirmed\r\n" +
		"Content-Type: text/plain\r\n\r\n" +
		"Confirmation HTL-9988\r\nHotel Arts Barcelona, check-in 2027-05-02.\r\n"

	authCtx := auth.ContextWithUserID(ctx, user.ID)
	resp, err := h.IngestEmail(authCtx, connect.NewRequest(&toquiv1.IngestEmailRequest{RawEmail: rawEmail}))
	if err != nil {
		t.Fatalf("IngestEmail: %v", err)
	}
	if len(resp.Msg.Bookings) != 1 {
		t.Fatalf("expected 1 booking, got %d", len(resp.Msg.Bookings))
	}
	b := resp.Msg.Bookings[0]
	if b.ConfirmationCode != "HTL-9988" {
		t.Errorf("confirmation code = %q", b.ConfirmationCode)
	}
	if b.TripId != bcn.ID.String() {
		t.Errorf("booking attached to trip %q, want Barcelona %q (subject match)", b.TripId, bcn.ID.String())
	}

	// Source must be "email".
	var source string
	if err := env.Pool.QueryRow(ctx, "SELECT source FROM bookings WHERE id = $1", b.Id).Scan(&source); err != nil {
		t.Fatalf("read source: %v", err)
	}
	if source != "email" {
		t.Errorf("booking source = %q, want email", source)
	}

	// An itinerary item was auto-linked to the booking.
	var itemCount int
	if err := env.Pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM itinerary_items WHERE booking_id = $1", b.Id,
	).Scan(&itemCount); err != nil {
		t.Fatalf("count itinerary items: %v", err)
	}
	if itemCount != 1 {
		t.Errorf("expected 1 auto-linked itinerary item, got %d", itemCount)
	}
}

// TestIngestEmail_EmptyBodyRejected verifies a message with no readable
// text is rejected with InvalidArgument rather than calling the AI.
func TestIngestEmail_EmptyBodyRejected(t *testing.T) {
	env := NewTestEnv(t)
	env.CleanDB(t)
	ctx := context.Background()
	queries := dbgen.New(env.Pool)

	user, err := queries.UpsertUserByGoogleID(ctx, dbgen.UpsertUserByGoogleIDParams{
		GoogleID: pgtype.Text{String: "g_empty_email", Valid: true},
		Email:    "empty@example.com",
		Name:     pgtype.Text{String: "Empty", Valid: true},
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Fail-loud AI provider: it must never be called for an empty body.
	svc := booking.NewService(env.Pool, &cannedAIProvider{json: `{"type":"other"}`})
	h := handlers.NewBookingHandler(svc, queries)

	rawEmail := "From: a@b.com\r\nSubject: nothing\r\nContent-Type: text/html\r\n\r\n<html><head></head><body></body></html>\r\n"
	authCtx := auth.ContextWithUserID(ctx, user.ID)
	_, err = h.IngestEmail(authCtx, connect.NewRequest(&toquiv1.IngestEmailRequest{RawEmail: rawEmail}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("empty body: got %v, want CodeInvalidArgument", err)
	}
}
