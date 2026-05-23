package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// --- Public view endpoint tests ---

func TestSharedHandler_HandlePublicView_MethodNotAllowed(t *testing.T) {
	handler := &SharedHandler{}

	req := httptest.NewRequest(http.MethodPost, "/shared/abc123", nil)
	w := httptest.NewRecorder()
	handler.HandlePublicView(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestSharedHandler_HandlePublicView_EmptyToken(t *testing.T) {
	handler := &SharedHandler{}

	req := httptest.NewRequest(http.MethodGet, "/shared/", nil)
	w := httptest.NewRecorder()
	handler.HandlePublicView(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestSharedHandler_HandlePublicView_InvalidToken(t *testing.T) {
	// tripSvc is nil, so GetByShareToken will panic if called.
	// We test that the handler does not crash on a nil tripSvc by testing
	// only the path extraction — a full integration test would need a DB.
	// This tests the 404 for token with slashes (invalid path).
	handler := &SharedHandler{}

	req := httptest.NewRequest(http.MethodGet, "/shared/abc/def", nil)
	w := httptest.NewRecorder()
	handler.HandlePublicView(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

// --- Enable sharing endpoint tests ---

func TestSharedHandler_HandleEnable_MethodNotAllowed(t *testing.T) {
	handler := &SharedHandler{}

	req := httptest.NewRequest(http.MethodGet, "/api/trips/share", nil)
	w := httptest.NewRecorder()
	handler.HandleEnable(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestSharedHandler_HandleEnable_MissingAuth(t *testing.T) {
	handler := &SharedHandler{}

	req := httptest.NewRequest(http.MethodPost, "/api/trips/share", strings.NewReader(`{"trip_id":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.HandleEnable(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestSharedHandler_HandleEnable_InvalidBearerToken(t *testing.T) {
	authSvc := newTestAuthService()
	handler := &SharedHandler{authSvc: authSvc}

	req := httptest.NewRequest(http.MethodPost, "/api/trips/share", strings.NewReader(`{"trip_id":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()
	handler.HandleEnable(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestSharedHandler_HandleEnable_InvalidTripID(t *testing.T) {
	authSvc := newTestAuthService()
	handler := &SharedHandler{authSvc: authSvc}

	// Generate a valid token for auth
	userID := mustNewUUID(t)
	token, err := authSvc.GenerateAccessToken(userID)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/trips/share", strings.NewReader(`{"trip_id":"not-a-uuid"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.HandleEnable(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// --- Disable sharing endpoint tests ---

func TestSharedHandler_HandleDisable_MethodNotAllowed(t *testing.T) {
	handler := &SharedHandler{}

	req := httptest.NewRequest(http.MethodGet, "/api/trips/unshare", nil)
	w := httptest.NewRecorder()
	handler.HandleDisable(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestSharedHandler_HandleDisable_MissingAuth(t *testing.T) {
	handler := &SharedHandler{}

	req := httptest.NewRequest(http.MethodPost, "/api/trips/unshare", strings.NewReader(`{"trip_id":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.HandleDisable(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestSharedHandler_HandleDisable_InvalidBearerToken(t *testing.T) {
	authSvc := newTestAuthService()
	handler := &SharedHandler{authSvc: authSvc}

	req := httptest.NewRequest(http.MethodPost, "/api/trips/unshare", strings.NewReader(`{"trip_id":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()
	handler.HandleDisable(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestSharedHandler_HandleDisable_InvalidTripID(t *testing.T) {
	authSvc := newTestAuthService()
	handler := &SharedHandler{authSvc: authSvc}

	userID := mustNewUUID(t)
	token, err := authSvc.GenerateAccessToken(userID)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/trips/unshare", strings.NewReader(`{"trip_id":"not-a-uuid"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.HandleDisable(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// --- Helpers ---

func mustNewUUID(t *testing.T) uuid.UUID {
	t.Helper()
	id, err := uuid.NewRandom()
	if err != nil {
		t.Fatalf("new UUID: %v", err)
	}
	return id
}
