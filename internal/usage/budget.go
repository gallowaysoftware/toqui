package usage

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/gallowaysoftware/toqui-backend/internal/tier"
)

// ErrBudgetExceeded is returned when the global daily AI cost budget has been exceeded.
var ErrBudgetExceeded = errors.New("daily AI cost budget exceeded")

// BudgetConfig holds budget limits in cents.
type BudgetConfig struct {
	// GlobalDailyCents is the hard ceiling for total AI spend across all users
	// in a calendar day (UTC). 0 means unlimited.
	GlobalDailyCents int64

	// Per-tier allocation as a percentage of GlobalDailyCents (0–100).
	// If a tier's allocation is 0, it falls back to the global limit.
	// These let operators reserve capacity so free users can't exhaust the
	// entire budget before paying customers get served.
	FreePct     int // e.g. 20 → free tier gets 20% of GlobalDailyCents
	ProPct      int // e.g. 30
	ExplorerPct int // e.g. 25
	VoyagerPct  int // e.g. 25
}

// TierBudgetCents returns the daily budget ceiling in cents for the given tier.
// Returns 0 (unlimited) when no global limit is configured.
func (c BudgetConfig) TierBudgetCents(t tier.UserTier) int64 {
	if c.GlobalDailyCents <= 0 {
		return 0
	}
	pct := c.pctForTier(t)
	if pct <= 0 {
		return c.GlobalDailyCents
	}
	return c.GlobalDailyCents * int64(pct) / 100
}

func (c BudgetConfig) pctForTier(t tier.UserTier) int {
	switch t {
	case tier.Free:
		return c.FreePct
	case tier.Pro:
		return c.ProPct
	case tier.Explorer:
		return c.ExplorerPct
	case tier.Voyager:
		return c.VoyagerPct
	default:
		return 0
	}
}

// dailyCostQuerier is the subset of dbgen.Queries needed by BudgetChecker.
type dailyCostQuerier interface {
	GetDailyAIUsageCost(ctx context.Context) (int64, error)
	GetDailyAIUsageCostByTier(ctx context.Context, userTier string) (int64, error)
}

// BudgetChecker enforces a daily AI cost hard limit backed by the ai_usage table.
// It caches the latest cost reading for a short TTL to avoid hitting the DB on
// every single AI call (the usage service already records costs after each call,
// so a small staleness window is acceptable).
type BudgetChecker struct {
	cfg   BudgetConfig
	query dailyCostQuerier

	mu             sync.RWMutex
	cachedGlobal   int64
	cachedTierCost map[string]int64
	cachedAt       time.Time
	cacheTTL       time.Duration

	// Track which warning thresholds have fired today to avoid log spam.
	// Reset when the date changes.
	warnDate     string
	warned70     bool
	warned90     bool
	warned70Tier map[string]bool
	warned90Tier map[string]bool
}

// NewBudgetChecker creates a new cost-based daily budget enforcer.
// Pass a nil querier to disable DB-backed checks (falls back to unlimited).
func NewBudgetChecker(cfg BudgetConfig, query dailyCostQuerier) *BudgetChecker {
	return &BudgetChecker{
		cfg:            cfg,
		query:          query,
		cacheTTL:       10 * time.Second,
		cachedTierCost: make(map[string]int64),
		warned70Tier:   make(map[string]bool),
		warned90Tier:   make(map[string]bool),
	}
}

// Check verifies whether the daily budget allows another AI call.
// It checks both the global budget and the per-tier allocation.
// Returns ErrBudgetExceeded if either limit is hit.
// Returns nil if budget is unlimited or the call is within budget.
func (b *BudgetChecker) Check(ctx context.Context, userTier tier.UserTier) error {
	if b.cfg.GlobalDailyCents <= 0 {
		return nil // unlimited
	}
	if b.query == nil {
		return nil // no DB, can't enforce
	}

	globalCost, tierCost, err := b.getCosts(ctx, userTier)
	if err != nil {
		// On DB error, fail open — better to allow the call than block all
		// AI traffic because of a transient query failure.
		slog.Warn("budget check: failed to query daily cost, allowing request",
			"error", err,
		)
		return nil
	}

	// Reset warning state on day change.
	b.resetWarningsIfNewDay()

	// Check per-tier budget first (more specific).
	tierBudget := b.cfg.TierBudgetCents(userTier)
	if tierBudget > 0 && tierCost >= tierBudget {
		slog.Error("daily AI budget exceeded for tier",
			"tier", string(userTier),
			"cost_cents", tierCost,
			"budget_cents", tierBudget,
		)
		return ErrBudgetExceeded
	}

	// Check global budget.
	if globalCost >= b.cfg.GlobalDailyCents {
		slog.Error("global daily AI budget exceeded",
			"cost_cents", globalCost,
			"budget_cents", b.cfg.GlobalDailyCents,
		)
		return ErrBudgetExceeded
	}

	// Log threshold warnings.
	b.logWarnings(globalCost, tierCost, userTier, tierBudget)

	return nil
}

