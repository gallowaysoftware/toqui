package ai

import (
	"log/slog"
	"os"
)

// ModelTier represents the quality/cost tier for model selection.
// Each provider maps tiers to its own model identifiers internally.
type ModelTier string

const (
	// ModelTierFast uses quick, low-cost models for simple queries.
	ModelTierFast ModelTier = "fast"

	// ModelTierSmart uses balanced models for most tasks including tool calling.
	ModelTierSmart ModelTier = "smart"

	// ModelTierBest uses the most capable models for complex planning.
	ModelTierBest ModelTier = "best"
)

// ModelConfig holds the shared parameters for a model tier.
// Provider-specific model identifiers are managed internally by each provider.
type ModelConfig struct {
	MaxTokens   int
	Temperature float64
}

// defaultModelConfigs returns the default configuration for each model tier.
//
// Token budgets are sized to let the smart/best tiers comfortably emit a
// narrative plus a large create_itinerary_items tool call (10-20 items with
// descriptions) in a single turn. Gemini tends to produce more preamble text
// than Claude before calling tools, so 8192 was getting truncated before the
// tool call fired (see toqui-backend#151).
func defaultModelConfigs() map[ModelTier]ModelConfig {
	configs := map[ModelTier]ModelConfig{
		ModelTierFast: {
			MaxTokens:   2048,
			Temperature: 0.7,
		},
		ModelTierSmart: {
			MaxTokens:   16384,
			Temperature: 0.7,
		},
		ModelTierBest: {
			MaxTokens:   32768,
			Temperature: 0.7,
		},
	}

	for tier, cfg := range configs {
		slog.Debug("model config loaded", "tier", tier, "max_tokens", cfg.MaxTokens)
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

// getEnvOrDefault returns the environment variable value or a fallback.
// Exported for use by provider model resolution (claude.go, gemini.go).
func getEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
