package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gallowaysoftware/toqui-backend/internal/audit"
	"github.com/gallowaysoftware/toqui-backend/internal/auth"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui-backend/internal/email"
	"github.com/gallowaysoftware/toqui-backend/internal/lifecycle"
)

// AdminHandler serves internal admin endpoints.
// All endpoints require JWT auth + email in the admin allow-list.
type AdminHandler struct {
	authSvc      *auth.Service
	queries      *dbgen.Queries
	adminEmails  []string
	emailSvc     *email.Sender
	appURL       string
	lifecycleSvc *lifecycle.Service
}

// NewAdminHandler creates a new AdminHandler.
func NewAdminHandler(authSvc *auth.Service, pool *pgxpool.Pool, adminEmails []string, emailSvc *email.Sender, appURL string, lifecycleSvc *lifecycle.Service) *AdminHandler {
	return &AdminHandler{
		authSvc:      authSvc,
		queries:      dbgen.New(pool),
		adminEmails:  adminEmails,
		emailSvc:     emailSvc,
		appURL:       appURL,
		lifecycleSvc: lifecycleSvc,
	}
}

// authenticateAdmin verifies JWT + checks the is_admin DB column.
// Falls back to ADMIN_EMAILS for bootstrapping: if the user's email matches
// the config list but is_admin is false, promote them and log the seed event.
func (h *AdminHandler) authenticateAdmin(r *http.Request) (uuid.UUID, error) {
	userID, ok := authenticateRESTRequest(r, h.authSvc)
	if !ok {
		return uuid.Nil, errUnauthorized
	}

	user, err := h.queries.GetUserByID(r.Context(), userID)
	if err != nil {
		return uuid.Nil, errUnauthorized
	}

	// Primary check: database column.
	if user.IsAdmin {
		return userID, nil
	}

	// Fallback/seed: if the user's email is in ADMIN_EMAILS, promote them in
	// the DB so subsequent requests use the column directly. This allows new
	// deployments to bootstrap an initial admin without manual SQL.
	if isEmailAllowListed(user.Email, h.adminEmails) {
		if seedErr := h.queries.SetAdmin(r.Context(), dbgen.SetAdminParams{
			IsAdmin: true,
			UserID:  userID,
		}); seedErr != nil {
			slog.Error("failed to seed admin role from ADMIN_EMAILS", "error", seedErr, "user_id", userID)
		} else {
			slog.Info("admin role seeded from ADMIN_EMAILS config", "email", maskEmail(user.Email), "user_id", userID)
			audit.Log(audit.EventAdminSeedRole,
				"user_id", userID.String(),
				"email", maskEmail(user.Email),
			)
		}
		return userID, nil
	}

	slog.Warn("admin access denied", "email", maskEmail(user.Email), "user_id", userID)
	return uuid.Nil, errForbidden
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
	proInterest, _ := h.queries.CountProInterest(ctx)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"total_users":    userCount,
		"waitlist_count": waitlistCount,
		"active_trips":   activeTrips,
		"pro_interest":   proInterest,
	})
}

// HandleListFeedback handles GET /admin/feedback — paginated user feedback.
func (h *AdminHandler) HandleListFeedback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if _, err := h.authenticateAdmin(r); err != nil {
		writeAdminError(w, err)
		return
	}

	pageSize, offset := parsePagination(r, 50, 200)
	feedbackType := r.URL.Query().Get("type")

	ctx := r.Context()
	var entries any
	var err error

	if feedbackType != "" {
		entries, err = h.queries.ListFeedbackByType(ctx, dbgen.ListFeedbackByTypeParams{
			Type:       feedbackType,
			PageSize:   int32(pageSize),
			PageOffset: int32(offset),
		})
	} else {
		entries, err = h.queries.ListFeedback(ctx, dbgen.ListFeedbackParams{
			PageSize:   int32(pageSize),
			PageOffset: int32(offset),
		})
	}
	if err != nil {
		slog.Error("admin list feedback failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	total, _ := h.queries.CountFeedback(ctx)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"feedback": entries,
		"total":    total,
	})
}

