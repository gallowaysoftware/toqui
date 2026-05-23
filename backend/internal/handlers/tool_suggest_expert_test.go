package handlers

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/gallowaysoftware/toqui/backend/internal/persona"
)

func TestSuggestExpertTool_Definition(t *testing.T) {
	tool := NewSuggestExpertTool(nil, "", nil)
	def := tool.Definition()

	if def.Name != "suggest_expert" {
		t.Errorf("expected name %q, got %q", "suggest_expert", def.Name)
	}

	if def.Description == "" {
		t.Error("expected non-empty description")
	}

	// Verify parameters is valid JSON with themes as required
	var params map[string]any
	if err := json.Unmarshal(def.Parameters, &params); err != nil {
		t.Fatalf("parameters is not valid JSON: %v", err)
	}

	required, ok := params["required"].([]any)
	if !ok {
		t.Fatal("expected required field in parameters")
	}
	found := false
	for _, r := range required {
		if r == "themes" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'themes' in required fields")
	}
}

func TestSuggestExpertTool_Execute_EmptyThemes(t *testing.T) {
	composer := persona.NewComposer(nil)
	registry := persona.NewRegistry(composer)
	tool := NewSuggestExpertTool(registry, "JP", nil)

	_, err := tool.Execute(context.Background(), json.RawMessage(`{"themes": []}`))
	if err == nil {
		t.Error("expected error for empty themes")
	}
}

func TestSuggestExpertTool_Execute_InvalidJSON(t *testing.T) {
	tool := NewSuggestExpertTool(nil, "", nil)

	_, err := tool.Execute(context.Background(), json.RawMessage(`not json`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestSuggestExpertTool_Execute_NoDestination(t *testing.T) {
	composer := persona.NewComposer(nil)
	registry := persona.NewRegistry(composer)
	// No destination country provided
	tool := NewSuggestExpertTool(registry, "", nil)

	result, err := tool.Execute(context.Background(), json.RawMessage(`{"themes": ["food"]}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resp map[string]string
	if err := json.Unmarshal(result, &resp); err != nil {
		t.Fatalf("expected valid JSON response: %v", err)
	}
	if resp["error"] != "no_destination" {
		t.Errorf("expected error 'no_destination', got %q", resp["error"])
	}
}

func TestSuggestExpertTool_Execute_ResolvesExpert(t *testing.T) {
	composer := persona.NewComposer(nil) // template-based identity (no AI)
	registry := persona.NewRegistry(composer)

	var switchCalled bool
	var switchedExpert *persona.Persona
	tool := NewSuggestExpertTool(registry, "JP", func(previous, expert *persona.Persona, handoffMessage string) {
		switchCalled = true
		switchedExpert = expert
		if previous.ID != "toqui" {
			t.Errorf("expected previous persona to be toqui, got %q", previous.ID)
		}
		if handoffMessage == "" {
			t.Error("expected non-empty handoff message")
		}
	})

	result, err := tool.Execute(context.Background(), json.RawMessage(`{"themes": ["food"]}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !switchCalled {
		t.Fatal("expected onSwitch callback to be called")
	}
	if switchedExpert == nil {
		t.Fatal("expected expert to be non-nil")
	}
	if switchedExpert.ID == "toqui" {
		t.Error("expected resolved persona to be an expert, not toqui")
	}

	var resp map[string]any
	if err := json.Unmarshal(result, &resp); err != nil {
		t.Fatalf("expected valid JSON response: %v", err)
	}
	if resp["expert_name"] == nil || resp["expert_name"] == "" {
		t.Error("expected expert_name in response")
	}
	if resp["handoff_message"] == nil || resp["handoff_message"] == "" {
		t.Error("expected handoff_message in response")
	}
}

func TestSuggestExpertTool_Execute_RegionCodeOverride(t *testing.T) {
	composer := persona.NewComposer(nil)
	registry := persona.NewRegistry(composer)

	var switchedExpert *persona.Persona
	// Default destination is JP, but we'll override to IT
	tool := NewSuggestExpertTool(registry, "JP", func(_, expert *persona.Persona, _ string) {
		switchedExpert = expert
	})

	result, err := tool.Execute(context.Background(), json.RawMessage(`{"themes": ["food"], "region_code": "IT"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if switchedExpert == nil {
		t.Fatal("expected expert to be resolved")
	}

	var resp map[string]any
	if err := json.Unmarshal(result, &resp); err != nil {
		t.Fatalf("expected valid JSON response: %v", err)
	}
	if resp["expert_id"] == nil {
		t.Error("expected expert_id in response")
	}
}

func TestSuggestExpertTool_Execute_MultipleThemes(t *testing.T) {
	composer := persona.NewComposer(nil)
	registry := persona.NewRegistry(composer)

	var switchedExpert *persona.Persona
	tool := NewSuggestExpertTool(registry, "FR", func(_, expert *persona.Persona, _ string) {
		switchedExpert = expert
	})

	result, err := tool.Execute(context.Background(), json.RawMessage(`{"themes": ["food", "wine"]}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if switchedExpert == nil {
		t.Fatal("expected expert to be resolved")
	}

	// Expert should have both specialties
	if len(switchedExpert.Specialties) != 2 {
		t.Errorf("expected 2 specialties, got %d", len(switchedExpert.Specialties))
	}

	var resp map[string]any
	if err := json.Unmarshal(result, &resp); err != nil {
		t.Fatalf("expected valid JSON response: %v", err)
	}
	specialties, ok := resp["specialties"].([]any)
	if !ok || len(specialties) != 2 {
		t.Errorf("expected 2 specialties in response, got %v", resp["specialties"])
	}
}
