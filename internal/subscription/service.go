// Package subscription handles Stripe subscription management for Explorer and
// Voyager tiers. It gracefully no-ops when STRIPE_SECRET_KEY is empty, allowing
// local development without Stripe credentials.
package subscription

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stripe/stripe-go/v82"

	"github.com/gallowaysoftware/toqui-backend/internal/audit"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui-backend/internal/payment"
	"github.com/gallowaysoftware/toqui-backend/internal/tier"
)

// ProductConfig holds the Stripe Product IDs for each subscription tier and
// billing interval. These are created in the Stripe Dashboard and provided via
// env vars. The checkout session uses PriceData to reference the product with
// its default price, avoiding the need to separately manage Price IDs.
type ProductConfig struct {
	ExplorerMonthly string
	ExplorerAnnual  string
	VoyagerMonthly  string
	VoyagerAnnual   string
}

// Subscription represents a user's subscription state returned to the handler.
type Subscription struct {
	Tier                 tier.UserTier `json:"tier"`
	Status               string        `json:"status"`
	CurrentPeriodEnd     *time.Time    `json:"current_period_end,omitempty"`
	CancelAtPeriodEnd    bool          `json:"cancel_at_period_end"`
	StripeCustomerID     string        `json:"-"`
	StripeSubscriptionID string        `json:"-"`
}

// Service manages Stripe subscriptions. All methods gracefully return zero
// values or no-op when the Stripe key is empty (disabled mode).
type Service struct {
	client  *stripe.Client
	queries *dbgen.Queries
	prices  ProductConfig
	enabled bool

	// frontendURL is used for Stripe Checkout success/cancel redirects.
	frontendURL string

	// paymentSvc handles Trip Pro one-time payment webhook processing.
	// Set via SetPaymentService after construction to avoid circular deps.
	paymentSvc *payment.Service
}

// NewService creates a new subscription service. If stripeKey is empty, the
// service operates in disabled mode — all methods return gracefully without
// contacting Stripe.
func NewService(stripeKey string, queries *dbgen.Queries, prices ProductConfig, frontendURL string) *Service {
	s := &Service{
		queries:     queries,
		prices:      prices,
		frontendURL: frontendURL,
		enabled:     stripeKey != "",
	}
	if s.enabled {
		s.client = stripe.NewClient(stripeKey)
		slog.Info("stripe subscription service enabled")
	} else {
		slog.Info("stripe subscription service disabled (no STRIPE_SECRET_KEY)")
	}
	return s
}

// SetPaymentService configures the payment service for handling Trip Pro
// one-time payment webhooks. Called after construction in main.go.
func (s *Service) SetPaymentService(ps *payment.Service) { s.paymentSvc = ps }

// Enabled returns true when Stripe is configured and the service is operational.
func (s *Service) Enabled() bool { return s.enabled }

// GetUserTier determines the effective tier for a user by checking both the
// subscriptions table (Explorer/Voyager) and the users.subscription_tier column.
// Trip-level Pro unlocks are handled separately by the payment service.
func (s *Service) GetUserTier(ctx context.Context, userID uuid.UUID) tier.UserTier {
	// Check subscriptions table first — active subscriptions take priority.
	sub, err := s.queries.GetSubscriptionByUserID(ctx, userID)
	if err == nil && sub.Status == "active" {
		t := tier.Parse(sub.Tier)
		if t == tier.Explorer || t == tier.Voyager {
			return t
		}
	}

	// Fall back to users.subscription_tier column (covers Pro from trip unlocks
	// and any admin-granted tiers).
	raw, err := s.queries.GetUserSubscriptionTier(ctx, userID)
	if err != nil {
		return tier.Free
	}
	return tier.Parse(raw)
}

// CreateCheckoutSession creates a Stripe Checkout session for the given tier
// and billing interval. Returns the session URL that the frontend should
// redirect the user to.
func (s *Service) CreateCheckoutSession(ctx context.Context, userID uuid.UUID, email string, t tier.UserTier, annual bool) (string, error) {
	if !s.enabled {
		return "", fmt.Errorf("stripe is not configured")
	}

	priceID, err := s.resolvePriceID(t, annual)
	if err != nil {
		return "", err
	}

	// Find or create a Stripe customer for this user.
	customerID, err := s.getOrCreateCustomer(ctx, userID, email)
	if err != nil {
		return "", fmt.Errorf("get or create customer: %w", err)
	}

	interval := "monthly"
	if annual {
		interval = "annual"
	}

	params := &stripe.CheckoutSessionCreateParams{
		Customer: stripe.String(customerID),
		Mode:     stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionCreateLineItemParams{
			{
				Price:    stripe.String(priceID),
				Quantity: stripe.Int64(1),
			},
		},
		SuccessURL:        stripe.String(fmt.Sprintf("%s/settings?subscription=success", s.frontendURL)),
		CancelURL:         stripe.String(fmt.Sprintf("%s/settings?subscription=canceled", s.frontendURL)),
		ClientReferenceID: stripe.String(userID.String()),
		Metadata: map[string]string{
			"user_id":  userID.String(),
			"tier":     string(t),
			"interval": interval,
		},
	}

	session, err := s.client.V1CheckoutSessions.Create(ctx, params)
	if err != nil {
		return "", fmt.Errorf("create checkout session: %w", err)
	}

	slog.Info("stripe checkout session created",
		"user_id", userID,
		"tier", t,
		"interval", interval,
		"session_id", session.ID,
	)

	return session.URL, nil
}

