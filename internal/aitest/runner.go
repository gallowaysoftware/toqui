//go:build aitest

package aitest

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
)

// ScenarioRunner executes test scenarios and collects results into a report.
type ScenarioRunner struct {
	env       *TestEnv
	evaluator *Evaluator
	report    *Report
}

// NewRunner creates a scenario runner with the given environment.
func NewRunner(env *TestEnv) *ScenarioRunner {
	return &ScenarioRunner{
		env:       env,
		evaluator: NewEvaluator(env.Provider),
		report:    NewReport(env.ProviderName),
	}
}

// Report returns the accumulated report for all executed scenarios.
func (r *ScenarioRunner) Report() *Report {
	return r.report
}

// Run executes a single scenario and records results in the report.
func (r *ScenarioRunner) Run(ctx context.Context, t *testing.T, scenario *TestScenario) *ScenarioReport {
	t.Helper()

	scenReport := &ScenarioReport{
		Name:      scenario.Name,
		Tags:      scenario.Tags,
		StartedAt: time.Now(),
	}

	slog.Info("aitest: running scenario", "scenario", scenario.Name, "steps", len(scenario.Steps))

	// Create isolated test user
	userID, err := r.env.CreateTestUser(ctx, scenario.UserName, scenario.UserEmail)
	if err != nil {
		t.Fatalf("create test user for %s: %v", scenario.Name, err)
	}

	state := &ScenarioState{
		UserID: userID,
		Trips:  make(map[string]TripInfo),
	}

	// Cleanup on exit
	t.Cleanup(func() {
		r.env.CleanupUser(context.Background(), userID)
	})

	// Run optional setup
	if scenario.Setup != nil {
		if err := scenario.Setup(ctx, r.env, state); err != nil {
			t.Fatalf("setup for %s: %v", scenario.Name, err)
		}
	}

	// Build conversation context for evaluator (accumulates across steps)
	var conversationLog string

	// Execute each step
	for i, step := range scenario.Steps {
		stepReport := r.runStep(ctx, t, &step, state, scenario.Name, i, &conversationLog)
		scenReport.Steps = append(scenReport.Steps, *stepReport)
	}

	// Compute pass/fail
	scenReport.Duration = time.Since(scenReport.StartedAt)
	scenReport.computePassRate()

	slog.Info("aitest: scenario complete",
		"scenario", scenario.Name,
		"passed", scenReport.Passed,
		"duration", scenReport.Duration.Round(time.Millisecond),
	)

	r.report.AddScenario(scenReport)
	return scenReport
}

func (r *ScenarioRunner) runStep(ctx context.Context, t *testing.T, step *TestStep, state *ScenarioState, scenarioName string, stepIdx int, conversationLog *string) *StepReport {
	t.Helper()

	stepReport := &StepReport{
		Name:      step.Name,
		StartedAt: time.Now(),
	}

	slog.Debug("aitest: running step", "scenario", scenarioName, "step", step.Name)

	// Execute with timeout
	stepCtx, cancel := context.WithTimeout(ctx, step.timeout())
	defer cancel()

	result, err := step.Action.Execute(stepCtx, r.env, state)
	if err != nil {
		// Execute returning an error means infrastructure failure (not an assertion failure)
		stepReport.Error = err.Error()
		stepReport.Duration = time.Since(stepReport.StartedAt)
		slog.Error("aitest: step execution failed", "scenario", scenarioName, "step", step.Name, "error", err)
		t.Errorf("[%s/%s] execution error: %v", scenarioName, step.Name, err)
		return stepReport
	}

	result.StepName = step.Name
	state.StepResults = append(state.StepResults, result)

	stepReport.Duration = result.Duration
	stepReport.FullResponse = result.FullResponse
	stepReport.ToolCalls = result.ToolCalls
	if result.Error != nil {
		stepReport.Error = result.Error.Error()
	}

	// Update conversation log for evaluator
	if sma, ok := step.Action.(*SendMessageAction); ok {
		*conversationLog += fmt.Sprintf("\n[USER (%s mode)]: %s\n", sma.Mode, sma.Content)
		if result.FullResponse != "" {
			*conversationLog += fmt.Sprintf("[ASSISTANT]: %s\n", result.FullResponse)
		}
		for _, tc := range result.ToolCalls {
			*conversationLog += fmt.Sprintf("[TOOL CALL]: %s(%s)\n", tc.Name, tc.Input)
			if tc.Result != "" {
				*conversationLog += fmt.Sprintf("[TOOL RESULT]: %s\n", tc.Result)
			}
		}
	}

	// Run structural assertions
	for _, assertion := range step.Assertions {
		ar := assertion.Check(result, state)
		stepReport.Assertions = append(stepReport.Assertions, AssertionReport{
			Name:     assertion.Name,
			Passed:   ar.Passed,
			Message:  ar.Message,
			Severity: ar.Severity,
		})
		if !ar.Passed && ar.Severity == "error" {
			t.Errorf("[%s/%s] FAIL %s: %s", scenarioName, step.Name, assertion.Name, ar.Message)
		} else if !ar.Passed {
			t.Logf("[%s/%s] WARN %s: %s", scenarioName, step.Name, assertion.Name, ar.Message)
		} else {
			t.Logf("[%s/%s] PASS %s", scenarioName, step.Name, assertion.Name)
		}
	}

	// Run LLM evaluations (informational — don't fail the test)
	for _, criterion := range step.EvalCriteria {
		evalResult := r.evaluator.Evaluate(ctx, result, criterion, *conversationLog)
		stepReport.Evaluations = append(stepReport.Evaluations, EvalReport{
			Criterion: criterion.Name,
			Score:     evalResult.Score,
			Reasoning: evalResult.Reasoning,
			Passed:    evalResult.Passed,
		})
		emoji := "⭐"
		if !evalResult.Passed {
			emoji = "⚠️"
		}
		t.Logf("[%s/%s] %s %s: %d/5 — %s", scenarioName, step.Name, emoji, criterion.Name, evalResult.Score, evalResult.Reasoning)
	}

	stepReport.computePassed()
	return stepReport
}

// RunGenerative creates and runs LLM-generated scenarios.
func (r *ScenarioRunner) RunGenerative(ctx context.Context, t *testing.T, count int) {
	t.Helper()

	slog.Info("aitest: generating scenarios", "count", count)
	builder := NewGenerativeBuilder(r.env.Provider)
	scenarios, err := builder.Generate(ctx, count)
	if err != nil {
		slog.Error("aitest: failed to generate scenarios", "error", err)
		t.Logf("Failed to generate scenarios: %v", err)
		return
	}
	slog.Info("aitest: generated scenarios", "count", len(scenarios))

	for _, scenario := range scenarios {
		// Assign unique user
		scenario.UserEmail = fmt.Sprintf("gen-%s@toqui-test.local", uuid.New().String()[:8])
		scenario.UserName = "Generated Test User"

		t.Run("generative/"+scenario.Name, func(t *testing.T) {
			report := r.Run(ctx, t, scenario)
			// Generative failures are informational, not hard errors
			if !report.Passed {
				t.Logf("generative scenario %s: %.0f%% pass rate", scenario.Name, report.PassRate*100)
			}
		})
	}
}
