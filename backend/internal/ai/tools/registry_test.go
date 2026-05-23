package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/gallowaysoftware/toqui/backend/internal/ai"
)

type fakeTool struct {
	name string
}

func (f *fakeTool) Definition() ai.ToolDefinition {
	return ai.ToolDefinition{Name: f.name, Description: "test"}
}

func (f *fakeTool) Execute(_ context.Context, _ json.RawMessage) (json.RawMessage, error) {
	return json.RawMessage(`{"ok":true}`), nil
}

func TestCanonicalToolName(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"create_itinerary_items", "createitineraryitems"},
		{"createItineraryItems", "createitineraryitems"},
		{"CreateItineraryItems", "createitineraryitems"},
		{"CreateItineraryItemsItems", "createitineraryitems"},
		{"create-itinerary-items", "createitineraryitems"},
		{"create_trip", "createtrip"},
		{"createTripTrip", "createtrip"},
		{"select_trip", "selecttrip"},
		{"suggest_expert", "suggestexpert"},
		{"suggestExpertExpert", "suggestexpert"},
		{"web_search", "websearch"},
	}
	for _, tc := range cases {
		if got := CanonicalToolName(tc.in); got != tc.want {
			t.Errorf("CanonicalToolName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestRegistryGetExactAndFuzzy(t *testing.T) {
	r := NewRegistry()
	r.Register(&fakeTool{name: "create_itinerary_items"})

	// Exact match.
	if _, ok := r.Get("create_itinerary_items"); !ok {
		t.Fatal("exact match should succeed")
	}

	// Gemini-mangled forms.
	for _, mangled := range []string{
		"createItineraryItems",
		"CreateItineraryItems",
		"CreateItineraryItemsItems",
		"create-itinerary-items",
	} {
		if _, ok := r.Get(mangled); !ok {
			t.Errorf("fuzzy lookup for %q should succeed", mangled)
		}
	}

	// A genuinely unknown tool must still miss.
	if _, ok := r.Get("totally_unrelated_tool"); ok {
		t.Error("unrelated name should not match")
	}
}

func TestRegistryExecuteFuzzy(t *testing.T) {
	r := NewRegistry()
	r.Register(&fakeTool{name: "create_itinerary_items"})

	out, err := r.Execute(context.Background(), "CreateItineraryItemsItems", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute fuzzy match failed: %v", err)
	}
	if string(out) != `{"ok":true}` {
		t.Errorf("unexpected output: %s", out)
	}
}
