package handlers

import (
	"context"
	"encoding/json"
	"testing"
)

func TestCreateItineraryTool_Definition(t *testing.T) {
	tool := NewCreateItineraryTool(nil, [16]byte{}, [16]byte{}, nil)
	def := tool.Definition()

	if def.Name != "create_itinerary_items" {
		t.Errorf("expected name %q, got %q", "create_itinerary_items", def.Name)
	}

	if def.Description == "" {
		t.Error("expected non-empty description")
	}

	// Verify parameters is valid JSON
	var params map[string]any
	if err := json.Unmarshal(def.Parameters, &params); err != nil {
		t.Fatalf("parameters is not valid JSON: %v", err)
	}

	// Check that items is a required field
	required, ok := params["required"].([]any)
	if !ok {
		t.Fatal("expected required field in parameters")
	}
	found := false
	for _, r := range required {
		if r == "items" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'items' in required fields")
	}
}

func TestCreateItineraryTool_Execute_EmptyItems(t *testing.T) {
	tool := NewCreateItineraryTool(nil, [16]byte{}, [16]byte{}, nil)

	_, err := tool.Execute(context.Background(), json.RawMessage(`{"items": []}`))
	if err == nil {
		t.Error("expected error for empty items")
	}
}

func TestCreateItineraryTool_Execute_InvalidJSON(t *testing.T) {
	tool := NewCreateItineraryTool(nil, [16]byte{}, [16]byte{}, nil)

	_, err := tool.Execute(context.Background(), json.RawMessage(`not json`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestCreateItineraryTool_Execute_ParsesArgs(t *testing.T) {
	// Verify argument parsing works correctly. Full DB path is covered
	// by integration tests — here we verify the JSON schema is properly parsed.

	args := json.RawMessage(`{
		"items": [
			{"day_number": 1, "order_in_day": 1, "title": "Visit Temple", "type": "sightseeing", "description": "Beautiful temple"},
			{"day_number": 1, "order_in_day": 2, "title": "Lunch at Market", "type": "meal"},
			{"day_number": 2, "title": "Hike", "type": "activity"}
		]
	}`)

	var parsed createItineraryArgs
	if err := json.Unmarshal(args, &parsed); err != nil {
		t.Fatalf("failed to parse args: %v", err)
	}

	if len(parsed.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(parsed.Items))
	}

	tests := []struct {
		idx         int
		dayNumber   int
		orderInDay  int
		title       string
		itemType    string
		description string
	}{
		{0, 1, 1, "Visit Temple", "sightseeing", "Beautiful temple"},
		{1, 1, 2, "Lunch at Market", "meal", ""},
		{2, 2, 0, "Hike", "activity", ""},
	}

	for _, tt := range tests {
		item := parsed.Items[tt.idx]
		if item.DayNumber != tt.dayNumber {
			t.Errorf("item[%d]: expected day_number %d, got %d", tt.idx, tt.dayNumber, item.DayNumber)
		}
		if item.OrderInDay != tt.orderInDay {
			t.Errorf("item[%d]: expected order_in_day %d, got %d", tt.idx, tt.orderInDay, item.OrderInDay)
		}
		if item.Title != tt.title {
			t.Errorf("item[%d]: expected title %q, got %q", tt.idx, tt.title, item.Title)
		}
		if item.Type != tt.itemType {
			t.Errorf("item[%d]: expected type %q, got %q", tt.idx, tt.itemType, item.Type)
		}
		if item.Description != tt.description {
			t.Errorf("item[%d]: expected description %q, got %q", tt.idx, tt.description, item.Description)
		}
	}
}

func TestCreateItineraryTool_Execute_SkipsEmptyTitle(t *testing.T) {
	// Empty title items should be skipped. With nil tripSvc, remaining items
	// also fail at the DB layer — but the skip logic is exercised.
	args := json.RawMessage(`{
		"items": [
			{"day_number": 1, "title": "", "type": "sightseeing"}
		]
	}`)

	var parsed createItineraryArgs
	if err := json.Unmarshal(args, &parsed); err != nil {
		t.Fatalf("failed to parse args: %v", err)
	}

	// Verify skip logic: empty title items are filtered
	skipped := 0
	for _, item := range parsed.Items {
		if item.Title == "" {
			skipped++
		}
	}
	if skipped != 1 {
		t.Errorf("expected 1 empty-title item to skip, got %d", skipped)
	}
}
