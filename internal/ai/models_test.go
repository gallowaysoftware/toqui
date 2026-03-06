package ai

import (
	"os"
	"testing"
)

func TestDefaultModelConfigs(t *testing.T) {
	// Ensure no env overrides are set for this test.
	for _, key := range []string{
		"AI_MODEL_FAST", "AI_MODEL_SMART", "AI_MODEL_BEST",
		"AI_OPENAI_MODEL_FAST", "AI_OPENAI_MODEL_SMART", "AI_OPENAI_MODEL_BEST",
	} {
		t.Setenv(key, "")
	}

	configs := defaultModelConfigs()

	tests := []struct {
		tier        ModelTier
		wantClaude  string
		wantOpenAI  string
		wantMaxTok  int
		wantTemp    float64
	}{
		{
			tier:       ModelTierFast,
			wantClaude: defaultClaudeFast,
			wantOpenAI: defaultOpenAIFast,
			wantMaxTok: 2048,
			wantTemp:   0.7,
		},
		{
			tier:       ModelTierSmart,
			wantClaude: defaultClaudeSmart,
			wantOpenAI: defaultOpenAISmart,
			wantMaxTok: 4096,
			wantTemp:   0.7,
		},
		{
			tier:       ModelTierBest,
			wantClaude: defaultClaudeBest,
			wantOpenAI: defaultOpenAIBest,
			wantMaxTok: 8192,
			wantTemp:   0.7,
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.tier), func(t *testing.T) {
			cfg, ok := configs[tt.tier]
			if !ok {
				t.Fatalf("no config for tier %q", tt.tier)
			}
			if cfg.ClaudeModel != tt.wantClaude {
				t.Errorf("ClaudeModel = %q, want %q", cfg.ClaudeModel, tt.wantClaude)
			}
			if cfg.OpenAIModel != tt.wantOpenAI {
				t.Errorf("OpenAIModel = %q, want %q", cfg.OpenAIModel, tt.wantOpenAI)
			}
			if cfg.MaxTokens != tt.wantMaxTok {
				t.Errorf("MaxTokens = %d, want %d", cfg.MaxTokens, tt.wantMaxTok)
			}
			if cfg.Temperature != tt.wantTemp {
				t.Errorf("Temperature = %f, want %f", cfg.Temperature, tt.wantTemp)
			}
		})
	}
}

func TestModelConfigEnvOverride(t *testing.T) {
	t.Setenv("AI_MODEL_FAST", "claude-3-custom-fast")
	t.Setenv("AI_MODEL_SMART", "claude-3-custom-smart")
	t.Setenv("AI_MODEL_BEST", "claude-3-custom-best")
	t.Setenv("AI_OPENAI_MODEL_FAST", "gpt-custom-fast")
	t.Setenv("AI_OPENAI_MODEL_SMART", "gpt-custom-smart")
	t.Setenv("AI_OPENAI_MODEL_BEST", "gpt-custom-best")

	configs := defaultModelConfigs()

	if configs[ModelTierFast].ClaudeModel != "claude-3-custom-fast" {
		t.Errorf("fast Claude = %q, want %q", configs[ModelTierFast].ClaudeModel, "claude-3-custom-fast")
	}
	if configs[ModelTierSmart].ClaudeModel != "claude-3-custom-smart" {
		t.Errorf("smart Claude = %q, want %q", configs[ModelTierSmart].ClaudeModel, "claude-3-custom-smart")
	}
	if configs[ModelTierBest].ClaudeModel != "claude-3-custom-best" {
		t.Errorf("best Claude = %q, want %q", configs[ModelTierBest].ClaudeModel, "claude-3-custom-best")
	}
	if configs[ModelTierFast].OpenAIModel != "gpt-custom-fast" {
		t.Errorf("fast OpenAI = %q, want %q", configs[ModelTierFast].OpenAIModel, "gpt-custom-fast")
	}
	if configs[ModelTierSmart].OpenAIModel != "gpt-custom-smart" {
		t.Errorf("smart OpenAI = %q, want %q", configs[ModelTierSmart].OpenAIModel, "gpt-custom-smart")
	}
	if configs[ModelTierBest].OpenAIModel != "gpt-custom-best" {
		t.Errorf("best OpenAI = %q, want %q", configs[ModelTierBest].OpenAIModel, "gpt-custom-best")
	}
}

func TestModelConfigPartialEnvOverride(t *testing.T) {
	// Only override fast Claude model — everything else should be default.
	t.Setenv("AI_MODEL_FAST", "claude-3-haiku-custom")
	// Explicitly clear others.
	for _, key := range []string{
		"AI_MODEL_SMART", "AI_MODEL_BEST",
		"AI_OPENAI_MODEL_FAST", "AI_OPENAI_MODEL_SMART", "AI_OPENAI_MODEL_BEST",
	} {
		t.Setenv(key, "")
	}

	configs := defaultModelConfigs()

	if configs[ModelTierFast].ClaudeModel != "claude-3-haiku-custom" {
		t.Errorf("fast Claude = %q, want %q", configs[ModelTierFast].ClaudeModel, "claude-3-haiku-custom")
	}
	// Smart should still be default.
	if configs[ModelTierSmart].ClaudeModel != defaultClaudeSmart {
		t.Errorf("smart Claude = %q, want default %q", configs[ModelTierSmart].ClaudeModel, defaultClaudeSmart)
	}
	// OpenAI fast should still be default.
	if configs[ModelTierFast].OpenAIModel != defaultOpenAIFast {
		t.Errorf("fast OpenAI = %q, want default %q", configs[ModelTierFast].OpenAIModel, defaultOpenAIFast)
	}
}

func TestConfigForTier(t *testing.T) {
	// Known tier returns correct config.
	cfg := ConfigForTier(ModelTierFast)
	if cfg.ClaudeModel == "" {
		t.Error("ConfigForTier(fast) returned empty ClaudeModel")
	}

	// Unknown tier falls back to smart.
	cfg = ConfigForTier(ModelTier("unknown"))
	smartCfg := ConfigForTier(ModelTierSmart)
	if cfg.ClaudeModel != smartCfg.ClaudeModel {
		t.Errorf("unknown tier ClaudeModel = %q, want smart default %q", cfg.ClaudeModel, smartCfg.ClaudeModel)
	}
}

func TestGetEnvOrDefault(t *testing.T) {
	// Set env var.
	os.Setenv("TEST_AI_MODEL_KEY", "custom-value")
	defer os.Unsetenv("TEST_AI_MODEL_KEY")

	if got := getEnvOrDefault("TEST_AI_MODEL_KEY", "fallback"); got != "custom-value" {
		t.Errorf("getEnvOrDefault with set env = %q, want %q", got, "custom-value")
	}

	// Unset env var — should return fallback.
	if got := getEnvOrDefault("TEST_AI_MODEL_KEY_MISSING", "fallback"); got != "fallback" {
		t.Errorf("getEnvOrDefault with missing env = %q, want %q", got, "fallback")
	}
}
