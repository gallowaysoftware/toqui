package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAuthLimiter_NotBlockedInitially(t *testing.T) {
	al := NewAuthLimiter(5, time.Minute, 15*time.Minute)
	defer al.Stop()

	if al.IsBlocked("1.2.3.4") {
		t.Error("IP should not be blocked before any failures")
	}
}

func TestAuthLimiter_BlockAfterMaxAttempts(t *testing.T) {
	al := NewAuthLimiter(3, time.Minute, 15*time.Minute)
	defer al.Stop()

	ip := "10.0.0.1"
	for i := 0; i < 2; i++ {
		blocked := al.RecordFailure(ip)
		if blocked {
			t.Errorf("should not be blocked after %d failures", i+1)
		}
	}
	blocked := al.RecordFailure(ip)
	if !blocked {
		t.Error("should be blocked after 3 failures")
	}
	if !al.IsBlocked(ip) {
		t.Error("IsBlocked should return true")
	}
}

func TestAuthLimiter_DifferentIPsIndependent(t *testing.T) {
	al := NewAuthLimiter(2, time.Minute, 15*time.Minute)
	defer al.Stop()

	al.RecordFailure("10.0.0.1")
	al.RecordFailure("10.0.0.1")

	if !al.IsBlocked("10.0.0.1") {
		t.Error("10.0.0.1 should be blocked")
	}
	if al.IsBlocked("10.0.0.2") {
		t.Error("10.0.0.2 should not be blocked")
	}
}

func TestAuthLimiter_ClearOnSuccess(t *testing.T) {
	al := NewAuthLimiter(3, time.Minute, 15*time.Minute)
	defer al.Stop()

	ip := "10.0.0.1"
	al.RecordFailure(ip)
	al.RecordFailure(ip)
	// 2 failures — not yet blocked
	al.ClearFailures(ip)
	// Counter should be reset
	if al.RecordFailure(ip) {
		t.Error("should not be blocked after clearing failures")
	}
}

func TestAuthLimiter_WindowExpiry(t *testing.T) {
	// Use a very short window to test expiry.
	al := NewAuthLimiter(3, 50*time.Millisecond, 15*time.Minute)
	defer al.Stop()

	ip := "10.0.0.1"
	al.RecordFailure(ip)
	al.RecordFailure(ip)
	// 2 failures — not yet blocked

	time.Sleep(60 * time.Millisecond) // window expires

	// Next failure should start a new window, not trigger lockout.
	blocked := al.RecordFailure(ip)
	if blocked {
		t.Error("window expired, should not be blocked")
	}
}

func TestAuthLimiter_Middleware(t *testing.T) {
	al := NewAuthLimiter(2, time.Minute, 15*time.Minute)
	defer al.Stop()

	handler := al.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	ip := "10.0.0.1"
	al.RecordFailure(ip)
	al.RecordFailure(ip)

	req := httptest.NewRequest(http.MethodPost, "/auth/exchange", nil)
	req.Header.Set("X-Forwarded-For", ip)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", rec.Code)
	}
}

func TestAuthLimiter_MiddlewareAllowsUnblocked(t *testing.T) {
	al := NewAuthLimiter(5, time.Minute, 15*time.Minute)
	defer al.Stop()

	handler := al.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/auth/exchange", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.2")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}
