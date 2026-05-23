package handlers

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"

	"github.com/gallowaysoftware/toqui/backend/internal/auth"
	"github.com/gallowaysoftware/toqui/backend/internal/dbgen"

	toquiv1 "github.com/gallowaysoftware/toqui/backend/gen/toqui/v1"
)

// stubEmailAuthQueries is a fail-loud test double for the emailAuthQueries
// interface. Every method panics with t.Fatalf unless the corresponding
// *Fn hook is set, matching the pattern used in feedback_test.go etc.
type stubEmailAuthQueries struct {
	tb testing.TB

	getUserByEmailFn         func(ctx context.Context, email string) (dbgen.User, error)
	getUserPasswordHashFn    func(ctx context.Context, email string) (dbgen.GetUserPasswordHashRow, error)
	getUserByIDFn            func(ctx context.Context, id uuid.UUID) (dbgen.User, error)
	createUserWithPasswordFn func(ctx context.Context, arg dbgen.CreateUserWithPasswordParams) (dbgen.User, error)
	createRefreshTokenFn     func(ctx context.Context, arg dbgen.CreateRefreshTokenParams) (dbgen.RefreshToken, error)

	createUserCalls []dbgen.CreateUserWithPasswordParams
}

func (s *stubEmailAuthQueries) GetUserByEmail(ctx context.Context, email string) (dbgen.User, error) {
	if s.getUserByEmailFn != nil {
		return s.getUserByEmailFn(ctx, email)
	}
	s.tb.Fatalf("unexpected stubEmailAuthQueries.GetUserByEmail(%q) — set getUserByEmailFn", email)
	return dbgen.User{}, nil
}

func (s *stubEmailAuthQueries) GetUserPasswordHash(ctx context.Context, email string) (dbgen.GetUserPasswordHashRow, error) {
	if s.getUserPasswordHashFn != nil {
		return s.getUserPasswordHashFn(ctx, email)
	}
	s.tb.Fatalf("unexpected stubEmailAuthQueries.GetUserPasswordHash(%q) — set getUserPasswordHashFn", email)
	return dbgen.GetUserPasswordHashRow{}, nil
}

func (s *stubEmailAuthQueries) GetUserByID(ctx context.Context, id uuid.UUID) (dbgen.User, error) {
	if s.getUserByIDFn != nil {
		return s.getUserByIDFn(ctx, id)
	}
	s.tb.Fatalf("unexpected stubEmailAuthQueries.GetUserByID(%s) — set getUserByIDFn", id)
	return dbgen.User{}, nil
}

