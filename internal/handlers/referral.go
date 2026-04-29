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

	"github.com/gallowaysoftware/toqui-backend/internal/analytics"
	"github.com/gallowaysoftware/toqui-backend/internal/audit"
	"github.com/gallowaysoftware/toqui-backend/internal/auth"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
)

// ReferralHandler handles referral code endpoints.
type ReferralHandler struct {
	authSvc         *auth.Service
	queries         *dbgen.Queries
	appURL          string
	maxRewards      int // maximum referral trip unlocks a referrer can earn
	analyticsClient *analytics.Client
}

// NewReferralHandler creates a new ReferralHandler.
func NewReferralHandler(authSvc *auth.Service, pool *pgxpool.Pool, appURL string, maxRewards int) *ReferralHandler {
	return &ReferralHandler{
		authSvc:    authSvc,
		queries:    dbgen.New(pool),
		appURL:     appURL,
		maxRewards: maxRewards,
	}
}

// WithAnalytics attaches a PostHog client so successful redemptions
// fire the `referral_redeemed` funnel event. Optional.
func (h *ReferralHandler) WithAnalytics(client *analytics.Client) *ReferralHandler {
	h.analyticsClient = client
	return h
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
	unlockCount, _ := h.queries.CountReferrerUnlocks(ctx, userID)

	// Check if this user was referred by someone and whether they received a reward.
	var refereeRewardGranted bool
	if refByOther, err := h.queries.GetReferralByReferee(ctx, pgtype.UUID{Bytes: userID, Valid: true}); err == nil {
		refereeRewardGranted = refByOther.RefereeRewardGranted
	}

	rewardsRemaining := h.maxRewards - int(unlockCount)
	if rewardsRemaining < 0 {
		rewardsRemaining = 0
	}

	w.Header().Set("Content-Type", "application/json")
	resp := map[string]any{
		"code":                    ref.Code,
		"link":                    h.appURL + "/?ref=" + ref.Code,
		"successful_referrals":    successful,
		"rewards_earned":          rewards,
		"referrer_reward_granted": rewards > 0,
		"referee_reward_granted":  refereeRewardGranted,
		"rewards_remaining":       rewardsRemaining,
		"max_rewards":             h.maxRewards,
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

	// Check if referrer has hit the reward cap before granting their reward.
	referrerUnlocks, _ := h.queries.CountReferrerUnlocks(ctx, ref.ReferrerID)
	referrerCapped := int(referrerUnlocks) >= h.maxRewards

	// Always grant the referee reward.
	if err := h.queries.GrantRefereeReward(ctx, ref.ID); err != nil {
		slog.Error("grant referee reward flag failed", "error", err, "referral_id", ref.ID)
	}

	// Only grant the referrer reward if they haven't hit the cap.
	if !referrerCapped {
		if err := h.queries.GrantReferrerReward(ctx, ref.ID); err != nil {
			slog.Error("grant referrer reward flag failed", "error", err, "referral_id", ref.ID)
		}
		if _, err := h.queries.GrantReferralTripUnlock(ctx, ref.ReferrerID); err != nil {
			slog.Info("referrer has no eligible trip to unlock yet", "referrer_id", ref.ReferrerID)
		}
	} else {
		slog.Info("referrer has reached reward cap, skipping unlock",
			"referrer_id", ref.ReferrerID, "unlocks", referrerUnlocks, "max", h.maxRewards)
	}

	// Attempt to unlock the referee's most recent trip. If they have no trip
	// yet, the boolean flag is persisted and the unlock can be applied later
	// (e.g. when they create their first trip via HasPendingReferralCredit).
	if _, err := h.queries.GrantReferralTripUnlock(ctx, userID); err != nil {
		slog.Info("referee has no eligible trip to unlock yet", "referee_id", userID)
	}

	audit.Log(audit.EventReferralRedeem,
		"referee_id", userID.String(),
		"referrer_id", ref.ReferrerID.String(),
		"code", req.Code,
	)

	// Funnel event — distinguishes referral-driven signups from organic.
	// Tracked under the REFEREE's user ID (the one who just redeemed).
	// `referrer_capped` lets us see when the cap is biting growth.
	// We deliberately do NOT include the literal referral code here — codes
	// are pseudo-PII (they identify the referrer) and shouldn't ride into
	// PostHog event properties (CLAUDE.md "Privacy" — pseudonymize).
	if h.analyticsClient != nil {
		h.analyticsClient.Track(userID.String(), "referral_redeemed", map[string]any{
			"referrer_capped": referrerCapped,
		})
	}

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
