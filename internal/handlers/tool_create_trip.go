package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/gallowaysoftware/toqui-backend/internal/ai"
	"github.com/gallowaysoftware/toqui-backend/internal/trip"
)

// CreateTripTool is a chat tool that lets the AI create a trip on behalf of the user.
// It's injected into selection mode chat so Toqui can say "Let me create that trip for you."
type CreateTripTool struct {
	tripSvc *trip.Service
	userID  uuid.UUID
	// onCreated is called after a trip is created so the handler can emit a TripCreated event.
	onCreated func(tripID, title, description string)
}

type createTripArgs struct {
	Title              string `json:"title"`
	Description        string `json:"description"`
	DestinationCountry string `json:"destination_country"`
}

func NewCreateTripTool(tripSvc *trip.Service, userID uuid.UUID, onCreated func(tripID, title, description string)) *CreateTripTool {
	return &CreateTripTool{tripSvc: tripSvc, userID: userID, onCreated: onCreated}
}

func (t *CreateTripTool) Definition() ai.ToolDefinition {
	return ai.ToolDefinition{
		Name:        "create_trip",
		Description: "Create a new trip for the user. Call this when the user wants to start planning a specific trip. Use the conversation context to set a good title and description.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"title": {
					"type": "string",
					"description": "Short trip title, e.g. 'Japan Spring 2026' or 'Weekend in Paris'"
				},
				"description": {
					"type": "string",
					"description": "Brief trip description based on what the user has said"
				},
				"destination_country": {
					"type": "string",
					"description": "ISO 3166-1 alpha-2 country code for the primary destination, e.g. 'JP', 'FR', 'CR'. Set this when the destination is clear."
				}
			},
			"required": ["title"]
		}`),
	}
}

func (t *CreateTripTool) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	var params createTripArgs
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("parse args: %w", err)
	}

	if params.Title == "" {
		return nil, fmt.Errorf("title is required")
	}

	created, err := t.tripSvc.Create(ctx, t.userID, params.Title, params.Description, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("create trip: %w", err)
	}

	// Set destination country immediately if the AI provided one (avoids async tagger race condition)
	if params.DestinationCountry != "" {
		_ = t.tripSvc.SetDestination(ctx, t.userID, created.ID, params.DestinationCountry)
	}

	if t.onCreated != nil {
		t.onCreated(created.ID.String(), created.Title, created.Description.String)
	}

	result := map[string]string{
		"trip_id": created.ID.String(),
		"title":   created.Title,
		"status":  "planning",
		"message": "Trip created successfully. You are now in planning mode for this trip.",
	}
	return json.Marshal(result)
}
