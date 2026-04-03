// Package analytics provides a lightweight PostHog client for server-side
// event tracking with privacy guarantees:
//   - User IDs are SHA-256 hashed before sending (64-bit truncation, hex-encoded, "u_" prefix)
//   - Async, non-blocking event dispatch via buffered channel + fixed worker pool
//   - Gracefully no-ops when API key is empty
//   - EU endpoint only (eu.i.posthog.com)
package analytics

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

const (
	// workerCount is the number of goroutines processing the event queue.
	workerCount = 4

	// queueSize is the buffered channel capacity. If the channel is full,
	// new events are dropped (logged as a warning) to avoid backpressure
	// leaking into request-serving goroutines.
	queueSize = 256
)

// Client is a lightweight PostHog event tracker. It uses the PostHog HTTP API
// directly (/capture) instead of importing a Go SDK to keep dependencies minimal.
// A nil or zero-value Client is safe to use — all methods are no-ops.
type Client struct {
	apiKey     string
	endpoint   string
	httpClient *http.Client

	queue chan eventPayload
	wg    sync.WaitGroup
	once  sync.Once // ensures Close only runs once
}

// eventPayload is the internal representation of a PostHog event.
type eventPayload struct {
	APIKey     string         `json:"api_key"`
	Event      string         `json:"event"`
	DistinctID string         `json:"distinct_id"`
	Properties map[string]any `json:"properties"`
	Timestamp  string         `json:"timestamp"`
}

// NewClient creates a new PostHog client. If apiKey is empty, returns a no-op
// client that silently discards all events. The client starts a fixed-size
// worker pool for async event delivery.
func NewClient(apiKey string) *Client {
	if apiKey == "" {
		return &Client{}
	}
	c := &Client{
		apiKey:   apiKey,
		endpoint: "https://eu.i.posthog.com",
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		queue: make(chan eventPayload, queueSize),
	}

	// Start worker pool.
	c.wg.Add(workerCount)
	for range workerCount {
		go c.worker()
	}

	return c
}

// Enabled reports whether this client will actually send events.
func (c *Client) Enabled() bool {
	return c != nil && c.apiKey != ""
}

// Track sends an event to PostHog asynchronously. The userID is hashed before
// sending. This method never blocks and never returns errors — if the internal
// queue is full the event is dropped with a warning log.
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

	payload := eventPayload{
		APIKey:     c.apiKey,
		Event:      event,
		DistinctID: distinctID,
		Properties: props,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
	}

	select {
	case c.queue <- payload:
		// Enqueued successfully.
	default:
		// Queue full — drop the event to avoid blocking the request goroutine.
		slog.Warn("posthog: event queue full, dropping event",
			"event", event,
			"queue_size", queueSize,
		)
	}
}

// Close drains the event queue and waits for all workers to finish. It should
// be called during server shutdown to ensure in-flight events are flushed.
// Close is safe to call multiple times and on a nil/no-op client.
func (c *Client) Close() {
	if !c.Enabled() {
		return
	}
	c.once.Do(func() {
		close(c.queue)
		c.wg.Wait()
	})
}

// worker processes events from the queue until the channel is closed.
func (c *Client) worker() {
	defer c.wg.Done()
	for payload := range c.queue {
		c.send(payload)
	}
}

func (c *Client) send(payload eventPayload) {
	body, err := json.Marshal(payload)
	if err != nil {
		slog.Error("posthog: marshal event failed", "error", err)
		return
	}

	// Use a timeout context so requests can be cancelled during shutdown.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+"/capture/", bytes.NewReader(body))
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
			"event", payload.Event)
	}
}

// HashUserID produces a privacy-safe distinct_id from a raw user ID.
// Algorithm: SHA-256 → first 8 bytes (64 bits) → hex-encoded → "u_" prefix.
// This MUST match the frontend implementation so that backend and frontend events
// are attributed to the same PostHog user.
func HashUserID(userID string) string {
	h := sha256.Sum256([]byte(userID))
	return "u_" + hex.EncodeToString(h[:8]) // 16 hex chars = 64 bits
}
