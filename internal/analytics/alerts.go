package analytics

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"
)

// AlertChecker logs warnings when the rolling-window error rate exceeds a
// threshold. Picked up by Cloud Logging alerting policies in production.
//
// Usage-idle alerts (no chat messages / no signups recently) lived here too
// but were noise at our scale — at small N they fire every 5 minutes from
// process boot and drown out signal in incident triage. Bring them back when
// idle gaps actually mean something we'd page on.
type AlertChecker struct {
	requestCount atomic.Int64
	errorCount   atomic.Int64

	// ErrorRateThreshold is the fraction of failed requests above which the
	// alert fires. Default: 0.05 (5%). Only evaluated once a minimum of 100
	// requests have been observed in the current window.
	ErrorRateThreshold float64
}

// NewAlertChecker creates a new alert checker with default thresholds.
func NewAlertChecker() *AlertChecker {
	return &AlertChecker{ErrorRateThreshold: 0.05}
}

// RecordRequest increments the request counter.
func (ac *AlertChecker) RecordRequest() {
	ac.requestCount.Add(1)
}

// RecordError increments the error counter.
func (ac *AlertChecker) RecordError() {
	ac.errorCount.Add(1)
}

// RecordMessage is retained as a no-op so existing chat handler call sites
// keep compiling. The "idle messages" alert it used to feed was removed (see
// the type comment); call sites can be deleted in a follow-up sweep.
func (ac *AlertChecker) RecordMessage() {}

// RecordSignup is retained as a no-op for the same reason as RecordMessage.
func (ac *AlertChecker) RecordSignup() {}

// Check evaluates all alert conditions and logs warnings for any that
// are breached. Should be called periodically (e.g., every 5 minutes
// via a background goroutine).
func (ac *AlertChecker) Check(_ context.Context) {
	requests := ac.requestCount.Load()
	errors := ac.errorCount.Load()
	if requests < 100 {
		return
	}

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
