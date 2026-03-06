package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/gallowaysoftware/toqui-backend/internal/ai"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui-backend/internal/trip"
)

// CreateItineraryTool is a chat tool that lets the AI add structured itinerary items
// to the current trip. It's injected into planning mode chat so the AI can say
// "Let me add that to your itinerary" and actually create the items.
type CreateItineraryTool struct {
	tripSvc   *trip.Service
	tripID    uuid.UUID
	onCreated func(items []dbgen.ItineraryItem)
}

type createItineraryArgs struct {
	Items []createItineraryItemArg `json:"items"`
}

type createItineraryItemArg struct {
	DayNumber   int    `json:"day_number"`
	OrderInDay  int    `json:"order_in_day"`
	Type        string `json:"type"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

func NewCreateItineraryTool(tripSvc *trip.Service, tripID uuid.UUID, onCreated func(items []dbgen.ItineraryItem)) *CreateItineraryTool {
	return &CreateItineraryTool{tripSvc: tripSvc, tripID: tripID, onCreated: onCreated}
}

func (t *CreateItineraryTool) Definition() ai.ToolDefinition {
	return ai.ToolDefinition{
		Name:        "create_itinerary_items",
		Description: "Add structured itinerary items to the current trip. Call this when you have specific activities, meals, or experiences to suggest for the trip. You can add multiple items at once across multiple days.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"items": {
					"type": "array",
					"description": "List of itinerary items to add",
					"items": {
						"type": "object",
						"properties": {
							"day_number": {
								"type": "integer",
								"description": "Day number (1-indexed)"
							},
							"order_in_day": {
								"type": "integer",
								"description": "Order within the day (1-indexed)"
							},
							"type": {
								"type": "string",
								"enum": ["activity", "meal", "transport", "accommodation", "sightseeing", "shopping", "nightlife"],
								"description": "Type of itinerary item"
							},
							"title": {
								"type": "string",
								"description": "Short title, e.g. 'Visit Fushimi Inari Shrine'"
							},
							"description": {
								"type": "string",
								"description": "Details, tips, or notes about this item"
							}
						},
						"required": ["day_number", "title", "type"]
					}
				}
			},
			"required": ["items"]
		}`),
	}
}

func (t *CreateItineraryTool) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	var params createItineraryArgs
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("parse args: %w", err)
	}

	if len(params.Items) == 0 {
		return nil, fmt.Errorf("at least one item is required")
	}

	var created []dbgen.ItineraryItem
	for _, item := range params.Items {
		if item.Title == "" {
			continue
		}
		dbItem, err := t.tripSvc.CreateItineraryItem(ctx, t.tripID, item.DayNumber, item.OrderInDay, item.Type, item.Title, item.Description)
		if err != nil {
			slog.Error("create itinerary item", "title", item.Title, "error", err)
			continue
		}
		created = append(created, dbItem)
	}

	if len(created) == 0 {
		return json.Marshal(map[string]string{
			"error": "failed to create any itinerary items",
		})
	}

	if t.onCreated != nil {
		t.onCreated(created)
	}

	// Build summary for the AI
	summary := make([]map[string]any, len(created))
	for i, item := range created {
		entry := map[string]any{
			"id":    item.ID.String(),
			"title": item.Title.String,
		}
		if item.DayNumber.Valid {
			entry["day_number"] = item.DayNumber.Int32
		}
		if item.Type.Valid {
			entry["type"] = item.Type.String
		}
		summary[i] = entry
	}

	result := map[string]any{
		"created_count": len(created),
		"items":         summary,
		"message":       fmt.Sprintf("Successfully added %d items to the itinerary.", len(created)),
	}
	return json.Marshal(result)
}
