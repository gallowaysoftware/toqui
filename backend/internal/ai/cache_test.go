package ai

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestResponseCache_HitAndMiss(t *testing.T) {
	cache := NewResponseCache()

	req := &ChatRequest{
		SystemPrompt: "You are a travel assistant.",
		Messages:     []Message{{Role: "user", Content: "Tell me about Paris"}},
		Mode:         "selection",
	}

	// Miss: nothing cached yet.
	if resp, ok := cache.Get("user-1", req); ok {
		t.Fatalf("expected cache miss, got hit with response: %q", resp)
	}

	// Put a response.
	cache.Put("user-1", req, "Paris is the capital of France!")

	// Hit: should return cached response.
	resp, ok := cache.Get("user-1", req)
	if !ok {
		t.Fatal("expected cache hit, got miss")
	}
	if resp != "Paris is the capital of France!" {
		t.Fatalf("unexpected cached response: %q", resp)
	}
}

func TestResponseCache_TTLExpiration(t *testing.T) {
	now := time.Now()
	clock := func() time.Time { return now }

	cache := NewResponseCache(
		WithTTL(10*time.Minute),
		withNow(clock),
	)

	req := &ChatRequest{
		SystemPrompt: "You are a travel assistant.",
		Messages:     []Message{{Role: "user", Content: "Tell me about Tokyo"}},
		Mode:         "selection",
	}

	cache.Put("user-1", req, "Tokyo is the capital of Japan!")

	// Hit: within TTL.
	if _, ok := cache.Get("user-1", req); !ok {
		t.Fatal("expected cache hit within TTL")
	}

	// Advance time past TTL.
	now = now.Add(11 * time.Minute)

	// Miss: entry expired.
	if _, ok := cache.Get("user-1", req); ok {
		t.Fatal("expected cache miss after TTL expiration")
	}

	// Verify the expired entry was cleaned up.
	if cache.Len() != 0 {
		t.Fatalf("expected 0 entries after expiration cleanup, got %d", cache.Len())
	}
}

func TestResponseCache_LRUEviction(t *testing.T) {
	cache := NewResponseCache(WithMaxSize(3))

	// Fill the cache with 3 entries.
	for i := 0; i < 3; i++ {
		req := &ChatRequest{
			SystemPrompt: "system",
			Messages:     []Message{{Role: "user", Content: fmt.Sprintf("query %d", i)}},
			Mode:         "selection",
		}
		cache.Put("user-1", req, fmt.Sprintf("response %d", i))
	}

	if cache.Len() != 3 {
		t.Fatalf("expected 3 entries, got %d", cache.Len())
	}

	// Access entry 0 to make it recently used.
	req0 := &ChatRequest{
		SystemPrompt: "system",
		Messages:     []Message{{Role: "user", Content: "query 0"}},
		Mode:         "selection",
	}
	if _, ok := cache.Get("user-1", req0); !ok {
		t.Fatal("expected hit for query 0")
	}

	// Add a 4th entry — should evict entry 1 (least recently used).
	req3 := &ChatRequest{
		SystemPrompt: "system",
		Messages:     []Message{{Role: "user", Content: "query 3"}},
		Mode:         "selection",
	}
	cache.Put("user-1", req3, "response 3")

	if cache.Len() != 3 {
		t.Fatalf("expected 3 entries after eviction, got %d", cache.Len())
	}

	// Entry 0 should still be present (was accessed recently).
	if _, ok := cache.Get("user-1", req0); !ok {
		t.Fatal("expected entry 0 to survive eviction")
	}

	// Entry 1 should be evicted.
	req1 := &ChatRequest{
		SystemPrompt: "system",
		Messages:     []Message{{Role: "user", Content: "query 1"}},
		Mode:         "selection",
	}
	if _, ok := cache.Get("user-1", req1); ok {
		t.Fatal("expected entry 1 to be evicted")
	}

	// Entry 2 should still be present.
	req2 := &ChatRequest{
		SystemPrompt: "system",
		Messages:     []Message{{Role: "user", Content: "query 2"}},
		Mode:         "selection",
	}
	if _, ok := cache.Get("user-1", req2); !ok {
		t.Fatal("expected entry 2 to survive eviction")
	}

	// Entry 3 should be present.
	if _, ok := cache.Get("user-1", req3); !ok {
		t.Fatal("expected entry 3 to be present")
	}
}

