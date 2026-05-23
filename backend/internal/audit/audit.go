// Package audit provides structured audit logging for security-relevant events.
// Events are written via slog and automatically collected by Cloud Logging.
package audit

import (
	"context"
	"log/slog"
)

// Event types for audit trail.
const (
	EventLogin               = "auth.login"
	EventLoginDeniedDomain   = "auth.login_denied.domain"
	EventLoginDeniedCapacity = "auth.login_denied.capacity"
	EventLoginDeniedUnderAge = "auth.login_denied.under_age"
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
	EventTripProPurchase     = "payment.trip_pro_purchase"
	EventPaymentValidation   = "payment.validation_failed"
	EventAdminInvite         = "admin.invite"
	EventAdminTripUnlock     = "admin.trip_unlock"
	EventAdminGrantPro       = "admin.grant_pro"
	EventAdminDeleteUser     = "admin.delete_user"
	EventReferralRedeem      = "referral.redeem"
	EventTripInvite          = "trip.invite"
	EventTripInviteAccept    = "trip.invite_accept"
	EventTripCollabRemove    = "trip.collaborator_remove"
	EventAdminSeedRole       = "admin.seed_role"
	EventAdminSetRole        = "admin.set_role"

	// EventWebhookAuthFailed fires when an inbound webhook fails Svix
	// signature verification (missing headers, expired timestamp, bad
	// signature, or replay of an already-seen svix-id). Compliance
	// dashboards filter on `webhook.*` to see signature-failure rates.
	EventWebhookAuthFailed = "webhook.email.auth_failed"

	// EventBookingMerge fires when an existing booking is updated via
	// the dedup/merge path (confirmation-code match, fuzzy match, or
	// 23505 race recovery). The public inbound-email webhook can
	// mutate existing booking data via merge, so we audit it.
	EventBookingMerge = "booking.merge"
)

// severityForEvent returns the appropriate slog level for an audit event.
// Security-critical events are routed to Error, suspicious events to Warn,
// and normal operational events to Info.
func severityForEvent(event string) slog.Level {
	switch event {
	// Security-critical: active attacks or account lockouts.
	case EventTokenReuse, EventAuthLockout, EventCSRFRejected:
		return slog.LevelError

	// Suspicious / denied: failed auth attempts, payment validation failures.
	case EventLoginDeniedDomain, EventLoginDeniedCapacity, EventLoginDeniedUnderAge,
		EventTokenRefreshDenied, EventPaymentValidation, EventWebhookAuthFailed:
		return slog.LevelWarn

	// Everything else: normal operational events.
	default:
		return slog.LevelInfo
	}
}

// Log records a structured audit event. All audit events include the event
// type and any number of additional key-value attributes for context.
// Events are routed to the appropriate slog severity level based on their
// security implications (Error for active threats, Warn for denied attempts,
// Info for normal operations).
func Log(event string, attrs ...any) {
	args := make([]any, 0, len(attrs)+2)
	args = append(args, "audit_event", event)
	args = append(args, attrs...)
	slog.Log(context.Background(), severityForEvent(event), "audit", args...)
}
