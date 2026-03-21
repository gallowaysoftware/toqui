package auth

import (
	"context"
	"fmt"
	"log/slog"

	"connectrpc.com/connect"
	"github.com/google/uuid"
)

// AgeCheckFunc checks whether a user has completed age verification.
// Returns true if verified, false if not.
type AgeCheckFunc func(ctx context.Context, userID uuid.UUID) (bool, error)

// Methods exempt from age verification (user needs to be able to verify age,
// view their profile, and manage their account before verifying).
var ageExemptMethods = map[string]bool{
	"/toqui.v1.AuthService/GoogleLogin":    true,
	"/toqui.v1.AuthService/RefreshToken":   true,
	"/toqui.v1.AuthService/GetCurrentUser": true,
	"/toqui.v1.AuthService/DeleteAccount":  true,
	"/toqui.v1.AuthService/ExportData":     true,
}

type ageInterceptor struct {
	checkAge AgeCheckFunc
}

// NewAgeInterceptor creates an interceptor that enforces age verification
// on authenticated RPC methods. Must be chained AFTER the auth interceptor
// so that the user ID is available in context.
func NewAgeInterceptor(checkAge AgeCheckFunc) connect.Interceptor {
	return &ageInterceptor{checkAge: checkAge}
}

func (i *ageInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		if err := i.enforceAge(ctx, req.Spec().Procedure); err != nil {
			return nil, err
		}
		return next(ctx, req)
	}
}

func (i *ageInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *ageInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		if err := i.enforceAge(ctx, conn.Spec().Procedure); err != nil {
			return err
		}
		return next(ctx, conn)
	}
}

func (i *ageInterceptor) enforceAge(ctx context.Context, procedure string) error {
	// Public and exempt methods skip age check.
	if publicMethods[procedure] || ageExemptMethods[procedure] {
		return nil
	}

	userID, ok := UserIDFromContext(ctx)
	if !ok {
		// No user in context — auth interceptor hasn't run or method is public.
		return nil
	}

	verified, err := i.checkAge(ctx, userID)
	if err != nil {
		slog.Error("age verification check failed", "user_id", userID.String(), "error", err)
		return connect.NewError(connect.CodeInternal, fmt.Errorf("age verification check failed"))
	}

	if !verified {
		return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("age verification required"))
	}

	return nil
}
