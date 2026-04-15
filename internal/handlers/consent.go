package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gallowaysoftware/toqui-backend/internal/auth"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui-backend/internal/ratelimit"
)

// validConsentTypes defines the allowed consent type values.
var validConsentTypes = map[string]bool{
	"terms":          true,
	"privacy_policy": true,
	"analytics":      true,
}

// ConsentHandler handles privacy consent management endpoints.
type ConsentHandler struct {
	authSvc *auth.Service
	queries *dbgen.Queries
}

// NewConsentHandler creates a new ConsentHandler.
func NewConsentHandler(authSvc *auth.Service, pool *pgxpool.Pool) *ConsentHandler {
	return &ConsentHandler{
		authSvc: authSvc,
		queries: dbgen.New(pool),
	}
}

// consentResponse is the JSON response shape for a single consent record.
type consentResponse struct {
	ID          uuid.UUID  `json:"id"`
	ConsentType string     `json:"consent_type"`
	GrantedAt   time.Time  `json:"granted_at"`
	CreatedAt   time.Time  `json:"created_at"`
	WithdrawnAt *time.Time `json:"withdrawn_at,omitempty"`
}

// HandleGetConsents handles GET /api/privacy/consents — returns active consents.
func (h *ConsentHandler) HandleGetConsents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := authenticateRESTRequest(r, h.authSvc)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	consents, err := h.queries.GetActiveConsents(r.Context(), userID)
	if err != nil {
		slog.Error("get active consents failed", "error", err, "user_id", userID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	resp := make([]consentResponse, 0, len(consents))
	for _, c := range consents {
		resp = append(resp, consentResponse{
			ID:          c.ID,
			ConsentType: c.ConsentType,
			GrantedAt:   c.GrantedAt,
			CreatedAt:   c.CreatedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("failed to encode consents response", "error", err)
	}
}

// HandleRecordConsent handles POST /api/privacy/consents — records a new consent.
func (h *ConsentHandler) HandleRecordConsent(w http.ResponseWriter, r *http.Request) {
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
		ConsentType string `json:"consent_type"`
		Granted     bool   `json:"granted"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.ConsentType == "" {
		http.Error(w, "consent_type is required", http.StatusBadRequest)
		return
	}

	if !validConsentTypes[req.ConsentType] {
		http.Error(w, "invalid consent_type", http.StatusBadRequest)
		return
	}

	if !req.Granted {
		http.Error(w, "granted must be true to record consent; use DELETE to withdraw", http.StatusBadRequest)
		return
	}

	ip := ratelimit.ExtractClientIP(r)
	ua := r.Header.Get("User-Agent")

	consent, err := h.queries.RecordConsent(r.Context(), dbgen.RecordConsentParams{
		UserID:      userID,
		ConsentType: req.ConsentType,
		IpAddress:   pgtype.Text{String: ip, Valid: ip != "" && ip != "unknown"},
		UserAgent:   pgtype.Text{String: ua, Valid: ua != ""},
	})
	if err != nil {
		slog.Error("record consent failed", "error", err, "user_id", userID, "consent_type", req.ConsentType)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	slog.Info("consent recorded",
		"user_id", userID,
		"consent_type", req.ConsentType,
		"consent_id", consent.ID,
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(consentResponse{
		ID:          consent.ID,
		ConsentType: consent.ConsentType,
		GrantedAt:   consent.GrantedAt,
		CreatedAt:   consent.CreatedAt,
	}); err != nil {
		slog.Error("failed to encode consent response", "error", err)
	}
}

// batchConsentRequest is the JSON body for POST /auth/consent.
type batchConsentRequest struct {
	TermsAccepted   bool `json:"terms_accepted"`
	PrivacyAccepted bool `json:"privacy_accepted"`
	MarketingOptIn  bool `json:"marketing_opt_in"`
}

// batchConsentResponse is the JSON response from POST /auth/consent.
type batchConsentResponse struct {
	Recorded []string `json:"recorded"` // list of consent types recorded
}

// HandleBatchConsent handles POST /auth/consent — records multiple consent
// choices at once during first login. The frontend shows a consent modal
// after signup and calls this endpoint with the user's explicit choices.
//
// Required: terms_accepted and privacy_accepted must both be true.
// Optional: marketing_opt_in records an analytics consent if true.
func (h *ConsentHandler) HandleBatchConsent(w http.ResponseWriter, r *http.Request) {
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
	var req batchConsentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if !req.TermsAccepted {
		http.Error(w, "terms_accepted is required", http.StatusBadRequest)
		return
	}
	if !req.PrivacyAccepted {
		http.Error(w, "privacy_accepted is required", http.StatusBadRequest)
		return
	}

	ip := ratelimit.ExtractClientIP(r)
	ua := r.Header.Get("User-Agent")

	ipText := pgtype.Text{String: ip, Valid: ip != "" && ip != "unknown"}
	uaText := pgtype.Text{String: ua, Valid: ua != ""}

	var recorded []string

	// Record terms consent.
	if _, err := h.queries.RecordConsent(r.Context(), dbgen.RecordConsentParams{
		UserID:      userID,
		ConsentType: "terms",
		IpAddress:   ipText,
		UserAgent:   uaText,
	}); err != nil {
		slog.Error("batch consent: record terms failed", "error", err, "user_id", userID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	recorded = append(recorded, "terms")

	// Record privacy_policy consent.
	if _, err := h.queries.RecordConsent(r.Context(), dbgen.RecordConsentParams{
		UserID:      userID,
		ConsentType: "privacy_policy",
		IpAddress:   ipText,
		UserAgent:   uaText,
	}); err != nil {
		slog.Error("batch consent: record privacy_policy failed", "error", err, "user_id", userID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	recorded = append(recorded, "privacy_policy")

	// Optionally record analytics consent (marketing opt-in).
	if req.MarketingOptIn {
		if _, err := h.queries.RecordConsent(r.Context(), dbgen.RecordConsentParams{
			UserID:      userID,
			ConsentType: "analytics",
			IpAddress:   ipText,
			UserAgent:   uaText,
		}); err != nil {
			slog.Error("batch consent: record analytics failed", "error", err, "user_id", userID)
			// Non-fatal: terms + privacy already recorded
		} else {
			recorded = append(recorded, "analytics")
		}
	}

	slog.Info("batch consent recorded",
		"user_id", userID,
		"consents", recorded,
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(batchConsentResponse{
		Recorded: recorded,
	}); err != nil {
		slog.Error("failed to encode batch consent response", "error", err)
	}
}

// HandleWithdrawConsent handles DELETE /api/privacy/consents/{type} — withdraws a consent.
func (h *ConsentHandler) HandleWithdrawConsent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := authenticateRESTRequest(r, h.authSvc)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Extract consent type from path: /api/privacy/consents/{type}
	consentType := strings.TrimPrefix(r.URL.Path, "/api/privacy/consents/")
	if consentType == "" || consentType == r.URL.Path {
		http.Error(w, "consent type is required in path", http.StatusBadRequest)
		return
	}

	if !validConsentTypes[consentType] {
		http.Error(w, "invalid consent_type", http.StatusBadRequest)
		return
	}

	err := h.queries.WithdrawConsent(r.Context(), dbgen.WithdrawConsentParams{
		UserID:      userID,
		ConsentType: consentType,
	})
	if err != nil {
		slog.Error("withdraw consent failed", "error", err, "user_id", userID, "consent_type", consentType)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	slog.Info("consent withdrawn",
		"user_id", userID,
		"consent_type", consentType,
	)

	w.WriteHeader(http.StatusNoContent)
}
