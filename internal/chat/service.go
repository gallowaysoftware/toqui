package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/google/uuid"

	"github.com/gallowaysoftware/toqui-backend/internal/ai"
	"github.com/gallowaysoftware/toqui-backend/internal/ai/tools"
	"github.com/gallowaysoftware/toqui-backend/internal/chatstore"
	"github.com/gallowaysoftware/toqui-backend/internal/persona"
)

type Service struct {
	provider  ai.Provider
	chatStore *chatstore.Store
	tools     *tools.Registry
	personas  *persona.Registry
	cache     *ai.ResponseCache // nil when caching is disabled
	budget    *ai.TokenBudget   // nil when budget is unlimited
}

func NewService(provider ai.Provider, chatStore *chatstore.Store, toolRegistry *tools.Registry, personas *persona.Registry) *Service {
	return &Service{
		provider:  provider,
		chatStore: chatStore,
		tools:     toolRegistry,
		personas:  personas,
	}
}

// SetCache enables LLM response caching. Pass nil to disable.
func (s *Service) SetCache(cache *ai.ResponseCache) {
	s.cache = cache
}

// SetBudget enables daily token budget tracking. Pass nil to disable.
func (s *Service) SetBudget(budget *ai.TokenBudget) {
	s.budget = budget
}

type StreamEvent struct {
	Type       string
	Text       string
	ToolName   string
	ToolInput  string
	ToolResult string
	MessageID  string
	SessionID  string
	Error      string
	// For trip_created events
	TripID          string
	TripTitle       string
	TripDescription string
}

type SendMessageParams struct {
	UserID             uuid.UUID
	TripID             string
	SessionID          string
	Content            string
	Mode               string
	PersonaID          string   // Optional: override active persona by ID
	DestinationCountry string   // For persona resolution
	TripThemes         []string // For persona resolution

	// LocationContext is the user's current lat/lng for companion mode.
	// PRIVACY: This is ephemeral — injected into the AI request as context
	// but NEVER stored in chat messages, Firestore, or any persistent storage.
	LocationLat float64
	LocationLng float64

	// ExtraTools are additional tools available for this request (e.g., create_trip in selection mode)
	ExtraTools []tools.Tool
	// ExtraSystemContext is appended to the system prompt (e.g., trip list for selection mode)
	ExtraSystemContext string
}

