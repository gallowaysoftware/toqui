package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

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

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(usageResponse{
		Used:     count,
		Limit:    limit,
		Tier:     string(userTier),
		ResetsAt: usage.ResetTime().Format("2006-01-02T15:04:05Z"),
	}); err != nil {
		slog.Error("failed to encode usage response", "error", err)
	}
}

// authenticateRequest delegates to the shared authenticateRESTRequest helper.
func (h *UsageHandler) authenticateRequest(r *http.Request) (uuid.UUID, bool) {
	return authenticateRESTRequest(r, h.authSvc)
}
