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
}

type Event struct {
	Type    EventType
	Text    string
	Tool    *ToolCall
	Error   error
}

type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
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