func (s *Service) SendMessage(ctx context.Context, params SendMessageParams) (<-chan StreamEvent, string, error) {
	sessionID := params.SessionID

	// Use "_lobby" as the Firestore trip path for selection mode (no trip)
	storeTripID := params.TripID
	if storeTripID == "" {
		storeTripID = "_lobby"
	}

	// Create session if needed
	if sessionID == "" {
		session, err := s.chatStore.CreateSession(ctx, params.UserID.String(), storeTripID, params.Mode)
		if err != nil {
			return nil, "", fmt.Errorf("create session: %w", err)
		}
		sessionID = session.ID
	}

	// Resolve persona: explicit ID > trip context resolution > default (Toqui)
	var activePersona *persona.Persona
	if params.PersonaID != "" {
		p, err := s.personas.Get(params.PersonaID)
		if err == nil {
			activePersona = p
		}
	}
	if activePersona == nil && params.DestinationCountry != "" && len(params.TripThemes) > 0 {
		p, err := s.personas.Resolve(ctx, params.DestinationCountry, params.TripThemes)
		if err == nil {
			activePersona = p
		}
	}
	if activePersona == nil {
		activePersona = s.personas.Default()
	}

	// Store user message
	userMsg := &chatstore.ChatMessage{
		Role:    "user",
		Content: params.Content,
	}
	if err := s.chatStore.AddMessage(ctx, params.UserID.String(), storeTripID, sessionID, userMsg); err != nil {
		return nil, "", fmt.Errorf("store user message: %w", err)
	}

	// Load history
	history, err := s.chatStore.GetMessages(ctx, params.UserID.String(), storeTripID, sessionID, 50)
	if err != nil {
		return nil, "", fmt.Errorf("load history: %w", err)
	}

	// Build AI request — reconstruct full messages including tool call/result data
	messages := make([]ai.Message, 0, len(history))
	for _, msg := range history {
		m := ai.Message{
			Role:    msg.Role,
			Content: msg.Content,
		}
		for _, tc := range msg.ToolCalls {
			m.ToolCalls = append(m.ToolCalls, ai.ToolCall{
				ID:        tc.ID,
				Name:      tc.Name,
				Arguments: tc.Arguments,
			})
		}
		for _, tr := range msg.ToolResults {
			m.ToolResults = append(m.ToolResults, ai.ToolResult{
				ToolCallID: tr.ToolCallID,
				Name:       tr.Name,
				Content:    tr.Content,
			})
		}
		messages = append(messages, m)
	}

	systemPrompt := activePersona.SystemPrompt(params.Mode)

	// Inject ephemeral location context — NEVER stored
	if params.LocationLat != 0 && params.LocationLng != 0 {
		systemPrompt += fmt.Sprintf("\n\nThe user's current location is approximately: %.4f, %.4f. Use this to provide relevant nearby recommendations. Do NOT repeat these coordinates back to the user.", params.LocationLat, params.LocationLng)
	}

	// Inject extra system context (e.g., trip list for selection mode)
	if params.ExtraSystemContext != "" {
		systemPrompt += "\n\n" + params.ExtraSystemContext
	}

	// Merge tool definitions
	toolDefs := s.tools.Definitions()
	for _, t := range params.ExtraTools {
		toolDefs = append(toolDefs, t.Definition())
	}

	aiReq := &ai.ChatRequest{
		SystemPrompt: systemPrompt,
		Messages:     messages,
		Tools:        toolDefs,
		Temperature:  0.7,
		Mode:         params.Mode,
	}

	// Classify the request to determine model tier. This uses deterministic
	// heuristics based on mode, message length, and tool availability.
	tier := ai.ClassifyRequest(aiReq)
	aiReq.ModelTier = tier

	// Apply tier-specific max tokens from model config.
	tierCfg := ai.ConfigForTier(tier)
	aiReq.MaxTokens = tierCfg.MaxTokens

	slog.Info("chat request classified",
		"mode", params.Mode,
		"tier", tier,
		"provider", s.provider.Name(),
		"has_tools", len(toolDefs) > 0,
	)

	// Check daily token budget before calling the LLM.
	if s.budget != nil {
		if err := s.budget.Check(); err != nil {
			return nil, "", err
		}
	}

	// Check response cache before calling the LLM.
	if s.cache != nil && s.cache.Eligible(aiReq) {
		if cached, ok := s.cache.Get(aiReq); ok {
			slog.Info("llm cache hit, returning cached response",
				"mode", params.Mode,
				"msg_len", len(params.Content),
			)
			return s.syntheticCacheResponse(ctx, cached, params.UserID, storeTripID, sessionID), sessionID, nil
		}
	}

	// Build extra tools map for execution
	extraToolsMap := make(map[string]tools.Tool, len(params.ExtraTools))
	for _, t := range params.ExtraTools {
		extraToolsMap[t.Definition().Name] = t
	}

	outCh := make(chan StreamEvent, 64)
	go func() {
		defer close(outCh)
		responseText := s.processEventsWithToolLoop(ctx, aiReq, outCh, extraToolsMap, params.UserID, storeTripID, sessionID)

		// Cache the response after streaming completes (only for eligible requests).
		if s.cache != nil && s.cache.Eligible(aiReq) && responseText != "" {
			s.cache.Put(aiReq, responseText)
			slog.Debug("llm response cached",
				"mode", params.Mode,
				"response_len", len(responseText),
			)
		}
	}()

	return outCh, sessionID, nil
}

