package handlers

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"

	"github.com/gallowaysoftware/toqui/backend/internal/audit"
	"github.com/gallowaysoftware/toqui/backend/internal/auth"
	"github.com/gallowaysoftware/toqui/backend/internal/dbgen"

	toquiv1 "github.com/gallowaysoftware/toqui/backend/gen/toqui/v1"
)

// emailAuthQueries narrows the dbgen surface used by the email-auth
// handlers so unit tests can supply a fail-loud stub without standing
// up a real Postgres. *dbgen.Queries satisfies this interface for
// free; tests use a hand-written stub in auth_email_test.go.
type emailAuthQueries interface {
	GetUserByEmail(ctx context.Context, email string) (dbgen.User, error)
	GetUserPasswordHash(ctx context.Context, email string) (dbgen.GetUserPasswordHashRow, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (dbgen.User, error)
	CreateUserWithPassword(ctx context.Context, arg dbgen.CreateUserWithPasswordParams) (dbgen.User, error)
	CreateRefreshToken(ctx context.Context, arg dbgen.CreateRefreshTokenParams) (dbgen.RefreshToken, error)
}

// emailQueries lets tests swap out the dbgen-backed query implementation.
// In production this returns h.emailQueries() (which satisfies emailAuthQueries
// implicitly); in tests, auth_email_test.go assigns its stub via
// withEmailQueriesForTest.
func (h *AuthHandler) emailQueries() emailAuthQueries {
	if h.testEmailQueries != nil {
		return h.testEmailQueries
	}
	return h.emailQueries()
}

// bcryptCost is the work factor used for password hashing. Cost 12 is
// ~250ms on a modern CPU — industry standard as of 2026. Bumping this is
// safe (existing hashes carry their original cost embedded in the encoded
// string), but lowering it weakens protection for new users.
const bcryptCost = 12

// dummyBcryptHash is a syntactically valid bcrypt $2a$12 hash that no
// real password will ever match. EmailLogin runs bcrypt against this
// when the user record is missing or has no password_hash so that
// successful enumeration of valid emails via timing is impossible.
const dummyBcryptHash = "$2a$12$Yws/4HRXuS6qzCgvjcr8.eGmKxRMmqYUaOAVtwAh6Ut2c2ckQzfgi"

// EmailRegister creates a new user with a bcrypt-hashed password and
// returns access + refresh tokens. Unlike EmailLogin, this RPC reveals
// when an email is already registered — registration UX is dead
// without that signal, and the account-enumeration risk is much lower
// at the signup surface than at the login surface.
func (h *AuthHandler) EmailRegister(ctx context.Context, req *connect.Request[toquiv1.EmailRegisterRequest]) (*connect.Response[toquiv1.EmailRegisterResponse], error) {
	ip := clientIPFromHeaders(req.Header())
	if h.authLimiter != nil && h.authLimiter.IsBlocked(ip) {
		return nil, connect.NewError(connect.CodeResourceExhausted, errors.New("too many failed attempts — please try again later"))
	}

	email := strings.ToLower(strings.TrimSpace(req.Msg.Email))
	name := strings.TrimSpace(req.Msg.Name)
	password := req.Msg.Password

	// Domain allowlist applies to all sign-up paths.
	if !isEmailDomainAllowed(email, h.allowedDomains) {
		audit.Log(audit.EventLoginDeniedDomain, "email", maskEmail(email))
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("email domain not allowed"))
	}

	// Reject collisions explicitly. Self-hosted operators run on small
	// user bases where account-enumeration via signup matters less than
	// giving the user a clear "you already have an account" path.
	if _, err := h.emailQueries().GetUserByEmail(ctx, email); err == nil {
		return nil, connect.NewError(connect.CodeAlreadyExists, errors.New("email already registered"))
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, internalError(ctx, "lookup existing email", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return nil, internalError(ctx, "hash password", err)
	}

	user, err := h.emailQueries().CreateUserWithPassword(ctx, dbgen.CreateUserWithPasswordParams{
		Email:        email,
		Name:         pgtype.Text{String: name, Valid: name != ""},
		PasswordHash: pgtype.Text{String: string(hash), Valid: true},
	})
	if err != nil {
		return nil, internalError(ctx, "create user with password", err)
	}

	accessToken, refreshResult, terr := h.issueTokens(ctx, user.ID)
	if terr != nil {
		return nil, terr
	}

	audit.Log(audit.EventLogin, "user_id", user.ID.String(), "email", maskEmail(user.Email), "method", "email_register")

	return connect.NewResponse(&toquiv1.EmailRegisterResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshResult.Token,
		User:         userToProto(&user),
	}), nil
}

