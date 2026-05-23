package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gallowaysoftware/toqui-backend/internal/audit"
	"github.com/gallowaysoftware/toqui-backend/internal/auth"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui-backend/internal/email"
	"github.com/gallowaysoftware/toqui-backend/internal/ratelimit"
)

type OAuthHandler struct {
	authSvc        *auth.Service
	pool           *pgxpool.Pool
	queries        *dbgen.Queries
	frontendURL    string
	secureCookies  bool
	allowedDomains []string
	authLimiter    *ratelimit.AuthLimiter
	emailSvc       *email.Sender

	// googleOAuthEnabled gates HandleLogin / HandleCallback. When false,
	// /auth/google/* returns 501 Not Implemented so self-hosters who
	// haven't provisioned a Google OAuth client don't see broken redirects.
	googleOAuthEnabled bool
}

func NewOAuthHandler(authSvc *auth.Service, pool *pgxpool.Pool, frontendURL string, secureCookies bool, allowedDomains []string, authLimiter *ratelimit.AuthLimiter, emailSvc *email.Sender) *OAuthHandler {
	return &OAuthHandler{
		authSvc:        authSvc,
		pool:           pool,
		queries:        dbgen.New(pool),
		frontendURL:    frontendURL,
		emailSvc:       emailSvc,
		secureCookies:  secureCookies,
		allowedDomains: allowedDomains,
		authLimiter:    authLimiter,
	}
}

// WithGoogleOAuthEnabled toggles the Google OAuth path. When false,
// HandleLogin and HandleCallback return 501.
func (h *OAuthHandler) WithGoogleOAuthEnabled(enabled bool) *OAuthHandler {
	h.googleOAuthEnabled = enabled
	return h
}

func (h *OAuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if !h.googleOAuthEnabled {
		http.Error(w, "google oauth not configured", http.StatusNotImplemented)
		return
	}

	state := generateState()

	// Set Domain so the cookie is available on the callback subdomain (api.*).
	domain := ""
	if h.secureCookies {
		domain = ".toqui.travel"
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		Domain:   domain,
		MaxAge:   300,
		HttpOnly: true,
		Secure:   h.secureCookies,
		SameSite: http.SameSiteLaxMode,
	})

	// If a return URL is requested (e.g. admin), store it in a cookie.
	if ret := r.URL.Query().Get("return"); ret == "admin" {
		http.SetCookie(w, &http.Cookie{
			Name:     "oauth_return",
			Value:    "admin",
			Path:     "/",
			Domain:   domain,
			MaxAge:   300,
			HttpOnly: true,
			Secure:   h.secureCookies,
			SameSite: http.SameSiteLaxMode,
		})
	}

	// PKCE: generate code verifier + challenge, store verifier in cookie.
	verifier, challenge, err := auth.GeneratePKCE()
	if err != nil {
		slog.Error("PKCE generation failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_pkce",
		Value:    verifier,
		Path:     "/",
		Domain:   domain,
		MaxAge:   300,
		HttpOnly: true,
		Secure:   h.secureCookies,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, h.authSvc.AuthCodeURL(state, challenge), http.StatusTemporaryRedirect)
}

// oauthResult is the JSON payload stored in the temporary cookie
// and returned by HandleExchange.
type oauthResult struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	UserID       string `json:"user_id"`
	Email        string `json:"email"`
	Name         string `json:"name,omitempty"`
	AvatarURL    string `json:"avatar_url,omitempty"`
	ExpiresAt    int64  `json:"expires_at"`
}

