package ai

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestClassifyRequest(t *testing.T) {
	// Helper to build a tool definition slice with one dummy tool.
	withTools := func() []ToolDefinition {
		return []ToolDefinition{
			{
				Name:        "create_trip",
				Description: "Create a new trip",
				Parameters:  json.RawMessage(`{"type":"object"}`),
			},
		}
	}

	tests := []struct {
		name     string
		req      *ChatRequest
		wantTier ModelTier
	}{
		// --- Explicit override ---
		{
			name: "explicit override to fast",
			req: &ChatRequest{
				ModelTier: ModelTierFast,
				Mode:      "planning",
				Messages:  []Message{{Role: "user", Content: "Plan a 2-week Japan itinerary with specific restaurants"}},
				Tools:     withTools(),
			},
			wantTier: ModelTierFast,
		},
		{
			name: "explicit override to best",
			req: &ChatRequest{
				ModelTier: ModelTierBest,
				Mode:      "selection",
				Messages:  []Message{{Role: "user", Content: "hi"}},
			},
			wantTier: ModelTierBest,
		},

		// --- Selection mode ---
		{
			name: "selection mode without tools is fast",
			req: &ChatRequest{
				Mode:     "selection",
				Messages: []Message{{Role: "user", Content: "hi"}},
			},
			wantTier: ModelTierFast,
		},
		{
			name: "selection mode with tools is smart",
			req: &ChatRequest{
				Mode:     "selection",
				Messages: []Message{{Role: "user", Content: "I want to plan a trip to Japan"}},
				Tools:    withTools(),
			},
			wantTier: ModelTierSmart,
		},

		// --- Companion mode ---
		{
			name: "companion short question without tools is fast",
			req: &ChatRequest{
				Mode:     "companion",
				Messages: []Message{{Role: "user", Content: "What time does the museum open?"}},
			},
			wantTier: ModelTierFast,
		},
		{
			name: "companion long question is smart",
			req: &ChatRequest{
				Mode:     "companion",
				Messages: []Message{{Role: "user", Content: strings.Repeat("x", 150)}},
			},
			wantTier: ModelTierSmart,
		},
		{
			name: "companion with tools is smart",
			req: &ChatRequest{
				Mode:     "companion",
				Messages: []Message{{Role: "user", Content: "Find me a restaurant nearby"}},
				Tools:    withTools(),
			},
			wantTier: ModelTierSmart,
		},

		// --- Planning mode ---
		{
			name: "planning mode short message is smart",
			req: &ChatRequest{
				Mode:     "planning",
				Messages: []Message{{Role: "user", Content: "hi"}},
			},
			wantTier: ModelTierSmart,
		},
		{
			name: "planning mode with tools is smart",
			req: &ChatRequest{
				Mode:     "planning",
				Messages: []Message{{Role: "user", Content: "Plan me a 2-week itinerary for Japan"}},
				Tools:    withTools(),
			},
			wantTier: ModelTierSmart,
		},
		{
			name: "planning mode long message is smart",
			req: &ChatRequest{
				Mode:     "planning",
				Messages: []Message{{Role: "user", Content: strings.Repeat("plan details ", 50)}},
				Tools:    withTools(),
			},
			wantTier: ModelTierSmart,
		},

		// --- No mode (fallback) ---
		{
			name: "no mode, no tools, short message is fast",
			req: &ChatRequest{
				Messages: []Message{{Role: "user", Content: "hello"}},
			},
			wantTier: ModelTierFast,
		},
		{
			name: "no mode, with tools is smart",
			req: &ChatRequest{
				Messages: []Message{{Role: "user", Content: "hello"}},
				Tools:    withTools(),
			},
			wantTier: ModelTierSmart,
		},
		{
			name: "no mode, no tools, long message is smart",
			req: &ChatRequest{
				Messages: []Message{{Role: "user", Content: strings.Repeat("x", 100)}},
			},
			wantTier: ModelTierSmart,
		},

		// --- Edge cases ---
		{
			name: "empty messages is smart (conservative)",
			req: &ChatRequest{
				Mode: "planning",
			},
			wantTier: ModelTierSmart,
		},
		{
			name: "only assistant messages, no user message, companion",
			req: &ChatRequest{
				Mode:     "companion",
				Messages: []Message{{Role: "assistant", Content: "How can I help you?"}},
			},
			wantTier: ModelTierSmart,
		},
		{
			name: "multiple messages uses last user message length",
			req: &ChatRequest{
				Mode: "companion",
				Messages: []Message{
					{Role: "user", Content: strings.Repeat("x", 200)},
					{Role: "assistant", Content: "Here is the answer..."},
					{Role: "user", Content: "thanks"},
				},
			},
			wantTier: ModelTierFast,
		},
		{
			name: "companion exactly 100 chars is smart",
			req: &ChatRequest{
				Mode:     "companion",
				Messages: []Message{{Role: "user", Content: strings.Repeat("x", 100)}},
			},
			wantTier: ModelTierSmart,
		},
		{
			name: "fallback exactly 50 chars is smart",
			req: &ChatRequest{
				Messages: []Message{{Role: "user", Content: strings.Repeat("x", 50)}},
			},
			wantTier: ModelTierSmart,
		},
		{
			name: "fallback 49 chars is fast",
			req: &ChatRequest{
				Messages: []Message{{Role: "user", Content: strings.Repeat("x", 49)}},
			},
			wantTier: ModelTierFast,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyRequest(tt.req)
			if got != tt.wantTier {
				t.Errorf("ClassifyRequest() = %q, want %q", got, tt.wantTier)
			}
		})
	}
}

func TestLastUserMessageLength(t *testing.T) {
	tests := []struct {
		name     string
		messages []Message
		want     int
	}{
		{
			name:     "empty messages",
			messages: nil,
			want:     0,
		},
		{
			name:     "single user message",
			messages: []Message{{Role: "user", Content: "hello"}},
			want:     5,
		},
		{
			name: "multiple messages, last is user",
			messages: []Message{
				{Role: "user", Content: "first"},
				{Role: "assistant", Content: "response"},
				{Role: "user", Content: "second message here"},
			},
			want: 19,
		},
		{
			name: "last message is assistant",
			messages: []Message{
				{Role: "user", Content: "question"},
				{Role: "assistant", Content: "answer"},
			},
			want: 8,
		},
		{
			name: "unicode characters counted as runes",
			messages: []Message{
				{Role: "user", Content: "cafe\u0301"},
			},
			want: 5, // c-a-f-e-combining accent = 5 runes
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &ChatRequest{Messages: tt.messages}
			got := lastUserMessageLength(req)
			if got != tt.want {
				t.Errorf("lastUserMessageLength() = %d, want %d", got, tt.want)
			}
		})
	}
}
