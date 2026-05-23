package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
)

// AgentReport is the Go representation of tests/agentic/report-schema.json.
// It's intentionally permissive in the Go struct — the authoritative
// validation happens against the JSON schema at the orchestrator level.
// Here we only parse the fields the testctl commands actually inspect.
type AgentReport struct {
	PersonaID       string             `json:"persona_id"`
	PersonaName     string             `json:"persona_name"`
	Status          string             `json:"status"`
	TripDestination string             `json:"trip_destination,omitempty"`
	CompletedSteps  []string           `json:"completed_steps"`
	Bugs            []AgentBug         `json:"bugs"`
	UXIssues        []AgentUX          `json:"ux_issues"`
	AIBehavior      []AgentAI          `json:"ai_behavior_issues"`
	ToolFailures    []AgentToolFailure `json:"tool_failures"`
	Usefulness      AgentUsefulness    `json:"usefulness_evaluation"`
	FeatureCoverage []string           `json:"feature_coverage"`
	RunID           string             `json:"run_id,omitempty"`
}

// AgentBug matches the `bugs[]` entry in the schema.
type AgentBug struct {
	Severity         string `json:"severity"`
	Title            string `json:"title"`
	Description      string `json:"description,omitempty"`
	StepsToReproduce string `json:"steps_to_reproduce,omitempty"`
	Expected         string `json:"expected,omitempty"`
	Actual           string `json:"actual,omitempty"`
	APICommand       string `json:"api_command,omitempty"`
}

// AgentUX matches the `ux_issues[]` entry.
type AgentUX struct {
	Description string `json:"description"`
	Severity    string `json:"severity,omitempty"`
	Suggestion  string `json:"suggestion,omitempty"`
}

// AgentAI matches the `ai_behavior_issues[]` entry.
type AgentAI struct {
	Issue    string `json:"issue"`
	Context  string `json:"context,omitempty"`
	Severity string `json:"severity,omitempty"`
}

// AgentToolFailure matches the `tool_failures[]` entry.
type AgentToolFailure struct {
	Tool  string `json:"tool"`
	Input string `json:"input,omitempty"`
	Error string `json:"error,omitempty"`
}

// AgentUsefulness matches the `usefulness_evaluation` object.
type AgentUsefulness struct {
	OverallScore        int    `json:"overall_score"`
	TripCreationScore   int    `json:"trip_creation_score"`
	ItineraryScore      int    `json:"itinerary_quality_score"`
	PersonaHandoffScore int    `json:"persona_handoff_score"`
	BookingScore        int    `json:"booking_parsing_score"`
	CompanionScore      int    `json:"companion_mode_score"`
	WouldUseAgain       bool   `json:"would_use_again"`
	Narrative           string `json:"narrative"`
}

// Run is a collection of AgentReports, typically one full 20-persona run.
type Run struct {
	RunID   string        `json:"run_id,omitempty"`
	Reports []AgentReport `json:"reports"`
}

// loadRun reads a run JSON file from disk. The file may be either:
//   - a top-level object with `{"run_id": ..., "reports": [...]}`
//   - a bare array of reports
//   - a single report object
//
// The loader normalises all three into a Run. If none of the three
// shapes match, the underlying JSON parse error (if any) is preserved
// so the caller can tell a malformed file from a shape mismatch.
func loadRun(path string) (*Run, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	// Try object with "reports" first. Capture the parse error so we
	// can surface it in the final error if nothing else matches.
	var firstParseErr error
	var wrapped Run
	if err := json.Unmarshal(data, &wrapped); err != nil {
		firstParseErr = err
	} else if wrapped.Reports != nil {
		return &wrapped, nil
	}

	// Try bare array.
	var arr []AgentReport
	if err := json.Unmarshal(data, &arr); err == nil {
		return &Run{Reports: arr}, nil
	}

	// Try single report.
	var single AgentReport
	if err := json.Unmarshal(data, &single); err == nil && single.PersonaID != "" {
		return &Run{Reports: []AgentReport{single}}, nil
	}

	if firstParseErr != nil {
		return nil, fmt.Errorf("parse %s as JSON: %w", path, firstParseErr)
	}
	return nil, errors.New("file is not a recognised run/report format (expected {reports: [...]} or [...] or a single report with persona_id)")
}

// reportByPersona builds a persona_id → report lookup from a Run.
func reportByPersona(run *Run) map[string]AgentReport {
	out := make(map[string]AgentReport, len(run.Reports))
	for _, r := range run.Reports {
		if r.PersonaID != "" {
			out[r.PersonaID] = r
		}
	}
	return out
}

// sortedPersonaIDs returns the union of persona IDs from both runs in
// lexicographic order so diff output is stable.
func sortedPersonaIDs(a, b map[string]AgentReport) []string {
	seen := make(map[string]struct{}, len(a)+len(b))
	for k := range a {
		seen[k] = struct{}{}
	}
	for k := range b {
		seen[k] = struct{}{}
	}
	ids := make([]string, 0, len(seen))
	for k := range seen {
		ids = append(ids, k)
	}
	sort.Strings(ids)
	return ids
}
