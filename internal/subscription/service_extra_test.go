package subscription

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stripe/stripe-go/v82"

	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui-backend/internal/tier"
)

// ---------------------------------------------------------------------------
// Test scaffolding
// ---------------------------------------------------------------------------

// stubQueries is a hand-rolled test double for subscriptionQueries with
// fail-loud defaults. Same pattern as internal/payment/stripe_test.go's
// stubQueries: each method calls tb.Fatalf when called without an injected
// `*Fn`, so a test that forgets to configure a query path fails with a
// precise "set <fnName>" message instead of silently passing on a
// zero-value response. Lessons-learned from #418's adversarial review.
type stubQueries struct {
	tb testing.TB

	getSubscriptionByUserIDFn               func(ctx context.Context, userID uuid.UUID) (dbgen.Subscription, error)
	getSubscriptionByStripeSubscriptionIDFn func(ctx context.Context, id pgtype.Text) (dbgen.Subscription, error)
	getUserSubscriptionTierFn               func(ctx context.Context, id uuid.UUID) (string, error)
	createSubscriptionFn                    func(ctx context.Context, arg dbgen.CreateSubscriptionParams) (dbgen.Subscription, error)
	updateSubscriptionStatusFn              func(ctx context.Context, arg dbgen.UpdateSubscriptionStatusParams) error
	updateSubscriptionPeriodFn              func(ctx context.Context, arg dbgen.UpdateSubscriptionPeriodParams) error
	updateSubscriptionBillingPeriodFn       func(ctx context.Context, arg dbgen.UpdateSubscriptionBillingPeriodParams) error
	updateSubscriptionTierFn                func(ctx context.Context, arg dbgen.UpdateSubscriptionTierParams) error
	setSubscriptionCancelAtPeriodEndFn      func(ctx context.Context, arg dbgen.SetSubscriptionCancelAtPeriodEndParams) error
	setUserSubscriptionTierByIDFn           func(ctx context.Context, arg dbgen.SetUserSubscriptionTierByIDParams) error

	// Captured calls.
	createSubscriptionCalls          []dbgen.CreateSubscriptionParams
	updateStatusCalls                []dbgen.UpdateSubscriptionStatusParams
	updatePeriodCalls                []dbgen.UpdateSubscriptionPeriodParams
	updateBillingPeriodCalls         []dbgen.UpdateSubscriptionBillingPeriodParams
	updateTierCalls                  []dbgen.UpdateSubscriptionTierParams
	setCancelAtPeriodEndCalls        []dbgen.SetSubscriptionCancelAtPeriodEndParams
	setUserSubscriptionTierByIDCalls []dbgen.SetUserSubscriptionTierByIDParams
}

func (s *stubQueries) GetSubscriptionByUserID(ctx context.Context, userID uuid.UUID) (dbgen.Subscription, error) {
	if s.getSubscriptionByUserIDFn != nil {
		return s.getSubscriptionByUserIDFn(ctx, userID)
	}
	s.tb.Fatalf("unexpected stubQueries.GetSubscriptionByUserID(%s) — set getSubscriptionByUserIDFn", userID)
	return dbgen.Subscription{}, nil
}

func (s *stubQueries) GetSubscriptionByStripeSubscriptionID(ctx context.Context, id pgtype.Text) (dbgen.Subscription, error) {
	if s.getSubscriptionByStripeSubscriptionIDFn != nil {
		return s.getSubscriptionByStripeSubscriptionIDFn(ctx, id)
	}
	s.tb.Fatalf("unexpected stubQueries.GetSubscriptionByStripeSubscriptionID(%v) — set getSubscriptionByStripeSubscriptionIDFn", id)
	return dbgen.Subscription{}, nil
}

func (s *stubQueries) GetUserSubscriptionTier(ctx context.Context, id uuid.UUID) (string, error) {
	if s.getUserSubscriptionTierFn != nil {
		return s.getUserSubscriptionTierFn(ctx, id)
	}
	s.tb.Fatalf("unexpected stubQueries.GetUserSubscriptionTier(%s) — set getUserSubscriptionTierFn", id)
	return "", nil
}

