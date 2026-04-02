package handlers

import (
	"context"
	"fmt"
	"log/slog"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gallowaysoftware/toqui-backend/internal/audit"
	"github.com/gallowaysoftware/toqui-backend/internal/auth"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui-backend/internal/lifecycle"
	"github.com/gallowaysoftware/toqui-backend/internal/ratelimit"

	toquiv1 "github.com/gallowaysoftware/toqui-backend/gen/toqui/v1"
)

type AuthHandler struct {
	authSvc        *auth.Service
	queries        *dbgen.Queries
	lifecycleSvc   *lifecycle.Service
	allowedDomains []string
	authLimiter    *ratelimit.AuthLimiter
}

func NewAuthHandler(authSvc *auth.Service, pool *pgxpool.Pool, lifecycleSvc *lifecycle.Service, allowedDomains []string, authLimiter *ratelimit.AuthLimiter) *AuthHandler {
	return &AuthHandler{
		authSvc:        authSvc,
		queries:        dbgen.New(pool),
		lifecycleSvc:   lifecycleSvc,
		allowedDomains: allowedDomains,
		authLimiter:    authLimiter,
	}
}

func (h *AuthHandler) GoogleLogin(ctx context.Context, req *connect.Request[toquiv1.GoogleLoginRequest]) (*connect.Response[toquiv1.GoogleLoginResponse], error) {
	info, err := h.authSvc.ExchangeCode(ctx, req.Msg.Code, auth.ExchangeCodeOpts{
		RedirectURI:  req.Msg.RedirectUri,
		CodeVerifier: req.Msg.CodeVerifier,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Domain allowlist: reject signups from unauthorized email domains.
	if !isEmailDomainAllowed(info.Email, h.allowedDomains) {
		audit.Log(audit.EventLoginDeniedDomain, "email", maskEmail(info.Email))
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("email domain not allowed"))
	}

	user, err := h.queries.UpsertUserByGoogleID(ctx, dbgen.UpsertUserByGoogleIDParams{
		GoogleID:  info.ID,
		Email:     info.Email,
		Name:      pgtype.Text{String: info.Name, Valid: info.Name != ""},
		AvatarUrl: pgtype.Text{String: info.AvatarURL, Valid: info.AvatarURL != ""},
	})
	if err != nil {
		return nil, internalError(ctx, "upsert user", err)
	}

	accessToken, err := h.authSvc.GenerateAccessToken(user.ID)
	if err != nil {
		return nil, internalError(ctx, "generate access token", err)
	}

	// New login → new token family.
	refreshResult, err := h.authSvc.GenerateRefreshToken(user.ID, uuid.Nil)
	if err != nil {
		return nil, internalError(ctx, "generate refresh token", err)
	}

	// Track the refresh token server-side for rotation.
	if _, err := h.queries.CreateRefreshToken(ctx, dbgen.CreateRefreshTokenParams{
		UserID:    user.ID,
		Jti:       refreshResult.JTI,
		Family:    refreshResult.Family,
		ExpiresAt: refreshResult.ExpiresAt,
	}); err != nil {
		return nil, internalError(ctx, "store refresh token", err)
	}

	audit.Log(audit.EventLogin, "user_id", user.ID.String(), "email", maskEmail(user.Email))

	tier := h.lookupTier(ctx, user.ID)

	return connect.NewResponse(&toquiv1.GoogleLoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshResult.Token,
		User:         userToProto(&user, tier),
	}), nil
}

