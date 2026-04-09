package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/gallowaysoftware/toqui-backend/internal/ai"
	"github.com/gallowaysoftware/toqui-backend/internal/ai/tools"
)

// CompanionGate wraps a tool (typically create_itinerary_items or
// delete_itinerary_items) and only allows execution when the user's most
// recent message explicitly requests an itinerary modification.
//
// This prevents the Run 5 / Run 8 regression where Gemini interprets
// "recommend a lunch spot" as "add a lunch spot to the itinerary".
// The gate uses fast substring matching — no LLM call needed.
//
// When the gate blocks a call, it returns a success-shaped JSON response
// (no "error" key) telling the AI the tool is not needed for this query.
// This prevents retry loops.
type CompanionGate struct {
	inner          tools.Tool
	lastUserMsg    func() string // returns the most recent user message
	toolNameForLog string
}

// NewCompanionGate wraps the given tool with an intent gate. The
// lastUserMsg function should return the user's most recent message
// content (typically from the SendMessage request).
func NewCompanionGate(inner tools.Tool, lastUserMsg func() string) *CompanionGate {
	return &CompanionGate{
		inner:          inner,
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

	if !userExplicitlyRequestsItineraryChange(userMsg) {
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

// userExplicitlyRequestsItineraryChange returns true when the user's
// message contains explicit intent to add, save, remove, or modify
// itinerary items. Info queries like "recommend a restaurant" or "what
// should I do tonight" return false.
func userExplicitlyRequestsItineraryChange(msg string) bool {
	if msg == "" {
		return false
	}
	lower := strings.ToLower(msg)

	// Explicit add/save/remove signals. These are phrases a user would
	// say when they want the itinerary modified, not just information.
	for _, phrase := range []string{
		"add to my itinerary",
		"add to my plan",
		"add to the itinerary",
		"add to the plan",
		"add that to my",
		"add this to my",
		"add it to my",
		"save to my itinerary",
		"save to my plan",
		"save this for later",
		"save that for later",
		"put it on my itinerary",
		"put this on my itinerary",
		"put that on my plan",
		"add it for tomorrow",
		"add it for today",
		"add this for tomorrow",
		"schedule this",
		"schedule that",
		"plan this for",
		"plan that for",
		"include this in my",
		"include that in my",
		"remove from my itinerary",
		"remove from my plan",
		"remove from the itinerary",
		"cut from my itinerary",
		"delete from my itinerary",
		"drop from my plan",
		"take off my itinerary",
		"remove the ",
		"cut the ",
		"delete the ",
	} {
		if strings.Contains(lower, phrase) {
			return true
		}
	}

	return false
}
