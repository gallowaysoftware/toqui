package ai

import (
	"crypto/sha256"
	"fmt"
	"log/slog"
	"sync"
	"time"
	"unicode/utf8"
)

// ResponseCache provides an in-memory LRU cache for LLM responses.
// It is designed to cache short, repetitive queries in selection mode
// (e.g., "tell me about Paris") to avoid redundant LLM calls.
//
// Cache key = SHA-256(system_prompt + last_user_message).
// Only messages under maxMessageLen characters in selection mode are cached.
type ResponseCache struct {
	mu      sync.RWMutex
	store   map[string]*lruEntry
	order   []string // LRU order: most recently used at the end
	ttl     time.Duration
	maxSize int

	// maxMessageLen is the maximum user message length (in runes) eligible
	// for caching. Longer messages are too specific to benefit from caching.
	maxMessageLen int

	// now is a function returning the current time, injectable for testing.
	now func() time.Time
}

type lruEntry struct {
	response string
	cachedAt time.Time
}

// CacheOption configures a ResponseCache.
type CacheOption func(*ResponseCache)

// WithTTL sets the cache entry time-to-live. Default: 1 hour.
func WithTTL(d time.Duration) CacheOption {
	return func(c *ResponseCache) { c.ttl = d }
}

// WithMaxSize sets the maximum number of cached entries. Default: 1000.
func WithMaxSize(n int) CacheOption {
	return func(c *ResponseCache) { c.maxSize = n }
}

// WithMaxMessageLen sets the max user message length (runes) for caching. Default: 200.
func WithMaxMessageLen(n int) CacheOption {
	return func(c *ResponseCache) { c.maxMessageLen = n }
}

// withNow overrides the time function (for testing).
func withNow(fn func() time.Time) CacheOption {
	return func(c *ResponseCache) { c.now = fn }
}

// NewResponseCache creates a new response cache with the given options.
func NewResponseCache(opts ...CacheOption) *ResponseCache {
	c := &ResponseCache{
		store:         make(map[string]*lruEntry),
		order:         make([]string, 0, 1000),
		ttl:           time.Hour,
		maxSize:       1000,
		maxMessageLen: 200,
		now:           time.Now,
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// cacheKey computes the cache key for a request. It hashes the system prompt
// and the last user message content together.
func cacheKey(systemPrompt, userMessage string) string {
	h := sha256.New()
	h.Write([]byte(systemPrompt))
	h.Write([]byte{0}) // separator
	h.Write([]byte(userMessage))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// Eligible returns true if the given ChatRequest is eligible for caching.
// Only selection mode with short user messages (< maxMessageLen runes)
// and no tool calls are cached.
func (c *ResponseCache) Eligible(req *ChatRequest) bool {
	if req.Mode != "selection" {
		return false
	}
	if len(req.Tools) > 0 {
		return false
	}
	msg := lastUserMessage(req)
	if msg == "" {
		return false
	}
	return utf8.RuneCountInString(msg) < c.maxMessageLen
}

// Get looks up a cached response for the given request.
// Returns the cached response and true on a hit, or empty string and false on a miss.
func (c *ResponseCache) Get(req *ChatRequest) (string, bool) {
	msg := lastUserMessage(req)
	if msg == "" {
		return "", false
	}
	key := cacheKey(req.SystemPrompt, msg)

	c.mu.RLock()
	entry, ok := c.store[key]
	c.mu.RUnlock()

	if !ok {
		return "", false
	}

	// Check TTL expiration.
	if c.now().Sub(entry.cachedAt) > c.ttl {
		c.mu.Lock()
		// Re-check under write lock (another goroutine may have already evicted).
		if e, stillOk := c.store[key]; stillOk && c.now().Sub(e.cachedAt) > c.ttl {
			delete(c.store, key)
			c.removeFromOrder(key)
		}
		c.mu.Unlock()
		return "", false
	}

	// Promote to most recently used.
	c.mu.Lock()
	c.promoteKey(key)
	c.mu.Unlock()

	slog.Debug("llm cache hit", "key_prefix", key[:12])
	return entry.response, true
}

// Put stores a response in the cache for the given request.
// If the cache is at capacity, the least recently used entry is evicted.
func (c *ResponseCache) Put(req *ChatRequest, response string) {
	msg := lastUserMessage(req)
	if msg == "" {
		return
	}
	key := cacheKey(req.SystemPrompt, msg)

	c.mu.Lock()
	defer c.mu.Unlock()

	// If key already exists, update it and promote.
	if _, exists := c.store[key]; exists {
		c.store[key] = &lruEntry{response: response, cachedAt: c.now()}
		c.promoteKey(key)
		return
	}

	// Evict LRU entries if at capacity.
	for len(c.store) >= c.maxSize && len(c.order) > 0 {
		evictKey := c.order[0]
		c.order = c.order[1:]
		delete(c.store, evictKey)
		slog.Debug("llm cache eviction", "evicted_key_prefix", evictKey[:12])
	}

	c.store[key] = &lruEntry{response: response, cachedAt: c.now()}
	c.order = append(c.order, key)
}

// Len returns the number of entries currently in the cache.
func (c *ResponseCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.store)
}

// promoteKey moves a key to the end of the LRU order (most recently used).
// Must be called with c.mu held (write lock).
func (c *ResponseCache) promoteKey(key string) {
	c.removeFromOrder(key)
	c.order = append(c.order, key)
}

// removeFromOrder removes a key from the LRU order slice.
// Must be called with c.mu held (write lock).
func (c *ResponseCache) removeFromOrder(key string) {
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			return
		}
	}
}

// lastUserMessage returns the content of the last user message in a ChatRequest.
func lastUserMessage(req *ChatRequest) string {
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			return req.Messages[i].Content
		}
	}
	return ""
}
