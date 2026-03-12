package theme

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui-backend/internal/persona"
)

// Service coordinates AI theme tagging with database persistence.
type Service struct {
	queries *dbgen.Queries
	tagger  *persona.ThemeTagger
}

func NewService(pool *pgxpool.Pool, tagger *persona.ThemeTagger) *Service {
	return &Service{
		queries: dbgen.New(pool),
		tagger:  tagger,
	}
}

// TagTrip runs the AI theme tagger and persists results to the database.
// It clears previous AI-assigned themes before writing new ones.
func (s *Service) TagTrip(ctx context.Context, userID, tripID uuid.UUID, title, description string, recentMessages []string) error {
	result, err := s.tagger.AnalyzeTrip(ctx, title, description, recentMessages)
	if err != nil {
		return fmt.Errorf("tag trip: %w", err)
	}

	// Clear previous AI tags
	if err := s.queries.ClearTripThemes(ctx, tripID); err != nil {
		return fmt.Errorf("clear trip themes: %w", err)
	}

	// Write new tags
	for _, ts := range result.Themes {
		if err := s.queries.SetTripTheme(ctx, dbgen.SetTripThemeParams{
			TripID:     tripID,
			ThemeSlug:  ts.Slug,
			Confidence: float32(ts.Confidence),
			Source:     "ai",
		}); err != nil {
			slog.Warn("failed to set trip theme", "theme", ts.Slug, "error", err)
		}
	}

	// Update destination country if detected
	if result.DestinationCode != "" {
		if _, err := s.queries.UpdateTripDestination(ctx, dbgen.UpdateTripDestinationParams{
			ID:                 tripID,
			DestinationCountry: pgtype.Text{String: result.DestinationCode, Valid: true},
			UserID:             userID,
		}); err != nil {
			slog.Warn("failed to update trip destination", "error", err)
		}
	}

	return nil
}

// TagTripAsync runs TagTrip in a background goroutine. Errors are logged, not returned.
// This intentionally uses a detached context because theme tagging is a best-effort
// background job that should complete even after the originating request ends.
func (s *Service) TagTripAsync(userID, tripID uuid.UUID, title, description string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.TagTrip(ctx, userID, tripID, title, description, nil); err != nil {
			slog.Warn("async theme tagging failed", "trip_id", tripID, "error", err)
		}
	}()
}

// GetTripThemes returns the current theme slugs for a trip, ordered by confidence.
func (s *Service) GetTripThemes(ctx context.Context, tripID uuid.UUID) ([]string, error) {
	rows, err := s.queries.GetTripThemes(ctx, tripID)
	if err != nil {
		return nil, fmt.Errorf("get trip themes: %w", err)
	}
	slugs := make([]string, len(rows))
	for i, r := range rows {
		slugs[i] = r.Slug
	}
	return slugs, nil
}
