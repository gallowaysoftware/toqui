package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

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
	pool           *pgxpool.Pool
	queries        *dbgen.Queries
	lifecycleSvc   *lifecycle.Service
	allowedDomains []string
	authLimiter    *ratelimit.AuthLimiter

	// Facebook/Meta OAuth config
	facebookClientID     string
	facebookClientSecret string
}

func NewAuthHandler(authSvc *auth.Service, pool *pgxpool.Pool, lifecycleSvc *lifecycle.Service, allowedDomains []string, authLimiter *ratelimit.AuthLimiter) *AuthHandler {
	return &AuthHandler{
		authSvc:        authSvc,
		pool:           pool,
		queries:        dbgen.New(pool),
		lifecycleSvc:   lifecycleSvc,
		allowedDomains: allowedDomains,
		authLimiter:    authLimiter,
	}
}

// WithFacebookCredentials configures Facebook/Meta OAuth credentials for native app login.
func (h *AuthHandler) WithFacebookCredentials(clientID, clientSecret string) *AuthHandler {
	h.facebookClientID = clientID
	h.facebookClientSecret = clientSecret
	return h
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
		GoogleID:  pgtype.Text{String: info.ID, Valid: info.ID != ""},
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

	return connect.NewResponse(&toquiv1.GoogleLoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshResult.Token,
		User:         userToProto(&user),
	}), nil
}

