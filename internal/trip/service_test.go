package trip

import (
	"regexp"
	"testing"
)

func TestGenerateShareToken_Length(t *testing.T) {
	token, err := generateShareToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(token) != shareTokenLength {
		t.Errorf("expected token length %d, got %d", shareTokenLength, len(token))
	}
}

func TestGenerateShareToken_Alphanumeric(t *testing.T) {
	token, err := generateShareToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	matched, err := regexp.MatchString(`^[a-zA-Z0-9]+$`, token)
	if err != nil {
		t.Fatalf("regex error: %v", err)
	}
	if !matched {
		t.Errorf("token %q is not alphanumeric", token)
	}
}

func TestGenerateShareToken_Uniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		token, err := generateShareToken()
		if err != nil {
			t.Fatalf("unexpected error on iteration %d: %v", i, err)
		}
		if seen[token] {
			t.Errorf("duplicate token generated: %q", token)
		}
		seen[token] = true
	}
}

func TestIsValidStatusTransition(t *testing.T) {
	cases := []struct {
		current, next string
		want          bool
	}{
		// Forward path.
		{"planning", "active", true},
		{"planning", "completed", true}, // shortcut
		{"active", "completed", true},

		// Same-status updates are no-ops and must be allowed.
		{"planning", "planning", true},
		{"active", "active", true},
		{"completed", "completed", true},

		// Reverse/invalid transitions from completed (Run 4 N-08 P2).
		{"completed", "planning", false},
		{"completed", "active", false},

		// Reverse transition from active.
		{"active", "planning", false},

		// Unknown current status — allow to avoid locking legacy trips.
		{"archived", "active", true},
	}
	for _, c := range cases {
		t.Run(c.current+"_to_"+c.next, func(t *testing.T) {
			if got := isValidStatusTransition(c.current, c.next); got != c.want {
				t.Errorf("isValidStatusTransition(%q, %q) = %v; want %v", c.current, c.next, got, c.want)
			}
		})
	}
}

func TestIsValidInitialStatus(t *testing.T) {
	cases := map[string]bool{
		"planning":  true,
		"active":    true,
		"completed": false, // cannot start in terminal state
		"":          false,
		"bogus":     false,
	}
	for status, want := range cases {
		if got := isValidInitialStatus(status); got != want {
			t.Errorf("isValidInitialStatus(%q) = %v; want %v", status, got, want)
		}
	}
}