// Utilization returns the current budget utilization as a percentage (0–100)
// and the raw cost/budget values in cents. Useful for admin dashboards.
func (b *BudgetChecker) Utilization(ctx context.Context) (pct float64, costCents, budgetCents int64, err error) {
	budgetCents = b.cfg.GlobalDailyCents
	if budgetCents <= 0 {
		return 0, 0, 0, nil
	}
	if b.query == nil {
		return 0, 0, budgetCents, nil
	}

	costCents, err = b.query.GetDailyAIUsageCost(ctx)
	if err != nil {
		return 0, 0, budgetCents, fmt.Errorf("query daily cost: %w", err)
	}

	pct = float64(costCents) / float64(budgetCents) * 100
	if pct > 100 {
		pct = 100
	}
	return pct, costCents, budgetCents, nil
}

// getCosts returns the cached or fresh global and per-tier cost values.
func (b *BudgetChecker) getCosts(ctx context.Context, userTier tier.UserTier) (global, tierCost int64, err error) {
	b.mu.RLock()
	if time.Since(b.cachedAt) < b.cacheTTL {
		g := b.cachedGlobal
		tc := b.cachedTierCost[string(userTier)]
		b.mu.RUnlock()
		return g, tc, nil
	}
	b.mu.RUnlock()

	// Cache miss — query the DB.
	g, err := b.query.GetDailyAIUsageCost(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("query global daily cost: %w", err)
	}

	tc, tierErr := b.query.GetDailyAIUsageCostByTier(ctx, string(userTier))
	if tierErr != nil {
		// If per-tier query fails, use global as fallback.
		slog.Warn("budget check: per-tier cost query failed, using global",
			"tier", string(userTier),
			"error", tierErr,
		)
		tc = g
	}

	b.mu.Lock()
	b.cachedGlobal = g
	b.cachedTierCost[string(userTier)] = tc
	b.cachedAt = time.Now()
	b.mu.Unlock()

	return g, tc, nil
}

// logWarnings emits slog warnings at 70% and errors at 90% utilization.
func (b *BudgetChecker) logWarnings(globalCost, tierCost int64, userTier tier.UserTier, tierBudget int64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Global warnings.
	globalPct := float64(globalCost) / float64(b.cfg.GlobalDailyCents) * 100
	if globalPct >= 90 && !b.warned90 {
		b.warned90 = true
		slog.Error("daily AI budget at 90%+ utilization",
			"cost_cents", globalCost,
			"budget_cents", b.cfg.GlobalDailyCents,
			"utilization_pct", globalPct,
		)
	} else if globalPct >= 70 && !b.warned70 {
		b.warned70 = true
		slog.Warn("daily AI budget at 70%+ utilization",
			"cost_cents", globalCost,
			"budget_cents", b.cfg.GlobalDailyCents,
			"utilization_pct", globalPct,
		)
	}

	// Per-tier warnings.
	if tierBudget > 0 {
		tierPct := float64(tierCost) / float64(tierBudget) * 100
		tierKey := string(userTier)
		if tierPct >= 90 && !b.warned90Tier[tierKey] {
			b.warned90Tier[tierKey] = true
			slog.Error("daily AI budget at 90%+ for tier",
				"tier", tierKey,
				"cost_cents", tierCost,
				"budget_cents", tierBudget,
				"utilization_pct", tierPct,
			)
		} else if tierPct >= 70 && !b.warned70Tier[tierKey] {
			b.warned70Tier[tierKey] = true
			slog.Warn("daily AI budget at 70%+ for tier",
				"tier", tierKey,
				"cost_cents", tierCost,
				"budget_cents", tierBudget,
				"utilization_pct", tierPct,
			)
		}
	}
}

// resetWarningsIfNewDay clears the warning flags when the UTC date changes.
func (b *BudgetChecker) resetWarningsIfNewDay() {
	today := time.Now().UTC().Format("2006-01-02")
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.warnDate != today {
		b.warnDate = today
		b.warned70 = false
		b.warned90 = false
		b.warned70Tier = make(map[string]bool)
		b.warned90Tier = make(map[string]bool)
	}
}
