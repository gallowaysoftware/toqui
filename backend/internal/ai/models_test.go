package ai

import (
	"os"
	"testing"
)

func TestDefaultModelConfigs(t *testing.T) {
	configs := defaultModelConfigs()

	tests := []struct {
		tier       ModelTier
		wantMaxTok int
		wantTemp   float64
	}{
		{
			tier:       ModelTierFast,
			wantMaxTok: 2048,
			wantTemp:   0.7,
		},
		{
			tier:       ModelTierSmart,
			wantMaxTok: 16384,
			wantTemp:   0.7,
		},
		{
			tier:       ModelTierBest,
			wantMaxTok: 32768,
			wantTemp:   0.7,
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.tier), func(t *testing.T) {
			cfg, ok := configs[tt.tier]
			if !ok {
				t.Fatalf("no config for tier %q", tt.tier)
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

func TestConfigForTier(t *testing.T) {
	// Known tier returns correct config.
	cfg := ConfigForTier(ModelTierFast)
	if cfg.MaxTokens == 0 {
		t.Error("ConfigForTier(fast) returned zero MaxTokens")
	}

	// Unknown tier falls back to smart.
	cfg = ConfigForTier(ModelTier("unknown"))
	smartCfg := ConfigForTier(ModelTierSmart)
	if cfg.MaxTokens != smartCfg.MaxTokens {
		t.Errorf("unknown tier MaxTokens = %d, want smart default %d", cfg.MaxTokens, smartCfg.MaxTokens)
	}
}

func TestClaudeModelMapping(t *testing.T) {
	// Default models should be set.
	if claudeModels[ModelTierFast] == "" {
		t.Error("claudeModels[fast] is empty")
	}
	if claudeModels[ModelTierSmart] == "" {
		t.Error("claudeModels[smart] is empty")
	}
	if claudeModels[ModelTierBest] == "" {
		t.Error("claudeModels[best] is empty")
	}
}

func TestGeminiModelMapping(t *testing.T) {
	if geminiModels[ModelTierFast] == "" {
		t.Error("geminiModels[fast] is empty")
	}
	if geminiModels[ModelTierSmart] == "" {
		t.Error("geminiModels[smart] is empty")
	}
	if geminiModels[ModelTierBest] == "" {
		t.Error("geminiModels[best] is empty")
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
