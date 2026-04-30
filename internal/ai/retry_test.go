package ai

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// retry.go is the resilience layer between the chat handler and the AI
// providers. A bug here either silently drops legitimate errors (data
// loss) or hammers the upstream during incidents (rate-limit pile-on).
// Each test pins one specific contract.

func TestDefaultRetryConfig_HasReasonableValues(t *testing.T) {
	cfg := defaultRetryConfig()
	if cfg.maxRetries != 3 {
		t.Errorf("maxRetries = %d, want 3", cfg.maxRetries)
	}
	if cfg.initialDelay != 1*time.Second {
		t.Errorf("initialDelay = %v, want 1s", cfg.initialDelay)
	}
	if cfg.maxDelay != 10*time.Second {
		t.Errorf("maxDelay = %v, want 10s", cfg.maxDelay)
	}
}

func TestIsTransientStatusCode(t *testing.T) {
	// Pin the exact set of transient codes. Adding 408 (Request
	// Timeout) here would actually be reasonable but is currently
	// excluded — pin the existing decision so any change is
	// deliberate.
	cases := map[int]bool{
		// Transient (should retry):
		http.StatusTooManyRequests:     true, // 429
		http.StatusInternalServerError: true, // 500
		http.StatusBadGateway:          true, // 502
		http.StatusServiceUnavailable:  true, // 503
		http.StatusGatewayTimeout:      true, // 504

		// NOT transient (don't retry — caller / auth issues):
		http.StatusOK:                  false, // 200
		http.StatusBadRequest:          false, // 400
		http.StatusUnauthorized:        false, // 401
		http.StatusForbidden:           false, // 403
		http.StatusNotFound:            false, // 404
		http.StatusRequestTimeout:      false, // 408 — not currently transient
		http.StatusConflict:            false, // 409
		http.StatusUnprocessableEntity: false, // 422
	}
	for status, want := range cases {
		if got := isTransientStatusCode(status); got != want {
			t.Errorf("isTransientStatusCode(%d) = %v, want %v", status, got, want)
		}
	}
}

func TestRetryableError_Format(t *testing.T) {
	// Format string is read by the chat handler's error-classification
	// logic and surfaces in slog audit logs. Pin the shape so log
	// queries that match on "API error <code>:" continue to work.
	e := &retryableError{statusCode: 503, body: "service unavailable"}
	got := e.Error()
	want := "API error 503: service unavailable"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestRetryDelay_FirstAttempt_BoundedByInitial(t *testing.T) {
	// Attempt 0: backoff = initialDelay * 2^0 = initialDelay.
	// Full jitter means actual delay ∈ [0, initialDelay).
	cfg := defaultRetryConfig()
	for i := 0; i < 50; i++ {
		got := retryDelay(cfg, 0)
		if got < 0 || got >= cfg.initialDelay {
			t.Errorf("attempt 0 delay = %v, want [0, %v)", got, cfg.initialDelay)
		}
	}
}

func TestRetryDelay_BackoffExponentialUntilMaxDelay(t *testing.T) {
	// Attempt N: backoff = initialDelay * 2^N, capped at maxDelay.
	// Full jitter means actual ∈ [0, capped). We sample the upper
	// bound by repeated runs — pin that the cap actually clamps.
	cfg := retryConfig{
		maxRetries:   10,
		initialDelay: 100 * time.Millisecond,
		maxDelay:     1 * time.Second,
	}
	// Attempt 10: 100ms * 2^10 = 102.4s, capped at 1s.
	// All samples should be ≤ maxDelay.
	for i := 0; i < 100; i++ {
		got := retryDelay(cfg, 10)
		if got > cfg.maxDelay {
			t.Errorf("attempt 10 delay = %v, want ≤ %v (clamp violated)", got, cfg.maxDelay)
		}
	}
}

func TestSleepWithContext_RespectsContextCancel(t *testing.T) {
	// A cancelled context during sleep MUST return promptly with
	// ctx.Err(). Pre-fix bugs in this pattern (using a bare time.Sleep)
	// would block the chat-handler goroutine for the full delay even
	// after the user disconnected.
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before the call

	start := time.Now()
	err := sleepWithContext(ctx, 5*time.Second)
	elapsed := time.Since(start)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("sleepWithContext on cancelled ctx = %v, want context.Canceled", err)
	}
	if elapsed > 100*time.Millisecond {
		t.Errorf("sleepWithContext on cancelled ctx slept for %v, should be near-instant", elapsed)
	}
}

func TestSleepWithContext_SleepsFullDurationWhenNotCancelled(t *testing.T) {
	start := time.Now()
	err := sleepWithContext(context.Background(), 50*time.Millisecond)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("uncancelled sleep errored: %v", err)
	}
	if elapsed < 40*time.Millisecond {
		t.Errorf("sleep duration = %v, expected ≥ 40ms", elapsed)
	}
}

func TestReadLimitedBody_TruncatesAt4KB(t *testing.T) {
	// 4 KB body limit prevents a malicious or buggy upstream from
	// piping a huge error body into our slog/audit pipeline. Pin the
	// limit so a future change to LimitReader's bound surfaces here.
	const bigSize = 10 * 1024 // 10 KB — well over the 4 KB limit
	body := strings.Repeat("X", bigSize)
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(body)),
	}
	got := readLimitedBody(resp)
	if len(got) != 4096 {
		t.Errorf("body len = %d, want 4096 (LimitReader cap)", len(got))
	}
}

