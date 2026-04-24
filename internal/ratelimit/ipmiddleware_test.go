package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"strings"
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
			// Single entry: nothing to pick between — return it.
			name:     "X-Forwarded-For single IP",
			headers:  map[string]string{"X-Forwarded-For": "1.2.3.4"},
			remote:   "127.0.0.1:12345",
			expected: "1.2.3.4",
		},
		{
			// Cloud Run appends the real client IP to the end. A malicious
			// client sending "X-Forwarded-For: 1.2.3.4" produces a header
			// like "1.2.3.4, <real-ip>"; we must take the rightmost entry.
			name:     "X-Forwarded-For multiple IPs takes rightmost (anti-spoof)",
			headers:  map[string]string{"X-Forwarded-For": "1.2.3.4, 10.0.0.1, 10.0.0.2"},
			remote:   "127.0.0.1:12345",
			expected: "10.0.0.2",
		},
		{
			// Regression: explicit attacker scenario. Client forges
			// "X-Forwarded-For: 1.2.3.4" to pivot the rate-limit key; Cloud Run
			// appends the real IP, and we must pick the appended one.
			name:     "X-Forwarded-For attacker-supplied header does not spoof real IP",
			headers:  map[string]string{"X-Forwarded-For": "1.2.3.4, 203.0.113.7"},
			remote:   "127.0.0.1:12345",
			expected: "203.0.113.7",
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
			name:     "X-Forwarded-For with spaces trims both ends of rightmost",
			headers:  map[string]string{"X-Forwarded-For": " 1.2.3.4 , 10.0.0.1 "},
			remote:   "127.0.0.1:12345",
			expected: "10.0.0.1",
		},
		{
			// Trailing empty entry (e.g. "a, b, ") — skip empty, pick the
			// rightmost non-empty value.
			name:     "X-Forwarded-For trailing empty entry is skipped",
			headers:  map[string]string{"X-Forwarded-For": "1.2.3.4, 10.0.0.1, "},
			remote:   "127.0.0.1:12345",
			expected: "10.0.0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = tt.remote
			for k, v := range tt.headers {
				r.Header.Set(k, v)
			}

			got := ExtractClientIP(r)
			if got != tt.expected {
				t.Errorf("ExtractClientIP() = %q, want %q", got, tt.expected)
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

func TestHashToken_Deterministic(t *testing.T) {
	token := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.test-payload.signature"
	h1 := hashToken(token)
	h2 := hashToken(token)
	if h1 != h2 {
		t.Errorf("same token produced different hashes: %q vs %q", h1, h2)
	}
}

func TestHashToken_DifferentTokensDifferentKeys(t *testing.T) {
	h1 := hashToken("token-aaa-111")
	h2 := hashToken("token-bbb-222")
	if h1 == h2 {
		t.Errorf("different tokens produced the same hash: %q", h1)
	}
}

func TestHashToken_NoRawTokenInOutput(t *testing.T) {
	token := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"
	h := hashToken(token)
	if strings.Contains(h, token[:16]) {
		t.Errorf("hash output %q contains raw token prefix %q", h, token[:16])
	}
	if len(h) != 16 {
		t.Errorf("hash output length = %d, want 16", len(h))
	}
}

func TestExtractRateLimitKey_AuthenticatedUsesHash(t *testing.T) {
	token := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.test"
	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.Header.Set("Authorization", "Bearer "+token)

	key := extractRateLimitKey(r)
	if !strings.HasPrefix(key, "user:") {
		t.Fatalf("expected key to start with 'user:', got %q", key)
	}
	// Must not contain any raw token material
	if strings.Contains(key, token[:16]) {
		t.Errorf("rate limit key %q contains raw token prefix", key)
	}
	// The hash portion should be exactly 16 hex chars
	hashPart := strings.TrimPrefix(key, "user:")
	if len(hashPart) != 16 {
		t.Errorf("hash portion length = %d, want 16", len(hashPart))
	}
}

func TestExtractRateLimitKey_UnauthenticatedFallsBackToIP(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.RemoteAddr = "10.0.0.1:54321"

	key := extractRateLimitKey(r)
	if key != "10.0.0.1" {
		t.Errorf("expected IP key %q, got %q", "10.0.0.1", key)
	}
}