// CreatePortalSession creates a Stripe Customer Portal session so the user can
// manage their subscription (update payment method, cancel, etc.).
func (s *Service) CreatePortalSession(ctx context.Context, userID uuid.UUID) (string, error) {
	if !s.enabled {
		return "", fmt.Errorf("stripe is not configured")
	}

	sub, err := s.queries.GetSubscriptionByUserID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("no subscription found for user")
	}

	params := &stripe.BillingPortalSessionCreateParams{
		Customer:  stripe.String(sub.StripeCustomerID),
		ReturnURL: stripe.String(fmt.Sprintf("%s/settings", s.frontendURL)),
	}

	session, err := s.client.V1BillingPortalSessions.Create(ctx, params)
	if err != nil {
		return "", fmt.Errorf("create portal session: %w", err)
	}

	return session.URL, nil
}

// CancelSubscription sets the user's subscription to cancel at the end of the
// current billing period. The subscription remains active until the period ends.
func (s *Service) CancelSubscription(ctx context.Context, userID uuid.UUID) error {
	if !s.enabled {
		return fmt.Errorf("stripe is not configured")
	}

	sub, err := s.queries.GetSubscriptionByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("no subscription found for user")
	}
	if !sub.StripeSubscriptionID.Valid || sub.StripeSubscriptionID.String == "" {
		return fmt.Errorf("no active stripe subscription")
	}

	_, err = s.client.V1Subscriptions.Update(ctx, sub.StripeSubscriptionID.String, &stripe.SubscriptionUpdateParams{
		CancelAtPeriodEnd: stripe.Bool(true),
	})
	if err != nil {
		return fmt.Errorf("cancel subscription: %w", err)
	}

	s.queries.SetSubscriptionCancelAtPeriodEnd(ctx, dbgen.SetSubscriptionCancelAtPeriodEndParams{
		CancelAtPeriodEnd:    pgtype.Bool{Bool: true, Valid: true},
		StripeSubscriptionID: sub.StripeSubscriptionID,
	})

	audit.Log("subscription.cancel",
		"user_id", userID,
		"subscription_id", sub.StripeSubscriptionID.String,
	)

	return nil
}

// GetSubscription returns the user's current subscription, or nil if none exists.
func (s *Service) GetSubscription(ctx context.Context, userID uuid.UUID) (*Subscription, error) {
	sub, err := s.queries.GetSubscriptionByUserID(ctx, userID)
	if err != nil {
		return nil, nil // No subscription is not an error
	}

	result := &Subscription{
		Tier:                 tier.Parse(sub.Tier),
		Status:               sub.Status,
		CancelAtPeriodEnd:    sub.CancelAtPeriodEnd.Valid && sub.CancelAtPeriodEnd.Bool,
		StripeCustomerID:     sub.StripeCustomerID,
		StripeSubscriptionID: sub.StripeSubscriptionID.String,
	}

	if sub.CurrentPeriodEnd.Valid {
		t := sub.CurrentPeriodEnd.Time
		result.CurrentPeriodEnd = &t
	}

	return result, nil
}

// HandleWebhook processes a Stripe webhook event. The caller must verify the
// signature before calling this method (see handler).
func (s *Service) HandleWebhook(ctx context.Context, event stripe.Event) error {
	switch event.Type {
	case stripe.EventTypeCheckoutSessionCompleted:
		return s.handleCheckoutCompleted(ctx, event)
	case stripe.EventTypeCustomerSubscriptionUpdated:
		return s.handleSubscriptionUpdated(ctx, event)
	case stripe.EventTypeCustomerSubscriptionDeleted:
		return s.handleSubscriptionDeleted(ctx, event)
	default:
		slog.Debug("unhandled stripe webhook event", "type", event.Type)
		return nil
	}
}

