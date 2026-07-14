package handlers

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/gallowaysoftware/toqui/backend/internal/audit"
	"github.com/gallowaysoftware/toqui/backend/internal/dbgen"

	toquiv1 "github.com/gallowaysoftware/toqui/backend/gen/toqui/v1"
)

// OIDCLogin completes a generic OpenID Connect sign-in. The client has
// already run the authorization-code + PKCE flow against the configured
// issuer (Authelia, Authentik, ...); it hands the code here, the backend
// exchanges + verifies the ID token, and finds-or-creates the user by the
// provider's verified email. Same token-issuing path as every other login.
func (h *AuthHandler) OIDCLogin(ctx context.Context, req *connect.Request[toquiv1.OIDCLoginRequest]) (*connect.Response[toquiv1.OIDCLoginResponse], error) {
	if h.oidcProvider == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errors.New("oidc not configured"))
	}

	info, err := h.oidcProvider.ExchangeAndVerify(ctx, req.Msg.Code, req.Msg.RedirectUri, req.Msg.CodeVerifier)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Domain allowlist applies to SSO too (defence in depth — the IdP is the
	// primary gate). Operators who want any-domain SSO leave it empty.
	if !isEmailDomainAllowed(info.Email, h.allowedDomains) {
		audit.Log(audit.EventLoginDeniedDomain, "email", maskEmail(info.Email))
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("email domain not allowed"))
	}

	user, err := h.queries.UpsertUserByEmail(ctx, dbgen.UpsertUserByEmailParams{
		Email: info.Email,
		Name:  pgtype.Text{String: info.Name, Valid: info.Name != ""},
	})
	if err != nil {
		return nil, internalError(ctx, "upsert user", err)
	}

	accessToken, refreshResult, cerr := h.issueTokens(ctx, user.ID)
	if cerr != nil {
		return nil, cerr
	}

	audit.Log(audit.EventLogin, "user_id", user.ID.String(), "email", maskEmail(user.Email))

	return connect.NewResponse(&toquiv1.OIDCLoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshResult.Token,
		User:         userToProto(&user),
	}), nil
}
