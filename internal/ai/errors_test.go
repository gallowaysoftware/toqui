package ai

import (
	"errors"
	"fmt"
	"testing"
)

func TestSanitizeProviderError_Nil(t *testing.T) {
	if got := SanitizeProviderError(nil); got != nil {
		t.Errorf("SanitizeProviderError(nil) = %v, want nil", got)
	}
}

func TestSanitizeProviderError_RateLimitPatterns(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"429 in message", fmt.Errorf("API error 429: rate limit exceeded for org-abc123")},
		{"rate_limit in message", fmt.Errorf("error type: rate_limit_error, message: too many requests")},
		{"Rate_Limit mixed case", fmt.Errorf("Rate_Limit exceeded")},
		{"retryableError 429", &retryableError{statusCode: 429, body: "rate limited"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeProviderError(tt.err)

			if got.Error() != errMsgRateLimit {
				t.Errorf("Error() = %q, want %q", got.Error(), errMsgRateLimit)
			}

			if !IsProviderRateLimit(got) {
				t.Error("IsProviderRateLimit() = false, want true")
			}

			// Original error should be preserved via Unwrap.
			var se *SanitizedError
			if !errors.As(got, &se) {
				t.Fatal("expected SanitizedError")
			}
			if !errors.Is(se.Unwrap(), tt.err) {
				t.Errorf("Unwrap() = %v, want %v", se.Unwrap(), tt.err)
			}
		})
	}
}

func TestSanitizeProviderError_NonRateLimit(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"generic API error", fmt.Errorf("API error 500: internal server error")},
		{"connection error", fmt.Errorf("connection refused")},
		{"timeout", fmt.Errorf("context deadline exceeded")},
		{"retryableError 500", &retryableError{statusCode: 500, body: "server error"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeProviderError(tt.err)

			if got.Error() != errMsgGeneric {
				t.Errorf("Error() = %q, want %q", got.Error(), errMsgGeneric)
			}

			if IsProviderRateLimit(got) {
				t.Error("IsProviderRateLimit() = true, want false")
			}
		})
	}
}

func TestSanitizeProviderError_NoLeakedDetails(t *testing.T) {
	sensitiveErr := fmt.Errorf("API error 429: {\"error\":{\"type\":\"rate_limit_error\",\"message\":\"Number of request tokens has exceeded your per-minute rate limit (https://docs.anthropic.com/en/api/rate-limits); see the response headers for current usage. Please reduce the prompt length or the maximum tokens requested, or try again later. You may also contact sales at https://www.anthropic.com/contact-sales to discuss your options for a rate limit increase.\"},\"org_id\":\"org-abc123def456\"}")

	got := SanitizeProviderError(sensitiveErr)
	msg := got.Error()

	for _, sensitive := range []string{
		"org-abc123",
		"anthropic.com",
		"docs.anthropic.com",
		"rate_limit_error",
		"contact-sales",
		"org_id",
	} {
		if contains(msg, sensitive) {
			t.Errorf("sanitized error %q still contains sensitive string %q", msg, sensitive)
		}
	}
}

func TestOriginalError(t *testing.T) {
	original := fmt.Errorf("the real error")
	sanitized := SanitizeProviderError(original)

	got := OriginalError(sanitized)
	if !errors.Is(got, original) {
		t.Errorf("OriginalError() = %v, want %v", got, original)
	}

	// Non-sanitized error returns itself.
	plain := fmt.Errorf("plain error")
	if !errors.Is(OriginalError(plain), plain) {
		t.Error("OriginalError(plain) should return the input unchanged")
	}
}

func TestIsProviderRateLimit_NonSanitizedError(t *testing.T) {
	if IsProviderRateLimit(fmt.Errorf("some error")) {
		t.Error("IsProviderRateLimit should be false for non-SanitizedError")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
