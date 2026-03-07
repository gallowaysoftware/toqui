//go:build aitest

package aitest

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gallowaysoftware/toqui-backend/internal/affiliate"
	"github.com/gallowaysoftware/toqui-backend/internal/ai/tools"
	"github.com/gallowaysoftware/toqui-backend/internal/chat"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui-backend/internal/handlers"
	"github.com/gallowaysoftware/toqui-backend/internal/persona"
	"github.com/gallowaysoftware/toqui-backend/internal/tier"
)

// ─── Scenario & Step Types ──────────────────────────────────────────────────

// TestScenario is a complete test case: a named sequence of steps.
type TestScenario struct {
	Name        string
	Description string
	Tags        []string
	UserName    string // Display name, e.g., "Alice (Solo Backpacker)"
	UserEmail   string // e.g., "alice@toqui-test.local"
	// Setup runs before steps — use to pre-create trips, etc.
	Setup func(ctx context.Context, env *TestEnv, state *ScenarioState) error
	Steps []TestStep
}

// TestStep is a single interaction within a scenario.
type TestStep struct {
	Name         string
	Action       StepAction
	Assertions   []Assertion     // Structural checks (hard fail)
	EvalCriteria []EvalCriterion // LLM judge checks (informational)
	Timeout      time.Duration   // Default: 120s
}

func (s *TestStep) timeout() time.Duration {
	if s.Timeout > 0 {
		return s.Timeout
	}
	return 120 * time.Second
}

// ─── State ──────────────────────────────────────────────────────────────────

// ScenarioState tracks mutable state across steps in a scenario.
type ScenarioState struct {
	UserID           uuid.UUID
	CurrentTripID    uuid.UUID
	CurrentSessionID string
	CurrentMode      string              // tracks mode to detect mode changes
	ActivePersonaID  string              // tracks active persona across steps (like the frontend does)
	Trips            map[string]TripInfo // tripID -> info
	StepResults      []*StepResult
}

// TripInfo captures trip metadata for assertions.
type TripInfo struct {
	ID          string
	Title       string
	Description string
	Status      string
	Country     string
}

// ─── Step Results ───────────────────────────────────────────────────────────

// StepResult captures everything that happened during a step.
type StepResult struct {
	StepName       string
	StartedAt      time.Time
	Duration       time.Duration
	FullResponse   string
	ToolCalls      []ToolCallInfo
	TripCreated    *TripInfo
	TripSelected   *TripInfo
	SessionCreated string
	Error          error
	// For non-chat steps:
	TripState *TripInfo
	TripList  []TripInfo
	// Itinerary and persona events
	ItineraryItemsCreated int
	PersonaSwitched       bool
	PersonaSwitchedTo     string // expert persona name
}

// ToolCallInfo records a single tool invocation.
type ToolCallInfo struct {
	Name   string
	Input  string // raw JSON
	Result string // raw JSON
}

// ─── Step Actions ───────────────────────────────────────────────────────────

// StepAction is the interface for things a test step can do.
type StepAction interface {
	Execute(ctx context.Context, env *TestEnv, state *ScenarioState) (*StepResult, error)
}

// SendMessageAction sends a chat message through the full service stack.
type SendMessageAction struct {
	Content string
	Mode    string // "selection", "planning", "companion"
	// Override trip/session (empty = use state)
	TripID    string
	SessionID string
}

