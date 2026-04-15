package usage

import (
	"context"
	"errors"
	"testing"

	"github.com/gallowaysoftware/toqui-backend/internal/tier"
)

// mockCostQuerier is a test double for the dailyCostQuerier interface.
type mockCostQuerier struct {
	globalCost int64
	tierCosts  map[string]int64
	globalErr  error
	tierErr    error
}

func (m *mockCostQuerier) GetDailyAIUsageCost(_ context.Context) (int64, error) {
	return m.globalCost, m.globalErr
}

func (m *mockCostQuerier) GetDailyAIUsageCostByTier(_ context.Context, userTier string) (int64, error) {
	if m.tierErr != nil {
		return 0, m.tierErr
	}
	if m.tierCosts != nil {
		return m.tierCosts[userTier], nil
	}
	return m.globalCost, nil
}

func TestBudgetConfig_TierBudgetCents(t *testing.T) {
	cfg := BudgetConfig{
		GlobalDailyCents: 10000, // $100
		FreePct:          20,
		ProPct:           30,
		ExplorerPct:      25,
		VoyagerPct:       25,
	}

	tests := []struct {
		tier tier.UserTier
		want int64
	}{
		{tier.Free, 2000},     // 20% of 10000
		{tier.Pro, 3000},      // 30% of 10000
		{tier.Explorer, 2500}, // 25% of 10000
		{tier.Voyager, 2500},  // 25% of 10000
	}

	for _, tt := range tests {
		t.Run(string(tt.tier), func(t *testing.T) {
			got := cfg.TierBudgetCents(tt.tier)
			if got != tt.want {
				t.Errorf("TierBudgetCents(%q) = %d, want %d", tt.tier, got, tt.want)
			}
		})
	}
}

func TestBudgetConfig_TierBudgetCents_ZeroPct(t *testing.T) {
	cfg := BudgetConfig{
		GlobalDailyCents: 10000,
		FreePct:          0, // no per-tier limit → falls back to global
	}

	got := cfg.TierBudgetCents(tier.Free)
	if got != 10000 {
		t.Errorf("expected global fallback 10000, got %d", got)
	}
}

func TestBudgetConfig_TierBudgetCents_Unlimited(t *testing.T) {
	cfg := BudgetConfig{
		GlobalDailyCents: 0, // unlimited
		FreePct:          20,
	}

	got := cfg.TierBudgetCents(tier.Free)
	if got != 0 {
		t.Errorf("expected 0 (unlimited), got %d", got)
	}
}

func TestBudgetChecker_Check_Unlimited(t *testing.T) {
	bc := NewBudgetChecker(BudgetConfig{GlobalDailyCents: 0}, nil)
	if err := bc.Check(context.Background(), tier.Free); err != nil {
		t.Errorf("unlimited budget should never fail, got: %v", err)
	}
}

func TestBudgetChecker_Check_NilQuerier(t *testing.T) {
	bc := NewBudgetChecker(BudgetConfig{GlobalDailyCents: 1000}, nil)
	if err := bc.Check(context.Background(), tier.Free); err != nil {
		t.Errorf("nil querier should fail open, got: %v", err)
	}
}

func TestBudgetChecker_Check_UnderBudget(t *testing.T) {
	q := &mockCostQuerier{
		globalCost: 500,
		tierCosts:  map[string]int64{"free": 100},
	}
	cfg := BudgetConfig{
		GlobalDailyCents: 10000,
		FreePct:          20, // 2000 cents for free
	}
	bc := NewBudgetChecker(cfg, q)

	if err := bc.Check(context.Background(), tier.Free); err != nil {
		t.Errorf("under budget should pass, got: %v", err)
	}
}

func TestBudgetChecker_Check_GlobalExceeded(t *testing.T) {
	q := &mockCostQuerier{
		globalCost: 10001, // exceeds 10000
		tierCosts:  map[string]int64{"free": 100},
	}
	cfg := BudgetConfig{
		GlobalDailyCents: 10000,
		FreePct:          20,
	}
	bc := NewBudgetChecker(cfg, q)

	err := bc.Check(context.Background(), tier.Free)
	if !errors.Is(err, ErrBudgetExceeded) {
		t.Errorf("expected ErrBudgetExceeded, got: %v", err)
	}
}

func TestBudgetChecker_Check_TierExceeded(t *testing.T) {
	q := &mockCostQuerier{
		globalCost: 5000,
		tierCosts:  map[string]int64{"free": 2500}, // exceeds 2000 (20% of 10000)
	}
	cfg := BudgetConfig{
		GlobalDailyCents: 10000,
		FreePct:          20,
	}
	bc := NewBudgetChecker(cfg, q)

	err := bc.Check(context.Background(), tier.Free)
	if !errors.Is(err, ErrBudgetExceeded) {
		t.Errorf("expected ErrBudgetExceeded for tier over-budget, got: %v", err)
	}
}

func TestBudgetChecker_Check_TierExceeded_GlobalOK(t *testing.T) {
	// Tier budget exceeded but global is fine — should still block.
	q := &mockCostQuerier{
		globalCost: 3000,
		tierCosts:  map[string]int64{"free": 2100},
	}
	cfg := BudgetConfig{
		GlobalDailyCents: 10000,
		FreePct:          20, // 2000 for free
	}
	bc := NewBudgetChecker(cfg, q)

	err := bc.Check(context.Background(), tier.Free)
	if !errors.Is(err, ErrBudgetExceeded) {
		t.Errorf("expected ErrBudgetExceeded when tier exceeds its allocation, got: %v", err)
	}
}

