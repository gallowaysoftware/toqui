//go:build aitest

package aitest

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gallowaysoftware/toqui-backend/internal/ai"
)

// GenerativeBuilder creates novel test scenarios using an LLM.
type GenerativeBuilder struct {
	provider ai.Provider
}

// NewGenerativeBuilder creates a generative scenario builder.
func NewGenerativeBuilder(provider ai.Provider) *GenerativeBuilder {
	return &GenerativeBuilder{provider: provider}
}

type generatedScenario struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Steps       []generatedStep  `json:"steps"`
}

type generatedStep struct {
	Content      string              `json:"content"`
	Mode         string              `json:"mode"`
	EvalCriteria []generatedCriteria `json:"eval_criteria"`
}

type generatedCriteria struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Generate creates novel test scenarios by asking the LLM.
func (g *GenerativeBuilder) Generate(ctx context.Context, count int) ([]*TestScenario, error) {
	prompt := fmt.Sprintf(`Generate %d novel test scenarios for an AI travel assistant chatbot called Toqui.

Each scenario should test an edge case or unusual situation. Focus on:
- Language switching or cultural misunderstandings
- Contradictory or confusing user requests
- Accessibility needs or dietary restrictions
- Very terse single-word messages vs very long rambling messages
- Users asking about sensitive or challenging destinations
- Last-minute changes (flight cancelled, plans changed)
- Users who are confused or need hand-holding
- Budget extremes (luxury vs backpacker)

Return a JSON array (no markdown fences). Each element:
{
  "name": "kebab-case-name",
  "description": "What this scenario tests",
  "steps": [
    {
      "content": "The user message to send",
      "mode": "selection or planning or companion",
      "eval_criteria": [
        {"name": "criterion_name", "description": "What to evaluate"}
      ]
    }
  ]
}

Rules:
- Each scenario should have 2-4 steps
- First step should usually be "selection" mode
- Steps should form a coherent conversation
- Criteria should test specific qualities (empathy, accuracy, safety awareness, etc.)
- Names should be unique and descriptive
`, count)

	genCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	req := &ai.ChatRequest{
		SystemPrompt: "You generate test scenarios for AI evaluation. Always respond with valid JSON arrays. No markdown fences.",
		Messages:     []ai.Message{{Role: "user", Content: prompt}},
		MaxTokens:    4096,
		Temperature:  0.8,
	}

	eventCh, err := g.provider.ChatStream(genCtx, req)
	if err != nil {
		return nil, fmt.Errorf("generate scenarios: %w", err)
	}

	var response strings.Builder
	for event := range eventCh {
		if event.Type == ai.EventTextDelta {
			response.WriteString(event.Text)
		}
	}

	// Parse response
	raw := strings.TrimSpace(response.String())
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var generated []generatedScenario
	if err := json.Unmarshal([]byte(raw), &generated); err != nil {
		return nil, fmt.Errorf("parse generated scenarios: %w (raw: %s)", err, truncate(raw, 500))
	}

	// Convert to TestScenario structs
	var scenarios []*TestScenario
	for _, gs := range generated {
		scenario := &TestScenario{
			Name:        gs.Name,
			Description: gs.Description,
			Tags:        []string{"generative"},
		}

		for i, step := range gs.Steps {
			ts := TestStep{
				Name: fmt.Sprintf("step-%d", i+1),
				Action: &SendMessageAction{
					Content: step.Content,
					Mode:    step.Mode,
				},
				Assertions: []Assertion{
					AssertResponseNonEmpty(),
					AssertNoErrors(),
				},
				Timeout: 120 * time.Second,
			}
			for _, ec := range step.EvalCriteria {
				ts.EvalCriteria = append(ts.EvalCriteria, EvalCriterion{
					Name:        ec.Name,
					Description: ec.Description,
					Weight:      0.8,
				})
			}
			scenario.Steps = append(scenario.Steps, ts)
		}

		scenarios = append(scenarios, scenario)
	}

	return scenarios, nil
}
