package payment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stripe/stripe-go/v82"

	"github.com/gallowaysoftware/toqui-backend/internal/audit"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui-backend/internal/trip"
)

// ErrNotTripOwner is returned when the caller tries to initiate Trip Pro
// checkout for a trip they don't own. Trip Pro unlocks an OWNED trip —
// collaborators (even editors) can't buy Pro on a trip they don't own
// because the receipt and unlock row are keyed to the purchasing user.
// Mapped to connect.CodePermissionDenied at the handler boundary. This
// is a re-export of trip.ErrNotOwnerOrEditor for API consistency with
// the other gated writes (#361 P1 #3 fix).
var ErrNotTripOwner = trip.ErrNotOwnerOrEditor

// Service handles Stripe payment operations for Trip Pro one-time purchases.
type Service struct {
	client         *stripe.Client
	productID      string // Stripe Product ID for Trip Pro one-time purchase
	priceCents     int
	queries        *dbgen.Queries
	alwaysUnlocked bool
	frontendURL    string
	enabled        bool
}

// NewService creates a new payment service. If stripeKey is empty, the service
// operates in disabled mode — IsTripUnlocked still works (reads from DB), but
// InitializeCheckout returns an error.
func NewService(stripeKey string, productID string, priceCents int, queries *dbgen.Queries, frontendURL string) *Service {
	s := &Service{
		productID:   productID,
		priceCents:  priceCents,
		queries:     queries,
		frontendURL: frontendURL,
		enabled:     stripeKey != "",
	}
	if s.enabled {
		s.client = stripe.NewClient(stripeKey)
		slog.Info("stripe payment service enabled (Trip Pro)")
	} else {
		slog.Info("stripe payment service disabled (no STRIPE_SECRET_KEY)")
	}
	return s
}

// SetAlwaysUnlocked configures the service to treat all trips as unlocked.
// Used in staging to give all users permanent pro access.
func (s *Service) SetAlwaysUnlocked(v bool) { s.alwaysUnlocked = v }

// CheckoutResult is returned after initializing a checkout session.
type CheckoutResult struct {
	URL string // Stripe Checkout Session URL for redirect
}

// InitializeCheckout creates a Stripe Checkout Session for a Trip Pro one-time
// purchase using the configured default price.
func (s *Service) InitializeCheckout(ctx context.Context, userID, tripID uuid.UUID) (*CheckoutResult, error) {
	return s.InitializeCheckoutWithPrice(ctx, userID, tripID, s.priceCents)
}

// InitializeCheckoutWithPrice creates a Stripe Checkout Session for a Trip Pro
// purchase at the specified price in cents. Used for A/B price testing.
func (s *Service) InitializeCheckoutWithPrice(ctx context.Context, userID, tripID uuid.UUID, priceCents int) (*CheckoutResult, error) {
	if !s.enabled {
		return nil, fmt.Errorf("stripe is not configured")
	}

	if s.productID == "" {
		return nil, fmt.Errorf("STRIPE_TRIP_PRO_PRODUCT_ID not configured")
	}

	// Verify the caller owns the trip before touching Stripe or the
	// checkout-sessions table (#361 P1). Previously the handler went
	// straight to IsTripUnlocked(userID, tripID) — which returns false
	// for ANY (userID, trip_id) pair that doesn't have a matching
	// trip_unlocks row, regardless of whether userID actually owns
	// trip_id. Net: a malicious client could burn their own payment
	// method creating Stripe sessions against a victim's trip_id.
	// GetTripByID filters on user_id in SQL, so pgx.ErrNoRows here
	// means "not the owner" and we return a clean sentinel that the
	// handler can map to CodePermissionDenied.
	if _, err := s.queries.GetTripByID(ctx, dbgen.GetTripByIDParams{
		ID:     tripID,
		UserID: userID,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotTripOwner
		}
		return nil, fmt.Errorf("check trip ownership: %w", err)
	}

	// Check if trip is already unlocked
	unlocked, err := s.queries.IsTripUnlocked(ctx, dbgen.IsTripUnlockedParams{
		UserID: userID,
		TripID: tripID,
	})
	if err != nil {
		return nil, fmt.Errorf("check trip unlock: %w", err)
	}
	if unlocked {
		return nil, fmt.Errorf("trip already unlocked")
	}

	params := &stripe.CheckoutSessionCreateParams{
		Mode:                stripe.String(string(stripe.CheckoutSessionModePayment)),
		AllowPromotionCodes: stripe.Bool(true),
		LineItems: []*stripe.CheckoutSessionCreateLineItemParams{
			{
				PriceData: &stripe.CheckoutSessionCreateLineItemPriceDataParams{
					Product:    stripe.String(s.productID),
					Currency:   stripe.String("cad"),
					UnitAmount: stripe.Int64(int64(priceCents)),
				},
				Quantity: stripe.Int64(1),
			},
		},
		SuccessURL: stripe.String(fmt.Sprintf("%s/trips/%s?payment=success", s.frontendURL, tripID.String())),
		CancelURL:  stripe.String(fmt.Sprintf("%s/trips/%s?payment=canceled", s.frontendURL, tripID.String())),
		Metadata: map[string]string{
			"user_id": userID.String(),
			"trip_id": tripID.String(),
			"type":    "trip_pro",
		},
	}

	session, err := s.client.V1CheckoutSessions.Create(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("create stripe checkout session: %w", err)
	}

	// Store the session in DB for tracking
	_, dbErr := s.queries.CreateCheckoutSession(ctx, dbgen.CreateCheckoutSessionParams{
		UserID:        userID,
		TripID:        tripID,
		CheckoutToken: session.ID, // Stripe session ID
		AmountCents:   int32(priceCents),
		Currency:      "CAD",
	})
	if dbErr != nil {
		// Log but don't fail — the Stripe session is created, and the webhook
		// will handle the unlock regardless of whether we tracked the session locally.
		slog.Warn("failed to store checkout session in DB", "error", dbErr, "session_id", session.ID)
	}

	slog.Info("stripe checkout session created (trip pro)",
		"user_id", userID,
		"trip_id", tripID,
		"amount_cents", priceCents,
		"session_id", session.ID,
	)

	return &CheckoutResult{
		URL: session.URL,
	}, nil
}

