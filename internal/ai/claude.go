package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// Claude model defaults per tier. Override via AI_MODEL_FAST, AI_MODEL_SMART, AI_MODEL_BEST.
var claudeModels = map[ModelTier]string{
	ModelTierFast:  getEnvOrDefault("AI_MODEL_FAST", "claude-3-5-haiku-latest"),
	ModelTierSmart: getEnvOrDefault("AI_MODEL_SMART", "claude-sonnet-4-20250514"),
	ModelTierBest:  getEnvOrDefault("AI_MODEL_BEST", "claude-sonnet-4-20250514"),
}

type ClaudeProvider struct {
	apiKey string
	model  string
	client *http.Client
}

func NewClaudeProvider(apiKey string) *ClaudeProvider {
	return &ClaudeProvider{
		apiKey: apiKey,
		model:  "claude-sonnet-4-20250514",
		client: &http.Client{Timeout: 5 * time.Minute},
	}
}

func (c *ClaudeProvider) Name() string {
	return "claude"
}

func (c *ClaudeProvider) ChatStream(ctx context.Context, req *ChatRequest) (<-chan Event, error) {
	body := c.buildRequest(req)

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-API-Key", c.apiKey)
	httpReq.Header.Set("Anthropic-Version", "2023-06-01")
	httpReq.Header.Set("Anthropic-Beta", "prompt-caching-2024-07-31")

	resp, err := c.client.Do(httpReq) //nolint:bodyclose // body is closed in the goroutine below
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, respBody)
	}

	ch := make(chan Event, 64)
	go func() {
		defer close(ch)
		defer resp.Body.Close()
		c.processStream(ctx, resp.Body, ch)
	}()

	return ch, nil
}

// resolveModel returns the Claude model identifier for the request's tier.
// If the request has no tier set, it falls back to the provider's default model.
func (c *ClaudeProvider) resolveModel(req *ChatRequest) string {
	tier := req.ModelTier
	if tier == "" {
		return c.model
	}
	if model, ok := claudeModels[tier]; ok {
		slog.Info("claude model resolved", "tier", tier, "model", model)
		return model
	}
	slog.Warn("unknown tier for claude, using default", "tier", tier)
	return c.model
}

func (c *ClaudeProvider) buildRequest(req *ChatRequest) map[string]any {
	messages := make([]map[string]any, 0, len(req.Messages))
	for _, msg := range req.Messages {
		var m map[string]any

		if len(msg.ToolCalls) > 0 || len(msg.ToolResults) > 0 {
			// Multi-part content for tool_use (assistant) or tool_result (user) messages
			var content []map[string]any

			// Include text content if present
			if msg.Content != "" {
				content = append(content, map[string]any{
					"type": "text",
					"text": msg.Content,
				})
			}

			// Assistant messages include tool_use blocks
			for _, tc := range msg.ToolCalls {
				var input json.RawMessage
				if tc.Arguments != "" {
					input = json.RawMessage(tc.Arguments)
				} else {
					input = json.RawMessage("{}")
				}
				content = append(content, map[string]any{
					"type":  "tool_use",
					"id":    tc.ID,
					"name":  tc.Name,
					"input": input,
				})
			}

			// User messages include tool_result blocks
			for _, tr := range msg.ToolResults {
				content = append(content, map[string]any{
					"type":        "tool_result",
					"tool_use_id": tr.ToolCallID,
					"content":     tr.Content,
				})
			}

			m = map[string]any{
				"role":    msg.Role,
				"content": content,
			}
		} else {
			m = map[string]any{
				"role":    msg.Role,
				"content": msg.Content,
			}
		}

		messages = append(messages, m)
	}

	model := c.resolveModel(req)

	body := map[string]any{
		"model":      model,
		"max_tokens": req.MaxTokens,
		"messages":   messages,
		"stream":     true,
	}

	if req.SystemPrompt != "" {
		// Use structured system format with cache_control for prompt caching.
		body["system"] = []map[string]any{
			{
				"type": "text",
				"text": req.SystemPrompt,
				"cache_control": map[string]string{
					"type": "ephemeral",
				},
			},
		}
	}

	if req.Temperature > 0 {
		body["temperature"] = req.Temperature
	}

	if len(req.Tools) > 0 {
		tools := make([]map[string]any, 0, len(req.Tools))
		for i, t := range req.Tools {
			tool := map[string]any{
				"name":         t.Name,
				"description":  t.Description,
				"input_schema": t.Parameters,
			}
			// Cache the last tool definition (covers the full tool block)
			if i == len(req.Tools)-1 {
				tool["cache_control"] = map[string]string{"type": "ephemeral"}
			}
			tools = append(tools, tool)
		}
		body["tools"] = tools
	}

	if req.MaxTokens == 0 {
		body["max_tokens"] = 4096
	}

	return body
}

