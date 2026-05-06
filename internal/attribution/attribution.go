// Package attribution parses the optional UTM/ref attribution payload that
// the marketing site (toqui-site/AttributionCapture.astro) captures on first
// visit and forwards on signup via the GoogleLogin / FacebookLogin /
// AppleLogin gRPC requests (and the REST /auth/exchange query param).
//
// The payload becomes properties on the existing PostHog `signup_completed`
// event, then is discarded — there is no database column for attribution.
//
// Privacy / safety:
//   - Whitelist only: ref, utm_source, utm_medium, utm_campaign. Everything
//     else is silently dropped.
//   - Each value is sanitized to ASCII alnum + `-_`, capped at 64 chars,
//     to prevent log injection / control-character abuse.
//   - Bad input (malformed base64, malformed JSON, oversized payload) is
//     logged and ignored. Login NEVER fails over a bad attribution string
//     — attribution is best-effort metadata, not a security control.
//
// See: audit issue #39 A-2.
package attribution

import (
	"encoding/base64"
	"encoding/json"
	"log/slog"
)

// maxEncodedLen caps the base64-encoded input we'll even attempt to decode.
// The whole payload (4 fields × 64 chars + JSON overhead) is well under 1 KB
// raw, so this is generous enough to survive a future field addition while
// still rejecting griefing attempts that send megabytes.
const maxEncodedLen = 2048

// maxValueLen caps each individual attribution value after sanitization.
// 64 chars matches what AttributionCapture.astro accepts client-side.
const maxValueLen = 64

// Attribution is the sanitized, in-memory representation of the payload.
// All fields are optional — empty strings mean "not present".
type Attribution struct {
	Ref      string `json:"ref,omitempty"`
	Source   string `json:"utm_source,omitempty"`
	Medium   string `json:"utm_medium,omitempty"`
	Campaign string `json:"utm_campaign,omitempty"`
}

// Empty reports whether no attribution fields are populated.
func (a Attribution) Empty() bool {
	return a.Ref == "" && a.Source == "" && a.Medium == "" && a.Campaign == ""
}

// Properties returns a map suitable for merging into a PostHog event's
// properties. Keys are namespaced with `attribution_` so they coexist
// cleanly alongside the existing `signup_completed` props (e.g.
// `auth_provider`). Empty values are omitted — we don't send keys with
// "" because PostHog still indexes them, and it pollutes the property
// list visible to operators in the dashboard.
func (a Attribution) Properties() map[string]any {
	if a.Empty() {
		return nil
	}
	out := make(map[string]any, 4)
	if a.Ref != "" {
		out["attribution_ref"] = a.Ref
	}
	if a.Source != "" {
		out["attribution_source"] = a.Source
	}
	if a.Medium != "" {
		out["attribution_medium"] = a.Medium
	}
	if a.Campaign != "" {
		out["attribution_campaign"] = a.Campaign
	}
	return out
}

// Parse decodes a base64-encoded JSON attribution payload, applies the
// whitelist + sanitization, and returns the result. The error return is
// always nil — callers are expected to use the returned (possibly empty)
// Attribution unconditionally. Bad input is logged at warn level so
// operators can spot client-side bugs without breaking signup.
//
// Returning a value (not error) is intentional: the rule per audit issue
// #39 A-2 is "never fail login over a bad attribution string." If the
// signature ever needs to grow an error return, callers MUST still treat
// errors as soft failures.
func Parse(encoded string) Attribution {
	if encoded == "" {
		return Attribution{}
	}
	if len(encoded) > maxEncodedLen {
		slog.Warn("attribution: payload too large, ignoring",
			"encoded_len", len(encoded),
			"max_len", maxEncodedLen,
		)
		return Attribution{}
	}

	// Accept both standard and URL-safe base64. The marketing site uses
	// btoa() (standard alphabet); a future client (or a manually-crafted
	// query param) might use base64url. Try standard first.
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		// Standard base64 with padding requires length % 4 == 0. Fall
		// back to URL-safe + raw (no padding) before giving up.
		raw, err = base64.RawURLEncoding.DecodeString(encoded)
		if err != nil {
			slog.Warn("attribution: base64 decode failed, ignoring",
				"error", err,
			)
			return Attribution{}
		}
	}

	// JSON shape mirrors what AttributionCapture.astro writes. We
	// deliberately don't unmarshal into Attribution directly — a typo'd
	// field would be silently dropped, but using a separate raw struct
	// makes the whitelist explicit and easier to audit.
	var rawPayload struct {
		Ref      string `json:"ref"`
		Source   string `json:"utm_source"`
		Medium   string `json:"utm_medium"`
		Campaign string `json:"utm_campaign"`
		// captured_at intentionally not consumed — it's noise for analytics.
	}
	if err := json.Unmarshal(raw, &rawPayload); err != nil {
		slog.Warn("attribution: json unmarshal failed, ignoring",
			"error", err,
		)
		return Attribution{}
	}

	return Attribution{
		Ref:      sanitize(rawPayload.Ref),
		Source:   sanitize(rawPayload.Source),
		Medium:   sanitize(rawPayload.Medium),
		Campaign: sanitize(rawPayload.Campaign),
	}
}

// sanitize coerces a raw value to a safe ASCII subset suitable for embedding
// in structured logs and PostHog properties. The allowed set is alnum +
// `-` + `_`. Anything else is dropped (not replaced) so an adversarial
// `"\nFAKE_LOG_LINE"` becomes `FAKE_LOG_LINE`, not `_FAKE_LOG_LINE` or
// similar that hints at the attempted injection. After sanitization we
// truncate to maxValueLen.
//
// Empty input returns empty output. A value that becomes empty after
// sanitization (e.g. "🎉🎉🎉") returns empty, signaling "no attribution"
// for that field rather than passing through ambiguous data.
func sanitize(s string) string {
	if s == "" {
		return ""
	}
	// Single allocation, written byte-by-byte. Avoids Replace chains and
	// keeps the predicate co-located with the sanitization rule.
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s) && len(out) < maxValueLen; i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z':
			out = append(out, c)
		case c >= 'A' && c <= 'Z':
			out = append(out, c)
		case c >= '0' && c <= '9':
			out = append(out, c)
		case c == '-' || c == '_':
			out = append(out, c)
		}
	}
	return string(out)
}
