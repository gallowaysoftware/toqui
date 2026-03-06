package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUsageHandler_HandleUsage_MethodNotAllowed(t *testing.T) {
	handler := &UsageHandler{}

	req := httptest.NewRequest(http.MethodPost, "/api/usage", nil)
	w := httptest.NewRecorder()
	handler.HandleUsage(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestUsageHandler_HandleUsage_MissingAuth(t *testing.T) {
	handler := &UsageHandler{}

	req := httptest.NewRequest(http.MethodGet, "/api/usage", nil)
	w := httptest.NewRecorder()
	handler.HandleUsage(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestUsageHandler_HandleUsage_InvalidAuth(t *testing.T) {
	handler := &UsageHandler{}

	req := httptest.NewRequest(http.MethodGet, "/api/usage", nil)
	req.Header.Set("Authorization", "Basic invalid")
	w := httptest.NewRecorder()
	handler.HandleUsage(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestUsageHandler_HandleUsage_InvalidBearerToken(t *testing.T) {
	// Need a real auth service to validate the token
	authSvc := newTestAuthService()
	handler := &UsageHandler{authSvc: authSvc}

	req := httptest.NewRequest(http.MethodGet, "/api/usage", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()
	handler.HandleUsage(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}