func (h *AuthHandler) RefreshToken(ctx context.Context, req *connect.Request[toquiv1.RefreshTokenRequest]) (*connect.Response[toquiv1.RefreshTokenResponse], error) {
	ip := clientIPFromHeaders(req.Header())
	if h.authLimiter != nil && h.authLimiter.IsBlocked(ip) {
		return nil, connect.NewError(connect.CodeResourceExhausted, fmt.Errorf("too many failed attempts — please try again later"))
	}

	claims, err := h.authSvc.ValidateRefreshToken(req.Msg.RefreshToken)
	if err != nil {
		if h.authLimiter != nil {
			h.authLimiter.RecordFailure(ip)
		}
		audit.Log(audit.EventTokenRefreshDenied, "ip", ip, "reason", "invalid_token")
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid refresh token"))
	}

	if h.authLimiter != nil {
		h.authLimiter.ClearFailures(ip)
	}

	// Token rotation: verify the token is tracked and not revoked.
	// Tokens without JTI (pre-rotation) are accepted but not rotated.
	family := claims.Family
	if claims.JTI != "" {
		stored, err := h.queries.GetRefreshTokenByJTI(ctx, claims.JTI)
		if err != nil {
			// Token not in DB — could be pre-rotation or manually deleted.
			slog.Warn("refresh token JTI not found in database", "jti", claims.JTI)
			return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid refresh token"))
		}
		if stored.Revoked {
			// Token reuse detected — revoke entire family (breach).
			audit.Log(audit.EventTokenReuse,
				"user_id", claims.UserID.String(),
				"jti", claims.JTI,
				"family", stored.Family.String(),
				"ip", ip,
			)
			if revokeErr := h.queries.RevokeRefreshTokenFamily(ctx, stored.Family); revokeErr != nil {
				slog.Error("failed to revoke token family on reuse detection",
					"error", revokeErr,
					"family", stored.Family.String(),
					"jti", claims.JTI,
				)
			}
			return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid refresh token"))
		}
		// Revoke the current token (it's been used).
		if revokeErr := h.queries.RevokeRefreshToken(ctx, claims.JTI); revokeErr != nil {
			slog.Error("failed to revoke consumed refresh token",
				"error", revokeErr,
				"jti", claims.JTI,
			)
		}
		family = stored.Family
	}

	user, err := h.queries.GetUserByID(ctx, claims.UserID)
	if err != nil {
		return nil, internalError(ctx, "get user for refresh", err)
	}

	accessToken, err := h.authSvc.GenerateAccessToken(user.ID)
	if err != nil {
		return nil, internalError(ctx, "generate access token", err)
	}

	// Issue new refresh token in the same family (rotation).
	refreshResult, err := h.authSvc.GenerateRefreshToken(user.ID, family)
	if err != nil {
		return nil, internalError(ctx, "generate refresh token", err)
	}

	// Track the new token.
	if _, err := h.queries.CreateRefreshToken(ctx, dbgen.CreateRefreshTokenParams{
		UserID:    user.ID,
		Jti:       refreshResult.JTI,
		Family:    refreshResult.Family,
		ExpiresAt: refreshResult.ExpiresAt,
	}); err != nil {
		return nil, internalError(ctx, "store refresh token", err)
	}

	audit.Log(audit.EventTokenRefresh, "user_id", user.ID.String(), "ip", ip)

	tier := h.lookupTier(ctx, user.ID)

	return connect.NewResponse(&toquiv1.RefreshTokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshResult.Token,
		User:         userToProto(&user, tier),
	}), nil
}

func (h *AuthHandler) GetCurrentUser(ctx context.Context, _ *connect.Request[toquiv1.GetCurrentUserRequest]) (*connect.Response[toquiv1.GetCurrentUserResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	user, err := h.queries.GetUserByID(ctx, userID)
	if err != nil {
		return nil, internalError(ctx, "get current user", err)
	}

	tier := h.lookupTier(ctx, user.ID)

	return connect.NewResponse(&toquiv1.GetCurrentUserResponse{
		User: userToProto(&user, tier),
	}), nil
}

func (h *AuthHandler) DeleteAccount(ctx context.Context, req *connect.Request[toquiv1.DeleteAccountRequest]) (*connect.Response[toquiv1.DeleteAccountResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	if !req.Msg.Confirm {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("must set confirm=true to delete account"))
	}

	requestID, err := h.lifecycleSvc.RequestDeletion(ctx, userID)
	if err != nil {
		return nil, internalError(ctx, "request deletion", err)
	}

	audit.Log(audit.EventAccountDelete, "user_id", userID.String())

	return connect.NewResponse(&toquiv1.DeleteAccountResponse{
		RequestId: requestID.String(),
		Message:   "Your account deletion has been requested and is being processed. All associated data will be permanently removed.",
	}), nil
}

func (h *AuthHandler) ExportData(ctx context.Context, _ *connect.Request[toquiv1.ExportDataRequest]) (*connect.Response[toquiv1.ExportDataResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	requestID, err := h.lifecycleSvc.RequestExport(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("data export is coming soon"))
	}

	audit.Log(audit.EventDataExport, "user_id", userID.String())

	return connect.NewResponse(&toquiv1.ExportDataResponse{
		RequestId: requestID.String(),
		Message:   "Your data export is being prepared. You'll be notified when it's ready to download.",
	}), nil
}

func (h *AuthHandler) lookupTier(ctx context.Context, userID uuid.UUID) string {
	if raw, err := h.queries.GetUserSubscriptionTier(ctx, userID); err == nil {
		return raw
	}
	return "free"
}

func userToProto(u *dbgen.User, subscriptionTier string) *toquiv1.User {
	if subscriptionTier == "" {
		subscriptionTier = "free"
	}
	user := &toquiv1.User{
		Id:               u.ID.String(),
		Email:            u.Email,
		SubscriptionTier: subscriptionTier,
	}
	if u.Name.Valid {
		user.Name = u.Name.String
	}
	if u.AvatarUrl.Valid {
		user.AvatarUrl = u.AvatarUrl.String
	}
	return user
}