func (s *stubQueries) CreateSubscription(ctx context.Context, arg dbgen.CreateSubscriptionParams) (dbgen.Subscription, error) {
	s.createSubscriptionCalls = append(s.createSubscriptionCalls, arg)
	if s.createSubscriptionFn != nil {
		return s.createSubscriptionFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubQueries.CreateSubscription(%+v) — set createSubscriptionFn", arg)
	return dbgen.Subscription{}, nil
}

func (s *stubQueries) UpdateSubscriptionStatus(ctx context.Context, arg dbgen.UpdateSubscriptionStatusParams) error {
	s.updateStatusCalls = append(s.updateStatusCalls, arg)
	if s.updateSubscriptionStatusFn != nil {
		return s.updateSubscriptionStatusFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubQueries.UpdateSubscriptionStatus(%+v) — set updateSubscriptionStatusFn", arg)
	return nil
}

func (s *stubQueries) UpdateSubscriptionPeriod(ctx context.Context, arg dbgen.UpdateSubscriptionPeriodParams) error {
	s.updatePeriodCalls = append(s.updatePeriodCalls, arg)
	if s.updateSubscriptionPeriodFn != nil {
		return s.updateSubscriptionPeriodFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubQueries.UpdateSubscriptionPeriod(%+v) — set updateSubscriptionPeriodFn", arg)
	return nil
}

func (s *stubQueries) UpdateSubscriptionBillingPeriod(ctx context.Context, arg dbgen.UpdateSubscriptionBillingPeriodParams) error {
	s.updateBillingPeriodCalls = append(s.updateBillingPeriodCalls, arg)
	if s.updateSubscriptionBillingPeriodFn != nil {
		return s.updateSubscriptionBillingPeriodFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubQueries.UpdateSubscriptionBillingPeriod(%+v) — set updateSubscriptionBillingPeriodFn", arg)
	return nil
}

func (s *stubQueries) UpdateSubscriptionTier(ctx context.Context, arg dbgen.UpdateSubscriptionTierParams) error {
	s.updateTierCalls = append(s.updateTierCalls, arg)
	if s.updateSubscriptionTierFn != nil {
		return s.updateSubscriptionTierFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubQueries.UpdateSubscriptionTier(%+v) — set updateSubscriptionTierFn", arg)
	return nil
}

func (s *stubQueries) SetSubscriptionCancelAtPeriodEnd(ctx context.Context, arg dbgen.SetSubscriptionCancelAtPeriodEndParams) error {
	s.setCancelAtPeriodEndCalls = append(s.setCancelAtPeriodEndCalls, arg)
	if s.setSubscriptionCancelAtPeriodEndFn != nil {
		return s.setSubscriptionCancelAtPeriodEndFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubQueries.SetSubscriptionCancelAtPeriodEnd(%+v) — set setSubscriptionCancelAtPeriodEndFn", arg)
	return nil
}

func (s *stubQueries) SetUserSubscriptionTierByID(ctx context.Context, arg dbgen.SetUserSubscriptionTierByIDParams) error {
	s.setUserSubscriptionTierByIDCalls = append(s.setUserSubscriptionTierByIDCalls, arg)
	if s.setUserSubscriptionTierByIDFn != nil {
		return s.setUserSubscriptionTierByIDFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubQueries.SetUserSubscriptionTierByID(%+v) — set setUserSubscriptionTierByIDFn", arg)
	return nil
}

// newTestService builds a Service via the real NewService constructor so
// constructor-side logic — `enabled = stripeKey != ""`, the structured
// log, future feature-flag wiring — is exercised by every test that
// goes through this helper. Then it swaps in the stub for `queries`
// (the only field NewService can't accept directly because its
// signature type-locks to *dbgen.Queries). Mirrors newTestService in
// internal/payment/stripe_test.go.
func newTestService(t *testing.T, q *stubQueries, enabled bool) *Service {
	t.Helper()
	stripeKey := ""
	if enabled {
		stripeKey = "sk_test_dummy"
	}
	prices := ProductConfig{
		ExplorerMonthly: "price_explorer_monthly",
		ExplorerAnnual:  "price_explorer_annual",
		VoyagerMonthly:  "price_voyager_monthly",
		VoyagerAnnual:   "price_voyager_annual",
	}
	svc := NewService(stripeKey, nil, prices, "https://app.toqui.test")
	svc.queries = q
	return svc
}

// stripeClientWithBaseURL returns a stripe.Client configured to talk to
// the given httptest.Server URL instead of the real Stripe API. Mirrors
// the helper in internal/payment/stripe_test.go.
func stripeClientWithBaseURL(baseURL string) *stripe.Client {
	cfg := &stripe.BackendConfig{URL: stripe.String(baseURL)}
	backends := &stripe.Backends{
		API:     stripe.GetBackendWithConfig(stripe.APIBackend, cfg),
		Connect: stripe.GetBackendWithConfig(stripe.ConnectBackend, cfg),
		Uploads: stripe.GetBackendWithConfig(stripe.UploadsBackend, cfg),
	}
	return stripe.NewClient("sk_test_dummy", stripe.WithBackends(backends))
}

// ---------------------------------------------------------------------------
// NewService
// ---------------------------------------------------------------------------

func TestNewService_DisabledModeWhenKeyEmpty(t *testing.T) {
	svc := NewService("", nil, ProductConfig{}, "https://app.toqui.test")
	if svc == nil {
		t.Fatal("NewService returned nil")
	}
	if svc.Enabled() {
		t.Error("expected Enabled()=false when stripeKey is empty")
	}
	if svc.client != nil {
		t.Error("expected nil stripe client in disabled mode")
	}
}

func TestNewService_EnabledWhenKeyPresent(t *testing.T) {
	svc := NewService("sk_test_dummy", nil, ProductConfig{}, "https://app.toqui.test")
	if !svc.Enabled() {
		t.Error("expected Enabled()=true when stripeKey is set")
	}
	if svc.client == nil {
		t.Error("expected non-nil stripe client when enabled")
	}
}

func TestSetPaymentService_RoundTrip(t *testing.T) {
	// Pure setter — only invariant we care about is "doesn't panic on
	// nil and stores what's passed". Pin the nil case explicitly because
	// main.go relies on being able to defer SetPaymentService until
	// after the payment service is constructed.
	svc := NewService("", nil, ProductConfig{}, "")
	svc.SetPaymentService(nil)
	if svc.paymentSvc != nil {
		t.Error("expected paymentSvc to remain nil after SetPaymentService(nil)")
	}
}

// ---------------------------------------------------------------------------
// GetUserTier
// ---------------------------------------------------------------------------

func TestGetUserTier_ActiveSubscriptionReturnsItsTier(t *testing.T) {
	userID := uuid.New()
	q := &stubQueries{tb: t,
		getSubscriptionByUserIDFn: func(ctx context.Context, _ uuid.UUID) (dbgen.Subscription, error) {
			return dbgen.Subscription{Tier: string(tier.Voyager), Status: "active"}, nil
		},
	}
	svc := newTestService(t, q, true)

	got := svc.GetUserTier(context.Background(), userID)
	if got != tier.Voyager {
		t.Errorf("expected Voyager, got %q", got)
	}
}

func TestGetUserTier_InactiveSubscriptionFallsBackToUserColumn(t *testing.T) {
	// Status != "active" must NOT bypass the fallback. This is the
	// invariant that prevents past_due subscriptions from granting
	// continued access — the subscription row exists but the tier the
	// user gets is whatever users.subscription_tier says (typically
	// "free" once Stripe lifecycle has reverted it).
	userID := uuid.New()
	q := &stubQueries{tb: t,
		getSubscriptionByUserIDFn: func(ctx context.Context, _ uuid.UUID) (dbgen.Subscription, error) {
			return dbgen.Subscription{Tier: string(tier.Voyager), Status: "past_due"}, nil
		},
		getUserSubscriptionTierFn: func(ctx context.Context, _ uuid.UUID) (string, error) {
			return string(tier.Free), nil
		},
	}
	svc := newTestService(t, q, true)

	got := svc.GetUserTier(context.Background(), userID)
	if got != tier.Free {
		t.Errorf("expected Free fallback for past_due subscription, got %q", got)
	}
}

func TestGetUserTier_NoSubscriptionReadsUserColumn(t *testing.T) {
	// Pro from a Trip Pro unlock lives in users.subscription_tier, not
	// the subscriptions table. This test pins the "no subscription row
	// → user column" path.
	userID := uuid.New()
	q := &stubQueries{tb: t,
		getSubscriptionByUserIDFn: func(ctx context.Context, _ uuid.UUID) (dbgen.Subscription, error) {
			return dbgen.Subscription{}, pgx.ErrNoRows
		},
		getUserSubscriptionTierFn: func(ctx context.Context, _ uuid.UUID) (string, error) {
			return string(tier.Pro), nil
		},
	}
	svc := newTestService(t, q, true)

	if got := svc.GetUserTier(context.Background(), userID); got != tier.Pro {
		t.Errorf("expected Pro from user column, got %q", got)
	}
}

func TestGetUserTier_BothQueriesErrorReturnsFree(t *testing.T) {
	// Defensive default: if both lookups error, return Free rather than
	// granting unintended access.
	userID := uuid.New()
	q := &stubQueries{tb: t,
		getSubscriptionByUserIDFn: func(ctx context.Context, _ uuid.UUID) (dbgen.Subscription, error) {
			return dbgen.Subscription{}, errors.New("db down")
		},
		getUserSubscriptionTierFn: func(ctx context.Context, _ uuid.UUID) (string, error) {
			return "", errors.New("db down")
		},
	}
	svc := newTestService(t, q, true)

	if got := svc.GetUserTier(context.Background(), userID); got != tier.Free {
		t.Errorf("expected Free when both lookups fail, got %q", got)
	}
}

func TestGetUserTier_ActiveButUnsupportedTierFallsBack(t *testing.T) {
	// The active-subscription branch returns its tier ONLY for
	// Explorer/Voyager. A subscriptions row with an unexpected tier
	// (e.g., a malformed migration) must not bypass the fallback.
	userID := uuid.New()
	q := &stubQueries{tb: t,
		getSubscriptionByUserIDFn: func(ctx context.Context, _ uuid.UUID) (dbgen.Subscription, error) {
			return dbgen.Subscription{Tier: "weird", Status: "active"}, nil
		},
		getUserSubscriptionTierFn: func(ctx context.Context, _ uuid.UUID) (string, error) {
			return string(tier.Free), nil
		},
	}
	svc := newTestService(t, q, true)

	if got := svc.GetUserTier(context.Background(), userID); got != tier.Free {
		t.Errorf("expected Free fallback for unsupported active tier, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// GetSubscription
// ---------------------------------------------------------------------------

func TestGetSubscription_FullMapping(t *testing.T) {
	userID := uuid.New()
	periodEnd := time.Now().Add(30 * 24 * time.Hour).UTC().Truncate(time.Second)
	q := &stubQueries{tb: t,
		getSubscriptionByUserIDFn: func(ctx context.Context, _ uuid.UUID) (dbgen.Subscription, error) {
			return dbgen.Subscription{
				Tier:                 string(tier.Explorer),
				Status:               "active",
				CancelAtPeriodEnd:    pgtype.Bool{Bool: true, Valid: true},
				BillingPeriod:        string(BillingPeriodAnnual),
				StripeCustomerID:     "cus_123",
				StripeSubscriptionID: pgtype.Text{String: "sub_123", Valid: true},
				CurrentPeriodEnd:     pgtype.Timestamptz{Time: periodEnd, Valid: true},
			}, nil
		},
	}
	svc := newTestService(t, q, true)

	got, err := svc.GetSubscription(context.Background(), userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil subscription")
	}
	if got.Tier != tier.Explorer {
		t.Errorf("Tier: got %q, want Explorer", got.Tier)
	}
	if !got.CancelAtPeriodEnd {
		t.Error("CancelAtPeriodEnd: expected true")
	}
	if got.BillingPeriod != BillingPeriodAnnual {
		t.Errorf("BillingPeriod: got %q, want annual", got.BillingPeriod)
	}
	if got.StripeCustomerID != "cus_123" {
		t.Errorf("StripeCustomerID: got %q, want cus_123", got.StripeCustomerID)
	}
	if got.StripeSubscriptionID != "sub_123" {
		t.Errorf("StripeSubscriptionID: got %q, want sub_123", got.StripeSubscriptionID)
	}
	if got.CurrentPeriodEnd == nil || !got.CurrentPeriodEnd.Equal(periodEnd) {
		t.Errorf("CurrentPeriodEnd: got %v, want %v", got.CurrentPeriodEnd, periodEnd)
	}
}

func TestGetSubscription_NoRowsReturnsNilNil(t *testing.T) {
	// Absence of a subscription row is not an error — caller gets
	// (nil, nil). The handler on the other side renders this as
	// "user is on the free tier" rather than 500'ing.
	userID := uuid.New()
	q := &stubQueries{tb: t,
		getSubscriptionByUserIDFn: func(ctx context.Context, _ uuid.UUID) (dbgen.Subscription, error) {
			return dbgen.Subscription{}, pgx.ErrNoRows
		},
	}
	svc := newTestService(t, q, true)

	got, err := svc.GetSubscription(context.Background(), userID)
	if err != nil {
		t.Errorf("expected nil error for no-row case, got %v", err)
	}
	if got != nil {
		t.Errorf("expected nil subscription, got %+v", got)
	}
}

// ---------------------------------------------------------------------------
// Disabled-mode error paths
// ---------------------------------------------------------------------------

func TestCreateCheckoutSession_DisabledReturnsError(t *testing.T) {
	q := &stubQueries{tb: t}
	svc := newTestService(t, q, false)
	_, err := svc.CreateCheckoutSession(context.Background(), uuid.New(), "u@example.com", tier.Explorer, false)
	if err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Errorf("expected 'not configured' error, got %v", err)
	}
}

func TestCreatePortalSession_DisabledReturnsError(t *testing.T) {
	q := &stubQueries{tb: t}
	svc := newTestService(t, q, false)
	_, err := svc.CreatePortalSession(context.Background(), uuid.New())
	if err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Errorf("expected 'not configured' error, got %v", err)
	}
}

func TestCancelSubscription_DisabledReturnsError(t *testing.T) {
	q := &stubQueries{tb: t}
	svc := newTestService(t, q, false)
	err := svc.CancelSubscription(context.Background(), uuid.New())
	if err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Errorf("expected 'not configured' error, got %v", err)
	}
}

func TestCancelSubscription_NoSubscriptionForUser(t *testing.T) {
	q := &stubQueries{tb: t,
		getSubscriptionByUserIDFn: func(ctx context.Context, _ uuid.UUID) (dbgen.Subscription, error) {
			return dbgen.Subscription{}, pgx.ErrNoRows
		},
	}
	svc := newTestService(t, q, true)
	err := svc.CancelSubscription(context.Background(), uuid.New())
	if err == nil || !strings.Contains(err.Error(), "no subscription") {
		t.Errorf("expected 'no subscription' error, got %v", err)
	}
}

func TestCancelSubscription_MissingStripeID(t *testing.T) {
	// Subscription row exists but stripe_subscription_id is null/empty
	// — could happen if a webhook never landed. Must NOT call Stripe.
	q := &stubQueries{tb: t,
		getSubscriptionByUserIDFn: func(ctx context.Context, _ uuid.UUID) (dbgen.Subscription, error) {
			return dbgen.Subscription{
				StripeSubscriptionID: pgtype.Text{Valid: false},
			}, nil
		},
	}
	svc := newTestService(t, q, true)
	err := svc.CancelSubscription(context.Background(), uuid.New())
	if err == nil || !strings.Contains(err.Error(), "no active stripe subscription") {
		t.Errorf("expected 'no active stripe subscription' error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// HandleWebhook routing
// ---------------------------------------------------------------------------

func TestHandleWebhook_UnknownEventTypeNoOps(t *testing.T) {
	// Stripe sends events we don't subscribe to (or new ones added in
	// future API versions). Must not error — the webhook handler
	// returns 200 OK to Stripe regardless.
	q := &stubQueries{tb: t} // no methods stubbed; would fail-loud if called
	svc := newTestService(t, q, true)
	err := svc.HandleWebhook(context.Background(), stripe.Event{Type: "customer.created"})
	if err != nil {
		t.Errorf("expected nil error for unknown event, got %v", err)
	}
}

func TestHandleWebhook_RoutesToSubscriptionDeleted(t *testing.T) {
	q := &stubQueries{tb: t,
		updateSubscriptionStatusFn: func(ctx context.Context, _ dbgen.UpdateSubscriptionStatusParams) error {
			return nil
		},
		getSubscriptionByStripeSubscriptionIDFn: func(ctx context.Context, _ pgtype.Text) (dbgen.Subscription, error) {
			return dbgen.Subscription{UserID: uuid.New()}, nil
		},
		setUserSubscriptionTierByIDFn: func(ctx context.Context, _ dbgen.SetUserSubscriptionTierByIDParams) error {
			return nil
		},
	}
	svc := newTestService(t, q, true)

	rawSub, _ := json.Marshal(stripe.Subscription{ID: "sub_routed"})
	err := svc.HandleWebhook(context.Background(), stripe.Event{
		Type: stripe.EventTypeCustomerSubscriptionDeleted,
		Data: &stripe.EventData{Raw: rawSub},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(q.updateStatusCalls) != 1 || q.updateStatusCalls[0].Status != "canceled" {
		t.Errorf("expected one canceled status update, got %+v", q.updateStatusCalls)
	}
}

// ---------------------------------------------------------------------------
// handleSubscriptionDeleted
// ---------------------------------------------------------------------------

func TestHandleSubscriptionDeleted_RevertsUserToFree(t *testing.T) {
	userID := uuid.New()
	q := &stubQueries{tb: t,
		updateSubscriptionStatusFn: func(ctx context.Context, _ dbgen.UpdateSubscriptionStatusParams) error {
			return nil
		},
		getSubscriptionByStripeSubscriptionIDFn: func(ctx context.Context, _ pgtype.Text) (dbgen.Subscription, error) {
			return dbgen.Subscription{UserID: userID}, nil
		},
		setUserSubscriptionTierByIDFn: func(ctx context.Context, _ dbgen.SetUserSubscriptionTierByIDParams) error {
			return nil
		},
	}
	svc := newTestService(t, q, true)

	rawSub, _ := json.Marshal(stripe.Subscription{ID: "sub_deleted"})
	err := svc.handleSubscriptionDeleted(context.Background(), stripe.Event{
		Data: &stripe.EventData{Raw: rawSub},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// User reverted to Free.
	if len(q.setUserSubscriptionTierByIDCalls) != 1 {
		t.Fatalf("expected 1 SetUserSubscriptionTierByID call, got %d", len(q.setUserSubscriptionTierByIDCalls))
	}
	got := q.setUserSubscriptionTierByIDCalls[0]
	if got.Tier != string(tier.Free) {
		t.Errorf("expected user tier reverted to Free, got %q", got.Tier)
	}
	if got.UserID != userID {
		t.Errorf("expected user_id=%s, got %s", userID, got.UserID)
	}
}

func TestHandleSubscriptionDeleted_MalformedJSONReturnsError(t *testing.T) {
	q := &stubQueries{tb: t}
	svc := newTestService(t, q, true)
	err := svc.handleSubscriptionDeleted(context.Background(), stripe.Event{
		Data: &stripe.EventData{Raw: []byte("not-json")},
	})
	if err == nil || !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("expected unmarshal error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// handleSubscriptionUpdated — the biggest, most-branching method
// ---------------------------------------------------------------------------

func TestHandleSubscriptionUpdated_StatusAndCancelAtPeriodEnd(t *testing.T) {
	q := &stubQueries{tb: t,
		updateSubscriptionStatusFn:         func(ctx context.Context, _ dbgen.UpdateSubscriptionStatusParams) error { return nil },
		setSubscriptionCancelAtPeriodEndFn: func(ctx context.Context, _ dbgen.SetSubscriptionCancelAtPeriodEndParams) error { return nil },
	}
	svc := newTestService(t, q, true)

	rawSub, _ := json.Marshal(stripe.Subscription{
		ID:                "sub_updated",
		Status:            stripe.SubscriptionStatusActive,
		CancelAtPeriodEnd: true,
		// Items deliberately nil → the period/billing/tier branches
		// short-circuit. We only want to assert status + cancel here.
	})
	err := svc.handleSubscriptionUpdated(context.Background(), stripe.Event{
		Data: &stripe.EventData{Raw: rawSub},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(q.updateStatusCalls) != 1 || q.updateStatusCalls[0].Status != string(stripe.SubscriptionStatusActive) {
		t.Errorf("expected one status update with 'active', got %+v", q.updateStatusCalls)
	}
	if len(q.setCancelAtPeriodEndCalls) != 1 {
		t.Fatalf("expected 1 SetSubscriptionCancelAtPeriodEnd call, got %d", len(q.setCancelAtPeriodEndCalls))
	}
	got := q.setCancelAtPeriodEndCalls[0]
	if !got.CancelAtPeriodEnd.Bool {
		t.Error("expected CancelAtPeriodEnd=true")
	}
}

func TestHandleSubscriptionUpdated_RevertsUserOnCanceled(t *testing.T) {
	// Subscription transitioned to a non-active terminal state — user
	// tier must be reset to Free even though the subscription row itself
	// stays put. The handler at service.go:~500 treats `Canceled` and
	// `Unpaid` identically (the standard Stripe dunning trail is
	// active → past_due → unpaid → canceled), so both must trigger the
	// revert. Parameterized so a refactor that drops one branch fails
	// loudly. (W1 from the PR #424 adversarial review.)
	terminalStatuses := []stripe.SubscriptionStatus{
		stripe.SubscriptionStatusCanceled,
		stripe.SubscriptionStatusUnpaid,
	}

	for _, status := range terminalStatuses {
		t.Run(string(status), func(t *testing.T) {
			userID := uuid.New()
			q := &stubQueries{tb: t,
				updateSubscriptionStatusFn:         func(ctx context.Context, _ dbgen.UpdateSubscriptionStatusParams) error { return nil },
				setSubscriptionCancelAtPeriodEndFn: func(ctx context.Context, _ dbgen.SetSubscriptionCancelAtPeriodEndParams) error { return nil },
				getSubscriptionByStripeSubscriptionIDFn: func(ctx context.Context, _ pgtype.Text) (dbgen.Subscription, error) {
					return dbgen.Subscription{UserID: userID}, nil
				},
				setUserSubscriptionTierByIDFn: func(ctx context.Context, _ dbgen.SetUserSubscriptionTierByIDParams) error {
					return nil
				},
			}
			svc := newTestService(t, q, true)

			rawSub, _ := json.Marshal(stripe.Subscription{
				ID:     "sub_" + string(status),
				Status: status,
			})
			if err := svc.handleSubscriptionUpdated(context.Background(), stripe.Event{Data: &stripe.EventData{Raw: rawSub}}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(q.setUserSubscriptionTierByIDCalls) != 1 {
				t.Fatalf("status %s: expected 1 SetUserSubscriptionTierByID call (revert to Free), got %d",
					status, len(q.setUserSubscriptionTierByIDCalls))
			}
			got := q.setUserSubscriptionTierByIDCalls[0]
			if got.Tier != string(tier.Free) || got.UserID != userID {
				t.Errorf("status %s: expected revert to Free for user %s, got tier=%q user=%s",
					status, userID, got.Tier, got.UserID)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// handleCheckoutCompleted — subscription mode happy path with mocked Stripe
// ---------------------------------------------------------------------------

func TestHandleCheckoutCompleted_SubscriptionModeCreatesRow(t *testing.T) {
	userID := uuid.New()
	subID := "sub_xyz"
	customerID := "cus_xyz"

	// Mock Stripe: Subscriptions.Retrieve must return an item with a
	// recurring price so the handler can extract the period/interval/tier.
	periodStart := time.Now().Add(-time.Hour).Unix()
	periodEnd := time.Now().Add(30 * 24 * time.Hour).Unix()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/v1/subscriptions/"+subID) {
			t.Errorf("unexpected stripe path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":     subID,
			"object": "subscription",
			"items": map[string]any{
				"object": "list",
				"data": []map[string]any{
					{
						"id":                   "si_x",
						"object":               "subscription_item",
						"current_period_start": periodStart,
						"current_period_end":   periodEnd,
						"price": map[string]any{
							"id":     "price_explorer_monthly",
							"object": "price",
							"recurring": map[string]any{
								"interval":       "month",
								"interval_count": 1,
							},
						},
					},
				},
			},
		})
	}))
	defer server.Close()

	q := &stubQueries{tb: t,
		createSubscriptionFn: func(ctx context.Context, arg dbgen.CreateSubscriptionParams) (dbgen.Subscription, error) {
			return dbgen.Subscription{ID: uuid.New()}, nil
		},
		setUserSubscriptionTierByIDFn: func(ctx context.Context, _ dbgen.SetUserSubscriptionTierByIDParams) error {
			return nil
		},
	}
	svc := newTestService(t, q, true)
	svc.client = stripeClientWithBaseURL(server.URL)

	session := stripe.CheckoutSession{
		Mode: stripe.CheckoutSessionModeSubscription,
		Metadata: map[string]string{
			"user_id":  userID.String(),
			"tier":     string(tier.Explorer),
			"interval": "monthly",
		},
		Subscription: &stripe.Subscription{ID: subID},
		Customer:     &stripe.Customer{ID: customerID},
	}
	rawSession, _ := json.Marshal(session)
	err := svc.handleCheckoutCompleted(context.Background(), stripe.Event{
		Data: &stripe.EventData{Raw: rawSession},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(q.createSubscriptionCalls) != 1 {
		t.Fatalf("expected 1 CreateSubscription call, got %d", len(q.createSubscriptionCalls))
	}
	cs := q.createSubscriptionCalls[0]
	if cs.UserID != userID {
		t.Errorf("UserID: got %s, want %s", cs.UserID, userID)
	}
	if cs.Tier != string(tier.Explorer) {
		t.Errorf("Tier: got %q, want Explorer", cs.Tier)
	}
	if cs.Status != "active" {
		t.Errorf("Status: got %q, want active", cs.Status)
	}
	if cs.StripeCustomerID != customerID {
		t.Errorf("StripeCustomerID: got %q, want %q", cs.StripeCustomerID, customerID)
	}
	if !cs.StripeSubscriptionID.Valid || cs.StripeSubscriptionID.String != subID {
		t.Errorf("StripeSubscriptionID: got %v, want %s", cs.StripeSubscriptionID, subID)
	}
	if cs.BillingPeriod != string(BillingPeriodMonthly) {
		t.Errorf("BillingPeriod: got %q, want monthly", cs.BillingPeriod)
	}

	// User tier column also updated.
	if len(q.setUserSubscriptionTierByIDCalls) != 1 {
		t.Errorf("expected 1 SetUserSubscriptionTierByID call, got %d", len(q.setUserSubscriptionTierByIDCalls))
	}
}

func TestHandleCheckoutCompleted_PaymentModeWithoutPaymentSvcErrors(t *testing.T) {
	// mode=payment routes to handlePaymentCheckoutCompleted, which
	// requires a payment service. Without one wired, must error rather
	// than silently dropping the Trip Pro purchase event.
	q := &stubQueries{tb: t}
	svc := newTestService(t, q, true)

	session := stripe.CheckoutSession{
		Mode: stripe.CheckoutSessionModePayment,
		Metadata: map[string]string{
			"user_id": uuid.New().String(),
			"trip_id": uuid.New().String(),
		},
	}
	rawSession, _ := json.Marshal(session)
	err := svc.handleCheckoutCompleted(context.Background(), stripe.Event{
		Data: &stripe.EventData{Raw: rawSession},
	})
	if err == nil || !strings.Contains(err.Error(), "payment service not configured") {
		t.Errorf("expected payment-service-not-configured error, got %v", err)
	}
}

func TestHandleCheckoutCompleted_UnknownModeNoOps(t *testing.T) {
	q := &stubQueries{tb: t}
	svc := newTestService(t, q, true)
	session := stripe.CheckoutSession{Mode: "bogus_mode"}
	rawSession, _ := json.Marshal(session)
	err := svc.handleCheckoutCompleted(context.Background(), stripe.Event{
		Data: &stripe.EventData{Raw: rawSession},
	})
	if err != nil {
		t.Errorf("expected nil error for unknown mode, got %v", err)
	}
}

func TestHandleCheckoutCompleted_MissingUserIDInMetadata(t *testing.T) {
	q := &stubQueries{tb: t}
	svc := newTestService(t, q, true)
	session := stripe.CheckoutSession{
		Mode:     stripe.CheckoutSessionModeSubscription,
		Metadata: map[string]string{}, // no user_id
	}
	rawSession, _ := json.Marshal(session)
	err := svc.handleCheckoutCompleted(context.Background(), stripe.Event{
		Data: &stripe.EventData{Raw: rawSession},
	})
	if err == nil || !strings.Contains(err.Error(), "user_id") {
		t.Errorf("expected user_id missing error, got %v", err)
	}
}

func TestHandleCheckoutCompleted_BadTierInMetadata(t *testing.T) {
	q := &stubQueries{tb: t}
	svc := newTestService(t, q, true)
	session := stripe.CheckoutSession{
		Mode: stripe.CheckoutSessionModeSubscription,
		Metadata: map[string]string{
			"user_id": uuid.New().String(),
			"tier":    "free", // invalid for subscriptions
		},
	}
	rawSession, _ := json.Marshal(session)
	err := svc.handleCheckoutCompleted(context.Background(), stripe.Event{
		Data: &stripe.EventData{Raw: rawSession},
	})
	if err == nil || !strings.Contains(err.Error(), "tier") {
		t.Errorf("expected unsupported-tier error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// handlePaymentCheckoutCompleted
// ---------------------------------------------------------------------------

func TestHandlePaymentCheckoutCompleted_NoPaymentSvcErrors(t *testing.T) {
	q := &stubQueries{tb: t}
	svc := newTestService(t, q, true)
	err := svc.handlePaymentCheckoutCompleted(context.Background(), stripe.CheckoutSession{
		Metadata: map[string]string{
			"user_id": uuid.New().String(),
			"trip_id": uuid.New().String(),
		},
	})
	if err == nil || !strings.Contains(err.Error(), "payment service not configured") {
		t.Errorf("expected error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// CreateCheckoutSession happy path with mocked Stripe — exercises
// getOrCreateCustomer + V1CheckoutSessions.Create end-to-end
// ---------------------------------------------------------------------------

func TestCreateCheckoutSession_HappyPath_NewCustomer(t *testing.T) {
	userID := uuid.New()

	// Mock Stripe: Customers.Create then CheckoutSessions.Create. The
	// order depends on path — getOrCreateCustomer hits Customers, then
	// the main flow hits CheckoutSessions.
	type captured struct {
		customerEmail    string
		checkoutCustomer string
		checkoutMode     string
		checkoutMetaTier string
	}
	var got captured

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(body))

		switch {
		case strings.HasPrefix(r.URL.Path, "/v1/customers"):
			form := string(body)
			if strings.Contains(form, "email=u%40example.com") {
				got.customerEmail = "u@example.com"
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":     "cus_new",
				"object": "customer",
			})
		case strings.HasPrefix(r.URL.Path, "/v1/checkout/sessions"):
			form := string(body)
			if strings.Contains(form, "customer=cus_new") {
				got.checkoutCustomer = "cus_new"
			}
			if strings.Contains(form, "mode=subscription") {
				got.checkoutMode = "subscription"
			}
			if strings.Contains(form, "metadata[tier]=explorer") {
				got.checkoutMetaTier = "explorer"
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":     "cs_new",
				"object": "checkout.session",
				"url":    "https://stripe.test/redirect",
			})
		default:
			t.Errorf("unexpected stripe path: %s", r.URL.Path)
			http.Error(w, "unexpected", http.StatusBadRequest)
		}
	}))
	defer server.Close()

	q := &stubQueries{tb: t,
		// First call (in getOrCreateCustomer) returns no rows so the
		// service falls through to creating a new Stripe customer.
		getSubscriptionByUserIDFn: func(ctx context.Context, _ uuid.UUID) (dbgen.Subscription, error) {
			return dbgen.Subscription{}, pgx.ErrNoRows
		},
	}
	svc := newTestService(t, q, true)
	svc.client = stripeClientWithBaseURL(server.URL)

	url, err := svc.CreateCheckoutSession(context.Background(), userID, "u@example.com", tier.Explorer, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "https://stripe.test/redirect" {
		t.Errorf("expected redirect URL, got %q", url)
	}
	if got.customerEmail != "u@example.com" {
		t.Errorf("Stripe customer create not seen with expected email: %+v", got)
	}
	if got.checkoutCustomer != "cus_new" {
		t.Errorf("Stripe checkout not seen with new customer ID: %+v", got)
	}
	if got.checkoutMode != "subscription" {
		t.Errorf("Stripe checkout mode: %+v", got)
	}
	if got.checkoutMetaTier != "explorer" {
		t.Errorf("Stripe checkout metadata.tier: %+v", got)
	}
}

func TestCreateCheckoutSession_ExistingCustomerReused(t *testing.T) {
	// If a subscription row already has a Stripe customer ID, we MUST
	// reuse it rather than creating a duplicate Stripe customer
	// (Stripe-side de-dupe is best-effort only).
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v1/customers") {
			t.Errorf("unexpected Stripe Customers call — should reuse existing customer")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":     "cs_x",
			"object": "checkout.session",
			"url":    "https://stripe.test/redirect",
		})
	}))
	defer server.Close()

	q := &stubQueries{tb: t,
		getSubscriptionByUserIDFn: func(ctx context.Context, _ uuid.UUID) (dbgen.Subscription, error) {
			return dbgen.Subscription{StripeCustomerID: "cus_existing"}, nil
		},
	}
	svc := newTestService(t, q, true)
	svc.client = stripeClientWithBaseURL(server.URL)

	if _, err := svc.CreateCheckoutSession(context.Background(), uuid.New(), "u@example.com", tier.Voyager, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateCheckoutSession_UnconfiguredPriceErrors(t *testing.T) {
	// Service with no price IDs → resolvePriceID returns an error and
	// the handler rejects the request before touching Stripe at all.
	q := &stubQueries{tb: t}
	svc := newTestService(t, q, true)
	svc.prices = ProductConfig{} // wipe the test defaults

	_, err := svc.CreateCheckoutSession(context.Background(), uuid.New(), "u@example.com", tier.Explorer, false)
	if err == nil || !strings.Contains(err.Error(), "price ID not configured") {
		t.Errorf("expected price-not-configured error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// CreatePortalSession happy path
// ---------------------------------------------------------------------------

func TestCreatePortalSession_HappyPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/v1/billing_portal/sessions") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "customer=cus_existing") {
			t.Errorf("portal session request missing existing customer ID: %s", body)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":     "bps_x",
			"object": "billing_portal.session",
			"url":    "https://stripe.test/portal",
		})
	}))
	defer server.Close()

	q := &stubQueries{tb: t,
		getSubscriptionByUserIDFn: func(ctx context.Context, _ uuid.UUID) (dbgen.Subscription, error) {
			return dbgen.Subscription{StripeCustomerID: "cus_existing"}, nil
		},
	}
	svc := newTestService(t, q, true)
	svc.client = stripeClientWithBaseURL(server.URL)

	url, err := svc.CreatePortalSession(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "https://stripe.test/portal" {
		t.Errorf("expected portal URL, got %q", url)
	}
}

func TestCreatePortalSession_NoSubscriptionForUser(t *testing.T) {
	q := &stubQueries{tb: t,
		getSubscriptionByUserIDFn: func(ctx context.Context, _ uuid.UUID) (dbgen.Subscription, error) {
			return dbgen.Subscription{}, pgx.ErrNoRows
		},
	}
	svc := newTestService(t, q, true)
	_, err := svc.CreatePortalSession(context.Background(), uuid.New())
	if err == nil || !strings.Contains(err.Error(), "no subscription") {
		t.Errorf("expected no-subscription error, got %v", err)
	}
}
