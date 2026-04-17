//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui-backend/internal/payment"
	"github.com/gallowaysoftware/toqui-backend/internal/trip"
)

// TestCheckoutAuthzGate pins the #361 P1 #3 fix: payment.InitializeCheckout
// must verify the caller owns the target trip before touching Stripe or
// writing a checkout_sessions row. Previously it only called
// IsTripUnlocked which returns false for any missing (user_id, trip_id)
// tuple — regardless of actual ownership — so a malicious client could
// burn their own payment method creating bogus sessions against a
// victim's trip_id.
func TestCheckoutAuthzGate(t *testing.T) {
	env := NewTestEnv(t)
	env.CleanDB(t)
	ctx := context.Background()
	queries := dbgen.New(env.Pool)
	tripSvc := trip.NewService(env.Pool)

	alice, err := queries.UpsertUserByGoogleID(ctx, dbgen.UpsertUserByGoogleIDParams{
		GoogleID: pgtype.Text{String: "checkout-alice", Valid: true},
		Email:    "checkout-alice@example.com",
		Name:     pgtype.Text{String: "Alice", Valid: true},
	})
	if err != nil {
		t.Fatalf("create alice: %v", err)
	}
	mallory, err := queries.UpsertUserByGoogleID(ctx, dbgen.UpsertUserByGoogleIDParams{
		GoogleID: pgtype.Text{String: "checkout-mallory", Valid: true},
		Email:    "checkout-mallory@example.com",
		Name:     pgtype.Text{String: "Mallory", Valid: true},
	})
	if err != nil {
		t.Fatalf("create mallory: %v", err)
	}

	aliceTrip, err := tripSvc.Create(ctx, alice.ID, "Alice's Trip", "", nil, nil)
	if err != nil {
		t.Fatalf("create alice trip: %v", err)
	}

	// Construct payment service in enabled mode with a fake Stripe key.
	// The ownership gate short-circuits BEFORE any Stripe API call, so
	// we never actually hit Stripe on the unhappy paths exercised here.
	// Don't test the happy path — that one really would call Stripe.
	svc := payment.NewService("sk_test_fake", "prod_fake_product", 1900, queries, "https://example.com")

	t.Run("ForeignTrip_ReturnsErrNotTripOwner", func(t *testing.T) {
		_, err := svc.InitializeCheckout(ctx, mallory.ID, aliceTrip.ID)
		if !errors.Is(err, payment.ErrNotTripOwner) {
			t.Errorf("mallory checkout against alice trip: expected payment.ErrNotTripOwner, got %v", err)
		}
	})

	t.Run("GhostTrip_ReturnsErrNotTripOwner", func(t *testing.T) {
		// Non-existent trip → same code path: GetTripByID returns
		// pgx.ErrNoRows, service translates to ErrNotTripOwner. The
		// client can't distinguish "not the owner" from "trip doesn't
		// exist" which is the right privacy posture.
		if err := tripSvc.Delete(ctx, alice.ID, aliceTrip.ID); err != nil {
			t.Fatalf("delete alice trip for ghost-trip case: %v", err)
		}
		_, err := svc.InitializeCheckout(ctx, alice.ID, aliceTrip.ID)
		if !errors.Is(err, payment.ErrNotTripOwner) {
			t.Errorf("alice checkout against deleted trip: expected payment.ErrNotTripOwner, got %v", err)
		}
	})
}
