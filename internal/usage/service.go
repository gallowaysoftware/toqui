package usage

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
)

// ErrDailyLimitExceeded is returned when a user has exceeded their daily message limit.
var ErrDailyLimitExceeded = errors.New("daily message limit exceeded")

// Service tracks per-user daily usage and enforces message limits.
type Service struct {
	queries *dbgen.Queries
	limit   int
}

// NewService creates a new usage tracking service.
func NewService(pool *pgxpool.Pool, dailyMessageLimit int) *Service {
	return &Service{
		queries: dbgen.New(pool),
		limit:   dailyMessageLimit,
	}
}

// IncrementAndCheck atomically increments today's message count for the user
// and checks whether the daily limit has been exceeded. The increment and
// check happen in a single PostgreSQL INSERT ... ON CONFLICT ... DO UPDATE
// ... WHERE count < limit ... RETURNING statement, preventing TOCTOU race
// conditions and ensuring the counter is never incremented past the limit.
//
// Returns the number of messages remaining (0 if at or over limit).
// Returns ErrDailyLimitExceeded if the limit was already reached (counter NOT incremented).
func (s *Service) IncrementAndCheck(ctx context.Context, userID uuid.UUID) (remaining int, err error) {
	// Guard: a limit of 0 means messaging is disabled (e.g., suspended user).
	// The SQL WHERE clause (count < 0) would always be false, but the INSERT
	// path would still create a row with count=1 — bypassing the intent.
	if s.limit <= 0 {
		return 0, ErrDailyLimitExceeded
	}

	// Atomic conditional increment: only bumps the counter if it is below the
	// limit. If the user is already at the limit the WHERE clause prevents the
	// UPDATE, PostgreSQL returns no rows, and pgx surfaces pgx.ErrNoRows.
	usage, err := s.queries.IncrementDailyUsage(ctx, dbgen.IncrementDailyUsageParams{
		UserID:   userID,
		MaxCount: int32(s.limit),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			slog.Info("daily message limit exceeded",
				"user_id", userID,
				"limit", s.limit,
			)
			return 0, ErrDailyLimitExceeded
		}
		return 0, fmt.Errorf("increment daily usage: %w", err)
	}

	count := int(usage.MessageCount)
	remaining = s.limit - count
	return remaining, nil
}

// GetDailyUsage returns the current day's message count and the configured limit.
// If no usage row exists for today, count is 0.
func (s *Service) GetDailyUsage(ctx context.Context, userID uuid.UUID) (count, limit int, err error) {
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	usage, err := s.queries.GetDailyUsage(ctx, dbgen.GetDailyUsageParams{
		UserID: userID,
		Date:   &today,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, s.limit, nil
		}
		return 0, 0, fmt.Errorf("get daily usage: %w", err)
	}

	return int(usage.MessageCount), s.limit, nil
}

// Limit returns the configured daily message limit.
func (s *Service) Limit() int {
	return s.limit
}

// RecordAICost records the AI cost in cents for the current day's usage row.
func (s *Service) RecordAICost(ctx context.Context, userID uuid.UUID, costCents int32) error {
	return s.queries.RecordAICost(ctx, dbgen.RecordAICostParams{
		UserID:    userID,
		CostCents: costCents,
	})
}

// ResetTime returns the time at which the daily usage counter resets
// (midnight UTC of the next day).
func ResetTime() time.Time {
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
}
