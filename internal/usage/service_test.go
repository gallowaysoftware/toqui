package usage

import (
	"testing"
	"time"
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
