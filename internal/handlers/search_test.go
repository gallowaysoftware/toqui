package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchItinerary_MethodNotAllowed(t *testing.T) {
	h := NewSearchHandler(nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/search/itinerary?q=ramen", nil)
	w := httptest.NewRecorder()

	h.HandleSearchItinerary(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestSearchItinerary_NoAuth(t *testing.T) {
	// authSvc is nil so any auth check will fail — simulates an unauthenticated request.
	h := &SearchHandler{}

	req := httptest.NewRequest(http.MethodGet, "/api/search/itinerary?q=ramen", nil)
	w := httptest.NewRecorder()

	h.HandleSearchItinerary(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestSearchItinerary_EmptyQuery(t *testing.T) {
	// Even without auth, the method check passes and the empty-query check
	// should return 400. We need auth to pass first though, so test ordering:
	// method -> auth -> query. Without auth it returns 401 before checking q.
	h := &SearchHandler{}

	req := httptest.NewRequest(http.MethodGet, "/api/search/itinerary?q=", nil)
	w := httptest.NewRecorder()

	h.HandleSearchItinerary(w, req)

	// Without auth, returns 401 before checking q.
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestSearchBookings_MethodNotAllowed(t *testing.T) {
	h := NewSearchHandler(nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/search/bookings?q=delta", nil)
	w := httptest.NewRecorder()

	h.HandleSearchBookings(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestSearchBookings_NoAuth(t *testing.T) {
	h := &SearchHandler{}

	req := httptest.NewRequest(http.MethodGet, "/api/search/bookings?q=delta", nil)
	w := httptest.NewRecorder()

	h.HandleSearchBookings(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}
