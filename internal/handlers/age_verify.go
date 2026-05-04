package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gallowaysoftware/toqui-backend/internal/audit"
	"github.com/gallowaysoftware/toqui-backend/internal/auth"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui-backend/internal/lifecycle"
)

// AgeVerifyHandler handles the POST /auth/verify-age endpoint.
//
// The handler is the single enforcement point for the 18+ age requirement
// declared in toqui-site/terms.astro §2 and privacy.astro §12. It runs
// AFTER login (the route is wrapped in the standard auth middleware) so
// every caller has a real user_id, and it has full DB access to act on
// that user_id when verification fails.
//
// Behaviour matrix:
//
//   - DOB present, age >= 18 → set users.age_verified_at, return 200
//     {"verified": true}.
//   - DOB present, age < 18 → record an under_age_blocks row keyed by
//     SHA-256 of the user's email, hard-delete the user via
//     lifecycle.Service.DeleteUser (synchronous, full fan-out — Postgres
//     CASCADE + Firestore chat purge), audit-log the refusal, return 403
//     with body {"error": "under_age", "message": "..."}. The frontend
//     uses the typed error code to drive a "your account has been
//     deleted" confirmation screen instead of leaving the user staring
//     at a generic 403.
//   - DOB malformed / future / >150 years old → 400 (no destructive
//     action; treat as user input error, not a refusal).
//
// Why hard-delete on under-18 instead of just refusing future RPCs:
// (a) the user was created by OAuth before we knew their age — keeping
// the row breaks the "we don't keep data we don't need" promise; (b) the
// deletion plus the under_age_blocks row is the cleanest way to refuse
// re-creation without maintaining a "blocked users" view of the live
// users table; (c) under_age_blocks survives the deletion (no FK to
// users) so the forensic record persists.
type AgeVerifyHandler struct {
	authSvc      *auth.Service
	queries      *dbgen.Queries
	lifecycleSvc *lifecycle.Service
}

// NewAgeVerifyHandler creates a new AgeVerifyHandler.
//
// `lifecycleSvc` is required: it owns the synchronous DeleteUser path
// used when an under-18 caller is refused. Wiring this dependency at
// construction time (rather than via a callback) keeps the destructive
// action under the same fan-out semantics as the user-initiated
// DeleteAccount RPC.
func NewAgeVerifyHandler(authSvc *auth.Service, queries *dbgen.Queries, lifecycleSvc *lifecycle.Service) *AgeVerifyHandler {
	return &AgeVerifyHandler{authSvc: authSvc, queries: queries, lifecycleSvc: lifecycleSvc}
}

type verifyAgeRequest struct {
	DateOfBirth string `json:"date_of_birth"` // format: "2000-01-15"
}

// verifyAgeUnderAgeResponse is the JSON body returned with a 403 when
// the caller is under 18. The `error` field is a stable enum the
// frontend matches on; treat its value as part of the public API.
type verifyAgeUnderAgeResponse struct {
	Error   string `json:"error"`   // "under_age"
	Message string `json:"message"` // user-visible explanation
}

const errUnderAge = "under_age"