// handleCheckoutCompleted processes a successful Checkout Session for a new
// subscription. It creates the subscription record and updates the user's tier.
func (s *Service) handleCheckoutCompleted(ctx context.Context, event stripe.Event) error {
	var session stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
		return fmt.Errorf("unmarshal checkout session: %w", err)
	}

	// Handle Trip Pro one-time payments (mode=payment).
	if string(session.Mode) == string(stripe.CheckoutSessionModePayment) {
		return s.handlePaymentCheckoutCompleted(ctx, session)
	}

	if string(session.Mode) != string(stripe.CheckoutSessionModeSubscription) {
		slog.Debug("ignoring checkout session with unexpected mode", "mode", session.Mode)
		return nil
	}

	userIDStr, ok := session.Metadata["user_id"]
	if !ok || userIDStr == "" {
		return fmt.Errorf("missing user_id in checkout session metadata")
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return fmt.Errorf("invalid user_id in metadata: %w", err)
	}

	tierStr, _ := session.Metadata["tier"]
	t := tier.Parse(tierStr)
	if t != tier.Explorer && t != tier.Voyager {
		return fmt.Errorf("unexpected tier in metadata: %s", tierStr)
	}

	// The subscription ID is on the session object.
	subscriptionID := ""
	if session.Subscription != nil {
		subscriptionID = session.Subscription.ID
	}

	customerID := ""
	if session.Customer != nil {
		customerID = session.Customer.ID
	}

	// Fetch the subscription to get period dates from items.
	var periodStart, periodEnd time.Time
	if subscriptionID != "" && s.client != nil {
		stripeSub, err := s.client.V1Subscriptions.Retrieve(ctx, subscriptionID, nil)
		if err == nil && stripeSub.Items != nil && len(stripeSub.Items.Data) > 0 {
			item := stripeSub.Items.Data[0]
			periodStart = time.Unix(item.CurrentPeriodStart, 0)
			periodEnd = time.Unix(item.CurrentPeriodEnd, 0)
		}
	}

	_, err = s.queries.CreateSubscription(ctx, dbgen.CreateSubscriptionParams{
		UserID:               userID,
		StripeCustomerID:     customerID,
		StripeSubscriptionID: pgtype.Text{String: subscriptionID, Valid: subscriptionID != ""},
		Tier:                 string(t),
		Status:               "active",
		CurrentPeriodStart:   pgtype.Timestamptz{Time: periodStart, Valid: !periodStart.IsZero()},
		CurrentPeriodEnd:     pgtype.Timestamptz{Time: periodEnd, Valid: !periodEnd.IsZero()},
	})
	if err != nil {
		return fmt.Errorf("create subscription record: %w", err)
	}

	// Update the user's tier column for quick lookups.
	if err := s.queries.SetUserSubscriptionTierByID(ctx, dbgen.SetUserSubscriptionTierByIDParams{
		Tier:   string(t),
		UserID: userID,
	}); err != nil {
		slog.Warn("failed to update user subscription_tier column", "error", err, "user_id", userID)
	}

	audit.Log("subscription.created",
		"user_id", userID,
		"tier", t,
		"stripe_subscription_id", subscriptionID,
	)

	slog.Info("subscription created via checkout",
		"user_id", userID,
		"tier", t,
		"subscription_id", subscriptionID,
	)

	return nil
}

// handleSubscriptionUpdated handles changes to an existing subscription (renewals,
// plan changes, payment issues).
func (s *Service) handleSubscriptionUpdated(ctx context.Context, event stripe.Event) error {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		return fmt.Errorf("unmarshal subscription: %w", err)
	}

	if sub.ID == "" {
		return fmt.Errorf("subscription ID is empty")
	}

	stripeSubID := pgtype.Text{String: sub.ID, Valid: true}

	// Update status
	s.queries.UpdateSubscriptionStatus(ctx, dbgen.UpdateSubscriptionStatusParams{
		Status:               string(sub.Status),
		StripeSubscriptionID: stripeSubID,
	})

	// Update cancel_at_period_end
	s.queries.SetSubscriptionCancelAtPeriodEnd(ctx, dbgen.SetSubscriptionCancelAtPeriodEndParams{
		CancelAtPeriodEnd:    pgtype.Bool{Bool: sub.CancelAtPeriodEnd, Valid: true},
		StripeSubscriptionID: stripeSubID,
	})

	// Update period dates from items
	if sub.Items != nil && len(sub.Items.Data) > 0 {
		item := sub.Items.Data[0]
		s.queries.UpdateSubscriptionPeriod(ctx, dbgen.UpdateSubscriptionPeriodParams{
			CurrentPeriodStart:   pgtype.Timestamptz{Time: time.Unix(item.CurrentPeriodStart, 0), Valid: true},
			CurrentPeriodEnd:     pgtype.Timestamptz{Time: time.Unix(item.CurrentPeriodEnd, 0), Valid: true},
			StripeSubscriptionID: stripeSubID,
		})
	}

	// If the subscription is no longer active, revert user tier to free.
	if sub.Status == stripe.SubscriptionStatusCanceled || sub.Status == stripe.SubscriptionStatusUnpaid {
		dbSub, err := s.queries.GetSubscriptionByStripeSubscriptionID(ctx, stripeSubID)
		if err == nil {
			s.queries.SetUserSubscriptionTierByID(ctx, dbgen.SetUserSubscriptionTierByIDParams{
				Tier:   string(tier.Free),
				UserID: dbSub.UserID,
			})
			audit.Log("subscription.tier_reverted",
				"user_id", dbSub.UserID,
				"stripe_subscription_id", sub.ID,
				"new_status", sub.Status,
			)
		}
	}

	slog.Info("subscription updated via webhook",
		"subscription_id", sub.ID,
		"status", sub.Status,
		"cancel_at_period_end", sub.CancelAtPeriodEnd,
	)

	return nil
}

