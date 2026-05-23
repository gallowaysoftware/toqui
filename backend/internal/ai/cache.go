package ai

import (
	"container/list"
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
// Cache key = SHA-256(user_id + system_prompt + last_user_message).
// Only messages under maxMessageLen characters in selection mode are cached.
//
// The LRU is implemented with a doubly-linked list (container/list) and a map
// from key to list element, giving O(1) promote and eviction.
type ResponseCache struct {
	mu      sync.RWMutex
	store   map[string]*list.Element
	lruList *list.List // front = LRU (oldest), back = MRU (newest)
	ttl     time.Duration
	maxSize int

	// maxMessageLen is the maximum user message length (in runes) eligible
	// for caching. Longer messages are too specific to benefit from caching.
	maxMessageLen int

	// now is a function returning the current time, injectable for testing.
	now func() time.Time
}

type lruEntry struct {
	key      string
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
		store:         make(map[string]*list.Element),
		lruList:       list.New(),
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

// cacheKey computes the cache key for a request. It hashes the user ID,
// system prompt, and the last user message content together. Including the
// user ID ensures different users never share cache entries even if they
// have identical system prompts and messages (defense-in-depth).
func cacheKey(userID, systemPrompt, userMessage string) string {
	h := sha256.New()
	h.Write([]byte(userID))
	h.Write([]byte{0}) // separator
	h.Write([]byte(systemPrompt))
	h.Write([]byte{0}) // separator
	h.Write([]byte(userMessage))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// Eligible returns true if the given ChatRequest is eligible for caching.
// Only selection mode with short user messages (< maxMessageLen runes)
// and no tool calls are cached. The userID parameter is accepted for API
// consistency but does not affect eligibility.
func (c *ResponseCache) Eligible(userID string, req *ChatRequest) bool {
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
// The userID is included in the cache key so different users never share entries.
func (c *ResponseCache) Get(userID string, req *ChatRequest) (string, bool) {
	msg := lastUserMessage(req)
	if msg == "" {
		return "", false
	}
	key := cacheKey(userID, req.SystemPrompt, msg)

	c.mu.RLock()
	elem, ok := c.store[key]
	if !ok {
		c.mu.RUnlock()
		return "", false
	}
	entry := elem.Value.(*lruEntry)
	expired := c.now().Sub(entry.cachedAt) > c.ttl
	c.mu.RUnlock()

	if expired {
		c.mu.Lock()
		// Re-check under write lock (another goroutine may have already evicted).
		if elem, stillOk := c.store[key]; stillOk {
			e := elem.Value.(*lruEntry)
			if c.now().Sub(e.cachedAt) > c.ttl {
				c.lruList.Remove(elem)
				delete(c.store, key)
			}
		}
		c.mu.Unlock()
		return "", false
	}

	// Promote to most recently used (O(1) move to back of list).
	// Re-lookup under write lock because `elem` captured under RLock may have
	// been evicted by a concurrent Put between the RUnlock and this Lock.
	c.mu.Lock()
	if current, ok := c.store[key]; ok {
		c.lruList.MoveToBack(current)
		// Re-read entry under write lock for safety.
		entry = current.Value.(*lruEntry)
	}
	c.mu.Unlock()

	slog.Debug("llm cache hit", "key_prefix", key[:12])
	return entry.response, true
}

// Put stores a response in the cache for the given request.
// If the cache is at capacity, the least recently used entry is evicted.
// The userID is included in the cache key so different users never share entries.
func (c *ResponseCache) Put(userID string, req *ChatRequest, response string) {
	msg := lastUserMessage(req)
	if msg == "" {
		return
	}
	key := cacheKey(userID, req.SystemPrompt, msg)

	c.mu.Lock()
	defer c.mu.Unlock()

	// If key already exists, update it and promote.
	if elem, exists := c.store[key]; exists {
		entry := elem.Value.(*lruEntry)
		entry.response = response
		entry.cachedAt = c.now()
		c.lruList.MoveToBack(elem)
		return
	}

	// Evict LRU entries if at capacity.
	for len(c.store) >= c.maxSize && c.lruList.Len() > 0 {
		oldest := c.lruList.Front()
		evicted := oldest.Value.(*lruEntry)
		c.lruList.Remove(oldest)
		delete(c.store, evicted.key)
		slog.Debug("llm cache eviction", "evicted_key_prefix", evicted.key[:12])
	}

	entry := &lruEntry{key: key, response: response, cachedAt: c.now()}
	elem := c.lruList.PushBack(entry)
	c.store[key] = elem
}

// Len returns the number of entries currently in the cache.
func (c *ResponseCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.store)
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
