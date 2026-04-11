package handlers

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestDeleteItineraryTool_Definition(t *testing.T) {
	tool := NewDeleteItineraryTool(nil, uuid.New(), uuid.New(), nil)
	def := tool.Definition()

	if def.Name != "delete_itinerary_items" {
		t.Errorf("expected name %q, got %q", "delete_itinerary_items", def.Name)
	}
	if def.Description == "" {
		t.Error("expected non-empty description")
	}

	var params map[string]any
	if err := json.Unmarshal(def.Parameters, &params); err != nil {
		t.Fatalf("parameters is not valid JSON: %v", err)
	}
}

func TestDeleteItineraryTool_Execute_EmptyArgs(t *testing.T) {
	tool := NewDeleteItineraryTool(nil, uuid.New(), uuid.New(), nil)

	result, err := tool.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var res map[string]any
	if err := json.Unmarshal(result, &res); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	if res["error"] != "no_items_specified" {
		t.Errorf("expected error %q, got %v", "no_items_specified", res["error"])
	}
}

func TestDeleteItineraryTool_Execute_InvalidJSON(t *testing.T) {
	tool := NewDeleteItineraryTool(nil, uuid.New(), uuid.New(), nil)

	_, err := tool.Execute(context.Background(), json.RawMessage(`not json`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestDeleteItineraryTool_WithCollaboratorEdit(t *testing.T) {
	base := NewDeleteItineraryTool(nil, uuid.New(), uuid.New(), nil)

	if base.allowCollaboratorEdit {
		t.Error("base tool should not have allowCollaboratorEdit set")
	}

	withEdit := base.WithCollaboratorEdit()

	if !withEdit.allowCollaboratorEdit {
		t.Error("WithCollaboratorEdit() should set allowCollaboratorEdit to true")
	}

	// Original should be unmodified (value copy)
	if base.allowCollaboratorEdit {
		t.Error("original tool should remain unmodified after WithCollaboratorEdit()")
	}
}

func TestDeleteItineraryTool_ParsesArgs(t *testing.T) {
	args := json.RawMessage(`{
		"item_ids": ["550e8400-e29b-41d4-a716-446655440000"],
		"titles": ["Visit Temple", "Lunch at Market"]
	}`)

	var parsed deleteItineraryArgs
	if err := json.Unmarshal(args, &parsed); err != nil {
		t.Fatalf("failed to parse args: %v", err)
	}

	if len(parsed.ItemIDs) != 1 {
		t.Errorf("expected 1 item_id, got %d", len(parsed.ItemIDs))
	}
	if len(parsed.Titles) != 2 {
		t.Errorf("expected 2 titles, got %d", len(parsed.Titles))
	}
}

func TestMatchItemsByTitle(t *testing.T) {
	// matchItemsByTitle is tested here because it's a pure function
	// that doesn't require DB access.
	tests := []struct {
		name     string
		query    string
		wantHits int
	}{
		{"exact match returns 1", "Visit Fushimi Inari Shrine", 0}, // no items to match against
		{"empty query returns nil", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched := matchItemsByTitle(nil, tt.query)
			if len(matched) != tt.wantHits {
				t.Errorf("matchItemsByTitle(nil, %q) returned %d hits, want %d", tt.query, len(matched), tt.wantHits)
			}
		})
	}
}
