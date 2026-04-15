package analytics

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"
)

// AlertChecker monitors key health indicators and logs warnings when
// thresholds are breached. The warnings are picked up by Cloud Logging
// alerting policies in production.
//
// This is intentionally simple — no external dependencies, no state
// beyond atomic counters. The goal is to catch obvious problems early
// (error rate spikes, usage drops) without building a full monitoring system.
type AlertChecker struct {
	requestCount atomic.Int64
	errorCount   atomic.Int64
	lastMessage  atomic.Int64 // Unix timestamp of last chat message
	lastSignup   atomic.Int64 // Unix timestamp of last signup

	// Thresholds (configurable for testing).
	ErrorRateThreshold float64 // default: 0.05 (5%)
	IdleMessageHours   int     // default: 6
	IdleSignupHours    int     // default: 24
}

// NewAlertChecker creates a new alert checker with default thresholds.
func NewAlertChecker() *AlertChecker {
	now := time.Now().Unix()
	ac := &AlertChecker{
		ErrorRateThreshold: 0.05,
		IdleMessageHours:   6,
		IdleSignupHours:    24,
	}
	ac.lastMessage.Store(now)
	ac.lastSignup.Store(now)
	return ac
}

// RecordRequest increments the request counter.
func (ac *AlertChecker) RecordRequest() {
	ac.requestCount.Add(1)
}

// RecordError increments the error counter.
func (ac *AlertChecker) RecordError() {
	ac.errorCount.Add(1)
}

// RecordMessage records that a chat message was sent.
func (ac *AlertChecker) RecordMessage() {
	ac.lastMessage.Store(time.Now().Unix())
}

// RecordSignup records that a user signed up.
func (ac *AlertChecker) RecordSignup() {
	ac.lastSignup.Store(time.Now().Unix())
}

// Check evaluates all alert conditions and logs warnings for any that
// are breached. Should be called periodically (e.g., every 5 minutes
// via a background goroutine).
func (ac *AlertChecker) Check(ctx context.Context) {
	// Error rate check.
	requests := ac.requestCount.Load()
	errors := ac.errorCount.Load()
	if requests >= 100 {
		errorRate := float64(errors) / float64(requests)
		if errorRate > ac.ErrorRateThreshold {
			slog.Warn("ALERT: error rate exceeds threshold",
				"error_rate", errorRate,
				"threshold", ac.ErrorRateThreshold,
				"requests", requests,
				"errors", errors,
			)
		}
		// Reset counters for next window.
		ac.requestCount.Store(0)
		ac.errorCount.Store(0)
	}

	now := time.Now().Unix()

	// Message idle check.
	lastMsg := ac.lastMessage.Load()
	msgIdleHours := float64(now-lastMsg) / 3600
	if msgIdleHours > float64(ac.IdleMessageHours) {
		slog.Warn("ALERT: no chat messages sent recently",
			"idle_hours", msgIdleHours,
			"threshold_hours", ac.IdleMessageHours,
		)
	}

	// Signup idle check.
	lastSignup := ac.lastSignup.Load()
	signupIdleHours := float64(now-lastSignup) / 3600
	if signupIdleHours > float64(ac.IdleSignupHours) {
		slog.Warn("ALERT: no signups recently",
			"idle_hours", signupIdleHours,
			"threshold_hours", ac.IdleSignupHours,
		)
	}
}

// StartPeriodicCheck runs Check every interval in a background goroutine.
// Stops when the context is cancelled.
func (ac *AlertChecker) StartPeriodicCheck(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				ac.Check(ctx)
			}
		}
	}()
}
