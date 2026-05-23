package handlers

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// SelectTripTool — same shape as CreateTripTool; cover Definition + the
// arg-validation paths that fail before tripSvc.GetByID. The happy path
// (real trip lookup) needs a DB and lives in integration tests.

func TestSelectTripTool_Definition(t *testing.T) {
	tool := NewSelectTripTool(nil, uuid.New(), nil)
	def := tool.Definition()

	if def.Name != "select_trip" {
		t.Errorf("name = %q, want %q", def.Name, "select_trip")
	}
	if def.Description == "" {
		t.Error("description is empty")
	}

	var params map[string]any
	if err := json.Unmarshal(def.Parameters, &params); err != nil {
		t.Fatalf("parameters is not valid JSON: %v", err)
	}
	props, _ := params["properties"].(map[string]any)
	if _, ok := props["trip_id"]; !ok {
		t.Error("trip_id property missing")
	}
	required, _ := params["required"].([]any)
	if len(required) != 1 || required[0] != "trip_id" {
		t.Errorf("required = %v, want [trip_id]", required)
	}
}

func TestSelectTripTool_Execute_InvalidJSON(t *testing.T) {
	tool := NewSelectTripTool(nil, uuid.New(), nil)
	_, err := tool.Execute(context.Background(), json.RawMessage(`not json`))
	if err == nil {
		t.Fatal("expected error for non-JSON args")
	}
	if !strings.Contains(err.Error(), "parse args") {
		t.Errorf("error = %q, want 'parse args'", err)
	}
}

func TestSelectTripTool_Execute_InvalidUUID(t *testing.T) {
	// Garbage trip_id must fail at uuid.Parse, before any service call.
	// Without this gate a malformed ID would either land as a 404 (if the
	// service is forgiving) or a confusing internal error.
	tool := NewSelectTripTool(nil, uuid.New(), nil)
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"trip_id":"not-a-uuid"}`))
	if err == nil {
		t.Fatal("expected error for malformed trip_id")
	}
	if !strings.Contains(err.Error(), "invalid trip_id") {
		t.Errorf("error = %q, want 'invalid trip_id'", err)
	}
}

func TestSelectTripTool_Execute_EmptyTripID(t *testing.T) {
	tool := NewSelectTripTool(nil, uuid.New(), nil)
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"trip_id":""}`))
	if err == nil {
		t.Fatal("expected error for empty trip_id")
	}
	// uuid.Parse("") returns an error — mirror the same code path as
	// arbitrary garbage, no separate "trip_id is required" gate.
	if !strings.Contains(err.Error(), "invalid trip_id") {
		t.Errorf("error = %q, want 'invalid trip_id' (empty falls through to uuid.Parse)", err)
	}
}
