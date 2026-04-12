package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gallowaysoftware/toqui-backend/internal/auth"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui-backend/internal/tier"
	"github.com/gallowaysoftware/toqui-backend/internal/usage"
)

// UsageHandler serves the authenticated GET /api/usage endpoint.
type UsageHandler struct {
	usageSvc *usage.Service
	authSvc  *auth.Service
	queries  *dbgen.Queries
}

// NewUsageHandler creates a new UsageHandler.
func NewUsageHandler(usageSvc *usage.Service, authSvc *auth.Service, pool *pgxpool.Pool) *UsageHandler {
	return &UsageHandler{
		usageSvc: usageSvc,
		authSvc:  authSvc,
		queries:  dbgen.New(pool),
	}
}

// usageResponse is the JSON response for GET /api/usage.
type usageResponse struct {
	Used     int    `json:"used"`
	Limit    int    `json:"limit"`
	Tier     string `json:"tier"`
	ResetsAt string `json:"resets_at"`

	// Per-trip fields (only populated when trip_id query param is provided)
	ExpertCallsUsed *int32  `json:"expert_calls_used,omitempty"`
	TrialActive     *bool   `json:"trial_active,omitempty"`
	TrialEndsAt     *string `json:"trial_ends_at,omitempty"`
}

// HandleUsage returns current daily usage for the authenticated user.
func (h *UsageHandler) HandleUsage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := h.authenticateRequest(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Look up user tier
	userTier := tier.Free
	if h.queries != nil {
		if raw, err := h.queries.GetUserSubscriptionTier(r.Context(), userID); err == nil {
			userTier = tier.Parse(raw)
		}
	}

	count, limit, err := h.usageSvc.GetDailyUsageForTier(r.Context(), userID, userTier)
	if err != nil {
		slog.Error("get daily usage failed", "user_id", userID, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	resp := usageResponse{
		Used:     count,
		Limit:    limit,
		Tier:     string(userTier),
		ResetsAt: usage.ResetTime().Format("2006-01-02T15:04:05Z"),
	}

	// If trip_id is provided, include per-trip usage fields (#271).
	if tripIDStr := r.URL.Query().Get("trip_id"); tripIDStr != "" {
		if tripID, parseErr := uuid.Parse(tripIDStr); parseErr == nil && h.queries != nil {
			// Expert calls used for this trip
			if calls, err := h.queries.GetExpertCalls(r.Context(), dbgen.GetExpertCallsParams{
				ID:     tripID,
				UserID: userID,
			}); err == nil {
				resp.ExpertCallsUsed = &calls
			}

			// Trial status from the trip record
			if trip, err := h.queries.GetTripByID(r.Context(), dbgen.GetTripByIDParams{
				ID:     tripID,
				UserID: userID,
			}); err == nil {
				if trip.TrialStartedAt.Valid {
					active := trip.TrialEndsAt.Valid && trip.TrialEndsAt.Time.After(time.Now())
					resp.TrialActive = &active
					if trip.TrialEndsAt.Valid {
						endsAt := trip.TrialEndsAt.Time.Format(time.RFC3339)
						resp.TrialEndsAt = &endsAt
					}
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("failed to encode usage response", "error", err)
	}
}

// authenticateRequest delegates to the shared authenticateRESTRequest helper.
func (h *UsageHandler) authenticateRequest(r *http.Request) (uuid.UUID, bool) {
	return authenticateRESTRequest(r, h.authSvc)
}
