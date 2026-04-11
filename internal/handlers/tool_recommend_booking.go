package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/gallowaysoftware/toqui-backend/internal/affiliate"
	"github.com/gallowaysoftware/toqui-backend/internal/ai"
	"github.com/gallowaysoftware/toqui-backend/internal/analytics"
	"github.com/gallowaysoftware/toqui-backend/internal/tier"
)

// RecommendBookingTool is a chat tool that generates booking recommendations.
// For free-tier users the recommendations include affiliate links and an FTC
// disclosure. Pro-tier users receive unbiased recommendations instead.
type RecommendBookingTool struct {
	linkBuilder     *affiliate.LinkBuilder
	userTier        tier.UserTier
	onRecommend     func(rec affiliate.Recommendation)
	tripDestination string // fallback destination from trip context
	tripStartDate   string // fallback start date (YYYY-MM-DD)
	tripEndDate     string // fallback end date (YYYY-MM-DD)
	tripID          string // raw trip ID for sub-ID hashing
	userID          string // raw user ID for analytics (hashed before sending)
	analyticsClient *analytics.Client
}

type recommendBookingArgs struct {
	Category     string `json:"category"`
	Query        string `json:"query"`
	Origin       string `json:"origin"`
	Destination  string `json:"destination"`
	PropertyName string `json:"property_name"`
	DateFrom     string `json:"date_from"`
	DateTo       string `json:"date_to"`
}

// NewRecommendBookingTool creates a recommend_booking tool with the given
// affiliate link builder, user tier, and an optional callback invoked on each
// recommendation.
func NewRecommendBookingTool(linkBuilder *affiliate.LinkBuilder, userTier tier.UserTier, onRecommend func(rec affiliate.Recommendation)) *RecommendBookingTool {
	return &RecommendBookingTool{
		linkBuilder: linkBuilder,
		userTier:    userTier,
		onRecommend: onRecommend,
	}
}

// WithTripContext sets fallback destination/dates and trip ID from the current
// trip so affiliate URLs are pre-populated when the AI doesn't specify them
// (#176). The tripID is hashed for sub-ID tracking in affiliate URLs.
func (t *RecommendBookingTool) WithTripContext(destination, startDate, endDate, tripID string) *RecommendBookingTool {
	cp := *t
	cp.tripDestination = destination
	cp.tripStartDate = startDate
	cp.tripEndDate = endDate
	cp.tripID = tripID
	return &cp
}

// WithAnalytics configures PostHog event tracking for affiliate link generation.
func (t *RecommendBookingTool) WithAnalytics(client *analytics.Client, userID string) *RecommendBookingTool {
	cp := *t
	cp.analyticsClient = client
	cp.userID = userID
	return &cp
}

func (t *RecommendBookingTool) Definition() ai.ToolDefinition {
	description := "Generate affiliate-linked booking recommendations. Use when the user asks about flights, hotels, activities, car rentals, or travel insurance. Returns partner-linked search results with disclosure."
	if t.userTier.IsPro() {
		description = "Generate booking recommendations from the best available sources. Use when the user asks about flights, hotels, activities, car rentals, or travel insurance. Returns search results from the best sources."
	}

	return ai.ToolDefinition{
		Name:        "recommend_booking",
		Description: description,
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"category": {
					"type": "string",
					"enum": ["flight", "hotel", "activity", "car_rental", "insurance"],
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
				"property_name": {
					"type": "string",
					"description": "For hotels: the specific property name if the user mentioned one (e.g. 'St. Regis Vommuli'). Leave empty for generic destination searches."
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
		"user_tier", string(t.userTier),
	)

	rec := t.buildRecommendation(params)

	// Track affiliate link generation (async, non-blocking, privacy-safe).
	// Only tracked for free-tier users who actually receive affiliate links.
	if t.analyticsClient != nil && !t.userTier.IsPro() {
		props := map[string]any{
			"partner":  string(rec.Partner),
			"category": rec.Category,
		}
		if t.tripDestination != "" {
			props["destination_country"] = t.tripDestination
		}
		t.analyticsClient.Track(t.userID, "affiliate_link_generated", props)
	}

	if t.onRecommend != nil {
		t.onRecommend(rec)
	}

	return json.Marshal(rec)
}

// buildRecommendation constructs an affiliate Recommendation based on the
// parsed tool arguments, user tier, and the configured link builder.
func (t *RecommendBookingTool) buildRecommendation(params recommendBookingArgs) affiliate.Recommendation {
	partner := affiliate.PartnerForCategory(params.Category)
	tripHash := affiliate.HashTripID(t.tripID)

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
			dest = t.tripDestination // fall back to trip context (#176)
		}
		if dest == "" {
			dest = "anywhere"
		}
		date := params.DateFrom
		if date == "" {
			date = t.tripStartDate // fall back to trip context (#176)
		}
		if date == "" {
			date = "anytime"
		}
		searchURL = t.linkBuilder.FlightSearchURL(origin, dest, date, tripHash)
		title = fmt.Sprintf("Search flights: %s to %s", origin, dest)
		description = fmt.Sprintf("Find and compare flight prices from %s to %s", origin, dest)
		if params.DateFrom != "" {
			description += fmt.Sprintf(" departing %s", params.DateFrom)
		}

	case "hotel":
		city := params.Destination
		if city == "" {
			city = t.tripDestination // fall back to trip context (#176)
		}
		if city == "" {
			city = "your destination"
		}
		searchURL = t.linkBuilder.HotelSearchURL(params.PropertyName, city, params.DateFrom, params.DateTo, tripHash)
		if params.PropertyName != "" {
			title = fmt.Sprintf("Book %s", params.PropertyName)
			description = fmt.Sprintf("View rates and availability for %s in %s", params.PropertyName, city)
		} else {
			title = fmt.Sprintf("Search hotels in %s", city)
			description = fmt.Sprintf("Browse and compare hotel options in %s", city)
		}
		if params.DateFrom != "" && params.DateTo != "" {
			description += fmt.Sprintf(" from %s to %s", params.DateFrom, params.DateTo)
		}

	case "activity":
		query := params.Query
		searchURL = t.linkBuilder.ActivityURL(query, tripHash)
		title = fmt.Sprintf("Search activities: %s", params.Query)
		description = fmt.Sprintf("Discover tours, experiences, and activities: %s", params.Query)

	case "car_rental":
		location := params.Destination
		if location == "" {
			location = "your destination"
		}
		searchURL = t.linkBuilder.CarRentalURL(location, params.DateFrom, params.DateTo)
		title = fmt.Sprintf("Search car rentals in %s", location)
		description = fmt.Sprintf("Compare car rental prices and options in %s", location)
		if params.DateFrom != "" && params.DateTo != "" {
			description += fmt.Sprintf(" from %s to %s", params.DateFrom, params.DateTo)
		}

	case "insurance":
		dest := params.Destination
		if dest == "" {
			dest = "your trip"
		}
		searchURL = t.linkBuilder.TravelInsuranceURL(dest)
		title = fmt.Sprintf("Travel insurance for %s", dest)
		description = fmt.Sprintf("Get travel medical insurance coverage for %s with SafetyWing Nomad Insurance", dest)

	default:
		// Fallback for unknown categories — should not happen given the enum constraint
		searchURL = ""
		title = params.Query
		description = params.Query
	}

	disclosure := affiliate.FTCDisclosure
	if t.userTier.IsPro() {
		disclosure = affiliate.ProDisclosure
	}

	return affiliate.Recommendation{
		Partner:     partner,
		Title:       title,
		Description: description,
		URL:         searchURL,
		Category:    params.Category,
		Disclosure:  disclosure,
	}
}
