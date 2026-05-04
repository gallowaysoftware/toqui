package payment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stripe/stripe-go/v82"

	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
)

// stubQueries is a hand-rolled test double for paymentQueries that lets each
// test inject canned responses or function bodies for the methods it cares
// about.
//
// Fail-loud defaults. Each method, when called without an injected `*Fn`,
// calls `tb.Fatalf("unexpected stubQueries.X call ...")` — the test fails
// with a precise message identifying which method was called by surprise.
//
// Why fail-loud instead of return zero-value? An earlier draft returned
// zero values from unconfigured methods; the adversarial review on
// PR #418 (the original 0% → 98.2% coverage PR) flagged the hazard:
// `GetTripByID` returning `dbgen.Trip{}, nil` when unconfigured silently
// passes the ownership check (real DB returns `pgx.ErrNoRows`, which the
// production code converts to `ErrNotTripOwner`). A future test that
// forgot to set `getTripByIDFn` while exercising `InitializeCheckout`
// would silently green-pass with zero-UUID trip data — masking the bug.
// Fail-loud forces every test to make its expectations explicit. (W2 in
// the review write-up.)
type stubQueries struct {
	tb                            testing.TB // for fail-loud defaults
	getTripByIDFn                 func(ctx context.Context, arg dbgen.GetTripByIDParams) (dbgen.Trip, error)
	isTripUnlockedFn              func(ctx context.Context, arg dbgen.IsTripUnlockedParams) (bool, error)
	createCheckoutSessionFn       func(ctx context.Context, arg dbgen.CreateCheckoutSessionParams) (dbgen.CheckoutSession, error)
	createPaymentFn               func(ctx context.Context, arg dbgen.CreatePaymentParams) (dbgen.Payment, error)
	createTripUnlockFn            func(ctx context.Context, arg dbgen.CreateTripUnlockParams) (dbgen.TripUnlock, error)
	markCheckoutSessionCompleteFn func(ctx context.Context, checkoutToken string) error

	// Captured calls for assertions.
	createPaymentCalls    []dbgen.CreatePaymentParams
	createTripUnlockCalls []dbgen.CreateTripUnlockParams
	markCompleteCalls     []string
}

