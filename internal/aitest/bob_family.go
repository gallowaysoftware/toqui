//go:build aitest

package aitest

import (
	"time"

)

// BobFamilyPlanner tests that planning mode has trip context —
// the AI must know the destination and not ask "where are you going?"
func BobFamilyPlanner() *TestScenario {
	return &TestScenario{
		Name:        "bob-family-planner",
		Description: "Family vacation to Costa Rica — tests planning context injection so AI knows the destination",
		Tags:        []string{"regression", "planning", "context", "companion"},
		UserName:    "Bob Test",
		UserEmail:   "bob-family@toqui-test.local",
		Steps: []TestStep{
			{
				Name: "selection-family-trip",
				Action: &SendMessageAction{
					Content: "We're planning a family vacation to Costa Rica! Two kids, ages 6 and 9. They love animals and nature.",
					Mode:    "selection",
				},
				Assertions: []Assertion{
					AssertToolCalled("create_trip"),
					AssertToolArgContains("create_trip", "Costa Rica"),
					AssertResponseNonEmpty(),
					AssertNoErrors(),
				},
				EvalCriteria: []EvalCriterion{
					{Name: "family_focus", Description: "Response should acknowledge the family context — kids' ages, animal/nature interests", Weight: 0.5},
				},
				Timeout: 120 * time.Second,
			},
			{
				Name: "planning-areas",
				Action: &SendMessageAction{
					Content: "What areas should we stay in? We want kid-friendly accommodations with pools.",
					Mode:    "planning",
				},
				Assertions: []Assertion{
					AssertResponseNonEmpty(),
					AssertResponseNotContains("where are you going"),
					AssertResponseNotContains("which country"),
					AssertResponseNotContains("what destination"),
					AssertNoErrors(),
				},
				EvalCriteria: []EvalCriterion{
					{Name: "destination_awareness", Description: "Must NOT ask where the user is going — should know it's Costa Rica from trip context and give Costa Rica-specific area recommendations", Weight: 1.0},
					{Name: "family_appropriate", Description: "Recommendations should be appropriate for families with young children (6 and 9)", Weight: 0.8},
				},
				Timeout: 120 * time.Second,
			},
			{
				Name: "planning-safety",
				Action: &SendMessageAction{
					Content: "What about safety? Any areas we should avoid with young children?",
					Mode:    "planning",
				},
				Assertions: []Assertion{
					AssertResponseNonEmpty(),
					AssertNoErrors(),
				},
				EvalCriteria: []EvalCriterion{
					{Name: "costa_rica_specific", Description: "Safety advice should be specific to Costa Rica — mention specific regions, common concerns (wildlife, currents, etc.), not generic travel safety tips", Weight: 1.0},
					{Name: "reassuring_tone", Description: "Should be honest about risks but reassuring — Costa Rica is generally safe for families", Weight: 0.5},
				},
				Timeout: 120 * time.Second,
			},
			{
				Name: "activate-trip",
				Action: &UpdateTripAction{
					Status: "active",
				},
				Assertions: []Assertion{
					AssertTripStatus("active"),
					AssertTripFieldNotEmpty("title"),
				},
				Timeout: 10 * time.Second,
			},
			{
				Name: "companion-la-fortuna",
				Action: &SendMessageAction{
					Content: "We're at our hotel in La Fortuna. Kids are restless after the drive — what can we do this afternoon?",
					Mode:    "companion",
				},
				Assertions: []Assertion{
					AssertResponseNonEmpty(),
					AssertNoErrors(),
				},
				EvalCriteria: []EvalCriterion{
					{Name: "location_specific", Description: "Should give La Fortuna-specific activity recommendations — Arenal volcano area attractions, hot springs, wildlife tours", Weight: 1.0},
					{Name: "kid_friendly", Description: "Activities should be appropriate for children ages 6 and 9 — not extreme adventure activities", Weight: 0.8},
					{Name: "immediate_actionable", Description: "Should suggest things doable THIS AFTERNOON — not multi-day itineraries", Weight: 0.7},
				},
				Timeout: 120 * time.Second,
			},
		},
	}
}
