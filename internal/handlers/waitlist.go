package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
)

// WaitlistHandler serves the public waitlist endpoints.
type WaitlistHandler struct {
	queries *dbgen.Queries
}

// NewWaitlistHandler creates a new WaitlistHandler.
func NewWaitlistHandler(pool *pgxpool.Pool) *WaitlistHandler {
	return &WaitlistHandler{
		queries: dbgen.New(pool),
	}
}

// waitlistRequest is the JSON body for POST /waitlist.
type waitlistRequest struct {
	Email string `json:"email"`
}

// waitlistResponse is the JSON response for POST /waitlist.
type waitlistResponse struct {
	Position int64 `json:"position"`
}

// waitlistStatusResponse is the JSON response for GET /waitlist/status.
type waitlistStatusResponse struct {
	Position int64 `json:"position"`
	Total    int64 `json:"total"`
}

// HandleJoin handles POST /waitlist — adds an email to the waitlist.
func (h *WaitlistHandler) HandleJoin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req waitlistRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Email == "" {
		http.Error(w, "email is required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Try to insert; ON CONFLICT DO NOTHING means no error if already exists,
	// but no row is returned either.
	entry, err := h.queries.AddToWaitlist(ctx, req.Email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Already on waitlist — look up existing entry for position
			entry, err = h.queries.GetWaitlistByEmail(ctx, req.Email)
			if err != nil {
				slog.Error("get waitlist by email failed", "email", req.Email, "error", err)
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
		} else {
			slog.Error("add to waitlist failed", "email", req.Email, "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}

	position, err := h.queries.CountWaitlistAhead(ctx, entry.SignedUpAt)
	if err != nil {
		slog.Error("count waitlist ahead failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Position is 1-indexed: the number of people ahead + 1
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(waitlistResponse{
		Position: position + 1,
	})
}

// HandleStatus handles GET /waitlist/status?email=... — returns position and total.
func (h *WaitlistHandler) HandleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	email := r.URL.Query().Get("email")
	if email == "" {
		http.Error(w, "email query parameter is required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	entry, err := h.queries.GetWaitlistByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "email not found on waitlist", http.StatusNotFound)
			return
		}
		slog.Error("get waitlist by email failed", "email", email, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	position, err := h.queries.CountWaitlistAhead(ctx, entry.SignedUpAt)
	if err != nil {
		slog.Error("count waitlist ahead failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	totalUsers, err := h.queries.CountUsers(ctx)
	if err != nil {
		slog.Error("count users failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(waitlistStatusResponse{
		Position: position + 1,
		Total:    totalUsers,
	})
}
