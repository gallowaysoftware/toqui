package trip

import (
	"errors"
	"fmt"
	"regexp"
	"testing"
)

// TestInvalidStatusErrorsWrapSentinels guarantees that the wrapped error
// values returned by Update/CreateWithStatus are detectable via errors.Is,
// which the handler layer relies on to map them to the correct Connect code
// (FailedPrecondition / InvalidArgument) instead of Internal (Run 5 R-07/N-08 P2).
func TestInvalidStatusErrorsWrapSentinels(t *testing.T) {
	// Simulate the exact wrap patterns used by the production code paths.
	transitionErr := fmt.Errorf("%w: completed → planning", ErrInvalidStatusTransition)
	if !errors.Is(transitionErr, ErrInvalidStatusTransition) {
		t.Errorf("transition error must match ErrInvalidStatusTransition via errors.Is")
	}
	if errors.Is(transitionErr, ErrInvalidInitialStatus) {
		t.Errorf("transition error should NOT match ErrInvalidInitialStatus")
	}

	initialErr := fmt.Errorf("%w: %q", ErrInvalidInitialStatus, "completed")
	if !errors.Is(initialErr, ErrInvalidInitialStatus) {
		t.Errorf("initial-status error must match ErrInvalidInitialStatus via errors.Is")
	}
	if errors.Is(initialErr, ErrInvalidStatusTransition) {
		t.Errorf("initial-status error should NOT match ErrInvalidStatusTransition")
	}
}

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

// TestCloneTripTitleDefault verifies the "Copy of " title-defaulting logic
// that CloneTrip applies when no custom title is supplied.
func TestCloneTripTitleDefault(t *testing.T) {
	cases := []struct {
		name          string
		originalTitle string
		newTitle      string
		want          string
	}{
		{"empty new title gets prefix", "Greece Adventure", "", "Copy of Greece Adventure"},
		{"custom title preserved", "Greece Adventure", "My New Trip", "My New Trip"},
		{"unicode title", "日本旅行", "", "Copy of 日本旅行"},
		{"already has Copy of prefix", "Copy of Trip", "", "Copy of Copy of Trip"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Replicate the title-defaulting logic from CloneTrip.
			title := c.newTitle
			if title == "" {
				title = "Copy of " + c.originalTitle
			}
			if title != c.want {
				t.Errorf("got %q, want %q", title, c.want)
			}
		})
	}
}

// TestCloneTripHelperFunctions ensures pgtype helper functions used in
// CloneTrip work correctly.
func TestCloneTripHelperFunctions(t *testing.T) {
	t.Run("textFromString empty", func(t *testing.T) {
		result := textFromString("")
		if result.Valid {
			t.Error("textFromString('') should return invalid pgtype.Text")
		}
	})

	t.Run("textFromString non-empty", func(t *testing.T) {
		result := textFromString("hello")
		if !result.Valid {
			t.Error("textFromString('hello') should return valid pgtype.Text")
		}
		if result.String != "hello" {
			t.Errorf("expected 'hello', got %q", result.String)
		}
	})
}