type pendingTool struct {
	id   string
	name string
	args strings.Builder
}

func (c *ClaudeProvider) processStream(ctx context.Context, body io.Reader, ch chan<- Event) {
	reader := NewSSEReader(body)
	// Track pending tool calls by content block index
	toolBlocks := make(map[int]*pendingTool)
	var stopReason string
	var usage *Usage

	for {
		data, done, err := reader.Next(ctx)
		if err != nil {
			ch <- Event{Type: EventError, Error: err}
			return
		}
		if done {
			ch <- Event{Type: EventDone, StopReason: stopReason, Usage: usage}
			return
		}

		var event struct {
			Type    string `json:"type"`
			Index   int    `json:"index"`
			Message struct {
				Usage struct {
					InputTokens  int `json:"input_tokens"`
					OutputTokens int `json:"output_tokens"`
				} `json:"usage"`
			} `json:"message"`
			Delta struct {
				Type        string `json:"type"`
				Text        string `json:"text"`
				PartialJSON string `json:"partial_json"`
				StopReason  string `json:"stop_reason"`
			} `json:"delta"`
			ContentBlock struct {
				Type  string `json:"type"`
				ID    string `json:"id"`
				Name  string `json:"name"`
				Input any    `json:"input"`
			} `json:"content_block"`
			Usage struct {
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		switch event.Type {
		case "message_start":
			// Capture initial usage (input tokens).
			if event.Message.Usage.InputTokens > 0 {
				usage = &Usage{
					InputTokens:  event.Message.Usage.InputTokens,
					OutputTokens: event.Message.Usage.OutputTokens,
				}
			}

		case "content_block_start":
			if event.ContentBlock.Type == "tool_use" {
				toolBlocks[event.Index] = &pendingTool{
					id:   event.ContentBlock.ID,
					name: event.ContentBlock.Name,
				}
			}

		case "content_block_delta":
			switch event.Delta.Type {
			case "text_delta":
				ch <- Event{Type: EventTextDelta, Text: event.Delta.Text}
			case "input_json_delta":
				if pt, ok := toolBlocks[event.Index]; ok {
					pt.args.WriteString(event.Delta.PartialJSON)
				}
			}

		case "content_block_stop":
			if pt, ok := toolBlocks[event.Index]; ok {
				args := pt.args.String()
				if args == "" {
					args = "{}"
				}
				ch <- Event{
					Type: EventToolCall,
					Tool: &ToolCall{
						ID:        pt.id,
						Name:      pt.name,
						Arguments: args,
					},
				}
				delete(toolBlocks, event.Index)
			}

		case "message_delta":
			if event.Delta.StopReason != "" {
				stopReason = event.Delta.StopReason
			}
			// Update output tokens from message_delta.
			if event.Usage.OutputTokens > 0 && usage != nil {
				usage.OutputTokens = event.Usage.OutputTokens
			}

		case "message_stop":
			ch <- Event{Type: EventDone, StopReason: stopReason, Usage: usage}
			return
		}
	}
}