func (a *SendMessageAction) Execute(ctx context.Context, env *TestEnv, state *ScenarioState) (*StepResult, error) {
	result := &StepResult{StartedAt: time.Now()}

	tripID := a.TripID
	if tripID == "" && state.CurrentTripID != uuid.Nil {
		tripID = state.CurrentTripID.String()
	}

	// Don't inherit session across mode changes — Firestore paths differ per trip/mode
	sessionID := a.SessionID
	if sessionID == "" && a.Mode == state.CurrentMode && a.Mode != "selection" {
		sessionID = state.CurrentSessionID
	}
	state.CurrentMode = a.Mode

	params := chat.SendMessageParams{
		UserID:    state.UserID,
		TripID:    tripID,
		SessionID: sessionID,
		Content:   a.Content,
		Mode:      a.Mode,
		PersonaID: state.ActivePersonaID, // persist active persona across steps (like frontend does)
	}

	// Mutex-protected state for tool callbacks
	var mu sync.Mutex
	var itineraryItems []dbgen.ItineraryItem
	var pendingSwitch *personaSwitchEvent

	// Suggest expert tool is available in all modes (mirrors chat.go)
	suggestExpertTool := handlers.NewSuggestExpertTool(env.PersonaReg, "",
		func(previous, expert *persona.Persona, handoffMessage string) {
			mu.Lock()
			pendingSwitch = &personaSwitchEvent{
				ExpertID:       expert.ID,
				ExpertName:     expert.Name,
				HandoffMessage: handoffMessage,
			}
			mu.Unlock()
		},
	)

	// Replicate handler-level wiring depending on mode
	switch a.Mode {
	case "selection":
		// Inject create_trip + select_trip + suggest_expert tools
		var createdTrips, selectedTrips []tripEventInfo

		createTool := handlers.NewCreateTripTool(env.TripSvc, state.UserID, func(id, title, desc string) {
			mu.Lock()
			createdTrips = append(createdTrips, tripEventInfo{ID: id, Title: title, Description: desc})
			mu.Unlock()
		})
		selectTool := handlers.NewSelectTripTool(env.TripSvc, state.UserID, func(id, title, desc string) {
			mu.Lock()
			selectedTrips = append(selectedTrips, tripEventInfo{ID: id, Title: title, Description: desc})
			mu.Unlock()
		})
		linkBuilder := affiliate.NewLinkBuilder("", "", "")
		recommendTool := handlers.NewRecommendBookingTool(linkBuilder, tier.Free, nil)
		params.ExtraTools = []tools.Tool{createTool, selectTool, suggestExpertTool, recommendTool}
		params.ExtraSystemContext = env.BuildSelectionContext(ctx, state.UserID)

	case "planning", "companion":
		var destinationCountry string
		if tripID != "" {
			if tid, err := uuid.Parse(tripID); err == nil {
				if t, err := env.TripSvc.GetByID(ctx, state.UserID, tid); err == nil {
					var country, desc string
					if t.DestinationCountry.Valid {
						country = t.DestinationCountry.String
					}
					if t.Description.Valid {
						desc = t.Description.String
					}
					var themes []string
					if env.ThemeSvc != nil {
						themes, _ = env.ThemeSvc.GetTripThemes(ctx, tid)
					}
					params.ExtraSystemContext = BuildTripContext(t.Title, desc, country, themes)
					params.DestinationCountry = country
					params.TripThemes = themes
					destinationCountry = country
				}

				// Planning mode: inject itinerary creation tool
				itineraryTool := handlers.NewCreateItineraryTool(env.TripSvc, tid, func(items []dbgen.ItineraryItem) {
					mu.Lock()
					itineraryItems = append(itineraryItems, items...)
					mu.Unlock()
				})
				params.ExtraTools = append(params.ExtraTools, itineraryTool)
			}
		}

		// Update suggest_expert with the destination country
		suggestExpertTool = handlers.NewSuggestExpertTool(env.PersonaReg, destinationCountry,
			func(previous, expert *persona.Persona, handoffMessage string) {
				mu.Lock()
				pendingSwitch = &personaSwitchEvent{
					ExpertID:       expert.ID,
					ExpertName:     expert.Name,
					HandoffMessage: handoffMessage,
				}
				mu.Unlock()
			},
		)
		params.ExtraTools = append(params.ExtraTools, suggestExpertTool)

		// Booking recommendation tool (mirrors chat.go — free tier in tests)
		linkBuilder := affiliate.NewLinkBuilder("", "", "")
		recommendTool := handlers.NewRecommendBookingTool(linkBuilder, tier.Free, nil)
		params.ExtraTools = append(params.ExtraTools, recommendTool)
	}

	eventCh, newSessionID, err := env.ChatSvc.SendMessage(ctx, params)
	if err != nil {
		result.Error = err
		result.Duration = time.Since(result.StartedAt)
		return result, nil // return result, not error — let assertions handle it
	}

	if sessionID == "" {
		result.SessionCreated = newSessionID
		state.CurrentSessionID = newSessionID
	}

	// Drain events
	for event := range eventCh {
		switch event.Type {
		case "text_delta":
			// accumulate (message_complete will have the full text)
		case "tool_call":
			result.ToolCalls = append(result.ToolCalls, ToolCallInfo{
				Name:  event.ToolName,
				Input: event.ToolInput,
			})
		case "tool_result":
			if len(result.ToolCalls) > 0 {
				last := &result.ToolCalls[len(result.ToolCalls)-1]
				if last.Result == "" {
					last.Result = event.ToolResult
				}
			}
			// Check for trip created/selected from tool results
			if event.ToolName == "create_trip" {
				var toolRes map[string]string
				if err := json.Unmarshal([]byte(event.ToolResult), &toolRes); err == nil {
					ti := &TripInfo{
						ID:    toolRes["trip_id"],
						Title: toolRes["title"],
					}
					result.TripCreated = ti
					if tid, err := uuid.Parse(ti.ID); err == nil {
						state.CurrentTripID = tid
						state.Trips[ti.ID] = *ti
					}
				}
			}
			if event.ToolName == "select_trip" {
				var toolRes map[string]string
				if err := json.Unmarshal([]byte(event.ToolResult), &toolRes); err == nil {
					ti := &TripInfo{
						ID:    toolRes["trip_id"],
						Title: toolRes["title"],
					}
					result.TripSelected = ti
					if tid, err := uuid.Parse(ti.ID); err == nil {
						state.CurrentTripID = tid
						state.Trips[ti.ID] = *ti
					}
				}
			}
			if event.ToolName == "create_itinerary_items" {
				mu.Lock()
				result.ItineraryItemsCreated += len(itineraryItems)
				itineraryItems = nil
				mu.Unlock()
			}
			if event.ToolName == "suggest_expert" {
				mu.Lock()
				if pendingSwitch != nil {
					result.PersonaSwitched = true
					result.PersonaSwitchedTo = pendingSwitch.ExpertName
					// Persist active persona so subsequent steps use the expert
					// (mirrors how the frontend sends persona_id with each message)
					state.ActivePersonaID = pendingSwitch.ExpertID
					pendingSwitch = nil
				}
				mu.Unlock()
			}
		case "message_complete":
			result.FullResponse = event.Text
		case "error":
			result.Error = fmt.Errorf("stream error: %s", event.Error)
		}
	}

	result.Duration = time.Since(result.StartedAt)
	return result, nil
}

