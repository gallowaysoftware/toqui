//go:build aitest

package aitest

import (
	"context"
	"time"

)

// UpdateRegressionTests verifies the COALESCE fix: partial UpdateTrip calls
// must not wipe unset fields. This test uses no LLM calls so it's fast.
func UpdateRegressionTests() *TestScenario {
	return &TestScenario{
		Name:        "update-regression",
		Description: "UpdateTrip COALESCE regression — status changes must not wipe title/description/dates",
		Tags:        []string{"regression", "update", "coalesce"},
		UserName:    "UpdateTest User",
		UserEmail:   "update-regression@toqui-test.local",
		Setup: func(ctx context.Context, env *TestEnv, state *ScenarioState) error {
			// Create a trip with all fields populated
			t, err := env.TripSvc.Create(ctx, state.UserID,
				"COALESCE Test Trip",
				"A trip to verify partial updates don't destroy data",
				nil, nil)
			if err != nil {
				return err
			}
			_ = env.TripSvc.SetDestination(ctx, state.UserID, t.ID, "JP")
			state.CurrentTripID = t.ID
			state.Trips[t.ID.String()] = TripInfo{
				ID:          t.ID.String(),
				Title:       "COALESCE Test Trip",
				Description: "A trip to verify partial updates don't destroy data",
				Status:      "planning",
				Country:     "JP",
			}
			return nil
		},
		Steps: []TestStep{
			{
				Name: "update-status-only",
				Action: &UpdateTripAction{
					Status: "active",
					// Title and Description intentionally empty — must NOT be wiped
				},
				Assertions: []Assertion{
					AssertTripStatus("active"),
					AssertTripFieldNotEmpty("title"),
					AssertTripFieldNotEmpty("description"),
				},
				Timeout: 10 * time.Second,
			},
			{
				Name: "verify-after-status-update",
				Action: &VerifyTripAction{},
				Assertions: []Assertion{
					AssertTripStatus("active"),
					AssertTripFieldNotEmpty("title"),
					AssertTripFieldNotEmpty("description"),
					AssertTripFieldNotEmpty("country"),
				},
				Timeout: 10 * time.Second,
			},
			{
				Name: "update-title-only",
				Action: &UpdateTripAction{
					Title: "Updated Title",
					// Status intentionally empty — must stay "active"
				},
				Assertions: []Assertion{
					AssertTripStatus("active"), // must NOT revert to "planning"
					AssertTripFieldNotEmpty("description"),
				},
				Timeout: 10 * time.Second,
			},
			{
				Name: "update-to-completed",
				Action: &UpdateTripAction{
					Status: "completed",
				},
				Assertions: []Assertion{
					AssertTripStatus("completed"),
					AssertTripFieldNotEmpty("title"),
					AssertTripFieldNotEmpty("description"),
				},
				Timeout: 10 * time.Second,
			},
			{
				Name: "final-verify",
				Action: &VerifyTripAction{},
				Assertions: []Assertion{
					AssertTripStatus("completed"),
					AssertTripFieldNotEmpty("title"),
					AssertTripFieldNotEmpty("description"),
					AssertTripFieldNotEmpty("country"),
				},
				Timeout: 10 * time.Second,
			},
		},
	}
}
