package ai

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math"
	"math/rand/v2"
	"net/http"
	"time"
)

// retryConfig holds parameters for the exponential backoff retry logic.
type retryConfig struct {
	maxRetries   int
	initialDelay time.Duration
	maxDelay     time.Duration
}

// defaultRetryConfig returns the default retry configuration:
// 3 retries, 1s initial delay, 10s max delay.
func defaultRetryConfig() retryConfig {
	return retryConfig{
		maxRetries:   3,
		initialDelay: 1 * time.Second,
		maxDelay:     10 * time.Second,
	}
}

// isTransientStatusCode returns true if the HTTP status code indicates a
// transient error that should be retried.
func isTransientStatusCode(statusCode int) bool {
	switch statusCode {
	case http.StatusTooManyRequests, // 429
		http.StatusInternalServerError, // 500
		http.StatusBadGateway,          // 502
		http.StatusServiceUnavailable,  // 503
		http.StatusGatewayTimeout:      // 504
		return true
	default:
		return false
	}
}

// retryableError wraps an HTTP error with its status code so the retry logic
// can distinguish transient from permanent errors.
type retryableError struct {
	statusCode int
	body       string
}

func (e *retryableError) Error() string {
	return fmt.Sprintf("API error %d: %s", e.statusCode, e.body)
}

// retryDelay computes the delay for a given attempt using exponential backoff
// with full jitter. attempt is 0-indexed.
func retryDelay(cfg retryConfig, attempt int) time.Duration {
	backoff := float64(cfg.initialDelay) * math.Pow(2, float64(attempt))
	if backoff > float64(cfg.maxDelay) {
		backoff = float64(cfg.maxDelay)
	}
	// Full jitter: random duration in [0, backoff)
	jittered := time.Duration(rand.Float64() * backoff) //nolint:gosec // jitter does not need crypto/rand
	return jittered
}

// sleepWithContext sleeps for the given duration or until the context is cancelled.
// Returns ctx.Err() if the context was cancelled during sleep.
func sleepWithContext(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// doWithRetry executes fn and retries on transient errors. fn must return
// (*http.Response, error). On a non-transient error or after exhausting retries,
// the last error is returned. On success, the response is returned with the body
// still open for the caller to consume.
func doWithRetry(ctx context.Context, cfg retryConfig, providerName string, fn func() (*http.Response, error)) (*http.Response, error) {
	var lastErr error

	for attempt := range cfg.maxRetries + 1 {
		resp, err := fn()
		if err != nil {
			// Network-level error (DNS, connection refused, etc.) — retry.
			lastErr = err
			if attempt < cfg.maxRetries {
				delay := retryDelay(cfg, attempt)
				slog.Warn("ai request failed, retrying",
					"provider", providerName,
					"attempt", attempt+1,
					"max_retries", cfg.maxRetries,
					"delay", delay,
					"error", err,
				)
				if sleepErr := sleepWithContext(ctx, delay); sleepErr != nil {
					return nil, sleepErr
				}
				continue
			}
			return nil, fmt.Errorf("after %d retries: %w", cfg.maxRetries, lastErr)
		}

		if resp.StatusCode == http.StatusOK {
			return resp, nil
		}

		// Non-OK status — check if transient.
		body := readLimitedBody(resp)
		resp.Body.Close()

		if isTransientStatusCode(resp.StatusCode) && attempt < cfg.maxRetries {
			delay := retryDelay(cfg, attempt)
			slog.Warn("ai request returned transient error, retrying",
				"provider", providerName,
				"status", resp.StatusCode,
				"attempt", attempt+1,
				"max_retries", cfg.maxRetries,
				"delay", delay,
			)
			lastErr = &retryableError{statusCode: resp.StatusCode, body: body}
			if sleepErr := sleepWithContext(ctx, delay); sleepErr != nil {
				return nil, sleepErr
			}
			continue
		}

		// Permanent error or retries exhausted.
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, body)
	}

	return nil, fmt.Errorf("after %d retries: %w", cfg.maxRetries, lastErr)
}

// readLimitedBody reads up to 4KB from the response body for error messages.
// Uses io.ReadAll with io.LimitReader to fully drain the body, which ensures
// HTTP/1.1 connection reuse (keep-alive) when resp.Body.Close() is called.
func readLimitedBody(resp *http.Response) string {
	if resp.Body == nil {
		return ""
	}
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return string(data)
}
