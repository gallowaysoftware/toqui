package handlers

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"connectrpc.com/connect"

	"github.com/gallowaysoftware/toqui-backend/internal/auth"
	"github.com/gallowaysoftware/toqui-backend/internal/trip"
)

// newTestAuthService creates an auth.Service for unit tests with a known secret.
func newTestAuthService() *auth.Service {
	return auth.NewService("test-client-id", "test-client-secret", "http://localhost/callback", "test-jwt-secret")
}

func TestMapTripErr(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name     string
		err      error
		wantCode connect.Code
	}{
		{
			name:     "ErrNotOwnerOrEditor → PermissionDenied",
			err:      trip.ErrNotOwnerOrEditor,
			wantCode: connect.CodePermissionDenied,
		},
		{
			name:     "wrapped ErrNotOwnerOrEditor still maps to PermissionDenied",
			err:      fmt.Errorf("replace itinerary: %w", trip.ErrNotOwnerOrEditor),
			wantCode: connect.CodePermissionDenied,
		},
		{
			name:     "ErrInvalidStatusTransition → FailedPrecondition",
			err:      trip.ErrInvalidStatusTransition,
			wantCode: connect.CodeFailedPrecondition,
		},
		{
			name:     "wrapped ErrInvalidStatusTransition still maps to FailedPrecondition",
			err:      fmt.Errorf("trip update: %w", trip.ErrInvalidStatusTransition),
			wantCode: connect.CodeFailedPrecondition,
		},
		{
			name:     "unrelated DB error → Internal (no PII leaked)",
			err:      errors.New("pq: connection reset by peer"),
			wantCode: connect.CodeInternal,
		},
		{
			name:     "context.Canceled (transient DB failure shape) → Internal",
			err:      fmt.Errorf("check edit access: %w", context.Canceled),
			wantCode: connect.CodeInternal,
		},
		{
			name:     "ErrInvalidInitialStatus → InvalidArgument",
			err:      trip.ErrInvalidInitialStatus,
			wantCode: connect.CodeInvalidArgument,
		},
		{
			// Nil err should never happen in practice (callers guard
			// err != nil). Pin the fail-loud behaviour: a non-nil
			// Internal error so the bug surfaces in logs instead of
			// a silent success.
			name:     "nil err → Internal (bug surfaces loudly)",
			err:      nil,
			wantCode: connect.CodeInternal,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mapTripErr(ctx, "test op", tc.err)
			if got == nil {
				t.Fatalf("mapTripErr returned nil for %v", tc.err)
			}
			if got.Code() != tc.wantCode {
				t.Errorf("mapTripErr code = %v, want %v", got.Code(), tc.wantCode)
			}
			// Internal errors must scrub the message to avoid leaking
			// DB details to the client. Known sentinels are fine to
			// surface since they carry no PII (editor access required,
			// invalid status transition, etc.).
			if tc.wantCode == connect.CodeInternal {
				if msg := got.Message(); msg != "an internal error occurred" {
					t.Errorf("internal error message leaked underlying detail: %q", msg)
				}
			}
		})
	}
}
