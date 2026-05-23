package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// HandleReadiness is the load-balancer probe target on /ready. It must
// stay dependency-free — the whole point of /ready vs /health is that it
// doesn't touch the DB, so a backend with a temporarily-degraded DB is
// still routable. Pin the wire shape so a future "let's also check the DB
// here" change is an intentional contract break.

func TestHandleReadiness_ReturnsReadyTrue(t *testing.T) {
	h := NewHealthHandler(nil, time.Now()) // nil pool is fine — readiness doesn't ping
	rec := httptest.NewRecorder()
	h.HandleReadiness(rec, httptest.NewRequest(http.MethodGet, "/ready", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}

	body, _ := io.ReadAll(rec.Body)
	var parsed map[string]bool
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}
	if !parsed["ready"] {
		t.Errorf("expected ready=true, got %v", parsed)
	}
}

func TestHandleReadiness_RejectsNonGet(t *testing.T) {
	h := NewHealthHandler(nil, time.Now())
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		rec := httptest.NewRecorder()
		h.HandleReadiness(rec, httptest.NewRequest(method, "/ready", nil))
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("method %s: status = %d, want 405", method, rec.Code)
		}
	}
}

// HandleHealth's method-allow gate is testable without a pool — it
// short-circuits before the Ping. The DB-ping branch needs a real (or
// mocked-out) pool and is left for an integration test or a follow-up
// refactor that introduces a pinger interface.

func TestHandleHealth_RejectsNonGet(t *testing.T) {
	h := NewHealthHandler(nil, time.Now())
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		rec := httptest.NewRecorder()
		h.HandleHealth(rec, httptest.NewRequest(method, "/health", nil))
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("method %s: status = %d, want 405", method, rec.Code)
		}
	}
}

func TestNewHealthHandler_StoresStartTimeAndPool(t *testing.T) {
	// Constructor smoke — keeps the field assignment from being silently
	// dropped by a refactor. Uptime feeds the /health response, so the
	// startTime field reaching the struct is observable behavior.
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	h := NewHealthHandler(nil, now)
	if h == nil {
		t.Fatal("NewHealthHandler returned nil")
	}
	if !h.startTime.Equal(now) {
		t.Errorf("startTime = %v, want %v", h.startTime, now)
	}
	if h.pool != nil {
		t.Errorf("pool field = %v, want nil (we passed nil)", h.pool)
	}
}
