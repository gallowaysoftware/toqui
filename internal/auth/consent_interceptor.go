package auth

import (
	"context"
	"fmt"
	"log/slog"

	"connectrpc.com/connect"
	"github.com/google/uuid"
)

// ConsentCheckFunc returns true if the user has satisfied every required
// consent (today: `terms` and `privacy_policy`). Implementations should
// hit the user_consents table — see queries.HasRequiredConsents.
type ConsentCheckFunc func(ctx context.Context, userID uuid.UUID) (bool, error)

// consentExemptMethods lists RPCs a user must still be able to call even
// when consent is missing: auth lifecycle (so they can log in, refresh,
// read their own profile, or delete/export their data) and account
// deletion / export (GDPR Article 17 / 20). Everything else is gated.
//
// `POST /auth/consent` is NOT a ConnectRPC method; it's an HTTP REST
// endpoint and lives outside this interceptor's reach — no entry
// needed. The frontend calls that endpoint before retrying a gated
// RPC, which is the intended unblock flow.
//
// This list is a superset of ageExemptMethods. A user without consent
// cannot be assumed to have verified their age either, but we still
// allow the read-only "what's my state?" RPCs so the frontend can
// render the consent modal.
var consentExemptMethods = map[string]bool{
	"/toqui.v1.AuthService/GoogleLogin":    true,
	"/toqui.v1.AuthService/FacebookLogin":  true,
	"/toqui.v1.AuthService/RefreshToken":   true,
	"/toqui.v1.AuthService/GetCurrentUser": true,
	"/toqui.v1.AuthService/DeleteAccount":  true,
	"/toqui.v1.AuthService/ExportData":     true,
}

type consentInterceptor struct {
	checkConsent ConsentCheckFunc
}

// NewConsentInterceptor enforces that authenticated users have recorded
// the required GDPR consents before any non-exempt RPC runs. Must be
// chained AFTER the auth interceptor so the user ID is on the context,
// and should run AFTER the age interceptor so age-enforcement errors
// (which indicate a deeper onboarding state) take precedence.
//
// Finding: toqui-backend#369 P1 #3 — prior to this interceptor
// `consent_pending` was only a hint in the login response; a client
// that ignored the hint could call every RPC.
func NewConsentInterceptor(checkConsent ConsentCheckFunc) connect.Interceptor {
	return &consentInterceptor{checkConsent: checkConsent}
}

func (i *consentInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		if err := i.enforce(ctx, req.Spec().Procedure); err != nil {
			return nil, err
		}
		return next(ctx, req)
	}
}

func (i *consentInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *consentInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		if err := i.enforce(ctx, conn.Spec().Procedure); err != nil {
			return err
		}
		return next(ctx, conn)
	}
}

func (i *consentInterceptor) enforce(ctx context.Context, procedure string) error {
	// Public and exempt methods skip the consent check.
	if publicMethods[procedure] || consentExemptMethods[procedure] {
		return nil
	}

	userID, ok := UserIDFromContext(ctx)
	if !ok {
		// No user in context — auth interceptor didn't run or method is
		// public. Mirror the age interceptor's behavior: don't second-
		// guess; let the next layer decide.
		return nil
	}

	ok, err := i.checkConsent(ctx, userID)
	if err != nil {
		slog.Error("consent check failed", "user_id", userID.String(), "error", err)
		return connect.NewError(connect.CodeInternal, fmt.Errorf("consent check failed"))
	}

	if !ok {
		// FailedPrecondition so the frontend can distinguish "you need
		// to accept consent" from "you're not authenticated" (which
		// would log you out) or "you're not permitted" (which would
		// hide the feature). The "consent_required" sentinel is what
		// the frontend matches on to pop the consent modal.
		return connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("consent_required"))
	}

	return nil
}
