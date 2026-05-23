package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/gallowaysoftware/toqui-backend/internal/ai"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
)

// ReorderItineraryTool is a chat tool that lets the AI move itinerary items
// between days or change their position within a day. It wraps the
// MoveItineraryItem query for targeted, non-destructive reordering.
type ReorderItineraryTool struct {
	queries *dbgen.Queries
	tripID  uuid.UUID
	userID  uuid.UUID
}

type reorderArgs struct {
	Moves []reorderMove `json:"moves"`
}

type reorderMove struct {
	ItemID    string `json:"item_id"`
	ItemTitle string `json:"item_title"`
	TargetDay int    `json:"target_day"`
	TargetPos int    `json:"target_position"`
}

func NewReorderItineraryTool(queries *dbgen.Queries, tripID, userID uuid.UUID) *ReorderItineraryTool {
	return &ReorderItineraryTool{queries: queries, tripID: tripID, userID: userID}
}

func (t *ReorderItineraryTool) Definition() ai.ToolDefinition {
	return ai.ToolDefinition{
		Name:        "reorder_itinerary_items",
		Description: "Move itinerary items to different days or positions within a day. Use this when the user wants to swap days, move an activity to a different day, or reorder items within a day. Provide the item ID (or title for fuzzy matching) and the target day number and position.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"moves": {
					"type": "array",
					"items": {
						"type": "object",
						"properties": {
							"item_id": {
								"type": "string",
								"description": "UUID of the itinerary item to move. Preferred over item_title."
							},
							"item_title": {
								"type": "string",
								"description": "Title of the item to move (fuzzy match). Use when item_id is not known."
							},
							"target_day": {
								"type": "integer",
								"description": "Day number to move the item to (e.g., 3 for Day 3)"
							},
							"target_position": {
								"type": "integer",
								"description": "Position within the target day (1 = first, 2 = second, etc.)"
							}
						},
						"required": ["target_day"]
					},
					"description": "List of items to move. Each must have either item_id or item_title, plus target_day."
				}
			},
			"required": ["moves"]
		}`),
	}
}

func (t *ReorderItineraryTool) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	var params reorderArgs
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("parse args: %w", err)
	}

	if len(params.Moves) == 0 {
		return json.Marshal(map[string]string{
			"error":   "no_moves",
			"message": "At least one move must be provided.",
		})
	}

	// Load current itinerary for fuzzy matching.
	var items []dbgen.ItineraryItem
	if t.queries != nil {
		var err error
		items, err = t.queries.ListItineraryItemsByTrip(ctx, t.tripID)
		if err != nil {
			slog.Warn("reorder tool: failed to load itinerary for matching", "error", err)
		}
	}

	results := make([]map[string]any, 0, len(params.Moves))
	movedCount := 0

	for _, move := range params.Moves {
		// Resolve item ID — prefer explicit UUID, fall back to title match.
		var itemID uuid.UUID
		if move.ItemID != "" {
			var err error
			itemID, err = uuid.Parse(move.ItemID)
			if err != nil {
				results = append(results, map[string]any{
					"item_id": move.ItemID,
					"status":  "error",
					"message": "invalid item_id format",
				})
				continue
			}
		} else if move.ItemTitle != "" {
			itemID = fuzzyMatchItem(items, move.ItemTitle)
			if itemID == uuid.Nil {
				results = append(results, map[string]any{
					"item_title": move.ItemTitle,
					"status":     "error",
					"message":    fmt.Sprintf("no itinerary item found matching '%s'", move.ItemTitle),
				})
				continue
			}
		} else {
			results = append(results, map[string]any{
				"status":  "error",
				"message": "each move must have either item_id or item_title",
			})
			continue
		}

		targetPos := move.TargetPos
		if targetPos <= 0 {
			targetPos = 1
		}

		moved, err := t.queries.MoveItineraryItem(ctx, dbgen.MoveItineraryItemParams{
			DayNumber:  pgtype.Int4{Int32: int32(move.TargetDay), Valid: true},
			OrderInDay: pgtype.Int4{Int32: int32(targetPos), Valid: true},
			ID:         itemID,
			UserID:     t.userID,
		})
		if err != nil {
			results = append(results, map[string]any{
				"item_id": itemID.String(),
				"status":  "error",
				"message": fmt.Sprintf("failed to move item: %v", err),
			})
			continue
		}

		title := ""
		if moved.Title.Valid {
			title = moved.Title.String
		}
		results = append(results, map[string]any{
			"item_id":    moved.ID.String(),
			"title":      title,
			"target_day": move.TargetDay,
			"position":   targetPos,
			"status":     "moved",
		})
		movedCount++
	}

	return json.Marshal(map[string]any{
		"moved_count": movedCount,
		"total":       len(params.Moves),
		"results":     results,
		"message":     fmt.Sprintf("Moved %d of %d items.", movedCount, len(params.Moves)),
	})
}

// fuzzyMatchItem finds the best-matching itinerary item by title.
func fuzzyMatchItem(items []dbgen.ItineraryItem, query string) uuid.UUID {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return uuid.Nil
	}

	// Exact match first, then substring.
	for _, item := range items {
		if item.Title.Valid && strings.EqualFold(item.Title.String, query) {
			return item.ID
		}
	}
	for _, item := range items {
		if item.Title.Valid && strings.Contains(strings.ToLower(item.Title.String), query) {
			return item.ID
		}
	}
	return uuid.Nil
}
