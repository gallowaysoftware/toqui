//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/gallowaysoftware/toqui/backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui/backend/internal/trip"
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
				got, err := tripSvc.CanEditTrip(ctx, tt.userID, tr.ID)
				if err != nil {
					t.Fatalf("CanEditTrip(%s) unexpected error: %v", tt.name, err)
				}
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
				got, err := tripSvc.IsEditorCollaborator(ctx, tt.userID, tr.ID)
				if err != nil {
					t.Fatalf("IsEditorCollaborator(%s) unexpected error: %v", tt.name, err)
				}
				if got != tt.want {
					t.Errorf("IsEditorCollaborator(%s) = %v, want %v", tt.name, got, tt.want)
				}
			})
		}
	})

	t.Run("OwnerOnlyToolGate_UsesUserIDComparison", func(t *testing.T) {
		// The chat handler gates owner-only tools (update_trip: title,
		// description, status, destinations — #263) by comparing the
		// authenticated userID against the trip row's UserID, NOT by
		// calling CanEditTrip — which would return true for editor
		// collaborators too and silently grant them update_trip.
		//
		// This test pins the invariant the handler relies on: for the
		// same trip+user, `trip.UserID == userID` discriminates owner
		// from editor, while `CanEditTrip` does not.
		tests := []struct {
			name        string
			userID      uuid.UUID
			wantIsOwner bool
			wantCanEdit bool
		}{
			{"owner is owner and can edit", owner.ID, true, true},
			{"editor is not owner but can edit", editor.ID, false, true},
			{"viewer is not owner and cannot edit", viewer.ID, false, false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				loaded, err := tripSvc.GetByIDOrCollaborator(ctx, tt.userID, tr.ID)
				if err != nil {
					t.Fatalf("GetByIDOrCollaborator(%s): %v", tt.name, err)
				}
				gotIsOwner := loaded.UserID == tt.userID
				if gotIsOwner != tt.wantIsOwner {
					t.Errorf("isOwner via userID comparison = %v, want %v", gotIsOwner, tt.wantIsOwner)
				}
				gotCanEdit, err := tripSvc.CanEditTrip(ctx, tt.userID, tr.ID)
				if err != nil {
					t.Fatalf("CanEditTrip(%s): %v", tt.name, err)
				}
				if gotCanEdit != tt.wantCanEdit {
					t.Errorf("CanEditTrip = %v, want %v", gotCanEdit, tt.wantCanEdit)
				}
				// The regression this test guards against: editors get
				// CanEditTrip=true but must get isOwner=false.
				if tt.userID == editor.ID && gotIsOwner {
					t.Errorf("editor must not be classified as owner; CanEditTrip cannot be used for owner-only gating")
				}
			})
		}
	})

	t.Run("CanEditTrip_PropagatesDBErrors", func(t *testing.T) {
		// #348: a transient DB failure during the authz pre-check must
		// surface as an error, not silently decay to "false" (which
		// would then render as PermissionDenied to the user instead of
		// Unavailable/Internal). We simulate a transient failure by
		// pre-cancelling the context — every pgx query immediately
		// returns context.Canceled, which is NOT pgx.ErrNoRows.
		cancelledCtx, cancel := context.WithCancel(ctx)
		cancel()

		gotCanEdit, err := tripSvc.CanEditTrip(cancelledCtx, owner.ID, tr.ID)
		if err == nil {
			t.Fatalf("CanEditTrip with cancelled ctx returned (canEdit=%v, err=nil); want non-nil error", gotCanEdit)
		}
		if errors.Is(err, trip.ErrNotOwnerOrEditor) {
			t.Errorf("CanEditTrip leaked ErrNotOwnerOrEditor for a DB failure; want a transport error: %v", err)
		}
		// Pin the error kind so a future change that starts treating
		// context.Canceled as a clean "not an editor" (returning
		// (false, nil)) can't silently turn this test into a no-op.
		if !errors.Is(err, context.Canceled) {
			t.Errorf("CanEditTrip expected wrapped context.Canceled, got %v", err)
		}
		if gotCanEdit {
			t.Errorf("CanEditTrip returned canEdit=true on DB failure; want false")
		}

		gotIsEditor, err := tripSvc.IsEditorCollaborator(cancelledCtx, editor.ID, tr.ID)
		if err == nil {
			t.Fatalf("IsEditorCollaborator with cancelled ctx returned (isEditor=%v, err=nil); want non-nil error", gotIsEditor)
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("IsEditorCollaborator expected wrapped context.Canceled, got %v", err)
		}
		if gotIsEditor {
			t.Errorf("IsEditorCollaborator returned isEditor=true on DB failure; want false")
		}

		// ReplaceItineraryForOwnerOrEditor must wrap the CanEditTrip
		// error, not confuse it with ErrNotOwnerOrEditor — otherwise
		// the handler would map it to PermissionDenied instead of
		// surfacing an Internal/Unavailable.
		err = tripSvc.ReplaceItineraryForOwnerOrEditor(cancelledCtx, owner.ID, tr.ID, []trip.ReplaceItineraryItem{
			{DayNumber: 1, OrderInDay: 1, Type: "activity", Title: "irrelevant"},
		})
		if err == nil {
			t.Fatal("ReplaceItineraryForOwnerOrEditor with cancelled ctx returned nil error; want non-nil")
		}
		if errors.Is(err, trip.ErrNotOwnerOrEditor) {
			t.Errorf("ReplaceItineraryForOwnerOrEditor leaked ErrNotOwnerOrEditor for a DB failure; want a transport error: %v", err)
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
		// because the trip doesn't exist. GetTripByID on a bogus
		// UUID returns pgx.ErrNoRows, which CanEditTrip treats as
		// "not the owner, try the editor path" (#348) — the editor
		// lookup then also misses and CanEditTrip returns
		// (false, nil). The net result is ErrNotOwnerOrEditor from
		// the service, which is what callers expect. The RPC
		// handler masks this with a GetByIDOrCollaborator pre-check
		// so users see NotFound, not PermissionDenied.
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

	t.Run("CreateItineraryItemForOwnerOrEditor_AuthzGate", func(t *testing.T) {
		// #346: the insert used by ReplaceItineraryForOwnerOrEditor
		// filters authz in SQL so the TOCTOU window between the
		// outer CanEditTrip pre-check and the per-insert write is
		// closed. Pin the gate's truth table by calling the query
		// directly — if any of these assertions break, the mid-tx
		// race re-opens and a demoted collaborator could sneak
		// items into someone else's trip.
		params := func(uid uuid.UUID) dbgen.CreateItineraryItemForOwnerOrEditorParams {
			return dbgen.CreateItineraryItemForOwnerOrEditorParams{
				TripID:      tr.ID,
				DayNumber:   pgtype.Int4{Int32: 9, Valid: true},
				OrderInDay:  pgtype.Int4{Int32: 1, Valid: true},
				Type:        pgtype.Text{String: "activity", Valid: true},
				Title:       pgtype.Text{String: "authz-gate probe", Valid: true},
				Description: pgtype.Text{String: "", Valid: true},
				UserID:      uid,
			}
		}

		// Owner insert lands.
		row, err := queries.CreateItineraryItemForOwnerOrEditor(ctx, params(owner.ID))
		if err != nil {
			t.Fatalf("owner insert: %v", err)
		}
		t.Cleanup(func() { _, _ = tripSvc.DeleteItineraryItems(ctx, owner.ID, []uuid.UUID{row.ID}) })

		// Editor insert lands.
		row2, err := queries.CreateItineraryItemForOwnerOrEditor(ctx, params(editor.ID))
		if err != nil {
			t.Fatalf("editor insert: %v", err)
		}
		t.Cleanup(func() { _, _ = tripSvc.DeleteItineraryItems(ctx, owner.ID, []uuid.UUID{row2.ID}) })

		// Viewer insert is blocked — the predicate misses and pgx
		// returns ErrNoRows. This is the exact signal the service
		// layer translates into ErrNotOwnerOrEditor, rolling back
		// the entire transaction.
		if _, err := queries.CreateItineraryItemForOwnerOrEditor(ctx, params(viewer.ID)); !errors.Is(err, pgx.ErrNoRows) {
			t.Errorf("viewer insert: expected pgx.ErrNoRows, got %v", err)
		}

		// Outsider insert is blocked for the same reason.
		if _, err := queries.CreateItineraryItemForOwnerOrEditor(ctx, params(outsider.ID)); !errors.Is(err, pgx.ErrNoRows) {
			t.Errorf("outsider insert: expected pgx.ErrNoRows, got %v", err)
		}
	})

	t.Run("ReplaceTranslatesGateMissToAuthzSentinel", func(t *testing.T) {
		// End-to-end counterpart to the gate probe above: when the
		// per-insert predicate misses mid-transaction (simulated
		// here by demoting the editor from 'editor' to 'viewer'
		// before the Replace call fires), the service must return
		// ErrNotOwnerOrEditor — NOT a raw pgx.ErrNoRows, and NOT a
		// partially-committed transaction.
		//
		// The outer CanEditTrip pre-check also fails for this
		// demoted user, so in production the pre-check short-circuits
		// before the insert. But the per-insert gate is what closes
		// the narrow TOCTOU window where role changes land between
		// the pre-check and the write. Both layers converge on the
		// same sentinel.
		if _, err := env.Pool.Exec(ctx,
			`UPDATE trip_collaborators SET role = 'viewer' WHERE trip_id = $1 AND user_id = $2`,
			tr.ID, editor.ID,
		); err != nil {
			t.Fatalf("demote editor: %v", err)
		}
		// Restore after the test so we don't leak state.
		t.Cleanup(func() {
			_, _ = env.Pool.Exec(ctx,
				`UPDATE trip_collaborators SET role = 'editor' WHERE trip_id = $1 AND user_id = $2`,
				tr.ID, editor.ID,
			)
		})

		before, err := tripSvc.GetItinerary(ctx, tr.ID)
		if err != nil {
			t.Fatalf("get itinerary before demoted replace: %v", err)
		}
		beforeLen := len(before)

		items := []trip.ReplaceItineraryItem{
			{DayNumber: 1, OrderInDay: 1, Type: "activity", Title: "Demoted Attempt"},
		}
		err = tripSvc.ReplaceItineraryForOwnerOrEditor(ctx, editor.ID, tr.ID, items)
		if err == nil {
			t.Fatal("demoted editor Replace should be rejected, got nil error")
		}
		if !errors.Is(err, trip.ErrNotOwnerOrEditor) {
			t.Errorf("expected trip.ErrNotOwnerOrEditor, got %v", err)
		}

		after, err := tripSvc.GetItinerary(ctx, tr.ID)
		if err != nil {
			t.Fatalf("get itinerary after demoted replace: %v", err)
		}
		if len(after) != beforeLen {
			t.Errorf("itinerary changed length: before=%d after=%d — transaction did not roll back cleanly", beforeLen, len(after))
		}
		for _, it := range after {
			if it.Title.Valid && it.Title.String == "Demoted Attempt" {
				t.Error("demoted editor's item leaked into itinerary")
			}
		}
	})

	t.Run("GetItineraryForOwnerOrEditor_AuthzGate", func(t *testing.T) {
		// Seed one item so the helper has something to return on the
		// success cases — otherwise a viewer getting back an empty
		// slice (with nil error) would look identical to a properly
		// denied call.
		seeded, err := tripSvc.CreateItineraryItem(ctx, tr.ID, 11, 1, "activity", "Gated Read Probe", "")
		if err != nil {
			t.Fatalf("seed item: %v", err)
		}
		t.Cleanup(func() { _, _ = tripSvc.DeleteItineraryItems(ctx, owner.ID, []uuid.UUID{seeded.ID}) })

		// Owner and editor both see the seeded item.
		for _, uc := range []struct {
			name   string
			userID uuid.UUID
		}{
			{"owner reads", owner.ID},
			{"editor reads", editor.ID},
		} {
			t.Run(uc.name, func(t *testing.T) {
				got, err := tripSvc.GetItineraryForOwnerOrEditor(ctx, uc.userID, tr.ID)
				if err != nil {
					t.Fatalf("GetItineraryForOwnerOrEditor: %v", err)
				}
				found := false
				for _, it := range got {
					if it.ID == seeded.ID {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("seeded item %s not returned", seeded.ID)
				}
			})
		}

		// Viewer is rejected — the point of the new helper. Handing
		// the itinerary back to a viewer here is the gap #353's
		// follow-up closes (read-only collaborators shouldn't get the
		// dedup peek's output routed anywhere).
		t.Run("viewer rejected", func(t *testing.T) {
			_, err := tripSvc.GetItineraryForOwnerOrEditor(ctx, viewer.ID, tr.ID)
			if !errors.Is(err, trip.ErrNotOwnerOrEditor) {
				t.Fatalf("expected ErrNotOwnerOrEditor for viewer, got %v", err)
			}
		})

		// Outsider is rejected identically.
		t.Run("outsider rejected", func(t *testing.T) {
			_, err := tripSvc.GetItineraryForOwnerOrEditor(ctx, outsider.ID, tr.ID)
			if !errors.Is(err, trip.ErrNotOwnerOrEditor) {
				t.Fatalf("expected ErrNotOwnerOrEditor for outsider, got %v", err)
			}
		})

		// A non-existent trip is rejected the same way — GetTripByID
		// misses, IsEditorCollaborator misses, CanEditTrip returns
		// false, and the helper maps to ErrNotOwnerOrEditor. This
		// mirrors ReplaceOnNonExistentTripReturnsPermissionDenied so
		// the read/write paths can't drift apart in the future.
		t.Run("non-existent trip rejected", func(t *testing.T) {
			_, err := tripSvc.GetItineraryForOwnerOrEditor(ctx, editor.ID, uuid.New())
			if !errors.Is(err, trip.ErrNotOwnerOrEditor) {
				t.Fatalf("expected ErrNotOwnerOrEditor for bogus trip, got %v", err)
			}
		})

		// Parallel to CanEditTrip_PropagatesDBErrors: a transient DB
		// failure in the authz gate must surface as a wrapped transport
		// error, never ErrNotOwnerOrEditor. Otherwise a flapping
		// Postgres would render as PermissionDenied to the user
		// instead of Unavailable/Internal.
		t.Run("propagates DB errors", func(t *testing.T) {
			cancelledCtx, cancel := context.WithCancel(ctx)
			cancel()
			_, err := tripSvc.GetItineraryForOwnerOrEditor(cancelledCtx, owner.ID, tr.ID)
			if err == nil {
				t.Fatal("expected non-nil error on cancelled ctx")
			}
			if errors.Is(err, trip.ErrNotOwnerOrEditor) {
				t.Errorf("leaked ErrNotOwnerOrEditor for a DB failure: %v", err)
			}
			if !errors.Is(err, context.Canceled) {
				t.Errorf("expected wrapped context.Canceled, got %v", err)
			}
		})
	})

	t.Run("EditorCanReorderItineraryItem", func(t *testing.T) {
		// #361 P2: the handler's CanEditTrip pre-check lets editors
		// through, but the old MoveItineraryItem SQL filtered only on
		// trips.user_id, so editors' reorders silently hit ErrNoRows
		// and got rewritten as CodeNotFound by the handler. Pin the
		// new ForOwnerOrEditor variant: an editor can move an item
		// on a trip they don't own.
		item, err := tripSvc.CreateItineraryItem(ctx, tr.ID, 1, 5, "activity", "Reorder Probe", "")
		if err != nil {
			t.Fatalf("create probe item: %v", err)
		}
		t.Cleanup(func() { _, _ = tripSvc.DeleteItineraryItems(ctx, owner.ID, []uuid.UUID{item.ID}) })

		moved, err := tripSvc.MoveItineraryItem(ctx, editor.ID, tr.ID, item.ID, 3, 1)
		if err != nil {
			t.Fatalf("editor reorder: %v", err)
		}
		if !moved.DayNumber.Valid || moved.DayNumber.Int32 != 3 {
			t.Errorf("expected DayNumber=3 after editor move, got %+v", moved.DayNumber)
		}
		if !moved.OrderInDay.Valid || moved.OrderInDay.Int32 != 1 {
			t.Errorf("expected OrderInDay=1, got %+v", moved.OrderInDay)
		}

		// Viewer reorder is still rejected.
		if _, err := tripSvc.MoveItineraryItem(ctx, viewer.ID, tr.ID, item.ID, 4, 1); err == nil {
			t.Errorf("viewer reorder should be rejected, got nil error")
		}

		// Outsider reorder is still rejected.
		if _, err := tripSvc.MoveItineraryItem(ctx, outsider.ID, tr.ID, item.ID, 4, 1); err == nil {
			t.Errorf("outsider reorder should be rejected, got nil error")
		}
	})
}