func (s *stubEmailAuthQueries) CreateUserWithPassword(ctx context.Context, arg dbgen.CreateUserWithPasswordParams) (dbgen.User, error) {
	s.createUserCalls = append(s.createUserCalls, arg)
	if s.createUserWithPasswordFn != nil {
		return s.createUserWithPasswordFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubEmailAuthQueries.CreateUserWithPassword(%+v) — set createUserWithPasswordFn", arg)
	return dbgen.User{}, nil
}

func (s *stubEmailAuthQueries) CreateRefreshToken(ctx context.Context, arg dbgen.CreateRefreshTokenParams) (dbgen.RefreshToken, error) {
	if s.createRefreshTokenFn != nil {
		return s.createRefreshTokenFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubEmailAuthQueries.CreateRefreshToken(%+v) — set createRefreshTokenFn", arg)
	return dbgen.RefreshToken{}, nil
}

// newEmailAuthHandler wires a real auth.Service with a known JWT secret
// (so token round-trips work in tests) plus a stub query layer. No
// pool / lifecycleSvc — the email-auth code paths don't touch them.
func newEmailAuthHandler(tb testing.TB, stub *stubEmailAuthQueries, allowedDomains []string) *AuthHandler {
	tb.Helper()
	authSvc := auth.NewService("g-id", "g-secret", "http://localhost/cb", "test-jwt-secret-32chars-min!!")
	return &AuthHandler{
		authSvc:          authSvc,
		allowedDomains:   allowedDomains,
		testEmailQueries: stub,
	}
}

func TestEmailRegister_HappyPath(t *testing.T) {
	stub := &stubEmailAuthQueries{tb: t}
	stub.getUserByEmailFn = func(ctx context.Context, email string) (dbgen.User, error) {
		return dbgen.User{}, pgx.ErrNoRows
	}
	createdID := uuid.New()
	stub.createUserWithPasswordFn = func(ctx context.Context, arg dbgen.CreateUserWithPasswordParams) (dbgen.User, error) {
		if arg.Email != "alice@example.com" {
			t.Fatalf("email not normalized: %q", arg.Email)
		}
		if !arg.PasswordHash.Valid || arg.PasswordHash.String == "" {
			t.Fatalf("password_hash not set: %+v", arg.PasswordHash)
		}
		// Verify the stored hash actually verifies against the original password.
		if err := bcrypt.CompareHashAndPassword([]byte(arg.PasswordHash.String), []byte("super-secret-pass")); err != nil {
			t.Fatalf("stored hash does not verify against password: %v", err)
		}
		return dbgen.User{
			ID:        createdID,
			Email:     arg.Email,
			Name:      arg.Name,
			CreatedAt: time.Now(),
		}, nil
	}
	stub.createRefreshTokenFn = func(ctx context.Context, arg dbgen.CreateRefreshTokenParams) (dbgen.RefreshToken, error) {
		if arg.UserID != createdID {
			t.Errorf("CreateRefreshToken UserID = %s, want %s", arg.UserID, createdID)
		}
		return dbgen.RefreshToken{}, nil
	}

	h := newEmailAuthHandler(t, stub, nil)
	resp, err := h.EmailRegister(context.Background(), connect.NewRequest(&toquiv1.EmailRegisterRequest{
		Email:    "  Alice@Example.com ",
		Password: "super-secret-pass",
		Name:     "Alice",
	}))
	if err != nil {
		t.Fatalf("EmailRegister: %v", err)
	}
	if resp.Msg.AccessToken == "" || resp.Msg.RefreshToken == "" {
		t.Fatal("expected non-empty tokens in response")
	}
	if resp.Msg.User.GetId() != createdID.String() {
		t.Errorf("user id = %q, want %q", resp.Msg.User.GetId(), createdID.String())
	}
	if len(stub.createUserCalls) != 1 {
		t.Fatalf("CreateUserWithPassword call count = %d, want 1", len(stub.createUserCalls))
	}
}

func TestEmailRegister_DuplicateEmailReturnsAlreadyExists(t *testing.T) {
	stub := &stubEmailAuthQueries{tb: t}
	stub.getUserByEmailFn = func(ctx context.Context, email string) (dbgen.User, error) {
		return dbgen.User{ID: uuid.New(), Email: email}, nil
	}

	h := newEmailAuthHandler(t, stub, nil)
	_, err := h.EmailRegister(context.Background(), connect.NewRequest(&toquiv1.EmailRegisterRequest{
		Email:    "alice@example.com",
		Password: "super-secret-pass",
		Name:     "Alice",
	}))
	if err == nil {
		t.Fatal("expected error for duplicate email, got nil")
	}
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("expected *connect.Error, got %T", err)
	}
	if ce.Code() != connect.CodeAlreadyExists {
		t.Errorf("code = %v, want CodeAlreadyExists", ce.Code())
	}
}

func TestEmailRegister_DomainAllowlistEnforced(t *testing.T) {
	stub := &stubEmailAuthQueries{tb: t}
	// No stub fns wired — domain check should reject before any DB call.

	h := newEmailAuthHandler(t, stub, []string{"toqui.travel"})
	_, err := h.EmailRegister(context.Background(), connect.NewRequest(&toquiv1.EmailRegisterRequest{
		Email:    "outsider@gmail.com",
		Password: "super-secret-pass",
		Name:     "Outsider",
	}))
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("expected *connect.Error, got %T (err=%v)", err, err)
	}
	if ce.Code() != connect.CodePermissionDenied {
		t.Errorf("code = %v, want CodePermissionDenied", ce.Code())
	}
}

func TestEmailLogin_HappyPath(t *testing.T) {
	userID := uuid.New()
	hash, err := bcrypt.GenerateFromPassword([]byte("super-secret-pass"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("setup hash: %v", err)
	}

	stub := &stubEmailAuthQueries{tb: t}
	stub.getUserPasswordHashFn = func(ctx context.Context, email string) (dbgen.GetUserPasswordHashRow, error) {
		if email != "alice@example.com" {
			t.Errorf("email not normalized: %q", email)
		}
		return dbgen.GetUserPasswordHashRow{
			ID:           userID,
			PasswordHash: pgtype.Text{String: string(hash), Valid: true},
		}, nil
	}
	stub.getUserByIDFn = func(ctx context.Context, id uuid.UUID) (dbgen.User, error) {
		if id != userID {
			t.Errorf("GetUserByID = %s, want %s", id, userID)
		}
		return dbgen.User{ID: userID, Email: "alice@example.com"}, nil
	}
	stub.createRefreshTokenFn = func(ctx context.Context, arg dbgen.CreateRefreshTokenParams) (dbgen.RefreshToken, error) {
		return dbgen.RefreshToken{}, nil
	}

	h := newEmailAuthHandler(t, stub, nil)
	resp, err := h.EmailLogin(context.Background(), connect.NewRequest(&toquiv1.EmailLoginRequest{
		Email:    "  ALICE@example.com",
		Password: "super-secret-pass",
	}))
	if err != nil {
		t.Fatalf("EmailLogin: %v", err)
	}
	if resp.Msg.AccessToken == "" || resp.Msg.RefreshToken == "" {
		t.Fatal("expected non-empty tokens")
	}
	if resp.Msg.User.GetId() != userID.String() {
		t.Errorf("user id = %q, want %q", resp.Msg.User.GetId(), userID.String())
	}
}

func TestEmailLogin_WrongPasswordReturnsUnauthenticated(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-pass"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("setup hash: %v", err)
	}

	stub := &stubEmailAuthQueries{tb: t}
	stub.getUserPasswordHashFn = func(ctx context.Context, email string) (dbgen.GetUserPasswordHashRow, error) {
		return dbgen.GetUserPasswordHashRow{
			ID:           uuid.New(),
			PasswordHash: pgtype.Text{String: string(hash), Valid: true},
		}, nil
	}

	h := newEmailAuthHandler(t, stub, nil)
	_, err = h.EmailLogin(context.Background(), connect.NewRequest(&toquiv1.EmailLoginRequest{
		Email:    "alice@example.com",
		Password: "wrong-pass",
	}))
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("expected *connect.Error, got %T (err=%v)", err, err)
	}
	if ce.Code() != connect.CodeUnauthenticated {
		t.Errorf("code = %v, want CodeUnauthenticated", ce.Code())
	}
}