// HandleMetrics handles GET /admin/metrics — detailed business KPIs.
func (h *AdminHandler) HandleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if _, err := h.authenticateAdmin(r); err != nil {
		writeAdminError(w, err)
		return
	}

	ctx := r.Context()
	totalUsers, _ := h.queries.CountTotalUsers(ctx)
	active7d, _ := h.queries.CountActiveUsersLast7Days(ctx)
	proUsers, _ := h.queries.CountProUsers(ctx)
	signupsToday, _ := h.queries.CountSignupsToday(ctx)
	signups7d, _ := h.queries.CountSignupsLast7Days(ctx)
	totalTrips, _ := h.queries.CountTotalTrips(ctx)
	activeTrips, _ := h.queries.CountActiveTrips(ctx)
	messagesToday, _ := h.queries.CountMessagesToday(ctx)
	purchases, _ := h.queries.CountTripProPurchases(ctx)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"users": map[string]any{
			"total":         totalUsers,
			"active_7d":     active7d,
			"pro":           proUsers,
			"signups_today": signupsToday,
			"signups_7d":    signups7d,
		},
		"trips": map[string]any{
			"total":  totalTrips,
			"active": activeTrips,
		},
		"engagement": map[string]any{
			"messages_today": messagesToday,
		},
		"revenue": map[string]any{
			"trip_pro_purchases": purchases,
		},
		"generated_at": time.Now().UTC(),
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
		slog.Error("admin set invite code failed", "error", err, "email", maskEmail(req.Email))
		http.Error(w, "failed to set invite code (is email on waitlist?)", http.StatusBadRequest)
		return
	}

	audit.Log(audit.EventAdminInvite,
		"admin_id", adminID.String(),
		"email", maskEmail(req.Email),
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

	// Cross-validate: the (user_id, trip_id) pair must correspond to
	// a real owned trip. Admin-gated so a compromised admin is the
	// only threat, but a typo or stale ID would otherwise silently
	// create a ghost trip_unlocks row referencing a trip the user
	// doesn't own — later webhook reconciliation could confuse this
	// with a legitimate purchase (#361 P3). GetTripByID's WHERE
	// filters on user_id, so ErrNoRows here means the pair doesn't
	// match.
	if _, err := h.queries.GetTripByID(r.Context(), dbgen.GetTripByIDParams{
		ID:     tripID,
		UserID: userID,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "trip not found for this user", http.StatusNotFound)
			return
		}
		slog.Error("admin unlock trip: validate ownership failed", "error", err, "user_id", userID, "trip_id", tripID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Trip unlock is a monetization concept that no longer exists; this
	// endpoint is retained as a no-op for compatibility.
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
		slog.Debug("invite: email already on waitlist", "email", maskEmail(req.Email))
	}

	// Auto-verify the entry (admin-invited users skip email verification)
	_, _ = h.queries.VerifyWaitlistEmail(ctx, verifyToken)

	// Set the invite code
	if err := h.queries.SetWaitlistInviteCode(ctx, dbgen.SetWaitlistInviteCodeParams{
		InviteCode: pgtype.Text{String: code, Valid: true},
		Email:      req.Email,
	}); err != nil {
		slog.Error("admin send invite: set code failed", "error", err, "email", maskEmail(req.Email))
		http.Error(w, "failed to set invite code", http.StatusInternalServerError)
		return
	}

	// Send invite email
	emailSent := false
	if h.emailSvc != nil {
		if sendErr := h.emailSvc.SendInvite(req.Email, code, h.appURL); sendErr != nil {
			slog.Error("admin send invite: email failed", "error", sendErr, "email", maskEmail(req.Email))
		} else {
			emailSent = true
		}
	}

	audit.Log(audit.EventAdminInvite,
		"admin_id", adminID.String(),
		"email", maskEmail(req.Email),
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

// HandleGrantPro handles POST /admin/grant-pro — sets a user's subscription tier.
func (h *AdminHandler) HandleGrantPro(w http.ResponseWriter, r *http.Request) {
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
		Tier  string `json:"tier"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" {
		http.Error(w, "email is required", http.StatusBadRequest)
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	if req.Tier != "free" && req.Tier != "pro" {
		http.Error(w, "tier must be 'free' or 'pro'", http.StatusBadRequest)
		return
	}

	if err := h.queries.SetUserSubscriptionTier(r.Context(), dbgen.SetUserSubscriptionTierParams{
		SubscriptionTier: req.Tier,
		Email:            req.Email,
	}); err != nil {
		slog.Error("admin grant pro failed", "error", err, "email", maskEmail(req.Email))
		http.Error(w, "failed to update tier (is the user registered?)", http.StatusBadRequest)
		return
	}

	audit.Log(audit.EventAdminGrantPro,
		"admin_id", adminID.String(),
		"email", maskEmail(req.Email),
		"tier", req.Tier,
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"email":  req.Email,
		"tier":   req.Tier,
		"status": "updated",
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

// HandleDeleteUser handles POST /admin/delete-user — permanently deletes a user and all their data.
func (h *AdminHandler) HandleDeleteUser(w http.ResponseWriter, r *http.Request) {
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

	user, err := h.queries.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		http.Error(w, "user not found", http.StatusBadRequest)
		return
	}

	if err := h.lifecycleSvc.DeleteUser(r.Context(), user.ID); err != nil {
		slog.Error("admin delete user failed", "error", err, "email", maskEmail(req.Email))
		http.Error(w, "failed to delete user", http.StatusInternalServerError)
		return
	}

	audit.Log(audit.EventAdminDeleteUser,
		"admin_id", adminID.String(),
		"email", maskEmail(req.Email),
		"user_id", user.ID.String(),
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"email":  req.Email,
		"status": "deleted",
	})
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

// HandleAICosts handles GET /admin/ai-costs — AI cost dashboard.
// Detailed AI cost tracking has been removed along with monetization tables.
// Returns a zeroed payload for backward compatibility with admin clients.
func (h *AdminHandler) HandleAICosts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if _, err := h.authenticateAdmin(r); err != nil {
		writeAdminError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"daily_cost":   0.0,
		"weekly_cost":  0.0,
		"monthly_cost": 0.0,
		"cost_by_tier": []map[string]any{},
		"top_users":    []map[string]any{},
		"by_model":     []map[string]any{},
	})
}

// HandleRevenue handles GET /admin/revenue — revenue dashboard.
// Monetization has been removed; this endpoint returns a zeroed payload for
// backward compatibility with admin clients.
func (h *AdminHandler) HandleRevenue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if _, err := h.authenticateAdmin(r); err != nil {
		writeAdminError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"mrr":              0.0,
		"trip_pro_monthly": 0.0,
		"total_revenue":    0.0,
	})
}

// HandleSetAdmin handles POST /admin/set-admin — grant or revoke admin role.
func (h *AdminHandler) HandleSetAdmin(w http.ResponseWriter, r *http.Request) {
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
		Email   string `json:"email"`
		IsAdmin bool   `json:"is_admin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" {
		http.Error(w, "email is required", http.StatusBadRequest)
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	ctx := r.Context()
	target, err := h.queries.GetUserByEmail(ctx, req.Email)
	if err != nil {
		http.Error(w, "user not found", http.StatusBadRequest)
		return
	}

	if err := h.queries.SetAdmin(ctx, dbgen.SetAdminParams{
		IsAdmin: req.IsAdmin,
		UserID:  target.ID,
	}); err != nil {
		slog.Error("admin set-admin failed", "error", err, "email", maskEmail(req.Email))
		http.Error(w, "failed to update admin role", http.StatusInternalServerError)
		return
	}

	audit.Log(audit.EventAdminSetRole,
		"admin_id", adminID.String(),
		"target_user_id", target.ID.String(),
		"email", maskEmail(req.Email),
		"is_admin", req.IsAdmin,
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"email":    req.Email,
		"is_admin": req.IsAdmin,
		"status":   "updated",
	})
}

// HandleRetention handles GET /admin/retention?since=2026-01-01 — weekly cohort retention data.
func (h *AdminHandler) HandleRetention(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if _, err := h.authenticateAdmin(r); err != nil {
		writeAdminError(w, err)
		return
	}

	since := parseSinceParam(r, 90)

	cohorts, err := h.queries.GetRetentionCohorts(r.Context(), since)
	if err != nil {
		slog.Error("admin retention query failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Compute retention rates alongside raw counts.
	type cohortRow struct {
		CohortWeek      string  `json:"cohort_week"`
		CohortSize      int64   `json:"cohort_size"`
		D1Active        int64   `json:"d1_active"`
		D7Active        int64   `json:"d7_active"`
		D30Active       int64   `json:"d30_active"`
		D1RetentionPct  float64 `json:"d1_retention_pct"`
		D7RetentionPct  float64 `json:"d7_retention_pct"`
		D30RetentionPct float64 `json:"d30_retention_pct"`
	}

	rows := make([]cohortRow, 0, len(cohorts))
	for _, c := range cohorts {
		row := cohortRow{
			CohortSize: c.CohortSize,
			D1Active:   c.D1Active,
			D7Active:   c.D7Active,
			D30Active:  c.D30Active,
		}
		if c.CohortWeek != nil {
			row.CohortWeek = c.CohortWeek.Format("2006-01-02")
		}
		if c.CohortSize > 0 {
			row.D1RetentionPct = float64(c.D1Active) / float64(c.CohortSize) * 100
			row.D7RetentionPct = float64(c.D7Active) / float64(c.CohortSize) * 100
			row.D30RetentionPct = float64(c.D30Active) / float64(c.CohortSize) * 100
		}
		rows = append(rows, row)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"cohorts":      rows,
		"since":        since.Format("2006-01-02"),
		"generated_at": time.Now().UTC(),
	})
}

// HandleFunnel handles GET /admin/funnel?days=30 — conversion funnel metrics.
func (h *AdminHandler) HandleFunnel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if _, err := h.authenticateAdmin(r); err != nil {
		writeAdminError(w, err)
		return
	}

	days := 30
	if d := r.URL.Query().Get("days"); d != "" {
		if n := parseInt(d); n > 0 && n <= 365 {
			days = n
		}
	}

	since := time.Now().UTC().AddDate(0, 0, -days)

	funnel, err := h.queries.GetFunnelMetrics(r.Context(), since)
	if err != nil {
		slog.Error("admin funnel query failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Compute conversion rates relative to total signups.
	var activatedPct, engagedPct, paidTripProPct, paidSubPct float64
	if funnel.TotalSignups > 0 {
		activatedPct = float64(funnel.Activated) / float64(funnel.TotalSignups) * 100
		engagedPct = float64(funnel.Engaged) / float64(funnel.TotalSignups) * 100
		paidTripProPct = float64(funnel.PaidTripPro) / float64(funnel.TotalSignups) * 100
		paidSubPct = float64(funnel.PaidSubscription) / float64(funnel.TotalSignups) * 100
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"period_days":       days,
		"since":             since.Format("2006-01-02"),
		"total_signups":     funnel.TotalSignups,
		"activated":         funnel.Activated,
		"engaged":           funnel.Engaged,
		"paid_trip_pro":     funnel.PaidTripPro,
		"paid_subscription": funnel.PaidSubscription,
		"conversion_rates": map[string]float64{
			"activated_pct":         activatedPct,
			"engaged_pct":           engagedPct,
			"paid_trip_pro_pct":     paidTripProPct,
			"paid_subscription_pct": paidSubPct,
		},
		"generated_at": time.Now().UTC(),
	})
}

// parseSinceParam parses the "since" query parameter as a date (YYYY-MM-DD),
// falling back to defaultDaysAgo days before now.
func parseSinceParam(r *http.Request, defaultDaysAgo int) time.Time {
	if s := r.URL.Query().Get("since"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			return t
		}
	}
	return time.Now().UTC().AddDate(0, 0, -defaultDaysAgo)
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
