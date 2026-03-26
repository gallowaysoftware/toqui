package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gallowaysoftware/toqui-backend/internal/audit"
	"github.com/gallowaysoftware/toqui-backend/internal/auth"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui-backend/internal/email"
)

// AdminHandler serves internal admin endpoints.
// All endpoints require JWT auth + email in the admin allow-list.
type AdminHandler struct {
	authSvc     *auth.Service
	queries     *dbgen.Queries
	adminEmails []string
	emailSvc    *email.Sender
	appURL      string
}

// NewAdminHandler creates a new AdminHandler.
func NewAdminHandler(authSvc *auth.Service, pool *pgxpool.Pool, adminEmails []string, emailSvc *email.Sender, appURL string) *AdminHandler {
	return &AdminHandler{
		authSvc:     authSvc,
		queries:     dbgen.New(pool),
		adminEmails: adminEmails,
		emailSvc:    emailSvc,
		appURL:      appURL,
	}
}

// authenticateAdmin verifies JWT + checks admin email list.
func (h *AdminHandler) authenticateAdmin(r *http.Request) (uuid.UUID, error) {
	userID, ok := authenticateRESTRequest(r, h.authSvc)
	if !ok {
		return uuid.Nil, errUnauthorized
	}

	user, err := h.queries.GetUserByID(r.Context(), userID)
	if err != nil {
		return uuid.Nil, errUnauthorized
	}

	if !isEmailAllowListed(user.Email, h.adminEmails) {
		slog.Warn("admin access denied", "email", user.Email, "user_id", userID, "admin_list", h.adminEmails)
		return uuid.Nil, errForbidden
	}

	return userID, nil
}

var (
	errUnauthorized = &httpError{code: http.StatusUnauthorized, msg: "unauthorized"}
	errForbidden    = &httpError{code: http.StatusForbidden, msg: "forbidden"}
)

type httpError struct {
	code int
	msg  string
}

func (e *httpError) Error() string { return e.msg }

func writeAdminError(w http.ResponseWriter, err error) {
	var he *httpError
	if errors.As(err, &he) {
		http.Error(w, he.msg, he.code)
		return
	}
	http.Error(w, "internal error", http.StatusInternalServerError)
}

// HandleStats handles GET /admin/stats — system dashboard.
func (h *AdminHandler) HandleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if _, err := h.authenticateAdmin(r); err != nil {
		writeAdminError(w, err)
		return
	}

	ctx := r.Context()
	userCount, _ := h.queries.CountUsers(ctx)
	waitlistCount, _ := h.queries.CountWaitlist(ctx)
	activeTrips, _ := h.queries.CountActiveTrips(ctx)
	dailyMessages, _ := h.queries.CountDailyMessages(ctx)
	proInterest, _ := h.queries.CountProInterest(ctx)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"total_users":    userCount,
		"waitlist_count": waitlistCount,
		"active_trips":   activeTrips,
		"daily_messages": dailyMessages,
		"pro_interest":   proInterest,
	})
}

// HandleListUsers handles GET /admin/users — paginated user list.
func (h *AdminHandler) HandleListUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if _, err := h.authenticateAdmin(r); err != nil {
		writeAdminError(w, err)
		return
	}

	pageSize, offset := parsePagination(r, 50, 200)
	query := r.URL.Query().Get("q")

	ctx := r.Context()
	if query != "" {
		users, err := h.queries.SearchUsers(ctx, dbgen.SearchUsersParams{
			Query:      query,
			PageSize:   int32(pageSize),
			PageOffset: int32(offset),
		})
		if err != nil {
			slog.Error("admin search users failed", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"users": users})
		return
	}

	users, err := h.queries.ListUsers(ctx, dbgen.ListUsersParams{
		PageSize:   int32(pageSize),
		PageOffset: int32(offset),
	})
	if err != nil {
		slog.Error("admin list users failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"users": users})
}

// HandleListWaitlist handles GET /admin/waitlist — paginated waitlist.
func (h *AdminHandler) HandleListWaitlist(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if _, err := h.authenticateAdmin(r); err != nil {
		writeAdminError(w, err)
		return
	}

	pageSize, offset := parsePagination(r, 50, 200)

	entries, err := h.queries.ListWaitlist(r.Context(), dbgen.ListWaitlistParams{
		PageSize:   int32(pageSize),
		PageOffset: int32(offset),
	})
	if err != nil {
		slog.Error("admin list waitlist failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	total, _ := h.queries.CountWaitlist(r.Context())

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"entries": entries,
		"total":   total,
	})
}

// HandleGenerateInvite handles POST /admin/invite — generate invite code for a waitlist email.
func (h *AdminHandler) HandleGenerateInvite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	adminID, err := h.authenticateAdmin(r)
	if err != nil {
		writeAdminError(w, err)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" {
		http.Error(w, "email is required", http.StatusBadRequest)
		return
	}

	// Generate random 8-char hex invite code
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	code := strings.ToUpper(hex.EncodeToString(b))

	if err := h.queries.SetWaitlistInviteCode(r.Context(), dbgen.SetWaitlistInviteCodeParams{
		InviteCode: pgtype.Text{String: code, Valid: true},
		Email:      req.Email,
	}); err != nil {
		slog.Error("admin set invite code failed", "error", err, "email", req.Email)
		http.Error(w, "failed to set invite code (is email on waitlist?)", http.StatusBadRequest)
		return
	}

	audit.Log(audit.EventAdminInvite,
		"admin_id", adminID.String(),
		"email", req.Email,
		"invite_code", code,
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"email":       req.Email,
		"invite_code": code,
	})
}