func TestResponseCache_Eligible(t *testing.T) {
	withTools := func() []ToolDefinition {
		return []ToolDefinition{
			{
				Name:        "create_trip",
				Description: "Create a new trip",
				Parameters:  json.RawMessage(`{"type":"object"}`),
			},
		}
	}

	tests := []struct {
		name string
		req  *ChatRequest
		want bool
	}{
		{
			name: "selection mode short message is eligible",
			req: &ChatRequest{
				Mode:     "selection",
				Messages: []Message{{Role: "user", Content: "Tell me about Paris"}},
			},
			want: true,
		},
		{
			name: "planning mode is not eligible",
			req: &ChatRequest{
				Mode:     "planning",
				Messages: []Message{{Role: "user", Content: "Tell me about Paris"}},
			},
			want: false,
		},
		{
			name: "companion mode is not eligible",
			req: &ChatRequest{
				Mode:     "companion",
				Messages: []Message{{Role: "user", Content: "Tell me about Paris"}},
			},
			want: false,
		},
		{
			name: "selection mode with tools is not eligible",
			req: &ChatRequest{
				Mode:     "selection",
				Messages: []Message{{Role: "user", Content: "Tell me about Paris"}},
				Tools:    withTools(),
			},
			want: false,
		},
		{
			name: "selection mode long message is not eligible",
			req: &ChatRequest{
				Mode:     "selection",
				Messages: []Message{{Role: "user", Content: strings.Repeat("x", 200)}},
			},
			want: false,
		},
		{
			name: "selection mode message at boundary (199 chars) is eligible",
			req: &ChatRequest{
				Mode:     "selection",
				Messages: []Message{{Role: "user", Content: strings.Repeat("x", 199)}},
			},
			want: true,
		},
		{
			name: "no messages is not eligible",
			req: &ChatRequest{
				Mode: "selection",
			},
			want: false,
		},
		{
			name: "only assistant messages is not eligible",
			req: &ChatRequest{
				Mode:     "selection",
				Messages: []Message{{Role: "assistant", Content: "Hello!"}},
			},
			want: false,
		},
	}

	cache := NewResponseCache()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cache.Eligible("user-1", tt.req)
			if got != tt.want {
				t.Errorf("Eligible() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResponseCache_DifferentUsersAreDifferentKeys(t *testing.T) {
	cache := NewResponseCache()

	req := &ChatRequest{
		SystemPrompt: "You are a travel assistant.",
		Messages:     []Message{{Role: "user", Content: "Tell me about Paris"}},
		Mode:         "selection",
	}

	cache.Put("user-A", req, "Response for user A")
	cache.Put("user-B", req, "Response for user B")

	respA, ok := cache.Get("user-A", req)
	if !ok || respA != "Response for user A" {
		t.Fatalf("expected user A response, got ok=%v resp=%q", ok, respA)
	}

	respB, ok := cache.Get("user-B", req)
	if !ok || respB != "Response for user B" {
		t.Fatalf("expected user B response, got ok=%v resp=%q", ok, respB)
	}

	// A third user should get a cache miss.
	if _, ok := cache.Get("user-C", req); ok {
		t.Fatal("expected cache miss for user C")
	}
}

func TestResponseCache_DifferentPromptsAreDifferentKeys(t *testing.T) {
	cache := NewResponseCache()

	req1 := &ChatRequest{
		SystemPrompt: "Persona A",
		Messages:     []Message{{Role: "user", Content: "Tell me about Paris"}},
		Mode:         "selection",
	}
	req2 := &ChatRequest{
		SystemPrompt: "Persona B",
		Messages:     []Message{{Role: "user", Content: "Tell me about Paris"}},
		Mode:         "selection",
	}

	cache.Put("user-1", req1, "Response from Persona A")
	cache.Put("user-1", req2, "Response from Persona B")

	resp1, ok := cache.Get("user-1", req1)
	if !ok || resp1 != "Response from Persona A" {
		t.Fatalf("expected Persona A response, got ok=%v resp=%q", ok, resp1)
	}

	resp2, ok := cache.Get("user-1", req2)
	if !ok || resp2 != "Response from Persona B" {
		t.Fatalf("expected Persona B response, got ok=%v resp=%q", ok, resp2)
	}
}

func TestResponseCache_UpdateExistingEntry(t *testing.T) {
	cache := NewResponseCache()

	req := &ChatRequest{
		SystemPrompt: "system",
		Messages:     []Message{{Role: "user", Content: "hello"}},
		Mode:         "selection",
	}

	cache.Put("user-1", req, "first response")
	cache.Put("user-1", req, "updated response")

	resp, ok := cache.Get("user-1", req)
	if !ok {
		t.Fatal("expected cache hit")
	}
	if resp != "updated response" {
		t.Fatalf("expected updated response, got %q", resp)
	}

	// Should not have created a duplicate entry.
	if cache.Len() != 1 {
		t.Fatalf("expected 1 entry, got %d", cache.Len())
	}
}

func TestResponseCache_EmptyUserMessage(t *testing.T) {
	cache := NewResponseCache()

	req := &ChatRequest{
		SystemPrompt: "system",
		Messages:     []Message{{Role: "user", Content: ""}},
		Mode:         "selection",
	}

	// Should not be eligible.
	if cache.Eligible("user-1", req) {
		t.Fatal("empty user message should not be eligible")
	}

	// Put should be a no-op.
	cache.Put("user-1", req, "should not be stored")
	if cache.Len() != 0 {
		t.Fatal("expected no entries after Put with empty user message")
	}
}

func TestResponseCache_ConcurrentAccess(t *testing.T) {
	cache := NewResponseCache(WithMaxSize(100))

	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 50; j++ {
				req := &ChatRequest{
					SystemPrompt: "system",
					Messages:     []Message{{Role: "user", Content: fmt.Sprintf("query %d-%d", id, j)}},
					Mode:         "selection",
				}
				cache.Put(fmt.Sprintf("user-%d", id), req, fmt.Sprintf("response %d-%d", id, j))
				cache.Get(fmt.Sprintf("user-%d", id), req)
			}
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// Just verify no panics and size is within bounds.
	if cache.Len() > 100 {
		t.Fatalf("cache size %d exceeds max size 100", cache.Len())
	}
}

func TestCacheKey_Deterministic(t *testing.T) {
	k1 := cacheKey("user-1", "system prompt", "user message")
	k2 := cacheKey("user-1", "system prompt", "user message")
	if k1 != k2 {
		t.Fatalf("same inputs produced different keys: %q vs %q", k1, k2)
	}

	k3 := cacheKey("user-1", "system prompt", "different message")
	if k1 == k3 {
		t.Fatal("different inputs produced same key")
	}

	// Different user IDs produce different keys even with identical prompt+message.
	k4 := cacheKey("user-2", "system prompt", "user message")
	if k1 == k4 {
		t.Fatal("different user IDs produced same key")
	}
}
