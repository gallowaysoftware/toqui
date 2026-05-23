package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gallowaysoftware/toqui/backend/internal/ai"
)

type Tool interface {
	Definition() ai.ToolDefinition
	Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error)
}

// Registry holds the set of available AI tools. It is safe for concurrent
// reads (Get, Execute, Definitions) but NOT concurrent writes. All Register
// calls must happen during initialization, before the server starts accepting
// requests. Per-request extra tools use a separate map in chat/service.go.
type Registry struct {
	tools map[string]Tool
}

func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

func (r *Registry) Register(tool Tool) {
	r.tools[tool.Definition().Name] = tool
}

func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	if ok {
		return t, true
	}
	// Fall back to fuzzy lookup for provider name mangling.
	canon := CanonicalToolName(name)
	for registered, tool := range r.tools {
		if CanonicalToolName(registered) == canon {
			return tool, true
		}
	}
	return nil, false
}

func (r *Registry) Definitions() []ai.ToolDefinition {
	defs := make([]ai.ToolDefinition, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, t.Definition())
	}
	return defs
}

func (r *Registry) Execute(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, error) {
	tool, ok := r.Get(name)
	if !ok {
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
	return tool.Execute(ctx, args)
}

// CanonicalToolName normalizes a tool name so that provider-side name
// mangling (camelCase conversion, duplicated suffixes, etc.) still resolves to
// our canonical snake_case names.
//
// Examples (all collapse to the same canonical form "createitineraryitems"):
//   - create_itinerary_items
//   - createItineraryItems
//   - CreateItineraryItems
//   - CreateItineraryItemsItems (Gemini occasionally duplicates the last segment)
//
// This is a forgiving lookup — we never rename registered tools, we only
// widen the set of incoming names that match them.
func CanonicalToolName(name string) string {
	// Lowercase and strip separators.
	s := strings.ToLower(name)
	s = strings.ReplaceAll(s, "_", "")
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, " ", "")

	// Collapse an immediately-repeated trailing word (e.g. "...itemsitems").
	// We only collapse one duplicate so benign names like "search" stay intact.
	for _, suffix := range []string{"items", "item", "tool", "trip", "expert", "booking"} {
		doubled := suffix + suffix
		if strings.HasSuffix(s, doubled) {
			s = strings.TrimSuffix(s, suffix)
			break
		}
	}
	return s
}
