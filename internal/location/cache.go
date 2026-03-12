package location

import (
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

// DefaultCacheTTL is the duration after which a cached location entry is
// considered stale and will no longer be returned by Get.
const DefaultCacheTTL = 30 * time.Minute

// DefaultMaxCacheSize is the maximum number of entries the location cache
// will hold. When exceeded, stale entries are evicted first, then the
// oldest half of remaining entries are removed.
const DefaultMaxCacheSize = 10_000

// CacheEntry holds a single user's most recent location update.
type CacheEntry struct {
	Lat       float64
	Lng       float64
	Accuracy  float64
	UpdatedAt time.Time
}

// Cache is an in-memory, concurrency-safe cache of user locations.
// It is designed to be a package-level singleton shared between the
// location handler (writes) and the chat handler (reads).
type Cache struct {
	mu      sync.RWMutex
	entries map[uuid.UUID]*CacheEntry
	ttl     time.Duration
	maxSize int
}

// NewCache creates a new location cache with the given TTL. Entries older
// than ttl are considered stale and will not be returned by Get.
func NewCache(ttl time.Duration) *Cache {
	return &Cache{
		entries: make(map[uuid.UUID]*CacheEntry),
		ttl:     ttl,
		maxSize: DefaultMaxCacheSize,
	}
}

// Set stores or updates the location for the given user.
// If the cache is at capacity, stale entries are evicted first; if still
// full, the oldest half of remaining entries are removed.
func (c *Cache) Set(userID uuid.UUID, lat, lng, accuracy float64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Only evict if this is a new key and we're at capacity.
	if _, exists := c.entries[userID]; !exists && len(c.entries) >= c.maxSize {
		c.evictLocked()
	}

	c.entries[userID] = &CacheEntry{
		Lat:       lat,
		Lng:       lng,
		Accuracy:  accuracy,
		UpdatedAt: time.Now(),
	}
}

// SetWithTime stores a location entry with an explicit timestamp. This is
// primarily useful for testing.
func (c *Cache) SetWithTime(userID uuid.UUID, lat, lng, accuracy float64, updatedAt time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.entries[userID]; !exists && len(c.entries) >= c.maxSize {
		c.evictLocked()
	}

	c.entries[userID] = &CacheEntry{
		Lat:       lat,
		Lng:       lng,
		Accuracy:  accuracy,
		UpdatedAt: updatedAt,
	}
}

// Get returns the cached location for the given user if it exists and is
// fresher than the configured TTL. Returns nil if absent or stale.
func (c *Cache) Get(userID uuid.UUID) *CacheEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[userID]
	if !ok {
		return nil
	}
	if time.Since(entry.UpdatedAt) >= c.ttl {
		return nil
	}
	return entry
}

// Delete removes the cached location for the given user.
func (c *Cache) Delete(userID uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, userID)
}

// Len returns the number of entries currently in the cache (including stale).
// This is primarily useful for testing and monitoring.
func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// evictLocked removes stale entries first. If the cache is still at capacity,
// it removes the oldest half of remaining entries. Must be called with c.mu held.
func (c *Cache) evictLocked() {
	now := time.Now()

	// Pass 1: remove stale entries (older than TTL).
	for uid, entry := range c.entries {
		if now.Sub(entry.UpdatedAt) >= c.ttl {
			delete(c.entries, uid)
		}
	}
	if len(c.entries) < c.maxSize {
		return
	}

	// Pass 2: still at capacity — remove the oldest half.
	// Find the median UpdatedAt by collecting all timestamps.
	type uidTime struct {
		uid uuid.UUID
		t   time.Time
	}
	all := make([]uidTime, 0, len(c.entries))
	for uid, entry := range c.entries {
		all = append(all, uidTime{uid, entry.UpdatedAt})
	}

	// Sort by UpdatedAt ascending (oldest first) and remove the first half.
	// Use a simple selection: find the midpoint count and delete the oldest.
	halfCount := len(all) / 2
	if halfCount == 0 {
		halfCount = 1
	}

	// Find the halfCount oldest entries by iterating and tracking the oldest.
	// For simplicity (and since this is a rare operation), delete entries
	// with the oldest timestamps.
	for range halfCount {
		oldestIdx := 0
		for i := 1; i < len(all); i++ {
			if all[i].t.Before(all[oldestIdx].t) {
				oldestIdx = i
			}
		}
		delete(c.entries, all[oldestIdx].uid)
		// Remove from slice by swapping with last element
		all[oldestIdx] = all[len(all)-1]
		all = all[:len(all)-1]
	}

	slog.Info("location cache eviction completed",
		"evicted", halfCount,
		"remaining", len(c.entries),
	)
}
