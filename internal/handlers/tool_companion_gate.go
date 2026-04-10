package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/gallowaysoftware/toqui-backend/internal/ai"
	"github.com/gallowaysoftware/toqui-backend/internal/ai/tools"
)

// CompanionGate wraps a tool (typically create_itinerary_items or
// delete_itinerary_items) and only allows execution when the user's most
// recent message explicitly requests an itinerary modification.
//
// This prevents the Run 5 / Run 8 regression where Gemini interprets
// "recommend a lunch spot" as "add a lunch spot to the itinerary".
//
// The gate uses a fast-tier LLM classifier to determine intent. This
// handles all languages and natural phrasings that substring matching
// would miss. Cost is ~$0.001 per check on the fast model tier.
//
// When the gate blocks a call, it returns a success-shaped JSON response
// (no "error" key) telling the AI the tool is not needed for this query.
// This prevents retry loops.
//
// The gate is fail-closed: on classifier error or timeout, the tool call
// is blocked rather than allowed. This protects the user's itinerary from
// unwanted modifications (Run 19 N-13 regression).
type CompanionGate struct {
	inner          tools.Tool
	provider       ai.Provider
	lastUserMsg    func() string // returns the most recent user message
	toolNameForLog string
}

// NewCompanionGate wraps the given tool with an LLM-based intent gate.
// The provider is used for the fast-tier classification call.
// The lastUserMsg function should return the user's most recent message
// content (typically from the SendMessage request).
func NewCompanionGate(inner tools.Tool, provider ai.Provider, lastUserMsg func() string) *CompanionGate {
	return &CompanionGate{
		inner:          inner,
		provider:       provider,
		lastUserMsg:    lastUserMsg,
		toolNameForLog: inner.Definition().Name,
	}
}

func (g *CompanionGate) Definition() ai.ToolDefinition {
	return g.inner.Definition()
}

func (g *CompanionGate) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	userMsg := ""
	if g.lastUserMsg != nil {
		userMsg = g.lastUserMsg()
	}

	if !g.classifyItineraryIntent(ctx, userMsg) {
		slog.Info("companion gate: blocked tool call on info query",
			"tool", g.toolNameForLog,
			"user_msg_preview", userMsg[:min(len(userMsg), 80)],
		)
		return json.Marshal(map[string]any{
			"status":  "not_needed",
			"message": "The user's message is an informational query, not a request to modify the itinerary. Answer the question directly in your response text instead. Do NOT call this tool again for this message.",
		})
	}

	return g.inner.Execute(ctx, args)
}

// classifyItineraryIntent uses a fast-tier LLM call to determine whether
// the user's message is an explicit request to modify the itinerary
// (add, remove, save items) vs an informational query (recommend, suggest,
// what should I do). This works across all languages.
//
// Returns true = allow the tool call, false = block it.
// On any error (timeout, provider failure), defaults to BLOCK to protect
// the user's itinerary from unwanted modifications (fail-closed).
func (g *CompanionGate) classifyItineraryIntent(ctx context.Context, userMsg string) bool {
	if userMsg == "" {
		return false
	}

	classifyCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	req := &ai.ChatRequest{
		SystemPrompt: `You are a strict binary classifier. Determine if the user EXPLICITLY asks to MODIFY their travel itinerary (add, remove, save, schedule, or delete specific items) or if they want INFORMATION (recommendations, suggestions, questions, opinions, directions, tips, etiquette).

Answer with exactly one word: MODIFY or INFO.

CRITICAL: The user must use EXPLICIT words like "add to my plan/itinerary", "save this", "put this on my schedule", "remove from my itinerary", "book this". If the user asks "what should I do", "recommend something", "what's good around here", or "suggest something" — that is ALWAYS INFO, even if they sound enthusiastic. Asking for a recommendation is NOT the same as asking to modify the itinerary.

When in doubt, answer INFO. It is much worse to modify someone's itinerary without permission than to miss a modification request.

MODIFY examples (user explicitly asks to change their plan):
- "add that to my plan" → MODIFY
- "save this for tomorrow" → MODIFY
- "put the temple visit on day 2" → MODIFY
- "remove the museum from my itinerary" → MODIFY
- "schedule this restaurant for dinner" → MODIFY
- "ajoute ça à mon planning" → MODIFY
- "これを旅程に追加して" → MODIFY

INFO examples (user asks for information, opinions, or recommendations):
- "recommend a good restaurant" → INFO
- "what's a good lunch spot around here?" → INFO
- "what should I do tonight?" → INFO
- "is the tram worth riding?" → INFO
- "how do I get to the museum?" → INFO
- "recommend something fun" → INFO
- "what's the tipping etiquette?" → INFO
- "what are the best things to see?" → INFO
- "where should I eat?" → INFO
- "suggest something for this evening" → INFO
- "what's nearby?" → INFO
- "quelle est la météo?" → INFO`,
		Messages: []ai.Message{
			{Role: "user", Content: userMsg},
		},
		MaxTokens:   4,
		Temperature: 0,
		ModelTier:   ai.ModelTierFast,
	}

	eventCh, err := g.provider.ChatStream(classifyCtx, req)
	if err != nil {
		slog.Debug("companion gate classifier failed, blocking tool call", "error", err)
		return false // fail-closed: block on error to protect itinerary
	}

	var response strings.Builder
	for event := range eventCh {
		if event.Type == ai.EventTextDelta {
			response.WriteString(event.Text)
		}
		if event.Type == ai.EventError {
			slog.Debug("companion gate classifier error, blocking tool call", "error", event.Error)
			return false // fail-closed
		}
	}

	result := strings.TrimSpace(strings.ToUpper(response.String()))
	isModify := strings.HasPrefix(result, "MODIFY")

	slog.Debug("companion gate classifier result",
		"result", result,
		"is_modify", isModify,
		"user_msg_preview", userMsg[:min(len(userMsg), 60)],
	)

	return isModify
}
