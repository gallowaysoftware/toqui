// Package middleware provides HTTP middleware for the Toqui backend.
package middleware

import (
	"net/http"

	"github.com/gallowaysoftware/toqui-backend/internal/auth"
)

// CookieAuth bridges HttpOnly cookie-based auth to the existing Bearer token
// flow. If no Authorization header is present, it checks for a toqui_access
// cookie and sets the Authorization header on the request. This allows the
// existing auth interceptor and REST handlers to work unchanged.
//
// Native apps set Authorization: Bearer directly and skip cookies entirely.
// Web browsers send HttpOnly cookies; this middleware translates them.
func CookieAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// If Authorization header already present, pass through (native app / gRPC client).
		if r.Header.Get("Authorization") != "" {
			next.ServeHTTP(w, r)
			return
		}

		// Check for access token cookie (web browser).
		token := auth.AccessTokenFromCookie(r)
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}

		next.ServeHTTP(w, r)
	})
}