func (h *AuthHandler) FacebookLogin(ctx context.Context, req *connect.Request[toquiv1.FacebookLoginRequest]) (*connect.Response[toquiv1.FacebookLoginResponse], error) {
	if h.facebookClientID == "" || h.facebookClientSecret == "" {
		return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("facebook login not configured"))
	}

	// Validate the Facebook access token and fetch user profile.
	fbUser, err := validateFacebookAccessToken(ctx, req.Msg.AccessToken, h.facebookClientID, h.facebookClientSecret)
	if err != nil {
		slog.Error("facebook token validation failed", "error", err)
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid Facebook access token"))
	}

	if fbUser.Email == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("email permission is required for Facebook login"))
	}

	// Domain allowlist
	if !isEmailDomainAllowed(fbUser.Email, h.allowedDomains) {
		audit.Log(audit.EventLoginDeniedDomain, "email", maskEmail(fbUser.Email))
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("email domain not allowed"))
	}

	// Use the shared findOrCreateFacebookUser logic via an inline OAuthHandler.
	// This avoids duplicating the user creation/linking logic.
	oauthH := &OAuthHandler{
		queries: h.queries,
	}
	user, err := oauthH.findOrCreateFacebookUser(ctx, fbUser)
	if err != nil {
		return nil, internalError(ctx, "facebook user upsert", err)
	}

	accessToken, err := h.authSvc.GenerateAccessToken(user.ID)
	if err != nil {
		return nil, internalError(ctx, "generate access token", err)
	}

	refreshResult, err := h.authSvc.GenerateRefreshToken(user.ID, uuid.Nil)
	if err != nil {
		return nil, internalError(ctx, "generate refresh token", err)
	}

	if _, err := h.queries.CreateRefreshToken(ctx, dbgen.CreateRefreshTokenParams{
		UserID:    user.ID,
		Jti:       refreshResult.JTI,
		Family:    refreshResult.Family,
		ExpiresAt: refreshResult.ExpiresAt,
	}); err != nil {
		return nil, internalError(ctx, "store refresh token", err)
	}

	audit.Log(audit.EventFacebookLogin, "user_id", user.ID.String(), "email", maskEmail(user.Email))

	return connect.NewResponse(&toquiv1.FacebookLoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshResult.Token,
		User:         userToProto(user),
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

	// Token rotation: verify the token is tracked and not revoked, then
	// revoke it and issue a new one IN THE SAME TRANSACTION. Without the
	// transaction + row lock, two concurrent refreshes with the same JTI
	// could both observe revoked=false and both succeed (TOCTOU).
	// Tokens without JTI (pre-rotation) are accepted but not rotated.
	family := claims.Family
	var refreshResult *auth.RefreshTokenResult
	var user dbgen.User

	if claims.JTI != "" {
		tx, err := h.pool.Begin(ctx)
		if err != nil {
			return nil, internalError(ctx, "begin refresh tx", err)
		}
		// Rollback is a no-op once Commit succeeds, so it's safe to defer
		// unconditionally.
		defer func() { _ = tx.Rollback(ctx) }()
		qtx := h.queries.WithTx(tx)

		// SELECT ... FOR UPDATE: block any concurrent RefreshToken RPC
		// carrying the same JTI until this transaction commits or rolls
		// back. The loser will see revoked=true and trip reuse detection.
		stored, err := qtx.GetRefreshTokenByJTIForUpdate(ctx, claims.JTI)
		if err != nil {
			// Token not in DB — could be pre-rotation or manually deleted.
			slog.Warn("refresh token JTI not found in database", "jti", claims.JTI)
			return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid refresh token"))
		}
		if stored.Revoked {
			// Token reuse detected — revoke entire family (breach).
			// Commit this transaction first so the family revoke actually
			// lands even if we bail immediately afterward.
			audit.Log(audit.EventTokenReuse,
				"user_id", claims.UserID.String(),
				"jti", claims.JTI,
				"family", stored.Family.String(),
				"ip", ip,
			)
			if revokeErr := qtx.RevokeRefreshTokenFamily(ctx, stored.Family); revokeErr != nil {
				slog.Error("failed to revoke token family on reuse detection",
					"error", revokeErr,
					"family", stored.Family.String(),
					"jti", claims.JTI,
				)
			}
			if commitErr := tx.Commit(ctx); commitErr != nil {
				slog.Error("failed to commit family revoke on reuse detection",
					"error", commitErr,
				)
			}
			return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid refresh token"))
		}
		// Revoke the current token (it's been used) inside the same tx.
		if err := qtx.RevokeRefreshToken(ctx, claims.JTI); err != nil {
			return nil, internalError(ctx, "revoke refresh token", err)
		}
		family = stored.Family

		user, err = qtx.GetUserByID(ctx, claims.UserID)
		if err != nil {
			return nil, internalError(ctx, "get user for refresh", err)
		}

		// Issue new refresh token in the same family (rotation).
		refreshResult, err = h.authSvc.GenerateRefreshToken(user.ID, family)
		if err != nil {
			return nil, internalError(ctx, "generate refresh token", err)
		}

		// Track the new token inside the same tx so revoke-old +
		// insert-new are atomic.
		if _, err := qtx.CreateRefreshToken(ctx, dbgen.CreateRefreshTokenParams{
			UserID:    user.ID,
			Jti:       refreshResult.JTI,
			Family:    refreshResult.Family,
			ExpiresAt: refreshResult.ExpiresAt,
		}); err != nil {
			return nil, internalError(ctx, "store refresh token", err)
		}

		if err := tx.Commit(ctx); err != nil {
			return nil, internalError(ctx, "commit refresh tx", err)
		}
	} else {
		// Pre-rotation token (no JTI): legacy path, no row to lock.
		var err error
		user, err = h.queries.GetUserByID(ctx, claims.UserID)
		if err != nil {
			return nil, internalError(ctx, "get user for refresh", err)
		}

		refreshResult, err = h.authSvc.GenerateRefreshToken(user.ID, family)
		if err != nil {
			return nil, internalError(ctx, "generate refresh token", err)
		}

		if _, err := h.queries.CreateRefreshToken(ctx, dbgen.CreateRefreshTokenParams{
			UserID:    user.ID,
			Jti:       refreshResult.JTI,
			Family:    refreshResult.Family,
			ExpiresAt: refreshResult.ExpiresAt,
		}); err != nil {
			return nil, internalError(ctx, "store refresh token", err)
		}
	}

	accessToken, err := h.authSvc.GenerateAccessToken(user.ID)
	if err != nil {
		return nil, internalError(ctx, "generate access token", err)
	}

	audit.Log(audit.EventTokenRefresh, "user_id", user.ID.String(), "ip", ip)

	return connect.NewResponse(&toquiv1.RefreshTokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshResult.Token,
		User:         userToProto(&user),
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

	return connect.NewResponse(&toquiv1.GetCurrentUserResponse{
		User: userToProto(&user),
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
		return nil, internalError(ctx, "request export", err)
	}

	audit.Log(audit.EventDataExport, "user_id", userID.String())

	return connect.NewResponse(&toquiv1.ExportDataResponse{
		RequestId: requestID.String(),
		Message:   "Your data export is being prepared. You'll be notified when it's ready to download.",
	}), nil
}

// HandleExportDownload serves the user's data export as a JSON download.
// GET /api/export/{requestID} — requires authentication, user must own the export.
//
// When the export has been persisted to GCS, the response redirects to the
// signed download URL. When using local storage or when no export store is
// configured, the export is served directly from the filesystem or regenerated
// live from the database.
func (h *AuthHandler) HandleExportDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := authenticateRESTRequest(r, h.authSvc)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Extract request ID from path: /api/export/{requestID}
	requestIDStr := strings.TrimPrefix(r.URL.Path, "/api/export/")
	if requestIDStr != "" {
		requestID, parseErr := uuid.Parse(requestIDStr)
		if parseErr == nil {
			// Look up the export request to check for a persisted download URL.
			exportReq, lookupErr := h.queries.GetExportRequestByID(r.Context(), requestID)
			if lookupErr == nil && exportReq.UserID == userID {
				if exportReq.DownloadUrl.Valid && exportReq.Status == "completed" {
					url := exportReq.DownloadUrl.String
					// Check expiry — if the signed URL has expired, fall through to regenerate.
					if exportReq.ExpiresAt.Valid && time.Now().Before(exportReq.ExpiresAt.Time) {
						// If the URL is an external URL (GCS signed URL), redirect to it.
						if strings.HasPrefix(url, "https://") {
							http.Redirect(w, r, url, http.StatusTemporaryRedirect)
							return
						}
						// If using local storage, the file was persisted locally.
						// Try to serve it via the lifecycle service's local store.
						if h.lifecycleSvc.HasLocalExport(requestID) {
							rc, openErr := h.lifecycleSvc.OpenLocalExport(requestID)
							if openErr == nil {
								defer rc.Close()
								w.Header().Set("Content-Type", "application/json")
								w.Header().Set("Content-Disposition", `attachment; filename="toqui-data-export.json"`)
								if _, copyErr := io.Copy(w, rc); copyErr != nil {
									slog.Error("failed to stream local export", "error", copyErr)
								}
								return
							}
							slog.Warn("local export file not found, regenerating", "request_id", requestID, "error", openErr)
						}
					}
				}
			}
		}
	}

	// Fallback: generate the export live from the database.
	export, err := h.lifecycleSvc.ExportUserData(r.Context(), userID)
	if err != nil {
		slog.Error("export download failed", "user_id", userID, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="toqui-data-export.json"`)
	if err := json.NewEncoder(w).Encode(export); err != nil {
		slog.Error("failed to encode export", "error", err)
	}
}

func userToProto(u *dbgen.User) *toquiv1.User {
	user := &toquiv1.User{
		Id:    u.ID.String(),
		Email: u.Email,
	}
	if u.Name.Valid {
		user.Name = u.Name.String
	}
	if u.AvatarUrl.Valid {
		user.AvatarUrl = u.AvatarUrl.String
	}
	return user
}
