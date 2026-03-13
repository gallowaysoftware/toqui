package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	"github.com/gallowaysoftware/toqui-backend/internal/auth"
	"github.com/gallowaysoftware/toqui-backend/internal/requestid"
)

// internalError logs the real error with context and returns a generic error
// message to the client. This prevents leaking internal implementation details
// (database errors, stack traces, etc.) while preserving debuggability via logs.
func internalError(ctx context.Context, operation string, err error) *connect.Error {
	reqID := requestid.FromContext(ctx)
	slog.Error(operation, "error", err, "request_id", reqID)
	return connect.NewError(connect.CodeInternal, fmt.Errorf("an internal error occurred"))
}

// maskEmail obscures the local part of an email address for safe logging.
// "john.doe@example.com" → "j***@example.com"
// Returns the input unchanged if it doesn't contain "@".
func maskEmail(email string) string {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return email
	}
	local := parts[0]
	if len(local) == 0 {
		return email
	}
	return string(local[0]) + "***@" + parts[1]
}

// clampPageSize enforces pagination bounds. If requested is 0 or negative,
// it returns defaultSize. If requested exceeds maxSize, it returns maxSize.
// Otherwise it returns requested unchanged.
func clampPageSize(requested, defaultSize, maxSize int32) int32 {
	if requested <= 0 {
		return defaultSize
	}
	if requested > maxSize {
		return maxSize
	}
	return requested
}

// authenticateRESTRequest extracts and validates a Bearer token from an HTTP
// request. It checks the Authorization header first (set by CookieAuth
// middleware for web browsers, or directly by native apps), then falls back to
// reading the HttpOnly access cookie directly as defense-in-depth.
func authenticateRESTRequest(r *http.Request, authSvc *auth.Service) (uuid.UUID, bool) {
	// Try Authorization header (covers both native Bearer and CookieAuth-bridged web)
	if authHeader := r.Header.Get("Authorization"); len(authHeader) >= 8 && authHeader[:7] == "Bearer " {
		userID, err := authSvc.ValidateToken(authHeader[7:])
		if err == nil {
			return userID, true
		}
	}

	// Fallback: read HttpOnly cookie directly (defense-in-depth for web users)
	if token := auth.AccessTokenFromCookie(r); token != "" {
		userID, err := authSvc.ValidateToken(token)
		if err == nil {
			return userID, true
		}
	}

	return uuid.Nil, false
}

// clientIPFromHeaders extracts the client IP from HTTP headers.
// Used for ConnectRPC requests where we only have access to the header map.
func clientIPFromHeaders(h http.Header) string {
	if xff := h.Get("X-Forwarded-For"); xff != "" {
		if ip, _, ok := strings.Cut(xff, ","); ok {
			return strings.TrimSpace(ip)
		}
		return strings.TrimSpace(xff)
	}
	if xri := h.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	return "unknown"
}
