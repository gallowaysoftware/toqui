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
// Gemini 3 Preview models via the Developer API (generativelanguage.googleapis.com).
// Falls back to Gemini 2.5 GA on Vertex AI when no API key is configured.
var geminiModels = map[ModelTier]string{
	ModelTierFast:  getEnvOrDefault("AI_GEMINI_MODEL_FAST", "gemini-3.1-flash-lite-preview"),
	ModelTierSmart: getEnvOrDefault("AI_GEMINI_MODEL_SMART", "gemini-3-flash-preview"),
	ModelTierBest:  getEnvOrDefault("AI_GEMINI_MODEL_BEST", "gemini-3.1-pro-preview"),
}

// GeminiProvider implements the Provider interface using either the Gemini
// Developer API (preferred, supports Gemini 3) or Vertex AI (fallback,
// Gemini 2.5 only).
//
// Developer API: generativelanguage.googleapis.com — uses API key
// Vertex AI:     {region}-aiplatform.googleapis.com — uses ADC/OAuth
type GeminiProvider struct {
	// Developer API fields (preferred)
	apiKey string

	// Vertex AI fields (fallback)
	projectID   string
	location    string
	tokenSource oauth2.TokenSource

	// Shared
	model     string
	client    *http.Client
	useDevAPI bool
}

// NewGeminiProvider creates a Gemini provider. It prefers the Developer API
// (when apiKey is set) for Gemini 3 access. Falls back to Vertex AI when
// only projectID is available.
func NewGeminiProvider(apiKey, projectID, location string) (*GeminiProvider, error) {
	p := &GeminiProvider{
		model:  geminiModels[ModelTierSmart],
		client: &http.Client{Timeout: 5 * time.Minute},
	}

	if apiKey != "" {
		// Developer API — supports Gemini 3
		p.apiKey = apiKey
		p.useDevAPI = true
		slog.Info("gemini provider: using Developer API (Gemini 3)",
			"model", p.model,
		)
		return p, nil
	}

	if projectID != "" {
		// Vertex AI fallback — Gemini 2.5 only
		ctx := context.Background()
		creds, err := google.FindDefaultCredentials(ctx, vertexAIScope)
		if err != nil {
			return nil, fmt.Errorf("find default credentials for Vertex AI: %w", err)
		}
		if location == "" {
			location = "us-central1"
		}
		p.projectID = projectID
		p.location = location
		p.tokenSource = creds.TokenSource
		p.useDevAPI = false
		// Override to 2.5 models for Vertex AI
		p.model = "gemini-2.5-flash"
		slog.Info("gemini provider: using Vertex AI (Gemini 2.5 fallback)",
			"project", projectID,
			"location", location,
			"model", p.model,
		)
		return p, nil
	}

	return nil, fmt.Errorf("gemini provider requires either GEMINI_API_KEY or VERTEX_AI_PROJECT_ID")
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
	url := g.buildURL(model)

	cfg := defaultRetryConfig()
	resp, err := doWithRetry(ctx, cfg, "gemini", func() (*http.Response, error) { //nolint:bodyclose // body is closed in the goroutine below; doWithRetry closes on retries
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")

		if !g.useDevAPI {
			// Vertex AI: Bearer token from ADC
			token, err := g.tokenSource.Token()
			if err != nil {
				return nil, fmt.Errorf("get access token: %w", err)
			}
			httpReq.Header.Set("Authorization", "Bearer "+token.AccessToken)
		}
		// Developer API: key is already in the URL query string

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

// buildURL constructs the streaming endpoint URL for the configured backend.
func (g *GeminiProvider) buildURL(model string) string {
	if g.useDevAPI {
		return fmt.Sprintf(
			"https://generativelanguage.googleapis.com/v1beta/models/%s:streamGenerateContent?alt=sse&key=%s",
			model, g.apiKey,
		)
	}
	return fmt.Sprintf(
		"https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/%s:streamGenerateContent?alt=sse",
		g.location, g.projectID, g.location, model,
	)
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
				fcPart := map[string]any{
					"functionCall": map[string]any{
						"name": tc.Name,
						"args": args,
					},
				}
				// Gemini 3 thought signatures: if the tool call has a thought
				// signature from a previous response, include it so the model
				// can maintain reasoning continuity across turns.
				if tc.ThoughtSignature != "" {
					fcPart["thoughtSignature"] = tc.ThoughtSignature
				}
				parts = append(parts, fcPart)
			}
		} else if len(msg.ToolResults) > 0 {
			// User message with function responses.
			for _, tr := range msg.ToolResults {
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

	// Tools (function declarations).
	if len(req.Tools) > 0 {
		funcDecls := make([]map[string]any, 0, len(req.Tools))
		for _, t := range req.Tools {
			funcDecl := map[string]any{
				"name":        t.Name,
				"description": t.Description,
			}
			var params any
			if err := json.Unmarshal(t.Parameters, &params); err == nil {
				funcDecl["parameters"] = params
			}
			funcDecls = append(funcDecls, funcDecl)
		}

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
							ID   string         `json:"id"`
						} `json:"functionCall"`
						// Gemini 3 thought signatures — must be captured and
						// circulated back in follow-up requests for reasoning
						// continuity across tool-call turns.
						ThoughtSignature string `json:"thoughtSignature"`
					} `json:"parts"`
				} `json:"content"`
				FinishReason string `json:"finishReason"`
			} `json:"candidates"`
			UsageMetadata *struct {
				PromptTokenCount     int `json:"promptTokenCount"`
				CandidatesTokenCount int `json:"candidatesTokenCount"`
				TotalTokenCount      int `json:"totalTokenCount"`
				ThoughtsTokenCount   int `json:"thoughtsTokenCount"`
			} `json:"usageMetadata"`
		}
		if err := json.Unmarshal([]byte(data), &resp); err != nil {
			continue
		}

		// Parse usage metadata (last chunk has final counts).
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

		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				ch <- Event{Type: EventTextDelta, Text: part.Text}
			}

			if part.FunctionCall != nil {
				hadFunctionCall = true

				argsJSON, err := json.Marshal(part.FunctionCall.Args)
				if err != nil {
					argsJSON = []byte("{}")
				}

				// Use Gemini's native ID if provided (Gemini 3), otherwise synthetic
				callID := part.FunctionCall.ID
				if callID == "" {
					callID = "gemini-" + uuid.New().String()
				}

				ch <- Event{
					Type: EventToolCall,
					Tool: &ToolCall{
						ID:               callID,
						Name:             part.FunctionCall.Name,
						Arguments:        string(argsJSON),
						ThoughtSignature: part.ThoughtSignature,
					},
				}
			}
		}

		if candidate.FinishReason != "" && candidate.FinishReason != "FINISH_REASON_UNSPECIFIED" {
			slog.Info("gemini stream finished",
				"finish_reason", candidate.FinishReason,
				"had_function_call", hadFunctionCall,
			)

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
