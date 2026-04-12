package usage

import (
	"testing"
	"time"

	"github.com/gallowaysoftware/toqui-backend/internal/tier"
)

func TestResetTime(t *testing.T) {
	resetAt := ResetTime()

	now := time.Now().UTC()
	expectedDate := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)

	if !resetAt.Equal(expectedDate) {
		t.Errorf("expected reset time %v, got %v", expectedDate, resetAt)
	}

	// Reset time should always be in the future
	if !resetAt.After(now) {
		t.Error("expected reset time to be in the future")
	}
}

func TestErrDailyLimitExceeded(t *testing.T) {
	if ErrDailyLimitExceeded == nil {
		t.Fatal("ErrDailyLimitExceeded should not be nil")
	}
	if ErrDailyLimitExceeded.Error() != "daily message limit exceeded" {
		t.Errorf("unexpected error message: %s", ErrDailyLimitExceeded.Error())
	}
}

func TestLimitForTier(t *testing.T) {
	svc := &Service{
		limitFree: 10,
		limitPro:  50,
	}

	tests := []struct {
		tier tier.UserTier
		want int
	}{
		{tier.Free, 10},
		{tier.Pro, 50},
		{tier.Explorer, 0}, // unlimited
		{tier.Voyager, 0},  // unlimited
	}

	for _, tt := range tests {
		t.Run(string(tt.tier), func(t *testing.T) {
			if got := svc.LimitForTier(tt.tier); got != tt.want {
				t.Errorf("LimitForTier(%q) = %d, want %d", tt.tier, got, tt.want)
			}
		})
	}
}

func TestWithTierLimits(t *testing.T) {
	svc := &Service{limitFree: 30, limitPro: 30}
	svc.WithTierLimits(15, 75)

	if svc.limitFree != 15 {
		t.Errorf("expected limitFree=15, got %d", svc.limitFree)
	}
	if svc.limitPro != 75 {
		t.Errorf("expected limitPro=75, got %d", svc.limitPro)
	}
}
