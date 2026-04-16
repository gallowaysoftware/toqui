//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui-backend/internal/trip"
)

func TestCollaboratorEditing(t *testing.T) {
	env := NewTestEnv(t)
	env.CleanDB(t)
	ctx := context.Background()
	queries := dbgen.New(env.Pool)
	tripSvc := trip.NewService(env.Pool)

	// Create owner and collaborator users
	owner, err := queries.UpsertUserByGoogleID(ctx, dbgen.UpsertUserByGoogleIDParams{
		GoogleID: pgtype.Text{String: "owner-google-111", Valid: true},
		Email:    "owner@example.com",
		Name:     pgtype.Text{String: "Owner", Valid: true},
	})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	editor, err := queries.UpsertUserByGoogleID(ctx, dbgen.UpsertUserByGoogleIDParams{
		GoogleID: pgtype.Text{String: "editor-google-222", Valid: true},
		Email:    "editor@example.com",
		Name:     pgtype.Text{String: "Editor", Valid: true},
	})
	if err != nil {
		t.Fatalf("create editor: %v", err)
	}
	viewer, err := queries.UpsertUserByGoogleID(ctx, dbgen.UpsertUserByGoogleIDParams{
		GoogleID: pgtype.Text{String: "viewer-google-333", Valid: true},
		Email:    "viewer@example.com",
		Name:     pgtype.Text{String: "Viewer", Valid: true},
	})
	if err != nil {
		t.Fatalf("create viewer: %v", err)
	}
	outsider, err := queries.UpsertUserByGoogleID(ctx, dbgen.UpsertUserByGoogleIDParams{
		GoogleID: pgtype.Text{String: "outsider-google-444", Valid: true},
		Email:    "outsider@example.com",
		Name:     pgtype.Text{String: "Outsider", Valid: true},
	})
	if err != nil {
		t.Fatalf("create outsider: %v", err)
	}

	// Create a trip owned by owner
	tr, err := tripSvc.Create(ctx, owner.ID, "Collab Trip", "Test collaboration", nil, nil)
	if err != nil {
		t.Fatalf("create trip: %v", err)
	}

	// Invite editor and viewer
	_, err = queries.AddCollaborator(ctx, dbgen.AddCollaboratorParams{
		TripID:      tr.ID,
		Email:       "editor@example.com",
		Role:        "editor",
		InviteToken: pgtype.Text{String: "editor-token-111", Valid: true},
		InvitedBy:   owner.ID,
	})
	if err != nil {
		t.Fatalf("add editor collaborator: %v", err)
	}
	_, err = queries.AcceptInvite(ctx, dbgen.AcceptInviteParams{
		InviteToken: pgtype.Text{String: "editor-token-111", Valid: true},
		UserID:      pgtype.UUID{Bytes: editor.ID, Valid: true},
	})
	if err != nil {
		t.Fatalf("accept editor invite: %v", err)
	}

	_, err = queries.AddCollaborator(ctx, dbgen.AddCollaboratorParams{
		TripID:      tr.ID,
		Email:       "viewer@example.com",
		Role:        "viewer",
		InviteToken: pgtype.Text{String: "viewer-token-222", Valid: true},
		InvitedBy:   owner.ID,
	})
	if err != nil {
		t.Fatalf("add viewer collaborator: %v", err)
	}
	_, err = queries.AcceptInvite(ctx, dbgen.AcceptInviteParams{
		InviteToken: pgtype.Text{String: "viewer-token-222", Valid: true},
		UserID:      pgtype.UUID{Bytes: viewer.ID, Valid: true},
	})
	if err != nil {
		t.Fatalf("accept viewer invite: %v", err)
	}

	t.Run("CanEditTrip", func(t *testing.T) {
		tests := []struct {
			name   string
			userID uuid.UUID
			want   bool
		}{
			{"owner can edit", owner.ID, true},
			{"editor can edit", editor.ID, true},
			{"viewer cannot edit", viewer.ID, false},
			{"outsider cannot edit", outsider.ID, false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got := tripSvc.CanEditTrip(ctx, tt.userID, tr.ID)
				if got != tt.want {
					t.Errorf("CanEditTrip(%s) = %v, want %v", tt.name, got, tt.want)
				}
			})
		}
	})

	t.Run("IsEditorCollaborator", func(t *testing.T) {
		tests := []struct {
			name   string
			userID uuid.UUID
			want   bool
		}{
			{"owner is not editor collaborator", owner.ID, false},
			{"editor is editor collaborator", editor.ID, true},
			{"viewer is not editor collaborator", viewer.ID, false},
			{"outsider is not editor collaborator", outsider.ID, false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got := tripSvc.IsEditorCollaborator(ctx, tt.userID, tr.ID)
				if got != tt.want {
					t.Errorf("IsEditorCollaborator(%s) = %v, want %v", tt.name, got, tt.want)
				}
			})
		}
	})

	t.Run("GetByIDOrCollaborator", func(t *testing.T) {
		tests := []struct {
			name    string
			userID  uuid.UUID
			wantErr bool
		}{
			{"owner can access", owner.ID, false},
			{"editor can access", editor.ID, false},
			{"viewer can access", viewer.ID, false},
			{"outsider cannot access", outsider.ID, true},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := tripSvc.GetByIDOrCollaborator(ctx, tt.userID, tr.ID)
				gotErr := err != nil
				if gotErr != tt.wantErr {
					t.Errorf("GetByIDOrCollaborator(%s): error=%v, wantErr=%v", tt.name, err, tt.wantErr)
				}
			})
		}
	})

	t.Run("EditorCanCreateAndDeleteItineraryItems", func(t *testing.T) {
		// Editor creates an itinerary item (no ownership check in CreateItineraryItem SQL)
		item, err := tripSvc.CreateItineraryItem(ctx, tr.ID, 1, 1, "activity", "Editor's Activity", "Added by editor")
		if err != nil {
			t.Fatalf("editor create itinerary item: %v", err)
		}

		// Editor deletes using owner-or-editor query
		deleted, err := tripSvc.DeleteItineraryItemsForOwnerOrEditor(ctx, editor.ID, []uuid.UUID{item.ID})
		if err != nil {
			t.Fatalf("editor delete itinerary item: %v", err)
		}
		if len(deleted) != 1 {
			t.Errorf("expected 1 deleted, got %d", len(deleted))
		}
	})

	t.Run("ViewerCannotDeleteItineraryItems", func(t *testing.T) {
		// Owner creates an item
		item, err := tripSvc.CreateItineraryItem(ctx, tr.ID, 2, 1, "activity", "Owner's Item", "For viewer test")
		if err != nil {
			t.Fatalf("create itinerary item: %v", err)
		}

		// Viewer tries to delete using owner-or-editor query. The SQL
		// filters on trip owner OR editor-role collaborator, so a
		// viewer matches nothing and zero rows are affected. Pre-#343
		// the service annotated the query :exec and reported a bogus
		// success for every ID; now it checks RowsAffected and returns
		// an empty "deleted" slice when authz denies the delete.
		deleted, err := tripSvc.DeleteItineraryItemsForOwnerOrEditor(ctx, viewer.ID, []uuid.UUID{item.ID})
		if err != nil {
			t.Fatalf("viewer delete should return nil error (zero rows is not an error): %v", err)
		}
		if len(deleted) != 0 {
			t.Errorf("viewer should not be able to delete items, got %d deleted", len(deleted))
		}

		// Sanity: the item should still be in the DB — handler/tool
		// success responses must never report deletion of rows that
		// Postgres refused to delete.
		itinerary, err := tripSvc.GetItinerary(ctx, tr.ID)
		if err != nil {
			t.Fatalf("get itinerary after viewer delete attempt: %v", err)
		}
		stillPresent := false
		for _, it := range itinerary {
			if it.ID == item.ID {
				stillPresent = true
				break
			}
		}
		if !stillPresent {
			t.Error("owner's item was deleted by viewer — authz bypass")
		}

		// Clean up: owner deletes
		_, _ = tripSvc.DeleteItineraryItems(ctx, owner.ID, []uuid.UUID{item.ID})
	})

	t.Run("OwnerCanDeleteItineraryItems", func(t *testing.T) {
		item, err := tripSvc.CreateItineraryItem(ctx, tr.ID, 3, 1, "activity", "Owner Delete Test", "")
		if err != nil {
			t.Fatalf("create itinerary item: %v", err)
		}

		deleted, err := tripSvc.DeleteItineraryItemsForOwnerOrEditor(ctx, owner.ID, []uuid.UUID{item.ID})
		if err != nil {
			t.Fatalf("owner delete: %v", err)
		}
		if len(deleted) != 1 {
			t.Errorf("expected 1 deleted, got %d", len(deleted))
		}
	})

	t.Run("EditorCanReplaceItinerary", func(t *testing.T) {
		// Editor replaces the entire itinerary
		items := []trip.ReplaceItineraryItem{
			{DayNumber: 1, OrderInDay: 1, Type: "activity", Title: "Editor Planned Activity"},
			{DayNumber: 1, OrderInDay: 2, Type: "meal", Title: "Editor Planned Lunch"},
		}
		if err := tripSvc.ReplaceItineraryForOwnerOrEditor(ctx, editor.ID, tr.ID, items); err != nil {
			t.Fatalf("editor replace itinerary: %v", err)
		}

		// Verify items were created
		got, err := tripSvc.GetItinerary(ctx, tr.ID)
		if err != nil {
			t.Fatalf("get itinerary: %v", err)
		}
		if len(got) != 2 {
			t.Errorf("expected 2 items, got %d", len(got))
		}
	})

	t.Run("ViewerCannotReplaceItinerary", func(t *testing.T) {
		// Capture the itinerary state BEFORE the viewer attempt so we
		// can assert it's untouched afterwards. The previous editor
		// Replace left the trip with exactly two items.
		before, err := tripSvc.GetItinerary(ctx, tr.ID)
		if err != nil {
			t.Fatalf("get itinerary before viewer attempt: %v", err)
		}
		if len(before) != 2 {
			t.Fatalf("precondition: expected 2 items before viewer attempt, got %d", len(before))
		}

		// Viewer tries to replace itinerary. Previously (#343) this
		// silently succeeded at the INSERT step because the delete
		// was a no-op (SQL filters by owner/editor) but
		// CreateItineraryItem had no authz check — the viewer could
		// append a new item to someone else's trip. The service now
		// pre-checks CanEditTrip and rejects with ErrNotOwnerOrEditor
		// before any write happens.
		items := []trip.ReplaceItineraryItem{
			{DayNumber: 1, OrderInDay: 1, Type: "activity", Title: "Viewer Attempt"},
		}
		err = tripSvc.ReplaceItineraryForOwnerOrEditor(ctx, viewer.ID, tr.ID, items)
		if err == nil {
			t.Fatal("viewer Replace should be rejected, got nil error")
		}
		if !errors.Is(err, trip.ErrNotOwnerOrEditor) {
			t.Errorf("expected trip.ErrNotOwnerOrEditor, got %v", err)
		}

		// The itinerary must be unchanged — no delete, no insert.
		after, err := tripSvc.GetItinerary(ctx, tr.ID)
		if err != nil {
			t.Fatalf("get itinerary after viewer attempt: %v", err)
		}
		if len(after) != 2 {
			t.Errorf("expected itinerary unchanged (2 items) after rejected viewer Replace, got %d", len(after))
		}
		// And specifically, the viewer's attempted title must not be
		// in the resulting itinerary.
		for _, it := range after {
			if it.Title.Valid && it.Title.String == "Viewer Attempt" {
				t.Error("viewer's attempted item leaked into itinerary")
			}
		}
	})

	t.Run("OutsiderCannotReplaceItinerary", func(t *testing.T) {
		// W1 from the #343 adversarial review: viewer has a
		// trip_collaborators row with role='viewer'; outsider has
		// none at all. Both paths must be rejected identically by
		// CanEditTrip. Pinning this covers a future SQL refactor
		// that might treat "no row" differently from "viewer row".
		before, err := tripSvc.GetItinerary(ctx, tr.ID)
		if err != nil {
			t.Fatalf("get itinerary before outsider attempt: %v", err)
		}
		beforeLen := len(before)

		items := []trip.ReplaceItineraryItem{
			{DayNumber: 1, OrderInDay: 1, Type: "activity", Title: "Outsider Attempt"},
		}
		err = tripSvc.ReplaceItineraryForOwnerOrEditor(ctx, outsider.ID, tr.ID, items)
		if err == nil {
			t.Fatal("outsider Replace should be rejected, got nil error")
		}
		if !errors.Is(err, trip.ErrNotOwnerOrEditor) {
			t.Errorf("expected trip.ErrNotOwnerOrEditor, got %v", err)
		}

		after, err := tripSvc.GetItinerary(ctx, tr.ID)
		if err != nil {
			t.Fatalf("get itinerary after outsider attempt: %v", err)
		}
		if len(after) != beforeLen {
			t.Errorf("expected itinerary unchanged (%d items) after rejected outsider Replace, got %d", beforeLen, len(after))
		}
	})

	t.Run("ReplaceOnNonExistentTripReturnsPermissionDenied", func(t *testing.T) {
		// W2 from the #343 adversarial review: pin the semantics
		// of a Replace against a trip the caller cannot edit
		// because the trip doesn't exist. CanEditTrip falls
		// through IsAcceptedCollaboratorWithRole on a bogus ID,
		// both return false, so we get ErrNotOwnerOrEditor rather
		// than a more specific "not found" signal. The RPC handler
		// masks this with a GetByIDOrCollaborator pre-check, but
		// pinning the service behaviour here catches any future
		// change that starts returning an error from CanEditTrip
		// (which would slip past the current errors.Is check).
		bogusTripID := uuid.New()
		items := []trip.ReplaceItineraryItem{
			{DayNumber: 1, OrderInDay: 1, Type: "activity", Title: "Ghost Trip"},
		}
		err := tripSvc.ReplaceItineraryForOwnerOrEditor(ctx, editor.ID, bogusTripID, items)
		if err == nil {
			t.Fatal("Replace on non-existent trip should be rejected, got nil error")
		}
		if !errors.Is(err, trip.ErrNotOwnerOrEditor) {
			t.Errorf("expected trip.ErrNotOwnerOrEditor, got %v", err)
		}
	})

	t.Run("BatchDeleteMixedAuthz", func(t *testing.T) {
		// W3 from the #343 adversarial review: the service loops
		// per-ID, so a batch mixing items the caller CAN delete
		// with items they CAN'T (e.g. a different trip they don't
		// collaborate on) must return exactly the IDs actually
		// removed — not the whole input. Guards against a future
		// regression where the return slice drifts back to "any
		// nil error counts as success".

		// Set up: owner creates an item on tr (editor has access)
		// and a second owner creates a separate trip with its own
		// item (editor has NO access).
		itemA, err := tripSvc.CreateItineraryItem(ctx, tr.ID, 8, 1, "activity", "Editor-accessible item", "")
		if err != nil {
			t.Fatalf("create itemA: %v", err)
		}

		otherOwner, err := queries.UpsertUserByGoogleID(ctx, dbgen.UpsertUserByGoogleIDParams{
			GoogleID: pgtype.Text{String: "other-owner-google-555", Valid: true},
			Email:    "other-owner@example.com",
			Name:     pgtype.Text{String: "Other Owner", Valid: true},
		})
		if err != nil {
			t.Fatalf("create other owner: %v", err)
		}
		otherTrip, err := tripSvc.Create(ctx, otherOwner.ID, "Other Trip", "Not shared with editor", nil, nil)
		if err != nil {
			t.Fatalf("create other trip: %v", err)
		}
		itemB, err := tripSvc.CreateItineraryItem(ctx, otherTrip.ID, 1, 1, "activity", "Editor-inaccessible item", "")
		if err != nil {
			t.Fatalf("create itemB: %v", err)
		}

		// Editor tries to delete both in one call.
		deleted, err := tripSvc.DeleteItineraryItemsForOwnerOrEditor(ctx, editor.ID, []uuid.UUID{itemA.ID, itemB.ID})
		if err != nil {
			t.Fatalf("mixed-authz delete: %v", err)
		}
		if len(deleted) != 1 {
			t.Fatalf("expected exactly 1 deleted (the accessible item), got %d: %v", len(deleted), deleted)
		}
		if deleted[0] != itemA.ID {
			t.Errorf("expected itemA (%s) in deleted, got %s", itemA.ID, deleted[0])
		}

		// Sanity: itemB must still exist on otherTrip.
		otherItin, err := tripSvc.GetItinerary(ctx, otherTrip.ID)
		if err != nil {
			t.Fatalf("get other itinerary: %v", err)
		}
		stillPresent := false
		for _, it := range otherItin {
			if it.ID == itemB.ID {
				stillPresent = true
				break
			}
		}
		if !stillPresent {
			t.Error("itemB on the unrelated trip was deleted — cross-trip authz bypass")
		}

		// Cleanup the leftover on otherTrip so later tests see a
		// deterministic global state.
		_, _ = tripSvc.DeleteItineraryItems(ctx, otherOwner.ID, []uuid.UUID{itemB.ID})
	})
}
