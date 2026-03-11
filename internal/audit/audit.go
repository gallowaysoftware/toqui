// Package audit provides structured audit logging for security-relevant events.
// Events are written via slog and automatically collected by Cloud Logging.
package audit

import (
	"log/slog"
)

// Event types for audit trail.
const (
	EventLogin               = "auth.login"
	EventLoginDeniedDomain   = "auth.login_denied.domain"
	EventLoginDeniedCapacity = "auth.login_denied.capacity"
	EventLoginAdmittedInvite = "auth.login_admitted.invite"
	EventTokenRefresh        = "auth.token_refresh"
	EventTokenRefreshDenied  = "auth.token_refresh_denied"
	EventTokenReuse          = "auth.token_reuse_detected"
	EventAuthLockout         = "auth.lockout"
	EventLogout              = "auth.logout"
	EventAccountDelete       = "auth.account_delete"
	EventDataExport          = "auth.data_export"
	EventTripShare           = "trip.share"
	EventTripUnshare         = "trip.unshare"
	EventCSRFRejected        = "security.csrf_rejected"
)

// Log records a structured audit event. All audit events include the event
// type and any number of additional key-value attributes for context.
func Log(event string, attrs ...any) {
	args := make([]any, 0, len(attrs)+2)
	args = append(args, "audit_event", event)
	args = append(args, attrs...)
	slog.Info("audit", args...)
}
