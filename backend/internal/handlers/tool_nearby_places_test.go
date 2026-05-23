package handlers

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/gallowaysoftware/toqui-backend/internal/location"
)

func TestNearbyPlacesTool_Definition(t *testing.T) {
	svc := location.NewService("")
	tool := NewNearbyPlacesTool(svc, 35.6762, 139.6503)
	def := tool.Definition()

	if def.Name != "nearby_places" {
		t.Errorf("expected name %q, got %q", "nearby_places", def.Name)
	}

	if def.Description == "" {
		t.Error("expected non-empty description")
	}

	// Verify parameters is valid JSON
	var params map[string]any
	if err := json.Unmarshal(def.Parameters, &params); err != nil {
		t.Fatalf("parameters is not valid JSON: %v", err)
	}

	// Check that query is a required field
	required, ok := params["required"].([]any)
	if !ok {
		t.Fatal("expected required field in parameters")
	}
	found := false
	for _, r := range required {
		if r == "query" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'query' in required fields")
	}
}

func TestNearbyPlacesTool_Execute_InvalidJSON(t *testing.T) {
	svc := location.NewService("")
	tool := NewNearbyPlacesTool(svc, 35.6762, 139.6503)

	_, err := tool.Execute(context.Background(), json.RawMessage(`not json`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestNearbyPlacesTool_Execute_EmptyQuery(t *testing.T) {
	svc := location.NewService("")
	tool := NewNearbyPlacesTool(svc, 35.6762, 139.6503)

	_, err := tool.Execute(context.Background(), json.RawMessage(`{"query": ""}`))
	if err == nil {
		t.Error("expected error for empty query")
	}
}

func TestNearbyPlacesTool_Execute_NoLocation(t *testing.T) {
	svc := location.NewService("")
	tool := NewNearbyPlacesTool(svc, 0, 0)

	result, err := tool.Execute(context.Background(), json.RawMessage(`{"query": "restaurants"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]string
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if parsed["error"] != "no_location" {
		t.Errorf("expected error 'no_location', got %q", parsed["error"])
	}
}

func TestNearbyPlacesTool_Execute_ParsesArgs(t *testing.T) {
	// Verify argument parsing works correctly.
	args := json.RawMessage(`{
		"query": "coffee shops",
		"radius": 2000
	}`)

	var parsed nearbyPlacesArgs
	if err := json.Unmarshal(args, &parsed); err != nil {
		t.Fatalf("failed to parse args: %v", err)
	}

	if parsed.Query != "coffee shops" {
		t.Errorf("expected query %q, got %q", "coffee shops", parsed.Query)
	}
	if parsed.Radius != 2000 {
		t.Errorf("expected radius 2000, got %d", parsed.Radius)
	}
}

func TestNearbyPlacesTool_Execute_DefaultRadius(t *testing.T) {
	// Without a radius specified, should use default
	args := json.RawMessage(`{"query": "restaurants"}`)

	var parsed nearbyPlacesArgs
	if err := json.Unmarshal(args, &parsed); err != nil {
		t.Fatalf("failed to parse args: %v", err)
	}

	if parsed.Radius != 0 {
		t.Errorf("expected radius 0 (for default), got %d", parsed.Radius)
	}
}

func TestNearbyPlacesTool_Execute_RadiusCap(t *testing.T) {
	// With no API key configured, GetNearby returns an error.
	// The tool handler should catch this and return a lookup_failed response.
	svc := location.NewService("")
	tool := NewNearbyPlacesTool(svc, 35.6762, 139.6503)

	result, err := tool.Execute(context.Background(), json.RawMessage(`{"query": "restaurants", "radius": 99999}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	// No API key → GetNearby returns error → tool returns lookup_failed
	if errStr, ok := parsed["error"].(string); !ok || errStr != "lookup_failed" {
		t.Errorf("expected error 'lookup_failed', got %v", parsed["error"])
	}
}

func TestNearbyPlacesTool_Execute_NoAPIKey(t *testing.T) {
	// Without an API key, the service returns an error. The tool handler
	// should catch it and return a user-friendly lookup_failed response.
	svc := location.NewService("")
	tool := NewNearbyPlacesTool(svc, 48.8566, 2.3522)

	result, err := tool.Execute(context.Background(), json.RawMessage(`{"query": "restaurants"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	if errStr, ok := parsed["error"].(string); !ok || errStr != "lookup_failed" {
		t.Errorf("expected error 'lookup_failed', got %v", parsed["error"])
	}
	if msg, ok := parsed["message"].(string); !ok || msg == "" {
		t.Error("expected non-empty message for lookup failure")
	}
}
