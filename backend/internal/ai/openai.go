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

// OpenAI model defaults per tier. Override via OPENAI_MODEL_FAST, OPENAI_MODEL_SMART, OPENAI_MODEL_BEST.
//
// Defaults target the OpenAI public API. When pointing at an
// OpenAI-compatible endpoint (Ollama, OpenRouter, vLLM, LM Studio, Together
// AI, etc.) via OPENAI_BASE_URL, override these to whatever models that
// endpoint exposes (e.g. OPENAI_MODEL_SMART=llama-3.1-70b-instruct).
var openAIModels = map[ModelTier]string{
	ModelTierFast:  getEnvOrDefault("OPENAI_MODEL_FAST", "gpt-4o-mini"),
	ModelTierSmart: getEnvOrDefault("OPENAI_MODEL_SMART", "gpt-4o"),
	ModelTierBest:  getEnvOrDefault("OPENAI_MODEL_BEST", "gpt-4o"),
}

const openAIDefaultBaseURL = "https://api.openai.com/v1"

// OpenAIProvider implements Provider against any OpenAI-compatible
// /v1/chat/completions endpoint with SSE streaming. baseURL is overridable so
// self-hosters can target OpenRouter, Ollama (http://host:11434/v1), vLLM,
// LM Studio, Together AI, etc. without code changes.
type OpenAIProvider struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

// NewOpenAIProvider creates a provider talking to ${baseURL}/chat/completions.
// If baseURL is empty, the public OpenAI API endpoint is used. The trailing
// slash on baseURL is trimmed so callers can pass either form.
func NewOpenAIProvider(apiKey, baseURL string) *OpenAIProvider {
	if baseURL == "" {
		baseURL = openAIDefaultBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")
	return &OpenAIProvider{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   openAIModels[ModelTierSmart],
		client:  &http.Client{Timeout: 5 * time.Minute},
	}
}

func (o *OpenAIProvider) Name() string {
	return "openai"
}

func (o *OpenAIProvider) ChatStream(ctx context.Context, req *ChatRequest) (<-chan Event, error) {
	body := o.buildRequest(req)

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	endpoint := o.baseURL + "/chat/completions"

	cfg := defaultRetryConfig()
	resp, err := doWithRetry(ctx, cfg, "openai", func() (*http.Response, error) { //nolint:bodyclose // body is closed in the goroutine below; doWithRetry closes on retries
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(jsonBody))
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}

		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Accept", "text/event-stream")
		if o.apiKey != "" {
			httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)
		}

		return o.client.Do(httpReq)
	})
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}

	ch := make(chan Event, 64)
	go func() {
		defer close(ch)
		defer resp.Body.Close()
		o.processStream(ctx, resp.Body, ch)
	}()

	return ch, nil
}

// resolveModel returns the OpenAI model identifier for the request's tier.
// If the request has no tier set, it falls back to the provider's default model.
func (o *OpenAIProvider) resolveModel(req *ChatRequest) string {
	tier := req.ModelTier
	if tier == "" {
		return o.model
	}
	if model, ok := openAIModels[tier]; ok {
		slog.Info("openai model resolved", "tier", tier, "model", model)
		return model
	}
	slog.Warn("unknown tier for openai, using default", "tier", tier)
	return o.model
}

