package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/gallowaysoftware/toqui-backend/internal/ai"
)

// stubInnerTool implements tools.Tool for expertTeaserGate tests.
// We can't talk to *dbgen.Queries without a DB, but the gate's
// "fail open when queries are nil" path is the most important one to pin
// — if it ever silently flips to "fail closed" we'd block free-tier
// expert handoffs entirely. The fail-open is also what local dev relies
// on (no DB, no quota enforcement, expert handoffs still work).
type stubInnerTool struct {
	defn       ai.ToolDefinition
	executeFn  func(ctx context.Context, args json.RawMessage) (json.RawMessage, error)
	execCalled int
}

func (s *stubInnerTool) Definition() ai.ToolDefinition { return s.defn }

func (s *stubInnerTool) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	s.execCalled++
	if s.executeFn != nil {
		return s.executeFn(ctx, args)
	}
	return json.RawMessage(`{"ok":true}`), nil
}

func TestExpertTeaserGate_DefinitionDelegatesToInner(t *testing.T) {
	inner := &stubInnerTool{defn: ai.ToolDefinition{Name: "suggest_expert", Description: "inner desc"}}
	gate := newExpertTeaserGate(inner, nil, uuid.Nil, uuid.New())
	def := gate.Definition()
	if def.Name != "suggest_expert" {
		t.Errorf("name = %q, want suggest_expert", def.Name)
	}
	if def.Description != "inner desc" {
		t.Errorf("description = %q, want inner desc — gate must NOT mutate the schema", def.Description)
	}
}

func TestExpertTeaserGate_FailOpenWhenQueriesNil(t *testing.T) {
	// Local dev runs without a DB-backed gate. nil queries means
	// "no enforcement" — pin this so a future "let's tighten the gate"
	// change doesn't silently break free-tier expert handoffs in dev.
	inner := &stubInnerTool{}
	gate := newExpertTeaserGate(inner, nil, uuid.New(), uuid.New())

	out, err := gate.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("expected fail-open success, got %v", err)
	}
	if string(out) != `{"ok":true}` {
		t.Errorf("expected inner result, got %s", out)
	}
	if inner.execCalled != 1 {
		t.Errorf("inner called %d times, want 1", inner.execCalled)
	}
}

func TestExpertTeaserGate_FailOpenWhenTripIDNil(t *testing.T) {
	// uuid.Nil tripID is the same fail-open trigger — without a trip we
	// can't increment the counter, so don't try.
	inner := &stubInnerTool{}
	// queries is left nil here too; the gate's `g.queries == nil` and
	// `g.tripID == uuid.Nil` checks are OR'd — either fires fail-open.
	gate := newExpertTeaserGate(inner, nil, uuid.Nil, uuid.New())

	if _, err := gate.Execute(context.Background(), json.RawMessage(`{}`)); err != nil {
		t.Fatalf("expected fail-open success, got %v", err)
	}
	if inner.execCalled != 1 {
		t.Errorf("inner called %d times, want 1", inner.execCalled)
	}
}

func TestExpertTeaserGate_PropagatesInnerError(t *testing.T) {
	// When the inner tool errors, the gate must surface the error and
	// must NOT increment the counter (the comment in the gate code is
	// explicit: failed calls don't consume quota). This test runs through
	// the fail-open path (no queries) but pins the error-propagation
	// behaviour from the inner tool, which is shared.
	innerErr := errors.New("inner failed")
	inner := &stubInnerTool{executeFn: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
		return nil, innerErr
	}}
	gate := newExpertTeaserGate(inner, nil, uuid.New(), uuid.New())

	_, err := gate.Execute(context.Background(), json.RawMessage(`{}`))
	if !errors.Is(err, innerErr) {
		t.Errorf("expected inner error to bubble up, got %v", err)
	}
}

func TestMaxFreeExpertCallsConstantIsStable(t *testing.T) {
	// The number "5" appears in the upgrade-prompt copy returned to the
	// AI. If someone changes the constant they need to update the copy
	// too — pin both ends so the docs and the gate can't drift apart.
	if maxFreeExpertCalls != 5 {
		t.Errorf("maxFreeExpertCalls = %d, want 5 (free-tier expert teaser quota)", maxFreeExpertCalls)
	}
}
