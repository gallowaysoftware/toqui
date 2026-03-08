package ai

import (
	"encoding/json"
	"testing"
)

func TestBuildRequestPromptCaching_SystemMessage(t *testing.T) {
	provider := NewClaudeProvider("test-key")
	req := &ChatRequest{
		SystemPrompt: "You are a helpful travel assistant.",
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
		MaxTokens: 1024,
	}

	body := provider.buildRequest(req)

	// System should be a slice of content blocks, not a plain string.
	systemRaw, ok := body["system"]
	if !ok {
		t.Fatal("expected 'system' key in request body")
	}

	systemBlocks, ok := systemRaw.([]map[string]any)
	if !ok {
		t.Fatalf("expected system to be []map[string]any, got %T", systemRaw)
	}
	if len(systemBlocks) != 1 {
		t.Fatalf("expected 1 system block, got %d", len(systemBlocks))
	}

	block := systemBlocks[0]
	if block["type"] != "text" {
		t.Errorf("system block type = %q, want %q", block["type"], "text")
	}
	if block["text"] != "You are a helpful travel assistant." {
		t.Errorf("system block text = %q, want system prompt", block["text"])
	}

	cacheControl, ok := block["cache_control"]
	if !ok {
		t.Fatal("system block missing cache_control")
	}
	cc, ok := cacheControl.(map[string]string)
	if !ok {
		t.Fatalf("cache_control has unexpected type %T", cacheControl)
	}
	if cc["type"] != "ephemeral" {
		t.Errorf("cache_control type = %q, want %q", cc["type"], "ephemeral")
	}
}

func TestBuildRequestPromptCaching_NoSystemPrompt(t *testing.T) {
	provider := NewClaudeProvider("test-key")
	req := &ChatRequest{
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
		MaxTokens: 1024,
	}

	body := provider.buildRequest(req)

	if _, ok := body["system"]; ok {
		t.Error("expected no 'system' key when SystemPrompt is empty")
	}
}

func TestBuildRequestPromptCaching_ToolsCacheControl(t *testing.T) {
	provider := NewClaudeProvider("test-key")
	req := &ChatRequest{
		SystemPrompt: "You are helpful.",
		Messages: []Message{
			{Role: "user", Content: "Find a restaurant"},
		},
		Tools: []ToolDefinition{
			{
				Name:        "search",
				Description: "Search the web",
				Parameters:  json.RawMessage(`{"type":"object"}`),
			},
			{
				Name:        "places",
				Description: "Find places nearby",
				Parameters:  json.RawMessage(`{"type":"object"}`),
			},
		},
		MaxTokens: 1024,
	}

	body := provider.buildRequest(req)

	toolsRaw, ok := body["tools"]
	if !ok {
		t.Fatal("expected 'tools' key in request body")
	}

	tools, ok := toolsRaw.([]map[string]any)
	if !ok {
		t.Fatalf("tools has unexpected type %T", toolsRaw)
	}
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}

	// First tool should NOT have cache_control.
	if _, ok := tools[0]["cache_control"]; ok {
		t.Error("first tool should not have cache_control")
	}

	// Last tool SHOULD have cache_control.
	cc, ok := tools[1]["cache_control"]
	if !ok {
		t.Fatal("last tool missing cache_control")
	}
	ccMap, ok := cc.(map[string]string)
	if !ok {
		t.Fatalf("tool cache_control has unexpected type %T", cc)
	}
	if ccMap["type"] != "ephemeral" {
		t.Errorf("tool cache_control type = %q, want %q", ccMap["type"], "ephemeral")
	}
}

func TestBuildRequestPromptCaching_SingleToolHasCacheControl(t *testing.T) {
	provider := NewClaudeProvider("test-key")
	req := &ChatRequest{
		SystemPrompt: "You are helpful.",
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
		Tools: []ToolDefinition{
			{
				Name:        "search",
				Description: "Search the web",
				Parameters:  json.RawMessage(`{"type":"object"}`),
			},
		},
		MaxTokens: 1024,
	}

	body := provider.buildRequest(req)

	tools := body["tools"].([]map[string]any)
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}

	if _, ok := tools[0]["cache_control"]; !ok {
		t.Error("single tool should have cache_control (it's both first and last)")
	}
}

