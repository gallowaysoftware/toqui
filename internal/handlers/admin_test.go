package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
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

func TestSubscriptionMRR_KnownTiers(t *testing.T) {
	// Verify that subscription MRR prices are set for known tiers.
	tests := []struct {
		tier string
		want float64
	}{
		{"explorer", 9.99},
		{"voyager", 19.99},
	}
	for _, tt := range tests {
		if got, ok := subscriptionMRR[tt.tier]; !ok {
			t.Errorf("subscriptionMRR missing tier %q", tt.tier)
		} else if got != tt.want {
			t.Errorf("subscriptionMRR[%q] = %v, want %v", tt.tier, got, tt.want)
		}
	}
}

func TestSubscriptionMRR_UnknownTier(t *testing.T) {
	// Unknown tiers should not be in the map (they contribute zero MRR).
	if _, ok := subscriptionMRR["free"]; ok {
		t.Error("subscriptionMRR should not contain 'free' tier")
	}
}
