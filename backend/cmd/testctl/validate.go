package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
)

// validateReport performs a lightweight JSON schema check on a single
// agent report file. This is a shallow validator — we don't pull in a
// full JSON Schema library because every required field and type is
// simple enough to check directly in Go. The authoritative schema
// lives in tests/agentic/report-schema.json for humans and agents to
// read.
//
//	testctl validate-report --file tmp/r-02.json
//	testctl validate-report --file tmp/r-02.json --strict
func validateReport(args []string) {
	fs := flag.NewFlagSet("validate-report", flag.ExitOnError)
	file := fs.String("file", "", "report JSON file to validate (required)")
	strict := fs.Bool("strict", false, "treat warnings as errors")
	if err := fs.Parse(args); err != nil {
		log.Fatal(err)
	}
	if *file == "" {
		log.Fatal("--file is required")
	}

	data, err := os.ReadFile(*file)
	if err != nil {
		log.Fatalf("read %s: %v", *file, err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		fmt.Fprintf(os.Stderr, "INVALID: file is not valid JSON: %v\n", err)
		os.Exit(1)
	}

	errs, warns := validateReportMap(raw)
	for _, e := range errs {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", e)
	}
	for _, w := range warns {
		fmt.Fprintf(os.Stderr, "WARN: %s\n", w)
	}

	if len(errs) > 0 || (*strict && len(warns) > 0) {
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "OK")
}

var personaIDRe = regexp.MustCompile(`^[RN]-[0-9]{2}$`)

var validStatuses = map[string]struct{}{
	"COMPLETED":                 {},
	"PARTIAL":                   {},
	"ABORTED_IDENTITY_MISMATCH": {},
	"ABORTED_SERVER_DOWN":       {},
	"ABORTED_BUDGET_EXHAUSTED":  {},
	"ABORTED_OTHER":             {},
}

var validSeverities = map[string]struct{}{
	"P0": {}, "P1": {}, "P2": {},
}

// validateReportMap returns (errors, warnings). Errors are schema
// violations that MUST be fixed; warnings are shape issues that the
// orchestrator can work around.
func validateReportMap(m map[string]any) (errors []string, warnings []string) {
	required := []string{
		"persona_id", "persona_name", "status", "completed_steps",
		"bugs", "ux_issues", "ai_behavior_issues", "tool_failures",
		"usefulness_evaluation", "feature_coverage",
	}
	for _, key := range required {
		if _, ok := m[key]; !ok {
			errors = append(errors, fmt.Sprintf("missing required key: %s", key))
		}
	}

	if id, ok := m["persona_id"].(string); ok {
		if !personaIDRe.MatchString(id) {
			errors = append(errors, fmt.Sprintf("persona_id %q does not match ^[RN]-[0-9]{2}$", id))
		}
	} else if _, present := m["persona_id"]; present {
		errors = append(errors, "persona_id must be a string")
	}

	if name, ok := m["persona_name"].(string); ok {
		if name == "" {
			errors = append(errors, "persona_name must be non-empty")
		}
	} else if _, present := m["persona_name"]; present {
		errors = append(errors, "persona_name must be a string")
	}

	if status, ok := m["status"].(string); ok {
		if _, valid := validStatuses[status]; !valid {
			errors = append(errors, fmt.Sprintf("status %q is not one of the allowed values", status))
		}
	} else if _, present := m["status"]; present {
		errors = append(errors, "status must be a string")
	}

	checkArray := func(key string) {
		if v, ok := m[key]; ok {
			if _, isArr := v.([]any); !isArr {
				errors = append(errors, fmt.Sprintf("%s must be an array (use [] for empty, not null)", key))
			}
		}
	}
	for _, key := range []string{"completed_steps", "bugs", "ux_issues", "ai_behavior_issues", "tool_failures", "feature_coverage"} {
		checkArray(key)
	}

	if bugs, ok := m["bugs"].([]any); ok {
		for i, b := range bugs {
			bugMap, ok := b.(map[string]any)
			if !ok {
				errors = append(errors, fmt.Sprintf("bugs[%d] is not an object", i))
				continue
			}
			for _, f := range []string{"severity", "title", "description"} {
				if _, has := bugMap[f]; !has {
					errors = append(errors, fmt.Sprintf("bugs[%d].%s is required", i, f))
				}
			}
			if sev, ok := bugMap["severity"].(string); ok {
				if _, valid := validSeverities[sev]; !valid {
					errors = append(errors, fmt.Sprintf("bugs[%d].severity %q must be P0/P1/P2", i, sev))
				}
			}
		}
	}

	if use, ok := m["usefulness_evaluation"].(map[string]any); ok {
		scoreFields := []string{
			"overall_score", "trip_creation_score", "itinerary_quality_score",
			"persona_handoff_score", "booking_parsing_score", "companion_mode_score",
		}
		for _, f := range scoreFields {
			v, has := use[f]
			if !has {
				errors = append(errors, fmt.Sprintf("usefulness_evaluation.%s is required", f))
				continue
			}
			n, ok := v.(float64) // json.Unmarshal decodes numbers as float64
			if !ok {
				errors = append(errors, fmt.Sprintf("usefulness_evaluation.%s must be a number", f))
				continue
			}
			if n < 0 || n > 5 {
				errors = append(errors, fmt.Sprintf("usefulness_evaluation.%s=%v must be 0..5", f, n))
			}
			if n != float64(int(n)) {
				warnings = append(warnings, fmt.Sprintf("usefulness_evaluation.%s=%v should be an integer", f, n))
			}
		}
		if _, has := use["would_use_again"]; !has {
			errors = append(errors, "usefulness_evaluation.would_use_again is required")
		} else if _, ok := use["would_use_again"].(bool); !ok {
			errors = append(errors, "usefulness_evaluation.would_use_again must be a boolean (not a string)")
		}
		if narr, has := use["narrative"].(string); has {
			if strings.TrimSpace(narr) == "" {
				errors = append(errors, "usefulness_evaluation.narrative must be non-empty")
			}
		} else if _, present := use["narrative"]; present {
			errors = append(errors, "usefulness_evaluation.narrative must be a string")
		} else {
			errors = append(errors, "usefulness_evaluation.narrative is required")
		}
	} else if _, present := m["usefulness_evaluation"]; present {
		errors = append(errors, "usefulness_evaluation must be an object")
	}

	return errors, warnings
}
