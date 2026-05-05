//go:build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"testing"

	"cloud.google.com/go/firestore"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestEnv holds shared resources for integration tests.
type TestEnv struct {
	Pool      *pgxpool.Pool
	Firestore *firestore.Client
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

	// Firestore emulator
	emulatorHost := os.Getenv("FIRESTORE_EMULATOR_HOST")
	if emulatorHost == "" {
		t.Skip("FIRESTORE_EMULATOR_HOST not set — skipping integration test")
	}
	os.Setenv("FIRESTORE_EMULATOR_HOST", emulatorHost)

	fsClient, err := firestore.NewClient(ctx, "toqui-test")
	if err != nil {
		t.Fatalf("create firestore client: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
		fsClient.Close()
	})

	return &TestEnv{Pool: pool, Firestore: fsClient}
}

// CleanDB truncates all tables for test isolation.
func (e *TestEnv) CleanDB(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	tables := []string{
		"export_requests", "deletion_requests",
		"trip_collaborators", "trip_themes", "bookings", "itinerary_items", "trips", "users",
		// under_age_blocks survives user deletion (no FK to users) so it
		// must be truncated explicitly between tests, otherwise the
		// per-email UNIQUE constraint will fail tests that reuse a
		// fixture email.
		"under_age_blocks",
	}
	for _, table := range tables {
		if _, err := e.Pool.Exec(ctx, fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table)); err != nil {
			t.Fatalf("truncate %s: %v", table, err)
		}
	}
}
