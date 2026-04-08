package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestValidateReportMap_ValidReport confirms a canonical well-formed
// report produces zero errors and zero warnings. This is the shape the
// orchestrator expects from every agent — any regression here means
// the skill file and validator have drifted apart.
func TestValidateReportMap_ValidReport(t *testing.T) {
	body := `{
		"persona_id": "R-02",
		"persona_name": "Family Costa Rica",
		"status": "COMPLETED",
		"trip_destination": "Costa Rica",
		"completed_steps": ["auth","selection","planning"],
		"bugs": [],
		"ux_issues": [],
		"ai_behavior_issues": [],
		"tool_failures": [],
		"usefulness_evaluation": {
			"overall_score": 5,
			"trip_creation_score": 5,
			"itinerary_quality_score": 5,
			"persona_handoff_score": 5,
			"booking_parsing_score": 0,
			"companion_mode_score": 5,
			"would_use_again": true,
			"narrative": "great"
		},
		"feature_coverage": ["selection","planning"]
	}`
	var m map[string]any
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		t.Fatalf("setup: %v", err)
	}
	errs, warns := validateReportMap(m)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
	if len(warns) != 0 {
		t.Errorf("expected no warnings, got: %v", warns)
	}
}

// TestValidateReportMap_CatchesCommonMistakes asserts each class of
// agent-side mistake we want to catch at validation time. A failure
// here means the validator is too permissive for that mistake.
func TestValidateReportMap_CatchesCommonMistakes(t *testing.T) {
	cases := []struct {
		name         string
		body         string
		wantContains string
	}{
		{
			name:         "bad persona_id format",
			body:         `{"persona_id":"bogus","persona_name":"x","status":"COMPLETED","completed_steps":[],"bugs":[],"ux_issues":[],"ai_behavior_issues":[],"tool_failures":[],"feature_coverage":[],"usefulness_evaluation":{"overall_score":5,"trip_creation_score":5,"itinerary_quality_score":5,"persona_handoff_score":5,"booking_parsing_score":5,"companion_mode_score":5,"would_use_again":true,"narrative":"x"}}`,
			wantContains: "persona_id",
		},
		{
			name:         "score out of range",
			body:         `{"persona_id":"R-02","persona_name":"x","status":"COMPLETED","completed_steps":[],"bugs":[],"ux_issues":[],"ai_behavior_issues":[],"tool_failures":[],"feature_coverage":[],"usefulness_evaluation":{"overall_score":7,"trip_creation_score":5,"itinerary_quality_score":5,"persona_handoff_score":5,"booking_parsing_score":5,"companion_mode_score":5,"would_use_again":true,"narrative":"x"}}`,
			wantContains: "must be 0..5",
		},
		{
			name:         "would_use_again as string",
			body:         `{"persona_id":"R-02","persona_name":"x","status":"COMPLETED","completed_steps":[],"bugs":[],"ux_issues":[],"ai_behavior_issues":[],"tool_failures":[],"feature_coverage":[],"usefulness_evaluation":{"overall_score":5,"trip_creation_score":5,"itinerary_quality_score":5,"persona_handoff_score":5,"booking_parsing_score":5,"companion_mode_score":5,"would_use_again":"yes","narrative":"x"}}`,
			wantContains: "would_use_again must be a boolean",
		},
		{
			name:         "unknown status",
			body:         `{"persona_id":"R-02","persona_name":"x","status":"MAYBE","completed_steps":[],"bugs":[],"ux_issues":[],"ai_behavior_issues":[],"tool_failures":[],"feature_coverage":[],"usefulness_evaluation":{"overall_score":5,"trip_creation_score":5,"itinerary_quality_score":5,"persona_handoff_score":5,"booking_parsing_score":5,"companion_mode_score":5,"would_use_again":true,"narrative":"x"}}`,
			wantContains: "not one of the allowed values",
		},
		{
			name:         "null bugs array",
			body:         `{"persona_id":"R-02","persona_name":"x","status":"COMPLETED","completed_steps":[],"bugs":null,"ux_issues":[],"ai_behavior_issues":[],"tool_failures":[],"feature_coverage":[],"usefulness_evaluation":{"overall_score":5,"trip_creation_score":5,"itinerary_quality_score":5,"persona_handoff_score":5,"booking_parsing_score":5,"companion_mode_score":5,"would_use_again":true,"narrative":"x"}}`,
			wantContains: "bugs must be an array",
		},
		{
			name:         "bug missing severity",
			body:         `{"persona_id":"R-02","persona_name":"x","status":"COMPLETED","completed_steps":[],"bugs":[{"title":"x","description":"y"}],"ux_issues":[],"ai_behavior_issues":[],"tool_failures":[],"feature_coverage":[],"usefulness_evaluation":{"overall_score":5,"trip_creation_score":5,"itinerary_quality_score":5,"persona_handoff_score":5,"booking_parsing_score":5,"companion_mode_score":5,"would_use_again":true,"narrative":"x"}}`,
			wantContains: "bugs[0].severity is required",
		},
		{
			name:         "missing narrative",
			body:         `{"persona_id":"R-02","persona_name":"x","status":"COMPLETED","completed_steps":[],"bugs":[],"ux_issues":[],"ai_behavior_issues":[],"tool_failures":[],"feature_coverage":[],"usefulness_evaluation":{"overall_score":5,"trip_creation_score":5,"itinerary_quality_score":5,"persona_handoff_score":5,"booking_parsing_score":5,"companion_mode_score":5,"would_use_again":true}}`,
			wantContains: "narrative is required",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var m map[string]any
			if err := json.Unmarshal([]byte(c.body), &m); err != nil {
				t.Fatalf("setup: %v", err)
			}
			errs, _ := validateReportMap(m)
			if len(errs) == 0 {
				t.Fatalf("expected validation errors, got none")
			}
			found := false
			for _, e := range errs {
				if strings.Contains(e, c.wantContains) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected error containing %q, got: %v", c.wantContains, errs)
			}
		})
	}
}
