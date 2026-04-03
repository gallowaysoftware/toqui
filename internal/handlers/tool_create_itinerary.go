package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gallowaysoftware/toqui-backend/internal/ai"
	"github.com/gallowaysoftware/toqui-backend/internal/analytics"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui-backend/internal/trip"
)

// itineraryItemWithLocation pairs a created DB item with an optional location
// name that will be resolved to coordinates in the background.
type itineraryItemWithLocation struct {
	item         dbgen.ItineraryItem
	locationName string
}

// CreateItineraryTool is a chat tool that lets the AI add structured itinerary items
// to the current trip. It's injected into planning mode chat so the AI can say
// "Let me add that to your itinerary" and actually create the items.
type CreateItineraryTool struct {
	tripSvc         *trip.Service
	tripID          uuid.UUID
	userID          string // for analytics only (hashed before sending)
	onCreated       func(items []dbgen.ItineraryItem)
	pool            *pgxpool.Pool
	placesAPIKey    string
	analyticsClient *analytics.Client
}

type createItineraryArgs struct {
	Items []createItineraryItemArg `json:"items"`
}

type createItineraryItemArg struct {
	DayNumber    int    `json:"day_number"`
	OrderInDay   int    `json:"order_in_day"`
	Type         string `json:"type"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	LocationName string `json:"location_name"`
}

func NewCreateItineraryTool(tripSvc *trip.Service, tripID uuid.UUID, onCreated func(items []dbgen.ItineraryItem)) *CreateItineraryTool {
	return &CreateItineraryTool{tripSvc: tripSvc, tripID: tripID, onCreated: onCreated}
}

// WithGeocoding returns a copy of the tool configured to geocode location_name
// values for each created item in a background goroutine.
func (t *CreateItineraryTool) WithGeocoding(pool *pgxpool.Pool, placesAPIKey string) *CreateItineraryTool {
	cp := *t
	cp.pool = pool
	cp.placesAPIKey = placesAPIKey
	return &cp
}

// WithAnalytics returns a copy of the tool configured to send events to PostHog.
func (t *CreateItineraryTool) WithAnalytics(client *analytics.Client, userID string) *CreateItineraryTool {
	cp := *t
	cp.analyticsClient = client
	cp.userID = userID
	return &cp
}

func (t *CreateItineraryTool) Definition() ai.ToolDefinition {
	return ai.ToolDefinition{
		Name:        "create_itinerary_items",
		Description: "Add structured itinerary items to the current trip. Call this when you have specific activities, meals, or experiences to suggest. You can add multiple items at once across multiple days. Group items by neighborhood to minimize transit. Use order_in_day to reflect the natural flow of a day (morning sightseeing, lunch, afternoon activities, dinner, evening). Each item's description should include: estimated duration, a practical tip, and transit notes to the next stop when locations are far apart.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"items": {
					"type": "array",
					"description": "List of itinerary items to add. Order items logically within each day: morning first, evening last. Group nearby locations together.",
					"items": {
						"type": "object",
						"properties": {
							"day_number": {
								"type": "integer",
								"description": "Day number (1-indexed)"
							},
							"order_in_day": {
								"type": "integer",
								"description": "Order within the day (1-indexed). Use this to sequence items chronologically: 1 for first morning activity, 2 for mid-morning, etc."
							},
							"type": {
								"type": "string",
								"enum": ["activity", "meal", "transport", "accommodation", "sightseeing", "shopping", "nightlife"],
								"description": "Type of itinerary item"
							},
							"title": {
								"type": "string",
								"description": "Short title, e.g. 'Visit Fushimi Inari Shrine' or 'Lunch at Nishiki Market'"
							},
							"description": {
								"type": "string",
								"description": "Include: (1) estimated duration (e.g., 'Allow 2-3 hours'), (2) a practical tip (e.g., 'Go early to avoid crowds', 'Book 2 weeks ahead'), (3) transit note to next stop if far (e.g., '15 min walk to next stop' or 'Take metro Line 2, 20 min'). Example: 'Allow 2-3 hours. The shrine is stunning at sunrise with fewer crowds. 10 min taxi to Gion district afterward.'"
							},
							"location_name": {
								"type": "string",
								"description": "Specific, geocodable place name including city/region, e.g. 'Fushimi Inari Shrine, Kyoto, Japan' or 'Eiffel Tower, Paris, France'. Be precise enough to place on a map."
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

	var created []itineraryItemWithLocation
	var failed []string
	for _, item := range params.Items {
		if item.Title == "" {
			continue
		}
		dbItem, err := t.tripSvc.CreateItineraryItem(ctx, t.tripID, item.DayNumber, item.OrderInDay, item.Type, item.Title, item.Description)
		if err != nil {
			slog.Error("create itinerary item", "title", item.Title, "error", err)
			failed = append(failed, item.Title)
			continue
		}
		created = append(created, itineraryItemWithLocation{item: dbItem, locationName: item.LocationName})
	}

	if len(created) == 0 {
		return json.Marshal(map[string]any{
			"error":        "failed to create any itinerary items",
			"failed_items": failed,
		})
	}

	dbItems := make([]dbgen.ItineraryItem, len(created))
	toGeocode := make([]itineraryItemWithLocation, 0, len(created))
	for i, c := range created {
		dbItems[i] = c.item
		if c.locationName != "" {
			toGeocode = append(toGeocode, c)
		}
	}

	if t.onCreated != nil {
		t.onCreated(dbItems)
	}

	// Track itinerary generation (async, non-blocking, no content — just counts)
	if t.analyticsClient != nil {
		daySet := make(map[int]struct{})
		for _, c := range created {
			if c.item.DayNumber.Valid {
				daySet[int(c.item.DayNumber.Int32)] = struct{}{}
			}
		}
		t.analyticsClient.Track(t.userID, "itinerary_generated", map[string]any{
			"item_count": len(created),
			"day_count":  len(daySet),
		})
	}

	// Fire-and-forget background geocoding so it never delays the streaming response.
	if t.pool != nil && t.placesAPIKey != "" && len(toGeocode) > 0 {
		go t.geocodeItems(toGeocode)
	}

	// Build summary for the AI
	summary := make([]map[string]any, len(created))
	for i, c := range created {
		entry := map[string]any{
			"id":    c.item.ID.String(),
			"title": c.item.Title.String,
		}
		if c.item.DayNumber.Valid {
			entry["day_number"] = c.item.DayNumber.Int32
		}
		if c.item.Type.Valid {
			entry["type"] = c.item.Type.String
		}
		summary[i] = entry
	}

	result := map[string]any{
		"created_count": len(created),
		"items":         summary,
		"message":       fmt.Sprintf("Successfully added %d items to the itinerary.", len(created)),
	}
	if len(failed) > 0 {
		result["failed_count"] = len(failed)
		result["failed_items"] = failed
		result["message"] = fmt.Sprintf("Added %d items to the itinerary. %d items failed: %s",
			len(created), len(failed), strings.Join(failed, ", "))
	}
	return json.Marshal(result)
}

// geocodeItems resolves location names to coordinates and persists them.
// Runs in a background goroutine — failures are logged and never surface to the user.
// Cap at 20 items to avoid excessive API calls in a single batch.
func (t *CreateItineraryTool) geocodeItems(items []itineraryItemWithLocation) {
	const maxBatch = 20
	if len(items) > maxBatch {
		items = items[:maxBatch]
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, c := range items {
		lat, lng, err := geocodeLocation(ctx, t.placesAPIKey, c.locationName)
		if err != nil {
			slog.Warn("geocode itinerary item failed",
				"item_id", c.item.ID,
				"location_name", c.locationName,
				"error", err,
			)
			continue
		}
		if lat == 0 && lng == 0 {
			continue
		}

		const updateSQL = `UPDATE itinerary_items
			SET location = ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography
			WHERE id = $3`
		if _, err := t.pool.Exec(ctx, updateSQL, lng, lat, c.item.ID); err != nil {
			slog.Warn("update itinerary item location failed",
				"item_id", c.item.ID,
				"lat", lat,
				"lng", lng,
				"error", err,
			)
		} else {
			slog.Debug("geocoded itinerary item",
				"item_id", c.item.ID,
				"location_name", c.locationName,
				"lat", lat,
				"lng", lng,
			)
		}
	}
}