// EmailLogin verifies a bcrypt-hashed password and returns tokens on
// success. All failure modes — unknown email, missing hash (OAuth-only
// user), wrong password — collapse to the same Unauthenticated error
// to prevent account enumeration. A dummy bcrypt comparison runs on
// missing-user / missing-hash so timing stays equivalent across paths.
func (h *AuthHandler) EmailLogin(ctx context.Context, req *connect.Request[toquiv1.EmailLoginRequest]) (*connect.Response[toquiv1.EmailLoginResponse], error) {
	ip := clientIPFromHeaders(req.Header())
	if h.authLimiter != nil && h.authLimiter.IsBlocked(ip) {
		return nil, connect.NewError(connect.CodeResourceExhausted, errors.New("too many failed attempts — please try again later"))
	}

	email := strings.ToLower(strings.TrimSpace(req.Msg.Email))
	password := req.Msg.Password

	hashRow, lookupErr := h.emailQueries().GetUserPasswordHash(ctx, email)
	hashStr := dummyBcryptHash
	hashUsable := false
	if lookupErr == nil && hashRow.PasswordHash.Valid && hashRow.PasswordHash.String != "" {
		hashStr = hashRow.PasswordHash.String
		hashUsable = true
	} else if lookupErr != nil && !errors.Is(lookupErr, pgx.ErrNoRows) {
		return nil, internalError(ctx, "lookup password hash", lookupErr)
	}

	// Always run bcrypt so unknown-user / OAuth-only-user / wrong-password
	// all take roughly the same wall time.
	bcryptErr := bcrypt.CompareHashAndPassword([]byte(hashStr), []byte(password))

	if !hashUsable || bcryptErr != nil {
		if h.authLimiter != nil {
			h.authLimiter.RecordFailure(ip)
		}
		audit.Log(audit.EventTokenRefreshDenied, "ip", ip, "reason", "email_login_failed", "email", maskEmail(email))
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid email or password"))
	}

	if h.authLimiter != nil {
		h.authLimiter.ClearFailures(ip)
	}

	user, err := h.emailQueries().GetUserByID(ctx, hashRow.ID)
	if err != nil {
		return nil, internalError(ctx, "load user after password verify", err)
	}

	accessToken, refreshResult, terr := h.issueTokens(ctx, user.ID)
	if terr != nil {
		return nil, terr
	}

	audit.Log(audit.EventLogin, "user_id", user.ID.String(), "email", maskEmail(user.Email), "method", "email_login")

	return connect.NewResponse(&toquiv1.EmailLoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshResult.Token,
		User:         userToProto(&user),
	}), nil
}

// GetAuthProviders reports which auth methods this deployment supports.
// Frontends call this before render to decide which buttons to draw.
// Public — runs before the user has a token, hence listed in the
// auth interceptor's allowlist.
func (h *AuthHandler) GetAuthProviders(_ context.Context, _ *connect.Request[toquiv1.GetAuthProvidersRequest]) (*connect.Response[toquiv1.GetAuthProvidersResponse], error) {
	return connect.NewResponse(&toquiv1.GetAuthProvidersResponse{
		EmailPassword: true,
		GoogleOauth:   h.googleOAuthEnabled,
	}), nil
}

// issueTokens mints an access token plus a fresh refresh-token family
// and persists the refresh-token row. Returns a connect error already
// wrapped via internalError on failure.
func (h *AuthHandler) issueTokens(ctx context.Context, userID uuid.UUID) (string, *auth.RefreshTokenResult, *connect.Error) {
	accessToken, err := h.authSvc.GenerateAccessToken(userID)
	if err != nil {
		return "", nil, internalError(ctx, "generate access token", err)
	}

	refreshResult, err := h.authSvc.GenerateRefreshToken(userID, uuid.Nil)
	if err != nil {
		return "", nil, internalError(ctx, "generate refresh token", err)
	}

	if _, err := h.emailQueries().CreateRefreshToken(ctx, dbgen.CreateRefreshTokenParams{
		UserID:    userID,
		Jti:       refreshResult.JTI,
		Family:    refreshResult.Family,
		ExpiresAt: refreshResult.ExpiresAt,
	}); err != nil {
		return "", nil, internalError(ctx, "store refresh token", err)
	}

	return accessToken, refreshResult, nil
}
