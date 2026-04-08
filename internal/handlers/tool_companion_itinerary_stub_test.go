package handlers

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// TestCompanionItineraryStub_Definition verifies the stub advertises itself
// as create_itinerary_items so name-based lookups in the chat service
// resolve to this tool (rather than hitting the "unknown tool" path that
// triggered the Run 4 apology loop).
func TestCompanionItineraryStub_Definition(t *testing.T) {
	def := NewCompanionItineraryStub().Definition()
	if def.Name != "create_itinerary_items" {
		t.Errorf("expected name create_itinerary_items, got %q", def.Name)
	}
	if !strings.Contains(def.Description, "UNAVAILABLE") {
		t.Errorf("description should announce UNAVAILABLE, got %q", def.Description)
	}
	// Parameters must be valid JSON so the AI provider accepts the tool.
	var params map[string]any
	if err := json.Unmarshal(def.Parameters, &params); err != nil {
		t.Fatalf("parameters is not valid JSON: %v", err)
	}
}

// TestCompanionItineraryStub_Execute verifies the stub always returns a
// success-shaped payload (no "error" key — Gemini treats any error-keyed
// response as a failure and retries, see Run 4 R-16 web_search fix).
func TestCompanionItineraryStub_Execute(t *testing.T) {
	stub := NewCompanionItineraryStub()
	raw, err := stub.Execute(context.Background(), json.RawMessage(`{"items":[{"title":"test"}]}`))
	if err != nil {
		t.Fatalf("stub should never return an error, got: %v", err)
	}

	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("stub output should be valid JSON, got: %v", err)
	}

	if _, hasError := out["error"]; hasError {
		t.Errorf("stub output must NOT contain an 'error' key — Gemini retries on errors. Got: %v", out)
	}
	if status, ok := out["status"].(string); !ok || status != "disabled_in_companion_mode" {
		t.Errorf("expected status=disabled_in_companion_mode, got: %v", out["status"])
	}
	if msg, ok := out["message"].(string); !ok || msg == "" {
		t.Errorf("expected non-empty message to guide the AI's reply")
	}
}
