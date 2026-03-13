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
	json.NewEncoder(w).Encode(usageResponse{
		Used:     count,
		Limit:    limit,
		ResetsAt: usage.ResetTime().Format("2006-01-02T15:04:05Z"),
	})
}

// authenticateRequest extracts and validates a Bearer token from the request.
// It checks the Authorization header first (set by CookieAuth middleware for
// web browsers, or directly by native apps), then falls back to reading the
// HttpOnly access cookie directly as defense-in-depth.
func (h *UsageHandler) authenticateRequest(r *http.Request) (uuid.UUID, bool) {
	// Try Authorization header (covers both native Bearer and CookieAuth-bridged web)
	authHeader := r.Header.Get("Authorization")
	if len(authHeader) >= 8 && authHeader[:7] == "Bearer " {
		userID, err := h.authSvc.ValidateToken(authHeader[7:])
		if err == nil {
			return userID, true
		}
	}

	// Fallback: read HttpOnly cookie directly (defense-in-depth for web users)
	token := auth.AccessTokenFromCookie(r)
	if token != "" {
		userID, err := h.authSvc.ValidateToken(token)
		if err == nil {
			return userID, true
		}
	}

	return uuid.Nil, false
}
