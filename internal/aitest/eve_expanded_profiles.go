//go:build aitest

package aitest

import (
	"context"
	"time"
)

// EveExpandedProfiles tests the expanded location and theme profiles:
// newly added locations (Czech Republic, Iceland) and themes (craft-beer, hiking)
// should resolve to valid expert personas and work end-to-end through the chat system.
func EveExpandedProfiles() *TestScenario {
	return &TestScenario{
		Name:        "eve-expanded-profiles",
		Description: "Tests newly added location profiles (CZ, IS) and theme profiles (craft-beer, hiking) with expert handoff and itinerary creation",
		Tags:        []string{"regression", "profiles", "persona", "handoff", "itinerary", "planning"},
		UserName:    "Eve Test",
		UserEmail:   "eve-profiles@toqui-test.local",
		Setup: func(ctx context.Context, env *TestEnv, state *ScenarioState) error {
			// Pre-create a Czech Republic trip for craft beer touring
			trip, err := env.TripSvc.Create(ctx, state.UserID, "Prague Beer Adventure",
				"Exploring Czech Republic's legendary beer culture and medieval towns", nil, nil)
			if err != nil {
				return err
			}
			_ = env.TripSvc.SetDestination(ctx, state.UserID, trip.ID, "CZ")
			state.CurrentTripID = trip.ID
			state.Trips[trip.ID.String()] = TripInfo{
				ID:          trip.ID.String(),
				Title:       "Prague Beer Adventure",
				Description: "Exploring Czech Republic's legendary beer culture and medieval towns",
				Status:      "planning",
				Country:     "CZ",
			}
			return nil
		},
		Steps: []TestStep{
			{
				Name: "planning-craft-beer-expertise",
				Action: &SendMessageAction{
					Content: "I want to really dive deep into the Czech beer scene. What are the best craft breweries in Prague? I'm talking about the real deal — not just Pilsner Urquell tourist stuff. I want the local microbreweries and pivnice where Czechs actually drink.",
					Mode:    "planning",
				},
				Assertions: []Assertion{
					AssertToolCalled("suggest_expert"),
					AssertPersonaSwitched(),
					AssertResponseNonEmpty(),
					AssertNoErrors(),
				},
				EvalCriteria: []EvalCriterion{
					{Name: "handoff_natural", Description: "The AI should naturally introduce a craft beer expert for Czech Republic — the transition should feel smooth, not mechanical", Weight: 1.0},
					{Name: "czech_beer_knowledge", Description: "The expert response should demonstrate knowledge of Czech beer culture — mention pivnice, Czech lager traditions, or specific Prague beer districts", Weight: 0.8},
				},
				Timeout: 180 * time.Second,
			},
			{
				Name: "planning-brewery-itinerary",
				Action: &SendMessageAction{
					Content: "Plan me a 2-day brewery tour itinerary. Day 1 in Prague, day 2 a day trip to somewhere outside Prague with great beer — maybe Pilsen or somewhere in Bohemia.",
					Mode:    "planning",
				},
				Assertions: []Assertion{
					AssertToolCalled("create_itinerary_items"),
					AssertItineraryItemsCreated(2), // At least 2 items across 2 days
					AssertResponseNonEmpty(),
					AssertNoErrors(),
				},
				EvalCriteria: []EvalCriterion{
					{Name: "itinerary_quality", Description: "Should create structured itinerary items with specific brewery names and locations for both days. Day 1 Prague, day 2 outside Prague.", Weight: 1.0},
					{Name: "practical_details", Description: "Should include practical details like how to get to day 2 destination, opening hours considerations, or food pairing suggestions", Weight: 0.6},
				},
				Timeout: 180 * time.Second,
			},
			{
				Name: "selection-iceland-hiking",
				Action: &SendMessageAction{
					Content: "I'm also planning a completely different trip — I want to go hiking in Iceland. Thinking about doing some multi-day treks through the highlands.",
					Mode:    "selection",
				},
				Assertions: []Assertion{
					AssertToolCalled("create_trip"),
					AssertToolArgContains("create_trip", "Iceland"),
					AssertResponseNonEmpty(),
					AssertNoErrors(),
				},
				EvalCriteria: []EvalCriterion{
					{Name: "new_trip_creation", Description: "Should create a NEW trip for Iceland hiking, not confuse it with the Czech Republic trip", Weight: 1.0},
					{Name: "enthusiasm", Description: "Should be enthusiastic about Iceland hiking — it's a dramatic destination", Weight: 0.5},
				},
				Timeout: 120 * time.Second,
			},
			{
				Name: "planning-hiking-expertise",
				Action: &SendMessageAction{
					Content: "Tell me about the Laugavegur Trail. What should I know about multi-day trekking in Iceland? I want the real expert breakdown — gear, weather, hut bookings, everything.",
					Mode:    "planning",
				},
				Assertions: []Assertion{
					AssertToolCalled("suggest_expert"),
					AssertPersonaSwitched(),
					AssertResponseNonEmpty(),
					AssertNoErrors(),
				},
				EvalCriteria: []EvalCriterion{
					{Name: "hiking_expertise", Description: "The expert should demonstrate Iceland hiking knowledge — mention Laugavegur trail specifics, mountain hut system, weather preparation, or highland access", Weight: 1.0},
					{Name: "expert_relevance", Description: "The suggest_expert call should request hiking theme for Iceland (IS)", Weight: 0.8},
				},
				Timeout: 180 * time.Second,
			},
			{
				Name: "verify-two-trips",
				Action: &ListTripsAction{},
				Assertions: []Assertion{
					AssertTripCount(2),
				},
				Timeout: 10 * time.Second,
			},
		},
	}
}
