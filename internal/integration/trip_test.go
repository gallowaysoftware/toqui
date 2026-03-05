//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui-backend/internal/trip"
)

func TestTripCRUD(t *testing.T) {
	env := NewTestEnv(t)
	env.CleanDB(t)
	ctx := context.Background()
	queries := dbgen.New(env.Pool)

	// Create a test user
	user, err := queries.UpsertUserByGoogleID(ctx, dbgen.UpsertUserByGoogleIDParams{
		GoogleID: "test-google-123",
		Email:    "test@example.com",
		Name:     pgtype.Text{String: "Test User", Valid: true},
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	tripSvc := trip.NewService(env.Pool)

	// Create
	startDate := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	created, err := tripSvc.Create(ctx, user.ID, "Japan Trip", "Two weeks in Tokyo and Kyoto", &startDate, nil)
	if err != nil {
		t.Fatalf("create trip: %v", err)
	}
	if created.Title != "Japan Trip" {
		t.Errorf("title = %q, want %q", created.Title, "Japan Trip")
	}
	if created.Status != "planning" {
		t.Errorf("status = %q, want %q", created.Status, "planning")
	}

	// Get by ID
	got, err := tripSvc.GetByID(ctx, user.ID, created.ID)
	if err != nil {
		t.Fatalf("get trip: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("got ID %v, want %v", got.ID, created.ID)
	}

	// List
	trips, count, err := tripSvc.ListByUser(ctx, user.ID, "", 10, 0)
	if err != nil {
		t.Fatalf("list trips: %v", err)
	}
	if len(trips) != 1 {
		t.Errorf("got %d trips, want 1", len(trips))
	}
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}

	// Update
	updated, err := tripSvc.Update(ctx, user.ID, created.ID, "Japan + Korea", "", "active", nil, nil)
	if err != nil {
		t.Fatalf("update trip: %v", err)
	}
	if updated.Title != "Japan + Korea" {
		t.Errorf("updated title = %q, want %q", updated.Title, "Japan + Korea")
	}
	if updated.Status != "active" {
		t.Errorf("updated status = %q, want %q", updated.Status, "active")
	}

	// Delete
	if err := tripSvc.Delete(ctx, user.ID, created.ID); err != nil {
		t.Fatalf("delete trip: %v", err)
	}

	trips, count, err = tripSvc.ListByUser(ctx, user.ID, "", 10, 0)
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if len(trips) != 0 {
		t.Errorf("got %d trips after delete, want 0", len(trips))
	}
	if count != 0 {
		t.Errorf("count after delete = %d, want 0", count)
	}
}

func TestTripListByStatus(t *testing.T) {
	env := NewTestEnv(t)
	env.CleanDB(t)
	ctx := context.Background()
	queries := dbgen.New(env.Pool)

	user, err := queries.UpsertUserByGoogleID(ctx, dbgen.UpsertUserByGoogleIDParams{
		GoogleID: "test-google-status",
		Email:    "status@example.com",
		Name:     pgtype.Text{String: "Status User", Valid: true},
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	tripSvc := trip.NewService(env.Pool)

	// Create trips with different statuses
	if _, err := tripSvc.Create(ctx, user.ID, "Planning Trip", "", nil, nil); err != nil {
		t.Fatalf("create planning trip: %v", err)
	}
	activeTrip, err := tripSvc.Create(ctx, user.ID, "Active Trip", "", nil, nil)
	if err != nil {
		t.Fatalf("create active trip: %v", err)
	}
	if _, err := tripSvc.Update(ctx, user.ID, activeTrip.ID, "Active Trip", "", "active", nil, nil); err != nil {
		t.Fatalf("update to active: %v", err)
	}

	// Filter by planning
	planning, count, err := tripSvc.ListByUser(ctx, user.ID, "planning", 10, 0)
	if err != nil {
		t.Fatalf("list planning: %v", err)
	}
	if len(planning) != 1 || count != 1 {
		t.Errorf("planning: got %d trips (count %d), want 1", len(planning), count)
	}

	// Filter by active
	active, count, err := tripSvc.ListByUser(ctx, user.ID, "active", 10, 0)
	if err != nil {
		t.Fatalf("list active: %v", err)
	}
	if len(active) != 1 || count != 1 {
		t.Errorf("active: got %d trips (count %d), want 1", len(active), count)
	}

	// All
	all, count, err := tripSvc.ListByUser(ctx, user.ID, "", 10, 0)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 2 || count != 2 {
		t.Errorf("all: got %d trips (count %d), want 2", len(all), count)
	}
}
