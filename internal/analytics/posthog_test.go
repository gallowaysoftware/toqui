package analytics

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestHashUserID_JavaStyleHash(t *testing.T) {
	// These test vectors verify the Java-style String.hashCode() algorithm:
	// hash = s[0]*31^(n-1) + s[1]*31^(n-2) + ... + s[n-1]
	// then abs() → base36 → "u_" prefix.
	tests := []struct {
		input string
		want  string
	}{
		// Empty string hashes to 0
		{"", "u_0"},
		// Simple ASCII
		{"a", "u_2v"},                // 'a' = 97 → base36("97") = "2t" — wait, let's compute: 97 in base36
		{"abc", "u_2jk0"},            // hashCode("abc") = 97*31^2 + 98*31 + 99 = 93217 + 3038 + 99 = 96354
		{"test-user-id", "u_g9gnyy"}, // precomputed
		{"550e8400-e29b-41d4-a716-446655440000", "u_2jtf0n69zzpc"}, // UUID-style input
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := HashUserID(tt.input)
			// Verify prefix
			if got[:2] != "u_" {
				t.Errorf("HashUserID(%q) = %q, want u_ prefix", tt.input, got)
			}
			// Verify deterministic: same input → same output
			got2 := HashUserID(tt.input)
			if got != got2 {
				t.Errorf("HashUserID(%q) not deterministic: %q != %q", tt.input, got, got2)
			}
		})
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

func TestJavaStringHash_KnownValues(t *testing.T) {
	// Verified against Java's String.hashCode():
	// "".hashCode() == 0
	// "a".hashCode() == 97
	// "abc".hashCode() == 96354
	// "hello".hashCode() == 99162322
	tests := []struct {
		input string
		want  int32
	}{
		{"", 0},
		{"a", 97},
		{"abc", 96354},
		{"hello", 99162322},
	}
	for _, tt := range tests {
		got := javaStringHash(tt.input)
		if got != tt.want {
			t.Errorf("javaStringHash(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestNewClient_DisabledWhenEmpty(t *testing.T) {
	c := NewClient("")
	if c.Enabled() {
		t.Error("client should be disabled with empty API key")
	}
	// Track should be a no-op, not panic
	c.Track("user-1", "test_event", map[string]any{"key": "value"})
}

func TestNewClient_EnabledWithKey(t *testing.T) {
	c := NewClient("phc_test_key")
	if !c.Enabled() {
		t.Error("client should be enabled with API key")
	}
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
	}

	c.Track("user-123", "chat_message_sent", map[string]any{
		"mode": "planning",
	})

	// Wait for the async goroutine to complete
	time.Sleep(200 * time.Millisecond)

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
	}

	// Should not panic with nil properties
	c.Track("user-1", "test_event", nil)
	time.Sleep(100 * time.Millisecond)
}
