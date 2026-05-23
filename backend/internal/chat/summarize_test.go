package chat

import (
	"testing"

	"github.com/gallowaysoftware/toqui/backend/internal/chatstore"
)

func TestNeedsSummary(t *testing.T) {
	cases := []struct {
		name    string
		session *chatstore.ChatSession
		want    bool
	}{
		{
			name:    "nil session",
			session: nil,
			want:    false,
		},
		{
			name: "below threshold — no summary needed",
			session: &chatstore.ChatSession{
				MessageCount: 30,
			},
			want: false,
		},
		{
			name: "exactly at threshold — no summary needed",
			session: &chatstore.ChatSession{
				MessageCount: 50,
			},
			want: false,
		},
		{
			name: "above threshold, no existing summary — needs summary",
			session: &chatstore.ChatSession{
				MessageCount: 51,
			},
			want: true,
		},
		{
			name: "above threshold, no existing summary, high count — needs summary",
			session: &chatstore.ChatSession{
				MessageCount: 100,
			},
			want: true,
		},
		{
			name: "has summary, not enough new messages — skip",
			session: &chatstore.ChatSession{
				MessageCount:        70,
				Summary:             "User prefers vegan food.",
				SummaryMessageCount: 60,
			},
			want: false,
		},
		{
			name: "has summary, exactly at refresh interval — refresh",
			session: &chatstore.ChatSession{
				MessageCount:        80,
				Summary:             "User prefers vegan food.",
				SummaryMessageCount: 60,
			},
			want: true,
		},
		{
			name: "has summary, well past refresh interval — refresh",
			session: &chatstore.ChatSession{
				MessageCount:        120,
				Summary:             "User prefers vegan food.",
				SummaryMessageCount: 60,
			},
			want: true,
		},
		{
			name: "has summary, 19 new messages — skip (just under threshold)",
			session: &chatstore.ChatSession{
				MessageCount:        79,
				Summary:             "User prefers vegan food.",
				SummaryMessageCount: 60,
			},
			want: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := NeedsSummary(c.session)
			if got != c.want {
				t.Errorf("NeedsSummary() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestOlderMessageCount(t *testing.T) {
	cases := []struct {
		name  string
		total int
		want  int
	}{
		{"zero messages", 0, 0},
		{"under window", 30, 0},
		{"exactly at window", 50, 0},
		{"one over window", 51, 1},
		{"well over window", 100, 50},
		{"large count", 200, 150},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := OlderMessageCount(c.total)
			if got != c.want {
				t.Errorf("OlderMessageCount(%d) = %d, want %d", c.total, got, c.want)
			}
		})
	}
}

func TestBuildSummaryContext(t *testing.T) {
	cases := []struct {
		name    string
		session *chatstore.ChatSession
		wantLen int // 0 means empty string expected
	}{
		{
			name:    "nil session — empty",
			session: nil,
			wantLen: 0,
		},
		{
			name: "no summary — empty",
			session: &chatstore.ChatSession{
				MessageCount: 100,
			},
			wantLen: 0,
		},
		{
			name: "has summary — returns context",
			session: &chatstore.ChatSession{
				MessageCount:        100,
				Summary:             "User is vegan, traveling to Japan for 10 days in March.",
				SummaryMessageCount: 80,
			},
			wantLen: 1, // non-zero, actual content
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := BuildSummaryContext(c.session)
			if c.wantLen == 0 && got != "" {
				t.Errorf("expected empty string, got %q", got)
			}
			if c.wantLen > 0 && got == "" {
				t.Error("expected non-empty string, got empty")
			}
		})
	}

	// Verify the summary content is included in the context string.
	t.Run("contains summary text", func(t *testing.T) {
		session := &chatstore.ChatSession{
			Summary:             "Traveler prefers budget accommodation and local food.",
			SummaryMessageCount: 60,
		}
		ctx := BuildSummaryContext(session)
		if ctx == "" {
			t.Fatal("expected non-empty context")
		}
		if got := ctx; !contains(got, "Traveler prefers budget accommodation") {
			t.Errorf("context should contain the summary text, got: %q", got)
		}
		if got := ctx; !contains(got, "PREVIOUS CONVERSATION SUMMARY") {
			t.Errorf("context should contain the header, got: %q", got)
		}
		if got := ctx; !contains(got, "Do not re-ask") {
			t.Errorf("context should contain the instruction not to re-ask, got: %q", got)
		}
	})
}

// contains is a simple substring check for test assertions.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsSubstring(s, substr)
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