// exchangeResponse is the JSON response from POST /auth/exchange.
// Tokens are in HttpOnly cookies — the body only contains user info and expiry.
type exchangeResponse struct {
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
	Name      string `json:"name,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
	ExpiresAt int64  `json:"expires_at"`
}

// refreshResponse is the JSON response from POST /auth/refresh.
// Tokens are in HttpOnly cookies — the body only contains user info and expiry.
type refreshResponse struct {
	User      refreshUser `json:"user"`
	ExpiresAt int64       `json:"expires_at"`
}

type refreshUser struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

func (h *OAuthHandler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	if !h.googleOAuthEnabled {
		http.Error(w, "google oauth not configured", http.StatusNotImplemented)
		return
	}

	stateCookie, err := r.Cookie("oauth_state")
	if err != nil || stateCookie.Value == "" || stateCookie.Value != r.URL.Query().Get("state") {
		http.Redirect(w, r, h.frontendURL+"/?error=invalid_state", http.StatusTemporaryRedirect)
		return
	}

	// Clear the state cookie
	http.SetCookie(w, &http.Cookie{
		Name:   "oauth_state",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	// Retrieve PKCE verifier from cookie
	var codeVerifier string
	if pkceCookie, pkceErr := r.Cookie("oauth_pkce"); pkceErr == nil {
		codeVerifier = pkceCookie.Value
	}
	// Clear the PKCE cookie
	http.SetCookie(w, &http.Cookie{
		Name:   "oauth_pkce",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Redirect(w, r, h.frontendURL+"/?error=missing_code", http.StatusTemporaryRedirect)
		return
	}

	info, err := h.authSvc.ExchangeCode(r.Context(), code, auth.ExchangeCodeOpts{
		CodeVerifier: codeVerifier,
	})
	if err != nil {
		http.Redirect(w, r, h.frontendURL+"/?error=exchange_failed", http.StatusTemporaryRedirect)
		return
	}

	// Domain allowlist: reject signups from unauthorized email domains.
	if !isEmailDomainAllowed(info.Email, h.allowedDomains) {
		audit.Log(audit.EventLoginDeniedDomain, "email", maskEmail(info.Email))
		http.Redirect(w, r, h.frontendURL+"/?error=domain_not_allowed", http.StatusTemporaryRedirect)
		return
	}

	user, err := h.queries.UpsertUserByGoogleID(r.Context(), dbgen.UpsertUserByGoogleIDParams{
		GoogleID:  pgtype.Text{String: info.ID, Valid: info.ID != ""},
		Email:     info.Email,
		Name:      pgtype.Text{String: info.Name, Valid: info.Name != ""},
		AvatarUrl: pgtype.Text{String: info.AvatarURL, Valid: info.AvatarURL != ""},
	})
	if err != nil {
		http.Redirect(w, r, h.frontendURL+"/?error=db_error", http.StatusTemporaryRedirect)
		return
	}

	// New user detection: created within the last minute means this is a signup, not a returning login.
	isNewUser := time.Since(user.CreatedAt) < time.Minute

	// Send welcome email for new users.
	if h.emailSvc != nil && isNewUser {
		name := ""
		if user.Name.Valid {
			name = user.Name.String
		}
		go func() {
			if err := h.emailSvc.SendWelcome(user.Email, name, h.frontendURL); err != nil {
				slog.Error("welcome email failed", "error", err, "user_id", user.ID)
			}
		}()
	}

	accessToken, err := h.authSvc.GenerateAccessToken(user.ID)
	if err != nil {
		http.Redirect(w, r, h.frontendURL+"/?error=token_error", http.StatusTemporaryRedirect)
		return
	}

	// New login → new token family.
	refreshResult, err := h.authSvc.GenerateRefreshToken(user.ID, uuid.Nil)
	if err != nil {
		http.Redirect(w, r, h.frontendURL+"/?error=token_error", http.StatusTemporaryRedirect)
		return
	}

	// Track the refresh token server-side for rotation.
	if _, dbErr := h.queries.CreateRefreshToken(r.Context(), dbgen.CreateRefreshTokenParams{
		UserID:    user.ID,
		Jti:       refreshResult.JTI,
		Family:    refreshResult.Family,
		ExpiresAt: refreshResult.ExpiresAt,
	}); dbErr != nil {
		slog.Error("store refresh token", "error", dbErr)
		http.Redirect(w, r, h.frontendURL+"/?error=internal", http.StatusTemporaryRedirect)
		return
	}

	// Store auth result in a short-lived HttpOnly cookie instead of URL params.
	result := oauthResult{
		AccessToken:  accessToken,
		RefreshToken: refreshResult.Token,
		UserID:       user.ID.String(),
		Email:        user.Email,
	}
	if user.Name.Valid {
		result.Name = user.Name.String
	}
	if user.AvatarUrl.Valid {
		result.AvatarURL = user.AvatarUrl.String
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		slog.Error("marshal oauth result", "error", err)
		http.Redirect(w, r, h.frontendURL+"/?error=internal", http.StatusTemporaryRedirect)
		return
	}

	audit.Log(audit.EventLogin, "user_id", user.ID.String(), "email", maskEmail(user.Email))

	// If the login was initiated from admin, pass the access token via URL fragment.
	// Admin is on a separate domain (Cloudflare Pages) so cookies won't work cross-origin.
	// The token is in the URL fragment (#) which is never sent to the server.
	if c, err := r.Cookie("oauth_return"); err == nil && c.Value == "admin" {
		clearDomain := ""
		if h.secureCookies {
			clearDomain = ".toqui.travel"
		}
		http.SetCookie(w, &http.Cookie{Name: "oauth_return", Value: "", Path: "/", Domain: clearDomain, MaxAge: -1})
		http.Redirect(w, r, "https://admin.toqui.travel/#token="+result.AccessToken, http.StatusTemporaryRedirect)
		return
	}

	// Normal flow: set OAuth result cookie, redirect to frontend for exchange.
	auth.SetOAuthResultCookie(w, string(resultJSON), h.secureCookies)
	http.Redirect(w, r, h.frontendURL+"/auth/callback", http.StatusTemporaryRedirect)
}

// HandleExchange reads the temporary OAuth result cookie, sets HttpOnly auth
// cookies, clears the temporary cookie, and returns user info (without tokens)
// in the response body. Tokens are only delivered via HttpOnly cookies.
func (h *OAuthHandler) HandleExchange(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB limit

	ip := ratelimit.ExtractClientIP(r)
	if h.authLimiter != nil && h.authLimiter.IsBlocked(ip) {
		http.Error(w, "Too many failed attempts. Please try again later.", http.StatusTooManyRequests)
		return
	}

	resultStr := auth.OAuthResultFromCookies(r)
	if resultStr == "" {
		if h.authLimiter != nil {
			h.authLimiter.RecordFailure(ip)
		}
		http.Error(w, "no pending auth result", http.StatusBadRequest)
		return
	}

	if h.authLimiter != nil {
		h.authLimiter.ClearFailures(ip)
	}

	// Parse the result to extract tokens for cookie setting.
	var result oauthResult
	if err := json.Unmarshal([]byte(resultStr), &result); err != nil {
		slog.Error("unmarshal oauth result for cookie auth", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Clear the one-time OAuth result cookie.
	auth.ClearOAuthResultCookie(w, h.secureCookies)

	// Set persistent HttpOnly auth cookies for web browser sessions.
	auth.SetAuthCookies(w, result.AccessToken, result.RefreshToken, h.secureCookies)

	// Return user info + expiry only — tokens are in HttpOnly cookies.
	resp := exchangeResponse{
		UserID:    result.UserID,
		Email:     result.Email,
		Name:      result.Name,
		AvatarURL: result.AvatarURL,
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("encode exchange response", "error", err)
	}
}

// HandleRefresh handles POST /auth/refresh — cookie-based token refresh for web browsers.
// Reads the refresh token from the toqui_refresh HttpOnly cookie, performs token rotation,
// sets new auth cookies, and returns user info + new expiry.
// Native apps use the gRPC RefreshToken RPC instead.
func (h *OAuthHandler) HandleRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB limit

	ip := ratelimit.ExtractClientIP(r)
	if h.authLimiter != nil && h.authLimiter.IsBlocked(ip) {
		http.Error(w, "Too many failed attempts. Please try again later.", http.StatusTooManyRequests)
		return
	}

	refreshToken := auth.RefreshTokenFromCookie(r)
	if refreshToken == "" {
		if h.authLimiter != nil {
			h.authLimiter.RecordFailure(ip)
		}
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	claims, err := h.authSvc.ValidateRefreshToken(refreshToken)
	if err != nil {
		if h.authLimiter != nil {
			h.authLimiter.RecordFailure(ip)
		}
		audit.Log(audit.EventTokenRefreshDenied, "ip", ip, "reason", "invalid_token")
		// Clear stale cookies on invalid token.
		auth.ClearAuthCookies(w, h.secureCookies)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if h.authLimiter != nil {
		h.authLimiter.ClearFailures(ip)
	}

	ctx := r.Context()

	// Token rotation inside a transaction with SELECT ... FOR UPDATE so two
	// concurrent refreshes with the same JTI cannot both observe
	// revoked=false and each issue a new token. The loser blocks on the
	// row lock until the winner commits, then sees revoked=true and trips
	// reuse detection. See toqui-backend#369 (P1 #2).
	family := claims.Family
	var refreshResult *auth.RefreshTokenResult
	var user dbgen.User

	if claims.JTI != "" {
		tx, txErr := h.pool.Begin(ctx)
		if txErr != nil {
			slog.Error("begin refresh tx", "error", txErr)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		defer func() { _ = tx.Rollback(ctx) }()
		qtx := h.queries.WithTx(tx)

		stored, dbErr := qtx.GetRefreshTokenByJTIForUpdate(ctx, claims.JTI)
		if dbErr != nil {
			slog.Warn("refresh token JTI not found in database", "jti", claims.JTI)
			auth.ClearAuthCookies(w, h.secureCookies)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if stored.Revoked {
			// Token reuse detected — revoke entire family (breach). Commit
			// so the family revoke persists even though we bail right after.
			audit.Log(audit.EventTokenReuse,
				"user_id", claims.UserID.String(),
				"jti", claims.JTI,
				"family", stored.Family.String(),
				"ip", ip,
			)
			if revokeErr := qtx.RevokeRefreshTokenFamily(ctx, stored.Family); revokeErr != nil {
				slog.Error("revoke token family on reuse detection", "error", revokeErr, "family", stored.Family.String())
			}
			if commitErr := tx.Commit(ctx); commitErr != nil {
				slog.Error("commit family revoke on reuse detection", "error", commitErr)
			}
			auth.ClearAuthCookies(w, h.secureCookies)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if revokeErr := qtx.RevokeRefreshToken(ctx, claims.JTI); revokeErr != nil {
			slog.Error("revoke used refresh token", "error", revokeErr, "jti", claims.JTI)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		family = stored.Family

		var uErr error
		user, uErr = qtx.GetUserByID(ctx, claims.UserID)
		if uErr != nil {
			slog.Error("get user for cookie refresh", "error", uErr)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		var rErr error
		refreshResult, rErr = h.authSvc.GenerateRefreshToken(user.ID, family)
		if rErr != nil {
			slog.Error("generate refresh token for cookie refresh", "error", rErr)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		if _, dbErr := qtx.CreateRefreshToken(ctx, dbgen.CreateRefreshTokenParams{
			UserID:    user.ID,
			Jti:       refreshResult.JTI,
			Family:    refreshResult.Family,
			ExpiresAt: refreshResult.ExpiresAt,
		}); dbErr != nil {
			slog.Error("store refresh token for cookie refresh", "error", dbErr)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		if commitErr := tx.Commit(ctx); commitErr != nil {
			slog.Error("commit cookie refresh tx", "error", commitErr)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	} else {
		// Pre-rotation token (no JTI): legacy path, no row to lock.
		var uErr error
		user, uErr = h.queries.GetUserByID(ctx, claims.UserID)
		if uErr != nil {
			slog.Error("get user for cookie refresh", "error", uErr)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		var rErr error
		refreshResult, rErr = h.authSvc.GenerateRefreshToken(user.ID, family)
		if rErr != nil {
			slog.Error("generate refresh token for cookie refresh", "error", rErr)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		if _, dbErr := h.queries.CreateRefreshToken(ctx, dbgen.CreateRefreshTokenParams{
			UserID:    user.ID,
			Jti:       refreshResult.JTI,
			Family:    refreshResult.Family,
			ExpiresAt: refreshResult.ExpiresAt,
		}); dbErr != nil {
			slog.Error("store refresh token for cookie refresh", "error", dbErr)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}

	accessToken, err := h.authSvc.GenerateAccessToken(user.ID)
	if err != nil {
		slog.Error("generate access token for cookie refresh", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	audit.Log(audit.EventTokenRefresh, "user_id", user.ID.String(), "ip", ip)

	// Set new auth cookies.
	auth.SetAuthCookies(w, accessToken, refreshResult.Token, h.secureCookies)

	resp := refreshResponse{
		User: refreshUser{
			ID:    user.ID.String(),
			Email: user.Email,
		},
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}
	if user.Name.Valid {
		resp.User.Name = user.Name.String
	}
	if user.AvatarUrl.Valid {
		resp.User.AvatarURL = user.AvatarUrl.String
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("encode refresh response", "error", err)
	}
}

// HandleLogout handles POST /auth/logout — clears auth cookies and revokes the refresh token.
func (h *OAuthHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB limit

	// Try to revoke the refresh token if present.
	refreshToken := auth.RefreshTokenFromCookie(r)
	if refreshToken != "" {
		claims, err := h.authSvc.ValidateRefreshToken(refreshToken)
		if err == nil && claims.JTI != "" {
			if revokeErr := h.queries.RevokeRefreshToken(r.Context(), claims.JTI); revokeErr != nil {
				slog.Error("revoke refresh token on logout", "error", revokeErr, "jti", claims.JTI)
			}
			audit.Log(audit.EventLogout, "user_id", claims.UserID.String())
		}
	}

	// Always clear cookies, even if token validation fails.
	auth.ClearAuthCookies(w, h.secureCookies)
	w.WriteHeader(http.StatusNoContent)
}

func generateState() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand.Read failing indicates a fundamental OS issue;
		// panic rather than return a predictable value that enables CSRF.
		panic(fmt.Sprintf("crypto/rand.Read failed: %v", err))
	}
	return hex.EncodeToString(b)
}

// isEmailDomainAllowed checks if the email's domain is in the allowlist.
// An empty allowlist permits all domains.
func isEmailDomainAllowed(email string, allowedDomains []string) bool {
	if len(allowedDomains) == 0 {
		return true
	}
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return false
	}
	domain := parts[1]
	for _, allowed := range allowedDomains {
		if strings.EqualFold(domain, allowed) {
			return true
		}
	}
	return false
}
