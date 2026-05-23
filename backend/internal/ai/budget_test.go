package ai

import (
	"errors"
	"sync"
	"testing"
)

// TokenBudget is the kill-switch that prevents a runaway AI cost spike
// from draining the daily budget. Each test pins a specific failure mode
// so a regression that broke the kill-switch (e.g. miscounted, wrong
// reset behaviour) gets caught before it shows up as an Anthropic bill.

func TestTokenBudget_ZeroLimit_IsUnlimited(t *testing.T) {
	// limit=0 means unlimited — the staging/local default. Check() must
	// never error and Record() must be a cheap no-op.
	b := NewTokenBudget(0)
	if err := b.Check(); err != nil {
		t.Errorf("Check() on unlimited budget = %v, want nil", err)
	}
	// Record arbitrary amounts; Check() should still pass.
	b.Record(1_000_000)
	if err := b.Check(); err != nil {
		t.Errorf("Check() after Record(1M) on unlimited = %v, want nil", err)
	}
}

func TestTokenBudget_WithinLimit_PassesCheck(t *testing.T) {
	b := NewTokenBudget(1000)
	b.Record(500)
	if err := b.Check(); err != nil {
		t.Errorf("Check() at 500/1000 used = %v, want nil", err)
	}
}

func TestTokenBudget_AtLimit_ReturnsErrBudgetExhausted(t *testing.T) {
	// At-limit case: used >= limit → exhausted. Pin the boundary so a
	// future change that flipped >= to > would let one extra request
	// through over the cap.
	b := NewTokenBudget(1000)
	b.Record(1000)
	err := b.Check()
	if !errors.Is(err, ErrBudgetExhausted) {
		t.Errorf("Check() at exactly limit = %v, want ErrBudgetExhausted", err)
	}
}

func TestTokenBudget_OverLimit_ReturnsErrBudgetExhausted(t *testing.T) {
	// Going over the limit can happen because Record() is called AFTER
	// the AI request completes — a single call that produces a large
	// response can push us past the limit. The next call's Check() must
	// reject; pin that behaviour.
	b := NewTokenBudget(1000)
	b.Record(1500)
	err := b.Check()
	if !errors.Is(err, ErrBudgetExhausted) {
		t.Errorf("Check() over limit = %v, want ErrBudgetExhausted", err)
	}
}

func TestTokenBudget_RecordOnUnlimitedIsNoop(t *testing.T) {
	// Unlimited budget should NOT track usage internally — saves the
	// atomic add on every AI request when the kill-switch is off.
	// Verifies via internal field that we don't accidentally regress
	// this hot-path optimisation.
	b := NewTokenBudget(0)
	b.Record(500)
	if got := b.used.Load(); got != 0 {
		t.Errorf("used after Record on unlimited = %d, want 0 (no-op path)", got)
	}
}

func TestTokenBudget_NewBudget_StartsAtZero(t *testing.T) {
	b := NewTokenBudget(1000)
	if got := b.used.Load(); got != 0 {
		t.Errorf("new budget used = %d, want 0", got)
	}
	// And reset day should be initialized to today.
	day := b.resetDay.Load()
	if day == nil || day.(string) == "" {
		t.Errorf("resetDay not initialized: %v", day)
	}
}

func TestTokenBudget_ResetClearsUsed(t *testing.T) {
	// maybeReset is normally triggered by date rollover (UTC midnight).
	// We simulate it here by stomping the resetDay field with a stale
	// value, then calling Check() / Record() which trigger the reset.
	b := NewTokenBudget(1000)
	b.Record(800) // close to limit
	if err := b.Check(); err != nil {
		t.Fatalf("pre-reset Check at 800/1000 should pass, got %v", err)
	}
	// Stomp resetDay with a stale value to force a reset.
	b.resetDay.Store("1970-01-01")
	// First Check after the stomp triggers maybeReset which CAS's the
	// new date and clears used to 0. After this we should be back to
	// 0/1000 used.
	if err := b.Check(); err != nil {
		t.Errorf("Check post-reset = %v, want nil", err)
	}
	if got := b.used.Load(); got != 0 {
		t.Errorf("used after stale-day reset = %d, want 0", got)
	}
}

func TestTokenBudget_ConcurrentRecord_AccumulatesAtomically(t *testing.T) {
	// Hot path: many goroutines call Record concurrently as AI requests
	// complete. The atomic counter must accumulate exactly — a race
	// here would let the budget overrun silently. Pre-existing CAS
	// pattern is correct; this test pins the contract so a future
	// non-atomic refactor breaks loudly.
	b := NewTokenBudget(1_000_000)
	const goroutines = 100
	const tokensPerGoroutine = 1000

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			b.Record(tokensPerGoroutine)
		}()
	}
	wg.Wait()

	want := int64(goroutines * tokensPerGoroutine)
	if got := b.used.Load(); got != want {
		t.Errorf("used after %d concurrent Record(%d) = %d, want %d",
			goroutines, tokensPerGoroutine, got, want)
	}
}

func TestTokenBudget_ResetDoubleCallSafe(t *testing.T) {
	// maybeReset uses CompareAndSwap to ensure only one goroutine
	// performs the reset when multiple racing goroutines see the same
	// stale date. Pin that the second concurrent caller doesn't double-
	// clear used (which would lose data accumulated during the race).
	b := NewTokenBudget(1000)
	b.Record(500)
	b.resetDay.Store("1970-01-01") // simulate stale day

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _ = b.Check() }()
	go func() { defer wg.Done(); b.Record(100) }()
	wg.Wait()

	// Used should be either 0 (reset won), 100 (reset won then Record
	// added 100 after), or 600 (reset lost — original 500 + 100).
	// Either way it must NOT be negative or above 600 (which would
	// suggest double-add). We pin the bounds.
	got := b.used.Load()
	if got < 0 || got > 600 {
		t.Errorf("used after racing reset+Record = %d, want in [0, 600]", got)
	}
}
