package ratelimit

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

// RESTLimiter provides per-key rate limiting for REST endpoints.
// Unlike the ConnectRPC interceptor, this works with composite keys
// (e.g., "userID:tripID") so you can rate-limit per user per resource.
type RESTLimiter struct {
	mu      sync.Mutex
	records map[string]*restRecord
	max     int
	window  time.Duration
	stopCh  chan struct{}
}

type restRecord struct {
	count       int
	windowStart time.Time
}

// NewRESTLimiter creates a rate limiter that allows max requests per window
// per key. For example, NewRESTLimiter(5, 10*time.Minute) allows 5 requests
// per 10-minute window.
func NewRESTLimiter(max int, window time.Duration) *RESTLimiter {
	rl := &RESTLimiter{
		records: make(map[string]*restRecord),
		max:     max,
		window:  window,
		stopCh:  make(chan struct{}),
	}
	go rl.cleanupLoop()
	return rl
}

// Allow checks if a request is allowed for the given key.
// Returns true if the request is within the rate limit.
func (rl *RESTLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	rec, ok := rl.records[key]
	if !ok {
		rl.records[key] = &restRecord{
			count:       1,
			windowStart: now,
		}
		return true
	}

	// If the window has expired, reset.
	if now.After(rec.windowStart.Add(rl.window)) {
		rec.count = 1
		rec.windowStart = now
		return true
	}

	if rec.count >= rl.max {
		return false
	}

	rec.count++
	return true
}

// Reject writes a 429 Too Many Requests response with a descriptive message.
func Reject(w http.ResponseWriter, msg string) {
	w.Header().Set("Retry-After", "60")
	http.Error(w, fmt.Sprintf("rate limit exceeded: %s", msg), http.StatusTooManyRequests)
}

// cleanupLoop removes stale records periodically.
func (rl *RESTLimiter) cleanupLoop() {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			rl.evictStale()
		case <-rl.stopCh:
			return
		}
	}
}

func (rl *RESTLimiter) evictStale() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for key, rec := range rl.records {
		if now.After(rec.windowStart.Add(rl.window * 2)) {
			delete(rl.records, key)
		}
	}
}

// Stop terminates the background cleanup goroutine.
func (rl *RESTLimiter) Stop() {
	close(rl.stopCh)
}
