// testctl manages agentic test users and run artifacts.
//
// Usage:
//
//	go run ./cmd/testctl create-user --name "Alice" --email "alice@toqui-test.local"
//	go run ./cmd/testctl cleanup-user --user-id "uuid"
//	go run ./cmd/testctl diff-runs --from run-5.json --to run-6.json
//	go run ./cmd/testctl baseline-compare --baselines tests/agentic/baselines --run run-6.json
//	go run ./cmd/testctl validate-report --file report.json
//	go run ./cmd/testctl run-persona --id R-02 --token "eyJ..." --expected-email "alice@..."
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gallowaysoftware/toqui-backend/internal/config"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: testctl <create-user|cleanup-user|diff-runs|baseline-compare|validate-report|run-persona> [flags]")
		os.Exit(1)
	}

	// Pure-file commands that don't need a database connection run before
	// config loading so they work offline without a DATABASE_URL.
	switch os.Args[1] {
	case "diff-runs":
		diffRuns(os.Args[2:])
		return
	case "baseline-compare":
		baselineCompare(os.Args[2:])
		return
	case "validate-report":
		validateReport(os.Args[2:])
		return
	case "run-persona":
		runPersonaPrompt(os.Args[2:])
		return
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer pool.Close()

	queries := dbgen.New(pool)

	switch os.Args[1] {
	case "create-user":
		createUser(ctx, queries, cfg, pool, os.Args[2:])
	case "cleanup-user":
		cleanupUser(ctx, pool, os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

type createUserResult struct {
	UserID string `json:"user_id"`
	Token  string `json:"token"`
}

func createUser(ctx context.Context, queries *dbgen.Queries, cfg *config.Config, pool *pgxpool.Pool, args []string) {
	fs := flag.NewFlagSet("create-user", flag.ExitOnError)
	name := fs.String("name", "Test User", "user display name")
	email := fs.String("email", "", "user email (required)")
	ttl := fs.Duration("ttl", 4*time.Hour, "token time-to-live (default 4h)")
	if err := fs.Parse(args); err != nil {
		log.Fatal(err)
	}
	if *email == "" {
		log.Fatal("--email is required")
	}

	googleID := "agentic-test-" + uuid.New().String()
	user, err := queries.UpsertUserByGoogleID(ctx, dbgen.UpsertUserByGoogleIDParams{
		GoogleID:  googleID,
		Email:     *email,
		Name:      pgtype.Text{String: *name, Valid: true},
		AvatarUrl: pgtype.Text{},
	})
	if err != nil {
		log.Fatalf("create user: %v", err)
	}

	// Set age_verified_at to pass the age gate interceptor.
	_, err = pool.Exec(ctx, "UPDATE users SET age_verified_at = NOW() WHERE id = $1", user.ID)
	if err != nil {
		log.Fatalf("set age verification: %v", err)
	}

	// Generate token with configurable TTL (default 4h for agentic tests)
	claims := jwt.MapClaims{
		"sub": user.ID.String(),
		"exp": time.Now().Add(*ttl).Unix(),
		"iat": time.Now().Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token, err := tok.SignedString([]byte(cfg.JWTSecret))
	if err != nil {
		log.Fatalf("generate token: %v", err)
	}

	result := createUserResult{
		UserID: user.ID.String(),
		Token:  token,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		log.Fatal(err)
	}
}

func cleanupUser(ctx context.Context, pool *pgxpool.Pool, args []string) {
	fs := flag.NewFlagSet("cleanup-user", flag.ExitOnError)
	userID := fs.String("user-id", "", "user UUID to delete (required)")
	if err := fs.Parse(args); err != nil {
		log.Fatal(err)
	}
	if *userID == "" {
		log.Fatal("--user-id is required")
	}

	uid, err := uuid.Parse(*userID)
	if err != nil {
		log.Fatalf("invalid UUID: %v", err)
	}

	// Delete trips first (CASCADE handles trip_themes, itinerary_items, bookings).
	_, err = pool.Exec(ctx, "DELETE FROM trips WHERE user_id = $1", uid)
	if err != nil {
		log.Fatalf("delete trips: %v", err)
	}

	_, err = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", uid)
	if err != nil {
		log.Fatalf("delete user: %v", err)
	}

	fmt.Fprintf(os.Stderr, "cleaned up user %s\n", uid)
}
