//go:build integration

package integration

import (
	"context"
	"testing"

	"connectrpc.com/connect"

	toquiv1 "github.com/gallowaysoftware/toqui/backend/gen/toqui/v1"
	"github.com/gallowaysoftware/toqui/backend/internal/auth"
	"github.com/gallowaysoftware/toqui/backend/internal/chatstore"
	"github.com/gallowaysoftware/toqui/backend/internal/handlers"
	"github.com/gallowaysoftware/toqui/backend/internal/lifecycle"
)

// TestEmailRegisterAndLogin exercises the email+password flow through the
// REAL handler wiring — h.queries backed by dbgen, no test stub. This pins
// the production code path that unit tests bypass: the emailQueries()
// accessor once recursed infinitely when testEmailQueries was nil (every
// EmailRegister on a deployed server crashed with a stack overflow), and
// only a test that leaves the stub unset can catch that class of bug.
func TestEmailRegisterAndLogin(t *testing.T) {
	env := NewTestEnv(t)
	env.CleanDB(t)
	ctx := context.Background()

	authSvc := auth.NewService("test-client-id", "test-secret", "http://localhost/callback", "test-jwt-secret-emailflow-32chars!")
	lifecycleSvc := lifecycle.NewService(env.Pool, chatstore.New(env.Pool))
	h := handlers.NewAuthHandler(authSvc, env.Pool, lifecycleSvc, nil, nil)

	const email = "email-flow@toqui-test.local"
	const password = "a-long-enough-password"

	reg, err := h.EmailRegister(ctx, connect.NewRequest(&toquiv1.EmailRegisterRequest{
		Email:    email,
		Password: password,
		Name:     "Email Flow",
	}))
	if err != nil {
		t.Fatalf("EmailRegister: %v", err)
	}
	if reg.Msg.AccessToken == "" || reg.Msg.RefreshToken == "" {
		t.Fatal("EmailRegister returned empty tokens")
	}
	if reg.Msg.User.GetEmail() != email {
		t.Errorf("registered user email = %q, want %q", reg.Msg.User.GetEmail(), email)
	}

	// Duplicate registration must fail with AlreadyExists.
	if _, err := h.EmailRegister(ctx, connect.NewRequest(&toquiv1.EmailRegisterRequest{
		Email:    email,
		Password: password,
		Name:     "Dupe",
	})); connect.CodeOf(err) != connect.CodeAlreadyExists {
		t.Errorf("duplicate register: got %v, want CodeAlreadyExists", err)
	}

	// Correct password logs in.
	login, err := h.EmailLogin(ctx, connect.NewRequest(&toquiv1.EmailLoginRequest{
		Email:    email,
		Password: password,
	}))
	if err != nil {
		t.Fatalf("EmailLogin: %v", err)
	}
	if login.Msg.AccessToken == "" {
		t.Fatal("EmailLogin returned empty access token")
	}
	userID, err := authSvc.ValidateToken(login.Msg.AccessToken)
	if err != nil {
		t.Fatalf("validate access token from login: %v", err)
	}
	if userID.String() != reg.Msg.User.GetId() {
		t.Errorf("login token user = %s, want %s", userID, reg.Msg.User.GetId())
	}

	// Wrong password collapses to Unauthenticated.
	if _, err := h.EmailLogin(ctx, connect.NewRequest(&toquiv1.EmailLoginRequest{
		Email:    email,
		Password: "wrong-password-entirely",
	})); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Errorf("wrong password: got %v, want CodeUnauthenticated", err)
	}

	// Unknown email collapses to Unauthenticated too (no enumeration).
	if _, err := h.EmailLogin(ctx, connect.NewRequest(&toquiv1.EmailLoginRequest{
		Email:    "nobody@toqui-test.local",
		Password: password,
	})); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Errorf("unknown email: got %v, want CodeUnauthenticated", err)
	}
}
