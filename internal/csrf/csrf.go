// Package csrf provides middleware to prevent Cross-Site Request Forgery
// by validating Origin and Referer headers on state-changing requests.
package csrf

import (
	"log/slog"
	"net/http"
	"net/url"
	"strings"
)

// Middleware validates Origin/Referer headers on state-changing requests
// (POST, PUT, DELETE, PATCH) to prevent CSRF. Requests whose Origin or
// Referer does not match the allowed origins are rejected with 403 Forbidden.
//
// Safe methods (GET, HEAD, OPTIONS) pass through unconditionally.
// Paths matching exemptPrefixes are also exempt (e.g., webhooks with their
// own signature-based auth).
//
// When neither Origin nor Referer is present, the request is allowed through.
// This covers non-browser clients (curl, Postman, server-to-server) which
// cannot perform CSRF attacks. Cross-origin browser requests always include
// the Origin header.
func Middleware(next http.Handler, allowedOrigins []string, exemptPrefixes []string) http.Handler {
	// Pre-parse allowed origins into sets for fast lookup.
	allowedOriginSet := make(map[string]bool, len(allowedOrigins))
	allowedHosts := make(map[string]bool, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		allowedOriginSet[strings.ToLower(origin)] = true
		if u, err := url.Parse(origin); err == nil && u.Host != "" {
			allowedHosts[strings.ToLower(u.Host)] = true
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Safe methods are immune to CSRF.
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		// Exempt paths (e.g., /webhooks/ with their own ECDSA signature auth).
		for _, prefix := range exemptPrefixes {
			if strings.HasPrefix(r.URL.Path, prefix) {
				next.ServeHTTP(w, r)
				return
			}
		}

		// Check Origin header (preferred — all modern browsers send it on
		// cross-origin and same-origin POST requests).
		origin := r.Header.Get("Origin")
		if origin != "" {
			// Reject the literal "null" origin (sandboxed iframes, data: URIs).
			if strings.EqualFold(origin, "null") {
				slog.Warn("CSRF: rejected null origin",
					"method", r.Method,
					"path", r.URL.Path,
				)
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			if allowedOriginSet[strings.ToLower(origin)] {
				next.ServeHTTP(w, r)
				return
			}
			slog.Warn("CSRF: rejected untrusted origin",
				"origin", origin,
				"method", r.Method,
				"path", r.URL.Path,
			)
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		// Fallback: check Referer header when Origin is absent.
		referer := r.Header.Get("Referer")
		if referer != "" {
			if u, err := url.Parse(referer); err == nil && allowedHosts[strings.ToLower(u.Host)] {
				next.ServeHTTP(w, r)
				return
			}
			slog.Warn("CSRF: rejected untrusted referer",
				"referer", referer,
				"method", r.Method,
				"path", r.URL.Path,
			)
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		// Neither Origin nor Referer present — reject.
		// Modern browsers always send Origin on POST/PUT/DELETE/PATCH.
		// Legitimate non-browser clients (curl, server-to-server) should
		// use Bearer token auth which bypasses cookie-based CSRF risk.
		slog.Warn("CSRF: rejected request with no Origin or Referer",
			"method", r.Method,
			"path", r.URL.Path,
		)
		http.Error(w, "Forbidden", http.StatusForbidden)
	})
}
