//go:build aitest

package aitest

import (
	"context"
	"time"
)

// FrankBookingRecommendations tests the recommend_booking tool across all three
// booking categories (flights, hotels, activities). This is the core monetisation
// pipeline — the AI should call the tool when users ask about bookable items and
// present the affiliate-linked results with proper FTC disclosure.
//
// Pre-creates a Barcelona trip so we skip trip-creation overhead and focus
// entirely on the booking recommendation flow.
func FrankBookingRecommendations() *TestScenario {
	return &TestScenario{
		Name:        "frank-booking-recommendations",
		Description: "Tests recommend_booking tool usage across all booking categories (flights, hotels, activities) with FTC disclosure for free-tier users",
		Tags:        []string{"regression", "booking", "affiliate", "monetization"},
		UserName:    "Frank Test",
		UserEmail:   "frank-booking@toqui-test.local",
		Setup: func(ctx context.Context, env *TestEnv, state *ScenarioState) error {
			trip, err := env.TripSvc.Create(ctx, state.UserID, "Barcelona Summer 2026",
				"A week exploring Barcelona — food, architecture, and beaches", nil, nil)
			if err != nil {
				return err
			}
			_ = env.TripSvc.SetDestination(ctx, state.UserID, trip.ID, "ES")
			state.CurrentTripID = trip.ID
			state.Trips[trip.ID.String()] = TripInfo{
				ID:          trip.ID.String(),
				Title:       "Barcelona Summer 2026",
				Description: "A week exploring Barcelona — food, architecture, and beaches",
				Status:      "planning",
				Country:     "ES",
			}
			return nil
		},
		Steps: []TestStep{
			{
				Name: "planning-flights",
				Action: &SendMessageAction{
					Content: "I need to book flights from New York to Barcelona for June 15-22. Can you help me find good options?",
					Mode:    "planning",
				},
				Assertions: []Assertion{
					AssertToolCalled("recommend_booking"),
					AssertToolArgContains("recommend_booking", "flight"),
					AssertToolResultContains("recommend_booking", "skyscanner"),
					AssertToolResultContains("recommend_booking", "commission"),
					AssertResponseNonEmpty(),
					AssertNoErrors(),
				},
				EvalCriteria: []EvalCriterion{
					{Name: "flight_recommendation", Description: "Should provide a useful flight booking recommendation with a search link. The response should present the recommendation helpfully and include the affiliate disclosure text about partner links.", Weight: 1.0},
					{Name: "disclosure_included", Description: "The response must surface the FTC disclosure text to the user — something about partner links or commissions", Weight: 1.0},
				},
				Timeout: 180 * time.Second,
			},
			{
				Name: "planning-hotels",
				Action: &SendMessageAction{
					Content: "Great, now I need a hotel in Barcelona near the Gothic Quarter. Something mid-range, around $150/night.",
					Mode:    "planning",
				},
				Assertions: []Assertion{
					AssertToolCalled("recommend_booking"),
					AssertToolArgContains("recommend_booking", "hotel"),
					AssertToolResultContains("recommend_booking", "booking.com"),
					AssertResponseNonEmpty(),
					AssertNoErrors(),
				},
				EvalCriteria: []EvalCriterion{
					{Name: "hotel_recommendation", Description: "Should provide a hotel booking recommendation with a search link for Barcelona. The response should acknowledge the user's preferences (Gothic Quarter, mid-range, ~$150/night).", Weight: 1.0},
				},
				Timeout: 180 * time.Second,
			},
			{
				Name: "planning-activities",
				Action: &SendMessageAction{
					Content: "What about tours and activities? I'd love to do a food tour and visit La Sagrada Familia.",
					Mode:    "planning",
				},
				Assertions: []Assertion{
					AssertToolCalled("recommend_booking"),
					AssertToolArgContains("recommend_booking", "activity"),
					AssertToolResultContains("recommend_booking", "getyourguide"),
					AssertResponseNonEmpty(),
					AssertNoErrors(),
				},
				EvalCriteria: []EvalCriterion{
					{Name: "activity_recommendation", Description: "Should provide activity booking recommendations related to food tours and/or Sagrada Familia visits. Should include a search link from the tool result.", Weight: 1.0},
				},
				Timeout: 180 * time.Second,
			},
			{
				Name: "planning-general-no-booking",
				Action: &SendMessageAction{
					Content: "What's the weather like in Barcelona in June? And do I need to learn any Spanish?",
					Mode:    "planning",
				},
				Assertions: []Assertion{
					AssertToolNotCalled("recommend_booking"),
					AssertResponseNonEmpty(),
					AssertNoErrors(),
				},
				EvalCriteria: []EvalCriterion{
					{Name: "no_unnecessary_booking", Description: "Should answer the weather and language question directly without calling the recommend_booking tool — this is general travel advice, not a booking request.", Weight: 1.0},
					{Name: "helpful_practical_info", Description: "Should provide practical info about June weather in Barcelona and basic Spanish/Catalan language tips.", Weight: 0.8},
				},
				Timeout: 120 * time.Second,
			},
		},
	}
}
