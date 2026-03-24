package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

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
// Payment processing is not yet available — records interest and returns "coming soon".
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

	// Record interest in Trip Pro
	user, err := h.queries.GetUserByID(r.Context(), userID)
	if err == nil {
		if _, recErr := h.queries.RecordProInterest(r.Context(), dbgen.RecordProInterestParams{
			UserID: userID,
			Email:  user.Email,
		}); recErr != nil {
			slog.Debug("pro interest record failed (likely duplicate)", "error", recErr, "user_id", userID)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status":  "coming_soon",
		"message": "Trip Pro is coming soon! Your interest has been recorded.",
	})
}

// HandleValidatePayment handles POST /api/checkout/validate.
// Payment processing is not yet available.
func (h *CheckoutHandler) HandleValidatePayment(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusGone)
	json.NewEncoder(w).Encode(map[string]string{
		"error": "payment processing not yet available",
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
