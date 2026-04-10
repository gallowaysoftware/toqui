package ai

import (
	"context"
	"encoding/json"
)

type EventType int

const (
	EventTextDelta EventType = iota
	EventToolCall
	EventToolResult
	EventDone
	EventError
)

type Provider interface {
	ChatStream(ctx context.Context, req *ChatRequest) (<-chan Event, error)
	Name() string
}

type ChatRequest struct {
	SystemPrompt string
	Messages     []Message
	Tools        []ToolDefinition
	MaxTokens    int
	Temperature  float64

	// ModelTier is an explicit override for model selection. When set, the
	// classifier is bypassed and this tier is used directly. When empty,
	// ClassifyRequest determines the tier from heuristics.
	ModelTier ModelTier

	// Mode is the chat mode ("selection", "planning", "companion") used by
	// the classifier to pick an appropriate model tier. This is informational
	// for routing purposes and does not affect the AI request payload.
	Mode string
}

type Message struct {
	Role        string       `json:"role"` // user, assistant, tool
	Content     string       `json:"content"`
	ToolCalls   []ToolCall   `json:"tool_calls,omitempty"`
	ToolResults []ToolResult `json:"tool_results,omitempty"`
	// ContentBlocks allows multimodal content (text + images) for user messages.
	// When set, Content is ignored and these blocks are serialized directly.
	ContentBlocks []ContentBlock `json:"content_blocks,omitempty"`
}

// ContentBlock represents a single block within a multimodal message.
type ContentBlock struct {
	Type string `json:"type"` // "text" or "image"
	// Text content (when Type == "text")
	Text string `json:"text,omitempty"`
	// Image source (when Type == "image")
	Source *ImageSource `json:"source,omitempty"`
}

// ImageSource is the base64-encoded image data for the Claude API.
type ImageSource struct {
	Type      string `json:"type"`       // "base64"
	MediaType string `json:"media_type"` // e.g. "image/jpeg"
	Data      string `json:"data"`       // base64-encoded bytes
}

type Event struct {
	Type       EventType
	Text       string
	Tool       *ToolCall
	Error      error
	StopReason string // "end_turn", "tool_use" — set on EventDone
	Usage      *Usage // token counts — populated on EventDone
}

// Usage holds token counts from a provider response.
// Both Claude and Gemini populate this on the final EventDone event.
type Usage struct {
	InputTokens  int
	OutputTokens int
}

type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
	// ThoughtSignature is a Gemini 3 opaque token that must be circulated
	// back in follow-up messages for reasoning continuity across tool-call
	// turns. Empty for Gemini 2.5 and Claude.
	ThoughtSignature string `json:"thought_signature,omitempty"`
}

type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Name       string `json:"name"`
	Content    string `json:"content"`
}

type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"` // JSON Schema
}
