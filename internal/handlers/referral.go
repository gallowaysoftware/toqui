package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gallowaysoftware/toqui-backend/internal/audit"
	"github.com/gallowaysoftware/toqui-backend/internal/auth"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
)

// ReferralHandler handles referral code endpoints.
type ReferralHandler struct {
	authSvc *auth.Service
	queries *dbgen.Queries
	appURL  string
}

// NewReferralHandler creates a new ReferralHandler.
func NewReferralHandler(authSvc *auth.Service, pool *pgxpool.Pool, appURL string) *ReferralHandler {
	return &ReferralHandler{
		authSvc: authSvc,
		queries: dbgen.New(pool),
		appURL:  appURL,
	}
}

// HandleGetReferralCode handles GET /api/referral — get or create user's referral code.
func (h *ReferralHandler) HandleGetReferralCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := authenticateRESTRequest(r, h.authSvc)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	ctx := r.Context()

	// Try to get existing referral code
	ref, err := h.queries.GetReferralByReferrer(ctx, userID)
	if err != nil {
		// No existing code — create one
		code := generateReferralCode()
		ref, err = h.queries.CreateReferral(ctx, dbgen.CreateReferralParams{
			ReferrerID: userID,
			Code:       code,
		})
		if err != nil {
			slog.Error("create referral code failed", "error", err, "user_id", userID)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}

	successful, _ := h.queries.CountSuccessfulReferrals(ctx, userID)
	rewards, _ := h.queries.CountRewardsEarned(ctx, userID)

	// Check if this user was referred by someone and whether they received a reward.
	var refereeRewardGranted bool
	if refByOther, err := h.queries.GetReferralByReferee(ctx, pgtype.UUID{Bytes: userID, Valid: true}); err == nil {
		refereeRewardGranted = refByOther.RefereeRewardGranted
	}

	w.Header().Set("Content-Type", "application/json")
	resp := map[string]any{
		"code":                    ref.Code,
		"link":                    h.appURL + "/?ref=" + ref.Code,
		"successful_referrals":    successful,
		"rewards_earned":          rewards,
		"referrer_reward_granted": rewards > 0,
		"referee_reward_granted":  refereeRewardGranted,
	}
	json.NewEncoder(w).Encode(resp)
}

// HandleRedeemReferral handles POST /api/referral/redeem — redeem a referral code.
func (h *ReferralHandler) HandleRedeemReferral(w http.ResponseWriter, r *http.Request) {
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
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Code == "" {
		http.Error(w, "code is required", http.StatusBadRequest)
		return
	}
	req.Code = strings.TrimSpace(strings.ToUpper(req.Code))

	ctx := r.Context()

	// Look up the referral
	ref, err := h.queries.GetReferralByCode(ctx, req.Code)
	if err != nil {
		http.Error(w, "invalid referral code", http.StatusNotFound)
		return
	}

	// Prevent self-referral
	if ref.ReferrerID == userID {
		http.Error(w, "cannot redeem your own referral code", http.StatusBadRequest)
		return
	}

	// Check if already redeemed
	if ref.RefereeID.Valid {
		http.Error(w, "this referral code has already been redeemed", http.StatusConflict)
		return
	}

	// Redeem
	if err := h.queries.RedeemReferral(ctx, dbgen.RedeemReferralParams{
		RefereeID: pgtype.UUID{Bytes: userID, Valid: true},
		Code:      req.Code,
	}); err != nil {
		slog.Error("redeem referral failed", "error", err, "user_id", userID, "code", req.Code)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Mark referral rewards as granted for both parties.
	if err := h.queries.GrantReferrerReward(ctx, ref.ID); err != nil {
		slog.Error("grant referrer reward flag failed", "error", err, "referral_id", ref.ID)
	}
	if err := h.queries.GrantRefereeReward(ctx, ref.ID); err != nil {
		slog.Error("grant referee reward flag failed", "error", err, "referral_id", ref.ID)
	}

	// Attempt to unlock each party's most recent trip. If either has no trip
	// yet, the boolean flag is persisted and the unlock can be applied later
	// (e.g. when they create their first trip via HasPendingReferralCredit).
	if _, err := h.queries.GrantReferralTripUnlock(ctx, ref.ReferrerID); err != nil {
		slog.Info("referrer has no eligible trip to unlock yet", "referrer_id", ref.ReferrerID)
	}
	if _, err := h.queries.GrantReferralTripUnlock(ctx, userID); err != nil {
		slog.Info("referee has no eligible trip to unlock yet", "referee_id", userID)
	}

	audit.Log(audit.EventReferralRedeem,
		"referee_id", userID.String(),
		"referrer_id", ref.ReferrerID.String(),
		"code", req.Code,
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "redeemed",
		"message": "Both you and your friend get a free Trip Pro unlock!",
	})
}

func generateReferralCode() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "TOQUI000"
	}
	return strings.ToUpper(hex.EncodeToString(b))
}