// buildRequest constructs the OpenAI /chat/completions request body.
//
// Key shape differences vs. Claude:
//   - System prompt is a "system" role message at the head of messages[].
//   - Tool calls live on assistant messages as a parallel tool_calls[] array;
//     each entry has {id, type:"function", function:{name, arguments(JSON string)}}.
//   - Tool results are separate {role:"tool", tool_call_id, content} messages.
//   - Multimodal user messages use content as an array of
//     {type:"text"|"image_url", ...} parts.
func (o *OpenAIProvider) buildRequest(req *ChatRequest) map[string]any {
	messages := make([]map[string]any, 0, len(req.Messages)+1)

	if req.SystemPrompt != "" {
		messages = append(messages, map[string]any{
			"role":    "system",
			"content": req.SystemPrompt,
		})
	}

	for _, msg := range req.Messages {
		// Tool result messages get split out into separate {role:"tool", ...}
		// messages — OpenAI does not allow a user message to carry tool
		// results inline the way Claude does.
		if len(msg.ToolResults) > 0 {
			// Emit any leading text content as its own user message first.
			if msg.Content != "" {
				messages = append(messages, map[string]any{
					"role":    msg.Role,
					"content": msg.Content,
				})
			}
			for _, tr := range msg.ToolResults {
				messages = append(messages, map[string]any{
					"role":         "tool",
					"tool_call_id": tr.ToolCallID,
					"content":      tr.Content,
				})
			}
			continue
		}

		// Assistant message that issued tool calls: include parallel
		// tool_calls[] with stringified JSON arguments.
		if len(msg.ToolCalls) > 0 {
			toolCalls := make([]map[string]any, 0, len(msg.ToolCalls))
			for _, tc := range msg.ToolCalls {
				args := tc.Arguments
				if args == "" {
					args = "{}"
				}
				toolCalls = append(toolCalls, map[string]any{
					"id":   tc.ID,
					"type": "function",
					"function": map[string]any{
						"name":      tc.Name,
						"arguments": args,
					},
				})
			}
			m := map[string]any{
				"role":       msg.Role,
				"tool_calls": toolCalls,
			}
			// OpenAI requires content to be present even when null; an empty
			// string is the canonical "no preamble" value.
			if msg.Content != "" {
				m["content"] = msg.Content
			} else {
				m["content"] = ""
			}
			messages = append(messages, m)
			continue
		}

		// Multimodal user message — translate ContentBlocks to OpenAI's
		// array-of-parts representation.
		if len(msg.ContentBlocks) > 0 {
			parts := make([]map[string]any, 0, len(msg.ContentBlocks))
			for _, block := range msg.ContentBlocks {
				switch block.Type {
				case "text":
					parts = append(parts, map[string]any{
						"type": "text",
						"text": block.Text,
					})
				case "image":
					if block.Source == nil {
						continue
					}
					// OpenAI accepts both base64 data URLs and remote URLs in
					// image_url.url. ContentBlock carries base64 data; encode
					// as a data URL so the same payload flows through any
					// OpenAI-compatible endpoint that supports vision.
					url := fmt.Sprintf("data:%s;base64,%s", block.Source.MediaType, block.Source.Data)
					parts = append(parts, map[string]any{
						"type": "image_url",
						"image_url": map[string]any{
							"url": url,
						},
					})
				}
			}
			messages = append(messages, map[string]any{
				"role":    msg.Role,
				"content": parts,
			})
			continue
		}

		// Plain text message.
		messages = append(messages, map[string]any{
			"role":    msg.Role,
			"content": msg.Content,
		})
	}

	model := o.resolveModel(req)

	body := map[string]any{
		"model":    model,
		"messages": messages,
		"stream":   true,
		// Ask for usage in the streaming response. OpenAI emits a final chunk
		// with usage populated when this is set; compatible servers ignore it.
		"stream_options": map[string]any{
			"include_usage": true,
		},
	}

	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	} else {
		body["max_tokens"] = 4096
	}

	if req.Temperature > 0 {
		body["temperature"] = req.Temperature
	}

	if len(req.Tools) > 0 {
		tools := make([]map[string]any, 0, len(req.Tools))
		for _, t := range req.Tools {
			var params any = t.Parameters
			// Default to an empty object schema when caller omitted parameters.
			if len(t.Parameters) == 0 {
				params = map[string]any{"type": "object", "properties": map[string]any{}}
			}
			tools = append(tools, map[string]any{
				"type": "function",
				"function": map[string]any{
					"name":        t.Name,
					"description": t.Description,
					"parameters":  params,
				},
			})
		}
		body["tools"] = tools
		body["tool_choice"] = "auto"
	}

	return body
}

// pendingOpenAITool accumulates a tool call across many streaming deltas.
// OpenAI sends a tool call's id + function.name once (or in early chunks) and
// then streams function.arguments as a sequence of partial JSON string
// fragments keyed by index — so the parser must stitch them by index.
type pendingOpenAITool struct {
	id   string
	name string
	args strings.Builder
}

