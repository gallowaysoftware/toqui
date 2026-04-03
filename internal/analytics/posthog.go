// Package analytics provides a lightweight PostHog client for server-side
// event tracking with privacy guarantees:
//   - User IDs are hashed before sending (Java-style string hash → abs → base36 → "u_" prefix)
//   - Async, non-blocking event dispatch via goroutines
//   - Gracefully no-ops when API key is empty
//   - EU endpoint only (eu.i.posthog.com)
package analytics

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"time"
)

// Client is a lightweight PostHog event tracker. It uses the PostHog HTTP API
// directly (/capture) instead of importing a Go SDK to keep dependencies minimal.
// A nil or zero-value Client is safe to use — all methods are no-ops.
type Client struct {
	apiKey     string
	endpoint   string
	httpClient *http.Client
}

// NewClient creates a new PostHog client. If apiKey is empty, returns a no-op
// client that silently discards all events.
func NewClient(apiKey string) *Client {
	if apiKey == "" {
		return &Client{}
	}
	return &Client{
		apiKey:   apiKey,
		endpoint: "https://eu.i.posthog.com",
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// Enabled reports whether this client will actually send events.
func (c *Client) Enabled() bool {
	return c != nil && c.apiKey != ""
}

// Track sends an event to PostHog asynchronously. The userID is hashed before
// sending. This method never blocks and never returns errors — failures are
// logged but do not affect the calling request.
func (c *Client) Track(userID, event string, properties map[string]any) {
	if !c.Enabled() {
		return
	}

	distinctID := HashUserID(userID)

	// Merge properties with library identifier.
	props := make(map[string]any, len(properties)+1)
	for k, v := range properties {
		props[k] = v
	}
	props["$lib"] = "toqui-backend"

	payload := map[string]any{
		"api_key":     c.apiKey,
		"event":       event,
		"distinct_id": distinctID,
		"properties":  props,
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
	}

	go c.send(payload)
}

func (c *Client) send(payload map[string]any) {
	body, err := json.Marshal(payload)
	if err != nil {
		slog.Error("posthog: marshal event failed", "error", err)
		return
	}

	req, err := http.NewRequest(http.MethodPost, c.endpoint+"/capture/", bytes.NewReader(body))
	if err != nil {
		slog.Error("posthog: create request failed", "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		slog.Error("posthog: send event failed", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		slog.Warn("posthog: unexpected status", "status", resp.StatusCode,
			"event", payload["event"])
	}
}

// HashUserID produces a privacy-safe distinct_id from a raw user ID.
// Algorithm: Java-style string hashCode → absolute value → base36 → "u_" prefix.
// This MUST match the frontend implementation so that backend and frontend events
// are attributed to the same PostHog user.
func HashUserID(userID string) string {
	h := javaStringHash(userID)
	abs := int64(math.Abs(float64(h)))
	return fmt.Sprintf("u_%s", strconv.FormatInt(abs, 36))
}

// javaStringHash replicates Java's String.hashCode():
//
//	s[0]*31^(n-1) + s[1]*31^(n-2) + ... + s[n-1]
//
// using int32 overflow semantics.
func javaStringHash(s string) int32 {
	var h int32
	for _, c := range s {
		h = 31*h + c
	}
	return h
}
