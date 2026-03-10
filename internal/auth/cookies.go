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