func TestReadLimitedBody_ReturnsEmptyForNilBody(t *testing.T) {
	// Defensive: a Response with nil Body shouldn't panic.
	resp := &http.Response{Body: nil}
	got := readLimitedBody(resp)
	if got != "" {
		t.Errorf("readLimitedBody(nil) = %q, want empty", got)
	}
}

func TestDoWithRetry_SuccessOnFirstAttempt(t *testing.T) {
	// Happy path: fn returns 200 immediately. Verify only one call.
	var calls atomic.Int32
	fn := func() (*http.Response, error) {
		calls.Add(1)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("ok")),
		}, nil
	}
	cfg := retryConfig{maxRetries: 3, initialDelay: time.Millisecond, maxDelay: time.Millisecond}
	resp, err := doWithRetry(context.Background(), cfg, "test", fn)
	if err != nil {
		t.Fatalf("doWithRetry errored on success: %v", err)
	}
	if resp == nil {
		t.Fatal("resp is nil despite no error")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("response = %v, want 200", resp)
	}
	if calls.Load() != 1 {
		t.Errorf("called fn %d times, want 1 (no retry on success)", calls.Load())
	}
}

func TestDoWithRetry_RetriesTransient503AndSucceeds(t *testing.T) {
	// Simulate Anthropic's "we're overloaded" — first call 503, second
	// call 200. The retry MUST eventually succeed (not give up after
	// the first transient).
	var calls atomic.Int32
	fn := func() (*http.Response, error) {
		n := calls.Add(1)
		if n == 1 {
			return &http.Response{
				StatusCode: http.StatusServiceUnavailable,
				Body:       io.NopCloser(strings.NewReader("overloaded")),
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("ok")),
		}, nil
	}
	cfg := retryConfig{maxRetries: 3, initialDelay: time.Millisecond, maxDelay: 10 * time.Millisecond}
	resp, err := doWithRetry(context.Background(), cfg, "test", fn)
	if err != nil {
		t.Fatalf("doWithRetry errored after retry: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("final status = %d, want 200", resp.StatusCode)
	}
	if calls.Load() != 2 {
		t.Errorf("called fn %d times, want 2 (one retry)", calls.Load())
	}
}

func TestDoWithRetry_PermanentErrorIsNotRetried(t *testing.T) {
	// 401 (auth failure) is permanent — retrying just delays the
	// inevitable error. Verify exactly one call.
	var calls atomic.Int32
	fn := func() (*http.Response, error) {
		calls.Add(1)
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Body:       io.NopCloser(strings.NewReader("invalid api key")),
		}, nil
	}
	cfg := retryConfig{maxRetries: 3, initialDelay: time.Millisecond, maxDelay: time.Millisecond}
	// doWithRetry closes the body internally on non-OK; resp is nil here.
	_, err := doWithRetry(context.Background(), cfg, "test", fn) //nolint:bodyclose // doWithRetry consumes the body internally on error paths
	if err == nil {
		t.Fatal("doWithRetry should error on 401")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error = %v, want it to mention 401", err)
	}
	if calls.Load() != 1 {
		t.Errorf("called fn %d times, want 1 (permanent error: NO retry)", calls.Load())
	}
}

func TestDoWithRetry_ExhaustsRetriesOnPersistentTransient(t *testing.T) {
	// All attempts return 503. Verify we retry up to maxRetries+1 times
	// total, then surface the final error.
	var calls atomic.Int32
	fn := func() (*http.Response, error) {
		calls.Add(1)
		return &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Body:       io.NopCloser(strings.NewReader("still down")),
		}, nil
	}
	cfg := retryConfig{maxRetries: 2, initialDelay: time.Millisecond, maxDelay: time.Millisecond}
	_, err := doWithRetry(context.Background(), cfg, "test", fn) //nolint:bodyclose // doWithRetry consumes the body internally
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	// maxRetries=2 + 1 initial attempt = 3 calls total.
	if calls.Load() != 3 {
		t.Errorf("called fn %d times, want 3 (initial + 2 retries)", calls.Load())
	}
}

func TestDoWithRetry_NetworkErrorIsRetried(t *testing.T) {
	// fn returns a Go error (DNS failure, connection refused). Retry
	// the same way as transient HTTP errors.
	var calls atomic.Int32
	netErr := errors.New("dial tcp: connection refused")
	fn := func() (*http.Response, error) {
		n := calls.Add(1)
		if n < 2 {
			return nil, netErr
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok"))}, nil
	}
	cfg := retryConfig{maxRetries: 3, initialDelay: time.Millisecond, maxDelay: time.Millisecond}
	resp, err := doWithRetry(context.Background(), cfg, "test", fn)
	if err != nil {
		t.Fatalf("doWithRetry should succeed after retry, got: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestDoWithRetry_ContextCancelDuringBackoffShortCircuits(t *testing.T) {
	// User disconnect during backoff sleep MUST short-circuit
	// immediately rather than completing the (now-pointless) retry.
	var calls atomic.Int32
	fn := func() (*http.Response, error) {
		calls.Add(1)
		return &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Body:       io.NopCloser(strings.NewReader("transient")),
		}, nil
	}
	// Long delay so the cancel races the sleep deterministically.
	cfg := retryConfig{maxRetries: 3, initialDelay: 5 * time.Second, maxDelay: 5 * time.Second}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err := doWithRetry(ctx, cfg, "test", fn) //nolint:bodyclose // doWithRetry consumes the body before sleep; we error out on cancel
	elapsed := time.Since(start)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	if elapsed > 1*time.Second {
		t.Errorf("doWithRetry blocked for %v after cancel — should short-circuit", elapsed)
	}
}
