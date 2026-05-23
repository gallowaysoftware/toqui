package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/gallowaysoftware/toqui-backend/internal/ai"
	"github.com/gallowaysoftware/toqui-backend/internal/trip"
)

// SelectTripTool lets the AI select an existing trip when the user describes one vaguely.
// The trip list is in the system prompt — the AI matches intent to a trip_id and calls this tool.
type SelectTripTool struct {
	tripSvc    *trip.Service
	userID     uuid.UUID
	onSelected func(tripID, title, description string)
}

type selectTripArgs struct {
	TripID string `json:"trip_id"`
}

func NewSelectTripTool(tripSvc *trip.Service, userID uuid.UUID, onSelected func(tripID, title, description string)) *SelectTripTool {
	return &SelectTripTool{tripSvc: tripSvc, userID: userID, onSelected: onSelected}
}

func (t *SelectTripTool) Definition() ai.ToolDefinition {
	return ai.ToolDefinition{
		Name:        "select_trip",
		Description: "Select an existing trip from the user's list. Use this when the user refers to a previous trip — e.g., 'let's go back to that Japan trip' or 'continue planning my Paris weekend'. Match their description to a trip_id from the list provided in your context.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"trip_id": {
					"type": "string",
					"description": "The UUID of the trip to select from the user's trip list"
				}
			},
			"required": ["trip_id"]
		}`),
	}
}

func (t *SelectTripTool) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	var params selectTripArgs
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("parse args: %w", err)
	}

	tripID, err := uuid.Parse(params.TripID)
	if err != nil {
		return nil, fmt.Errorf("invalid trip_id: %w", err)
	}

	tr, err := t.tripSvc.GetByID(ctx, t.userID, tripID)
	if err != nil {
		return nil, fmt.Errorf("trip not found: %w", err)
	}

	if t.onSelected != nil {
		desc := ""
		if tr.Description.Valid {
			desc = tr.Description.String
		}
		t.onSelected(tr.ID.String(), tr.Title, desc)
	}

	result := map[string]string{
		"trip_id": tr.ID.String(),
		"title":   tr.Title,
		"status":  tr.Status,
		"message": "Trip selected. The user will be taken to this trip's planning chat.",
	}
	return json.Marshal(result)
}
