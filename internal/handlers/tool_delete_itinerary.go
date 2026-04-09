package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	"github.com/gallowaysoftware/toqui-backend/internal/ai"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui-backend/internal/trip"
)

// DeleteItineraryTool lets the AI remove specific itinerary items from the
// current trip. This enables conversations like "cut Venice from my itinerary"
// or "remove the day 3 items". The AI identifies the items to delete by
// matching against the itinerary injected in the system prompt, then passes
// their IDs (or titles for fuzzy matching) to this tool.
//
// Run 7 N-09 showed that without a delete tool, items persist after the user
// explicitly asks to remove them — the AI can only add, never subtract.
type DeleteItineraryTool struct {
	tripSvc   *trip.Service
	tripID    uuid.UUID
	userID    uuid.UUID
	onDeleted func(deletedIDs []string)
}

type deleteItineraryArgs struct {
	// ItemIDs is the preferred way to identify items — stable UUIDs from the
	// itinerary context injected in the system prompt.
	ItemIDs []string `json:"item_ids"`
	// Titles is a fallback when the AI doesn't have IDs. The tool fuzzy-matches
	// against existing items on the trip.
	Titles []string `json:"titles"`
}

func NewDeleteItineraryTool(tripSvc *trip.Service, tripID, userID uuid.UUID, onDeleted func(deletedIDs []string)) *DeleteItineraryTool {
	return &DeleteItineraryTool{
		tripSvc:   tripSvc,
		tripID:    tripID,
		userID:    userID,
		onDeleted: onDeleted,
	}
}

func (t *DeleteItineraryTool) Definition() ai.ToolDefinition {
	return ai.ToolDefinition{
		Name:        "delete_itinerary_items",
		Description: "Remove specific items from the current trip's itinerary. Use this when the user asks to cut, remove, or drop activities, days, or destinations from their plan. You can identify items by their IDs (from the itinerary in your context) or by title for fuzzy matching.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"item_ids": {
					"type": "array",
					"description": "UUIDs of itinerary items to delete. Preferred — use the IDs from the CURRENT TRIP CONTEXT itinerary.",
					"items": {"type": "string"}
				},
				"titles": {
					"type": "array",
					"description": "Titles of items to delete (fuzzy matched). Use when IDs are not available.",
					"items": {"type": "string"}
				}
			}
		}`),
	}
}

func (t *DeleteItineraryTool) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	var params deleteItineraryArgs
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("parse args: %w", err)
	}

	if len(params.ItemIDs) == 0 && len(params.Titles) == 0 {
		return json.Marshal(map[string]any{
			"error":   "no_items_specified",
			"message": "Provide either item_ids or titles to identify the items to delete.",
		})
	}

	// Collect UUIDs to delete — from explicit IDs and from title matching.
	toDelete := make([]uuid.UUID, 0, len(params.ItemIDs)+len(params.Titles))
	var notFound []string

	// Parse explicit IDs.
	for _, idStr := range params.ItemIDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			slog.Warn("delete_itinerary_items: invalid UUID", "id", idStr)
			notFound = append(notFound, idStr)
			continue
		}
		toDelete = append(toDelete, id)
	}

	// Fuzzy-match titles against existing items. The tripID is set at
	// construction time from the authenticated user's request, so this
	// query is scoped to a trip the user owns. The actual deletion query
	// (DeleteItineraryItem) additionally joins on trips.user_id.
	if len(params.Titles) > 0 {
		existing, err := t.tripSvc.GetItinerary(ctx, t.tripID)
		if err != nil {
			slog.Error("delete_itinerary_items: failed to load itinerary", "error", err)
			return json.Marshal(map[string]any{
				"error":   "load_failed",
				"message": "Could not load the current itinerary to match titles.",
			})
		}
		for _, title := range params.Titles {
			matched := matchItemsByTitle(existing, title)
			if len(matched) == 0 {
				notFound = append(notFound, title)
			}
			toDelete = append(toDelete, matched...)
		}
	}

	if len(toDelete) == 0 {
		return json.Marshal(map[string]any{
			"status":    "not_found",
			"message":   fmt.Sprintf("No matching items found for: %s. Check the item IDs or titles and try again.", strings.Join(notFound, ", ")),
			"not_found": notFound,
		})
	}

	// Deduplicate.
	seen := make(map[uuid.UUID]bool, len(toDelete))
	unique := make([]uuid.UUID, 0, len(toDelete))
	for _, id := range toDelete {
		if !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}

	deletedUUIDs, err := t.tripSvc.DeleteItineraryItems(ctx, t.userID, unique)
	if err != nil {
		return json.Marshal(map[string]any{
			"error":   "delete_failed",
			"message": fmt.Sprintf("Failed to delete items: %v", err),
		})
	}

	deletedIDs := make([]string, len(deletedUUIDs))
	for i, id := range deletedUUIDs {
		deletedIDs[i] = id.String()
	}
	if t.onDeleted != nil {
		t.onDeleted(deletedIDs)
	}

	result := map[string]any{
		"deleted_count": len(deletedUUIDs),
		"message":       fmt.Sprintf("Removed %d item(s) from the itinerary.", len(deletedUUIDs)),
	}
	if len(notFound) > 0 {
		result["not_found"] = notFound
	}
	return json.Marshal(result)
}

// matchItemsByTitle returns the IDs of items whose titles fuzzy-match the query.
// Matching uses case-insensitive containment — same approach as isDuplicateItem.
func matchItemsByTitle(items []dbgen.ItineraryItem, query string) []uuid.UUID {
	norm := strings.ToLower(strings.Join(strings.Fields(query), " "))
	if norm == "" {
		return nil
	}
	var matched []uuid.UUID
	for _, item := range items {
		if !item.Title.Valid {
			continue
		}
		itemNorm := strings.ToLower(strings.Join(strings.Fields(item.Title.String), " "))
		if itemNorm == norm || strings.Contains(itemNorm, norm) || strings.Contains(norm, itemNorm) {
			matched = append(matched, item.ID)
		}
	}
	return matched
}
