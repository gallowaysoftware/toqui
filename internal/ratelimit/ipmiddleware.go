package ratelimit

import (
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type ipEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// IPRateLimiter provides per-IP rate limiting as HTTP middleware.
// It uses a token bucket algorithm to limit the number of requests per IP.
type IPRateLimiter struct {
	mu          sync.Mutex
	ips         map[string]*ipEntry
	ratePerSec  rate.Limit
	burst       int
	cleanupStop chan struct{}
}

// NewIPRateLimiter creates a per-IP rate limiter.
// requestsPerMinute is the sustained rate; burst allows short spikes.
func NewIPRateLimiter(requestsPerMinute, burst int) *IPRateLimiter {
	l := &IPRateLimiter{
		ips:         make(map[string]*ipEntry),
		ratePerSec:  rate.Limit(float64(requestsPerMinute) / 60.0),
		burst:       burst,
		cleanupStop: make(chan struct{}),
	}
	go l.cleanupLoop()
	return l
}

// Middleware returns an http.Handler that enforces per-IP rate limits.
func (l *IPRateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := extractClientIP(r)

		entry := l.getOrCreate(ip)
		if !entry.limiter.Allow() {
			slog.Warn("ip rate limit exceeded", "ip", ip, "path", r.URL.Path)
			w.Header().Set("Retry-After", "60")
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (l *IPRateLimiter) getOrCreate(ip string) *ipEntry {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry, ok := l.ips[ip]
	if !ok {
		entry = &ipEntry{
			limiter: rate.NewLimiter(l.ratePerSec, l.burst),
		}
		l.ips[ip] = entry
	}
	entry.lastSeen = time.Now()
	return entry
}

// extractClientIP returns the client's real IP address.
// Cloud Run and load balancers set X-Forwarded-For; we take the first
// (leftmost) entry, which is the original client IP.
func extractClientIP(r *http.Request) string {
	// X-Forwarded-For: client, proxy1, proxy2
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		ip := strings.TrimSpace(parts[0])
		if ip != "" {
			return ip
		}
	}

	// X-Real-IP (some proxies set this instead)
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr (may include port)
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// cleanupLoop removes stale IP entries every 2 minutes.
func (l *IPRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			l.evictStale()
		case <-l.cleanupStop:
			return
		}
	}
}

func (l *IPRateLimiter) evictStale() {
	l.mu.Lock()
	defer l.mu.Unlock()

	cutoff := time.Now().Add(-10 * time.Minute)
	for ip, entry := range l.ips {
		if entry.lastSeen.Before(cutoff) {
			delete(l.ips, ip)
		}
	}
}

// Stop terminates the background cleanup goroutine.
func (l *IPRateLimiter) Stop() {
	close(l.cleanupStop)
}
