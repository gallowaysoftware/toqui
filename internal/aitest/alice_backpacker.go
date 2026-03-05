//go:build aitest

package aitest

import (
	"time"

)

// AliceBackpackerLifecycle tests the full trip lifecycle for a first-time user:
// selection → planning (multi-turn) → activate → companion → complete.
func AliceBackpackerLifecycle() *TestScenario {
	return &TestScenario{
		Name:        "alice-backpacker-lifecycle",
		Description: "First-time user creates Vietnam backpacking trip, plans route/budget, travels, completes",
		Tags:        []string{"regression", "lifecycle", "selection", "planning", "companion"},
		UserName:    "Alice Test",
		UserEmail:   "alice-backpacker@toqui-test.local",
		Steps: []TestStep{
			{
				Name: "selection-create-trip",
				Action: &SendMessageAction{
					Content: "I'm thinking about backpacking through Vietnam for a month. Street food, motorbikes, the whole deal",
					Mode:    "selection",
				},
				Assertions: []Assertion{
					AssertToolCalled("create_trip"),
					AssertToolArgContains("create_trip", "Vietnam"),
					AssertResponseNonEmpty(),
					AssertNoErrors(),
				},
				EvalCriteria: []EvalCriterion{
					{Name: "enthusiasm", Description: "Response should be enthusiastic about Vietnam backpacking and feel like a friend excited to help plan", Weight: 0.5},
					{Name: "proactive_creation", Description: "AI should proactively create the trip without waiting for the user to explicitly ask. It should feel natural.", Weight: 1.0},
				},
				Timeout: 120 * time.Second,
			},
			{
				Name: "planning-route",
				Action: &SendMessageAction{
					Content: "What's the best route? I want to start in Hanoi and end in Ho Chi Minh City",
					Mode:    "planning",
				},
				Assertions: []Assertion{
					AssertResponseNonEmpty(),
					AssertResponseMinLength(200),
					AssertNoErrors(),
				},
				EvalCriteria: []EvalCriterion{
					{Name: "route_quality", Description: "Should suggest a logical north-to-south route with specific cities/stops between Hanoi and Ho Chi Minh City", Weight: 1.0},
					{Name: "destination_awareness", Description: "Must NOT ask 'where are you going?' — should know it's Vietnam from the trip context", Weight: 1.0},
				},
				Timeout: 120 * time.Second,
			},
			{
				Name: "planning-budget",
				Action: &SendMessageAction{
					Content: "I'm on a tight budget, maybe $30/day. What kind of hostels should I look for?",
					Mode:    "planning",
					// SessionID inherited from state — same session
				},
				Assertions: []Assertion{
					AssertResponseNonEmpty(),
					AssertNoErrors(),
				},
				EvalCriteria: []EvalCriterion{
					{Name: "budget_relevance", Description: "Advice should be specific to $30/day Vietnam budget — mention hostel prices in USD or VND, specific hostel chains or types common in Vietnam", Weight: 1.0},
					{Name: "continuity", Description: "Should reference the route from the previous message or at least acknowledge the Vietnam backpacking context", Weight: 0.5},
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
					AssertTripFieldNotEmpty("title"),       // COALESCE regression
					AssertTripFieldNotEmpty("description"), // COALESCE regression
				},
				Timeout: 10 * time.Second,
			},
			{
				Name: "companion-arrival",
				Action: &SendMessageAction{
					Content: "I just arrived at Hanoi airport. Where should I go first?",
					Mode:    "companion",
				},
				Assertions: []Assertion{
					AssertResponseNonEmpty(),
					AssertNoErrors(),
				},
				EvalCriteria: []EvalCriterion{
					{Name: "actionable", Description: "Should give immediate, practical advice — transportation from airport, first destination, what to do upon arrival", Weight: 1.0},
					{Name: "conciseness", Description: "Companion mode should be concise and actionable, not a wall of text. Under 500 words ideally.", Weight: 0.5},
				},
				Timeout: 120 * time.Second,
			},
			{
				Name: "complete-trip",
				Action: &UpdateTripAction{
					Status: "completed",
				},
				Assertions: []Assertion{
					AssertTripStatus("completed"),
					AssertTripFieldNotEmpty("title"), // COALESCE regression
				},
				Timeout: 10 * time.Second,
			},
		},
	}
}
