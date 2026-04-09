package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const vertexAIScope = "https://www.googleapis.com/auth/cloud-platform"

// Gemini model defaults per tier. Override via AI_GEMINI_MODEL_FAST, AI_GEMINI_MODEL_SMART, AI_GEMINI_MODEL_BEST.
//
// Using Gemini 2.5 (GA). Gemini 3 Preview models are not yet accessible
// on Vertex AI (404 on all regions as of April 2026). Will upgrade to
// Gemini 3 when it goes GA — the env var overrides make this a config-only
// change. Gemini 2.5 retirement is no earlier than Oct 16, 2026.
var geminiModels = map[ModelTier]string{
	ModelTierFast:  getEnvOrDefault("AI_GEMINI_MODEL_FAST", "gemini-2.5-flash-lite"),
	ModelTierSmart: getEnvOrDefault("AI_GEMINI_MODEL_SMART", "gemini-2.5-flash"),
	ModelTierBest:  getEnvOrDefault("AI_GEMINI_MODEL_BEST", "gemini-2.5-pro"),
}

// GeminiProvider implements the Provider interface using Google Vertex AI.
// Authentication uses Application Default Credentials (ADC), the same
// mechanism used for Secret Manager resolution — no API keys needed.
type GeminiProvider struct {
	projectID   string
	location    string
	tokenSource oauth2.TokenSource
	model       string
	client      *http.Client
}

// NewGeminiProvider creates a Vertex AI Gemini provider.
// It resolves Application Default Credentials at construction time.
// Works with:
//   - gcloud auth application-default login (local dev)
//   - GOOGLE_APPLICATION_CREDENTIALS env var (service account JSON)
//   - GCE/Cloud Run metadata server (production)
func NewGeminiProvider(projectID, location string) (*GeminiProvider, error) {
	ctx := context.Background()
	creds, err := google.FindDefaultCredentials(ctx, vertexAIScope)
	if err != nil {
		return nil, fmt.Errorf("find default credentials for Vertex AI: %w", err)
	}

	if location == "" {
		location = "us-central1"
	}

	return &GeminiProvider{
		projectID:   projectID,
		location:    location,
		tokenSource: creds.TokenSource,
		model:       "gemini-2.5-flash",
		client:      &http.Client{Timeout: 5 * time.Minute},
	}, nil
}

func (g *GeminiProvider) Name() string {
	return "gemini"
}

func (g *GeminiProvider) ChatStream(ctx context.Context, req *ChatRequest) (<-chan Event, error) {
	body := g.buildRequest(req)

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	model := g.resolveModel(req)
	url := fmt.Sprintf(
		"https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/%s:streamGenerateContent?alt=sse",
		g.location, g.projectID, g.location, model,
	)

	cfg := defaultRetryConfig()
	resp, err := doWithRetry(ctx, cfg, "gemini", func() (*http.Response, error) { //nolint:bodyclose // body is closed in the goroutine below; doWithRetry closes on retries
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}

		// Get Bearer token from ADC (auto-refreshes).
		token, err := g.tokenSource.Token()
		if err != nil {
			return nil, fmt.Errorf("get access token: %w", err)
		}

		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+token.AccessToken)

		return g.client.Do(httpReq)
	})
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}

	ch := make(chan Event, 64)
	go func() {
		defer close(ch)
		defer resp.Body.Close()
		g.processStream(ctx, resp.Body, ch)
	}()

	return ch, nil
}

// resolveModel returns the Gemini model identifier for the request's tier.
func (g *GeminiProvider) resolveModel(req *ChatRequest) string {
	tier := req.ModelTier
	if tier == "" {
		return g.model
	}
	if model, ok := geminiModels[tier]; ok {
		slog.Info("gemini model resolved", "tier", tier, "model", model)
		return model
	}
	slog.Warn("unknown tier for gemini, using default", "tier", tier)
	return g.model
}

