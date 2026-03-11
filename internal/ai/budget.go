package ai

import (
	"errors"
	"sync/atomic"
	"time"
)

// ErrBudgetExhausted is returned when the daily AI token budget has been exceeded.
var ErrBudgetExhausted = errors.New("daily AI token budget exhausted — please try again tomorrow")

// TokenBudget enforces a global daily token limit across all AI calls.
// It resets automatically at midnight UTC each day.
// A limit of 0 means unlimited.
type TokenBudget struct {
	limit    int64
	used     atomic.Int64
	resetDay atomic.Int64 // day-of-year when last reset occurred
}

// NewTokenBudget creates a daily token budget. Pass 0 for unlimited.
func NewTokenBudget(dailyLimit int) *TokenBudget {
	b := &TokenBudget{
		limit: int64(dailyLimit),
	}
	b.resetDay.Store(int64(time.Now().UTC().YearDay()))
	return b
}

// Check returns ErrBudgetExhausted if the daily budget has been exceeded.
// Call this before starting an AI request.
func (b *TokenBudget) Check() error {
	if b.limit <= 0 {
		return nil // unlimited
	}
	b.maybeReset()
	if b.used.Load() >= b.limit {
		return ErrBudgetExhausted
	}
	return nil
}

// Record adds tokens to the daily counter.
// Call this after an AI request completes with the total tokens used.
func (b *TokenBudget) Record(tokens int) {
	if b.limit <= 0 {
		return // unlimited, no need to track
	}
	b.maybeReset()
	b.used.Add(int64(tokens))
}

// maybeReset resets the counter if we've crossed into a new UTC day.
func (b *TokenBudget) maybeReset() {
	today := int64(time.Now().UTC().YearDay())
	lastReset := b.resetDay.Load()
	if today != lastReset {
		if b.resetDay.CompareAndSwap(lastReset, today) {
			b.used.Store(0)
		}
	}
}
