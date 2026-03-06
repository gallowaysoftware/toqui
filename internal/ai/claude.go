package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

type ClaudeProvider struct {
	apiKey string
	model  string
	client *http.Client
}

func NewClaudeProvider(apiKey string) *ClaudeProvider {
	return &ClaudeProvider{
		apiKey: apiKey,
		model:  "claude-sonnet-4-20250514",
		client: http.DefaultClient,
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

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-API-Key", c.apiKey)
	httpReq.Header.Set("Anthropic-Version", "2023-06-01")
	httpReq.Header.Set("Anthropic-Beta", "prompt-caching-2024-07-31")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
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
		// No tier set — use provider default (backward compatible).
		return c.model
	}
	model := ConfigForTier(tier).ClaudeModel
	slog.Info("claude model resolved", "tier", tier, "model", model)
	return model
}

func (c *ClaudeProvider) buildRequest(req *ChatRequest) map[string]any {
	messages := make([]map[string]any, 0, len(req.Messages))
	for _, msg := range req.Messages {
		m := map[string]any{
			"role":    msg.Role,
			"content": msg.Content,
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
		// The system prompt (persona identity + trip context) is stable across
		// messages in a session, so caching it saves significant cost.
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
				"input_schema": json.RawMessage(t.Parameters),
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
	scanner := bufio.NewScanner(body)
	// Track pending tool calls by content block index
	toolBlocks := make(map[int]*pendingTool)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			ch <- Event{Type: EventError, Error: ctx.Err()}
			return
		default:
		}

		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			ch <- Event{Type: EventDone}
			return
		}

		var event struct {
			Type  string `json:"type"`
			Index int    `json:"index"`
			Delta struct {
				Type        string `json:"type"`
				Text        string `json:"text"`
				PartialJSON string `json:"partial_json"`
			} `json:"delta"`
			ContentBlock struct {
				Type  string `json:"type"`
				ID    string `json:"id"`
				Name  string `json:"name"`
				Input any    `json:"input"`
			} `json:"content_block"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		switch event.Type {
		case "content_block_start":
			if event.ContentBlock.Type == "tool_use" {
				toolBlocks[event.Index] = &pendingTool{
					id:   event.ContentBlock.ID,
					name: event.ContentBlock.Name,
				}
			}

		case "content_block_delta":
			if event.Delta.Type == "text_delta" {
				ch <- Event{Type: EventTextDelta, Text: event.Delta.Text}
			} else if event.Delta.Type == "input_json_delta" {
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

		case "message_stop":
			ch <- Event{Type: EventDone}
			return
		}
	}
}