// HandleVerifyAge validates the user's date of birth and records age verification.
func (h *AgeVerifyHandler) HandleVerifyAge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := authenticateRESTRequest(r, h.authSvc)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB
	var req verifyAgeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.DateOfBirth == "" {
		http.Error(w, "date_of_birth is required", http.StatusBadRequest)
		return
	}

	dob, err := time.Parse("2006-01-02", req.DateOfBirth)
	if err != nil {
		http.Error(w, "invalid date_of_birth format (expected YYYY-MM-DD)", http.StatusBadRequest)
		return
	}

	// Reject obviously-bogus inputs BEFORE the under-18 branch so a typo'd
	// date doesn't trigger account deletion. A future-dated DOB or one
	// implying age > 150 is "user input error", not "underage refusal".
	now := time.Now()
	if dob.After(now) {
		http.Error(w, "invalid date of birth", http.StatusBadRequest)
		return
	}
	age := computeAge(dob, now)
	if age > 150 {
		http.Error(w, "invalid date of birth", http.StatusBadRequest)
		return
	}

	if age < 18 {
		h.handleUnderAge(r.Context(), w, userID)
		return
	}

	if err := h.queries.SetAgeVerified(r.Context(), userID); err != nil {
		slog.Error("failed to set age verification", "user_id", userID.String(), "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	slog.Info("age verification succeeded", "user_id", userID.String())

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"verified":true}`))
}

// handleUnderAge is the destructive path: record the refusal in
// under_age_blocks (so future OAuth attempts with the same email are
// rejected before we re-create a user row), then hard-delete the user
// via the lifecycle service, then return 403 with a typed error body so
// the frontend can show a "your account has been deleted" confirmation.
//
// The order matters: we record the block FIRST. If lifecycle.DeleteUser
// fails partway through, we still want the email on the block list so
// the user can't simply refresh and try again with the same OAuth
// identity. The block record is the durable refusal; the deletion is
// the (best-effort but highly reliable) data cleanup that pairs with it.
//
// Known concurrency window. If the same authenticated user submits two
// verify-age requests simultaneously (double-click, retry-on-network-blip),
// both requests can observe the user as "exists" via GetUserByID before
// either deletion lands, leading to double audit events for the same
// user_id and a parallel Firestore purge. Postgres CASCADE handles the
// duplicate row delete idempotently and the under_age_blocks insert is
// ON CONFLICT DO NOTHING, so the *outcome* is correct, but compliance
// reports may double-count the refusal. A SELECT FOR UPDATE on the
// users row would close this — filed as a follow-up rather than a
// blocker since the failure mode is "two log lines instead of one"
// not "wrong policy decision". (W1 from the PR #420 adversarial review.)
//
// Known UX hazard. The brief explicitly accepts that a typo'd birth year
// (e.g. user enters 2008 meaning 1988) will trigger account deletion.
// The frontend's deletion-confirmation screen is the only safeguard —
// there's no client-side "are you sure?" because that would require us
// to surface "you typed an under-18 DOB" client-side, which softens
// the policy boundary. Operators reviewing under_age_blocks rows
// should expect occasional false positives.
func (h *AgeVerifyHandler) handleUnderAge(ctx context.Context, w http.ResponseWriter, userID uuid.UUID) {
	// Look up the user so we can hash their email + capture which OAuth
	// provider they came in on for the audit record. If the user is gone
	// already (deleted by another path mid-request), short-circuit to
	// the 403 response — the block-list write is still desirable but we
	// can't compute the email hash without the row, and the data is
	// already gone so the deletion goal is satisfied.
	user, err := h.queries.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			slog.Warn("under-age verification: user already deleted", "user_id", userID.String())
			respondUnderAge(w)
			return
		}
		slog.Error("failed to load user for under-age handling", "user_id", userID.String(), "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	emailHash := sha256OfEmail(user.Email)
	provider := oauthProviderForUser(user)

	// Best-effort block record. Failure here is logged but not fatal —
	// the under-18 user must be deleted regardless. The block record is
	// an anti-evasion convenience, not a correctness requirement: the
	// deletion itself enforces the policy.
	if blockErr := h.queries.RecordUnderAgeBlock(ctx, dbgen.RecordUnderAgeBlockParams{
		EmailSha256:   emailHash,
		OauthProvider: provider,
	}); blockErr != nil {
		slog.Error("failed to record under_age_blocks row",
			"user_id", userID.String(),
			"error", blockErr,
		)
	}

	// Synchronous hard delete. Returns only after Postgres CASCADE +
	// Firestore chat purge complete. If this fails we surface a 500
	// rather than the under_age 403 — leaving the user logged in but
	// with their account half-deleted is the worst outcome, so we'd
	// rather they retry than be in a half-state. The block row is
	// already in place so a retry can't re-create the account.
	if delErr := h.lifecycleSvc.DeleteUser(ctx, userID); delErr != nil {
		slog.Error("failed to hard-delete under-age user", "user_id", userID.String(), "error", delErr)
		http.Error(w, "internal error during account cleanup, please try again", http.StatusInternalServerError)
		return
	}

	// Two audit events fire here, by design (W2 from the PR #420
	// adversarial review). The deletion is recorded as
	// auth.account_delete (with reason=under_age) so it appears in the
	// general account-deletion stream alongside user-initiated deletions.
	// The refusal is ALSO recorded as auth.login_denied.under_age so
	// compliance reports that filter for "every under-age refusal we've
	// ever made" can use a single event name across both this path and
	// the OAuth re-attempt path in under_age_block_check.go.
	audit.Log(audit.EventAccountDelete,
		"user_id", userID.String(),
		"reason", "under_age",
		"oauth_provider", provider,
	)
	audit.Log(audit.EventLoginDeniedUnderAge,
		"user_id", userID.String(),
		"oauth_provider", provider,
		"path", "verify_age",
	)
	slog.Info("age verification failed: under 18 — account deleted",
		"user_id", userID.String(),
		"oauth_provider", provider,
	)

	respondUnderAge(w)
}

func respondUnderAge(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(verifyAgeUnderAgeResponse{
		Error: errUnderAge,
		Message: "Toqui is for travelers 18 and over. Your account has been deleted in line with our terms — " +
			"no chat, trip, or booking data has been retained.",
	})
}

// sha256OfEmail returns the SHA-256 hex digest of the lowercased,
// trimmed email. The same normalisation runs on the OAuth login
// pre-check (PR 2 of this stack) so a refused email recognises the
// same person regardless of casing variation across providers.
func sha256OfEmail(email string) string {
	normalised := strings.ToLower(strings.TrimSpace(email))
	sum := sha256.Sum256([]byte(normalised))
	return hex.EncodeToString(sum[:])
}

// oauthProviderForUser inspects which OAuth identity column is populated.
// Returns one of "google", "facebook", "apple", or "unknown". Used as
// the provider tag on the under_age_blocks row and the audit event.
func oauthProviderForUser(u dbgen.User) string {
	switch {
	case u.GoogleID.Valid && u.GoogleID.String != "":
		return "google"
	case u.FacebookID.Valid && u.FacebookID.String != "":
		return "facebook"
	case u.AppleSub.Valid && u.AppleSub.String != "":
		return "apple"
	default:
		return "unknown"
	}
}

// computeAge returns the integer age in completed years from dob to now.
// Pulled out for direct unit testing — the inline version with
// `now.YearDay() < dob.YearDay()` was correct but easy to mis-read on
// leap years, so this version uses month/day comparison directly.
func computeAge(dob, now time.Time) int {
	age := now.Year() - dob.Year()
	if now.Month() < dob.Month() || (now.Month() == dob.Month() && now.Day() < dob.Day()) {
		age--
	}
	return age
}
