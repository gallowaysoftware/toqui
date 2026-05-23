//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"sort"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/gallowaysoftware/toqui/backend/internal/ai"
	"github.com/gallowaysoftware/toqui/backend/internal/ai/tools"
	"github.com/gallowaysoftware/toqui/backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui/backend/internal/handlers"
	"github.com/gallowaysoftware/toqui/backend/internal/trip"
)

// TestChatOwnerOnlyToolGate pins the end-to-end behaviour of the chat
// tool-registration path fixed in #354: only trip owners get the
// update_trip tool (title/description/status/destinations — #263).
// Editor-role collaborators and viewers must not.
//
// This complements the primitive-contract subtest
// TestCollaboratorEditing/OwnerOnlyToolGate_UsesUserIDComparison, which
// pins the discrimination primitive (trip.UserID == userID vs
// CanEditTrip). That subtest will still pass under a hypothetical
// revert that re-wires update_trip onto CanEditTrip inside the chat
// handler — because it never calls the handler. This test asserts on
// the handler helper's output directly: any reverted discrimination
// inside BuildPlanningAndCompanionTools produces a tool-set mismatch.
//
// The helper derives isOwner from the tripOwnerID parameter, not from
// a caller-supplied bool — so this test also pins the invariant that
// feeding a real (tripOwnerID, userID) pair through the handler helper
// produces the correct tool list.
func TestChatOwnerOnlyToolGate(t *testing.T) {
	env := NewTestEnv(t)
	env.CleanDB(t)
	ctx := context.Background()
	queries := dbgen.New(env.Pool)
	tripSvc := trip.NewService(env.Pool)

	owner, err := queries.UpsertUserByGoogleID(ctx, dbgen.UpsertUserByGoogleIDParams{
		GoogleID: pgtype.Text{String: "owner-chatgate-111", Valid: true},
		Email:    "owner-chatgate@example.com",
		Name:     pgtype.Text{String: "Owner", Valid: true},
	})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	editor, err := queries.UpsertUserByGoogleID(ctx, dbgen.UpsertUserByGoogleIDParams{
		GoogleID: pgtype.Text{String: "editor-chatgate-222", Valid: true},
		Email:    "editor-chatgate@example.com",
		Name:     pgtype.Text{String: "Editor", Valid: true},
	})
	if err != nil {
		t.Fatalf("create editor: %v", err)
	}
	viewer, err := queries.UpsertUserByGoogleID(ctx, dbgen.UpsertUserByGoogleIDParams{
		GoogleID: pgtype.Text{String: "viewer-chatgate-333", Valid: true},
		Email:    "viewer-chatgate@example.com",
		Name:     pgtype.Text{String: "Viewer", Valid: true},
	})
	if err != nil {
		t.Fatalf("create viewer: %v", err)
	}

	tr, err := tripSvc.Create(ctx, owner.ID, "Chat Gate Trip", "Owner-only tool regression", nil, nil)
	if err != nil {
		t.Fatalf("create trip: %v", err)
	}

	if _, err := queries.AddCollaborator(ctx, dbgen.AddCollaboratorParams{
		TripID:      tr.ID,
		Email:       "editor-chatgate@example.com",
		Role:        "editor",
		InviteToken: pgtype.Text{String: "editor-chatgate-token", Valid: true},
		InvitedBy:   owner.ID,
	}); err != nil {
		t.Fatalf("invite editor: %v", err)
	}
	if _, err := queries.AcceptInvite(ctx, dbgen.AcceptInviteParams{
		InviteToken: pgtype.Text{String: "editor-chatgate-token", Valid: true},
		UserID:      pgtype.UUID{Bytes: editor.ID, Valid: true},
	}); err != nil {
		t.Fatalf("accept editor invite: %v", err)
	}

	if _, err := queries.AddCollaborator(ctx, dbgen.AddCollaboratorParams{
		TripID:      tr.ID,
		Email:       "viewer-chatgate@example.com",
		Role:        "viewer",
		InviteToken: pgtype.Text{String: "viewer-chatgate-token", Valid: true},
		InvitedBy:   owner.ID,
	}); err != nil {
		t.Fatalf("invite viewer: %v", err)
	}
	if _, err := queries.AcceptInvite(ctx, dbgen.AcceptInviteParams{
		InviteToken: pgtype.Text{String: "viewer-chatgate-token", Valid: true},
		UserID:      pgtype.UUID{Bytes: viewer.ID, Valid: true},
	}); err != nil {
		t.Fatalf("accept viewer invite: %v", err)
	}

	// Minimal ChatHandler — only tripSvc and pool are needed for the
	// planning/companion tool-registration path.
	h := handlers.NewChatHandler(nil, tripSvc, nil, nil, nil, nil, nil, nil, env.Pool, nil)

	const createName = "create_itinerary_items"
	const deleteName = "delete_itinerary_items"
	const reorderName = "reorder_itinerary_items"
	const updateTripName = "update_trip"

	cases := []struct {
		name      string
		userID    uuid.UUID
		wantTools []string
	}{
		{
			name:      "owner gets create+delete+reorder+update_trip",
			userID:    owner.ID,
			wantTools: []string{createName, deleteName, reorderName, updateTripName},
		},
		{
			name:      "editor gets create+delete+reorder but NOT update_trip",
			userID:    editor.ID,
			wantTools: []string{createName, deleteName, reorderName},
		},
		{
			name:      "viewer gets no write tools",
			userID:    viewer.ID,
			wantTools: nil,
		},
	}

	// Drive both planning and companion modes. For companion mode we
	// skip CompanionGate wrapping (aiProvider nil) so the assertion
	// below inspects the base tools directly; a separate subtest
	// (companion_wraps_in_CompanionGate) exercises the wrapping path.
	for _, mode := range []string{"planning", "companion"} {
		for _, tc := range cases {
			tc := tc
			mode := mode
			t.Run(mode+"/"+tc.name, func(t *testing.T) {
				// Resolve editor status the same way SendMessage does.
				// Ownership is NOT pre-computed by the test — the helper
				// derives it from the tripOwnerID we pass in, so any
				// revert of the ownership check inside the helper is
				// directly caught by the assertions below.
				loaded, err := tripSvc.GetByIDOrCollaborator(ctx, tc.userID, tr.ID)
				if err != nil {
					t.Fatalf("GetByIDOrCollaborator(%s): %v", tc.name, err)
				}
				var isEditor bool
				if loaded.UserID != tc.userID {
					isEditor, err = tripSvc.IsEditorCollaborator(ctx, tc.userID, tr.ID)
					if err != nil {
						t.Fatalf("IsEditorCollaborator(%s): %v", tc.name, err)
					}
				}

				got := h.BuildPlanningAndCompanionTools(
					tr.ID, loaded.UserID, tc.userID,
					isEditor, mode, "what's on the agenda",
					func([]dbgen.ItineraryItem) {}, func(string, string, string, []string) {},
				)

				gotNames := toolNames(got)
				if !sameStringSet(gotNames, tc.wantTools) {
					t.Errorf("tool set mismatch for %s (mode=%s):\n got:  %v\n want: %v",
						tc.name, mode, gotNames, tc.wantTools)
				}

				// Explicit #263 regression probe. The set comparison
				// above already catches this, but pin it as a named
				// assertion so any future change that widens the
				// editor's tool set has to explain the slip.
				if tc.userID == editor.ID {
					for _, n := range gotNames {
						if n == updateTripName {
							t.Errorf("editor unexpectedly received %q (mode=%s) — owner-only gate broken", updateTripName, mode)
						}
					}
				}

				// Viewers must never receive any write tool — no
				// create, no delete, no reorder, no update.
				if tc.userID == viewer.ID && len(gotNames) != 0 {
					t.Errorf("viewer received unexpected write tools (mode=%s): %v", mode, gotNames)
				}
			})
		}
	}

	// Regression probe against a likely future revert: if someone tries
	// to "simplify" the helper by taking the tripOwnerID arg and
	// feeding CanEditTrip into it (i.e. flipping owner discrimination
	// to "anyone who can edit"), this call — which passes the EDITOR
	// as tripOwnerID — would cause the editor to receive update_trip.
	// We don't call the real code that way; we call it with the real
	// owner ID and assert the editor does not get update_trip. See the
	// subtests above for that assertion.

	// Companion-mode wrapping path: when aiProvider is set, every
	// write tool returned in companion mode must be a CompanionGate
	// instance (which delegates Definition().Name but intercepts
	// Execute to block unsolicited modifications — Run 5/Run 8
	// regression). Without this assertion, a regression that drops
	// the wrap on, say, deleteTool would slip through since
	// CompanionGate delegates Name. Exercised for both owner and
	// editor so a conditional like `if !isEditor { wrap }` can't slip
	// past either.
	//
	// update_trip is NOT wrapped by the gate (it is not an itinerary
	// mutation), so it's excluded from this assertion.
	hh := handlers.NewChatHandler(nil, tripSvc, nil, nil, nil, nil, nil, nil, env.Pool, nil)
	hh.WithAIProvider(stubAIProvider{})
	wrapCases := []struct {
		name        string
		tripOwnerID uuid.UUID
		userID      uuid.UUID
		isEditor    bool
	}{
		{"owner", owner.ID, owner.ID, false},
		{"editor", owner.ID, editor.ID, true},
	}
	for _, wc := range wrapCases {
		wc := wc
		t.Run("companion_wraps_in_CompanionGate/"+wc.name, func(t *testing.T) {
			got := hh.BuildPlanningAndCompanionTools(
				tr.ID, wc.tripOwnerID, wc.userID,
				wc.isEditor, "companion", "what's on the agenda",
				func([]dbgen.ItineraryItem) {}, func(string, string, string, []string) {},
			)

			gated := map[string]bool{}
			for _, tool := range got {
				if _, ok := tool.(*handlers.CompanionGate); ok {
					gated[tool.Definition().Name] = true
				}
			}
			for _, name := range []string{createName, deleteName, reorderName} {
				if !gated[name] {
					t.Errorf("companion mode (%s): %q is not wrapped in CompanionGate", wc.name, name)
				}
			}
			for _, tool := range got {
				if tool.Definition().Name != updateTripName {
					continue
				}
				if _, ok := tool.(*handlers.CompanionGate); ok {
					t.Errorf("%q must not be wrapped in CompanionGate (%s)", updateTripName, wc.name)
				}
			}
		})
	}
}