func (s *stubQueries) GetTripByID(ctx context.Context, arg dbgen.GetTripByIDParams) (dbgen.Trip, error) {
	if s.getTripByIDFn != nil {
		return s.getTripByIDFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubQueries.GetTripByID(%+v) — set getTripByIDFn", arg)
	return dbgen.Trip{}, nil
}

func (s *stubQueries) IsTripUnlocked(ctx context.Context, arg dbgen.IsTripUnlockedParams) (bool, error) {
	if s.isTripUnlockedFn != nil {
		return s.isTripUnlockedFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubQueries.IsTripUnlocked(%+v) — set isTripUnlockedFn", arg)
	return false, nil
}

func (s *stubQueries) CreateCheckoutSession(ctx context.Context, arg dbgen.CreateCheckoutSessionParams) (dbgen.CheckoutSession, error) {
	if s.createCheckoutSessionFn != nil {
		return s.createCheckoutSessionFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubQueries.CreateCheckoutSession(%+v) — set createCheckoutSessionFn", arg)
	return dbgen.CheckoutSession{}, nil
}

func (s *stubQueries) CreatePayment(ctx context.Context, arg dbgen.CreatePaymentParams) (dbgen.Payment, error) {
	s.createPaymentCalls = append(s.createPaymentCalls, arg)
	if s.createPaymentFn != nil {
		return s.createPaymentFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubQueries.CreatePayment(%+v) — set createPaymentFn", arg)
	return dbgen.Payment{}, nil
}

func (s *stubQueries) CreateTripUnlock(ctx context.Context, arg dbgen.CreateTripUnlockParams) (dbgen.TripUnlock, error) {
	s.createTripUnlockCalls = append(s.createTripUnlockCalls, arg)
	if s.createTripUnlockFn != nil {
		return s.createTripUnlockFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubQueries.CreateTripUnlock(%+v) — set createTripUnlockFn", arg)
	return dbgen.TripUnlock{}, nil
}

func (s *stubQueries) MarkCheckoutSessionComplete(ctx context.Context, checkoutToken string) error {
	s.markCompleteCalls = append(s.markCompleteCalls, checkoutToken)
	if s.markCheckoutSessionCompleteFn != nil {
		return s.markCheckoutSessionCompleteFn(ctx, checkoutToken)
	}
	s.tb.Fatalf("unexpected stubQueries.MarkCheckoutSessionComplete(%q) — set markCheckoutSessionCompleteFn", checkoutToken)
	return nil
}

// recordingTracker captures Track() calls for analytics assertions. Mirrors
// the pattern from internal/handlers/tool_recommend_booking_test.go.
type recordingTracker struct {
	events []recordedEvent
}

type recordedEvent struct {
	userID     string
	event      string
	properties map[string]any
}

func (r *recordingTracker) Track(userID, event string, properties map[string]any) {
	r.events = append(r.events, recordedEvent{userID: userID, event: event, properties: properties})
}

// newTestService builds a Service via the real NewService constructor so
// constructor-side logic — `enabled = stripeKey != ""`, the structured
// log, future feature-flag wiring — is exercised by every test that goes
// through this helper. Then it swaps in the stub for `queries` (the only
// field that NewService can't accept directly because its signature
// type-locks to `*dbgen.Queries`).
//
// `enabled` is plumbed via the stripeKey: when true, we pass a dummy
// non-empty key so NewService takes the enabled branch and constructs a
// real `*stripe.Client`. Tests that need to override the client (e.g. the
// httptest.Server-based happy-path test) replace `svc.client` after
// construction.
//
// W3 from the PR #418 adversarial review: an earlier draft built the
// struct literal directly, bypassing NewService — meaning a future
// constructor change (feature flag, env-var read, etc.) would silently
// not be exercised by 80% of these tests.
func newTestService(t *testing.T, q paymentQueries, enabled bool) *Service {
	t.Helper()
	stripeKey := ""
	if enabled {
		stripeKey = "sk_test_dummy"
	}
	svc := NewService(stripeKey, "prod_test", 1900, nil, "https://app.toqui.test")
	svc.queries = q
	return svc
}

// --- NewService + disabled mode ---

func TestNewService_DisabledModeWhenKeyEmpty(t *testing.T) {
	// Passing an empty stripeKey must produce a Service with enabled=false
	// and a nil Stripe client. The package contract is that disabled mode
	// is a first-class state — IsTripUnlocked still works, but checkout is
	// gated. nil queries is fine here because we don't call any DB methods.
	svc := NewService("", "prod_test", 1900, nil, "https://app.toqui.test")
	if svc == nil {
		t.Fatal("NewService returned nil")
	}
	if svc.enabled {
		t.Error("expected enabled=false when stripeKey is empty")
	}
	if svc.client != nil {
		t.Error("expected nil stripe client in disabled mode")
	}
	if svc.PriceCents() != 1900 {
		t.Errorf("expected priceCents=1900, got %d", svc.PriceCents())
	}
}

func TestNewService_EnabledWhenKeyPresent(t *testing.T) {
	svc := NewService("sk_test_dummy", "prod_test", 1900, nil, "https://app.toqui.test")
	if !svc.enabled {
		t.Error("expected enabled=true when stripeKey is set")
	}
	if svc.client == nil {
		t.Error("expected non-nil stripe client when enabled")
	}
}

func TestInitializeCheckout_DisabledReturnsError(t *testing.T) {
	// Disabled-mode checkout must fail with a clean "stripe is not
	// configured" error so handlers can map it to a meaningful response
	// instead of nil-pointer-panicking on s.client.
	svc := newTestService(t, &stubQueries{tb: t}, false)

	_, err := svc.InitializeCheckout(context.Background(), uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("expected error in disabled mode")
	}
	if !strings.Contains(err.Error(), "not configured") {
		t.Errorf("expected 'not configured' error, got %v", err)
	}
}

func TestInitializeCheckout_MissingProductID(t *testing.T) {
	// Build via newTestService (so constructor logic runs), then zero out
	// productID — NewService doesn't validate productID at construction
	// (the validation lives in InitializeCheckoutWithPrice), so this is
	// representative of the "key set but product unconfigured" misconfig
	// path we actually care about.
	svc := newTestService(t, &stubQueries{tb: t}, true)
	svc.productID = ""

	_, err := svc.InitializeCheckout(context.Background(), uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("expected error when productID is empty")
	}
	if !strings.Contains(err.Error(), "STRIPE_TRIP_PRO_PRODUCT_ID") {
		t.Errorf("expected error to reference STRIPE_TRIP_PRO_PRODUCT_ID, got %v", err)
	}
}

// --- IsTripUnlocked + alwaysUnlocked ---

func TestIsTripUnlocked_AlwaysUnlockedShortCircuits(t *testing.T) {
	// alwaysUnlocked (staging) must short-circuit BEFORE hitting the DB.
	// We prove it by giving the stub a function that fails the test if
	// called — alwaysUnlocked should never reach it.
	q := &stubQueries{tb: t,
		isTripUnlockedFn: func(ctx context.Context, arg dbgen.IsTripUnlockedParams) (bool, error) {
			t.Errorf("IsTripUnlocked DB query should not be called when alwaysUnlocked=true")
			return false, nil
		},
	}
	svc := newTestService(t, q, true)
	svc.SetAlwaysUnlocked(true)

	got, err := svc.IsTripUnlocked(context.Background(), uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got {
		t.Error("expected true when alwaysUnlocked is set")
	}
}

func TestIsTripUnlocked_DelegatesToQueriesWhenNotAlwaysUnlocked(t *testing.T) {
	userID := uuid.New()
	tripID := uuid.New()
	called := false
	q := &stubQueries{tb: t,
		isTripUnlockedFn: func(ctx context.Context, arg dbgen.IsTripUnlockedParams) (bool, error) {
			called = true
			if arg.UserID != userID || arg.TripID != tripID {
				t.Errorf("unexpected args: got user=%s trip=%s want user=%s trip=%s", arg.UserID, arg.TripID, userID, tripID)
			}
			return true, nil
		},
	}
	svc := newTestService(t, q, true)

	got, err := svc.IsTripUnlocked(context.Background(), userID, tripID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got {
		t.Error("expected DB result to propagate (true)")
	}
	if !called {
		t.Error("expected IsTripUnlocked to delegate to queries")
	}
}

func TestIsTripUnlocked_PropagatesQueryError(t *testing.T) {
	wantErr := errors.New("db down")
	q := &stubQueries{tb: t,
		isTripUnlockedFn: func(ctx context.Context, arg dbgen.IsTripUnlockedParams) (bool, error) {
			return false, wantErr
		},
	}
	svc := newTestService(t, q, true)

	_, err := svc.IsTripUnlocked(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, wantErr) {
		t.Errorf("expected wantErr to propagate, got %v", err)
	}
}

// --- InitializeCheckout ownership gating (#361 P1) ---

func TestInitializeCheckout_NotOwnerReturnsErrNotTripOwner(t *testing.T) {
	// pgx.ErrNoRows from GetTripByID means the user doesn't own this trip
	// (the query filters on user_id). The service must convert that into
	// the ErrNotTripOwner sentinel so handlers can map to PermissionDenied
	// rather than InternalError. This is the #361 regression test —
	// previously the code went straight to IsTripUnlocked, leaking
	// other-user trip IDs into Stripe sessions.
	q := &stubQueries{tb: t,
		getTripByIDFn: func(ctx context.Context, arg dbgen.GetTripByIDParams) (dbgen.Trip, error) {
			return dbgen.Trip{}, pgx.ErrNoRows
		},
	}
	svc := newTestService(t, q, true)

	_, err := svc.InitializeCheckout(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, ErrNotTripOwner) {
		t.Fatalf("expected ErrNotTripOwner, got %v", err)
	}
}

func TestInitializeCheckout_OtherDBErrorPropagatesWrapped(t *testing.T) {
	wantErr := errors.New("connection refused")
	q := &stubQueries{tb: t,
		getTripByIDFn: func(ctx context.Context, arg dbgen.GetTripByIDParams) (dbgen.Trip, error) {
			return dbgen.Trip{}, wantErr
		},
	}
	svc := newTestService(t, q, true)

	_, err := svc.InitializeCheckout(context.Background(), uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
	if errors.Is(err, ErrNotTripOwner) {
		t.Error("non-pgx.ErrNoRows DB errors must NOT be conflated with ownership failure")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("expected wrapped wantErr, got %v", err)
	}
	if !strings.Contains(err.Error(), "check trip ownership") {
		t.Errorf("expected error to mention 'check trip ownership', got %v", err)
	}
}

// --- InitializeCheckout already-unlocked ---

func TestInitializeCheckout_AlreadyUnlockedReturnsError(t *testing.T) {
	// When IsTripUnlocked returns true, we must NOT create a Stripe
	// session — the user already paid. Returning an error short-circuits
	// the handler and prevents a duplicate charge attempt.
	q := &stubQueries{tb: t,
		getTripByIDFn: func(ctx context.Context, arg dbgen.GetTripByIDParams) (dbgen.Trip, error) {
			return dbgen.Trip{ID: arg.ID, UserID: arg.UserID}, nil
		},
		isTripUnlockedFn: func(ctx context.Context, arg dbgen.IsTripUnlockedParams) (bool, error) {
			return true, nil
		},
		createCheckoutSessionFn: func(ctx context.Context, arg dbgen.CreateCheckoutSessionParams) (dbgen.CheckoutSession, error) {
			t.Error("CreateCheckoutSession must not be called when trip is already unlocked")
			return dbgen.CheckoutSession{}, nil
		},
	}
	svc := newTestService(t, q, true)

	_, err := svc.InitializeCheckout(context.Background(), uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("expected error when trip already unlocked")
	}
	if !strings.Contains(err.Error(), "already unlocked") {
		t.Errorf("expected 'already unlocked' error, got %v", err)
	}
}

func TestInitializeCheckout_IsTripUnlockedErrorPropagates(t *testing.T) {
	wantErr := errors.New("db down")
	q := &stubQueries{tb: t,
		getTripByIDFn: func(ctx context.Context, arg dbgen.GetTripByIDParams) (dbgen.Trip, error) {
			return dbgen.Trip{ID: arg.ID, UserID: arg.UserID}, nil
		},
		isTripUnlockedFn: func(ctx context.Context, arg dbgen.IsTripUnlockedParams) (bool, error) {
			return false, wantErr
		},
	}
	svc := newTestService(t, q, true)

	_, err := svc.InitializeCheckout(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, wantErr) {
		t.Errorf("expected wantErr, got %v", err)
	}
}

// --- InitializeCheckout happy path with a fake Stripe HTTP backend ---

// fakeStripeServer returns an httptest.Server that imitates the subset of
// the Stripe API the service touches: POST /v1/checkout/sessions returning
// a minimal Session payload. The captured request lets the test assert the
// outbound params (currency, mode, metadata, URLs, line items).
//
// We intentionally do NOT validate the form body's HMAC signature or any
// other Stripe-specific framing — we're testing the toqui service, not the
// Stripe SDK. The assertions below check the parameters our code sets, not
// every field the SDK serializes.
func fakeStripeServer(t *testing.T, capture *url.Values) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/v1/checkout/sessions") {
			t.Errorf("unexpected stripe path: %s", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		*capture = r.PostForm
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":  "cs_test_abc123",
			"url": "https://checkout.stripe.com/c/pay/cs_test_abc123",
		})
	}))
}

// stripeClientWithBaseURL builds a stripe.Client that points at the given
// base URL via a custom Backends config. The dummy key is fine — our fake
// server doesn't validate it.
func stripeClientWithBaseURL(baseURL string) *stripe.Client {
	cfg := &stripe.BackendConfig{URL: stripe.String(baseURL)}
	backends := &stripe.Backends{
		API:     stripe.GetBackendWithConfig(stripe.APIBackend, cfg),
		Connect: stripe.GetBackendWithConfig(stripe.ConnectBackend, cfg),
		Uploads: stripe.GetBackendWithConfig(stripe.UploadsBackend, cfg),
	}
	return stripe.NewClient("sk_test_dummy", stripe.WithBackends(backends))
}

func TestInitializeCheckout_HappyPathSendsExpectedStripeParams(t *testing.T) {
	userID := uuid.New()
	tripID := uuid.New()

	var captureCheckout dbgen.CreateCheckoutSessionParams
	q := &stubQueries{tb: t,
		getTripByIDFn: func(ctx context.Context, arg dbgen.GetTripByIDParams) (dbgen.Trip, error) {
			return dbgen.Trip{ID: arg.ID, UserID: arg.UserID}, nil
		},
		isTripUnlockedFn: func(ctx context.Context, arg dbgen.IsTripUnlockedParams) (bool, error) {
			return false, nil
		},
		createCheckoutSessionFn: func(ctx context.Context, arg dbgen.CreateCheckoutSessionParams) (dbgen.CheckoutSession, error) {
			captureCheckout = arg
			return dbgen.CheckoutSession{}, nil
		},
	}

	var stripeForm url.Values
	server := fakeStripeServer(t, &stripeForm)
	defer server.Close()

	svc := newTestService(t, q, true)
	svc.client = stripeClientWithBaseURL(server.URL)

	result, err := svc.InitializeCheckout(context.Background(), userID, tripID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.URL == "" {
		t.Fatalf("expected non-empty CheckoutResult URL, got %+v", result)
	}
	if !strings.Contains(result.URL, "checkout.stripe.com") {
		t.Errorf("expected stripe checkout URL, got %q", result.URL)
	}

	// Assert the params we send to Stripe. CAD currency, payment mode,
	// trip-pro metadata, and the trip-scoped success/cancel URLs are
	// load-bearing for the conversion funnel — a regression here silently
	// breaks revenue.
	if got := stripeForm.Get("mode"); got != string(stripe.CheckoutSessionModePayment) {
		t.Errorf("expected mode=payment, got %q", got)
	}
	if got := stripeForm.Get("line_items[0][price_data][currency]"); got != "cad" {
		t.Errorf("expected currency=cad, got %q", got)
	}
	if got := stripeForm.Get("line_items[0][price_data][unit_amount]"); got != "1900" {
		t.Errorf("expected unit_amount=1900, got %q", got)
	}
	if got := stripeForm.Get("line_items[0][price_data][product]"); got != "prod_test" {
		t.Errorf("expected product=prod_test, got %q", got)
	}
	if got := stripeForm.Get("metadata[user_id]"); got != userID.String() {
		t.Errorf("expected metadata[user_id]=%s, got %q", userID, got)
	}
	if got := stripeForm.Get("metadata[trip_id]"); got != tripID.String() {
		t.Errorf("expected metadata[trip_id]=%s, got %q", tripID, got)
	}
	if got := stripeForm.Get("metadata[type]"); got != "trip_pro" {
		t.Errorf("expected metadata[type]=trip_pro, got %q", got)
	}
	wantSuccess := fmt.Sprintf("https://app.toqui.test/trips/%s?payment=success", tripID)
	if got := stripeForm.Get("success_url"); got != wantSuccess {
		t.Errorf("expected success_url=%s, got %q", wantSuccess, got)
	}
	wantCancel := fmt.Sprintf("https://app.toqui.test/trips/%s?payment=canceled", tripID)
	if got := stripeForm.Get("cancel_url"); got != wantCancel {
		t.Errorf("expected cancel_url=%s, got %q", wantCancel, got)
	}
	if got := stripeForm.Get("allow_promotion_codes"); got != "true" {
		t.Errorf("expected allow_promotion_codes=true, got %q", got)
	}

	// Assert we also persisted the session to checkout_sessions with the
	// Stripe session ID and CAD currency / int32 amount cents.
	if captureCheckout.UserID != userID {
		t.Errorf("expected CheckoutSession.UserID=%s, got %s", userID, captureCheckout.UserID)
	}
	if captureCheckout.TripID != tripID {
		t.Errorf("expected CheckoutSession.TripID=%s, got %s", tripID, captureCheckout.TripID)
	}
	if captureCheckout.CheckoutToken != "cs_test_abc123" {
		t.Errorf("expected CheckoutToken from Stripe, got %q", captureCheckout.CheckoutToken)
	}
	if captureCheckout.AmountCents != 1900 {
		t.Errorf("expected AmountCents=1900, got %d", captureCheckout.AmountCents)
	}
	if captureCheckout.Currency != "CAD" {
		t.Errorf("expected Currency=CAD (uppercase), got %q", captureCheckout.Currency)
	}
}

func TestInitializeCheckout_DBErrorOnSessionStoreIsNonFatal(t *testing.T) {
	// CreateCheckoutSession failing is logged but does NOT fail the call —
	// the Stripe webhook is the source of truth and will unlock without
	// the local row. Verify the user still gets a redirect URL.
	q := &stubQueries{tb: t,
		getTripByIDFn: func(ctx context.Context, arg dbgen.GetTripByIDParams) (dbgen.Trip, error) {
			return dbgen.Trip{ID: arg.ID, UserID: arg.UserID}, nil
		},
		isTripUnlockedFn: func(ctx context.Context, arg dbgen.IsTripUnlockedParams) (bool, error) {
			return false, nil
		},
		createCheckoutSessionFn: func(ctx context.Context, arg dbgen.CreateCheckoutSessionParams) (dbgen.CheckoutSession, error) {
			return dbgen.CheckoutSession{}, errors.New("checkout_sessions DB write failed")
		},
	}

	var stripeForm url.Values
	server := fakeStripeServer(t, &stripeForm)
	defer server.Close()

	svc := newTestService(t, q, true)
	svc.client = stripeClientWithBaseURL(server.URL)

	result, err := svc.InitializeCheckout(context.Background(), uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("expected DB error to be non-fatal, got %v", err)
	}
	if result == nil || result.URL == "" {
		t.Errorf("expected URL even when session-store failed, got %+v", result)
	}
}

// --- HandlePaymentWebhook idempotency ---

func TestHandlePaymentWebhook_AlreadyUnlockedNoOp(t *testing.T) {
	// Idempotency invariant: if Stripe re-delivers the webhook (which they
	// do — at-least-once delivery is the contract), the second call must
	// not create a duplicate payment row, duplicate audit log, or fire a
	// second analytics event. The service short-circuits at the
	// IsTripUnlocked check.
	tracker := &recordingTracker{}
	q := &stubQueries{tb: t,
		isTripUnlockedFn: func(ctx context.Context, arg dbgen.IsTripUnlockedParams) (bool, error) {
			return true, nil
		},
	}
	svc := newTestService(t, q, true)
	svc.WithAnalytics(tracker)

	err := svc.HandlePaymentWebhook(context.Background(), uuid.New(), uuid.New(), "cs_test_dup", 1900)
	if err != nil {
		t.Fatalf("expected nil error on idempotent re-delivery, got %v", err)
	}

	if len(q.createPaymentCalls) != 0 {
		t.Errorf("expected zero CreatePayment calls on duplicate webhook, got %d", len(q.createPaymentCalls))
	}
	if len(q.createTripUnlockCalls) != 0 {
		t.Errorf("expected zero CreateTripUnlock calls on duplicate webhook, got %d", len(q.createTripUnlockCalls))
	}
	if len(tracker.events) != 0 {
		t.Errorf("expected no analytics events on idempotent webhook, got %d: %+v", len(tracker.events), tracker.events)
	}
}

func TestHandlePaymentWebhook_IsTripUnlockedErrorPropagates(t *testing.T) {
	wantErr := errors.New("db down")
	q := &stubQueries{tb: t,
		isTripUnlockedFn: func(ctx context.Context, arg dbgen.IsTripUnlockedParams) (bool, error) {
			return false, wantErr
		},
	}
	svc := newTestService(t, q, true)

	err := svc.HandlePaymentWebhook(context.Background(), uuid.New(), uuid.New(), "cs_test", 1900)
	if !errors.Is(err, wantErr) {
		t.Errorf("expected wantErr, got %v", err)
	}
	if len(q.createPaymentCalls) != 0 {
		t.Error("must not record payment when unlock-check fails")
	}
}

// --- HandlePaymentWebhook happy path ---

func TestHandlePaymentWebhook_HappyPathRecordsPaymentAndUnlocksAndFiresAnalytics(t *testing.T) {
	userID := uuid.New()
	tripID := uuid.New()
	paymentID := uuid.New()

	tracker := &recordingTracker{}
	q := &stubQueries{tb: t,
		isTripUnlockedFn: func(ctx context.Context, arg dbgen.IsTripUnlockedParams) (bool, error) {
			return false, nil
		},
		createPaymentFn: func(ctx context.Context, arg dbgen.CreatePaymentParams) (dbgen.Payment, error) {
			return dbgen.Payment{ID: paymentID}, nil
		},
		// Both downstream calls must be configured explicitly so
		// fail-loud doesn't fire — assertions below check captured
		// args, not return values.
		markCheckoutSessionCompleteFn: func(ctx context.Context, checkoutToken string) error {
			return nil
		},
		createTripUnlockFn: func(ctx context.Context, arg dbgen.CreateTripUnlockParams) (dbgen.TripUnlock, error) {
			return dbgen.TripUnlock{}, nil
		},
	}
	svc := newTestService(t, q, true)
	svc.WithAnalytics(tracker)

	err := svc.HandlePaymentWebhook(context.Background(), userID, tripID, "cs_test_xyz", 1900)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Payment row created with the right shape.
	if len(q.createPaymentCalls) != 1 {
		t.Fatalf("expected 1 CreatePayment call, got %d", len(q.createPaymentCalls))
	}
	pc := q.createPaymentCalls[0]
	if pc.UserID != userID || pc.TripID != tripID {
		t.Errorf("payment user/trip mismatch: got user=%s trip=%s", pc.UserID, pc.TripID)
	}
	if pc.ExternalPaymentID != "cs_test_xyz" {
		t.Errorf("expected ExternalPaymentID=cs_test_xyz, got %q", pc.ExternalPaymentID)
	}
	if pc.AmountCents != 1900 {
		t.Errorf("expected AmountCents=1900, got %d", pc.AmountCents)
	}
	if pc.Currency != "CAD" {
		t.Errorf("expected Currency=CAD, got %q", pc.Currency)
	}
	if pc.Status != "approved" {
		t.Errorf("expected Status=approved, got %q", pc.Status)
	}

	// Session marked complete with the Stripe session ID.
	if len(q.markCompleteCalls) != 1 || q.markCompleteCalls[0] != "cs_test_xyz" {
		t.Errorf("expected MarkCheckoutSessionComplete(cs_test_xyz), got %v", q.markCompleteCalls)
	}

	// Unlock row created with the payment ID linked.
	if len(q.createTripUnlockCalls) != 1 {
		t.Fatalf("expected 1 CreateTripUnlock call, got %d", len(q.createTripUnlockCalls))
	}
	uc := q.createTripUnlockCalls[0]
	if uc.UserID != userID || uc.TripID != tripID {
		t.Errorf("unlock user/trip mismatch: got user=%s trip=%s", uc.UserID, uc.TripID)
	}
	if uc.Source != "purchase" {
		t.Errorf("expected source=purchase, got %q", uc.Source)
	}
	if !uc.PaymentID.Valid {
		t.Error("expected payment_id to be valid (linked)")
	}
	if uc.PaymentID.Bytes != paymentID {
		t.Errorf("expected payment_id=%s, got %x", paymentID, uc.PaymentID.Bytes)
	}

	// Analytics event fires with amount_cents + currency, NEVER trip_id
	// (CLAUDE.md privacy rule — Article 9 categories).
	if len(tracker.events) != 1 {
		t.Fatalf("expected 1 analytics event, got %d: %+v", len(tracker.events), tracker.events)
	}
	ev := tracker.events[0]
	if ev.event != "trip_pro_purchased" {
		t.Errorf("expected event=trip_pro_purchased, got %q", ev.event)
	}
	if ev.userID != userID.String() {
		t.Errorf("expected userID=%s, got %s", userID, ev.userID)
	}
	if got := ev.properties["amount_cents"]; got != int32(1900) {
		t.Errorf("expected amount_cents=1900 (int32), got %v (%T)", got, got)
	}
	if got := ev.properties["currency"]; got != "CAD" {
		t.Errorf("expected currency=CAD, got %v", got)
	}
	// Privacy regression guard. CLAUDE.md ("PostHog Analytics" + "User
	// Privacy") treats trip metadata as GDPR Article 9 sensitive content
	// and forbids destination, dates, hotel/flight names, booking details
	// from analytics — counts and categories only. Inverted check: assert
	// the property bag contains EXACTLY {amount_cents, currency} and no
	// other key. The previous deny-list version (trip_id, destination,
	// destination_country, country) only caught the four most-likely
	// accidental additions; an inverted allow-list catches anything new
	// — `start_date`, `hotel_name`, `trip_title`, etc. — without us
	// having to anticipate which field a future regression might leak.
	allowedKeys := map[string]bool{"amount_cents": true, "currency": true}
	if len(ev.properties) != len(allowedKeys) {
		t.Errorf("analytics event must have exactly %d properties (%v), got %d: %+v",
			len(allowedKeys), allowedKeysSorted(allowedKeys), len(ev.properties), ev.properties)
	}
	for k := range ev.properties {
		if !allowedKeys[k] {
			t.Errorf("analytics event includes disallowed property %q (CLAUDE.md GDPR Article 9 — only counts/categories permitted), got props=%v",
				k, ev.properties)
		}
	}
}

// allowedKeysSorted is a tiny test helper for stable error messages — map
// iteration order is randomised in Go, so the failure output for the
// privacy guard would otherwise be flaky and slightly harder to grep.
func allowedKeysSorted(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func TestHandlePaymentWebhook_NilAnalyticsClientSafe(t *testing.T) {
	// If WithAnalytics was never called the Track() call site must be
	// guarded — webhook processing should succeed without an analytics
	// client wired up.
	q := &stubQueries{tb: t,
		isTripUnlockedFn: func(ctx context.Context, arg dbgen.IsTripUnlockedParams) (bool, error) {
			return false, nil
		},
		createPaymentFn: func(ctx context.Context, arg dbgen.CreatePaymentParams) (dbgen.Payment, error) {
			return dbgen.Payment{ID: uuid.New()}, nil
		},
		markCheckoutSessionCompleteFn: func(ctx context.Context, checkoutToken string) error {
			return nil
		},
		createTripUnlockFn: func(ctx context.Context, arg dbgen.CreateTripUnlockParams) (dbgen.TripUnlock, error) {
			return dbgen.TripUnlock{}, nil
		},
	}
	svc := newTestService(t, q, true)
	// no WithAnalytics call

	err := svc.HandlePaymentWebhook(context.Background(), uuid.New(), uuid.New(), "cs_test", 1900)
	if err != nil {
		t.Fatalf("unexpected error with nil analytics: %v", err)
	}
}

// --- HandlePaymentWebhook mark-session failure is non-fatal ---

func TestHandlePaymentWebhook_MarkSessionFailureDoesNotBlockUnlock(t *testing.T) {
	// MarkCheckoutSessionComplete failing is best-effort. The unlock MUST
	// still be created — the session row is a tracking record, not the
	// source of truth for entitlement.
	q := &stubQueries{tb: t,
		isTripUnlockedFn: func(ctx context.Context, arg dbgen.IsTripUnlockedParams) (bool, error) {
			return false, nil
		},
		createPaymentFn: func(ctx context.Context, arg dbgen.CreatePaymentParams) (dbgen.Payment, error) {
			return dbgen.Payment{ID: uuid.New()}, nil
		},
		markCheckoutSessionCompleteFn: func(ctx context.Context, checkoutToken string) error {
			return errors.New("session not found")
		},
		// Explicit no-op so fail-loud doesn't fire — this test asserts
		// the unlock IS created via captured calls; the return value
		// itself is irrelevant.
		createTripUnlockFn: func(ctx context.Context, arg dbgen.CreateTripUnlockParams) (dbgen.TripUnlock, error) {
			return dbgen.TripUnlock{}, nil
		},
	}
	svc := newTestService(t, q, true)

	err := svc.HandlePaymentWebhook(context.Background(), uuid.New(), uuid.New(), "cs_test", 1900)
	if err != nil {
		t.Fatalf("mark-session failure should not fail the webhook, got %v", err)
	}

	if len(q.createTripUnlockCalls) != 1 {
		t.Errorf("expected unlock to be created despite mark-session error, got %d unlock calls", len(q.createTripUnlockCalls))
	}
}

func TestHandlePaymentWebhook_PaymentRecordFailureBlocksUnlock(t *testing.T) {
	// Inverse: if recording the payment fails, we MUST NOT create the
	// unlock — that would grant access without a payment audit trail.
	wantErr := errors.New("payment insert failed")
	q := &stubQueries{tb: t,
		isTripUnlockedFn: func(ctx context.Context, arg dbgen.IsTripUnlockedParams) (bool, error) {
			return false, nil
		},
		createPaymentFn: func(ctx context.Context, arg dbgen.CreatePaymentParams) (dbgen.Payment, error) {
			return dbgen.Payment{}, wantErr
		},
	}
	svc := newTestService(t, q, true)

	err := svc.HandlePaymentWebhook(context.Background(), uuid.New(), uuid.New(), "cs_test", 1900)
	if !errors.Is(err, wantErr) {
		t.Errorf("expected wantErr, got %v", err)
	}
	if len(q.createTripUnlockCalls) != 0 {
		t.Errorf("must not create unlock when payment record failed, got %d calls", len(q.createTripUnlockCalls))
	}
}

func TestHandlePaymentWebhook_UnlockCreateFailurePropagates(t *testing.T) {
	wantErr := errors.New("unlock insert failed")
	q := &stubQueries{tb: t,
		isTripUnlockedFn: func(ctx context.Context, arg dbgen.IsTripUnlockedParams) (bool, error) {
			return false, nil
		},
		createPaymentFn: func(ctx context.Context, arg dbgen.CreatePaymentParams) (dbgen.Payment, error) {
			return dbgen.Payment{ID: uuid.New()}, nil
		},
		// MarkCheckoutSessionComplete fires before the unlock create —
		// explicit no-op so fail-loud doesn't mask the wantErr we're
		// actually testing.
		markCheckoutSessionCompleteFn: func(ctx context.Context, checkoutToken string) error {
			return nil
		},
		createTripUnlockFn: func(ctx context.Context, arg dbgen.CreateTripUnlockParams) (dbgen.TripUnlock, error) {
			return dbgen.TripUnlock{}, wantErr
		},
	}
	svc := newTestService(t, q, true)

	err := svc.HandlePaymentWebhook(context.Background(), uuid.New(), uuid.New(), "cs_test", 1900)
	if !errors.Is(err, wantErr) {
		t.Errorf("expected wantErr, got %v", err)
	}
}
