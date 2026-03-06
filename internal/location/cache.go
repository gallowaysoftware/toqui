package location

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// DefaultCacheTTL is the duration after which a cached location entry is
// considered stale and will no longer be returned by Get.
const DefaultCacheTTL = 30 * time.Minute

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
}

// NewCache creates a new location cache with the given TTL. Entries older
// than ttl are considered stale and will not be returned by Get.
func NewCache(ttl time.Duration) *Cache {
	return &Cache{
		entries: make(map[uuid.UUID]*CacheEntry),
		ttl:     ttl,
	}
}

// Set stores or updates the location for the given user.
func (c *Cache) Set(userID uuid.UUID, lat, lng, accuracy float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
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
