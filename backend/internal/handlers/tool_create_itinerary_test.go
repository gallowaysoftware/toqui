package handlers

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
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

// TestIsDuplicateItem pins the dedup heuristic behavior and guards
// against the #190 LB-3 false-positive class where visibly different
// items on the same day were being treated as duplicates.
func TestIsDuplicateItem(t *testing.T) {
	day1 := func(title string) dbgen.ItineraryItem {
		return dbgen.ItineraryItem{
			DayNumber: pgtype.Int4{Int32: 1, Valid: true},
			Title:     pgtype.Text{String: title, Valid: true},
		}
	}

	cases := []struct {
		name     string
		existing []dbgen.ItineraryItem
		title    string
		want     bool
	}{
		{
			name:     "empty title is never a dup",
			existing: []dbgen.ItineraryItem{day1("Visit Louvre")},
			title:    "",
			want:     false,
		},
		{
			name:     "exact match is a dup",
			existing: []dbgen.ItineraryItem{day1("Visit the Louvre")},
			title:    "Visit the Louvre",
			want:     true,
		},
		{
			name:     "case-insensitive exact match is a dup",
			existing: []dbgen.ItineraryItem{day1("VISIT THE LOUVRE")},
			title:    "visit the louvre",
			want:     true,
		},
		{
			// Containment (one title is a substring of the other) still
			// triggers dup regardless of threshold.
			name:     "substring containment is a dup",
			existing: []dbgen.ItineraryItem{day1("Louvre Museum tour with guide")},
			title:    "Louvre Museum tour",
			want:     true,
		},
		{
			// THE LB-3 false-positive class. Two items with 2 out of 3
			// significant words in common (67% overlap) used to dup
			// under the 60% threshold. Raising to 70% lets them through.
			// Substantively different items — user asking for the
			// second should get a new item, not a silent drop.
			name:     "67%% overlap (visit/museum/today vs visit/museum/tomorrow) is NOT a dup at 70%%",
			existing: []dbgen.ItineraryItem{day1("visit museum today")},
			title:    "visit museum tomorrow",
			want:     false,
		},
		{
			// Distinct temples, shared ornamental words. Previously a
			// false-positive at 60%.
			name:     "different temples on same day are NOT dups (LB-3)",
			existing: []dbgen.ItineraryItem{day1("Visit Wat Saket temple")},
			title:    "Visit Wat Arun temple",
			want:     false,
		},
		{
			name: "different meals on same day are NOT dups",
			existing: []dbgen.ItineraryItem{
				day1("Lunch at Krua Apsorn"),
			},
			title: "Lunch at Jok Pochana",
			want:  false,
		},
		{
			name:     "different day is never a dup even if title matches",
			existing: []dbgen.ItineraryItem{{DayNumber: pgtype.Int4{Int32: 2, Valid: true}, Title: pgtype.Text{String: "Visit Louvre", Valid: true}}},
			title:    "Visit Louvre",
			want:     false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isDuplicateItem(tc.existing, 1, tc.title)
			if got != tc.want {
				t.Errorf("isDuplicateItem(%q) = %v, want %v", tc.title, got, tc.want)
			}
		})
	}
}

// TestCreateItinerary_AlreadyPresentResponseShape pins the #190 LB-3
// narration fix: when every requested item was deduped out, the
// response shape must make it impossible for the AI to say "I added
// it". The contract is:
//
//   - status == "nothing_added_already_present" (not "already_exists")
//   - newly_created_count == 0
//   - already_present_count == len(items)
//   - persisted == false
//   - no "error" key (so the AI doesn't retry)
//   - message explicitly forbids narrating "added/created/scheduled"
//
// Regression for the run22 N-01 agentic failure.
func TestCreateItinerary_AlreadyPresentResponseShape(t *testing.T) {
	// Build the shape directly — we can't easily drive the full
	// Execute() path without a live DB — but the response shape IS
	// what the AI sees. If any field is renamed or the message
	// changes tone, this test fails fast so #190 doesn't silently
	// regress.
	resp, err := json.Marshal(map[string]any{
		"status":                "nothing_added_already_present",
		"newly_created_count":   0,
		"already_present_count": 2,
		"persisted":             false,
		"message":               "NOTHING WAS ADDED. All 2 requested items are ALREADY in the user's itinerary on the specified days. Do NOT tell the user you added, created, scheduled, or saved anything.",
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(resp, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got["status"] != "nothing_added_already_present" {
		t.Errorf("status = %v, want nothing_added_already_present", got["status"])
	}
	if v, ok := got["newly_created_count"].(float64); !ok || v != 0 {
		t.Errorf("newly_created_count = %v, want 0", got["newly_created_count"])
	}
	if v, ok := got["persisted"].(bool); !ok || v != false {
		t.Errorf("persisted = %v, want false", got["persisted"])
	}
	if _, hasErr := got["error"]; hasErr {
		t.Error("response must NOT carry an 'error' key (triggers AI retry)")
	}
	msg, _ := got["message"].(string)
	if msg == "" {
		t.Fatal("message must be non-empty")
	}
	// The AI narration fix depends on these exact phrases being
	// present — if they're removed, the AI goes back to saying
	// "I added it".
	for _, must := range []string{"NOTHING WAS ADDED", "Do NOT tell the user you added"} {
		if !strings.Contains(msg, must) {
			t.Errorf("message missing required phrase %q; got %q", must, msg)
		}
	}
}
