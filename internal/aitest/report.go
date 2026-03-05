//go:build aitest

package aitest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ─── Report Types ───────────────────────────────────────────────────────────

// Report is the top-level test report.
type Report struct {
	RunID     string           `json:"run_id"`
	Timestamp time.Time        `json:"timestamp"`
	Provider  string           `json:"provider"`
	Scenarios []ScenarioReport `json:"scenarios"`
	Summary   ReportSummary    `json:"summary"`
	mu        sync.Mutex
}

// ScenarioReport summarizes one scenario's execution.
type ScenarioReport struct {
	Name      string       `json:"name"`
	Tags      []string     `json:"tags"`
	StartedAt time.Time    `json:"started_at"`
	Duration  time.Duration `json:"duration_ms"`
	Steps     []StepReport `json:"steps"`
	Passed    bool         `json:"passed"`
	PassRate  float64      `json:"pass_rate"`
}

// StepReport summarizes one step's execution.
type StepReport struct {
	Name        string            `json:"name"`
	StartedAt   time.Time         `json:"started_at"`
	Duration    time.Duration     `json:"duration_ms"`
	Assertions  []AssertionReport `json:"assertions,omitempty"`
	Evaluations []EvalReport      `json:"evaluations,omitempty"`
	FullResponse string           `json:"full_response,omitempty"`
	ToolCalls   []ToolCallInfo    `json:"tool_calls,omitempty"`
	Error       string            `json:"error,omitempty"`
	Passed      bool              `json:"passed"`
}

// AssertionReport records one assertion's outcome.
type AssertionReport struct {
	Name     string `json:"name"`
	Passed   bool   `json:"passed"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
}

// EvalReport records one LLM evaluation's outcome.
type EvalReport struct {
	Criterion string `json:"criterion"`
	Score     int    `json:"score"`
	Reasoning string `json:"reasoning"`
	Passed    bool   `json:"passed"`
}

// ReportSummary aggregates stats across all scenarios.
type ReportSummary struct {
	TotalScenarios   int     `json:"total_scenarios"`
	PassedScenarios  int     `json:"passed_scenarios"`
	FailedScenarios  int     `json:"failed_scenarios"`
	TotalAssertions  int     `json:"total_assertions"`
	PassedAssertions int     `json:"passed_assertions"`
	TotalEvals       int     `json:"total_evals"`
	AvgEvalScore     float64 `json:"avg_eval_score"`
	TotalDuration    string  `json:"total_duration"`
}

// ─── Report Methods ─────────────────────────────────────────────────────────

// NewReport creates a fresh report.
func NewReport(providerName string) *Report {
	return &Report{
		RunID:     time.Now().Format("20060102-150405"),
		Timestamp: time.Now(),
		Provider:  providerName,
	}
}

// AddScenario thread-safely appends a scenario report.
func (r *Report) AddScenario(sr *ScenarioReport) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Scenarios = append(r.Scenarios, *sr)
}

// Finalize computes the summary from all scenarios.
func (r *Report) Finalize() {
	r.mu.Lock()
	defer r.mu.Unlock()

	var totalAssertions, passedAssertions, totalEvals int
	var evalScoreSum float64

	for _, s := range r.Scenarios {
		r.Summary.TotalScenarios++
		if s.Passed {
			r.Summary.PassedScenarios++
		} else {
			r.Summary.FailedScenarios++
		}

		for _, step := range s.Steps {
			for _, a := range step.Assertions {
				totalAssertions++
				if a.Passed {
					passedAssertions++
				}
			}
			for _, e := range step.Evaluations {
				totalEvals++
				evalScoreSum += float64(e.Score)
			}
		}
	}

	r.Summary.TotalAssertions = totalAssertions
	r.Summary.PassedAssertions = passedAssertions
	r.Summary.TotalEvals = totalEvals
	if totalEvals > 0 {
		r.Summary.AvgEvalScore = evalScoreSum / float64(totalEvals)
	}
	r.Summary.TotalDuration = time.Since(r.Timestamp).Round(time.Second).String()
}

// WriteJSON writes the full report to a JSON file.
func (r *Report) WriteJSON(dir string) (string, error) {
	r.Finalize()

	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create report dir: %w", err)
	}

	path := filepath.Join(dir, fmt.Sprintf("report-%s.json", r.RunID))
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal report: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", fmt.Errorf("write report: %w", err)
	}
	return path, nil
}

// PrintSummary writes a human-readable summary to stdout.
func (r *Report) PrintSummary() {
	r.Finalize()

	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Printf("  AI Test Report | %s | Provider: %s\n", r.RunID, r.Provider)
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println()

	for _, s := range r.Scenarios {
		status := "PASS"
		if !s.Passed {
			status = "FAIL"
		}
		dots := 60 - len(s.Name) - len(status)
		if dots < 3 {
			dots = 3
		}
		fmt.Printf("  %s %s %s (%s)\n", s.Name, strings.Repeat(".", dots), status, s.Duration.Round(time.Second))

		for _, step := range s.Steps {
			stepStatus := "PASS"
			if !step.Passed {
				stepStatus = "FAIL"
			}
			stepDots := 56 - len(step.Name) - len(stepStatus)
			if stepDots < 3 {
				stepDots = 3
			}
			fmt.Printf("    %s %s %s\n", step.Name, strings.Repeat(".", stepDots), stepStatus)

			for _, a := range step.Assertions {
				icon := "✅"
				if !a.Passed {
					icon = "❌"
				}
				fmt.Printf("      %s %s\n", icon, a.Name)
			}
			for _, e := range step.Evaluations {
				icon := "⭐"
				if !e.Passed {
					icon = "⚠️ "
				}
				fmt.Printf("      %s %s: %d/5 — %s\n", icon, e.Criterion, e.Score, e.Reasoning)
			}
		}
		fmt.Println()
	}

	fmt.Println("───────────────────────────────────────────────────────────────")
	fmt.Printf("  Scenarios: %d/%d passed", r.Summary.PassedScenarios, r.Summary.TotalScenarios)
	if r.Summary.TotalAssertions > 0 {
		fmt.Printf(" | Assertions: %d/%d", r.Summary.PassedAssertions, r.Summary.TotalAssertions)
	}
	if r.Summary.TotalEvals > 0 {
		fmt.Printf(" | Avg eval: %.1f/5", r.Summary.AvgEvalScore)
	}
	fmt.Printf(" | %s\n", r.Summary.TotalDuration)
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println()
}

// ─── Scenario/Step Helpers ──────────────────────────────────────────────────

func (s *ScenarioReport) computePassRate() {
	total := 0
	passed := 0
	for _, step := range s.Steps {
		for _, a := range step.Assertions {
			total++
			if a.Passed {
				passed++
			}
		}
	}
	if total > 0 {
		s.PassRate = float64(passed) / float64(total)
	} else {
		s.PassRate = 1.0 // no assertions = pass
	}
	s.Passed = s.PassRate == 1.0
}

func (s *StepReport) computePassed() {
	s.Passed = true
	for _, a := range s.Assertions {
		if !a.Passed && a.Severity == "error" {
			s.Passed = false
			return
		}
	}
}
