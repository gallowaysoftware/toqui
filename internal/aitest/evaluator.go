//go:build aitest

package aitest

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gallowaysoftware/toqui-backend/internal/ai"
)

// Evaluator uses an LLM as a judge to score AI response quality.
type Evaluator struct {
	provider ai.Provider
}

// NewEvaluator creates an evaluator using the given AI provider.
func NewEvaluator(provider ai.Provider) *Evaluator {
	return &Evaluator{provider: provider}
}

// EvalResult is the judge's assessment of a single criterion.
type EvalResult struct {
	Criterion string
	Score     int    // 1-5
	Reasoning string // LLM's explanation
	Passed    bool   // Score >= 3
}

// Evaluate asks the LLM judge to score a response against a criterion.
func (e *Evaluator) Evaluate(ctx context.Context, result *StepResult, criterion EvalCriterion, conversationContext string) *EvalResult {
	// Build tool calls summary
	var toolsSummary string
	if len(result.ToolCalls) > 0 {
		var parts []string
		for _, tc := range result.ToolCalls {
			parts = append(parts, fmt.Sprintf("%s(%s)", tc.Name, tc.Input))
		}
		toolsSummary = strings.Join(parts, "\n")
	} else {
		toolsSummary = "(none)"
	}

	prompt := fmt.Sprintf(`Evaluate this AI travel assistant response.

CONVERSATION SO FAR:
%s

TOOL CALLS MADE IN THIS STEP:
%s

CRITERION TO EVALUATE: %s
%s

Score 1-5:
1 = Completely fails
2 = Mostly fails, significant issues
3 = Acceptable, meets basic expectations
4 = Good, exceeds expectations
5 = Excellent

Respond with ONLY this JSON (no markdown):
{"score": <1-5>, "reasoning": "<1-2 sentences>", "passed": <true if score >= 3>}`,
		truncate(conversationContext, 3000),
		toolsSummary,
		criterion.Name,
		criterion.Description,
	)

	evalCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req := &ai.ChatRequest{
		SystemPrompt: "You are a strict but fair evaluator of AI travel assistant responses. Score against the given criterion. Respond with JSON only, no markdown fences.",
		Messages:     []ai.Message{{Role: "user", Content: prompt}},
		MaxTokens:    256,
		Temperature:  0.1,
	}

	eventCh, err := e.provider.ChatStream(evalCtx, req)
	if err != nil {
		slog.Warn("aitest: evaluator stream error", "criterion", criterion.Name, "error", err)
		return &EvalResult{
			Criterion: criterion.Name,
			Score:     0,
			Reasoning: fmt.Sprintf("evaluator error: %v", err),
			Passed:    false,
		}
	}

	var response strings.Builder
	for event := range eventCh {
		if event.Type == ai.EventTextDelta {
			response.WriteString(event.Text)
		}
	}

	// Parse JSON response
	raw := strings.TrimSpace(response.String())
	// Strip markdown fences if present
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var parsed struct {
		Score     int    `json:"score"`
		Reasoning string `json:"reasoning"`
		Passed    bool   `json:"passed"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		slog.Warn("aitest: evaluator parse error", "criterion", criterion.Name, "raw", truncate(raw, 200))
		return &EvalResult{
			Criterion: criterion.Name,
			Score:     0,
			Reasoning: fmt.Sprintf("failed to parse eval response: %s", truncate(raw, 200)),
			Passed:    false,
		}
	}

	return &EvalResult{
		Criterion: criterion.Name,
		Score:     parsed.Score,
		Reasoning: parsed.Reasoning,
		Passed:    parsed.Score >= 3,
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