func TestResolveModel_DefaultFallback(t *testing.T) {
	provider := NewClaudeProvider("test-key")

	// No tier set — should use provider default (Sonnet).
	req := &ChatRequest{}
	model := provider.resolveModel(req)
	if model != "claude-sonnet-4-20250514" {
		t.Errorf("resolveModel with no tier = %q, want %q", model, "claude-sonnet-4-20250514")
	}
}

func TestResolveModel_FastTier(t *testing.T) {
	// Clear env to ensure defaults are used.
	t.Setenv("AI_MODEL_FAST", "")

	// Reinitialize configs to pick up the cleared env.

	provider := NewClaudeProvider("test-key")
	req := &ChatRequest{ModelTier: ModelTierFast}

	model := provider.resolveModel(req)
	if model != claudeModels[ModelTierFast] {
		t.Errorf("resolveModel with fast tier = %q, want %q", model, claudeModels[ModelTierFast])
	}
}

func TestResolveModel_SmartTier(t *testing.T) {
	t.Setenv("AI_MODEL_SMART", "")

	provider := NewClaudeProvider("test-key")
	req := &ChatRequest{ModelTier: ModelTierSmart}

	model := provider.resolveModel(req)
	if model != claudeModels[ModelTierSmart] {
		t.Errorf("resolveModel with smart tier = %q, want %q", model, claudeModels[ModelTierSmart])
	}
}

func TestResolveModel_BestTier(t *testing.T) {
	t.Setenv("AI_MODEL_BEST", "")

	provider := NewClaudeProvider("test-key")
	req := &ChatRequest{ModelTier: ModelTierBest}

	model := provider.resolveModel(req)
	if model != claudeModels[ModelTierBest] {
		t.Errorf("resolveModel with best tier = %q, want %q", model, claudeModels[ModelTierBest])
	}
}

func TestResolveModel_UnknownTierFallsBackToSmart(t *testing.T) {
	t.Setenv("AI_MODEL_SMART", "")

	provider := NewClaudeProvider("test-key")
	req := &ChatRequest{ModelTier: ModelTier("ultra")}

	model := provider.resolveModel(req)
	if model != claudeModels[ModelTierSmart] {
		t.Errorf("resolveModel with unknown tier = %q, want smart default %q", model, claudeModels[ModelTierSmart])
	}
}

func TestBuildRequest_ModelTierSelectsCorrectModel(t *testing.T) {
	t.Setenv("AI_MODEL_FAST", "")
	t.Setenv("AI_MODEL_SMART", "")

	provider := NewClaudeProvider("test-key")

	tests := []struct {
		name      string
		tier      ModelTier
		wantModel string
	}{
		{"fast tier uses Haiku", ModelTierFast, claudeModels[ModelTierFast]},
		{"smart tier uses Sonnet", ModelTierSmart, claudeModels[ModelTierSmart]},
		{"no tier uses provider default", "", "claude-sonnet-4-20250514"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &ChatRequest{
				Messages:  []Message{{Role: "user", Content: "hi"}},
				MaxTokens: 1024,
				ModelTier: tt.tier,
			}
			body := provider.buildRequest(req)
			if body["model"] != tt.wantModel {
				t.Errorf("model = %q, want %q", body["model"], tt.wantModel)
			}
		})
	}
}

func TestBuildRequest_StreamAlwaysTrue(t *testing.T) {
	provider := NewClaudeProvider("test-key")
	req := &ChatRequest{
		Messages:  []Message{{Role: "user", Content: "hi"}},
		MaxTokens: 1024,
	}
	body := provider.buildRequest(req)
	if body["stream"] != true {
		t.Error("stream should always be true")
	}
}

func TestBuildRequest_DefaultMaxTokens(t *testing.T) {
	provider := NewClaudeProvider("test-key")
	// MaxTokens = 0 should be set to 4096 as default.
	req := &ChatRequest{
		Messages: []Message{{Role: "user", Content: "hi"}},
	}
	body := provider.buildRequest(req)
	if body["max_tokens"] != 4096 {
		t.Errorf("default max_tokens = %v, want 4096", body["max_tokens"])
	}
}