func TestBudgetChecker_Check_DBError_FailsOpen(t *testing.T) {
	q := &mockCostQuerier{
		globalErr: errors.New("connection refused"),
	}
	cfg := BudgetConfig{
		GlobalDailyCents: 10000,
	}
	bc := NewBudgetChecker(cfg, q)

	// Should fail open on DB error.
	if err := bc.Check(context.Background(), tier.Free); err != nil {
		t.Errorf("expected fail-open on DB error, got: %v", err)
	}
}

func TestBudgetChecker_Check_TierQueryError_FallsBackToGlobal(t *testing.T) {
	q := &mockCostQuerier{
		globalCost: 5000,
		tierErr:    errors.New("tier query broken"),
	}
	cfg := BudgetConfig{
		GlobalDailyCents: 10000,
		FreePct:          20, // 2000
	}
	bc := NewBudgetChecker(cfg, q)

	// When tier query fails, tierCost falls back to globalCost (5000) which
	// exceeds the free tier allocation of 2000.
	err := bc.Check(context.Background(), tier.Free)
	if !errors.Is(err, ErrBudgetExceeded) {
		t.Errorf("expected ErrBudgetExceeded with fallback, got: %v", err)
	}
}

func TestBudgetChecker_Check_VoyagerNoPctLimit(t *testing.T) {
	q := &mockCostQuerier{
		globalCost: 5000,
		tierCosts:  map[string]int64{"voyager": 4999},
	}
	cfg := BudgetConfig{
		GlobalDailyCents: 10000,
		VoyagerPct:       0, // no per-tier limit for voyager
	}
	bc := NewBudgetChecker(cfg, q)

	// Should pass — no per-tier limit means only global matters.
	if err := bc.Check(context.Background(), tier.Voyager); err != nil {
		t.Errorf("voyager with no pct limit should pass, got: %v", err)
	}
}

func TestBudgetChecker_Utilization(t *testing.T) {
	q := &mockCostQuerier{globalCost: 7500}
	cfg := BudgetConfig{GlobalDailyCents: 10000}
	bc := NewBudgetChecker(cfg, q)

	pct, costCents, budgetCents, err := bc.Utilization(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pct != 75.0 {
		t.Errorf("expected 75%%, got %f", pct)
	}
	if costCents != 7500 {
		t.Errorf("expected costCents=7500, got %d", costCents)
	}
	if budgetCents != 10000 {
		t.Errorf("expected budgetCents=10000, got %d", budgetCents)
	}
}

func TestBudgetChecker_Utilization_Unlimited(t *testing.T) {
	bc := NewBudgetChecker(BudgetConfig{GlobalDailyCents: 0}, nil)

	pct, costCents, budgetCents, err := bc.Utilization(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pct != 0 || costCents != 0 || budgetCents != 0 {
		t.Errorf("expected zeros for unlimited, got pct=%f cost=%d budget=%d", pct, costCents, budgetCents)
	}
}

func TestBudgetChecker_Utilization_CapsAt100(t *testing.T) {
	q := &mockCostQuerier{globalCost: 15000}
	cfg := BudgetConfig{GlobalDailyCents: 10000}
	bc := NewBudgetChecker(cfg, q)

	pct, _, _, err := bc.Utilization(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pct != 100 {
		t.Errorf("expected capped at 100%%, got %f", pct)
	}
}

func TestBudgetChecker_CachingBehavior(t *testing.T) {
	callCount := 0
	q := &mockCostQuerier{globalCost: 500, tierCosts: map[string]int64{"free": 100}}
	original := q.GetDailyAIUsageCost
	_ = original

	// Wrap to count calls
	wrapper := &countingQuerier{inner: q}
	cfg := BudgetConfig{GlobalDailyCents: 10000, FreePct: 20}
	bc := NewBudgetChecker(cfg, wrapper)

	ctx := context.Background()
	_ = bc.Check(ctx, tier.Free) // first call: cache miss → queries DB
	_ = bc.Check(ctx, tier.Free) // second call: cache hit
	_ = bc.Check(ctx, tier.Free) // third call: cache hit

	callCount = wrapper.calls
	if callCount != 1 {
		t.Errorf("expected 1 DB call (cached), got %d", callCount)
	}
}

// countingQuerier wraps a dailyCostQuerier and counts GetDailyAIUsageCost calls.
type countingQuerier struct {
	inner dailyCostQuerier
	calls int
}

func (c *countingQuerier) GetDailyAIUsageCost(ctx context.Context) (int64, error) {
	c.calls++
	return c.inner.GetDailyAIUsageCost(ctx)
}

func (c *countingQuerier) GetDailyAIUsageCostByTier(ctx context.Context, userTier string) (int64, error) {
	return c.inner.GetDailyAIUsageCostByTier(ctx, userTier)
}

func TestErrBudgetExceeded(t *testing.T) {
	if ErrBudgetExceeded == nil {
		t.Fatal("ErrBudgetExceeded should not be nil")
	}
	if ErrBudgetExceeded.Error() != "daily AI cost budget exceeded" {
		t.Errorf("unexpected error message: %s", ErrBudgetExceeded.Error())
	}
}