func (o *OpenAIProvider) processStream(ctx context.Context, body io.Reader, ch chan<- Event) {
	reader := NewSSEReader(body)

	// Tool calls keyed by their index in delta.tool_calls[]. OpenAI uses the
	// index as the stable identifier across deltas; id/name may appear in
	// later chunks but always for the same index.
	toolBlocks := make(map[int]*pendingOpenAITool)
	// Preserve emission order so multi-tool turns dispatch in the same order
	// the model produced them.
	toolOrder := make([]int, 0, 4)

	var stopReason string
	var usage *Usage

	for {
		data, done, err := reader.Next(ctx)
		if err != nil {
			slog.Error("openai stream error", "error", err)
			ch <- Event{Type: EventError, Error: SanitizeProviderError(err)}
			return
		}
		if done {
			// Emit accumulated tool calls (if the upstream closed without an
			// explicit [DONE]+finish_reason path, we still want them out).
			o.emitPendingTools(ch, toolBlocks, toolOrder)
			ch <- Event{Type: EventDone, StopReason: stopReason, Usage: usage}
			return
		}

		var chunk struct {
			Choices []struct {
				Index int `json:"index"`
				Delta struct {
					Role      string `json:"role"`
					Content   string `json:"content"`
					ToolCalls []struct {
						Index    int    `json:"index"`
						ID       string `json:"id"`
						Type     string `json:"type"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
			Usage *struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
			} `json:"usage"`
			// Error envelope returned by some compatible servers mid-stream
			// (e.g. Ollama returns a final chunk with an error field instead
			// of failing the HTTP status).
			Error *struct {
				Message string `json:"message"`
				Type    string `json:"type"`
			} `json:"error"`
		}

		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			// Never log raw SSE data — it could leak chat content.
			slog.Warn("openai SSE: failed to unmarshal chunk", "error", err, "data_len", len(data))
			continue
		}

		if chunk.Error != nil {
			ch <- Event{Type: EventError, Error: SanitizeProviderError(fmt.Errorf("openai stream error: %s", chunk.Error.Message))}
			return
		}

		// Final usage-only chunk (OpenAI emits this after the last choice when
		// stream_options.include_usage is true) — choices is empty.
		if chunk.Usage != nil {
			usage = &Usage{
				InputTokens:  chunk.Usage.PromptTokens,
				OutputTokens: chunk.Usage.CompletionTokens,
			}
		}

		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				ch <- Event{Type: EventTextDelta, Text: choice.Delta.Content}
			}

			for _, tc := range choice.Delta.ToolCalls {
				pt, ok := toolBlocks[tc.Index]
				if !ok {
					pt = &pendingOpenAITool{}
					toolBlocks[tc.Index] = pt
					toolOrder = append(toolOrder, tc.Index)
				}
				if tc.ID != "" {
					pt.id = tc.ID
				}
				if tc.Function.Name != "" {
					pt.name = tc.Function.Name
				}
				if tc.Function.Arguments != "" {
					pt.args.WriteString(tc.Function.Arguments)
				}
			}

			if choice.FinishReason != "" {
				stopReason = mapOpenAIStopReason(choice.FinishReason)
				if choice.FinishReason == "tool_calls" {
					o.emitPendingTools(ch, toolBlocks, toolOrder)
					// Reset accumulator; OpenAI doesn't reuse indices across
					// turns but stay safe.
					toolBlocks = make(map[int]*pendingOpenAITool)
					toolOrder = toolOrder[:0]
				}
			}
		}
	}
}

// emitPendingTools drains accumulated tool calls in their original order.
func (o *OpenAIProvider) emitPendingTools(ch chan<- Event, toolBlocks map[int]*pendingOpenAITool, toolOrder []int) {
	for _, idx := range toolOrder {
		pt, ok := toolBlocks[idx]
		if !ok {
			continue
		}
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
	}
}

// mapOpenAIStopReason translates OpenAI's finish_reason values to the
// canonical {"end_turn","tool_use"} vocabulary the chat loop uses.
func mapOpenAIStopReason(reason string) string {
	switch reason {
	case "tool_calls":
		return "tool_use"
	case "stop", "length", "content_filter":
		return "end_turn"
	default:
		return reason
	}
}
