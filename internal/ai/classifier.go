package ai

import (
	"log/slog"
	"unicode/utf8"
)

// ClassifyRequest determines the appropriate ModelTier for a ChatRequest using
// deterministic heuristics. No LLM call is made — the classification is based on
// message length, tool availability, and chat mode.
//
// When in doubt, the classifier is conservative and returns ModelTierSmart.
//
// If req.ModelTier is already set, it is returned as-is (explicit override).
func ClassifyRequest(req *ChatRequest) ModelTier {
	// Explicit override — caller forced a tier.
	if req.ModelTier != "" {
		slog.Debug("model tier override", "tier", req.ModelTier)
		return req.ModelTier
	}

	tier := classifyByHeuristics(req)
	slog.Debug("model tier classified", "tier", tier, "mode", req.Mode, "msg_len", lastUserMessageLength(req), "has_tools", len(req.Tools) > 0)
	return tier
}

func classifyByHeuristics(req *ChatRequest) ModelTier {
	hasTools := len(req.Tools) > 0
	msgLen := lastUserMessageLength(req)

	// Mode-based classification.
	switch req.Mode {
	case "selection":
		// Selection mode with tools needs reliable function calling.
		if hasTools {
			return ModelTierSmart
		}
		// Simple selection chat (e.g., "hi", "what trips do I have?").
		return ModelTierFast

	case "companion":
		// Companion mode: quick local questions are fast, longer queries are smart.
		if msgLen > 0 && msgLen < 100 && !hasTools {
			return ModelTierFast
		}
		if req.PriorityModel {
			return ModelTierBest
		}
		return ModelTierSmart

	case "planning":
		// Planning always needs at least smart for reliable tool calling
		// and quality responses. Voyager subscribers get the best model.
		if req.PriorityModel {
			return ModelTierBest
		}
		return ModelTierSmart
	}

	// Fallback: use tools and message length as signals.

	// Tools require reliable function calling — at least smart.
	if hasTools {
		return ModelTierSmart
	}

	// Short messages without tools can use fast.
	if msgLen > 0 && msgLen < 50 {
		return ModelTierFast
	}

	// Default: smart is the safe choice.
	return ModelTierSmart
}

// lastUserMessageLength returns the character count of the last user message
// in the request, or 0 if there are no user messages.
func lastUserMessageLength(req *ChatRequest) int {
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			return utf8.RuneCountInString(req.Messages[i].Content)
		}
	}
	return 0
}
