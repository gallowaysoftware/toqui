package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/gallowaysoftware/toqui/backend/internal/ai"
	"github.com/gallowaysoftware/toqui/backend/internal/persona"
)

// SuggestExpertTool is a chat tool that lets Toqui hand off the conversation
// to a composed expert persona. When the AI detects the user needs specialized
// knowledge (food, history, distilleries, etc.), it calls this tool to trigger
// a PersonaSwitch event on the frontend.
type SuggestExpertTool struct {
	registry            *persona.Registry
	destinationCountry  string        // from current trip context
	destinationProvider func() string // lazy fallback for selection mode
	onSwitch            func(previous, expert *persona.Persona, handoffMessage string)
}

type suggestExpertArgs struct {
	Themes     []string `json:"themes"`
	RegionCode string   `json:"region_code"`
}

func NewSuggestExpertTool(registry *persona.Registry, destinationCountry string, onSwitch func(previous, expert *persona.Persona, handoffMessage string)) *SuggestExpertTool {
	return &SuggestExpertTool{
		registry:           registry,
		destinationCountry: destinationCountry,
		onSwitch:           onSwitch,
	}
}

// WithDeferredDestination sets a lazy destination resolver for selection mode
// where the destination isn't known at tool construction time but becomes
// available after create_trip fires in the same turn (Run 12 R-16 P2).
func (t *SuggestExpertTool) WithDeferredDestination(provider func() string) *SuggestExpertTool {
	cp := *t
	cp.destinationProvider = provider
	return &cp
}

func (t *SuggestExpertTool) Definition() ai.ToolDefinition {
	return ai.ToolDefinition{
		Name:        "suggest_expert",
		Description: "Suggest switching to a specialist expert persona for the conversation. Call this when the user's questions call for deep expertise in a specific domain (local cuisine, history, spirits, adventure, etc.) that would be better served by a dedicated local expert.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"themes": {
					"type": "array",
					"items": {
						"type": "string",
						"enum": ["food", "history", "distilleries", "adventure", "wellness", "wine", "architecture", "nightlife", "shopping", "family", "photography", "nature", "romance", "budget", "luxury", "art", "music", "craft-beer", "diving", "hiking"]
					},
					"description": "Theme specialties the expert should have (1-3 themes)"
				},
				"region_code": {
					"type": "string",
					"description": "ISO 3166-1 alpha-2 country code for the expert's region (e.g., 'JP', 'IT', 'FR'). Leave empty to use the trip's destination country."
				}
			},
			"required": ["themes"]
		}`),
	}
}

func (t *SuggestExpertTool) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	var params suggestExpertArgs
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("parse args: %w", err)
	}

	if len(params.Themes) == 0 {
		return nil, fmt.Errorf("at least one theme is required")
	}

	// Use provided region code, or fall back to trip's destination country,
	// or resolve lazily from the deferred provider (selection mode — trip
	// may have been created earlier in the same turn).
	regionCode := params.RegionCode
	if regionCode == "" {
		regionCode = t.destinationCountry
	}
	if regionCode == "" && t.destinationProvider != nil {
		regionCode = t.destinationProvider()
	}

	if regionCode == "" {
		return json.Marshal(map[string]string{
			"error":   "no_destination",
			"message": "Cannot resolve an expert without a destination country. Ask the user where they're going first.",
		})
	}

	expert, err := t.registry.Resolve(ctx, regionCode, params.Themes)
	if err != nil {
		slog.Error("resolve expert persona", "region", regionCode, "themes", params.Themes, "error", err)
		return json.Marshal(map[string]string{
			"error":   "resolution_failed",
			"message": "Could not find a matching expert. Continue helping the user yourself.",
		})
	}

	// If we resolved back to Toqui (no matching expert), let the AI know
	if expert.ID == "toqui" {
		return json.Marshal(map[string]string{
			"error":   "no_expert_available",
			"message": "No specialist expert is available for this combination. Continue helping the user yourself.",
		})
	}

	handoffMessage := t.registry.HandoffMessage(expert)

	if t.onSwitch != nil {
		t.onSwitch(t.registry.Default(), expert, handoffMessage)
	}

	// The directive is the most important field — it tells the expert (whose
	// system prompt becomes active in the next tool-loop iteration) to
	// actually answer the user's question instead of just introducing
	// themselves and deferring back. Without this, Gemini treats the handoff
	// as the final action of the turn and the expert's first response is
	// often a one-line intro with no substantive answer (#193).
	result := map[string]any{
		"expert_id":       expert.ID,
		"expert_name":     expert.Name,
		"expert_greeting": expert.Greeting,
		"handoff_message": handoffMessage,
		"specialties":     expert.Specialties,
		"directive": fmt.Sprintf(
			"You are now %s. The handoff is complete — do NOT introduce yourself again, do NOT defer back to anyone, and do NOT say 'let me bring in...'. Answer the user's most recent message DIRECTLY and substantively using your expertise as %s. Treat this as your first turn answering them.",
			expert.Name, expert.Name,
		),
	}
	return json.Marshal(result)
}
