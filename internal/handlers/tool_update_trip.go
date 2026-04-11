package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/gallowaysoftware/toqui-backend/internal/ai"
	"github.com/gallowaysoftware/toqui-backend/internal/trip"
)

// UpdateTripTool is a chat tool that lets the AI modify a trip's title,
// description, or destination countries through conversation. It's injected
// into planning and companion modes so the AI can say "Let me update that
// for you" when the user wants to change trip details.
type UpdateTripTool struct {
	tripSvc   *trip.Service
	tripID    uuid.UUID
	userID    uuid.UUID
	onUpdated func(tripID, title, description string, countries []string)
}

type updateTripArgs struct {
	Title                string   `json:"title"`
	Description          string   `json:"description"`
	DestinationCountries []string `json:"destination_countries"`
}

func NewUpdateTripTool(
	tripSvc *trip.Service,
	tripID uuid.UUID,
	userID uuid.UUID,
	onUpdated func(tripID, title, description string, countries []string),
) *UpdateTripTool {
	return &UpdateTripTool{
		tripSvc:   tripSvc,
		tripID:    tripID,
		userID:    userID,
		onUpdated: onUpdated,
	}
}

func (t *UpdateTripTool) Definition() ai.ToolDefinition {
	return ai.ToolDefinition{
		Name:        "update_trip",
		Description: "Update the current trip's title, description, or destination countries. Call this when the user wants to rename the trip, change the description, or add/change destinations. Only provide the fields you want to change — omitted fields stay the same.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"title": {
					"type": "string",
					"description": "New trip title, e.g. 'Japan Spring 2026' or 'Weekend in Paris'"
				},
				"description": {
					"type": "string",
					"description": "New trip description summarizing the travel plans"
				},
				"destination_countries": {
					"type": "array",
					"items": {"type": "string"},
					"description": "ISO 3166-1 alpha-2 country codes for the destination countries, e.g. ['JP'] or ['GR','TR']. Replaces the entire list of destinations."
				}
			}
		}`),
	}
}

func (t *UpdateTripTool) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	var params updateTripArgs
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("parse args: %w", err)
	}

	// At least one field must be provided.
	if params.Title == "" && params.Description == "" && len(params.DestinationCountries) == 0 {
		return json.Marshal(map[string]string{
			"error":   "no_fields",
			"message": "At least one of title, description, or destination_countries must be provided.",
		})
	}

	// Update title/description via the trip service. The Update method uses
	// COALESCE so empty strings leave the existing value untouched.
	updated, err := t.tripSvc.Update(ctx, t.userID, t.tripID, params.Title, params.Description, "", nil, nil)
	if err != nil {
		return nil, fmt.Errorf("update trip: %w", err)
	}

	// Update destinations if provided.
	if len(params.DestinationCountries) > 0 {
		if err := t.tripSvc.SetDestinations(ctx, t.userID, t.tripID, params.DestinationCountries); err != nil {
			slog.Warn("failed to set trip destinations on update",
				"trip_id", t.tripID,
				"countries", params.DestinationCountries,
				"error", err,
			)
			// Non-fatal: title/description update already succeeded.
		}
	}

	if t.onUpdated != nil {
		t.onUpdated(
			updated.ID.String(),
			updated.Title,
			updated.Description.String,
			params.DestinationCountries,
		)
	}

	result := map[string]any{
		"trip_id": updated.ID.String(),
		"title":   updated.Title,
		"message": "Trip updated successfully.",
	}
	if updated.Description.Valid {
		result["description"] = updated.Description.String
	}
	if len(params.DestinationCountries) > 0 {
		result["destination_countries"] = params.DestinationCountries
	}
	return json.Marshal(result)
}
