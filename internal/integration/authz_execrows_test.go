//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/gallowaysoftware/toqui-backend/internal/booking"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui-backend/internal/trip"
)

// TestAuthzExecRowsOwnerPaths pins the non-collaborator authz behaviour
// that #345 locked down. Before #345 the underlying SQL queries were
// annotated :exec (returns only error), so pgx's silent-zero-rows path
// made callers like DeleteItineraryItems, ReplaceItinerary, and booking.Delete
// cheerfully report success for rows that Postgres refused to touch (items or
// bookings owned by someone else). After #345 they are :execrows and the
// service layer surfaces the truth — the delete helpers skip non-owned IDs,
// ReplaceItinerary pre-checks ownership and rejects with
// trip.ErrNotOwnerOrEditor, and booking.Delete returns (deleted=false, nil).
func TestAuthzExecRowsOwnerPaths(t *testing.T) {
	env := NewTestEnv(t)
	env.CleanDB(t)
	ctx := context.Background()
	queries := dbgen.New(env.Pool)
	tripSvc := trip.NewService(env.Pool)
	// aiProvider is only needed for IngestText/ExtractField — the Delete
	// path is pure DB and safe to exercise with nil.
	bookingSvc := booking.NewService(env.Pool, nil)

	// Two unrelated users. "alice" owns the trip/items/bookings; "mallory"
	// tries to mutate them.
	alice, err := queries.UpsertUserByGoogleID(ctx, dbgen.UpsertUserByGoogleIDParams{
		GoogleID: pgtype.Text{String: "authz-alice-google", Valid: true},
		Email:    "authz-alice@example.com",
		Name:     pgtype.Text{String: "Alice", Valid: true},
	})
	if err != nil {
		t.Fatalf("create alice: %v", err)
	}
	mallory, err := queries.UpsertUserByGoogleID(ctx, dbgen.UpsertUserByGoogleIDParams{
		GoogleID: pgtype.Text{String: "authz-mallory-google", Valid: true},
		Email:    "authz-mallory@example.com",
		Name:     pgtype.Text{String: "Mallory", Valid: true},
	})
	if err != nil {
		t.Fatalf("create mallory: %v", err)
	}

	aliceTrip, err := tripSvc.Create(ctx, alice.ID, "Alice's Trip", "Authz test", nil, nil)
	if err != nil {
		t.Fatalf("create alice trip: %v", err)
	}

	t.Run("DeleteItineraryItemsSkipsForeignItems", func(t *testing.T) {
		// Alice creates an item on her trip; Mallory tries to delete
		// it. Before #345 the :exec query returned nil on 0 rows, and
		// DeleteItineraryItems appended the ID to the "deleted" slice
		// regardless — a silent authz bypass from the caller's
		// perspective (the AI tool told the user the item was gone
		// while it was still in the DB). Now the rows > 0 guard
		// keeps foreign IDs out of the returned slice.
		item, err := tripSvc.CreateItineraryItem(ctx, aliceTrip.ID, 1, 1, "activity", "Alice's Item", "")
		if err != nil {
			t.Fatalf("create alice item: %v", err)
		}

		deleted, err := tripSvc.DeleteItineraryItems(ctx, mallory.ID, []uuid.UUID{item.ID})
		if err != nil {
			t.Fatalf("mallory delete (zero rows should not be an error): %v", err)
		}
		if len(deleted) != 0 {
			t.Errorf("expected 0 deleted for foreign item, got %d: %v", len(deleted), deleted)
		}

		// Sanity: the item must still exist on Alice's trip.
		itinerary, err := tripSvc.GetItinerary(ctx, aliceTrip.ID)
		if err != nil {
			t.Fatalf("get itinerary: %v", err)
		}
		stillPresent := false
		for _, it := range itinerary {
			if it.ID == item.ID {
				stillPresent = true
				break
			}
		}
		if !stillPresent {
			t.Error("Alice's item was deleted by Mallory — authz bypass")
		}

		// Cleanup.
		_, _ = tripSvc.DeleteItineraryItems(ctx, alice.ID, []uuid.UUID{item.ID})
	})

	t.Run("DeleteItineraryItemsMixedOwnership", func(t *testing.T) {
		// A batch of two items where only one belongs to the caller.
		// The return slice must contain exactly the one Alice owns
		// (not Mallory's, even though no pgx error is surfaced for
		// the foreign ID). Pins the per-ID rows > 0 filter so a
		// future refactor can't drift back to "any nil counts as
		// success".
		aliceItem, err := tripSvc.CreateItineraryItem(ctx, aliceTrip.ID, 2, 1, "activity", "Alice Batch", "")
		if err != nil {
			t.Fatalf("create alice item: %v", err)
		}

		malloryTrip, err := tripSvc.Create(ctx, mallory.ID, "Mallory's Trip", "", nil, nil)
		if err != nil {
			t.Fatalf("create mallory trip: %v", err)
		}
		malloryItem, err := tripSvc.CreateItineraryItem(ctx, malloryTrip.ID, 1, 1, "activity", "Mallory Batch", "")
		if err != nil {
			t.Fatalf("create mallory item: %v", err)
		}

		// Alice asks to delete both (maybe via a malicious client that
		// supplied Mallory's UUID). She should only see her own item
		// in the result.
		deleted, err := tripSvc.DeleteItineraryItems(ctx, alice.ID, []uuid.UUID{aliceItem.ID, malloryItem.ID})
		if err != nil {
			t.Fatalf("mixed-ownership delete: %v", err)
		}
		if len(deleted) != 1 {
			t.Fatalf("expected 1 deleted, got %d: %v", len(deleted), deleted)
		}
		if deleted[0] != aliceItem.ID {
			t.Errorf("expected %s (Alice's) in deleted, got %s", aliceItem.ID, deleted[0])
		}

		// Mallory's item must still exist.
		malloryItin, err := tripSvc.GetItinerary(ctx, malloryTrip.ID)
		if err != nil {
			t.Fatalf("get mallory itinerary: %v", err)
		}
		stillPresent := false
		for _, it := range malloryItin {
			if it.ID == malloryItem.ID {
				stillPresent = true
				break
			}
		}
		if !stillPresent {
			t.Error("Mallory's item was deleted by Alice — cross-user authz bypass")
		}

		// Cleanup.
		_, _ = tripSvc.DeleteItineraryItems(ctx, mallory.ID, []uuid.UUID{malloryItem.ID})
	})

	t.Run("ReplaceItineraryRejectsNonOwner", func(t *testing.T) {
		// Put the trip in a known state first.
		baseline := []trip.ReplaceItineraryItem{
			{DayNumber: 1, OrderInDay: 1, Type: "activity", Title: "Alice Baseline"},
		}
		if err := tripSvc.ReplaceItinerary(ctx, alice.ID, aliceTrip.ID, baseline); err != nil {
			t.Fatalf("alice baseline replace: %v", err)
		}

		// Mallory tries to replace Alice's itinerary. Before #345 the
		// DELETE step would silently no-op (SQL filters by owner) but
		// CreateItineraryItem has no authz check — Mallory could
		// APPEND items to Alice's trip. Now the service pre-checks
		// ownership via GetTripByID and returns
		// trip.ErrNotOwnerOrEditor before any write touches the DB.
		mal := []trip.ReplaceItineraryItem{
			{DayNumber: 1, OrderInDay: 1, Type: "activity", Title: "Mallory Injection"},
		}
		err := tripSvc.ReplaceItinerary(ctx, mallory.ID, aliceTrip.ID, mal)
		if err == nil {
			t.Fatal("Mallory Replace should be rejected, got nil error")
		}
		if !errors.Is(err, trip.ErrNotOwnerOrEditor) {
			t.Errorf("expected trip.ErrNotOwnerOrEditor, got %v", err)
		}

		// Alice's baseline must be untouched — no delete, no insert.
		got, err := tripSvc.GetItinerary(ctx, aliceTrip.ID)
		if err != nil {
			t.Fatalf("get itinerary: %v", err)
		}
		if len(got) != 1 {
			t.Errorf("expected 1 item (untouched baseline), got %d", len(got))
		}
		for _, it := range got {
			if it.Title.Valid && it.Title.String == "Mallory Injection" {
				t.Error("Mallory's item leaked into Alice's itinerary")
			}
		}
	})

	t.Run("ReplaceItineraryRejectsGhostTrip", func(t *testing.T) {
		// Replace against a trip that doesn't exist at all must
		// return ErrNotOwnerOrEditor, not fall through to a
		// transaction that silently no-ops. Mirrors the collaborator
		// suite's ReplaceOnNonExistentTripReturnsPermissionDenied and
		// pins the GetTripByID+pgx.ErrNoRows branch.
		ghost := uuid.New()
		items := []trip.ReplaceItineraryItem{
			{DayNumber: 1, OrderInDay: 1, Type: "activity", Title: "Into the void"},
		}
		err := tripSvc.ReplaceItinerary(ctx, alice.ID, ghost, items)
		if err == nil {
			t.Fatal("Replace on non-existent trip should be rejected, got nil error")
		}
		if !errors.Is(err, trip.ErrNotOwnerOrEditor) {
			t.Errorf("expected trip.ErrNotOwnerOrEditor, got %v", err)
		}
	})

	t.Run("DeleteBookingReportsNoOpForForeignBooking", func(t *testing.T) {
		// Alice creates a booking on her trip. Mallory tries to
		// delete it. Before #345 the :exec query returned nil on 0
		// rows and the service's Delete returned nil error — the
		// handler saw "success" and returned an empty DeleteBookingResponse.
		// Now Delete returns (deleted=false, nil) so callers
		// (handlers, audit code, tests) can distinguish "booking
		// gone" from "booking belonged to someone else".
		b, err := queries.CreateBooking(ctx, dbgen.CreateBookingParams{
			UserID:           alice.ID,
			TripID:           pgtype.UUID{Bytes: aliceTrip.ID, Valid: true},
			Type:             "activity",
			ConfirmationCode: pgtype.Text{String: "AUTHZ-345-1", Valid: true},
			Provider:         pgtype.Text{String: "test", Valid: true},
			Title:            "Alice's Booking",
			Source:           "manual",
		})
		if err != nil {
			t.Fatalf("create booking: %v", err)
		}

		deleted, err := bookingSvc.Delete(ctx, mallory.ID, b.ID)
		if err != nil {
			t.Fatalf("mallory delete (zero rows is not an error): %v", err)
		}
		if deleted {
			t.Error("expected deleted=false for booking owned by another user")
		}

		// Sanity: the booking must still exist for Alice.
		got, err := queries.GetBookingByID(ctx, dbgen.GetBookingByIDParams{
			ID:     b.ID,
			UserID: alice.ID,
		})
		if err != nil {
			t.Fatalf("get booking after mallory delete attempt: %v", err)
		}
		if got.ID != b.ID {
			t.Errorf("got booking ID %v, want %v", got.ID, b.ID)
		}

		// Cleanup: Alice actually deletes.
		actuallyDeleted, err := bookingSvc.Delete(ctx, alice.ID, b.ID)
		if err != nil {
			t.Fatalf("alice delete: %v", err)
		}
		if !actuallyDeleted {
			t.Error("expected deleted=true for owner delete")
		}
	})

	t.Run("DeleteBookingReportsNoOpForGhostBooking", func(t *testing.T) {
		// Delete with a freshly minted UUID that has never been
		// inserted. Must return (false, nil) — idempotent at the
		// handler but honest at the service layer.
		deleted, err := bookingSvc.Delete(ctx, alice.ID, uuid.New())
		if err != nil {
			t.Fatalf("delete ghost booking: %v", err)
		}
		if deleted {
			t.Error("expected deleted=false for ghost booking UUID")
		}
	})
}
