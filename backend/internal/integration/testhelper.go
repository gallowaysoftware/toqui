//go:build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestEnv holds shared resources for integration tests.
type TestEnv struct {
	Pool *pgxpool.Pool
}

func NewTestEnv(t *testing.T) *TestEnv {
	t.Helper()

	ctx := context.Background()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set — skipping integration test")
	}

	// Run migrations
	m, err := migrate.New("file://../../db/migrations", dbURL)
	if err != nil {
		t.Fatalf("create migrator: %v", err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		t.Fatalf("migrate up: %v", err)
	}
	m.Close()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
	})

	return &TestEnv{Pool: pool}
}

// CleanDB truncates all tables for test isolation.
func (e *TestEnv) CleanDB(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	tables := []string{
		"export_requests", "deletion_requests",
		"chat_messages", "chat_sessions",
		"trip_collaborators", "trip_themes", "bookings", "itinerary_items", "trips", "users",
	}
	for _, table := range tables {
		if _, err := e.Pool.Exec(ctx, fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table)); err != nil {
			t.Fatalf("truncate %s: %v", table, err)
		}
	}
}
