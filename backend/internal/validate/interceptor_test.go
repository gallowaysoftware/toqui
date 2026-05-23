package validate_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"

	toquiv1 "github.com/gallowaysoftware/toqui-backend/gen/toqui/v1"
	"github.com/gallowaysoftware/toqui-backend/internal/validate"
)

// TestInterceptor_WrapUnary_ValidRequestPassesThrough pins the happy path:
// a request whose protovalidate constraints are satisfied must reach the
// downstream handler with no modification. Any future refactor that
// short-circuits valid requests would silently break every RPC.
func TestInterceptor_WrapUnary_ValidRequestPassesThrough(t *testing.T) {
	interceptor := validate.NewInterceptor()

	var nextCalled bool
	next := connect.UnaryFunc(func(_ context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		nextCalled = true
		return connect.NewResponse(&toquiv1.GoogleLoginResponse{}), nil
	})

	// GoogleLoginRequest.code has [(buf.validate.field).string.min_len = 1];
	// a non-empty value satisfies it. Same for redirect_uri.
	req := connect.NewRequest(&toquiv1.GoogleLoginRequest{
		Code:        "valid-auth-code",
		RedirectUri: "https://example.com/callback",
	})

	if _, err := interceptor.WrapUnary(next)(context.Background(), req); err != nil {
		t.Fatalf("WrapUnary errored on a valid request: %v", err)
	}
	if !nextCalled {
		t.Error("next handler was not called for a valid request")
	}
}

// TestInterceptor_WrapUnary_ValidationErrorMapsToInvalidArgument is the
// core contract of this interceptor: protovalidate.ValidationError MUST
// surface to the caller as connect.CodeInvalidArgument (HTTP 400) rather
// than CodeInternal (HTTP 500). Mapping is what lets clients render
// per-field errors and what keeps validation failures out of error
// budgets and on-call alerts.
func TestInterceptor_WrapUnary_ValidationErrorMapsToInvalidArgument(t *testing.T) {
	interceptor := validate.NewInterceptor()

	var nextCalled bool
	next := connect.UnaryFunc(func(_ context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		nextCalled = true
		return nil, nil
	})

	// Empty code violates min_len=1.
	req := connect.NewRequest(&toquiv1.GoogleLoginRequest{Code: "", RedirectUri: ""})

	_, err := interceptor.WrapUnary(next)(context.Background(), req)
	if nextCalled {
		t.Error("next must NOT be called when validation fails (request is rejected)")
	}
	if err == nil {
		t.Fatal("expected a validation error, got nil")
	}
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		t.Fatalf("expected *connect.Error, got %T: %v", err, err)
	}
	if got, want := connectErr.Code(), connect.CodeInvalidArgument; got != want {
		t.Errorf("Code = %v, want %v (validation failures are CLIENT errors, not 500s)", got, want)
	}
}

// nonProto is intentionally NOT a proto.Message — used to exercise the
// type-assertion guard in WrapUnary.
type nonProto struct{}

// TestInterceptor_WrapUnary_NonProtoMessageBypassesValidation pins the
// defensive type-assertion in the interceptor. ConnectRPC always wraps
// proto.Message bodies in practice, but the `if msg, ok := …; ok` guard
// means a hypothetical non-proto payload (e.g. a future custom transport,
// or a test double) passes through rather than panicking on Validate(nil).
func TestInterceptor_WrapUnary_NonProtoMessageBypassesValidation(t *testing.T) {
	interceptor := validate.NewInterceptor()

	var nextCalled bool
	next := connect.UnaryFunc(func(_ context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		nextCalled = true
		return nil, nil
	})

	req := connect.NewRequest(&nonProto{})

	if _, err := interceptor.WrapUnary(next)(context.Background(), req); err != nil {
		t.Fatalf("non-proto request should pass through, got error: %v", err)
	}
	if !nextCalled {
		t.Error("next must be called when the request is not a proto.Message (no validation to run)")
	}
}

// TestInterceptor_WrapStreamingClient_PassesThroughUnchanged pins the
// pass-through wiring for streaming clients. The interceptor doesn't
// inject validation here (initial-message validation on streams isn't
// wired — there are no client-streaming RPCs), so the wrapped function
// must be observably equivalent to the bare next func.
func TestInterceptor_WrapStreamingClient_PassesThroughUnchanged(t *testing.T) {
	interceptor := validate.NewInterceptor()

	var called bool
	next := connect.StreamingClientFunc(func(_ context.Context, _ connect.Spec) connect.StreamingClientConn {
		called = true
		return nil
	})

	wrapped := interceptor.WrapStreamingClient(next)
	wrapped(context.Background(), connect.Spec{})

	if !called {
		t.Error("WrapStreamingClient must invoke next (it is a pass-through)")
	}
}

// TestInterceptor_WrapStreamingHandler_PassesThrough exercises the
// server-streaming wrapper. Documented intent (see comment in
// interceptor.go): "validation of streaming messages would need
// per-message interception which isn't needed for our current RPCs".
// This test pins that pass-through so a future refactor that adds
// validation here is forced to update the test deliberately.
func TestInterceptor_WrapStreamingHandler_PassesThrough(t *testing.T) {
	interceptor := validate.NewInterceptor()

	var called bool
	wantErr := errors.New("downstream handler error")
	next := connect.StreamingHandlerFunc(func(_ context.Context, _ connect.StreamingHandlerConn) error {
		called = true
		return wantErr
	})

	wrapped := interceptor.WrapStreamingHandler(next)
	gotErr := wrapped(context.Background(), nil)

	if !called {
		t.Error("WrapStreamingHandler must invoke next (it is a pass-through)")
	}
	if !errors.Is(gotErr, wantErr) {
		t.Errorf("error = %v, want %v (handler error must propagate unchanged)", gotErr, wantErr)
	}
}