func (g *GeminiProvider) buildRequest(req *ChatRequest) map[string]any {
	body := map[string]any{}

	// System instruction (separate from contents in Gemini).
	if req.SystemPrompt != "" {
		body["systemInstruction"] = map[string]any{
			"parts": []map[string]any{
				{"text": req.SystemPrompt},
			},
		}
	}

	// Build contents (message history).
	contents := make([]map[string]any, 0, len(req.Messages))
	for _, msg := range req.Messages {
		// Map our roles to Gemini roles: "assistant" → "model", "user" stays "user".
		role := msg.Role
		if role == "assistant" {
			role = "model"
		}

		var parts []map[string]any

		if len(msg.ToolCalls) > 0 {
			// Model message with function calls.
			if msg.Content != "" {
				parts = append(parts, map[string]any{"text": msg.Content})
			}
			for _, tc := range msg.ToolCalls {
				// Gemini expects args as a JSON object, not a string.
				var args any
				if tc.Arguments != "" && tc.Arguments != "{}" {
					if err := json.Unmarshal([]byte(tc.Arguments), &args); err != nil {
						args = map[string]any{}
					}
				} else {
					args = map[string]any{}
				}
				parts = append(parts, map[string]any{
					"functionCall": map[string]any{
						"name": tc.Name,
						"args": args,
					},
				})
			}
		} else if len(msg.ToolResults) > 0 {
			// User message with function responses.
			// Gemini matches by function name (no tool_call_id concept).
			for _, tr := range msg.ToolResults {
				// Try to parse content as JSON object; fall back to wrapping as {"result": content}.
				var response any
				if err := json.Unmarshal([]byte(tr.Content), &response); err != nil {
					response = map[string]any{"result": tr.Content}
				}
				parts = append(parts, map[string]any{
					"functionResponse": map[string]any{
						"name":     tr.Name,
						"response": response,
					},
				})
			}
		} else if len(msg.ContentBlocks) > 0 {
			// Multimodal content (text + images).
			for _, block := range msg.ContentBlocks {
				switch block.Type {
				case "text":
					parts = append(parts, map[string]any{"text": block.Text})
				case "image":
					if block.Source != nil {
						parts = append(parts, map[string]any{
							"inlineData": map[string]any{
								"mimeType": block.Source.MediaType,
								"data":     block.Source.Data,
							},
						})
					}
				}
			}
		} else {
			// Plain text message.
			parts = append(parts, map[string]any{"text": msg.Content})
		}

		contents = append(contents, map[string]any{
			"role":  role,
			"parts": parts,
		})
	}
	body["contents"] = contents

	// Tools (function declarations + optional grounding tools).
	if len(req.Tools) > 0 {
		funcDecls := make([]map[string]any, 0, len(req.Tools))
		for _, t := range req.Tools {
			funcDecl := map[string]any{
				"name":        t.Name,
				"description": t.Description,
			}
			// Parameters is already JSON Schema — parse to object for Gemini.
			var params any
			if err := json.Unmarshal(t.Parameters, &params); err == nil {
				funcDecl["parameters"] = params
			}
			funcDecls = append(funcDecls, funcDecl)
		}

		// NOTE: Google Search and Maps grounding tools cannot be mixed
		// with functionDeclarations in the same tools array on Gemini 2.5
		// ("Multiple tools are supported only when they are all search
		// tools"). Grounding will be enabled when we upgrade to Gemini 3
		// or via a separate non-function-calling request path.
		body["tools"] = []map[string]any{
			{"functionDeclarations": funcDecls},
		}
		body["toolConfig"] = map[string]any{
			"functionCallingConfig": map[string]any{
				"mode": "AUTO",
			},
		}
	}

	// Generation config.
	genConfig := map[string]any{}
	if req.MaxTokens > 0 {
		genConfig["maxOutputTokens"] = req.MaxTokens
	} else {
		genConfig["maxOutputTokens"] = 4096
	}
	if req.Temperature > 0 {
		genConfig["temperature"] = req.Temperature
	}
	body["generationConfig"] = genConfig

	return body
}

func (g *GeminiProvider) processStream(ctx context.Context, body io.Reader, ch chan<- Event) {
	reader := NewSSEReader(body)
	var usage *Usage
	var hadFunctionCall bool

	for {
		data, done, err := reader.Next(ctx)
		if err != nil {
			ch <- Event{Type: EventError, Error: err}
			return
		}
		if done {
			// Stream ended. Determine stop reason from what we saw.
			stopReason := "end_turn"
			if hadFunctionCall {
				stopReason = "tool_use"
			}
			ch <- Event{Type: EventDone, StopReason: stopReason, Usage: usage}
			return
		}

		var resp struct {
			Candidates []struct {
				Content struct {
					Role  string `json:"role"`
					Parts []struct {
						Text         string `json:"text"`
						FunctionCall *struct {
							Name string         `json:"name"`
							Args map[string]any `json:"args"`
						} `json:"functionCall"`
					} `json:"parts"`
				} `json:"content"`
				FinishReason string `json:"finishReason"`
			} `json:"candidates"`
			UsageMetadata *struct {
				PromptTokenCount     int `json:"promptTokenCount"`
				CandidatesTokenCount int `json:"candidatesTokenCount"`
				TotalTokenCount      int `json:"totalTokenCount"`
			} `json:"usageMetadata"`
		}
		if err := json.Unmarshal([]byte(data), &resp); err != nil {
			continue
		}

		// Parse usage metadata (appears in chunks, last one has final counts).
		if resp.UsageMetadata != nil {
			usage = &Usage{
				InputTokens:  resp.UsageMetadata.PromptTokenCount,
				OutputTokens: resp.UsageMetadata.CandidatesTokenCount,
			}
		}

		if len(resp.Candidates) == 0 {
			continue
		}

		candidate := resp.Candidates[0]

		// Process parts: text deltas and function calls.
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				ch <- Event{Type: EventTextDelta, Text: part.Text}
			}

			if part.FunctionCall != nil {
				hadFunctionCall = true

				// Marshal args object to JSON string for our unified ToolCall format.
				argsJSON, err := json.Marshal(part.FunctionCall.Args)
				if err != nil {
					argsJSON = []byte("{}")
				}

				// Gemini has no tool call IDs — generate a synthetic one.
				ch <- Event{
					Type: EventToolCall,
					Tool: &ToolCall{
						ID:        "gemini-" + uuid.New().String(),
						Name:      part.FunctionCall.Name,
						Arguments: string(argsJSON),
					},
				}
			}
		}

		// Check finish reason. Gemini uses "STOP" for both normal completion
		// and tool calls — we distinguish by whether a functionCall was present.
		if candidate.FinishReason != "" && candidate.FinishReason != "FINISH_REASON_UNSPECIFIED" {
			slog.Info("gemini stream finished",
				"finish_reason", candidate.FinishReason,
				"had_function_call", hadFunctionCall,
			)

			// MAX_TOKENS means the model was truncated mid-generation. Treat this
			// as an error so the caller can surface a retry prompt rather than
			// silently returning an incomplete (possibly empty) response.
			if candidate.FinishReason == "MAX_TOKENS" {
				ch <- Event{
					Type:  EventError,
					Error: fmt.Errorf("response truncated (MAX_TOKENS) — try a more specific request"),
				}
				return
			}

			stopReason := "end_turn"
			if hadFunctionCall {
				stopReason = "tool_use"
			}
			ch <- Event{Type: EventDone, StopReason: stopReason, Usage: usage}
			return
		}
	}
}
