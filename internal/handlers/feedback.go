package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gallowaysoftware/toqui-backend/internal/auth"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
)

// FeedbackHandler handles user feedback submission.
type FeedbackHandler struct {
	authSvc *auth.Service
	queries *dbgen.Queries
}

// NewFeedbackHandler creates a new FeedbackHandler.
func NewFeedbackHandler(authSvc *auth.Service, pool *pgxpool.Pool) *FeedbackHandler {
	return &FeedbackHandler{
		authSvc: authSvc,
		queries: dbgen.New(pool),
	}
}

// HandleSubmitFeedback handles POST /api/feedback — submit user feedback with context.
func (h *FeedbackHandler) HandleSubmitFeedback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := authenticateRESTRequest(r, h.authSvc)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req struct {
		Type    string         `json:"type"`    // "bug", "feature", "general", "chat_quality"
		Message string         `json:"message"` // The user's feedback text
		Page    string         `json:"page"`    // Which screen they're on
		TripID  string         `json:"trip_id"` // Optional: which trip this is about
		Context map[string]any `json:"context"` // Optional: app version, OS, screen size, etc.
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Message == "" {
		http.Error(w, "message is required", http.StatusBadRequest)
		return
	}

	if req.Type == "" {
		req.Type = "general"
	}

	var contextBytes []byte
	if req.Context != nil {
		contextBytes, _ = json.Marshal(req.Context)
	}

	var tripID pgtype.UUID
	if req.TripID != "" {
		if parsed, err := uuid.Parse(req.TripID); err == nil {
			tripID = pgtype.UUID{Bytes: parsed, Valid: true}
		}
	}

	feedback, err := h.queries.CreateFeedback(r.Context(), dbgen.CreateFeedbackParams{
		UserID:  userID,
		Type:    req.Type,
		Message: req.Message,
		Context: contextBytes,
		Page:    pgtype.Text{String: req.Page, Valid: req.Page != ""},
		TripID:  tripID,
	})
	if err != nil {
		slog.Error("create feedback failed", "error", err, "user_id", userID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	slog.Info("user feedback received",
		"feedback_id", feedback.ID,
		"user_id", userID,
		"type", req.Type,
		"page", req.Page,
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "received",
		"id":     feedback.ID.String(),
	})
}