// HandleUnlockTrip handles POST /admin/unlock-trip — manually unlock a trip.
func (h *AdminHandler) HandleUnlockTrip(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	adminID, err := h.authenticateAdmin(r)
	if err != nil {
		writeAdminError(w, err)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req struct {
		UserID string `json:"user_id"`
		TripID string `json:"trip_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		http.Error(w, "invalid user_id", http.StatusBadRequest)
		return
	}
	tripID, err := uuid.Parse(req.TripID)
	if err != nil {
		http.Error(w, "invalid trip_id", http.StatusBadRequest)
		return
	}

	if _, err := h.queries.CreateTripUnlock(r.Context(), dbgen.CreateTripUnlockParams{
		UserID: userID,
		TripID: tripID,
		Source: "admin",
	}); err != nil {
		slog.Error("admin unlock trip failed", "error", err, "user_id", userID, "trip_id", tripID)
		http.Error(w, "failed to unlock trip", http.StatusInternalServerError)
		return
	}

	audit.Log(audit.EventAdminTripUnlock,
		"admin_id", adminID.String(),
		"user_id", userID.String(),
		"trip_id", tripID.String(),
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "unlocked"})
}

// parsePagination extracts page_size and offset from query params with defaults.
func parsePagination(r *http.Request, defaultSize, maxSize int) (int, int) {
	size := defaultSize
	offset := 0
	if s := r.URL.Query().Get("page_size"); s != "" {
		if n := parseInt(s); n > 0 && n <= maxSize {
			size = n
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if n := parseInt(o); n >= 0 {
			offset = n
		}
	}
	return size, offset
}

// HandleSendInvite handles POST /admin/send-invite — creates waitlist entry + invite code + sends email.
// This is the one-step "invite a friend" flow: enter an email, they get an invite.
func (h *AdminHandler) HandleSendInvite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	adminID, err := h.authenticateAdmin(r)
	if err != nil {
		writeAdminError(w, err)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" {
		http.Error(w, "email is required", http.StatusBadRequest)
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	ctx := r.Context()

	// Generate invite code
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	code := strings.ToUpper(hex.EncodeToString(b))

	// Create waitlist entry if not exists (auto-verified since admin is inviting)
	token := hex.EncodeToString(make([]byte, 16))
	rand.Read([]byte(token))
	verifyToken := pgtype.Text{String: token, Valid: true}
	_, addErr := h.queries.AddToWaitlist(ctx, dbgen.AddToWaitlistParams{
		Email:       req.Email,
		VerifyToken: verifyToken,
	})
	if addErr != nil {
		// Already on waitlist — that's fine, continue
		slog.Debug("invite: email already on waitlist", "email", req.Email)
	}

	// Auto-verify the entry (admin-invited users skip email verification)
	_, _ = h.queries.VerifyWaitlistEmail(ctx, verifyToken)

	// Set the invite code
	if err := h.queries.SetWaitlistInviteCode(ctx, dbgen.SetWaitlistInviteCodeParams{
		InviteCode: pgtype.Text{String: code, Valid: true},
		Email:      req.Email,
	}); err != nil {
		slog.Error("admin send invite: set code failed", "error", err, "email", req.Email)
		http.Error(w, "failed to set invite code", http.StatusInternalServerError)
		return
	}

	// Send invite email
	emailSent := false
	if h.emailSvc != nil {
		if sendErr := h.emailSvc.SendInvite(req.Email, code, h.appURL); sendErr != nil {
			slog.Error("admin send invite: email failed", "error", sendErr, "email", req.Email)
		} else {
			emailSent = true
		}
	}

	audit.Log(audit.EventAdminInvite,
		"admin_id", adminID.String(),
		"email", req.Email,
		"invite_code", code,
		"email_sent", emailSent,
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"email":       req.Email,
		"invite_code": code,
		"email_sent":  emailSent,
	})
}

// HandleRevokeInvite handles POST /admin/revoke-invite — revokes an invite code.
func (h *AdminHandler) HandleRevokeInvite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if _, err := h.authenticateAdmin(r); err != nil {
		writeAdminError(w, err)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" {
		http.Error(w, "email is required", http.StatusBadRequest)
		return
	}

	if err := h.queries.RevokeWaitlistInvite(r.Context(), req.Email); err != nil {
		http.Error(w, "failed to revoke invite", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "revoked"})
}

// HandleDeleteWaitlistEntry handles POST /admin/delete-waitlist — removes an entry.
func (h *AdminHandler) HandleDeleteWaitlistEntry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if _, err := h.authenticateAdmin(r); err != nil {
		writeAdminError(w, err)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" {
		http.Error(w, "email is required", http.StatusBadRequest)
		return
	}

	if err := h.queries.DeleteFromWaitlist(r.Context(), req.Email); err != nil {
		http.Error(w, "failed to delete entry", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

func parseInt(s string) int {
	var n int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			return -1
		}
	}
	return n
}
