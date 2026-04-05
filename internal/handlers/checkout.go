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

// allowedPriceVariants maps A/B test variant strings to validated price-in-cents.
// Only these values are accepted from the client; anything else falls back to the
// configured default price.
var allowedPriceVariants = map[string]int{
	"15": 1500,
	"19": 1900,
	"24": 2400,
}

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
// Initializes a Helcim checkout session for a trip purchase.
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
		TripID       string `json:"trip_id"`
		PriceVariant string `json:"price_variant"`
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

	// Resolve price from A/B variant (server-validated) or fall back to configured default.
	priceCents := h.paymentSvc.PriceCents()
	variant := "default"
	if v, ok := allowedPriceVariants[req.PriceVariant]; ok {
		priceCents = v
		variant = req.PriceVariant
	}

	slog.Info("checkout initiated",
		"user_id", userID,
		"trip_id", tripID,
		"price_variant", variant,
		"price_cents", priceCents,
	)

	result, err := h.paymentSvc.InitializeCheckoutWithPrice(r.Context(), userID, tripID, priceCents)
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

	// Track variant for analytics (async, non-blocking)
	if h.analyticsClient != nil {
		h.analyticsClient.Track(userID.String(), "checkout_initiated", map[string]any{
			"price_variant": variant,
			"price_cents":   priceCents,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"checkout_token": result.CheckoutToken,
		"secret_token":   result.SecretToken,
		"price_cents":    priceCents,
		"currency":       "CAD",
	})
}

// HandleValidatePayment handles POST /api/checkout/validate.
// Validates the HelcimPay.js response hash and records a successful payment.
func (h *CheckoutHandler) HandleValidatePayment(w http.ResponseWriter, r *http.Request) {
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
		CheckoutToken string          `json:"checkout_token"`
		Response      json.RawMessage `json:"response"`
		Hash          string          `json:"hash"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.CheckoutToken == "" || req.Hash == "" || len(req.Response) == 0 {
		http.Error(w, "checkout_token, response, and hash are required", http.StatusBadRequest)
		return
	}

	if err := h.paymentSvc.ValidateAndRecordPayment(r.Context(), userID, req.CheckoutToken, req.Response, req.Hash); err != nil {
		errMsg := err.Error()
		switch {
		case strings.Contains(errMsg, "not found"):
			http.Error(w, "checkout session not found", http.StatusNotFound)
		case strings.Contains(errMsg, "expired"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusGone)
			json.NewEncoder(w).Encode(map[string]string{"error": "checkout session expired"})
		case strings.Contains(errMsg, "hash mismatch"):
			http.Error(w, "payment validation failed", http.StatusBadRequest)
		default:
			slog.Error("payment validation failed", "error", err, "user_id", userID)
			http.Error(w, "internal error", http.StatusInternalServerError)
		}
		return
	}

	// Track successful checkout (async, non-blocking)
	if h.analyticsClient != nil {
		h.analyticsClient.Track(userID.String(), "checkout_completed", map[string]any{
			"amount_cents": h.paymentSvc.PriceCents(),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"unlocked": true})
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
