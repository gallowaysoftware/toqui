package handlers

import (
	"context"
	"encoding/json"
	"sync/atomic"

	"github.com/gallowaysoftware/toqui-backend/internal/ai"
	"github.com/gallowaysoftware/toqui-backend/internal/ai/tools"
)

const maxFreeExpertCalls = 3

// expertTeaserGate wraps the suggest_expert tool for free-tier users.
// It allows 3 expert handoffs per chat session as a teaser, then returns
// an upgrade prompt instead of resolving the expert.
type expertTeaserGate struct {
	inner tools.Tool
	calls atomic.Int32
}

func newExpertTeaserGate(inner tools.Tool) *expertTeaserGate {
	return &expertTeaserGate{inner: inner}
}

func (g *expertTeaserGate) Definition() ai.ToolDefinition {
	return g.inner.Definition()
}

func (g *expertTeaserGate) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	count := g.calls.Add(1)
	if count > maxFreeExpertCalls {
		return json.Marshal(map[string]string{
			"error":   "trip_pro_required",
			"message": "This trip has used all 3 free expert consultations. Upgrade to Trip Pro ($12) to unlock unlimited access to all 800+ expert personas for this trip. Tell the user about Trip Pro and suggest they upgrade.",
		})
	}

	return g.inner.Execute(ctx, args)
}
