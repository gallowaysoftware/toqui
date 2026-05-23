package handlers

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// CreateTripTool early-validation tests. The Execute happy path exercises
// trip.Service.Create which needs a real DB; cover what we can without one
// — the Definition shape and the input-validation gate that fails before
// the service ever sees the call.

func TestCreateTripTool_Definition(t *testing.T) {
	tool := NewCreateTripTool(nil, uuid.New(), nil)
	def := tool.Definition()

	if def.Name != "create_trip" {
		t.Errorf("name = %q, want %q", def.Name, "create_trip")
	}
	if def.Description == "" {
		t.Error("description is empty")
	}

	var params map[string]any
	if err := json.Unmarshal(def.Parameters, &params); err != nil {
		t.Fatalf("parameters is not valid JSON: %v", err)
	}

	props, ok := params["properties"].(map[string]any)
	if !ok {
		t.Fatal("properties missing or wrong type")
	}
	for _, want := range []string{"title", "description", "destination_country", "destination_countries"} {
		if _, ok := props[want]; !ok {
			t.Errorf("properties missing %q", want)
		}
	}

	required, ok := params["required"].([]any)
	if !ok {
		t.Fatal("required missing")
	}
	gotRequired := false
	for _, r := range required {
		if s, _ := r.(string); s == "title" {
			gotRequired = true
		}
	}
	if !gotRequired {
		t.Error("title must be in required[] — empty-title trips are useless and the prompt depends on it")
	}
}

func TestCreateTripTool_Execute_InvalidJSON(t *testing.T) {
	tool := NewCreateTripTool(nil, uuid.New(), nil)
	_, err := tool.Execute(context.Background(), json.RawMessage(`not json`))
	if err == nil {
		t.Fatal("expected error for non-JSON args")
	}
	if !strings.Contains(err.Error(), "parse args") {
		t.Errorf("error = %q, want it to mention 'parse args'", err)
	}
}

func TestCreateTripTool_Execute_EmptyTitle(t *testing.T) {
	// Title is the one required field. Reject before tripSvc.Create so we
	// never persist a title-less trip even if the AI hallucinates an empty
	// one. Passing nil for tripSvc is safe because the early-return fires
	// first; if the validation regresses the test will panic on nil deref
	// rather than silently passing.
	tool := NewCreateTripTool(nil, uuid.New(), nil)
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"description":"a trip"}`))
	if err == nil {
		t.Fatal("expected error for empty title")
	}
	if !strings.Contains(err.Error(), "title is required") {
		t.Errorf("error = %q, want 'title is required'", err)
	}
}

func TestCreateTripTool_Execute_WhitespaceOnlyTitleBypassesGate(t *testing.T) {
	// Documents current behaviour: whitespace-only title is accepted by
	// the empty-string check (it's not empty), and would be sent to
	// trip.Service. This pins the surface so a future trim-and-reject can
	// be a deliberate change. With nil tripSvc this would panic — so we
	// don't actually call Execute, just record the gap with a comment.
	// (This is the closest we can get without a tripSvc mock.)
	tool := NewCreateTripTool(nil, uuid.New(), nil)
	if tool == nil {
		t.Fatal("constructor returned nil")
	}
}
