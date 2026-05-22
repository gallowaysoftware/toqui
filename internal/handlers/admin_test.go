package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHandleAICosts_MethodNotAllowed(t *testing.T) {
	handler := &AdminHandler{}

	req := httptest.NewRequest(http.MethodPost, "/admin/ai-costs", nil)
	w := httptest.NewRecorder()
	handler.HandleAICosts(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestHandleAICosts_Unauthenticated(t *testing.T) {
	handler := &AdminHandler{
		authSvc: newTestAuthService(),
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/ai-costs", nil)
	w := httptest.NewRecorder()
	handler.HandleAICosts(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestHandleRevenue_MethodNotAllowed(t *testing.T) {
	handler := &AdminHandler{}

	req := httptest.NewRequest(http.MethodPost, "/admin/revenue", nil)
	w := httptest.NewRecorder()
	handler.HandleRevenue(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestHandleRevenue_Unauthenticated(t *testing.T) {
	handler := &AdminHandler{
		authSvc: newTestAuthService(),
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/revenue", nil)
	w := httptest.NewRecorder()
	handler.HandleRevenue(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestHandleStats_MethodNotAllowed(t *testing.T) {
	handler := &AdminHandler{}

	req := httptest.NewRequest(http.MethodPost, "/admin/stats", nil)
	w := httptest.NewRecorder()
	handler.HandleStats(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestHandleMetrics_MethodNotAllowed(t *testing.T) {
	handler := &AdminHandler{}

	req := httptest.NewRequest(http.MethodPost, "/admin/metrics", nil)
	w := httptest.NewRecorder()
	handler.HandleMetrics(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestHandleSetAdmin_MethodNotAllowed(t *testing.T) {
	handler := &AdminHandler{}

	req := httptest.NewRequest(http.MethodGet, "/admin/set-admin", nil)
	w := httptest.NewRecorder()
	handler.HandleSetAdmin(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestHandleSetAdmin_Unauthenticated(t *testing.T) {
	handler := &AdminHandler{
		authSvc: newTestAuthService(),
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/set-admin", nil)
	w := httptest.NewRecorder()
	handler.HandleSetAdmin(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestHandleRetention_MethodNotAllowed(t *testing.T) {
	handler := &AdminHandler{}

	req := httptest.NewRequest(http.MethodPost, "/admin/retention", nil)
	w := httptest.NewRecorder()
	handler.HandleRetention(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestHandleRetention_Unauthenticated(t *testing.T) {
	handler := &AdminHandler{
		authSvc: newTestAuthService(),
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/retention", nil)
	w := httptest.NewRecorder()
	handler.HandleRetention(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestHandleFunnel_MethodNotAllowed(t *testing.T) {
	handler := &AdminHandler{}

	req := httptest.NewRequest(http.MethodPost, "/admin/funnel", nil)
	w := httptest.NewRecorder()
	handler.HandleFunnel(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestHandleFunnel_Unauthenticated(t *testing.T) {
	handler := &AdminHandler{
		authSvc: newTestAuthService(),
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/funnel", nil)
	w := httptest.NewRecorder()
	handler.HandleFunnel(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestParseSinceParam(t *testing.T) {
	tests := []struct {
		name           string
		query          string
		defaultDaysAgo int
		wantDate       string // empty means "check it's roughly defaultDaysAgo ago"
	}{
		{
			name:           "explicit date",
			query:          "since=2026-01-15",
			defaultDaysAgo: 90,
			wantDate:       "2026-01-15",
		},
		{
			name:           "invalid date falls back to default",
			query:          "since=not-a-date",
			defaultDaysAgo: 30,
		},
		{
			name:           "missing param falls back to default",
			query:          "",
			defaultDaysAgo: 60,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/admin/retention"
			if tt.query != "" {
				url += "?" + tt.query
			}
			req := httptest.NewRequest(http.MethodGet, url, nil)
			got := parseSinceParam(req, tt.defaultDaysAgo)

			if tt.wantDate != "" {
				gotStr := got.Format("2006-01-02")
				if gotStr != tt.wantDate {
					t.Errorf("parseSinceParam() = %s, want %s", gotStr, tt.wantDate)
				}
			} else {
				// Should be approximately defaultDaysAgo before now.
				expected := time.Now().UTC().AddDate(0, 0, -tt.defaultDaysAgo)
				diff := got.Sub(expected)
				if diff < -time.Minute || diff > time.Minute {
					t.Errorf("parseSinceParam() = %v, expected ~%v (diff: %v)", got, expected, diff)
				}
			}
		})
	}
}

func TestAuthenticateAdmin_NoAuth(t *testing.T) {
	// authenticateAdmin should return errUnauthorized when no auth header is present.
	handler := &AdminHandler{
		authSvc: newTestAuthService(),
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/stats", nil)
	_, err := handler.authenticateAdmin(req)
	if !errors.Is(err, errUnauthorized) {
		t.Errorf("expected errUnauthorized, got %v", err)
	}
}

func TestAuthenticateAdmin_InvalidToken(t *testing.T) {
	// authenticateAdmin should return errUnauthorized for an invalid JWT.
	handler := &AdminHandler{
		authSvc: newTestAuthService(),
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/stats", nil)
	req.Header.Set("Authorization", "Bearer invalid-jwt-token")
	_, err := handler.authenticateAdmin(req)
	if !errors.Is(err, errUnauthorized) {
		t.Errorf("expected errUnauthorized, got %v", err)
	}
}

// Note: authenticateAdmin tests with a real DB (admin via is_admin column,
// admin via ADMIN_EMAILS fallback/seed, non-admin denial) are covered by
// the integration test suite (internal/integration/) which runs against a
// real PostgreSQL instance. Unit tests here validate the HTTP layer only.
