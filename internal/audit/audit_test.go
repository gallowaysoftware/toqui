package audit

import (
	"log/slog"
	"testing"
)

func TestSeverityForEvent(t *testing.T) {
	tests := []struct {
		event string
		want  slog.Level
	}{
		// Error-level: active threats.
		{EventTokenReuse, slog.LevelError},
		{EventAuthLockout, slog.LevelError},
		{EventCSRFRejected, slog.LevelError},

		// Warn-level: denied attempts, validation failures.
		{EventLoginDeniedDomain, slog.LevelWarn},
		{EventLoginDeniedCapacity, slog.LevelWarn},
		{EventTokenRefreshDenied, slog.LevelWarn},
		{EventPaymentValidation, slog.LevelWarn},

		// Info-level: normal operations.
		{EventLogin, slog.LevelInfo},
		{EventLogout, slog.LevelInfo},
		{EventTokenRefresh, slog.LevelInfo},
		{EventAccountDelete, slog.LevelInfo},
		{EventDataExport, slog.LevelInfo},
		{EventTripShare, slog.LevelInfo},
		{EventTripUnshare, slog.LevelInfo},
		{EventTripProPurchase, slog.LevelInfo},
		{EventAdminInvite, slog.LevelInfo},
		{EventReferralRedeem, slog.LevelInfo},
		{EventLoginAdmittedInvite, slog.LevelInfo},
		{EventFacebookLogin, slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.event, func(t *testing.T) {
			got := severityForEvent(tt.event)
			if got != tt.want {
				t.Errorf("severityForEvent(%q) = %v, want %v", tt.event, got, tt.want)
			}
		})
	}
}
