package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/gallowaysoftware/toqui-backend/internal/affiliate"
	"github.com/gallowaysoftware/toqui-backend/internal/ai"
)

// RecommendBookingTool is a chat tool that generates booking recommendations
// with affiliate links. The AI calls this when a user asks about flights,
// hotels, or activities to book.
type RecommendBookingTool struct {
	linkBuilder *affiliate.LinkBuilder
	onRecommend func(rec affiliate.Recommendation)
}

type recommendBookingArgs struct {
	Category    string `json:"category"`
	Query       string `json:"query"`
	Origin      string `json:"origin"`
	Destination string `json:"destination"`
	DateFrom    string `json:"date_from"`
	DateTo      string `json:"date_to"`
}

// NewRecommendBookingTool creates a recommend_booking tool with the given
// affiliate link builder and an optional callback invoked on each recommendation.
func NewRecommendBookingTool(linkBuilder *affiliate.LinkBuilder, onRecommend func(rec affiliate.Recommendation)) *RecommendBookingTool {
	return &RecommendBookingTool{
		linkBuilder: linkBuilder,
		onRecommend: onRecommend,
	}
}

func (t *RecommendBookingTool) Definition() ai.ToolDefinition {
	return ai.ToolDefinition{
		Name:        "recommend_booking",
		Description: "Generate booking recommendations with links. Use when the user asks about flights, hotels, or activities to book. Returns affiliate-linked search results.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"category": {
					"type": "string",
					"enum": ["flight", "hotel", "activity"],
					"description": "Type of booking to recommend"
				},
				"query": {
					"type": "string",
					"description": "Search description, e.g., 'flights from NYC to Prague in June' or 'hotels in Reykjavik'"
				},
				"origin": {
					"type": "string",
					"description": "For flights: origin city or airport code"
				},
				"destination": {
					"type": "string",
					"description": "Destination city or airport code"
				},
				"date_from": {
					"type": "string",
					"description": "Start date (YYYY-MM-DD)"
				},
				"date_to": {
					"type": "string",
					"description": "End date (YYYY-MM-DD)"
				}
			},
			"required": ["category", "query"]
		}`),
	}
}

func (t *RecommendBookingTool) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	var params recommendBookingArgs
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("parse args: %w", err)
	}

	if params.Category == "" {
		return nil, fmt.Errorf("category is required")
	}
	if params.Query == "" {
		return nil, fmt.Errorf("query is required")
	}

	slog.Debug("recommend_booking tool executing",
		"category", params.Category,
		"query", params.Query,
		"origin", params.Origin,
		"destination", params.Destination,
		"date_from", params.DateFrom,
		"date_to", params.DateTo,
	)

	rec := t.buildRecommendation(params)

	if t.onRecommend != nil {
		t.onRecommend(rec)
	}

	return json.Marshal(rec)
}

// buildRecommendation constructs an affiliate Recommendation based on the
// parsed tool arguments and the configured link builder.
func (t *RecommendBookingTool) buildRecommendation(params recommendBookingArgs) affiliate.Recommendation {
	partner := affiliate.PartnerForCategory(params.Category)

	var searchURL string
	var title, description string

	switch params.Category {
	case "flight":
		origin := params.Origin
		if origin == "" {
			origin = "anywhere"
		}
		dest := params.Destination
		if dest == "" {
			dest = "anywhere"
		}
		date := params.DateFrom
		if date == "" {
			date = "anytime"
		}
		searchURL = t.linkBuilder.FlightSearchURL(origin, dest, date)
		title = fmt.Sprintf("Search flights: %s to %s", origin, dest)
		description = fmt.Sprintf("Find and compare flight prices from %s to %s", origin, dest)
		if params.DateFrom != "" {
			description += fmt.Sprintf(" departing %s", params.DateFrom)
		}

	case "hotel":
		city := params.Destination
		if city == "" {
			city = "your destination"
		}
		searchURL = t.linkBuilder.HotelSearchURL(city, params.DateFrom, params.DateTo)
		title = fmt.Sprintf("Search hotels in %s", city)
		description = fmt.Sprintf("Browse and compare hotel options in %s", city)
		if params.DateFrom != "" && params.DateTo != "" {
			description += fmt.Sprintf(" from %s to %s", params.DateFrom, params.DateTo)
		}

	case "activity":
		query := params.Query
		searchURL = t.linkBuilder.ActivityURL(query)
		title = fmt.Sprintf("Search activities: %s", params.Query)
		description = fmt.Sprintf("Discover tours, experiences, and activities: %s", params.Query)

	default:
		// Fallback for unknown categories — should not happen given the enum constraint
		searchURL = ""
		title = params.Query
		description = params.Query
	}

	return affiliate.Recommendation{
		Partner:     partner,
		Title:       title,
		Description: description,
		URL:         searchURL,
		Category:    params.Category,
		Disclosure:  affiliate.FTCDisclosure,
	}
}
