package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stripe/stripe-go/v82"

	"github.com/gallowaysoftware/toqui-backend/internal/analytics"
	"github.com/gallowaysoftware/toqui-backend/internal/auth"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui-backend/internal/ratelimit"
	"github.com/gallowaysoftware/toqui-backend/internal/subscription"
	"github.com/gallowaysoftware/toqui-backend/internal/tier"
)

// SubscriptionHandler handles Stripe subscription REST endpoints.
type SubscriptionHandler struct {
	subSvc          *subscription.Service
	authSvc         *auth.Service
	queries         *dbgen.Queries
	webhookSecret   string
	checkoutLimiter *ratelimit.RESTLimiter
	analyticsClient *analytics.Client
}

// NewSubscriptionHandler creates a new SubscriptionHandler. If the subscription
// service is disabled (no Stripe key), the handler still registers but returns
// appropriate error messages.
func NewSubscriptionHandler(
	subSvc *subscription.Service,
	authSvc *auth.Service,
	pool *pgxpool.Pool,
	webhookSecret string,
) *SubscriptionHandler {
	return &SubscriptionHandler{
		subSvc:          subSvc,
		authSvc:         authSvc,
		queries:         dbgen.New(pool),
		webhookSecret:   webhookSecret,
		checkoutLimiter: ratelimit.NewRESTLimiter(3, 1*time.Hour),
	}
}

// WithAnalytics configures the handler to send events to PostHog.
func (h *SubscriptionHandler) WithAnalytics(client *analytics.Client) *SubscriptionHandler {
	h.analyticsClient = client
	return h
}

// HandleCreateCheckout handles POST /api/subscription/checkout.
// Creates a Stripe Checkout session for a subscription purchase.
func (h *SubscriptionHandler) HandleCreateCheckout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !h.subSvc.Enabled() {
		http.Error(w, "subscriptions not available", http.StatusServiceUnavailable)
		return
	}

	userID, ok := authenticateRESTRequest(r, h.authSvc)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	rateLimitKey := fmt.Sprintf("sub_checkout:%s", userID.String())
	if !h.checkoutLimiter.Allow(rateLimitKey) {
		ratelimit.Reject(w, "too many checkout attempts, please try again later")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req struct {
		Tier   string `json:"tier"`   // "explorer" or "voyager"
		Annual bool   `json:"annual"` // true for annual billing
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	t := tier.Parse(req.Tier)
	if t != tier.Explorer && t != tier.Voyager {
		http.Error(w, "tier must be 'explorer' or 'voyager'", http.StatusBadRequest)
		return
	}

	// Look up the user's email for the Stripe customer.
	user, err := h.queries.GetUserByID(r.Context(), userID)
	if err != nil {
		slog.Error("subscription checkout: user lookup failed", "error", err, "user_id", userID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	sessionURL, err := h.subSvc.CreateCheckoutSession(r.Context(), userID, user.Email, t, req.Annual)
	if err != nil {
		slog.Error("subscription checkout failed", "error", err, "user_id", userID, "tier", req.Tier)
		http.Error(w, "failed to create checkout session", http.StatusInternalServerError)
		return
	}

	if h.analyticsClient != nil {
		interval := "monthly"
		if req.Annual {
			interval = "annual"
		}
		h.analyticsClient.Track(userID.String(), "subscription_checkout_initiated", map[string]any{
			"tier":     req.Tier,
			"interval": interval,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"url": sessionURL,
	})
}

// HandleGetSubscription handles GET /api/subscription.
// Returns the user's current subscription status.
func (h *SubscriptionHandler) HandleGetSubscription(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := authenticateRESTRequest(r, h.authSvc)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	sub, err := h.subSvc.GetSubscription(r.Context(), userID)
	if err != nil {
		slog.Error("get subscription failed", "error", err, "user_id", userID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if sub == nil {
		// No subscription — return free tier info.
		freeTier := tier.Free
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"tier":     string(freeTier),
			"status":   "none",
			"features": freeTier.Features(),
		})
		return
	}

	resp := map[string]any{
		"tier":                 string(sub.Tier),
		"status":               sub.Status,
		"cancel_at_period_end": sub.CancelAtPeriodEnd,
		"features":             sub.Tier.Features(),
	}
	if sub.CurrentPeriodEnd != nil {
		resp["current_period_end"] = sub.CurrentPeriodEnd.Format(time.RFC3339)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleCancelSubscription handles POST /api/subscription/cancel.
// Cancels the user's subscription at the end of the current billing period.
func (h *SubscriptionHandler) HandleCancelSubscription(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !h.subSvc.Enabled() {
		http.Error(w, "subscriptions not available", http.StatusServiceUnavailable)
		return
	}

	userID, ok := authenticateRESTRequest(r, h.authSvc)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := h.subSvc.CancelSubscription(r.Context(), userID); err != nil {
		slog.Error("cancel subscription failed", "error", err, "user_id", userID)
		http.Error(w, "failed to cancel subscription", http.StatusInternalServerError)
		return
	}

	if h.analyticsClient != nil {
		h.analyticsClient.Track(userID.String(), "subscription_canceled", nil)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "canceled"})
}

// HandleWebhook handles POST /api/subscription/webhook.
// Verifies the Stripe signature and processes the webhook event.
func (h *SubscriptionHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Stripe webhooks can be up to 64KB.
	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	sigHeader := r.Header.Get("Stripe-Signature")
	if sigHeader == "" {
		http.Error(w, "missing Stripe-Signature header", http.StatusBadRequest)
		return
	}

	// Verify the webhook signature.
	// WithIgnoreAPIVersionMismatch allows events from newer Stripe API versions
	// to be processed without upgrading stripe-go (fields are best-effort deserialized).
	event, err := stripe.ConstructEvent(payload, sigHeader, h.webhookSecret, stripe.WithIgnoreAPIVersionMismatch())
	if err != nil {
		slog.Warn("stripe webhook signature verification failed", "error", err)
		http.Error(w, "invalid signature", http.StatusBadRequest)
		return
	}

	if err := h.subSvc.HandleWebhook(r.Context(), event); err != nil {
		slog.Error("stripe webhook processing failed", "error", err, "event_type", event.Type)
		// Return 200 anyway to prevent Stripe from retrying indefinitely.
		// The error is logged for investigation.
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"received":true}`))
}

// HandleCreatePortal handles POST /api/subscription/portal.
// Creates a Stripe Customer Portal session for subscription management.
func (h *SubscriptionHandler) HandleCreatePortal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !h.subSvc.Enabled() {
		http.Error(w, "subscriptions not available", http.StatusServiceUnavailable)
		return
	}

	userID, ok := authenticateRESTRequest(r, h.authSvc)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	portalURL, err := h.subSvc.CreatePortalSession(r.Context(), userID)
	if err != nil {
		slog.Error("create portal session failed", "error", err, "user_id", userID)
		http.Error(w, "failed to create portal session", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"url": portalURL,
	})
}
