package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gallowaysoftware/toqui-backend/internal/analytics"
	"github.com/gallowaysoftware/toqui-backend/internal/auth"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui-backend/internal/payment"
	"github.com/gallowaysoftware/toqui-backend/internal/ratelimit"
)

// CheckoutHandler handles Trip Pro purchase endpoints.
type CheckoutHandler struct {
	paymentSvc      *payment.Service
	authSvc         *auth.Service
	queries         *dbgen.Queries
	checkoutLimiter *ratelimit.RESTLimiter
	analyticsClient *analytics.Client
}

// NewCheckoutHandler creates a new CheckoutHandler.
func NewCheckoutHandler(paymentSvc *payment.Service, authSvc *auth.Service, pool *pgxpool.Pool) *CheckoutHandler {
	return &CheckoutHandler{
		paymentSvc:      paymentSvc,
		authSvc:         authSvc,
		queries:         dbgen.New(pool),
		checkoutLimiter: ratelimit.NewRESTLimiter(3, 1*time.Hour), // 3 checkout initiations per hour
	}
}

// WithAnalytics configures the checkout handler to send events to PostHog.
func (h *CheckoutHandler) WithAnalytics(client *analytics.Client) *CheckoutHandler {
	h.analyticsClient = client
	return h
}

// HandleCreateCheckout handles POST /api/checkout.
// Creates a Stripe Checkout Session for a Trip Pro one-time purchase.
// Returns { "url": "https://checkout.stripe.com/..." } for frontend redirect.
func (h *CheckoutHandler) HandleCreateCheckout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := authenticateRESTRequest(r, h.authSvc)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Per-user rate limit: 3 checkout initiations per hour
	rateLimitKey := fmt.Sprintf("checkout:%s", userID.String())
	if !h.checkoutLimiter.Allow(rateLimitKey) {
		ratelimit.Reject(w, "too many checkout attempts, please try again later")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req struct {
		TripID string `json:"trip_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	tripID, err := uuid.Parse(req.TripID)
	if err != nil {
		http.Error(w, "invalid trip_id", http.StatusBadRequest)
		return
	}

	priceCents := h.paymentSvc.PriceCents()

	slog.Info("checkout initiated",
		"user_id", userID,
		"trip_id", tripID,
		"price_cents", priceCents,
	)

	result, err := h.paymentSvc.InitializeCheckout(r.Context(), userID, tripID)
	if err != nil {
		if strings.Contains(err.Error(), "already unlocked") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]string{"error": "trip already unlocked"})
			return
		}
		slog.Error("checkout initialization failed", "error", err, "user_id", userID, "trip_id", tripID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Track checkout initiation for analytics (async, non-blocking)
	if h.analyticsClient != nil {
		h.analyticsClient.Track(userID.String(), "checkout_initiated", map[string]any{
			"price_cents": priceCents,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"url":         result.URL,
		"price_cents": priceCents,
		"currency":    "CAD",
	})
}

// HandleCheckUnlock handles GET /api/checkout/status?trip_id=UUID — checks if a trip is unlocked.
func (h *CheckoutHandler) HandleCheckUnlock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := authenticateRESTRequest(r, h.authSvc)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	tripIDStr := r.URL.Query().Get("trip_id")
	tripID, err := uuid.Parse(tripIDStr)
	if err != nil {
		http.Error(w, "invalid trip_id", http.StatusBadRequest)
		return
	}

	unlocked, err := h.paymentSvc.IsTripUnlocked(r.Context(), userID, tripID)
	if err != nil {
		slog.Error("check unlock failed", "error", err, "user_id", userID, "trip_id", tripID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"unlocked":    unlocked,
		"price_cents": h.paymentSvc.PriceCents(),
		"currency":    "CAD",
	})
}
