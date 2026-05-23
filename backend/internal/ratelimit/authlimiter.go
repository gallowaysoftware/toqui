package ratelimit

import (
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// AuthLimiter tracks failed authentication attempts per IP and temporarily
// blocks IPs that exceed the threshold. This provides lockout-like behavior
// for OAuth-based apps where traditional account lockout doesn't apply.
type AuthLimiter struct {
	mu          sync.Mutex
	attempts    map[string]*authRecord
	maxAttempts int
	window      time.Duration
	blockFor    time.Duration
	stopCh      chan struct{}
}

type authRecord struct {
	failures  int
	firstFail time.Time
	blockedAt time.Time
}

// NewAuthLimiter creates a limiter that blocks an IP after maxAttempts failures
// within the given window, for blockFor duration.
func NewAuthLimiter(maxAttempts int, window, blockFor time.Duration) *AuthLimiter {
	al := &AuthLimiter{
		attempts:    make(map[string]*authRecord),
		maxAttempts: maxAttempts,
		window:      window,
		blockFor:    blockFor,
		stopCh:      make(chan struct{}),
	}
	go al.cleanup()
	return al
}

// Stop stops the background cleanup goroutine.
func (al *AuthLimiter) Stop() {
	close(al.stopCh)
}

// RecordFailure records a failed auth attempt for the given IP.
// Returns true if the IP is now blocked.
func (al *AuthLimiter) RecordFailure(ip string) bool {
	al.mu.Lock()
	defer al.mu.Unlock()

	now := time.Now()
	rec, ok := al.attempts[ip]
	if !ok {
		al.attempts[ip] = &authRecord{
			failures:  1,
			firstFail: now,
		}
		return false
	}

	// If currently blocked, don't reset the counter.
	if !rec.blockedAt.IsZero() && now.Before(rec.blockedAt.Add(al.blockFor)) {
		return true
	}

	// If the window has expired, reset the counter.
	if now.After(rec.firstFail.Add(al.window)) {
		rec.failures = 1
		rec.firstFail = now
		rec.blockedAt = time.Time{}
		return false
	}

	rec.failures++
	if rec.failures >= al.maxAttempts {
		rec.blockedAt = now
		return true
	}
	return false
}

// IsBlocked checks if an IP is currently blocked.
func (al *AuthLimiter) IsBlocked(ip string) bool {
	al.mu.Lock()
	defer al.mu.Unlock()

	rec, ok := al.attempts[ip]
	if !ok {
		return false
	}
	if rec.blockedAt.IsZero() {
		return false
	}
	return time.Now().Before(rec.blockedAt.Add(al.blockFor))
}

// ClearFailures resets the failure counter for an IP (call on successful auth).
func (al *AuthLimiter) ClearFailures(ip string) {
	al.mu.Lock()
	defer al.mu.Unlock()
	delete(al.attempts, ip)
}

// Middleware wraps an http.Handler to reject requests from blocked IPs.
// Use this on auth-related routes.
func (al *AuthLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := ExtractClientIP(r)
		if al.IsBlocked(ip) {
			slog.Warn("auth lockout: request blocked",
				"ip", ip,
				"method", r.Method,
				"path", r.URL.Path,
			)
			http.Error(w, "Too many failed attempts. Please try again later.", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// cleanup periodically removes stale records.
func (al *AuthLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-al.stopCh:
			return
		case <-ticker.C:
			al.mu.Lock()
			now := time.Now()
			for ip, rec := range al.attempts {
				// Remove if: not blocked and window expired, or block has expired.
				windowExpired := now.After(rec.firstFail.Add(al.window))
				blockExpired := !rec.blockedAt.IsZero() && now.After(rec.blockedAt.Add(al.blockFor))
				if (rec.blockedAt.IsZero() && windowExpired) || blockExpired {
					delete(al.attempts, ip)
				}
			}
			al.mu.Unlock()
		}
	}
}
