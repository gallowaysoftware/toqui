//go:build aitest

package aitest

import (
	"context"
	"time"

)

// CarolReturningUser tests multi-trip workflow: selecting existing trips,
// switching between trips, and creating new ones.
func CarolReturningUser() *TestScenario {
	return &TestScenario{
		Name:        "carol-returning-user",
		Description: "Returning user with existing trips — tests select_trip matching, trip switching, new trip creation",
		Tags:        []string{"regression", "selection", "select_trip", "multi_trip"},
		UserName:    "Carol Test",
		UserEmail:   "carol-returning@toqui-test.local",
		Setup: func(ctx context.Context, env *TestEnv, state *ScenarioState) error {
			// Pre-create two trips so Carol has existing trips to select from.

			// Trip 1: Moroccan Adventure (active)
			morocco, err := env.TripSvc.Create(ctx, state.UserID, "Moroccan Adventure",
				"Two weeks exploring Morocco — Marrakech, Fes, Sahara desert", nil, nil)
			if err != nil {
				return err
			}
			_ = env.TripSvc.SetDestination(ctx, state.UserID, morocco.ID, "MA")
			_, _ = env.TripSvc.Update(ctx, state.UserID, morocco.ID, "", "", "active", nil, nil)
			state.Trips[morocco.ID.String()] = TripInfo{
				ID: morocco.ID.String(), Title: "Moroccan Adventure",
				Description: "Two weeks exploring Morocco", Status: "active", Country: "MA",
			}

			// Trip 2: Greek Islands Hopping (planning)
			greece, err := env.TripSvc.Create(ctx, state.UserID, "Greek Islands Hopping",
				"Island hopping through Santorini, Mykonos, and Crete", nil, nil)
			if err != nil {
				return err
			}
			_ = env.TripSvc.SetDestination(ctx, state.UserID, greece.ID, "GR")
			state.Trips[greece.ID.String()] = TripInfo{
				ID: greece.ID.String(), Title: "Greek Islands Hopping",
				Description: "Island hopping through Santorini, Mykonos, and Crete", Status: "planning", Country: "GR",
			}

			return nil
		},
		Steps: []TestStep{
			{
				Name: "selection-vague-greece-ref",
				Action: &SendMessageAction{
					Content: "Hey! What's happening with my Greece trip?",
					Mode:    "selection",
				},
				Assertions: []Assertion{
					AssertToolCalled("select_trip"),
					AssertResponseNonEmpty(),
					AssertNoErrors(),
				},
				EvalCriteria: []EvalCriterion{
					{Name: "trip_matching", Description: "AI should correctly match 'my Greece trip' to the Greek Islands Hopping trip and acknowledge the selection", Weight: 1.0},
					{Name: "acknowledge_before_select", Description: "AI should briefly acknowledge which trip it's selecting before calling the tool, e.g., 'Let me pull up your Greek Islands trip!'", Weight: 0.8},
				},
				Timeout: 120 * time.Second,
			},
			{
				Name: "planning-santorini",
				Action: &SendMessageAction{
					Content: "I want to spend most of my time in Santorini. What are the must-see spots?",
					Mode:    "planning",
				},
				Assertions: []Assertion{
					AssertResponseNonEmpty(),
					AssertNoErrors(),
				},
				EvalCriteria: []EvalCriterion{
					{Name: "santorini_knowledge", Description: "Should give specific Santorini recommendations — Oia, Fira, Red Beach, caldera views, etc.", Weight: 1.0},
				},
				Timeout: 120 * time.Second,
			},
			{
				Name: "selection-switch-to-morocco",
				Action: &SendMessageAction{
					Content: "Actually, take me to the Morocco one instead",
					Mode:    "selection",
				},
				Assertions: []Assertion{
					AssertToolCalled("select_trip"),
					AssertResponseNonEmpty(),
					AssertNoErrors(),
				},
				EvalCriteria: []EvalCriterion{
					{Name: "correct_trip", Description: "Should select the Moroccan Adventure trip, not the Greece one", Weight: 1.0},
				},
				Timeout: 120 * time.Second,
			},
			{
				Name: "selection-new-trip",
				Action: &SendMessageAction{
					Content: "I'm also dreaming about New Zealand — maybe a road trip on the South Island",
					Mode:    "selection",
				},
				Assertions: []Assertion{
					AssertToolCalled("create_trip"),
					AssertToolArgContains("create_trip", "New Zealand"),
					AssertResponseNonEmpty(),
					AssertNoErrors(),
				},
				EvalCriteria: []EvalCriterion{
					{Name: "new_trip_creation", Description: "Should create a NEW trip for New Zealand rather than matching to existing Morocco/Greece trips", Weight: 1.0},
				},
				Timeout: 120 * time.Second,
			},
			{
				Name: "verify-trip-count",
				Action: &ListTripsAction{},
				Assertions: []Assertion{
					AssertTripCount(3),
				},
				Timeout: 10 * time.Second,
			},
		},
	}
}
