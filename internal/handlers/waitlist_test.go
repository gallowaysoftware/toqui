package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWaitlistHandler_HandleJoin_MethodNotAllowed(t *testing.T) {
	// WaitlistHandler requires a DB pool, but we can test HTTP-level
	// validation without hitting the database.
	handler := &WaitlistHandler{}

	req := httptest.NewRequest(http.MethodGet, "/waitlist", nil)
	w := httptest.NewRecorder()
	handler.HandleJoin(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestWaitlistHandler_HandleJoin_EmptyBody(t *testing.T) {
	handler := &WaitlistHandler{}

	req := httptest.NewRequest(http.MethodPost, "/waitlist", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.HandleJoin(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestWaitlistHandler_HandleJoin_InvalidJSON(t *testing.T) {
	handler := &WaitlistHandler{}

	req := httptest.NewRequest(http.MethodPost, "/waitlist", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.HandleJoin(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestWaitlistHandler_HandleStatus_MethodNotAllowed(t *testing.T) {
	handler := &WaitlistHandler{}

	req := httptest.NewRequest(http.MethodPost, "/waitlist/status", nil)
	w := httptest.NewRecorder()
	handler.HandleStatus(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestWaitlistHandler_HandleStatus_MissingEmail(t *testing.T) {
	handler := &WaitlistHandler{}

	req := httptest.NewRequest(http.MethodGet, "/waitlist/status", nil)
	w := httptest.NewRecorder()
	handler.HandleStatus(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}
