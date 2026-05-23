package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gallowaysoftware/toqui-backend/internal/auth"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
)

// SearchHandler serves cross-trip search endpoints.
type SearchHandler struct {
	authSvc *auth.Service
	queries *dbgen.Queries
}

// NewSearchHandler creates a new SearchHandler.
func NewSearchHandler(authSvc *auth.Service, pool *pgxpool.Pool) *SearchHandler {
	return &SearchHandler{
		authSvc: authSvc,
		queries: dbgen.New(pool),
	}
}

// HandleSearchItinerary handles GET /api/search/itinerary?q=ramen&limit=20
func (h *SearchHandler) HandleSearchItinerary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := authenticateRESTRequest(r, h.authSvc)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		http.Error(w, "q parameter is required", http.StatusBadRequest)
		return
	}

	limit := int32(20)
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = int32(n)
		}
	}
	if limit > 100 {
		limit = 100
	}

	items, err := h.queries.SearchItineraryItems(r.Context(), dbgen.SearchItineraryItemsParams{
		UserID:     userID,
		Query:      pgtype.Text{String: q, Valid: true},
		MaxResults: limit,
	})
	if err != nil {
		slog.Error("search itinerary items failed", "error", err, "user_id", userID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"items": items,
		"count": len(items),
	})
}

// HandleSearchBookings handles GET /api/search/bookings?q=delta&limit=20
func (h *SearchHandler) HandleSearchBookings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := authenticateRESTRequest(r, h.authSvc)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		http.Error(w, "q parameter is required", http.StatusBadRequest)
		return
	}

	limit := int32(20)
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = int32(n)
		}
	}
	if limit > 100 {
		limit = 100
	}

	bookings, err := h.queries.SearchBookings(r.Context(), dbgen.SearchBookingsParams{
		UserID:     userID,
		Query:      pgtype.Text{String: q, Valid: true},
		MaxResults: limit,
	})
	if err != nil {
		slog.Error("search bookings failed", "error", err, "user_id", userID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"bookings": bookings,
		"count":    len(bookings),
	})
}
