package auth

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
)

// TestConsentInterceptor_Enforces pins the behaviour of the consent
// gate added in toqui-backend#369 P1 #3. Auth interceptor has already
// set the user ID on the context by the time this runs.
func TestConsentInterceptor_Enforces(t *testing.T) {
	userID := uuid.New()
	ctx := ContextWithUserID(context.Background(), userID)

	// Exempt path: consent-less user must still be able to call the
	// exempt methods (otherwise they can't log in or record consent).
	for method := range consentExemptMethods {
		t.Run("exempt_"+method, func(t *testing.T) {
			i := &consentInterceptor{
				checkConsent: func(_ context.Context, _ uuid.UUID) (bool, error) {
					t.Fatal("checkConsent must not be called for exempt methods")
					return false, nil
				},
			}
			if err := i.enforce(ctx, method); err != nil {
				t.Fatalf("exempt method %q rejected: %v", method, err)
			}
		})
	}

	// Public methods (login, refresh) — same story.
	for method := range publicMethods {
		t.Run("public_"+method, func(t *testing.T) {
			i := &consentInterceptor{
				checkConsent: func(_ context.Context, _ uuid.UUID) (bool, error) {
					t.Fatal("checkConsent must not be called for public methods")
					return false, nil
				},
			}
			if err := i.enforce(ctx, method); err != nil {
				t.Fatalf("public method %q rejected: %v", method, err)
			}
		})
	}

	// Missing consents: non-exempt RPC → FailedPrecondition with
	// "consent_required". The error code is load-bearing — the frontend
	// matches on it to pop the consent modal.
	t.Run("missing_consent_blocks_non_exempt", func(t *testing.T) {
		i := &consentInterceptor{
			checkConsent: func(_ context.Context, _ uuid.UUID) (bool, error) {
				return false, nil
			},
		}
		err := i.enforce(ctx, "/toqui.v1.TripService/ListTrips")
		if err == nil {
			t.Fatal("expected error when consents are missing")
		}
		if got, want := connect.CodeOf(err), connect.CodeFailedPrecondition; got != want {
			t.Fatalf("err code = %v, want %v", got, want)
		}
		if msg := err.Error(); msg == "" || !contains(msg, "consent_required") {
			t.Fatalf("err message = %q, want it to contain 'consent_required'", msg)
		}
	})

	// Present consents: non-exempt RPC passes through.
	t.Run("present_consent_allows_non_exempt", func(t *testing.T) {
		i := &consentInterceptor{
			checkConsent: func(_ context.Context, _ uuid.UUID) (bool, error) {
				return true, nil
			},
		}
		if err := i.enforce(ctx, "/toqui.v1.TripService/ListTrips"); err != nil {
			t.Fatalf("consented user was still rejected: %v", err)
		}
	})

	// Check-failure: DB unreachable → Internal, not a silent allow. If
	// the consent table is down, we fail closed.
	t.Run("check_error_is_internal", func(t *testing.T) {
		i := &consentInterceptor{
			checkConsent: func(_ context.Context, _ uuid.UUID) (bool, error) {
				return false, errors.New("db exploded")
			},
		}
		err := i.enforce(ctx, "/toqui.v1.TripService/ListTrips")
		if err == nil {
			t.Fatal("expected error when checkConsent returns error")
		}
		if got, want := connect.CodeOf(err), connect.CodeInternal; got != want {
			t.Fatalf("err code = %v, want %v", got, want)
		}
	})

	// No user ID on context (pre-auth public path that happens to hit a
	// non-exempt method): pass through — auth interceptor decides.
	t.Run("no_user_id_passes_through", func(t *testing.T) {
		i := &consentInterceptor{
			checkConsent: func(_ context.Context, _ uuid.UUID) (bool, error) {
				t.Fatal("checkConsent must not be called without a userID in context")
				return false, nil
			},
		}
		if err := i.enforce(context.Background(), "/toqui.v1.TripService/ListTrips"); err != nil {
			t.Fatalf("no-user-id context rejected: %v", err)
		}
	})
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
