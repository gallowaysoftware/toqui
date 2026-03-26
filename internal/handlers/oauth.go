package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gallowaysoftware/toqui-backend/internal/audit"
	"github.com/gallowaysoftware/toqui-backend/internal/auth"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui-backend/internal/ratelimit"
)

type OAuthHandler struct {
	authSvc        *auth.Service
	queries        *dbgen.Queries
	frontendURL    string
	secureCookies  bool
	maxFreeUsers   int
	allowedDomains []string
	allowedEmails  []string
	authLimiter    *ratelimit.AuthLimiter
}

func NewOAuthHandler(authSvc *auth.Service, pool *pgxpool.Pool, frontendURL string, secureCookies bool, maxFreeUsers int, allowedDomains []string, allowedEmails []string, authLimiter *ratelimit.AuthLimiter) *OAuthHandler {
	return &OAuthHandler{
		authSvc:        authSvc,
		queries:        dbgen.New(pool),
		frontendURL:    frontendURL,
		secureCookies:  secureCookies,
		maxFreeUsers:   maxFreeUsers,
		allowedDomains: allowedDomains,
		allowedEmails:  allowedEmails,
		authLimiter:    authLimiter,
	}
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

	http.Redirect(w, r, h.authSvc.AuthCodeURL(state), http.StatusTemporaryRedirect)
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

	info, err := h.authSvc.ExchangeCode(r.Context(), code)
	if err != nil {
		http.Redirect(w, r, h.frontendURL+"/?error=exchange_failed", http.StatusTemporaryRedirect)
		return
	}

	// Domain allowlist: reject signups from unauthorized email domains.
	if !isEmailDomainAllowed(info.Email, h.allowedDomains) {
		audit.Log(audit.EventLoginDeniedDomain, "email", maskEmail(info.Email))
		http.Redirect(w, r, h.frontendURL+"/waitlist?reason=domain_not_allowed", http.StatusTemporaryRedirect)
		return
	}

	// Capacity check: if user doesn't already exist and we're at capacity,
	// check for a valid invite code before allowing registration.
	// Allow-listed emails bypass this entirely (team + friends/family).
	if !isEmailAllowListed(info.Email, h.allowedEmails) && h.maxFreeUsers > 0 {
		_, existErr := h.queries.GetUserByGoogleID(r.Context(), info.ID)
		if errors.Is(existErr, pgx.ErrNoRows) {
			// New user — check capacity
			userCount, countErr := h.queries.CountUsers(r.Context())
			if countErr != nil {
				slog.Error("count users for capacity check failed", "error", countErr)
				http.Redirect(w, r, h.frontendURL+"/?error=db_error", http.StatusTemporaryRedirect)
				return
			}
			if int(userCount) >= h.maxFreeUsers {
				// At capacity — check if user has a valid invite
				waitlistEntry, wlErr := h.queries.GetWaitlistByEmail(r.Context(), info.Email)
				if wlErr != nil || !waitlistEntry.InviteCode.Valid {
					audit.Log(audit.EventLoginDeniedCapacity,
						"email", maskEmail(info.Email),
						"user_count", userCount,
						"max_free_users", h.maxFreeUsers,
					)
					http.Redirect(w, r, h.frontendURL+"/waitlist?reason=at_capacity", http.StatusTemporaryRedirect)
					return
				}
				// Has valid invite — allow through, mark accepted
				if markErr := h.queries.MarkWaitlistAccepted(r.Context(), info.Email); markErr != nil {
					slog.Error("mark waitlist accepted failed", "email", maskEmail(info.Email), "error", markErr)
				}
				audit.Log(audit.EventLoginAdmittedInvite,
					"email", maskEmail(info.Email),
					"invite_code", waitlistEntry.InviteCode.String,
				)
			}
		}
	}

	user, err := h.queries.UpsertUserByGoogleID(r.Context(), dbgen.UpsertUserByGoogleIDParams{
		GoogleID:  info.ID,
		Email:     info.Email,
		Name:      pgtype.Text{String: info.Name, Valid: info.Name != ""},
		AvatarUrl: pgtype.Text{String: info.AvatarURL, Valid: info.AvatarURL != ""},
	})
	if err != nil {
		http.Redirect(w, r, h.frontendURL+"/?error=db_error", http.StatusTemporaryRedirect)
		return
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

	// If the login was initiated from admin, set auth cookies directly and redirect.
	// Admin doesn't go through the frontend /auth/callback exchange flow.
	if c, err := r.Cookie("oauth_return"); err == nil && c.Value == "admin" {
		auth.SetAuthCookies(w, result.AccessToken, result.RefreshToken, h.secureCookies)
		// Clear the return cookie.
		clearDomain := ""
		if h.secureCookies {
			clearDomain = ".toqui.travel"
		}
		http.SetCookie(w, &http.Cookie{Name: "oauth_return", Value: "", Path: "/", Domain: clearDomain, MaxAge: -1})
		http.Redirect(w, r, "https://admin.toqui.travel/admin-ui/", http.StatusTemporaryRedirect)
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

	// Token rotation: verify the token is tracked and not revoked.
	family := claims.Family
	if claims.JTI != "" {
		stored, dbErr := h.queries.GetRefreshTokenByJTI(ctx, claims.JTI)
		if dbErr != nil {
			slog.Warn("refresh token JTI not found in database", "jti", claims.JTI)
			auth.ClearAuthCookies(w, h.secureCookies)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
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
				slog.Error("revoke token family on reuse detection", "error", revokeErr, "family", stored.Family.String())
			}
			auth.ClearAuthCookies(w, h.secureCookies)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		// Revoke the current token (it's been used).
		if revokeErr := h.queries.RevokeRefreshToken(ctx, claims.JTI); revokeErr != nil {
			slog.Error("revoke used refresh token", "error", revokeErr, "jti", claims.JTI)
		}
		family = stored.Family
	}

	user, err := h.queries.GetUserByID(ctx, claims.UserID)
	if err != nil {
		slog.Error("get user for cookie refresh", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	accessToken, err := h.authSvc.GenerateAccessToken(user.ID)
	if err != nil {
		slog.Error("generate access token for cookie refresh", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Issue new refresh token in the same family (rotation).
	refreshResult, err := h.authSvc.GenerateRefreshToken(user.ID, family)
	if err != nil {
		slog.Error("generate refresh token for cookie refresh", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Track the new token.
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
