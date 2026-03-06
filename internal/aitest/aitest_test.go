//go:build aitest

package aitest

import (
	"context"
	"flag"
	"os"
	"strings"
	"testing"
	"time"
)

var (
	flagScenario   = flag.String("scenario", "", "Run only scenarios matching this substring (empty = all)")
	flagGenerative = flag.Bool("generative", false, "Also run LLM-generated exploratory scenarios")
	flagGenCount   = flag.Int("gen-count", 3, "Number of generative scenarios to create")
	flagReportDir  = flag.String("report-dir", "testdata/aitest-reports", "Directory for JSON report output")
)

func TestAIScenarios(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	env := NewTestEnv(t)
	runner := NewRunner(env)

	// Register all regression scenarios
	allScenarios := []*TestScenario{
		AliceBackpackerLifecycle(),
		BobFamilyPlanner(),
		CarolReturningUser(),
		UpdateRegressionTests(),
		DaveItineraryAndHandoff(),
		EveExpandedProfiles(),
	}

	// Filter by name if specified
	for _, scenario := range allScenarios {
		if *flagScenario != "" && !strings.Contains(scenario.Name, *flagScenario) {
			continue
		}

		s := scenario // capture loop var
		t.Run(s.Name, func(t *testing.T) {
			report := runner.Run(ctx, t, s)
			if !report.Passed {
				t.Errorf("scenario %s failed: %.0f%% assertions passed", s.Name, report.PassRate*100)
			}
		})
	}

	// Generative scenarios (opt-in)
	if *flagGenerative {
		t.Run("generative", func(t *testing.T) {
			runner.RunGenerative(ctx, t, *flagGenCount)
		})
	}

	// Write report
	reportPath, err := runner.Report().WriteJSON(*flagReportDir)
	if err != nil {
		t.Logf("Failed to write report: %v", err)
	} else {
		t.Logf("Report written to %s", reportPath)
	}
	runner.Report().PrintSummary()

	// Also write to a well-known "latest" path for quick access
	latestPath := *flagReportDir + "/latest.json"
	if data, err := os.ReadFile(reportPath); err == nil {
		_ = os.WriteFile(latestPath, data, 0644)
	}
}
