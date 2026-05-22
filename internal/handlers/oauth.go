package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gallowaysoftware/toqui-backend/internal/analytics"
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
	allowedEmails  []string
	maxFreeUsers   int
	authLimiter    *ratelimit.AuthLimiter
	emailSvc       *email.Sender

	// Facebook/Meta OAuth config (covers Facebook + Instagram login)
	facebookClientID     string
	facebookClientSecret string
	facebookRedirectURI  string

	analyticsClient *analytics.Client
	alertChecker    *analytics.AlertChecker
}

func NewOAuthHandler(authSvc *auth.Service, pool *pgxpool.Pool, frontendURL string, secureCookies bool, allowedDomains []string, allowedEmails []string, authLimiter *ratelimit.AuthLimiter, emailSvc *email.Sender) *OAuthHandler {
	return &OAuthHandler{
		authSvc:        authSvc,
		pool:           pool,
		queries:        dbgen.New(pool),
		frontendURL:    frontendURL,
		emailSvc:       emailSvc,
		secureCookies:  secureCookies,
		allowedDomains: allowedDomains,
		allowedEmails:  allowedEmails,
		authLimiter:    authLimiter,
	}
}

// WithMaxFreeUsers configures the capacity cap for Facebook OAuth new user registration.
func (h *OAuthHandler) WithMaxFreeUsers(maxFreeUsers int) *OAuthHandler {
	h.maxFreeUsers = maxFreeUsers
	return h
}

// WithAnalytics configures the OAuth handler to send events to PostHog.
func (h *OAuthHandler) WithAnalytics(client *analytics.Client) *OAuthHandler {
	h.analyticsClient = client
	return h
}

// WithAlertChecker wires the in-process AlertChecker so each successful
// signup_completed Track call also resets the idle-signup timer. The
// AlertChecker's idle-signup threshold (default 24h) fires a Cloud
// Logging warning when no signups happen — early detection of e.g. a
// broken Google OAuth callback. Optional.
func (h *OAuthHandler) WithAlertChecker(checker *analytics.AlertChecker) *OAuthHandler {
	h.alertChecker = checker
	return h
}

// WithFacebookOAuth configures Facebook/Meta OAuth credentials on the handler.
func (h *OAuthHandler) WithFacebookOAuth(clientID, clientSecret, redirectURI string) *OAuthHandler {
	h.facebookClientID = clientID
	h.facebookClientSecret = clientSecret
	h.facebookRedirectURI = redirectURI
	return h
}