// HandlePaymentWebhook processes a Stripe checkout.session.completed event
// for a one-time Trip Pro payment. Called by the subscription webhook handler
// when it detects mode=payment.
func (s *Service) HandlePaymentWebhook(ctx context.Context, userID, tripID uuid.UUID, sessionID string, amountCents int32) error {
	// Check idempotency — don't double-unlock
	unlocked, err := s.queries.IsTripUnlocked(ctx, dbgen.IsTripUnlockedParams{
		UserID: userID,
		TripID: tripID,
	})
	if err != nil {
		return fmt.Errorf("check trip unlock: %w", err)
	}
	if unlocked {
		slog.Info("trip already unlocked, skipping (idempotent)", "user_id", userID, "trip_id", tripID)
		return nil
	}

	// Record payment
	payment, err := s.queries.CreatePayment(ctx, dbgen.CreatePaymentParams{
		UserID:            userID,
		TripID:            tripID,
		ExternalPaymentID: sessionID,
		AmountCents:       amountCents,
		Currency:          "CAD",
		Status:            "approved",
	})
	if err != nil {
		return fmt.Errorf("record payment: %w", err)
	}

	// Mark checkout session complete (best-effort — session may not exist if DB write failed earlier)
	if markErr := s.queries.MarkCheckoutSessionComplete(ctx, sessionID); markErr != nil {
		slog.Warn("failed to mark checkout session complete", "error", markErr, "session_id", sessionID)
	}

	// Unlock the trip
	_, err = s.queries.CreateTripUnlock(ctx, dbgen.CreateTripUnlockParams{
		UserID:    userID,
		TripID:    tripID,
		PaymentID: pgtype.UUID{Bytes: payment.ID, Valid: true},
		Source:    "purchase",
	})
	if err != nil {
		return fmt.Errorf("create trip unlock: %w", err)
	}

	audit.Log(audit.EventTripProPurchase,
		"user_id", userID.String(),
		"trip_id", tripID.String(),
		"amount_cents", amountCents,
		"stripe_session", sessionID,
	)

	slog.Info("trip pro purchased via stripe",
		"user_id", userID,
		"trip_id", tripID,
		"stripe_session", sessionID,
		"amount_cents", amountCents,
	)

	return nil
}

// IsTripUnlocked checks if a user has access to Trip Pro features for a given trip.
// When alwaysUnlocked is set (staging), all trips are treated as unlocked.
func (s *Service) IsTripUnlocked(ctx context.Context, userID, tripID uuid.UUID) (bool, error) {
	if s.alwaysUnlocked {
		return true, nil
	}
	return s.queries.IsTripUnlocked(ctx, dbgen.IsTripUnlockedParams{
		UserID: userID,
		TripID: tripID,
	})
}

// PriceCents returns the configured Trip Pro price in cents.
func (s *Service) PriceCents() int {
	return s.priceCents
}
