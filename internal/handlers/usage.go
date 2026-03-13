package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/gallowaysoftware/toqui-backend/internal/auth"
	"github.com/gallowaysoftware/toqui-backend/internal/usage"
)

// UsageHandler serves the authenticated GET /api/usage endpoint.
type UsageHandler struct {
	usageSvc *usage.Service
	authSvc  *auth.Service
}

// NewUsageHandler creates a new UsageHandler.
func NewUsageHandler(usageSvc *usage.Service, authSvc *auth.Service) *UsageHandler {
	return &UsageHandler{
		usageSvc: usageSvc,
		authSvc:  authSvc,
	}
}

// usageResponse is the JSON response for GET /api/usage.
type usageResponse struct {
	Used     int    `json:"used"`
	Limit    int    `json:"limit"`
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

	count, limit, err := h.usageSvc.GetDailyUsage(r.Context(), userID)
	if err != nil {
		slog.Error("get daily usage failed", "user_id", userID, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(usageResponse{
		Used:     count,
		Limit:    limit,
		ResetsAt: usage.ResetTime().Format("2006-01-02T15:04:05Z"),
	}); err != nil {
		slog.Error("failed to encode usage response", "error", err)
	}
}

// authenticateRequest delegates to the shared authenticateRESTRequest helper.
func (h *UsageHandler) authenticateRequest(r *http.Request) (uuid.UUID, bool) {
	return authenticateRESTRequest(r, h.authSvc)
}
