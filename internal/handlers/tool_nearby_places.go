package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/gallowaysoftware/toqui-backend/internal/ai"
	"github.com/gallowaysoftware/toqui-backend/internal/location"
)

// NearbyPlacesTool is a chat tool that finds nearby places using the user's
// cached location as the center point. It is injected only in companion mode
// so the AI can answer "what's nearby?" questions with real data.
type NearbyPlacesTool struct {
	locationSvc *location.Service
	lat         float64
	lng         float64
}

type nearbyPlacesArgs struct {
	Query  string `json:"query"`
	Radius int    `json:"radius"`
}

// NewNearbyPlacesTool creates a nearby places tool centered on the given
// coordinates. The lat/lng should come from the user's cached or
// request-level location.
func NewNearbyPlacesTool(locationSvc *location.Service, lat, lng float64) *NearbyPlacesTool {
	return &NearbyPlacesTool{locationSvc: locationSvc, lat: lat, lng: lng}
}

func (t *NearbyPlacesTool) Definition() ai.ToolDefinition {
	return ai.ToolDefinition{
		Name:        "nearby_places",
		Description: "Find nearby places of interest around the user's current location. Use this when the user asks about restaurants, attractions, shops, or other points of interest near them. The search is centered on the user's current location.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"query": {
					"type": "string",
					"description": "What type of place to search for, e.g. 'restaurants', 'coffee shops', 'museums', 'ATM'"
				},
				"radius": {
					"type": "integer",
					"description": "Search radius in meters (default: 1000, max: 5000)"
				}
			},
			"required": ["query"]
		}`),
	}
}

func (t *NearbyPlacesTool) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	var params nearbyPlacesArgs
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("parse args: %w", err)
	}

	if params.Query == "" {
		return nil, fmt.Errorf("query is required")
	}

	// Validate that we have a location
	if t.lat == 0 && t.lng == 0 {
		return json.Marshal(map[string]string{
			"error":   "no_location",
			"message": "The user's location is not available. Ask them to enable location sharing in the app.",
		})
	}

	// Default and cap radius
	radius := params.Radius
	if radius <= 0 {
		radius = 1000
	}
	if radius > 5000 {
		radius = 5000
	}

	slog.Debug("nearby_places tool executing",
		"query", params.Query,
		"lat", t.lat,
		"lng", t.lng,
		"radius", radius,
	)

	places, err := t.locationSvc.GetNearby(ctx, t.lat, t.lng, params.Query, radius)
	if err != nil {
		slog.Error("nearby_places lookup failed", "error", err)
		return json.Marshal(map[string]string{
			"error":   "lookup_failed",
			"message": "Could not search for nearby places. Try again later.",
		})
	}

	if len(places) == 0 {
		return json.Marshal(map[string]any{
			"places":  []any{},
			"message": fmt.Sprintf("No %s found within %d meters of the user's location.", params.Query, radius),
		})
	}

	type placeResult struct {
		Name      string  `json:"name"`
		Category  string  `json:"category,omitempty"`
		Address   string  `json:"address,omitempty"`
		Distance  float64 `json:"distance_meters"`
		Rating    float64 `json:"rating,omitempty"`
		PlaceID   string  `json:"place_id,omitempty"`
	}

	results := make([]placeResult, len(places))
	for i, p := range places {
		results[i] = placeResult{
			Name:     p.Name,
			Category: p.Category,
			Address:  p.Address,
			Distance: p.DistanceM,
			Rating:   p.Rating,
			PlaceID:  p.GooglePlaceID,
		}
	}

	return json.Marshal(map[string]any{
		"places":  results,
		"count":   len(results),
		"message": fmt.Sprintf("Found %d places matching '%s' near the user.", len(results), params.Query),
	})
}
