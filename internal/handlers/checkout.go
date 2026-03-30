package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gallowaysoftware/toqui-backend/internal/auth"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui-backend/internal/payment"
)

// CheckoutHandler handles Trip Pro purchase endpoints.
type CheckoutHandler struct {
	paymentSvc *payment.Service
	authSvc    *auth.Service
	queries    *dbgen.Queries
}

// NewCheckoutHandler creates a new CheckoutHandler.
func NewCheckoutHandler(paymentSvc *payment.Service, authSvc *auth.Service, pool *pgxpool.Pool) *CheckoutHandler {
	return &CheckoutHandler{
		paymentSvc: paymentSvc,
		authSvc:    authSvc,
		queries:    dbgen.New(pool),
	}
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"checkout_token": result.CheckoutToken,
		"secret_token":   result.SecretToken,
		"price_cents":    h.paymentSvc.PriceCents(),
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
