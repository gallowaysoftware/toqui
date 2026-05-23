package email

import "testing"

func TestMaskEmail(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"john.doe@example.com", "j***@example.com"},
		{"k@toqui.travel", "k***@toqui.travel"},
		// Caller passed something non-email — return unchanged so the
		// log still has *something* useful and we don't pretend to
		// have masked when there was nothing to mask.
		{"not-an-email", "not-an-email"},
		{"", ""},
		// Empty local part: don't return "@example.com" because that
		// silently turns "@example.com" into a string that looks like
		// a real but truncated email. Return the input as-is.
		{"@example.com", "@example.com"},
		// Trailing "@" with no domain — return unchanged.
		{"local@", "local@"},
		// Adversarial multi-"@" input: split on the LAST "@" so the
		// quoted localpart gets fully masked instead of leaking part of
		// it after the first "@".
		{"a@b@example.com", "a***@example.com"},
		// Unicode first character: mask by rune, not byte. The previous
		// implementation emitted "\xc3***@…" and corrupted the log line.
		{"étienne@example.com", "é***@example.com"},
	}
	for _, c := range cases {
		got := MaskEmail(c.in)
		if got != c.want {
			t.Errorf("MaskEmail(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
