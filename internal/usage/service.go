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
	"github.com/gallowaysoftware/toqui-backend/internal/tier"
)

// ErrDailyLimitExceeded is returned when a user has exceeded their daily message limit.
var ErrDailyLimitExceeded = errors.New("daily message limit exceeded")

// Service tracks per-user daily usage and enforces message limits.
type Service struct {
	queries   *dbgen.Queries
	limitFree int // daily limit for free tier
	limitPro  int // daily limit for pro tier
}

// NewService creates a new usage tracking service.
// The dailyMessageLimit is used as the default for both free and pro tiers.
// Call WithTierLimits to set tier-specific limits.
func NewService(pool *pgxpool.Pool, dailyMessageLimit int) *Service {
	return &Service{
		queries:   dbgen.New(pool),
		limitFree: dailyMessageLimit,
		limitPro:  dailyMessageLimit,
	}
}

// WithTierLimits configures tier-specific daily message limits.
// A limit of 0 means unlimited for that tier.
func (s *Service) WithTierLimits(free, pro int) *Service {
	s.limitFree = free
	s.limitPro = pro
	return s
}

// LimitForTier returns the daily message limit for a given tier.
// Returns 0 for unlimited tiers (Explorer, Voyager).
func (s *Service) LimitForTier(t tier.UserTier) int {
	if t.IsUnlimited() {
		return 0 // unlimited
	}
	if t.IsPro() {
		return s.limitPro
	}
	return s.limitFree
}

// IncrementAndCheckTier atomically increments today's message count for the user
// and checks whether the tier-specific daily limit has been exceeded.
//
// Returns the number of messages remaining (0 if at or over limit).
// Returns ErrDailyLimitExceeded if the limit was already reached (counter NOT incremented).
// For unlimited tiers, always succeeds and returns a high remaining count.
func (s *Service) IncrementAndCheckTier(ctx context.Context, userID uuid.UUID, t tier.UserTier) (remaining int, err error) {
	limit := s.LimitForTier(t)

	// Unlimited tier: always increment, never reject.
	if limit == 0 {
		// Still track usage for analytics, but with a very high max to never reject.
		_, err := s.queries.IncrementDailyUsage(ctx, dbgen.IncrementDailyUsageParams{
			UserID:   userID,
			MaxCount: int32(999999),
		})
		if err != nil {
			return 0, fmt.Errorf("increment daily usage (unlimited): %w", err)
		}
		return 999999, nil
	}

	// Guard: a limit of 0 means messaging is disabled (e.g., suspended user).
	// The SQL WHERE clause (count < 0) would always be false, but the INSERT
	// path would still create a row with count=1 — bypassing the intent.
	if limit < 0 {
		return 0, ErrDailyLimitExceeded
	}

	usage, err := s.queries.IncrementDailyUsage(ctx, dbgen.IncrementDailyUsageParams{
		UserID:   userID,
		MaxCount: int32(limit),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			slog.Info("daily message limit exceeded",
				"user_id", userID,
				"limit", limit,
				"tier", string(t),
			)
			return 0, ErrDailyLimitExceeded
		}
		return 0, fmt.Errorf("increment daily usage: %w", err)
	}

	count := int(usage.MessageCount)
	remaining = limit - count
	return remaining, nil
}

// GetDailyUsageForTier returns the current day's message count and the tier-specific limit.
// If no usage row exists for today, count is 0.
func (s *Service) GetDailyUsageForTier(ctx context.Context, userID uuid.UUID, t tier.UserTier) (count, limit int, err error) {
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	usage, err := s.queries.GetDailyUsage(ctx, dbgen.GetDailyUsageParams{
		UserID: userID,
		Date:   &today,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, s.LimitForTier(t), nil
		}
		return 0, 0, fmt.Errorf("get daily usage: %w", err)
	}

	return int(usage.MessageCount), s.LimitForTier(t), nil
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
