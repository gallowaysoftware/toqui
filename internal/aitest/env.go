//go:build aitest

package aitest

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"

	"cloud.google.com/go/firestore"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gallowaysoftware/toqui-backend/internal/ai"
	"github.com/gallowaysoftware/toqui-backend/internal/ai/tools"
	"github.com/gallowaysoftware/toqui-backend/internal/chat"
	"github.com/gallowaysoftware/toqui-backend/internal/chatstore"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui-backend/internal/lifecycle"
	"github.com/gallowaysoftware/toqui-backend/internal/persona"
	"github.com/gallowaysoftware/toqui-backend/internal/theme"
	"github.com/gallowaysoftware/toqui-backend/internal/trip"
)

// TestEnv holds all services needed to run AI integration tests in-process.
// It mirrors the wiring in cmd/server/main.go but without the HTTP server.
type TestEnv struct {
	Pool    *pgxpool.Pool
	Queries *dbgen.Queries

	// Core services (the system under test)
	ChatSvc      *chat.Service
	TripSvc      *trip.Service
	ThemeSvc     *theme.Service
	LifecycleSvc *lifecycle.Service
	ChatStore    *chatstore.Store

	// AI providers
	Provider      ai.Provider // System-under-test AND judge (same provider)
	ProviderName  string      // "claude" or "openai"
	ToolRegistry  *tools.Registry
	PersonaReg    *persona.Registry
}

// NewTestEnv constructs the full service graph for AI tests.
// Requires: DATABASE_URL (or default), FIRESTORE_EMULATOR_HOST, and an AI provider key.
func NewTestEnv(t *testing.T) *TestEnv {
	t.Helper()
	ctx := context.Background()

	// Database
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://toqui:toqui@localhost:5432/toqui?sslmode=disable"
	}

	slog.Info("aitest: running migrations")
	m, err := migrate.New("file://../../db/migrations", dbURL)
	if err != nil {
		t.Fatalf("aitest: create migrator: %v", err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		t.Fatalf("aitest: migrate up: %v", err)
	}
	m.Close()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("aitest: create pool: %v", err)
	}
	slog.Info("aitest: database ready")

	// Firestore emulator
	emulatorHost := os.Getenv("FIRESTORE_EMULATOR_HOST")
	if emulatorHost == "" {
		t.Skip("FIRESTORE_EMULATOR_HOST not set — skipping AI test")
	}
	os.Setenv("FIRESTORE_EMULATOR_HOST", emulatorHost)

	projectID := os.Getenv("FIRESTORE_PROJECT_ID")
	if projectID == "" {
		projectID = "toqui-test"
	}

	fsClient, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		t.Fatalf("aitest: create firestore client: %v", err)
	}

	// AI provider
	anthropicKey := os.Getenv("ANTHROPIC_API_KEY")
	openaiKey := os.Getenv("OPENAI_API_KEY")
	if anthropicKey == "" && openaiKey == "" {
		t.Skip("No AI provider key set (ANTHROPIC_API_KEY or OPENAI_API_KEY) — skipping AI test")
	}

	var provider ai.Provider
	var providerName string
	if anthropicKey != "" {
		provider = ai.NewClaudeProvider(anthropicKey)
		providerName = "claude"
	} else {
		provider = ai.NewOpenAIProvider(openaiKey)
		providerName = "openai"
	}
	slog.Info("aitest: AI provider configured", "provider", providerName)

	// Build service graph
	chatStr := chatstore.New(fsClient)
	toolRegistry := tools.NewRegistry()
	personaComposer := persona.NewComposer(nil) // nil = template fallback (no AI token spend)
	personaReg := persona.NewRegistry(personaComposer)

	tagger := persona.NewThemeTagger(newSimpleChatFn(provider))
	themeSvc := theme.NewService(pool, tagger)
	tripSvc := trip.NewService(pool)
	chatSvc := chat.NewService(provider, chatStr, toolRegistry, personaReg)
	lifecycleSvc := lifecycle.NewService(pool, chatStr)

	t.Cleanup(func() {
		pool.Close()
		fsClient.Close()
	})

	slog.Info("aitest: test environment ready",
		"provider", providerName,
		"firestore_project", projectID,
	)

	return &TestEnv{
		Pool:         pool,
		Queries:      dbgen.New(pool),
		ChatSvc:      chatSvc,
		TripSvc:      tripSvc,
		ThemeSvc:     themeSvc,
		LifecycleSvc: lifecycleSvc,
		ChatStore:    chatStr,
		Provider:     provider,
		ProviderName: providerName,
		ToolRegistry: toolRegistry,
		PersonaReg:   personaReg,
	}
}