func TestEmailLogin_UnknownEmailReturnsUnauthenticated(t *testing.T) {
	stub := &stubEmailAuthQueries{tb: t}
	stub.getUserPasswordHashFn = func(ctx context.Context, email string) (dbgen.GetUserPasswordHashRow, error) {
		return dbgen.GetUserPasswordHashRow{}, pgx.ErrNoRows
	}

	h := newEmailAuthHandler(t, stub, nil)
	_, err := h.EmailLogin(context.Background(), connect.NewRequest(&toquiv1.EmailLoginRequest{
		Email:    "nobody@example.com",
		Password: "any-password",
	}))
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("expected *connect.Error, got %T (err=%v)", err, err)
	}
	// Same Unauthenticated code as wrong-password — no enumeration leak.
	if ce.Code() != connect.CodeUnauthenticated {
		t.Errorf("code = %v, want CodeUnauthenticated", ce.Code())
	}
}

func TestEmailLogin_OAuthOnlyUserReturnsUnauthenticated(t *testing.T) {
	stub := &stubEmailAuthQueries{tb: t}
	stub.getUserPasswordHashFn = func(ctx context.Context, email string) (dbgen.GetUserPasswordHashRow, error) {
		return dbgen.GetUserPasswordHashRow{
			ID:           uuid.New(),
			PasswordHash: pgtype.Text{Valid: false},
		}, nil
	}

	h := newEmailAuthHandler(t, stub, nil)
	_, err := h.EmailLogin(context.Background(), connect.NewRequest(&toquiv1.EmailLoginRequest{
		Email:    "google-only@example.com",
		Password: "anything",
	}))
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("expected *connect.Error, got %T (err=%v)", err, err)
	}
	if ce.Code() != connect.CodeUnauthenticated {
		t.Errorf("code = %v, want CodeUnauthenticated", ce.Code())
	}
}

func TestGetAuthProviders_GoogleEnabled(t *testing.T) {
	h := newEmailAuthHandler(t, &stubEmailAuthQueries{tb: t}, nil).
		WithGoogleOAuthEnabled(true)

	resp, err := h.GetAuthProviders(context.Background(), connect.NewRequest(&toquiv1.GetAuthProvidersRequest{}))
	if err != nil {
		t.Fatalf("GetAuthProviders: %v", err)
	}
	if !resp.Msg.EmailPassword {
		t.Error("EmailPassword should always be true")
	}
	if !resp.Msg.GoogleOauth {
		t.Error("GoogleOauth should be true when WithGoogleOAuthEnabled(true)")
	}
}

func TestGetAuthProviders_GoogleDisabled(t *testing.T) {
	h := newEmailAuthHandler(t, &stubEmailAuthQueries{tb: t}, nil)
	// default googleOAuthEnabled=false

	resp, err := h.GetAuthProviders(context.Background(), connect.NewRequest(&toquiv1.GetAuthProvidersRequest{}))
	if err != nil {
		t.Fatalf("GetAuthProviders: %v", err)
	}
	if !resp.Msg.EmailPassword {
		t.Error("EmailPassword should always be true")
	}
	if resp.Msg.GoogleOauth {
		t.Error("GoogleOauth should be false when not configured")
	}
}

func TestGoogleLogin_UnimplementedWhenNotConfigured(t *testing.T) {
	h := newEmailAuthHandler(t, &stubEmailAuthQueries{tb: t}, nil)
	// googleOAuthEnabled defaults to false.

	_, err := h.GoogleLogin(context.Background(), connect.NewRequest(&toquiv1.GoogleLoginRequest{
		Code:        "code",
		RedirectUri: "http://localhost/cb",
	}))
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("expected *connect.Error, got %T", err)
	}
	if ce.Code() != connect.CodeUnimplemented {
		t.Errorf("code = %v, want CodeUnimplemented", ce.Code())
	}
}