func toolNames(ts []tools.Tool) []string {
	if len(ts) == 0 {
		return nil
	}
	out := make([]string, 0, len(ts))
	for _, tool := range ts {
		out = append(out, tool.Definition().Name)
	}
	return out
}

func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	sa := append([]string(nil), a...)
	sb := append([]string(nil), b...)
	sort.Strings(sa)
	sort.Strings(sb)
	for i := range sa {
		if sa[i] != sb[i] {
			return false
		}
	}
	return true
}

// stubAIProvider satisfies ai.Provider with no real work — it exists
// solely so the CompanionGate wrap path fires during test construction.
// TestCreateItineraryToolEnforcesAuthzGate exercises the #353 TOCTOU
// fix end-to-end. The chat tool used to call the un-gated
// CreateItineraryItem, so a collaborator whose role changed between
// the handler's pre-check and the AI firing the tool call would still
// land inserts. Now the tool calls the SQL-gated
// CreateItineraryItemForOwnerOrEditor helper; a caller who isn't
// owner-or-editor at the exact moment of the insert gets the
// predicate miss translated to a clean "forbidden" tool result.
//
// Covers both at-risk items flagged in #353:
//  1. Post-pre-check demotion (simulated by constructing the tool with
//     a viewer/outsider callerID — equivalent to the mid-stream demote
//     from the handler's perspective, since the gate fires only at
//     insert time).
//  2. Selection-mode req.Msg.TripId spoofing (outsider caller passing
//     a trip_id they don't own).
func TestCreateItineraryToolEnforcesAuthzGate(t *testing.T) {
	env := NewTestEnv(t)
	env.CleanDB(t)
	ctx := context.Background()
	queries := dbgen.New(env.Pool)
	tripSvc := trip.NewService(env.Pool)

	owner, err := queries.UpsertUserByGoogleID(ctx, dbgen.UpsertUserByGoogleIDParams{
		GoogleID: pgtype.Text{String: "owner-toctou-111", Valid: true},
		Email:    "owner-toctou@example.com",
		Name:     pgtype.Text{String: "Owner", Valid: true},
	})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	viewer, err := queries.UpsertUserByGoogleID(ctx, dbgen.UpsertUserByGoogleIDParams{
		GoogleID: pgtype.Text{String: "viewer-toctou-222", Valid: true},
		Email:    "viewer-toctou@example.com",
		Name:     pgtype.Text{String: "Viewer", Valid: true},
	})
	if err != nil {
		t.Fatalf("create viewer: %v", err)
	}
	outsider, err := queries.UpsertUserByGoogleID(ctx, dbgen.UpsertUserByGoogleIDParams{
		GoogleID: pgtype.Text{String: "outsider-toctou-333", Valid: true},
		Email:    "outsider-toctou@example.com",
		Name:     pgtype.Text{String: "Outsider", Valid: true},
	})
	if err != nil {
		t.Fatalf("create outsider: %v", err)
	}

	tr, err := tripSvc.Create(ctx, owner.ID, "TOCTOU Trip", "", nil, nil)
	if err != nil {
		t.Fatalf("create trip: %v", err)
	}

	if _, err := queries.AddCollaborator(ctx, dbgen.AddCollaboratorParams{
		TripID:      tr.ID,
		Email:       "viewer-toctou@example.com",
		Role:        "viewer",
		InviteToken: pgtype.Text{String: "viewer-toctou-token", Valid: true},
		InvitedBy:   owner.ID,
	}); err != nil {
		t.Fatalf("invite viewer: %v", err)
	}
	if _, err := queries.AcceptInvite(ctx, dbgen.AcceptInviteParams{
		InviteToken: pgtype.Text{String: "viewer-toctou-token", Valid: true},
		UserID:      pgtype.UUID{Bytes: viewer.ID, Valid: true},
	}); err != nil {
		t.Fatalf("accept viewer invite: %v", err)
	}

	args := json.RawMessage(`{"items":[{"day_number":1,"order_in_day":1,"title":"Injection","type":"activity"}]}`)

	cases := []struct {
		name        string
		callerID    uuid.UUID
		wantError   string // substring the marshalled result must contain
		mustNotLand bool   // trip's itinerary must be unchanged after the call
	}{
		{
			name:        "viewer caller gets forbidden, no insert lands",
			callerID:    viewer.ID,
			wantError:   "forbidden",
			mustNotLand: true,
		},
		{
			name:        "outsider caller (e.g. spoofed trip_id in selection mode) gets forbidden",
			callerID:    outsider.ID,
			wantError:   "forbidden",
			mustNotLand: true,
		},
		{
			name:        "owner caller succeeds",
			callerID:    owner.ID,
			wantError:   "",
			mustNotLand: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			beforeCount, err := countItineraryItems(ctx, env, tr.ID)
			if err != nil {
				t.Fatalf("count before: %v", err)
			}

			tool := handlers.NewCreateItineraryTool(tripSvc, tr.ID, tc.callerID, nil)
			res, err := tool.Execute(ctx, args)
			if err != nil {
				t.Fatalf("tool.Execute: %v", err)
			}
			var parsed map[string]any
			if jsonErr := json.Unmarshal(res, &parsed); jsonErr != nil {
				t.Fatalf("parse tool result: %v", jsonErr)
			}

			if tc.wantError != "" {
				errField, _ := parsed["error"].(string)
				if errField != tc.wantError {
					t.Errorf("tool.error = %q, want %q (full result: %s)", errField, tc.wantError, res)
				}
			} else {
				if _, hasErr := parsed["error"]; hasErr {
					t.Errorf("unexpected error in tool result: %s", res)
				}
				created, _ := parsed["created_count"].(float64)
				if created < 1 {
					t.Errorf("expected at least one item created, got %v", parsed["created_count"])
				}
			}

			afterCount, err := countItineraryItems(ctx, env, tr.ID)
			if err != nil {
				t.Fatalf("count after: %v", err)
			}
			switch {
			case tc.mustNotLand && afterCount != beforeCount:
				t.Errorf("items leaked despite forbidden result: before=%d after=%d", beforeCount, afterCount)
			case !tc.mustNotLand && afterCount <= beforeCount:
				t.Errorf("expected item to land for authorised caller: before=%d after=%d", beforeCount, afterCount)
			}
		})
	}
}

func countItineraryItems(ctx context.Context, env *TestEnv, tripID uuid.UUID) (int, error) {
	var n int
	err := env.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM itinerary_items WHERE trip_id = $1", tripID).Scan(&n)
	return n, err
}

// The gate does not invoke the provider at construction time, only at
// Execute, which this test does not call.
type stubAIProvider struct{}

func (stubAIProvider) ChatStream(_ context.Context, _ *ai.ChatRequest) (<-chan ai.Event, error) {
	ch := make(chan ai.Event)
	close(ch)
	return ch, nil
}

func (stubAIProvider) Name() string { return "stub" }
