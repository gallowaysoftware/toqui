package auth

import (
	"encoding/base64"
	"net/http"
)

const (
	// OAuthResultCookie is a short-lived HttpOnly cookie that carries the
	// auth tokens from the OAuth callback redirect to the frontend exchange
	// endpoint, avoiding tokens in URL parameters.
	OAuthResultCookie = "toqui_oauth_result"

	// AccessTokenCookie is an HttpOnly cookie containing the JWT access token.
	// Used by web browsers; native apps use Authorization: Bearer header instead.
	AccessTokenCookie = "toqui_access"

	// RefreshTokenCookie is an HttpOnly cookie containing the JWT refresh token.
	// Scoped to /auth path so it's only sent on auth-related requests.
	RefreshTokenCookie = "toqui_refresh"

	// accessTokenMaxAge is the cookie lifetime for access tokens (1 hour).
	accessTokenMaxAge = 3600

	// refreshTokenMaxAge is the cookie lifetime for refresh tokens (30 days).
	refreshTokenMaxAge = 30 * 24 * 3600
)

// SetOAuthResultCookie sets a short-lived HttpOnly cookie containing a value
// (typically a JWT pair encoded as JSON) for the frontend to exchange.
// The value is base64url-encoded to avoid invalid cookie characters (e.g. `"`
// in JSON is stripped by net/http per RFC 6265).
func SetOAuthResultCookie(w http.ResponseWriter, value string, secure bool) {
	encoded := base64.RawURLEncoding.EncodeToString([]byte(value))
	http.SetCookie(w, &http.Cookie{
		Name:     OAuthResultCookie,
		Value:    encoded,
		Path:     "/",
		MaxAge:   60, // 1 minute — just enough for the frontend redirect
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteNoneMode,
	})
}

// ClearOAuthResultCookie removes the temporary OAuth result cookie.
func ClearOAuthResultCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     OAuthResultCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteNoneMode,
	})
}

// OAuthResultFromCookies extracts the OAuth result from the request cookie,
// decoding the base64url value back to the original string.
func OAuthResultFromCookies(r *http.Request) string {
	c, err := r.Cookie(OAuthResultCookie)
	if err != nil {
		return ""
	}
	decoded, err := base64.RawURLEncoding.DecodeString(c.Value)
	if err != nil {
		return ""
	}
	return string(decoded)
}

// SetAuthCookies sets HttpOnly access and refresh token cookies.
// Web browsers use these cookies for authentication; the cookie-to-header
// middleware translates them into Authorization: Bearer headers for handlers.
// Native apps bypass cookies and set Authorization headers directly.
// SetAuthCookieDomain sets the domain for auth cookies. Must be called at
// startup. Empty string = host-only cookies (local dev). ".toqui.travel" for
// prod so cookies work across api/app/admin subdomains.
var cookieDomain string

func SetAuthCookieDomain(domain string) { cookieDomain = domain }

func SetAuthCookies(w http.ResponseWriter, accessToken, refreshToken string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     AccessTokenCookie,
		Value:    accessToken,
		Path:     "/",
		Domain:   cookieDomain,
		MaxAge:   accessTokenMaxAge,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     RefreshTokenCookie,
		Value:    refreshToken,
		Path:     "/",
		Domain:   cookieDomain,
		MaxAge:   refreshTokenMaxAge,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearAuthCookies removes both access and refresh token cookies.
func ClearAuthCookies(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     AccessTokenCookie,
		Value:    "",
		Path:     "/",
		Domain:   cookieDomain,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     RefreshTokenCookie,
		Value:    "",
		Path:     "/",
		Domain:   cookieDomain,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// AccessTokenFromCookie extracts the access token from the request cookie.
// Returns empty string if the cookie is not present.
func AccessTokenFromCookie(r *http.Request) string {
	c, err := r.Cookie(AccessTokenCookie)
	if err != nil {
		return ""
	}
	return c.Value
}

// RefreshTokenFromCookie extracts the refresh token from the request cookie.
// Returns empty string if the cookie is not present.
func RefreshTokenFromCookie(r *http.Request) string {
	c, err := r.Cookie(RefreshTokenCookie)
	if err != nil {
		return ""
	}
	return c.Value
}
