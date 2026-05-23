//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/gallowaysoftware/toqui/backend/internal/chatstore"
	"github.com/gallowaysoftware/toqui/backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui/backend/internal/lifecycle"
	"github.com/gallowaysoftware/toqui/backend/internal/trip"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestDeleteUser(t *testing.T) {
	env := NewTestEnv(t)
	env.CleanDB(t)
	ctx := context.Background()
	queries := dbgen.New(env.Pool)
	store := chatstore.New(env.Firestore)
	tripSvc := trip.NewService(env.Pool)
	lifecycleSvc := lifecycle.NewService(env.Pool, store)

	// Create user and trip
	user, err := queries.UpsertUserByGoogleID(ctx, dbgen.UpsertUserByGoogleIDParams{
		GoogleID: pgtype.Text{String: "test-delete-user", Valid: true},
		Email:    "delete@example.com",
		Name:     pgtype.Text{String: "Delete Me", Valid: true},
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	created, err := tripSvc.Create(ctx, user.ID, "Trip to Delete", "", nil, nil)
	if err != nil {
		t.Fatalf("create trip: %v", err)
	}

	// Add some chat data
	session, err := store.CreateSession(ctx, user.ID.String(), created.ID.String(), "planning")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if err := store.AddMessage(ctx, user.ID.String(), created.ID.String(), session.ID, &chatstore.ChatMessage{
		Role: "user", Content: "Hello",
	}); err != nil {
		t.Fatalf("add message: %v", err)
	}

	// Delete user
	if err := lifecycleSvc.DeleteUser(ctx, user.ID); err != nil {
		t.Fatalf("delete user: %v", err)
	}

	// Verify trips are gone
	trips, count, err := tripSvc.ListByUser(ctx, user.ID, "", 10, 0)
	if err != nil {
		t.Fatalf("list trips after delete: %v", err)
	}
	if len(trips) != 0 || count != 0 {
		t.Errorf("expected 0 trips, got %d (count %d)", len(trips), count)
	}

	// Verify Firestore data is gone
	sessions, err := store.ListSessions(ctx, user.ID.String(), created.ID.String(), 10)
	if err != nil {
		t.Fatalf("list sessions after delete: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}