// UpdateTripAction updates the current trip's status/fields.
type UpdateTripAction struct {
	Title       string
	Description string
	Status      string // "planning", "active", "completed"
}

func (a *UpdateTripAction) Execute(ctx context.Context, env *TestEnv, state *ScenarioState) (*StepResult, error) {
	result := &StepResult{StartedAt: time.Now()}

	t, err := env.TripSvc.Update(ctx, state.UserID, state.CurrentTripID, a.Title, a.Description, a.Status, nil, nil)
	if err != nil {
		result.Error = err
		result.Duration = time.Since(result.StartedAt)
		return result, nil
	}

	ti := &TripInfo{
		ID:     t.ID.String(),
		Title:  t.Title,
		Status: t.Status,
	}
	if t.Description.Valid {
		ti.Description = t.Description.String
	}
	if t.DestinationCountry.Valid {
		ti.Country = t.DestinationCountry.String
	}
	result.TripState = ti
	state.Trips[ti.ID] = *ti
	result.Duration = time.Since(result.StartedAt)
	return result, nil
}

// VerifyTripAction fetches the current trip and stores it for assertions.
type VerifyTripAction struct{}

func (a *VerifyTripAction) Execute(ctx context.Context, env *TestEnv, state *ScenarioState) (*StepResult, error) {
	result := &StepResult{StartedAt: time.Now()}

	t, err := env.TripSvc.GetByID(ctx, state.UserID, state.CurrentTripID)
	if err != nil {
		result.Error = err
		result.Duration = time.Since(result.StartedAt)
		return result, nil
	}

	ti := &TripInfo{
		ID:     t.ID.String(),
		Title:  t.Title,
		Status: t.Status,
	}
	if t.Description.Valid {
		ti.Description = t.Description.String
	}
	if t.DestinationCountry.Valid {
		ti.Country = t.DestinationCountry.String
	}
	result.TripState = ti
	result.Duration = time.Since(result.StartedAt)
	return result, nil
}

// ListTripsAction fetches all trips for the user.
type ListTripsAction struct{}

