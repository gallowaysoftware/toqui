//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/gallowaysoftware/toqui-backend/internal/auth"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
)

// TestConsentInterceptor_EndToEnd exercises the consent gate against a
// real Postgres. The happy-path assertion is that the exact sqlc
// query wired into main.go (`HasRequiredConsents`) returns the
// expected booleans for the three interesting states (no consents at
// all, partial consents, both required consents).
//
// Pairs with the unit test in internal/auth — the unit test pins the
// interceptor's gating logic, this test pins the DB contract it's
// riding on. A schema change to user_consents that breaks
// HasRequiredConsents would otherwise only surface in prod as a
// wide-open API.
func TestConsentInterceptor_EndToEnd(t *testing.T) {
	env := NewTestEnv(t)
	env.CleanDB(t)
	ctx := context.Background()
	queries := dbgen.New(env.Pool)

	user, err := queries.UpsertUserByGoogleID(ctx, dbgen.UpsertUserByGoogleIDParams{
		GoogleID: pgtype.Text{String: "consent-e2e-google", Valid: true},
		Email:    "consent-e2e@example.com",
		Name:     pgtype.Text{String: "Consent User", Valid: true},
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	checkConsent := func(ctx context.Context, userID uuid.UUID) (bool, error) {
		return queries.HasRequiredConsents(ctx, userID)
	}

	// 1. Zero consents → blocked with FailedPrecondition.
	has, err := checkConsent(ctx, user.ID)
	if err != nil {
		t.Fatalf("HasRequiredConsents (none): %v", err)
	}
	if has {
		t.Fatal("user with no consents should NOT have required consents")
	}

	// 2. Only terms → still blocked (privacy_policy missing).
	if _, err := queries.RecordConsent(ctx, dbgen.RecordConsentParams{
		UserID:      user.ID,
		ConsentType: "terms",
		IpAddress:   pgtype.Text{},
		UserAgent:   pgtype.Text{},
	}); err != nil {
		t.Fatalf("record terms: %v", err)
	}
	has, err = checkConsent(ctx, user.ID)
	if err != nil {
		t.Fatalf("HasRequiredConsents (terms only): %v", err)
	}
	if has {
		t.Fatal("user with only 'terms' should NOT have required consents")
	}

	// 3. Terms + privacy_policy → passes.
	if _, err := queries.RecordConsent(ctx, dbgen.RecordConsentParams{
		UserID:      user.ID,
		ConsentType: "privacy_policy",
		IpAddress:   pgtype.Text{},
		UserAgent:   pgtype.Text{},
	}); err != nil {
		t.Fatalf("record privacy_policy: %v", err)
	}
	has, err = checkConsent(ctx, user.ID)
	if err != nil {
		t.Fatalf("HasRequiredConsents (both): %v", err)
	}
	if !has {
		t.Fatal("user with terms+privacy_policy SHOULD have required consents")
	}

	// 4. Smoke-test the interceptor constructor accepts the real DB
	// query's shape. The gating logic itself is covered by the unit
	// test in internal/auth/consent_interceptor_test.go; this test
	// pins the DB contract the interceptor relies on.
	_ = auth.NewConsentInterceptor(auth.ConsentCheckFunc(checkConsent))

	// 5. Withdrawn consents: revoke the privacy_policy; check flips to
	// false. Pins that soft-delete / withdrawn_at is respected.
	if err := queries.WithdrawConsent(ctx, dbgen.WithdrawConsentParams{
		UserID:      user.ID,
		ConsentType: "privacy_policy",
	}); err != nil {
		t.Fatalf("withdraw privacy_policy: %v", err)
	}
	has, err = checkConsent(ctx, user.ID)
	if err != nil {
		t.Fatalf("HasRequiredConsents (after withdraw): %v", err)
	}
	if has {
		t.Fatal("user with withdrawn privacy_policy should NOT have required consents")
	}
}
