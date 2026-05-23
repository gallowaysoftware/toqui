package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gallowaysoftware/toqui/backend/internal/auth"
	"github.com/gallowaysoftware/toqui/backend/internal/dbgen"
)

// feedbackQueries is the slice of *dbgen.Queries that FeedbackHandler
// depends on. Defining a small interface here lets unit tests inject
// a stub without spinning up Postgres. Mirrors the pattern in
// internal/booking, internal/lifecycle, internal/trip.
type feedbackQueries interface {
	GetTripByIDOrCollaborator(ctx context.Context, arg dbgen.GetTripByIDOrCollaboratorParams) (dbgen.Trip, error)
	CreateFeedback(ctx context.Context, arg dbgen.CreateFeedbackParams) (dbgen.Feedback, error)
}

var _ feedbackQueries = (*dbgen.Queries)(nil)

// FeedbackHandler handles user feedback submission.
type FeedbackHandler struct {
	authSvc *auth.Service
	queries feedbackQueries
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

	// Validate trip_id: only accept it if the caller can actually see
	// the trip (owner or accepted collaborator of any role). Without
	// this check any user could attach feedback to someone else's
	// trip; admin dashboards and /admin/feedback would then show the
	// misdirected association (#361 P3). Silent drop on unverified
	// IDs is fine — feedback without a trip_id is still a valid
	// submission.
	var tripID pgtype.UUID
	if req.TripID != "" {
		if parsed, err := uuid.Parse(req.TripID); err == nil {
			if _, accessErr := h.queries.GetTripByIDOrCollaborator(r.Context(), dbgen.GetTripByIDOrCollaboratorParams{
				ID:     parsed,
				UserID: userID,
			}); accessErr == nil {
				tripID = pgtype.UUID{Bytes: parsed, Valid: true}
			} else {
				slog.Info("feedback dropped unverified trip_id",
					"user_id", userID,
					"trip_id", parsed,
					"reason", "not owner or collaborator",
				)
			}
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
