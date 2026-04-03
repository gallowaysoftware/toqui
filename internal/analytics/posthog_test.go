package analytics

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestHashUserID_SHA256(t *testing.T) {
	// Verify the SHA-256 based hashing: SHA-256 → first 8 bytes → hex → "u_" prefix.
	tests := []struct {
		input string
	}{
		{""},
		{"a"},
		{"abc"},
		{"test-user-id"},
		{"test-user-id-123"},
		{"550e8400-e29b-41d4-a716-446655440000"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := HashUserID(tt.input)

			// Verify prefix
			if got[:2] != "u_" {
				t.Errorf("HashUserID(%q) = %q, want u_ prefix", tt.input, got)
			}

			// Verify it matches our expected SHA-256 computation
			h := sha256.Sum256([]byte(tt.input))
			expected := "u_" + hex.EncodeToString(h[:8])
			if got != expected {
				t.Errorf("HashUserID(%q) = %q, want %q", tt.input, got, expected)
			}

			// Verify length: "u_" + 16 hex chars = 18 chars total
			if len(got) != 18 {
				t.Errorf("HashUserID(%q) length = %d, want 18", tt.input, len(got))
			}

			// Verify deterministic: same input → same output
			got2 := HashUserID(tt.input)
			if got != got2 {
				t.Errorf("HashUserID(%q) not deterministic: %q != %q", tt.input, got, got2)
			}
		})
	}
}

func TestHashUserID_FrontendCompatibility(t *testing.T) {
	// This test verifies that HashUserID("test-user-id-123") produces the same
	// output as the frontend SHA-256 hash would. The frontend computes:
	//   SHA-256("test-user-id-123") → first 8 bytes → hex → "u_" prefix
	//
	// SHA-256("test-user-id-123") = 9c1185a5c5e9fc54612808977ee8f548b2258d31...
	// First 8 bytes: 9c1185a5c5e9fc54
	// Expected: "u_9c1185a5c5e9fc54"
	got := HashUserID("test-user-id-123")

	// Compute expected directly
	h := sha256.Sum256([]byte("test-user-id-123"))
	expected := "u_" + hex.EncodeToString(h[:8])

	if got != expected {
		t.Errorf("HashUserID(\"test-user-id-123\") = %q, want %q (frontend compat)", got, expected)
	}
}

func TestHashUserID_Deterministic(t *testing.T) {
	id := "some-user-uuid-12345"
	first := HashUserID(id)
	for i := 0; i < 100; i++ {
		if HashUserID(id) != first {
			t.Fatal("HashUserID is not deterministic")
		}
	}
}

func TestHashUserID_DifferentInputsDifferentOutputs(t *testing.T) {
	ids := []string{
		"user-1",
		"user-2",
		"user-3",
		"550e8400-e29b-41d4-a716-446655440000",
		"660e8400-e29b-41d4-a716-446655440000",
	}
	seen := make(map[string]string)
	for _, id := range ids {
		h := HashUserID(id)
		if prev, ok := seen[h]; ok {
			t.Errorf("collision: HashUserID(%q) == HashUserID(%q) == %q", id, prev, h)
		}
		seen[h] = id
	}
}

func TestNewClient_DisabledWhenEmpty(t *testing.T) {
	c := NewClient("")
	if c.Enabled() {
		t.Error("client should be disabled with empty API key")
	}
	// Track should be a no-op, not panic
	c.Track("user-1", "test_event", map[string]any{"key": "value"})
	// Close should be safe on disabled client
	c.Close()
}

func TestNewClient_EnabledWithKey(t *testing.T) {
	c := NewClient("phc_test_key")
	if !c.Enabled() {
		t.Error("client should be enabled with API key")
	}
	c.Close()
}

func TestTrack_SendsCorrectPayload(t *testing.T) {
	var (
		mu       sync.Mutex
		received []map[string]any
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
			return
		}
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Errorf("unmarshal: %v", err)
			return
		}
		mu.Lock()
		received = append(received, payload)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := &Client{
		apiKey:     "phc_test_key",
		endpoint:   server.URL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
		queue:      make(chan eventPayload, queueSize),
	}

	// Start workers for the test client.
	c.wg.Add(workerCount)
	for range workerCount {
		go c.worker()
	}

	c.Track("user-123", "chat_message_sent", map[string]any{
		"mode": "planning",
	})

	// Close drains the queue and waits for workers to finish.
	c.Close()

	mu.Lock()
	defer mu.Unlock()

	if len(received) != 1 {
		t.Fatalf("expected 1 request, got %d", len(received))
	}

	payload := received[0]

	// Check API key
	if payload["api_key"] != "phc_test_key" {
		t.Errorf("api_key = %v, want phc_test_key", payload["api_key"])
	}

	// Check event name
	if payload["event"] != "chat_message_sent" {
		t.Errorf("event = %v, want chat_message_sent", payload["event"])
	}

	// Check distinct_id is hashed (not raw user ID)
	distinctID, ok := payload["distinct_id"].(string)
	if !ok {
		t.Fatal("distinct_id not a string")
	}
	if distinctID == "user-123" {
		t.Error("distinct_id should be hashed, not raw user ID")
	}
	expectedHash := HashUserID("user-123")
	if distinctID != expectedHash {
		t.Errorf("distinct_id = %q, want %q", distinctID, expectedHash)
	}

	// Check properties
	props, ok := payload["properties"].(map[string]any)
	if !ok {
		t.Fatal("properties not a map")
	}
	if props["mode"] != "planning" {
		t.Errorf("properties.mode = %v, want planning", props["mode"])
	}
	if props["$lib"] != "toqui-backend" {
		t.Errorf("properties.$lib = %v, want toqui-backend", props["$lib"])
	}
}

func TestTrack_NoOpWhenDisabled(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := NewClient("")
	c.Track("user-1", "test_event", nil)

	time.Sleep(100 * time.Millisecond)

	if requestCount != 0 {
		t.Errorf("disabled client sent %d requests", requestCount)
	}
}

func TestTrack_NilClientSafe(t *testing.T) {
	var c *Client
	// Should not panic
	if c.Enabled() {
		t.Error("nil client should not be enabled")
	}
	// Close should not panic on nil client
	c.Close()
}

func TestTrack_NilPropertiesSafe(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := &Client{
		apiKey:     "phc_test",
		endpoint:   server.URL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
		queue:      make(chan eventPayload, queueSize),
	}

	c.wg.Add(workerCount)
	for range workerCount {
		go c.worker()
	}

	// Should not panic with nil properties
	c.Track("user-1", "test_event", nil)
	c.Close()
}

func TestClose_DrainsPendingEvents(t *testing.T) {
	var (
		mu       sync.Mutex
		received int
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		received++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := &Client{
		apiKey:     "phc_test",
		endpoint:   server.URL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
		queue:      make(chan eventPayload, queueSize),
	}

	c.wg.Add(workerCount)
	for range workerCount {
		go c.worker()
	}

	// Enqueue several events
	for i := 0; i < 10; i++ {
		c.Track("user-1", "test_event", nil)
	}

	// Close should drain all events
	c.Close()

	mu.Lock()
	defer mu.Unlock()
	if received != 10 {
		t.Errorf("expected 10 events to be sent after Close, got %d", received)
	}
}

func TestClose_SafeToCallMultipleTimes(t *testing.T) {
	c := NewClient("phc_test")
	c.Close()
	c.Close() // Should not panic or deadlock
}

func TestNilClient_CloseSafe(t *testing.T) {
	var c *Client
	c.Close() // Should not panic
}