func (h *OAuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
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
	// IsNewUser is set by HandleCallback when this is the user's first-ever
	// login (created within the last minute). HandleExchange uses this to
	// decide whether to fire `signup_completed`.
	IsNewUser bool `json:"is_new_user,omitempty"`
	// AuthProvider is "google" or "facebook" — propagated to the
	// `signup_completed` event when IsNewUser is true.
	AuthProvider string `json:"auth_provider,omitempty"`
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

	// Always mark waitlist as accepted when the user successfully signs up,
	// regardless of whether capacity check was needed.
	if markErr := h.queries.MarkWaitlistAccepted(r.Context(), info.Email); markErr != nil && !errors.Is(markErr, pgx.ErrNoRows) {
		slog.Error("mark waitlist accepted on signup failed", "email", maskEmail(info.Email), "error", markErr)
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

	// `signup_completed` is fired in HandleExchange (not here) so callsites
	// remain symmetric with the native gRPC flow. The alert-checker still
	// ticks here — it's a server-side health signal (idle-signup detection).
	if h.alertChecker != nil && isNewUser {
		h.alertChecker.RecordSignup()
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
		IsNewUser:    isNewUser,
		AuthProvider: "google",
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

	// Fire `signup_completed` here (not in HandleCallback) so callsites
	// remain symmetric with the native gRPC flow. The IsNewUser +
	// AuthProvider flags are stamped into the OAuth result cookie by
	// HandleCallback / HandleFacebookCallback.
	if h.analyticsClient != nil && result.IsNewUser {
		h.analyticsClient.Track(result.UserID, "signup_completed", map[string]any{
			"auth_provider": result.AuthProvider,
		})
	}

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

// facebookUserInfo holds the response from Facebook's Graph API /me endpoint.
type facebookUserInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Email   string `json:"email"`
	Picture struct {
		Data struct {
			URL string `json:"url"`
		} `json:"data"`
	} `json:"picture"`
}

// HandleFacebookLogin redirects to Facebook's OAuth consent screen.
func (h *OAuthHandler) HandleFacebookLogin(w http.ResponseWriter, r *http.Request) {
	if h.facebookClientID == "" {
		http.Error(w, "Facebook login not configured", http.StatusNotImplemented)
		return
	}

	state := generateState()

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

	params := url.Values{
		"client_id":    {h.facebookClientID},
		"redirect_uri": {h.facebookRedirectURI},
		"state":        {state},
		"scope":        {"email,public_profile"},
	}

	authURL := "https://www.facebook.com/v19.0/dialog/oauth?" + params.Encode()
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

// HandleFacebookCallback handles the OAuth callback from Facebook.
func (h *OAuthHandler) HandleFacebookCallback(w http.ResponseWriter, r *http.Request) {
	if h.facebookClientID == "" {
		http.Error(w, "Facebook login not configured", http.StatusNotImplemented)
		return
	}

	// Validate state
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

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Redirect(w, r, h.frontendURL+"/?error=missing_code", http.StatusTemporaryRedirect)
		return
	}

	// Exchange code for access token
	fbToken, err := h.exchangeFacebookCode(r.Context(), code)
	if err != nil {
		slog.Error("facebook code exchange failed", "error", err)
		http.Redirect(w, r, h.frontendURL+"/?error=exchange_failed", http.StatusTemporaryRedirect)
		return
	}

	// Fetch user profile from Facebook
	fbUser, err := fetchFacebookUser(fbToken)
	if err != nil {
		slog.Error("facebook user info failed", "error", err)
		http.Redirect(w, r, h.frontendURL+"/?error=exchange_failed", http.StatusTemporaryRedirect)
		return
	}

	if fbUser.Email == "" {
		http.Redirect(w, r, h.frontendURL+"/?error=email_required", http.StatusTemporaryRedirect)
		return
	}

	// Domain allowlist
	if !isEmailDomainAllowed(fbUser.Email, h.allowedDomains) {
		audit.Log(audit.EventLoginDeniedDomain, "email", maskEmail(fbUser.Email))
		http.Redirect(w, r, h.frontendURL+"/waitlist?reason=domain_not_allowed", http.StatusTemporaryRedirect)
		return
	}

	ctx := r.Context()
	user, err := h.findOrCreateFacebookUser(ctx, fbUser)
	if err != nil {
		slog.Error("facebook user upsert failed", "error", err)
		http.Redirect(w, r, h.frontendURL+"/?error=db_error", http.StatusTemporaryRedirect)
		return
	}

	// New user detection: created within the last minute means this is a signup, not a returning login.
	fbIsNewUser := time.Since(user.CreatedAt) < time.Minute

	// Send welcome email for new users.
	if h.emailSvc != nil && fbIsNewUser {
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

	// `signup_completed` is fired in HandleExchange (not here) so callsites
	// remain symmetric with the native gRPC flow. See the matching comment
	// in HandleCallback (Google) above.
	if h.alertChecker != nil && fbIsNewUser {
		h.alertChecker.RecordSignup()
	}

	accessToken, err := h.authSvc.GenerateAccessToken(user.ID)
	if err != nil {
		http.Redirect(w, r, h.frontendURL+"/?error=token_error", http.StatusTemporaryRedirect)
		return
	}

	refreshResult, err := h.authSvc.GenerateRefreshToken(user.ID, uuid.Nil)
	if err != nil {
		http.Redirect(w, r, h.frontendURL+"/?error=token_error", http.StatusTemporaryRedirect)
		return
	}

	if _, dbErr := h.queries.CreateRefreshToken(ctx, dbgen.CreateRefreshTokenParams{
		UserID:    user.ID,
		Jti:       refreshResult.JTI,
		Family:    refreshResult.Family,
		ExpiresAt: refreshResult.ExpiresAt,
	}); dbErr != nil {
		slog.Error("store refresh token", "error", dbErr)
		http.Redirect(w, r, h.frontendURL+"/?error=internal", http.StatusTemporaryRedirect)
		return
	}

	result := oauthResult{
		AccessToken:  accessToken,
		RefreshToken: refreshResult.Token,
		UserID:       user.ID.String(),
		Email:        user.Email,
		IsNewUser:    fbIsNewUser,
		AuthProvider: "facebook",
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

	audit.Log(audit.EventFacebookLogin, "user_id", user.ID.String(), "email", maskEmail(user.Email))

	// Admin return flow (same as Google)
	if c, err := r.Cookie("oauth_return"); err == nil && c.Value == "admin" {
		clearDomain := ""
		if h.secureCookies {
			clearDomain = ".toqui.travel"
		}
		http.SetCookie(w, &http.Cookie{Name: "oauth_return", Value: "", Path: "/", Domain: clearDomain, MaxAge: -1})
		http.Redirect(w, r, "https://admin.toqui.travel/#token="+result.AccessToken, http.StatusTemporaryRedirect)
		return
	}

	auth.SetOAuthResultCookie(w, string(resultJSON), h.secureCookies)
	http.Redirect(w, r, h.frontendURL+"/auth/callback", http.StatusTemporaryRedirect)
}

// exchangeFacebookCode exchanges a Facebook authorization code for an access token.
func (h *OAuthHandler) exchangeFacebookCode(ctx context.Context, code string) (string, error) {
	params := url.Values{
		"client_id":     {h.facebookClientID},
		"client_secret": {h.facebookClientSecret},
		"redirect_uri":  {h.facebookRedirectURI},
		"code":          {code},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://graph.facebook.com/v19.0/oauth/access_token?"+params.Encode(), nil)
	if err != nil {
		return "", fmt.Errorf("create token request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("facebook token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("facebook token exchange failed (%d): %s", resp.StatusCode, body)
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("unmarshal token response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("empty access token in response")
	}

	return tokenResp.AccessToken, nil
}

// fetchFacebookUser fetches the user profile from Facebook's Graph API.
func fetchFacebookUser(accessToken string) (*facebookUserInfo, error) {
	u := "https://graph.facebook.com/me?fields=id,name,email,picture.type(large)&access_token=" + url.QueryEscape(accessToken)
	resp, err := http.Get(u) //nolint:gosec // URL is constructed from trusted components
	if err != nil {
		return nil, fmt.Errorf("facebook user info request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read user info: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("facebook user info failed (%d): %s", resp.StatusCode, body)
	}

	var info facebookUserInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("unmarshal user info: %w", err)
	}

	return &info, nil
}

// validateFacebookAccessToken validates a Facebook access token using the debug_token endpoint.
func validateFacebookAccessToken(ctx context.Context, inputToken, appID, appSecret string) (*facebookUserInfo, error) {
	// Debug the token to verify it's valid and belongs to our app.
	debugURL := fmt.Sprintf("https://graph.facebook.com/debug_token?input_token=%s&access_token=%s|%s",
		url.QueryEscape(inputToken), url.QueryEscape(appID), url.QueryEscape(appSecret))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, debugURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create debug request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("facebook debug_token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read debug response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("facebook debug_token failed (%d): %s", resp.StatusCode, body)
	}

	var debugResp struct {
		Data struct {
			AppID   string `json:"app_id"`
			IsValid bool   `json:"is_valid"`
			UserID  string `json:"user_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &debugResp); err != nil {
		return nil, fmt.Errorf("unmarshal debug response: %w", err)
	}

	if !debugResp.Data.IsValid {
		return nil, fmt.Errorf("facebook access token is not valid")
	}

	if debugResp.Data.AppID != appID {
		return nil, fmt.Errorf("facebook access token is for a different app")
	}

	// Token is valid — fetch the user profile.
	return fetchFacebookUser(inputToken)
}

// findOrCreateFacebookUser looks up or creates a user from Facebook login info.
// Logic: 1) find by facebook_id → login, 2) find by email → link, 3) create new.
func (h *OAuthHandler) findOrCreateFacebookUser(ctx context.Context, fbUser *facebookUserInfo) (*dbgen.User, error) {
	fbID := pgtype.Text{String: fbUser.ID, Valid: true}
	avatarURL := fbUser.Picture.Data.URL

	// 1. Check if user exists by Facebook ID
	user, err := h.queries.GetUserByFacebookID(ctx, fbID)
	if err == nil {
		return &user, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("get user by facebook_id: %w", err)
	}

	// 2. Check if user exists by email → link Facebook ID
	user, err = h.queries.GetUserByEmail(ctx, fbUser.Email)
	if err == nil {
		// Link Facebook ID to existing account
		if linkErr := h.queries.UpdateUserFacebookID(ctx, dbgen.UpdateUserFacebookIDParams{
			ID:         user.ID,
			FacebookID: fbID,
		}); linkErr != nil {
			return nil, fmt.Errorf("link facebook_id to user: %w", linkErr)
		}
		user.FacebookID = fbID
		audit.Log(audit.EventFacebookLink, "user_id", user.ID.String(), "email", maskEmail(user.Email))
		return &user, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("get user by email: %w", err)
	}

	// 3. Capacity check for new users
	if !isEmailAllowListed(fbUser.Email, h.allowedEmails) && h.maxFreeUsers > 0 {
		userCount, countErr := h.queries.CountUsers(ctx)
		if countErr != nil {
			return nil, fmt.Errorf("count users: %w", countErr)
		}
		if int(userCount) >= h.maxFreeUsers {
			waitlistEntry, wlErr := h.queries.GetWaitlistByEmail(ctx, fbUser.Email)
			if wlErr != nil || !waitlistEntry.InviteCode.Valid {
				audit.Log(audit.EventLoginDeniedCapacity,
					"email", maskEmail(fbUser.Email),
					"user_count", userCount,
					"max_free_users", h.maxFreeUsers,
				)
				return nil, fmt.Errorf("at capacity")
			}
			if markErr := h.queries.MarkWaitlistAccepted(ctx, fbUser.Email); markErr != nil {
				slog.Error("mark waitlist accepted failed", "email", maskEmail(fbUser.Email), "error", markErr)
			}
			audit.Log(audit.EventLoginAdmittedInvite,
				"email", maskEmail(fbUser.Email),
				"invite_code", waitlistEntry.InviteCode.String,
			)
		}
	}

	// Mark waitlist as accepted
	if markErr := h.queries.MarkWaitlistAccepted(ctx, fbUser.Email); markErr != nil && !errors.Is(markErr, pgx.ErrNoRows) {
		slog.Error("mark waitlist accepted on signup failed", "email", maskEmail(fbUser.Email), "error", markErr)
	}

	// Create new user with Facebook ID
	user, err = h.queries.CreateUserWithFacebook(ctx, dbgen.CreateUserWithFacebookParams{
		Email:      fbUser.Email,
		Name:       pgtype.Text{String: fbUser.Name, Valid: fbUser.Name != ""},
		FacebookID: fbID,
		AvatarUrl:  pgtype.Text{String: avatarURL, Valid: avatarURL != ""},
	})
	if err != nil {
		return nil, fmt.Errorf("create user with facebook: %w", err)
	}

	audit.Log(audit.EventFacebookLoginNew, "user_id", user.ID.String(), "email", maskEmail(user.Email))
	return &user, nil
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

// isEmailAllowListed checks if the email is on the explicit allow-list.
// Used to bypass capacity/waitlist checks for team and friends/family.
func isEmailAllowListed(email string, allowedEmails []string) bool {
	for _, allowed := range allowedEmails {
		if strings.EqualFold(email, allowed) {
			return true
		}
	}
	return false
}