// maxToolLoopIterations prevents infinite tool call loops.
const maxToolLoopIterations = 5

// processEventsWithToolLoop handles the AI stream and implements the tool call
// continuation loop. When the AI stops for tool use, tool results are sent back
// and the AI continues generating. This loops until the AI produces a final
// response (stop_reason=end_turn) or the iteration limit is reached.
func (s *Service) processEventsWithToolLoop(ctx context.Context, aiReq *ai.ChatRequest, outCh chan<- StreamEvent, extraTools map[string]tools.Tool, userID uuid.UUID, tripID, sessionID string) string {
	var fullResponse strings.Builder
	var totalInputTokens, totalOutputTokens int

	for iteration := 0; iteration < maxToolLoopIterations; iteration++ {
		// Stop if the client disconnected.
		if ctx.Err() != nil {
			slog.Info("tool loop: client disconnected, stopping", "iteration", iteration)
			return fullResponse.String()
		}

		// Start (or continue) streaming
		eventCh, err := s.provider.ChatStream(ctx, aiReq)
		if err != nil {
			outCh <- StreamEvent{Type: "error", Error: fmt.Sprintf("start chat stream: %v", err)}
			return ""
		}

		// Process this turn's events
		turnText, toolCalls, toolResults, stopReason, turnUsage, streamErr := s.processOneTurn(ctx, eventCh, outCh, extraTools)
		fullResponse.WriteString(turnText)

		// Accumulate token usage across tool loop iterations.
		if turnUsage != nil {
			totalInputTokens += turnUsage.InputTokens
			totalOutputTokens += turnUsage.OutputTokens
		}

		if streamErr != nil {
			outCh <- StreamEvent{Type: "error", Error: streamErr.Error()}
			return ""
		}

		// If the AI stopped for tool use and we have results, continue the loop
		if stopReason == "tool_use" && len(toolCalls) > 0 {
			slog.Info("tool loop: continuing after tool use",
				"iteration", iteration+1,
				"tools_called", len(toolCalls),
			)

			// Append the assistant message (with text + tool_use blocks) to the request
			assistantMsg := ai.Message{
				Role:      "assistant",
				Content:   turnText,
				ToolCalls: toolCalls,
			}
			aiReq.Messages = append(aiReq.Messages, assistantMsg)

			// Append the user message with tool results
			toolResultMsg := ai.Message{
				Role:        "user",
				ToolResults: toolResults,
			}
			aiReq.Messages = append(aiReq.Messages, toolResultMsg)

			// Persist intermediate messages so history reconstruction includes tool data
			storedAssistant := &chatstore.ChatMessage{
				Role:    "assistant",
				Content: turnText,
			}
			for _, tc := range toolCalls {
				storedAssistant.ToolCalls = append(storedAssistant.ToolCalls, chatstore.StoredToolCall{
					ID:        tc.ID,
					Name:      tc.Name,
					Arguments: tc.Arguments,
				})
			}
			if err := s.chatStore.AddMessage(ctx, userID.String(), tripID, sessionID, storedAssistant); err != nil {
				slog.Error("failed to store tool call message", "error", err)
			}

			storedToolResult := &chatstore.ChatMessage{
				Role: "user",
			}
			for _, tr := range toolResults {
				storedToolResult.ToolResults = append(storedToolResult.ToolResults, chatstore.StoredToolResult{
					ToolCallID: tr.ToolCallID,
					Name:       tr.Name,
					Content:    tr.Content,
				})
			}
			if err := s.chatStore.AddMessage(ctx, userID.String(), tripID, sessionID, storedToolResult); err != nil {
				slog.Error("failed to store tool result message", "error", err)
			}

			continue // Next iteration of the tool loop
		}

		// AI finished (end_turn) — store and emit the final response
		responseText := fullResponse.String()
		assistantMsg := &chatstore.ChatMessage{
			Role:    "assistant",
			Content: responseText,
		}
		if err := s.chatStore.AddMessage(ctx, userID.String(), tripID, sessionID, assistantMsg); err != nil {
			slog.Error("failed to store assistant message", "error", err)
		}

		outCh <- StreamEvent{
			Type:      "message_complete",
			Text:      responseText,
			SessionID: sessionID,
			MessageID: assistantMsg.ID,
		}

		// Log usage with environment label for cost tracking.
		s.logUsage(iteration+1, totalInputTokens, totalOutputTokens)
		return responseText
	}

	// Hit iteration limit — store what we have
	slog.Warn("tool loop: hit max iterations", "max", maxToolLoopIterations)
	responseText := fullResponse.String()
	assistantMsg := &chatstore.ChatMessage{
		Role:    "assistant",
		Content: responseText,
	}
	if err := s.chatStore.AddMessage(ctx, userID.String(), tripID, sessionID, assistantMsg); err != nil {
		slog.Error("failed to store assistant message", "error", err)
	}

	outCh <- StreamEvent{
		Type:      "message_complete",
		Text:      responseText,
		SessionID: sessionID,
		MessageID: assistantMsg.ID,
	}

	s.logUsage(maxToolLoopIterations, totalInputTokens, totalOutputTokens)
	return responseText
}

