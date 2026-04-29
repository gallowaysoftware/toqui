package handlers

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"

	toquiv1 "github.com/gallowaysoftware/toqui-backend/gen/toqui/v1"
	"github.com/gallowaysoftware/toqui-backend/internal/auth"
)

// When Apple Sign-In is not configured (no Apple client attached to the auth
// service), the AppleLogin RPC must return Unimplemented. This is the
// scaffold-friendly mode: the code ships before Apple Developer enrollment
// completes, and only flips on once the four env vars are populated.
func TestAppleLogin_UnimplementedWhenNotConfigured(t *testing.T) {
	authSvc := auth.NewService("g-id", "g-secret", "http://localhost/cb", "test-jwt-secret-32chars-min!!")
	// authSvc.AppleClient() == nil because we never called WithAppleClient.

	h := &AuthHandler{authSvc: authSvc}

	_, err := h.AppleLogin(context.Background(), connect.NewRequest(&toquiv1.AppleLoginRequest{
		AuthorizationCode: "code",
		IdToken:           "idtok",
	}))
	if err == nil {
		t.Fatal("expected Unimplemented error, got nil")
	}
	var connErr *connect.Error
	if !errors.As(err, &connErr) {
		t.Fatalf("expected *connect.Error, got %T", err)
	}
	if connErr.Code() != connect.CodeUnimplemented {
		t.Errorf("got code %v, want CodeUnimplemented", connErr.Code())
	}
}
