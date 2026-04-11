package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/gallowaysoftware/toqui-backend/internal/ai"
	"github.com/gallowaysoftware/toqui-backend/internal/ai/tools"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
)

const maxFreeExpertCalls = 5

// expertTeaserGate wraps the suggest_expert tool for free-tier users.
// It allows 5 expert handoffs per trip as a generous teaser — enough to
// experience 2-3 different experts — then returns an upgrade prompt.
//
// The counter is persisted in the trips table (expert_calls column) so it
// survives across messages and sessions. The counter is only incremented
// AFTER the inner tool executes successfully, so failed calls don't
// consume the user's quota.
type expertTeaserGate struct {
	inner   tools.Tool
	queries *dbgen.Queries
	tripID  uuid.UUID
	userID  uuid.UUID
}

func newExpertTeaserGate(inner tools.Tool, queries *dbgen.Queries, tripID, userID uuid.UUID) *expertTeaserGate {
	return &expertTeaserGate{inner: inner, queries: queries, tripID: tripID, userID: userID}
}

func (g *expertTeaserGate) Definition() ai.ToolDefinition {
	return g.inner.Definition()
}

func (g *expertTeaserGate) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	// If we can't persist the counter (no queries or no trip ID), fall through
	// to the inner tool — better to allow than to silently break expert handoffs.
	if g.queries == nil || g.tripID == uuid.Nil {
		slog.Warn("expert gate: no queries or trip ID, allowing call without counting")
		return g.inner.Execute(ctx, args)
	}

	// Check current count before executing. This is a read-only check to
	// avoid incrementing on calls that will be rejected.
	currentCount, err := g.queries.GetExpertCalls(ctx, dbgen.GetExpertCallsParams{
		ID:     g.tripID,
		UserID: g.userID,
	})
	if err != nil {
		slog.Error("expert gate: failed to check counter", "error", err, "trip_id", g.tripID)
		// Fail open on DB error.
		return g.inner.Execute(ctx, args)
	}

	if currentCount >= int32(maxFreeExpertCalls) {
		return json.Marshal(map[string]string{
			"error":   "trip_pro_required",
			"message": fmt.Sprintf("This trip has used all %d free expert consultations. Upgrade to Trip Pro ($19) to unlock unlimited access to all 800+ expert personas for this trip. Tell the user about Trip Pro and suggest they upgrade.", maxFreeExpertCalls),
		})
	}

	// Execute the inner tool first.
	result, execErr := g.inner.Execute(ctx, args)
	if execErr != nil {
		return result, execErr
	}

	// Only increment after successful execution so failed calls don't
	// consume the user's quota.
	if _, err := g.queries.IncrementExpertCalls(ctx, dbgen.IncrementExpertCallsParams{
		ID:     g.tripID,
		UserID: g.userID,
	}); err != nil {
		slog.Error("expert gate: failed to increment counter after success", "error", err, "trip_id", g.tripID)
	}

	return result, nil
}
