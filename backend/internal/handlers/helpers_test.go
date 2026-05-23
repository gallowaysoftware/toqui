package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"connectrpc.com/connect"

	"github.com/gallowaysoftware/toqui/backend/internal/auth"
	"github.com/gallowaysoftware/toqui/backend/internal/trip"
)

// newTestAuthService creates an auth.Service for unit tests with a known secret.
func newTestAuthService() *auth.Service {
	return auth.NewService("test-client-id", "test-client-secret", "http://localhost/callback", "test-jwt-secret")
}

// TestMaskEmail pins the one scrubber everything else in the PII-logs
// sweep (#369 P1 #9–#11) relies on. The contract is: the local-part
// first character is preserved, everything else before the "@" becomes
// "***", and the domain is kept intact. A missing "@" returns the input
// unchanged — callers must ensure that bare strings without an "@" (like
// internal user_ids) use a different log key.
func TestMaskEmail(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"john.doe@example.com", "j***@example.com"},
		{"a@b.co", "a***@b.co"},
		{"kyle.galloway@cantina.ai", "k***@cantina.ai"},
		// Empty local-part ("@example.com") is nonsensical; return as-is.
		{"@example.com", "@example.com"},
		// Missing "@": return unchanged — not a real email, but scrubber
		// must not panic or synthesize.
		{"not-an-email", "not-an-email"},
		// Empty string passes through.
		{"", ""},
		// Two "@" signs: SplitN with 2 keeps the right side intact.
		{"first@middle@last.com", "f***@middle@last.com"},
	}

	for _, tc := range cases {
		got := maskEmail(tc.in)
		if got != tc.want {
			t.Errorf("maskEmail(%q) = %q, want %q", tc.in, got, tc.want)
		}
		// Contract: if input had a local-part with len > 1, the tail of
		// the local-part must NOT appear in the output. Pins the actual
		// scrubbing behavior (vs. just a string-swap).
		if at := strings.Index(tc.in, "@"); at > 1 {
			tail := tc.in[1:at]
			if strings.Contains(got, tail) {
				t.Errorf("maskEmail(%q) leaked local-part tail %q in output %q", tc.in, tail, got)
			}
		}
	}
}

func TestMapTripErr(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name     string
		err      error
		wantCode connect.Code
	}{
		{
			name:     "ErrNotOwnerOrEditor → PermissionDenied",
			err:      trip.ErrNotOwnerOrEditor,
			wantCode: connect.CodePermissionDenied,
		},
		{
			name:     "wrapped ErrNotOwnerOrEditor still maps to PermissionDenied",
			err:      fmt.Errorf("replace itinerary: %w", trip.ErrNotOwnerOrEditor),
			wantCode: connect.CodePermissionDenied,
		},
		{
			name:     "ErrInvalidStatusTransition → FailedPrecondition",
			err:      trip.ErrInvalidStatusTransition,
			wantCode: connect.CodeFailedPrecondition,
		},
		{
			name:     "wrapped ErrInvalidStatusTransition still maps to FailedPrecondition",
			err:      fmt.Errorf("trip update: %w", trip.ErrInvalidStatusTransition),
			wantCode: connect.CodeFailedPrecondition,
		},
		{
			name:     "unrelated DB error → Internal (no PII leaked)",
			err:      errors.New("pq: connection reset by peer"),
			wantCode: connect.CodeInternal,
		},
		{
			name:     "context.Canceled (transient DB failure shape) → Internal",
			err:      fmt.Errorf("check edit access: %w", context.Canceled),
			wantCode: connect.CodeInternal,
		},
		{
			name:     "ErrInvalidInitialStatus → InvalidArgument",
			err:      trip.ErrInvalidInitialStatus,
			wantCode: connect.CodeInvalidArgument,
		},
		{
			// Nil err should never happen in practice (callers guard
			// err != nil). Pin the fail-loud behaviour: a non-nil
			// Internal error so the bug surfaces in logs instead of
			// a silent success.
			name:     "nil err → Internal (bug surfaces loudly)",
			err:      nil,
			wantCode: connect.CodeInternal,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mapTripErr(ctx, "test op", tc.err)
			if got == nil {
				t.Fatalf("mapTripErr returned nil for %v", tc.err)
			}
			if got.Code() != tc.wantCode {
				t.Errorf("mapTripErr code = %v, want %v", got.Code(), tc.wantCode)
			}
			// Internal errors must scrub the message to avoid leaking
			// DB details to the client. Known sentinels are fine to
			// surface since they carry no PII (editor access required,
			// invalid status transition, etc.).
			if tc.wantCode == connect.CodeInternal {
				if msg := got.Message(); msg != "an internal error occurred" {
					t.Errorf("internal error message leaked underlying detail: %q", msg)
				}
			}
		})
	}
}

// TestClientIPFromHeaders pins the anti-spoof contract for the ConnectRPC
// header-only IP extractor. This helper keys the 5-strike auth lockout on
// RefreshToken (internal/handlers/auth.go) — if it reads the leftmost
// X-Forwarded-For entry, an attacker can forge that header per-request and
// bypass the lockout entirely. Same class of bug as #369 P1 #1 / PR #370; this
// is the duplicate call path on the ConnectRPC side.
func TestClientIPFromHeaders(t *testing.T) {
	cases := []struct {
		name    string
		headers map[string]string
		want    string
	}{
		{
			// Attacker scenario from the finding: client forges
			// "X-Forwarded-For: 1.2.3.4"; Cloud Run appends the real IP.
			// Must pick the appended (rightmost) entry.
			name:    "X-Forwarded-For attacker-supplied header does not spoof real IP",
			headers: map[string]string{"X-Forwarded-For": "1.2.3.4, 203.0.113.7"},
			want:    "203.0.113.7",
		},
		{
			name:    "X-Forwarded-For single IP returns it",
			headers: map[string]string{"X-Forwarded-For": "1.2.3.4"},
			want:    "1.2.3.4",
		},
		{
			name:    "X-Forwarded-For multiple IPs takes rightmost",
			headers: map[string]string{"X-Forwarded-For": "1.2.3.4, 10.0.0.1, 10.0.0.2"},
			want:    "10.0.0.2",
		},
		{
			name:    "X-Forwarded-For trailing empty entry is skipped",
			headers: map[string]string{"X-Forwarded-For": "1.2.3.4, 10.0.0.1, "},
			want:    "10.0.0.1",
		},
		{
			name:    "X-Forwarded-For with spaces trims rightmost",
			headers: map[string]string{"X-Forwarded-For": " 1.2.3.4 , 10.0.0.1 "},
			want:    "10.0.0.1",
		},
		{
			name:    "X-Real-IP fallback when X-Forwarded-For absent",
			headers: map[string]string{"X-Real-IP": "5.6.7.8"},
			want:    "5.6.7.8",
		},
		{
			name:    "no headers returns unknown",
			headers: map[string]string{},
			want:    "unknown",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := http.Header{}
			for k, v := range tc.headers {
				h.Set(k, v)
			}
			if got := clientIPFromHeaders(h); got != tc.want {
				t.Errorf("clientIPFromHeaders(%v) = %q, want %q", tc.headers, got, tc.want)
			}
		})
	}
}