// logUsage logs token usage with provider and environment labels for cost tracking,
// and records against the daily token budget if configured.
func (s *Service) logUsage(iterations, inputTokens, outputTokens int) {
	if inputTokens == 0 && outputTokens == 0 {
		return
	}
	totalTokens := inputTokens + outputTokens
	slog.Info("ai request completed",
		"provider", s.provider.Name(),
		"env", os.Getenv("TARGET_ENV"),
		"input_tokens", inputTokens,
		"output_tokens", outputTokens,
		"total_tokens", totalTokens,
		"tool_loop_iterations", iterations,
	)

	if s.budget != nil {
		s.budget.Record(totalTokens)
	}
}

// processOneTurn drains a single AI stream, executing any tool calls.
// Returns the text produced, tool calls made, tool results, stop reason, usage, and any error.
func (s *Service) processOneTurn(ctx context.Context, eventCh <-chan ai.Event, outCh chan<- StreamEvent, extraTools map[string]tools.Tool) (text string, toolCalls []ai.ToolCall, toolResults []ai.ToolResult, stopReason string, usage *ai.Usage, err error) {
	var turnText strings.Builder

	var turnUsage *ai.Usage

	for event := range eventCh {
		// Stop processing if the client disconnected.
		if ctx.Err() != nil {
			return turnText.String(), toolCalls, toolResults, "", turnUsage, ctx.Err()
		}

		switch event.Type {
		case ai.EventTextDelta:
			turnText.WriteString(event.Text)
			select {
			case outCh <- StreamEvent{Type: "text_delta", Text: event.Text}:
			case <-ctx.Done():
				return turnText.String(), toolCalls, toolResults, "", turnUsage, ctx.Err()
			}

		case ai.EventToolCall:
			if event.Tool != nil {
				select {
				case outCh <- StreamEvent{
					Type:      "tool_call",
					ToolName:  event.Tool.Name,
					ToolInput: event.Tool.Arguments,
				}:
				case <-ctx.Done():
					return turnText.String(), toolCalls, toolResults, "", turnUsage, ctx.Err()
				}

				// Track this tool call for the continuation message
				toolCalls = append(toolCalls, *event.Tool)

				// Execute tool — check extra tools first, then global registry
				var result json.RawMessage
				var execErr error
				if extra, ok := extraTools[event.Tool.Name]; ok {
					result, execErr = extra.Execute(ctx, json.RawMessage(event.Tool.Arguments))
				} else {
					result, execErr = s.tools.Execute(ctx, event.Tool.Name, []byte(event.Tool.Arguments))
				}

				var resultStr string
				if execErr != nil {
					resultStr = fmt.Sprintf(`{"error": %q}`, execErr.Error())
				} else {
					resultStr = string(result)
				}

				select {
				case outCh <- StreamEvent{
					Type:       "tool_result",
					ToolName:   event.Tool.Name,
					ToolResult: resultStr,
				}:
				case <-ctx.Done():
					return turnText.String(), toolCalls, toolResults, "", turnUsage, ctx.Err()
				}

				// Collect tool result for the continuation message
				toolResults = append(toolResults, ai.ToolResult{
					ToolCallID: event.Tool.ID,
					Name:       event.Tool.Name,
					Content:    resultStr,
				})
			}

		case ai.EventDone:
			stopReason = event.StopReason
			turnUsage = event.Usage
			return turnText.String(), toolCalls, toolResults, stopReason, turnUsage, nil

		case ai.EventError:
			select {
			case outCh <- StreamEvent{Type: "error", Error: event.Error.Error()}:
			case <-ctx.Done():
			}
			return turnText.String(), toolCalls, toolResults, "", turnUsage, event.Error
		}
	}

	// If we get here, the event channel closed without an EventDone. This can
	// happen if the AI provider stream terminates unexpectedly. Log a warning
	// so we can investigate, and return what we have.
	if stopReason == "" {
		slog.Warn("ai stream closed without EventDone",
			"text_len", turnText.Len(),
			"tool_calls", len(toolCalls),
		)
	}

	return turnText.String(), toolCalls, toolResults, stopReason, turnUsage, nil
}

