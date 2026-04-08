package handlers

import (
	"context"
	"encoding/json"

	"github.com/gallowaysoftware/toqui-backend/internal/ai"
)

// CompanionItineraryStub is a no-op implementation of the create_itinerary_items
// tool that is injected in companion mode. It has three jobs:
//
//  1. Prevent the AI from hallucinating a nonexistent tool name. In Run 4 N-10
//     the real tool was not registered in companion mode; the AI still tried
//     to call it and got a generic "unknown tool" error, then apologised and
//     retried in a loop.
//
//  2. Prevent the real tool from being called proactively on informational
//     queries like "what do I do first?". In Run 5 N-01 and N-10 the PR #197
//     fix to register the real tool in companion mode caused the AI to
//     silently add items to the itinerary on pure info queries, which is
//     exactly what the companion mode persona spec forbids.
//
//  3. Give the AI a clear, actionable refusal so it knows what to tell the
//     user instead of retrying the call or apologising.
//
// The tool definition matches the real create_itinerary_items schema so Gemini
// treats it as the same tool, but Execute always returns a decline message.
type CompanionItineraryStub struct{}

// NewCompanionItineraryStub constructs the companion-mode decline stub.
func NewCompanionItineraryStub() *CompanionItineraryStub {
	return &CompanionItineraryStub{}
}

// Definition mirrors the real create_itinerary_items tool name so name-based
// lookups (and any model that learned the name during planning mode) resolve
// to this stub in companion mode. The description makes the decline behaviour
// explicit so the model doesn't try to work around it.
func (t *CompanionItineraryStub) Definition() ai.ToolDefinition {
	return ai.ToolDefinition{
		Name:        "create_itinerary_items",
		Description: "UNAVAILABLE in companion mode. The user is currently traveling — itinerary editing is disabled until they return home. Do NOT call this tool. If the user explicitly asks to add or save something to their itinerary, tell them their change will be waiting for them when they finish their trip; offer to remember the suggestion verbally for now.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"items": {
					"type": "array",
					"description": "Items to add — ignored in companion mode.",
					"items": {
						"type": "object",
						"properties": {
							"title": {"type": "string"}
						}
					}
				}
			}
		}`),
	}
}

// Execute always returns a status payload that tells the AI the call was
// refused and instructs it how to reply to the user. The response shape is
// intentionally success-like (no "error" key) so Gemini does not interpret it
// as a failure and retry — see the Run 4 R-16 web_search stub fix for the
// same rationale.
func (t *CompanionItineraryStub) Execute(_ context.Context, _ json.RawMessage) (json.RawMessage, error) {
	return json.Marshal(map[string]any{
		"status":  "disabled_in_companion_mode",
		"message": "Itinerary editing is disabled while the user is traveling. Tell them their plan is locked during their trip and any changes they want to make can be done after they get back. If this is urgent (e.g. a booking change), suggest they note it down somewhere for later. Do NOT call this tool again.",
	})
}