func (a *ListTripsAction) Execute(ctx context.Context, env *TestEnv, state *ScenarioState) (*StepResult, error) {
	result := &StepResult{StartedAt: time.Now()}

	trips, _, err := env.TripSvc.ListByUser(ctx, state.UserID, "", 100, 0)
	if err != nil {
		result.Error = err
		result.Duration = time.Since(result.StartedAt)
		return result, nil
	}

	for _, t := range trips {
		ti := TripInfo{
			ID:     t.ID.String(),
			Title:  t.Title,
			Status: t.Status,
		}
		if t.Description.Valid {
			ti.Description = t.Description.String
		}
		if t.DestinationCountry.Valid {
			ti.Country = t.DestinationCountry.String
		}
		result.TripList = append(result.TripList, ti)
	}
	result.Duration = time.Since(result.StartedAt)
	return result, nil
}

// CreateTripAction directly creates a trip (for setup, not AI-driven).
type CreateTripAction struct {
	Title       string
	Description string
	Country     string // ISO country code
}

func (a *CreateTripAction) Execute(ctx context.Context, env *TestEnv, state *ScenarioState) (*StepResult, error) {
	result := &StepResult{StartedAt: time.Now()}

	t, err := env.TripSvc.Create(ctx, state.UserID, a.Title, a.Description, nil, nil)
	if err != nil {
		result.Error = err
		result.Duration = time.Since(result.StartedAt)
		return result, nil
	}

	if a.Country != "" {
		_ = env.TripSvc.SetDestination(ctx, t.ID, a.Country)
	}

	ti := &TripInfo{
		ID:          t.ID.String(),
		Title:       t.Title,
		Country:     a.Country,
		Status:      t.Status,
		Description: a.Description,
	}
	result.TripCreated = ti
	state.CurrentTripID = t.ID
	state.Trips[ti.ID] = *ti
	result.Duration = time.Since(result.StartedAt)
	return result, nil
}

type tripEventInfo struct {
	ID, Title, Description string
}

type personaSwitchEvent struct {
	ExpertID       string
	ExpertName     string
	HandoffMessage string
}

// ─── Assertions ─────────────────────────────────────────────────────────────

// Assertion checks a structural property of a StepResult.
type Assertion struct {
	Name  string
	Check func(result *StepResult, state *ScenarioState) AssertionResult
}

// AssertionResult is the outcome of running an assertion.
type AssertionResult struct {
	Passed   bool
	Message  string
	Severity string // "error" or "warn"
}

// EvalCriterion is a quality criterion scored by the LLM judge.
type EvalCriterion struct {
	Name        string
	Description string
	Weight      float64 // 0-1 for report aggregation
}

// ─── Assertion Constructors ─────────────────────────────────────────────────

// AssertToolCalled checks that a specific tool was invoked during the step.
func AssertToolCalled(toolName string) Assertion {
	return Assertion{
		Name: "tool_called:" + toolName,
		Check: func(r *StepResult, _ *ScenarioState) AssertionResult {
			for _, tc := range r.ToolCalls {
				if tc.Name == toolName {
					return AssertionResult{Passed: true, Message: fmt.Sprintf("tool %s was called", toolName), Severity: "error"}
				}
			}
			names := make([]string, len(r.ToolCalls))
			for i, tc := range r.ToolCalls {
				names[i] = tc.Name
			}
			return AssertionResult{
				Passed:   false,
				Message:  fmt.Sprintf("expected tool %s, got: [%s]", toolName, strings.Join(names, ", ")),
				Severity: "error",
			}
		},
	}
}

// AssertToolArgContains checks that a tool's input JSON contains a substring.
func AssertToolArgContains(toolName, substring string) Assertion {
	return Assertion{
		Name: fmt.Sprintf("tool_arg:%s contains %q", toolName, substring),
		Check: func(r *StepResult, _ *ScenarioState) AssertionResult {
			for _, tc := range r.ToolCalls {
				if tc.Name == toolName {
					if strings.Contains(strings.ToLower(tc.Input), strings.ToLower(substring)) {
						return AssertionResult{Passed: true, Message: fmt.Sprintf("tool %s args contain %q", toolName, substring), Severity: "error"}
					}
					return AssertionResult{Passed: false, Message: fmt.Sprintf("tool %s args %q do not contain %q", toolName, tc.Input, substring), Severity: "error"}
				}
			}
			return AssertionResult{Passed: false, Message: fmt.Sprintf("tool %s not called", toolName), Severity: "error"}
		},
	}
}

