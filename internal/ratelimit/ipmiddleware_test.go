package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestExtractClientIP(t *testing.T) {
	tests := []struct {
		name     string
		headers  map[string]string
		remote   string
		expected string
	}{
		{
			name:     "X-Forwarded-For single IP",
			headers:  map[string]string{"X-Forwarded-For": "1.2.3.4"},
			remote:   "127.0.0.1:12345",
			expected: "1.2.3.4",
		},
		{
			name:     "X-Forwarded-For multiple IPs takes first",
			headers:  map[string]string{"X-Forwarded-For": "1.2.3.4, 10.0.0.1, 10.0.0.2"},
			remote:   "127.0.0.1:12345",
			expected: "1.2.3.4",
		},
		{
			name:     "X-Real-IP fallback",
			headers:  map[string]string{"X-Real-IP": "5.6.7.8"},
			remote:   "127.0.0.1:12345",
			expected: "5.6.7.8",
		},
		{
			name:     "X-Forwarded-For takes precedence over X-Real-IP",
			headers:  map[string]string{"X-Forwarded-For": "1.2.3.4", "X-Real-IP": "5.6.7.8"},
			remote:   "127.0.0.1:12345",
			expected: "1.2.3.4",
		},
		{
			name:     "RemoteAddr fallback with port",
			headers:  map[string]string{},
			remote:   "9.8.7.6:54321",
			expected: "9.8.7.6",
		},
		{
			name:     "RemoteAddr fallback without port",
			headers:  map[string]string{},
			remote:   "9.8.7.6",
			expected: "9.8.7.6",
		},
		{
			name:     "X-Forwarded-For with spaces",
			headers:  map[string]string{"X-Forwarded-For": " 1.2.3.4 , 10.0.0.1"},
			remote:   "127.0.0.1:12345",
			expected: "1.2.3.4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = tt.remote
			for k, v := range tt.headers {
				r.Header.Set(k, v)
			}

			got := extractClientIP(r)
			if got != tt.expected {
				t.Errorf("extractClientIP() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestIPRateLimiter_AllowsNormalTraffic(t *testing.T) {
	limiter := NewIPRateLimiter(60, 10) // 60/min, burst of 10
	defer limiter.Stop()

	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First 10 requests should succeed (burst)
	for i := range 10 {
		r := httptest.NewRequest(http.MethodGet, "/test", nil)
		r.RemoteAddr = "1.2.3.4:12345"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)

		if w.Code != http.StatusOK {
			t.Fatalf("request %d: got status %d, want 200", i+1, w.Code)
		}
	}
}

func TestIPRateLimiter_BlocksExcessiveTraffic(t *testing.T) {
	limiter := NewIPRateLimiter(60, 5) // 60/min, burst of 5
	defer limiter.Stop()

	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Exhaust burst
	for range 5 {
		r := httptest.NewRequest(http.MethodGet, "/test", nil)
		r.RemoteAddr = "1.2.3.4:12345"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
	}

	// Next request should be rate limited
	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.RemoteAddr = "1.2.3.4:12345"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("got status %d, want 429", w.Code)
	}

	if w.Header().Get("Retry-After") == "" {
		t.Error("missing Retry-After header")
	}
}

func TestIPRateLimiter_DifferentIPsIndependent(t *testing.T) {
	limiter := NewIPRateLimiter(60, 3) // small burst to test quickly
	defer limiter.Stop()

	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Exhaust burst for IP 1
	for range 3 {
		r := httptest.NewRequest(http.MethodGet, "/test", nil)
		r.RemoteAddr = "1.1.1.1:12345"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
	}

	// IP 1 should be blocked
	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.RemoteAddr = "1.1.1.1:12345"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("IP 1 got status %d, want 429", w.Code)
	}

	// IP 2 should still work
	r = httptest.NewRequest(http.MethodGet, "/test", nil)
	r.RemoteAddr = "2.2.2.2:12345"
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("IP 2 got status %d, want 200", w.Code)
	}
}

func TestIPRateLimiter_EvictsStaleEntries(t *testing.T) {
	limiter := NewIPRateLimiter(60, 10)
	defer limiter.Stop()

	// Create an entry
	limiter.getOrCreate("10.0.0.1")

	limiter.mu.Lock()
	if len(limiter.ips) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(limiter.ips))
	}

	// Backdate the entry so it looks stale
	limiter.ips["10.0.0.1"].lastSeen = limiter.ips["10.0.0.1"].lastSeen.Add(-15 * time.Minute)
	limiter.mu.Unlock()

	limiter.evictStale()

	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	if len(limiter.ips) != 0 {
		t.Fatalf("expected 0 entries after eviction, got %d", len(limiter.ips))
	}
}
