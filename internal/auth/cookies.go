package auth

import (
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
func SetOAuthResultCookie(w http.ResponseWriter, value string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     OAuthResultCookie,
		Value:    value,
		Path:     "/",
		MaxAge:   60, // 1 minute — just enough for the frontend redirect
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
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
		SameSite: http.SameSiteLaxMode,
	})
}

// OAuthResultFromCookies extracts the OAuth result from the request cookie.
func OAuthResultFromCookies(r *http.Request) string {
	c, err := r.Cookie(OAuthResultCookie)
	if err != nil {
		return ""
	}
	return c.Value
}
