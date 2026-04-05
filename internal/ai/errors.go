package ai

import (
	"errors"
	"strings"
)

// Sanitized error messages returned to clients. These must not contain
// provider names, org IDs, API URLs, model names, or rate limit details.
const (
	errMsgRateLimit = "AI service is temporarily busy, please try again in a moment"
	errMsgGeneric   = "AI service encountered an error"
)

// SanitizedError wraps an original provider error with a client-safe message.
// Use Unwrap() or errors.Is/As to access the original error for logging.
type SanitizedError struct {
	// sanitized is the client-safe message.
	sanitized string
	// original is the full provider error for server-side logging.
	original error
	// rateLimit indicates whether the original error was a rate limit (429).
	rateLimit bool
}

func (e *SanitizedError) Error() string {
	return e.sanitized
}

func (e *SanitizedError) Unwrap() error {
	return e.original
}

// IsRateLimit reports whether the sanitized error originated from a rate limit.
func (e *SanitizedError) IsRateLimit() bool {
	return e.rateLimit
}

// SanitizeProviderError wraps an AI provider error with a client-safe message.
// Rate limit errors (429) get a specific "temporarily busy" message; all other
// errors get a generic message. The original error is preserved via Unwrap()
// for server-side logging.
func SanitizeProviderError(err error) error {
	if err == nil {
		return nil
	}

	if isRateLimitError(err) {
		return &SanitizedError{
			sanitized: errMsgRateLimit,
			original:  err,
			rateLimit: true,
		}
	}

	return &SanitizedError{
		sanitized: errMsgGeneric,
		original:  err,
		rateLimit: false,
	}
}

// IsProviderRateLimit checks whether err is a sanitized rate limit error.
func IsProviderRateLimit(err error) bool {
	var se *SanitizedError
	if errors.As(err, &se) {
		return se.IsRateLimit()
	}
	return false
}

// isRateLimitError detects rate limit errors from AI providers by checking
// for common indicators: HTTP 429 status codes, "rate_limit" error types,
// and retryableError with status 429.
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}

	// Check for our own retryableError type from the retry layer.
	var re *retryableError
	if errors.As(err, &re) && re.statusCode == 429 {
		return true
	}

	msg := err.Error()
	lower := strings.ToLower(msg)

	// Anthropic returns "rate_limit_error" in the error type and "429" in status.
	if strings.Contains(lower, "rate_limit") {
		return true
	}
	if strings.Contains(msg, "429") {
		return true
	}

	return false
}

// OriginalError extracts the original provider error from a SanitizedError.
// If err is not a SanitizedError, it returns err unchanged.
func OriginalError(err error) error {
	var se *SanitizedError
	if errors.As(err, &se) {
		return se.original
	}
	return err
}

