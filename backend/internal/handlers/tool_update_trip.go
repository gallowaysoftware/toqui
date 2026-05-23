package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

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
	StartDate            string   `json:"start_date"`
	EndDate              string   `json:"end_date"`
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
		Description: "Update the current trip's title, description, destination countries, or travel dates. Call this when the user wants to rename the trip, change the description, add/change destinations, or set/change the travel date range. Only provide the fields you want to change — omitted fields stay the same.",
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
				},
				"start_date": {
					"type": "string",
					"description": "Trip start date in YYYY-MM-DD format, e.g. '2026-10-05'. Set when the user specifies when they're traveling."
				},
				"end_date": {
					"type": "string",
					"description": "Trip end date in YYYY-MM-DD format, e.g. '2026-10-12'. Set when the user specifies when they're returning."
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
	if params.Title == "" && params.Description == "" && len(params.DestinationCountries) == 0 && params.StartDate == "" && params.EndDate == "" {
		return json.Marshal(map[string]string{
			"error":   "no_fields",
			"message": "At least one of title, description, destination_countries, start_date, or end_date must be provided.",
		})
	}

	// Parse dates if provided.
	var startDate, endDate *time.Time
	if params.StartDate != "" {
		t, err := time.Parse("2006-01-02", params.StartDate)
		if err != nil {
			return json.Marshal(map[string]string{
				"error":   "invalid_start_date",
				"message": fmt.Sprintf("start_date must be in YYYY-MM-DD format, got: %s", params.StartDate),
			})
		}
		startDate = &t
	}
	if params.EndDate != "" {
		t, err := time.Parse("2006-01-02", params.EndDate)
		if err != nil {
			return json.Marshal(map[string]string{
				"error":   "invalid_end_date",
				"message": fmt.Sprintf("end_date must be in YYYY-MM-DD format, got: %s", params.EndDate),
			})
		}
		endDate = &t
	}

	// Update title/description/dates via the trip service. The Update method
	// uses COALESCE so empty strings leave the existing value untouched.
	updated, err := t.tripSvc.Update(ctx, t.userID, t.tripID, params.Title, params.Description, "", startDate, endDate, nil, "", "", "", "")
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
	if updated.StartDate.Valid {
		result["start_date"] = updated.StartDate.Time.Format("2006-01-02")
	}
	if updated.EndDate.Valid {
		result["end_date"] = updated.EndDate.Time.Format("2006-01-02")
	}
	return json.Marshal(result)
}