// handleSubscriptionDeleted handles a subscription that has been fully canceled
// and removed.
func (s *Service) handleSubscriptionDeleted(ctx context.Context, event stripe.Event) error {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		return fmt.Errorf("unmarshal subscription: %w", err)
	}

	stripeSubID := pgtype.Text{String: sub.ID, Valid: true}

	// Mark as canceled in our database
	s.queries.UpdateSubscriptionStatus(ctx, dbgen.UpdateSubscriptionStatusParams{
		Status:               "canceled",
		StripeSubscriptionID: stripeSubID,
	})

	// Revert user tier to free
	dbSub, err := s.queries.GetSubscriptionByStripeSubscriptionID(ctx, stripeSubID)
	if err == nil {
		s.queries.SetUserSubscriptionTierByID(ctx, dbgen.SetUserSubscriptionTierByIDParams{
			Tier:   string(tier.Free),
			UserID: dbSub.UserID,
		})
		audit.Log("subscription.deleted",
			"user_id", dbSub.UserID,
			"stripe_subscription_id", sub.ID,
		)
	}

	slog.Info("subscription deleted via webhook",
		"subscription_id", sub.ID,
	)

	return nil
}

// handlePaymentCheckoutCompleted processes a Stripe checkout.session.completed
// event where mode=payment (Trip Pro one-time purchase). It delegates to the
// payment service to unlock the trip.
func (s *Service) handlePaymentCheckoutCompleted(ctx context.Context, session stripe.CheckoutSession) error {
	if s.paymentSvc == nil {
		return fmt.Errorf("payment service not configured for Trip Pro webhook handling")
	}

	userIDStr, ok := session.Metadata["user_id"]
	if !ok || userIDStr == "" {
		return fmt.Errorf("missing user_id in payment checkout session metadata")
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return fmt.Errorf("invalid user_id in payment metadata: %w", err)
	}

	tripIDStr, ok := session.Metadata["trip_id"]
	if !ok || tripIDStr == "" {
		return fmt.Errorf("missing trip_id in payment checkout session metadata")
	}
	tripID, err := uuid.Parse(tripIDStr)
	if err != nil {
		return fmt.Errorf("invalid trip_id in payment metadata: %w", err)
	}

	// Extract amount from session (Stripe returns amount in smallest currency unit)
	var amountCents int32
	if session.AmountTotal > 0 {
		amountCents = int32(session.AmountTotal)
	}

	return s.paymentSvc.HandlePaymentWebhook(ctx, userID, tripID, session.ID, amountCents)
}

// getOrCreateCustomer looks up the user's Stripe customer ID from the
// subscriptions table, or creates a new Stripe customer if none exists.
func (s *Service) getOrCreateCustomer(ctx context.Context, userID uuid.UUID, email string) (string, error) {
	// Check if we already have a customer for this user.
	sub, err := s.queries.GetSubscriptionByUserID(ctx, userID)
	if err == nil && sub.StripeCustomerID != "" {
		return sub.StripeCustomerID, nil
	}

	// Create a new Stripe customer.
	customer, err := s.client.V1Customers.Create(ctx, &stripe.CustomerCreateParams{
		Email: stripe.String(email),
		Metadata: map[string]string{
			"user_id": userID.String(),
		},
	})
	if err != nil {
		return "", fmt.Errorf("create stripe customer: %w", err)
	}

	return customer.ID, nil
}

// resolvePriceID maps a tier + annual flag to the corresponding Stripe Price ID.
func (s *Service) resolvePriceID(t tier.UserTier, annual bool) (string, error) {
	var priceID string
	switch t {
	case tier.Explorer:
		if annual {
			priceID = s.prices.ExplorerAnnual
		} else {
			priceID = s.prices.ExplorerMonthly
		}
	case tier.Voyager:
		if annual {
			priceID = s.prices.VoyagerAnnual
		} else {
			priceID = s.prices.VoyagerMonthly
		}
	default:
		return "", fmt.Errorf("unsupported tier for subscription: %s", t)
	}

	if priceID == "" {
		return "", fmt.Errorf("stripe price ID not configured for %s (annual=%v)", t, annual)
	}
	return priceID, nil
}
