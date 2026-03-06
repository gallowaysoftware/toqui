package ai

import (
	"log/slog"
	"os"
)

// ModelTier represents the quality/cost tier for model selection.
type ModelTier string

const (
	// ModelTierFast uses quick, low-cost models for simple queries.
	// Claude: Haiku, OpenAI: GPT-4o-mini
	ModelTierFast ModelTier = "fast"

	// ModelTierSmart uses balanced models for most tasks including tool calling.
	// Claude: Sonnet, OpenAI: GPT-4o
	ModelTierSmart ModelTier = "smart"

	// ModelTierBest uses the most capable models for complex planning.
	// Claude: Opus (or Sonnet as fallback), OpenAI: GPT-4o
	ModelTierBest ModelTier = "best"
)

// ModelConfig holds the provider-specific model identifiers and parameters for a tier.
type ModelConfig struct {
	ClaudeModel string
	OpenAIModel string
	MaxTokens   int
	Temperature float64
}

// Default model identifiers per tier.
const (
	defaultClaudeFast  = "claude-haiku-3-5-20241022"
	defaultClaudeSmart = "claude-sonnet-4-20250514"
	defaultClaudeBest  = "claude-sonnet-4-20250514"

	defaultOpenAIFast  = "gpt-4o-mini"
	defaultOpenAISmart = "gpt-4o"
	defaultOpenAIBest  = "gpt-4o"
)

// defaultModelConfigs returns the default configuration for each model tier.
// Claude models are configurable via AI_MODEL_FAST, AI_MODEL_SMART, AI_MODEL_BEST
// environment variables. OpenAI models are configurable via AI_OPENAI_MODEL_FAST,
// AI_OPENAI_MODEL_SMART, AI_OPENAI_MODEL_BEST.
func defaultModelConfigs() map[ModelTier]ModelConfig {
	configs := map[ModelTier]ModelConfig{
		ModelTierFast: {
			ClaudeModel: getEnvOrDefault("AI_MODEL_FAST", defaultClaudeFast),
			OpenAIModel: getEnvOrDefault("AI_OPENAI_MODEL_FAST", defaultOpenAIFast),
			MaxTokens:   2048,
			Temperature: 0.7,
		},
		ModelTierSmart: {
			ClaudeModel: getEnvOrDefault("AI_MODEL_SMART", defaultClaudeSmart),
			OpenAIModel: getEnvOrDefault("AI_OPENAI_MODEL_SMART", defaultOpenAISmart),
			MaxTokens:   4096,
			Temperature: 0.7,
		},
		ModelTierBest: {
			ClaudeModel: getEnvOrDefault("AI_MODEL_BEST", defaultClaudeBest),
			OpenAIModel: getEnvOrDefault("AI_OPENAI_MODEL_BEST", defaultOpenAIBest),
			MaxTokens:   8192,
			Temperature: 0.7,
		},
	}

	// Log any overrides for observability.
	for tier, cfg := range configs {
		slog.Debug("model config loaded", "tier", tier, "claude_model", cfg.ClaudeModel, "openai_model", cfg.OpenAIModel)
	}

	return configs
}

// ModelConfigs is the global model configuration, initialized once at package load.
var ModelConfigs = defaultModelConfigs()

// ConfigForTier returns the ModelConfig for the given tier, falling back to smart
// if the tier is unrecognized.
func ConfigForTier(tier ModelTier) ModelConfig {
	if cfg, ok := ModelConfigs[tier]; ok {
		return cfg
	}
	slog.Warn("unknown model tier, falling back to smart", "tier", tier)
	return ModelConfigs[ModelTierSmart]
}

func getEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
