package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gallowaysoftware/toqui-backend/internal/audit"
	"github.com/gallowaysoftware/toqui-backend/internal/auth"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui-backend/internal/email"
	"github.com/gallowaysoftware/toqui-backend/internal/ratelimit"
)

// maxInvitesPerTrip is the maximum number of collaborators allowed per trip.
const maxInvitesPerTrip = 10

// CollaborateHandler handles trip collaboration endpoints.
type CollaborateHandler struct {
	authSvc       *auth.Service
	queries       *dbgen.Queries
	emailSvc      *email.Sender
	appURL        string
	inviteLimiter *ratelimit.RESTLimiter
}

// NewCollaborateHandler creates a new CollaborateHandler.
func NewCollaborateHandler(authSvc *auth.Service, pool *pgxpool.Pool, emailSvc *email.Sender, appURL string) *CollaborateHandler {
	return &CollaborateHandler{
		authSvc:       authSvc,
		queries:       dbgen.New(pool),
		emailSvc:      emailSvc,
		appURL:        appURL,
		inviteLimiter: ratelimit.NewRESTLimiter(5, 10*time.Minute), // 5 invites per trip per 10 minutes
	}
}

// HandleInvite handles POST /api/trips/{tripId}/invite — invite a collaborator by email.
func (h *CollaborateHandler) HandleInvite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := authenticateRESTRequest(r, h.authSvc)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	tripID, err := parseTripIDFromPath(r.URL.Path, "/api/trips/", "/invite")
	if err != nil {
		http.Error(w, "invalid trip ID", http.StatusBadRequest)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" {
		http.Error(w, "email is required", http.StatusBadRequest)
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Role == "" {
		req.Role = "editor"
	}
	if req.Role != "editor" && req.Role != "viewer" {
		http.Error(w, "role must be 'editor' or 'viewer'", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Only trip owner can invite
	ownerID, err := h.queries.GetTripOwner(ctx, tripID)
	if err != nil {
		http.Error(w, "trip not found", http.StatusNotFound)
		return
	}
	if ownerID != userID {
		http.Error(w, "only the trip owner can invite collaborators", http.StatusForbidden)
		return
	}

	// Per-user rate limit: 5 invites per trip per 10 minutes
	rateLimitKey := fmt.Sprintf("invite:%s:%s", userID.String(), tripID.String())
	if !h.inviteLimiter.Allow(rateLimitKey) {
		ratelimit.Reject(w, "too many invites for this trip, please try again later")
		return
	}

	// Rate limit: max 10 invites per trip
	count, err := h.queries.CountCollaboratorsByTrip(ctx, tripID)
	if err != nil {
		slog.Error("count collaborators failed", "error", err, "trip_id", tripID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if count >= maxInvitesPerTrip {
		http.Error(w, "maximum number of collaborators reached", http.StatusConflict)
		return
	}

	// Generate invite token
	token, err := generateInviteToken()
	if err != nil {
		slog.Error("generate invite token failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	collab, err := h.queries.AddCollaborator(ctx, dbgen.AddCollaboratorParams{
		TripID:      tripID,
		Email:       req.Email,
		Role:        req.Role,
		InviteToken: pgtype.Text{String: token, Valid: true},
		InvitedBy:   userID,
	})
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
			http.Error(w, "this email has already been invited to this trip", http.StatusConflict)
			return
		}
		slog.Error("add collaborator failed", "error", err, "trip_id", tripID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Send invite email
	if h.emailSvc != nil {
		// Get inviter name and trip title for the email
		inviter, _ := h.queries.GetUserByID(ctx, userID)
		trip, _ := h.queries.GetTripByID(ctx, dbgen.GetTripByIDParams{ID: tripID, UserID: userID})

		inviterName := "Someone"
		if inviter.Name.Valid && inviter.Name.String != "" {
			inviterName = inviter.Name.String
		} else if inviter.Email != "" {
			inviterName = inviter.Email
		}

		tripTitle := "a trip"
		if trip.Title != "" {
			tripTitle = trip.Title
		}

		acceptURL := h.appURL + "/trips/invite?token=" + token
		go func() {
			if err := h.emailSvc.SendCollabInvite(req.Email, inviterName, tripTitle, acceptURL); err != nil {
				slog.Error("collab invite email failed", "error", err, "to", maskEmail(req.Email))
			}
		}()
	}

	audit.Log(audit.EventTripInvite,
		"trip_id", tripID.String(),
		"invited_by", userID.String(),
		"invited_email", maskEmail(req.Email),
		"role", req.Role,
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"id":         collab.ID.String(),
		"email":      collab.Email,
		"role":       collab.Role,
		"invited_at": collab.InvitedAt,
	})
}

// HandleAcceptInvite handles POST /api/trips/accept-invite — accept a collaboration invite.
func (h *CollaborateHandler) HandleAcceptInvite(w http.ResponseWriter, r *http.Request) {
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
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" {
		http.Error(w, "token is required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Look up the invite
	collab, err := h.queries.GetCollaboratorByToken(ctx, pgtype.Text{String: req.Token, Valid: true})
	if err != nil {
		http.Error(w, "invalid or expired invite token", http.StatusNotFound)
		return
	}

	// Check if already accepted
	if collab.AcceptedAt.Valid {
		http.Error(w, "this invite has already been accepted", http.StatusConflict)
		return
	}

	// Verify the accepting user's email matches the invited email
	acceptingUser, err := h.queries.GetUserByID(ctx, userID)
	if err != nil {
		slog.Error("get user for invite verification failed", "error", err, "user_id", userID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !strings.EqualFold(acceptingUser.Email, collab.Email) {
		http.Error(w, "this invite was sent to a different email address", http.StatusForbidden)
		return
	}

	// Accept the invite
	accepted, err := h.queries.AcceptInvite(ctx, dbgen.AcceptInviteParams{
		InviteToken: pgtype.Text{String: req.Token, Valid: true},
		UserID:      pgtype.UUID{Bytes: userID, Valid: true},
	})
	if err != nil {
		slog.Error("accept invite failed", "error", err, "token", req.Token)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Fetch the trip to return details
	trip, err := h.queries.GetTripByIDOrCollaborator(ctx, dbgen.GetTripByIDOrCollaboratorParams{
		ID:     accepted.TripID,
		UserID: userID,
	})
	if err != nil {
		slog.Error("get trip after accept failed", "error", err, "trip_id", accepted.TripID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	audit.Log(audit.EventTripInviteAccept,
		"trip_id", accepted.TripID.String(),
		"user_id", userID.String(),
		"role", accepted.Role,
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"trip_id":     trip.ID.String(),
		"title":       trip.Title,
		"description": trip.Description.String,
		"status":      trip.Status,
		"role":        accepted.Role,
	})
}

// HandleListCollaborators handles GET /api/trips/{tripId}/collaborators — list collaborators.
func (h *CollaborateHandler) HandleListCollaborators(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := authenticateRESTRequest(r, h.authSvc)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	tripID, err := parseTripIDFromPath(r.URL.Path, "/api/trips/", "/collaborators")
	if err != nil {
		http.Error(w, "invalid trip ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Check access: must be owner or editor
	if !h.hasAccess(ctx, tripID, userID, "editor") {
		http.Error(w, "access denied", http.StatusForbidden)
		return
	}

	collabs, err := h.queries.ListCollaborators(ctx, tripID)
	if err != nil {
		slog.Error("list collaborators failed", "error", err, "trip_id", tripID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	result := make([]map[string]any, len(collabs))
	for i, c := range collabs {
		entry := map[string]any{
			"id":         c.ID.String(),
			"email":      c.Email,
			"role":       c.Role,
			"invited_at": c.InvitedAt,
		}
		if c.UserID.Valid {
			entry["user_id"] = uuid.UUID(c.UserID.Bytes).String()
		}
		if c.AcceptedAt.Valid {
			entry["accepted_at"] = c.AcceptedAt.Time
		}
		result[i] = entry
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"collaborators": result,
	})
}

// HandleRemoveCollaborator handles DELETE /api/trips/{tripId}/collaborators/{email} — remove a collaborator.
func (h *CollaborateHandler) HandleRemoveCollaborator(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := authenticateRESTRequest(r, h.authSvc)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse: /api/trips/{tripId}/collaborators/{email}
	path := r.URL.Path
	const prefix = "/api/trips/"
	const collabSegment = "/collaborators/"

	if !strings.HasPrefix(path, prefix) {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	rest := path[len(prefix):]
	collabIdx := strings.Index(rest, collabSegment)
	if collabIdx < 0 {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	tripIDStr := rest[:collabIdx]
	emailStr := rest[collabIdx+len(collabSegment):]

	tripID, err := uuid.Parse(tripIDStr)
	if err != nil {
		http.Error(w, "invalid trip ID", http.StatusBadRequest)
		return
	}

	if emailStr == "" {
		http.Error(w, "email is required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Only trip owner can remove collaborators
	ownerID, err := h.queries.GetTripOwner(ctx, tripID)
	if err != nil {
		http.Error(w, "trip not found", http.StatusNotFound)
		return
	}
	if ownerID != userID {
		http.Error(w, "only the trip owner can remove collaborators", http.StatusForbidden)
		return
	}

	if err := h.queries.RemoveCollaborator(ctx, dbgen.RemoveCollaboratorParams{
		TripID: tripID,
		Email:  emailStr,
	}); err != nil {
		slog.Error("remove collaborator failed", "error", err, "trip_id", tripID, "email", maskEmail(emailStr))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	audit.Log(audit.EventTripCollabRemove,
		"trip_id", tripID.String(),
		"removed_by", userID.String(),
		"removed_email", maskEmail(emailStr),
	)

	w.WriteHeader(http.StatusNoContent)
}

// hasAccess checks if a user has at least the specified role level on a trip.
// "viewer" means any role, "editor" means editor or owner, "owner" means owner only.
func (h *CollaborateHandler) hasAccess(ctx context.Context, tripID, userID uuid.UUID, minRole string) bool {
	// Check if user is the trip owner
	ownerID, err := h.queries.GetTripOwner(ctx, tripID)
	if err != nil {
		return false
	}
	if ownerID == userID {
		return true
	}

	// Check collaborator access
	collab, err := h.queries.GetCollaboratorAccess(ctx, dbgen.GetCollaboratorAccessParams{
		TripID: tripID,
		UserID: pgtype.UUID{Bytes: userID, Valid: true},
	})
	if err != nil {
		return false
	}
	if !collab.AcceptedAt.Valid {
		return false
	}

	switch minRole {
	case "viewer":
		return true // any accepted collaborator
	case "editor":
		return collab.Role == "editor" || collab.Role == "owner"
	case "owner":
		return collab.Role == "owner"
	default:
		return false
	}
}

// parseTripIDFromPath extracts a UUID from a URL path between a prefix and suffix.
// e.g., parseTripIDFromPath("/api/trips/uuid-here/invite", "/api/trips/", "/invite")
func parseTripIDFromPath(path, prefix, suffix string) (uuid.UUID, error) {
	if !strings.HasPrefix(path, prefix) {
		return uuid.Nil, fmt.Errorf("invalid path")
	}
	rest := path[len(prefix):]
	if suffix != "" {
		idx := strings.Index(rest, suffix)
		if idx < 0 {
			return uuid.Nil, fmt.Errorf("invalid path")
		}
		rest = rest[:idx]
	}
	return uuid.Parse(rest)
}

func generateInviteToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