// AssertResponseContains checks that the AI response text contains a substring (case-insensitive).
func AssertResponseContains(substring string) Assertion {
	return Assertion{
		Name: fmt.Sprintf("response_contains:%q", substring),
		Check: func(r *StepResult, _ *ScenarioState) AssertionResult {
			if strings.Contains(strings.ToLower(r.FullResponse), strings.ToLower(substring)) {
				return AssertionResult{Passed: true, Message: fmt.Sprintf("response contains %q", substring), Severity: "error"}
			}
			preview := r.FullResponse
			if len(preview) > 200 {
				preview = preview[:200] + "..."
			}
			return AssertionResult{Passed: false, Message: fmt.Sprintf("response does not contain %q: %s", substring, preview), Severity: "error"}
		},
	}
}

// AssertResponseNotContains checks that the AI response does NOT contain a substring.
func AssertResponseNotContains(substring string) Assertion {
	return Assertion{
		Name: fmt.Sprintf("response_not_contains:%q", substring),
		Check: func(r *StepResult, _ *ScenarioState) AssertionResult {
			if !strings.Contains(strings.ToLower(r.FullResponse), strings.ToLower(substring)) {
				return AssertionResult{Passed: true, Message: fmt.Sprintf("response does not contain %q", substring), Severity: "error"}
			}
			return AssertionResult{Passed: false, Message: fmt.Sprintf("response unexpectedly contains %q", substring), Severity: "error"}
		},
	}
}

// AssertResponseNonEmpty checks that the AI produced a non-empty response.
func AssertResponseNonEmpty() Assertion {
	return Assertion{
		Name: "response_non_empty",
		Check: func(r *StepResult, _ *ScenarioState) AssertionResult {
			if strings.TrimSpace(r.FullResponse) != "" {
				return AssertionResult{Passed: true, Message: fmt.Sprintf("response is %d chars", len(r.FullResponse)), Severity: "error"}
			}
			return AssertionResult{Passed: false, Message: "response is empty", Severity: "error"}
		},
	}
}

// AssertResponseMinLength checks minimum response length.
func AssertResponseMinLength(minLen int) Assertion {
	return Assertion{
		Name: fmt.Sprintf("response_min_length:%d", minLen),
		Check: func(r *StepResult, _ *ScenarioState) AssertionResult {
			if len(r.FullResponse) >= minLen {
				return AssertionResult{Passed: true, Message: fmt.Sprintf("response is %d chars (min %d)", len(r.FullResponse), minLen), Severity: "error"}
			}
			return AssertionResult{Passed: false, Message: fmt.Sprintf("response is %d chars, expected at least %d", len(r.FullResponse), minLen), Severity: "error"}
		},
	}
}

// AssertNoErrors checks that no errors occurred during the step.
func AssertNoErrors() Assertion {
	return Assertion{
		Name: "no_errors",
		Check: func(r *StepResult, _ *ScenarioState) AssertionResult {
			if r.Error == nil {
				return AssertionResult{Passed: true, Message: "no errors", Severity: "error"}
			}
			return AssertionResult{Passed: false, Message: fmt.Sprintf("error: %v", r.Error), Severity: "error"}
		},
	}
}

// AssertTripStatus checks the trip's status after an update.
func AssertTripStatus(expected string) Assertion {
	return Assertion{
		Name: fmt.Sprintf("trip_status:%s", expected),
		Check: func(r *StepResult, _ *ScenarioState) AssertionResult {
			if r.TripState == nil {
				return AssertionResult{Passed: false, Message: "no trip state available", Severity: "error"}
			}
			if r.TripState.Status == expected {
				return AssertionResult{Passed: true, Message: fmt.Sprintf("trip status is %s", expected), Severity: "error"}
			}
			return AssertionResult{Passed: false, Message: fmt.Sprintf("trip status is %q, expected %q", r.TripState.Status, expected), Severity: "error"}
		},
	}
}

// AssertTripFieldNotEmpty checks that a trip field was preserved (not wiped by COALESCE bug).
func AssertTripFieldNotEmpty(field string) Assertion {
	return Assertion{
		Name: fmt.Sprintf("trip_field_not_empty:%s", field),
		Check: func(r *StepResult, _ *ScenarioState) AssertionResult {
			if r.TripState == nil {
				return AssertionResult{Passed: false, Message: "no trip state available", Severity: "error"}
			}
			var val string
			switch field {
			case "title":
				val = r.TripState.Title
			case "description":
				val = r.TripState.Description
			case "country":
				val = r.TripState.Country
			}
			if val != "" {
				return AssertionResult{Passed: true, Message: fmt.Sprintf("trip.%s = %q", field, val), Severity: "error"}
			}
			return AssertionResult{Passed: false, Message: fmt.Sprintf("trip.%s is empty (data loss!)", field), Severity: "error"}
		},
	}
}

