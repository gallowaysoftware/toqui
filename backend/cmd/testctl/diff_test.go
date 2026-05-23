package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeJSON(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestLoadRun_Shapes verifies that loadRun accepts all three supported
// input formats (wrapped object, bare array, single report).
func TestLoadRun_Shapes(t *testing.T) {
	dir := t.TempDir()

	wrapped := writeJSON(t, dir, "wrapped.json", `{
		"run_id": "run-6",
		"reports": [
			{"persona_id":"R-02","persona_name":"x","status":"COMPLETED","completed_steps":[],"bugs":[],"ux_issues":[],"ai_behavior_issues":[],"tool_failures":[],"feature_coverage":[],"usefulness_evaluation":{"overall_score":5,"trip_creation_score":5,"itinerary_quality_score":5,"persona_handoff_score":5,"booking_parsing_score":5,"companion_mode_score":5,"would_use_again":true,"narrative":"x"}}
		]
	}`)
	arr := writeJSON(t, dir, "arr.json", `[
		{"persona_id":"R-02","persona_name":"x","status":"COMPLETED","completed_steps":[],"bugs":[],"ux_issues":[],"ai_behavior_issues":[],"tool_failures":[],"feature_coverage":[],"usefulness_evaluation":{"overall_score":5,"trip_creation_score":5,"itinerary_quality_score":5,"persona_handoff_score":5,"booking_parsing_score":5,"companion_mode_score":5,"would_use_again":true,"narrative":"x"}}
	]`)
	single := writeJSON(t, dir, "single.json", `{"persona_id":"R-02","persona_name":"x","status":"COMPLETED","completed_steps":[],"bugs":[],"ux_issues":[],"ai_behavior_issues":[],"tool_failures":[],"feature_coverage":[],"usefulness_evaluation":{"overall_score":5,"trip_creation_score":5,"itinerary_quality_score":5,"persona_handoff_score":5,"booking_parsing_score":5,"companion_mode_score":5,"would_use_again":true,"narrative":"x"}}`)

	for _, path := range []string{wrapped, arr, single} {
		run, err := loadRun(path)
		if err != nil {
			t.Errorf("loadRun(%s) failed: %v", path, err)
			continue
		}
		if len(run.Reports) != 1 {
			t.Errorf("loadRun(%s): expected 1 report, got %d", path, len(run.Reports))
		}
		if run.Reports[0].PersonaID != "R-02" {
			t.Errorf("loadRun(%s): wrong persona_id: %s", path, run.Reports[0].PersonaID)
		}
	}
}

// TestBugKey locks in the dedupe key so fuzzy-matching bug titles
// across runs doesn't silently hide new bugs that look similar to
// old ones. Same persona + same severity + same first-60-chars of
// title should collapse; anything else should not.
func TestBugKey(t *testing.T) {
	same := []AgentBug{
		{Severity: "P1", Title: "fabrication regressed"},
		{Severity: "P1", Title: "Fabrication Regressed"},         // case-insensitive
		{Severity: "P1", Title: "fabrication regressed in R-02"}, // first 60 chars match
	}
	base := bugKey("R-02", same[0])
	for i, b := range same {
		if got := bugKey("R-02", b); !strings.HasPrefix(got, "R-02|P1|fabrication regressed") {
			t.Errorf("same[%d]=%q produced unexpected key: %s", i, b.Title, got)
		}
		_ = base
	}

	different := []AgentBug{
		{Severity: "P0", Title: "fabrication regressed"}, // different severity
		{Severity: "P1", Title: "completely new bug"},    // different title
	}
	k0 := bugKey("R-02", different[0])
	k1 := bugKey("R-02", different[1])
	if k0 == base {
		t.Errorf("P0 and P1 with same title must produce distinct keys")
	}
	if k1 == base {
		t.Errorf("different titles must produce distinct keys")
	}

	// Different persona but same bug = different key.
	if bugKey("R-03", same[0]) == base {
		t.Errorf("persona_id must be part of the key")
	}
}
