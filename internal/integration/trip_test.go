//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui-backend/internal/trip"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestTripCRUD(t *testing.T) {
	env := NewTestEnv(t)
	env.CleanDB(t)
	ctx := context.Background()
	queries := dbgen.New(env.Pool)

	// Create a test user
	user, err := queries.UpsertUserByGoogleID(ctx, dbgen.UpsertUserByGoogleIDParams{
		GoogleID: pgtype.Text{String: "test-google-123", Valid: true},
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
		GoogleID: pgtype.Text{String: "test-google-status", Valid: true},
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

func TestItineraryItemCRUD(t *testing.T) {
	env := NewTestEnv(t)
	env.CleanDB(t)
	ctx := context.Background()
	queries := dbgen.New(env.Pool)

	// Create test user
	user, err := queries.UpsertUserByGoogleID(ctx, dbgen.UpsertUserByGoogleIDParams{
		GoogleID: pgtype.Text{String: "test-google-itinerary", Valid: true},
		Email:    "itinerary@example.com",
		Name:     pgtype.Text{String: "Itinerary User", Valid: true},
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	tripSvc := trip.NewService(env.Pool)

	// Create a trip to attach itinerary items to
	created, err := tripSvc.Create(ctx, user.ID, "Tokyo Food Tour", "3 days of incredible food", nil, nil)
	if err != nil {
		t.Fatalf("create trip: %v", err)
	}

	// Create itinerary items
	item1, err := tripSvc.CreateItineraryItem(ctx, created.ID, 1, 1, "meal", "Breakfast at Tsukiji Outer Market", "Get there early for fresh sushi and tamagoyaki")
	if err != nil {
		t.Fatalf("create item 1: %v", err)
	}
	if !item1.Title.Valid || item1.Title.String != "Breakfast at Tsukiji Outer Market" {
		t.Errorf("item1 title = %v, want %q", item1.Title, "Breakfast at Tsukiji Outer Market")
	}
	if !item1.DayNumber.Valid || item1.DayNumber.Int32 != 1 {
		t.Errorf("item1 day_number = %v, want 1", item1.DayNumber)
	}
	if !item1.OrderInDay.Valid || item1.OrderInDay.Int32 != 1 {
		t.Errorf("item1 order_in_day = %v, want 1", item1.OrderInDay)
	}
	if !item1.Type.Valid || item1.Type.String != "meal" {
		t.Errorf("item1 type = %v, want %q", item1.Type, "meal")
	}

	item2, err := tripSvc.CreateItineraryItem(ctx, created.ID, 1, 2, "sightseeing", "Visit Senso-ji Temple", "")
	if err != nil {
		t.Fatalf("create item 2: %v", err)
	}

	item3, err := tripSvc.CreateItineraryItem(ctx, created.ID, 2, 1, "activity", "Ramen cooking class", "Learn to make tonkotsu ramen from scratch")
	if err != nil {
		t.Fatalf("create item 3: %v", err)
	}

	// List all itinerary items
	items, err := tripSvc.GetItinerary(ctx, created.ID)
	if err != nil {
		t.Fatalf("get itinerary: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}

	// Verify ordering — should be sorted by day_number, order_in_day
	if items[0].ID != item1.ID {
		t.Errorf("first item should be item1 (day 1, order 1)")
	}
	if items[1].ID != item2.ID {
		t.Errorf("second item should be item2 (day 1, order 2)")
	}
	if items[2].ID != item3.ID {
		t.Errorf("third item should be item3 (day 2, order 1)")
	}

	// Verify item with no description has empty pgtype.Text
	if item2.Description.Valid && item2.Description.String != "" {
		t.Errorf("item2 description should be empty, got %q", item2.Description.String)
	}

	// Verify items are tied to the correct trip
	for _, item := range items {
		if item.TripID != created.ID {
			t.Errorf("item trip_id = %v, want %v", item.TripID, created.ID)
		}
	}

	// Items should be cascade-deleted when trip is deleted
	if err := tripSvc.Delete(ctx, user.ID, created.ID); err != nil {
		t.Fatalf("delete trip: %v", err)
	}
	itemsAfter, err := tripSvc.GetItinerary(ctx, created.ID)
	if err != nil {
		t.Fatalf("get itinerary after delete: %v", err)
	}
	if len(itemsAfter) != 0 {
		t.Errorf("got %d items after trip delete, want 0", len(itemsAfter))
	}
}

func TestItineraryItemOptionalFields(t *testing.T) {
	env := NewTestEnv(t)
	env.CleanDB(t)
	ctx := context.Background()
	queries := dbgen.New(env.Pool)

	user, err := queries.UpsertUserByGoogleID(ctx, dbgen.UpsertUserByGoogleIDParams{
		GoogleID: pgtype.Text{String: "test-google-itin-opt", Valid: true},
		Email:    "itin-opt@example.com",
		Name:     pgtype.Text{String: "Opt User", Valid: true},
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	tripSvc := trip.NewService(env.Pool)
	created, err := tripSvc.Create(ctx, user.ID, "Minimal Trip", "", nil, nil)
	if err != nil {
		t.Fatalf("create trip: %v", err)
	}

	// Create item with minimal fields (only title and type required by schema)
	item, err := tripSvc.CreateItineraryItem(ctx, created.ID, 0, 0, "activity", "Free day exploring", "")
	if err != nil {
		t.Fatalf("create minimal item: %v", err)
	}

	// day_number=0 maps to pgtype.Int4{Valid: false} (null)
	if item.DayNumber.Valid {
		t.Errorf("expected null day_number for 0 input, got %d", item.DayNumber.Int32)
	}
	if item.OrderInDay.Valid {
		t.Errorf("expected null order_in_day for 0 input, got %d", item.OrderInDay.Int32)
	}
	if !item.Title.Valid || item.Title.String != "Free day exploring" {
		t.Errorf("title = %v, want %q", item.Title, "Free day exploring")
	}
}
