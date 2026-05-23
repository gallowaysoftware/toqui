package handlers

import (
	"context"
	"encoding/json"
	"testing"
)

func TestUpdateTripTool_Definition(t *testing.T) {
	tool := NewUpdateTripTool(nil, [16]byte{}, [16]byte{}, nil)
	def := tool.Definition()

	if def.Name != "update_trip" {
		t.Errorf("expected name %q, got %q", "update_trip", def.Name)
	}

	if def.Description == "" {
		t.Error("expected non-empty description")
	}

	// Verify parameters is valid JSON
	var params map[string]any
	if err := json.Unmarshal(def.Parameters, &params); err != nil {
		t.Fatalf("parameters is not valid JSON: %v", err)
	}

	// All fields are optional, so there should be no required array
	if _, ok := params["required"]; ok {
		t.Error("expected no required fields — all update_trip parameters are optional")
	}

	// Verify the properties exist
	props, ok := params["properties"].(map[string]any)
	if !ok {
		t.Fatal("expected properties in parameters")
	}
	for _, field := range []string{"title", "description", "destination_countries"} {
		if _, ok := props[field]; !ok {
			t.Errorf("expected %q in properties", field)
		}
	}
}

func TestUpdateTripTool_Execute_InvalidJSON(t *testing.T) {
	tool := NewUpdateTripTool(nil, [16]byte{}, [16]byte{}, nil)

	_, err := tool.Execute(context.Background(), json.RawMessage(`not json`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestUpdateTripTool_Execute_NoFields(t *testing.T) {
	tool := NewUpdateTripTool(nil, [16]byte{}, [16]byte{}, nil)

	result, err := tool.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resp map[string]string
	if err := json.Unmarshal(result, &resp); err != nil {
		t.Fatalf("expected valid JSON response: %v", err)
	}
	if resp["error"] != "no_fields" {
		t.Errorf("expected error 'no_fields', got %q", resp["error"])
	}
}

func TestUpdateTripTool_Execute_EmptyStrings(t *testing.T) {
	// Empty strings and empty array should be treated as no-op
	tool := NewUpdateTripTool(nil, [16]byte{}, [16]byte{}, nil)

	result, err := tool.Execute(context.Background(), json.RawMessage(`{"title": "", "description": "", "destination_countries": []}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resp map[string]string
	if err := json.Unmarshal(result, &resp); err != nil {
		t.Fatalf("expected valid JSON response: %v", err)
	}
	if resp["error"] != "no_fields" {
		t.Errorf("expected error 'no_fields', got %q", resp["error"])
	}
}

func TestUpdateTripArgs_ParseTitle(t *testing.T) {
	var args updateTripArgs
	if err := json.Unmarshal([]byte(`{"title": "New Title"}`), &args); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if args.Title != "New Title" {
		t.Errorf("expected title %q, got %q", "New Title", args.Title)
	}
	if args.Description != "" {
		t.Errorf("expected empty description, got %q", args.Description)
	}
	if len(args.DestinationCountries) != 0 {
		t.Errorf("expected empty destination_countries, got %v", args.DestinationCountries)
	}
}

func TestUpdateTripArgs_ParseDescription(t *testing.T) {
	var args updateTripArgs
	if err := json.Unmarshal([]byte(`{"description": "A relaxing beach trip"}`), &args); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if args.Title != "" {
		t.Errorf("expected empty title, got %q", args.Title)
	}
	if args.Description != "A relaxing beach trip" {
		t.Errorf("expected description %q, got %q", "A relaxing beach trip", args.Description)
	}
}

func TestUpdateTripArgs_ParseDestinations(t *testing.T) {
	var args updateTripArgs
	if err := json.Unmarshal([]byte(`{"destination_countries": ["GR", "TR"]}`), &args); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(args.DestinationCountries) != 2 {
		t.Fatalf("expected 2 destination countries, got %d", len(args.DestinationCountries))
	}
	if args.DestinationCountries[0] != "GR" || args.DestinationCountries[1] != "TR" {
		t.Errorf("expected [GR, TR], got %v", args.DestinationCountries)
	}
}

func TestUpdateTripArgs_ParseAllFields(t *testing.T) {
	var args updateTripArgs
	if err := json.Unmarshal([]byte(`{
		"title": "Mediterranean Adventure",
		"description": "Exploring coastal towns",
		"destination_countries": ["IT", "HR", "GR"]
	}`), &args); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if args.Title != "Mediterranean Adventure" {
		t.Errorf("expected title %q, got %q", "Mediterranean Adventure", args.Title)
	}
	if args.Description != "Exploring coastal towns" {
		t.Errorf("expected description %q, got %q", "Exploring coastal towns", args.Description)
	}
	if len(args.DestinationCountries) != 3 {
		t.Errorf("expected 3 countries, got %d", len(args.DestinationCountries))
	}
}