// AssertTripCount checks the number of trips the user has.
func AssertTripCount(expected int) Assertion {
	return Assertion{
		Name: fmt.Sprintf("trip_count:%d", expected),
		Check: func(r *StepResult, _ *ScenarioState) AssertionResult {
			actual := len(r.TripList)
			if actual == expected {
				return AssertionResult{Passed: true, Message: fmt.Sprintf("user has %d trips", expected), Severity: "error"}
			}
			return AssertionResult{Passed: false, Message: fmt.Sprintf("user has %d trips, expected %d", actual, expected), Severity: "error"}
		},
	}
}

// AssertItineraryItemsCreated checks that at least minCount itinerary items were created.
func AssertItineraryItemsCreated(minCount int) Assertion {
	return Assertion{
		Name: fmt.Sprintf("itinerary_items_created:%d+", minCount),
		Check: func(r *StepResult, _ *ScenarioState) AssertionResult {
			if r.ItineraryItemsCreated >= minCount {
				return AssertionResult{Passed: true, Message: fmt.Sprintf("%d itinerary items created (min %d)", r.ItineraryItemsCreated, minCount), Severity: "error"}
			}
			return AssertionResult{Passed: false, Message: fmt.Sprintf("%d itinerary items created, expected at least %d", r.ItineraryItemsCreated, minCount), Severity: "error"}
		},
	}
}

// AssertPersonaSwitched checks that a persona switch occurred during the step.
func AssertPersonaSwitched() Assertion {
	return Assertion{
		Name: "persona_switched",
		Check: func(r *StepResult, _ *ScenarioState) AssertionResult {
			if r.PersonaSwitched {
				return AssertionResult{Passed: true, Message: fmt.Sprintf("persona switched to %s", r.PersonaSwitchedTo), Severity: "error"}
			}
			return AssertionResult{Passed: false, Message: "expected persona switch but none occurred", Severity: "error"}
		},
	}
}

// AssertPersonaNotSwitched checks that NO persona switch occurred.
func AssertPersonaNotSwitched() Assertion {
	return Assertion{
		Name: "persona_not_switched",
		Check: func(r *StepResult, _ *ScenarioState) AssertionResult {
			if !r.PersonaSwitched {
				return AssertionResult{Passed: true, Message: "no persona switch (expected)", Severity: "error"}
			}
			return AssertionResult{Passed: false, Message: fmt.Sprintf("unexpected persona switch to %s", r.PersonaSwitchedTo), Severity: "error"}
		},
	}
}

// AssertToolNotCalled checks that a specific tool was NOT invoked during the step.
func AssertToolNotCalled(toolName string) Assertion {
	return Assertion{
		Name: "tool_not_called:" + toolName,
		Check: func(r *StepResult, _ *ScenarioState) AssertionResult {
			for _, tc := range r.ToolCalls {
				if tc.Name == toolName {
					return AssertionResult{
						Passed:   false,
						Message:  fmt.Sprintf("tool %s was unexpectedly called with args: %s", toolName, tc.Input),
						Severity: "error",
					}
				}
			}
			return AssertionResult{Passed: true, Message: fmt.Sprintf("tool %s was not called (expected)", toolName), Severity: "error"}
		},
	}
}

// AssertToolResultContains checks that a tool's result JSON contains a substring (case-insensitive).
func AssertToolResultContains(toolName, substring string) Assertion {
	return Assertion{
		Name: fmt.Sprintf("tool_result:%s contains %q", toolName, substring),
		Check: func(r *StepResult, _ *ScenarioState) AssertionResult {
			for _, tc := range r.ToolCalls {
				if tc.Name == toolName {
					if strings.Contains(strings.ToLower(tc.Result), strings.ToLower(substring)) {
						return AssertionResult{Passed: true, Message: fmt.Sprintf("tool %s result contains %q", toolName, substring), Severity: "error"}
					}
					return AssertionResult{
						Passed:   false,
						Message:  fmt.Sprintf("tool %s result does not contain %q", toolName, substring),
						Severity: "error",
					}
				}
			}
			return AssertionResult{Passed: false, Message: fmt.Sprintf("tool %s not called", toolName), Severity: "error"}
		},
	}
}
