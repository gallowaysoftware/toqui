package handlers

import (
	"testing"

	"github.com/gallowaysoftware/toqui/backend/internal/chatstore"
)

// TestIsToolLoopIntermediate documents exactly which chat-store messages are
// hidden from the user-facing GetChatHistory response. The filter exists so
// clients never see empty assistant bubbles from tool-use turns while the
// underlying Firestore documents remain available for AI history
// reconstruction (Run 4 N-02 P0).
func TestIsToolLoopIntermediate(t *testing.T) {
	cases := []struct {
		name string
		msg  *chatstore.ChatMessage
		want bool
	}{
		{
			name: "nil message is not an intermediate",
			msg:  nil,
			want: false,
		},
		{
			name: "normal user message",
			msg: &chatstore.ChatMessage{
				Role:    "user",
				Content: "plan me a trip",
			},
			want: false,
		},
		{
			name: "normal assistant message",
			msg: &chatstore.ChatMessage{
				Role:    "assistant",
				Content: "Sure — which destination?",
			},
			want: false,
		},
		{
			name: "assistant with empty content and tool calls — HIDDEN",
			msg: &chatstore.ChatMessage{
				Role: "assistant",
				ToolCalls: []chatstore.StoredToolCall{
					{ID: "call-1", Name: "create_itinerary_items", Arguments: "{}"},
				},
			},
			want: true,
		},
		{
			name: "assistant with whitespace-only content and tool calls — HIDDEN",
			msg: &chatstore.ChatMessage{
				Role:    "assistant",
				Content: "   \n\t  ",
				ToolCalls: []chatstore.StoredToolCall{
					{ID: "call-1", Name: "web_search", Arguments: "{}"},
				},
			},
			want: true,
		},
		{
			name: "user-role message carrying only tool results — HIDDEN",
			msg: &chatstore.ChatMessage{
				Role: "user",
				ToolResults: []chatstore.StoredToolResult{
					{ToolCallID: "call-1", Name: "create_itinerary_items", Content: "{}"},
				},
			},
			want: true,
		},
		{
			name: "assistant with text AND tool calls is NOT hidden",
			msg: &chatstore.ChatMessage{
				Role:    "assistant",
				Content: "Let me add those to your itinerary:",
				ToolCalls: []chatstore.StoredToolCall{
					{ID: "call-1", Name: "create_itinerary_items", Arguments: "{}"},
				},
			},
			want: false,
		},
		{
			name: "empty assistant message with no tool calls IS hidden (blank bubble)",
			msg: &chatstore.ChatMessage{
				Role:    "assistant",
				Content: "",
			},
			want: true,
		},
		{
			name: "empty user message with no tool results is NOT hidden",
			msg: &chatstore.ChatMessage{
				Role:    "user",
				Content: "",
			},
			want: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isToolLoopIntermediate(c.msg); got != c.want {
				t.Errorf("isToolLoopIntermediate() = %v; want %v", got, c.want)
			}
		})
	}
}
