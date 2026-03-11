//go:build aitest

package aitest

import (
	"context"
	"time"
)

// DaveItineraryAndHandoff tests the new itinerary creation tool and expert handoff:
// AI should proactively create structured itinerary items during planning,
// and hand off to an expert persona when the user asks domain-specific questions.
func DaveItineraryAndHandoff() *TestScenario {
	return &TestScenario{
		Name:        "dave-itinerary-and-handoff",
		Description: "Tests create_itinerary_items tool usage in planning and suggest_expert handoff for food expertise",
		Tags:        []string{"regression", "itinerary", "handoff", "planning", "persona"},
		UserName:    "Dave Test",
		UserEmail:   "dave-itinerary@toqui-test.local",
		Setup: func(ctx context.Context, env *TestEnv, state *ScenarioState) error {
			// Pre-create a Japan trip with destination set so tools can resolve
			trip, err := env.TripSvc.Create(ctx, state.UserID, "Tokyo Food & Culture",
				"One week exploring Tokyo's food scene and cultural landmarks", nil, nil)
			if err != nil {
				return err
			}
			_ = env.TripSvc.SetDestination(ctx, state.UserID, trip.ID, "JP")
			state.CurrentTripID = trip.ID
			state.Trips[trip.ID.String()] = TripInfo{
				ID:          trip.ID.String(),
				Title:       "Tokyo Food & Culture",
				Description: "One week exploring Tokyo's food scene and cultural landmarks",
				Status:      "planning",
				Country:     "JP",
			}
			return nil
		},
		Steps: []TestStep{
			{
				Name: "planning-request-itinerary",
				Action: &SendMessageAction{
					Content: "Plan me a 3-day itinerary for Tokyo focusing on food and sightseeing. Day 1 should be Asakusa/Senso-ji area, day 2 Shibuya/Harajuku, day 3 Tsukiji and Ginza.",
					Mode:    "planning",
				},
				Assertions: []Assertion{
					AssertToolCalled("create_itinerary_items"),
					AssertItineraryItemsCreated(3), // At least 3 items across the days
					AssertResponseNonEmpty(),
					AssertNoErrors(),
				},
				EvalCriteria: []EvalCriterion{
					{Name: "itinerary_quality", Description: "Should create a structured multi-day itinerary with specific activities, meals, and sightseeing for each day. Items should be relevant to the specified areas (Asakusa, Shibuya, Tsukiji).", Weight: 1.0},
					{Name: "proactive_tool_use", Description: "AI should proactively use create_itinerary_items to add structured items, not just describe what the user could do", Weight: 1.0},
				},
				Timeout: 180 * time.Second,
			},
			{
				Name: "planning-add-more-items",
				Action: &SendMessageAction{
					Content: "Can you add a ramen dinner for day 2? Somewhere in Shibuya. And maybe a morning coffee spot for day 1.",
					Mode:    "planning",
				},
				Assertions: []Assertion{
					AssertToolCalled("create_itinerary_items"),
					AssertItineraryItemsCreated(1), // At least 1 new item
					AssertResponseNonEmpty(),
					AssertNoErrors(),
				},
				EvalCriteria: []EvalCriterion{
					{Name: "specific_additions", Description: "Should add specific restaurant/cafe suggestions with names, not just generic 'ramen dinner'", Weight: 0.8},
				},
				Timeout: 180 * time.Second,
			},
			{
				Name: "planning-request-food-expertise",
				Action: &SendMessageAction{
					Content: "I really want to deep-dive into the Tokyo food scene. What are the absolute best ramen shops? What about hidden izakaya gems? I want the real local food experience, not touristy stuff.",
					Mode:    "planning",
				},
				Assertions: []Assertion{
					AssertToolCalled("suggest_expert"),
					AssertPersonaSwitched(),
					AssertResponseNonEmpty(),
					AssertNoErrors(),
				},
				EvalCriteria: []EvalCriterion{
					{Name: "handoff_natural", Description: "The AI should naturally introduce the food expert — not just mechanically switch. It should feel like Toqui is bringing in a friend who knows the food scene.", Weight: 1.0},
					{Name: "expert_relevance", Description: "The suggest_expert tool should be called with 'food' theme for Japan", Weight: 0.8},
				},
				Timeout: 180 * time.Second,
			},
			{
				Name: "planning-logistics-no-handoff",
				Action: &SendMessageAction{
					Content: "What's the best way to get from Narita airport to Asakusa? And do I need a Suica card?",
					Mode:    "planning",
				},
				Assertions: []Assertion{
					AssertResponseNonEmpty(),
					AssertNoErrors(),
					// Should NOT trigger a handoff — this is general logistics (Toqui's job)
				},
				EvalCriteria: []EvalCriterion{
					{Name: "practical_logistics", Description: "Should give practical transport advice — Narita Express, Skyliner, or bus options, Suica card recommendation. No need for food expert for this.", Weight: 1.0},
					{Name: "no_unnecessary_handoff", Description: "Should NOT suggest bringing in an expert for basic logistics questions", Weight: 0.8},
				},
				Timeout: 120 * time.Second,
			},
		},
	}
}