// syntheticCacheResponse creates a channel that emits a cached response as if it
// were streamed from the LLM. It also stores the assistant message in the chat store.
func (s *Service) syntheticCacheResponse(ctx context.Context, cachedText string, userID uuid.UUID, tripID, sessionID string) <-chan StreamEvent {
	outCh := make(chan StreamEvent, 4)
	go func() {
		defer close(outCh)

		// Emit the full text as a single delta (no need to chunk for cached responses).
		outCh <- StreamEvent{Type: "text_delta", Text: cachedText}

		// Store the assistant message.
		assistantMsg := &chatstore.ChatMessage{
			Role:    "assistant",
			Content: cachedText,
		}
		if err := s.chatStore.AddMessage(ctx, userID.String(), tripID, sessionID, assistantMsg); err != nil {
			slog.Error("failed to store cached assistant message", "error", err)
		}

		outCh <- StreamEvent{
			Type:      "message_complete",
			Text:      cachedText,
			SessionID: sessionID,
			MessageID: assistantMsg.ID,
		}
	}()
	return outCh
}

// Personas returns the persona registry for use by handlers.
func (s *Service) Personas() *persona.Registry {
	return s.personas
}

func (s *Service) ListSessions(ctx context.Context, userID uuid.UUID, tripID string, limit int) ([]*chatstore.ChatSession, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return s.chatStore.ListSessions(ctx, userID.String(), tripID, limit)
}

func (s *Service) GetHistory(ctx context.Context, userID uuid.UUID, tripID, sessionID string, limit int) ([]*chatstore.ChatMessage, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	// If no session ID specified, fetch the latest session for the trip.
	// This prevents passing an empty string to Firestore Doc("") which has
	// undefined behavior.
	if sessionID == "" {
		sessions, err := s.chatStore.ListSessions(ctx, userID.String(), tripID, 1)
		if err != nil {
			return nil, fmt.Errorf("list sessions: %w", err)
		}
		if len(sessions) == 0 {
			// No sessions yet — return empty history
			return nil, nil
		}
		sessionID = sessions[0].ID
	}

	// Verify session exists and belongs to user
	_, err := s.chatStore.GetSession(ctx, userID.String(), tripID, sessionID)
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}
	return s.chatStore.GetMessages(ctx, userID.String(), tripID, sessionID, limit)
}