// CreateTestUser inserts a synthetic user and returns their UUID.
func (e *TestEnv) CreateTestUser(ctx context.Context, name, email string) (uuid.UUID, error) {
	user, err := e.Queries.UpsertUserByGoogleID(ctx, dbgen.UpsertUserByGoogleIDParams{
		GoogleID:  "aitest-" + uuid.New().String(),
		Email:     email,
		Name:      pgtype.Text{String: name, Valid: true},
		AvatarUrl: pgtype.Text{},
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("create test user: %w", err)
	}
	return user.ID, nil
}

// CleanupUser deletes all data for a test user (trips, chat, Firestore).
func (e *TestEnv) CleanupUser(ctx context.Context, userID uuid.UUID) {
	// Delete all trips (cascades to trip_themes, itinerary_items, bookings)
	trips, _, _ := e.TripSvc.ListByUser(ctx, userID, "", 100, 0)
	for _, t := range trips {
		_ = e.LifecycleSvc.DeleteTrip(ctx, userID, t.ID)
	}
	// Delete the user row
	_, _ = e.Pool.Exec(ctx, "DELETE FROM users WHERE id = $1", userID)
}

// BuildSelectionContext replicates handlers.ChatHandler.buildSelectionContext
// for use in test scenarios that need handler-level wiring.
func (e *TestEnv) BuildSelectionContext(ctx context.Context, userID uuid.UUID) string {
	trips, _, err := e.TripSvc.ListByUser(ctx, userID, "", 20, 0)
	if err != nil || len(trips) == 0 {
		return `You are in SELECTION mode — no trip is selected yet.

Help the user decide on a trip. You can:
- Help them brainstorm destinations and trip ideas
- Create a trip for them when they're ready (use the create_trip tool)

The user has no existing trips yet. Help them get started!

When the user expresses interest in a specific destination or trip idea, proactively create the trip for them using the create_trip tool. Don't wait for them to explicitly say "create a trip" — if they say something like "I want to go to Japan" or "planning a weekend in Paris", go ahead and create it.`
	}

	var sb strings.Builder
	sb.WriteString(`You are in SELECTION mode — no trip is selected yet.

Help the user decide on a trip. You can:
- Help them brainstorm destinations and trip ideas
- Select an existing trip if the user refers to one (use the select_trip tool with the trip_id)
- Create a NEW trip when they're ready (use the create_trip tool)

IMPORTANT: When the user vaguely refers to an existing trip (e.g., "that Japan thing", "continue planning my Europe trip", "the one from last week"), use your best judgment to match it to a trip from the list below and call select_trip. Always briefly acknowledge which trip you're selecting before calling the tool, e.g., "Let me pull up your Greek Islands trip!" If you're unsure which trip they mean, ask them to clarify.

When the user expresses interest in a NEW destination or trip idea, proactively create the trip using create_trip. Don't wait for them to explicitly say "create a trip" — if they say something like "I want to go to Japan" or "planning a weekend in Paris" and there's no matching existing trip, go ahead and create it.

The user's existing trips:
`)
	for _, t := range trips {
		sb.WriteString(fmt.Sprintf("- %s (id: %s, status: %s", t.Title, t.ID, t.Status))
		if t.DestinationCountry.Valid && t.DestinationCountry.String != "" {
			sb.WriteString(fmt.Sprintf(", destination: %s", t.DestinationCountry.String))
		}
		sb.WriteString(")")
		if t.Description.Valid && t.Description.String != "" {
			sb.WriteString(fmt.Sprintf(" — %s", t.Description.String))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// BuildTripContext replicates handlers.buildTripContext for planning/companion modes.
func BuildTripContext(title, description, destinationCountry string, themes []string) string {
	if title == "" && description == "" && destinationCountry == "" {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("CURRENT TRIP CONTEXT:\n")
	if title != "" {
		sb.WriteString(fmt.Sprintf("- Title: %s\n", title))
	}
	if description != "" {
		sb.WriteString(fmt.Sprintf("- Description: %s\n", description))
	}
	if destinationCountry != "" {
		sb.WriteString(fmt.Sprintf("- Destination country: %s\n", destinationCountry))
	}
	if len(themes) > 0 {
		sb.WriteString(fmt.Sprintf("- Trip themes: %s\n", strings.Join(themes, ", ")))
	}
	sb.WriteString("\nUse this context to give specific, relevant advice. Do NOT ask the user where they are going — you already know from the trip details above.")
	return sb.String()
}

// newSimpleChatFn creates the simple chat function that ThemeTagger expects.
func newSimpleChatFn(provider ai.Provider) func(ctx context.Context, system, prompt string) (string, error) {
	return func(ctx context.Context, system, prompt string) (string, error) {
		req := &ai.ChatRequest{
			SystemPrompt: system,
			Messages:     []ai.Message{{Role: "user", Content: prompt}},
			MaxTokens:    1024,
			Temperature:  0.3,
		}
		eventCh, err := provider.ChatStream(ctx, req)
		if err != nil {
			return "", err
		}

		var result strings.Builder
		for event := range eventCh {
			switch event.Type {
			case ai.EventTextDelta:
				result.WriteString(event.Text)
			case ai.EventError:
				return "", event.Error
			}
		}
		return result.String(), nil
	}
}
