package chat

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ledongthuc/pdf"

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
	usageSvc  usageCostRecorder // nil when cost tracking is disabled
}

// usageCostRecorder is the subset of usage.Service needed for cost recording.
type usageCostRecorder interface {
	RecordAICost(ctx context.Context, userID uuid.UUID, costCents int32) error
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

// SetUsageService enables AI cost recording to the database. Pass nil to disable.
func (s *Service) SetUsageService(svc usageCostRecorder) {
	s.usageSvc = svc
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

	// Attachments are file attachments (images, PDFs, text) sent with the message.
	Attachments []Attachment

	// ExtraTools are additional tools available for this request (e.g., create_trip in selection mode)
	ExtraTools []tools.Tool
	// ExtraSystemContext is appended to the system prompt (e.g., trip list for selection mode)
	ExtraSystemContext string

	// PersonaSwitchCh is an optional buffered channel (size 1) that signals
	// a mid-turn persona handoff. When suggest_expert fires, the handler
	// writes the new expert to this channel; the chat service drains it
	// between tool loop iterations and rebuilds the system prompt so the
	// expert answers in the same turn (#175).
	PersonaSwitchCh chan *persona.Persona
}

// Attachment represents a file attached to a chat message.
type Attachment struct {
	Filename  string
	MediaType string
	Data      []byte
}

func (s *Service) SendMessage(ctx context.Context, params SendMessageParams) (<-chan StreamEvent, string, error) {
	sessionID := params.SessionID

	// Use "_lobby" as the Firestore trip path for selection mode (no trip)
	storeTripID := params.TripID
	if storeTripID == "" {
		storeTripID = "_lobby"
	}

	// Create session if needed
	isNewSession := false
	if sessionID == "" {
		session, err := s.chatStore.CreateSession(ctx, params.UserID.String(), storeTripID, params.Mode)
		if err != nil {
			return nil, "", fmt.Errorf("create session: %w", err)
		}
		sessionID = session.ID
		isNewSession = true
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

	// Store user message. We save the ID so it can be rolled back if the AI
	// response fails (preventing orphaned messages in chat history).
	userMsg := &chatstore.ChatMessage{
		Role:    "user",
		Content: params.Content,
	}
	if err := s.chatStore.AddMessage(ctx, params.UserID.String(), storeTripID, sessionID, userMsg); err != nil {
		return nil, "", fmt.Errorf("store user message: %w", err)
	}
	userMsgID := userMsg.ID // populated by AddMessage

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

	// If the current message has attachments, enrich the last user message
	// with multimodal content blocks so the AI provider receives them.
	if len(params.Attachments) > 0 && len(messages) > 0 {
		last := &messages[len(messages)-1]
		if last.Role == "user" {
			last.ContentBlocks = buildAttachmentBlocks(last.Content, params.Attachments)
			last.Content = "" // Content is now in ContentBlocks
		}
	}

	// buildPrompt assembles the full system prompt for a given persona,
	// including ephemeral location context and extra trip context. Captured
	// as a closure so the tool loop can rebuild the prompt when an expert
	// hands off mid-turn (#175).
	buildPrompt := func(p *persona.Persona) string {
		sp := p.SystemPrompt(params.Mode)
		if params.LocationLat != 0 && params.LocationLng != 0 {
			sp += fmt.Sprintf("\n\nThe user's current location is approximately: %.4f, %.4f. Use this to provide relevant nearby recommendations. Do NOT repeat these coordinates back to the user.", params.LocationLat, params.LocationLng)
		}
		if params.ExtraSystemContext != "" {
			sp += "\n\n" + params.ExtraSystemContext
		}
		return sp
	}
	systemPrompt := buildPrompt(activePersona)

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
	userIDStr := params.UserID.String()
	if s.cache != nil && s.cache.Eligible(userIDStr, aiReq) {
		if cached, ok := s.cache.Get(userIDStr, aiReq); ok {
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
		responseText := s.processEventsWithToolLoop(ctx, aiReq, outCh, extraToolsMap, params.UserID, storeTripID, sessionID, params.PersonaSwitchCh, buildPrompt)

		// Clean up on AI failure: if the AI produced no response (e.g., 429,
		// timeout, provider error), roll back the user message and optionally
		// the session so they don't appear as orphaned entries in history.
		if responseText == "" {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Roll back the user message so history doesn't show a message
			// with no AI response.
			if userMsgID != "" {
				if err := s.chatStore.DeleteMessage(cleanupCtx, params.UserID.String(), storeTripID, sessionID, userMsgID); err != nil {
					slog.Error("failed to delete orphaned user message", "message_id", userMsgID, "error", err)
				}
			}

			// Also delete the session itself if it was freshly created and is
			// now empty, to keep the session list tidy.
			if isNewSession {
				slog.Warn("cleaning up orphaned session with no AI response",
					"session_id", sessionID,
					"user_id", params.UserID.String(),
				)
				if err := s.chatStore.DeleteSession(cleanupCtx, params.UserID.String(), storeTripID, sessionID); err != nil {
					slog.Error("failed to delete orphaned session", "session_id", sessionID, "error", err)
				}
			}
		}

		// Cache the response after streaming completes (only for eligible requests).
		if s.cache != nil && s.cache.Eligible(userIDStr, aiReq) && responseText != "" {
			s.cache.Put(userIDStr, aiReq, responseText)
			slog.Debug("llm response cached",
				"mode", params.Mode,
				"response_len", len(responseText),
			)
		}
	}()

	return outCh, sessionID, nil
}

const (
	// maxToolLoopIterations prevents infinite tool call loops.
	maxToolLoopIterations = 5

	// turnTimeout is the per-turn deadline for the AI provider channel. If the
	// provider goroutine stalls or panics without closing the channel,
	// processOneTurn returns an error after this duration rather than hanging.
	turnTimeout = 90 * time.Second
)

// processEventsWithToolLoop handles the AI stream and implements the tool call
// continuation loop. When the AI stops for tool use, tool results are sent back
// and the AI continues generating. This loops until the AI produces a final
// response (stop_reason=end_turn) or the iteration limit is reached.
func (s *Service) processEventsWithToolLoop(ctx context.Context, aiReq *ai.ChatRequest, outCh chan<- StreamEvent, extraTools map[string]tools.Tool, userID uuid.UUID, tripID, sessionID string, personaSwitchCh <-chan *persona.Persona, buildPrompt func(*persona.Persona) string) string {
	var fullResponse strings.Builder
	var totalInputTokens, totalOutputTokens int
	var fabricationRetried bool // one-shot guard against fabricated tool success (#171)

	for iteration := 0; iteration < maxToolLoopIterations; iteration++ {
		// Stop if the client disconnected.
		if ctx.Err() != nil {
			slog.Info("tool loop: client disconnected, stopping", "iteration", iteration)
			return fullResponse.String()
		}

		// Reset the text accumulator at the start of each iteration so that
		// only the final turn's text is stored in the assistant message.
		// Some providers (Gemini) re-send earlier text in continuation turns,
		// which would otherwise cause duplicated content in chat history.
		// Real-time text_delta events sent to outCh are unaffected.
		fullResponse.Reset()

		// Start (or continue) streaming.
		// Log session_id on every iteration so cross-user contamination can be
		// traced in Cloud Logging if it recurs (see toqui-backend#125).
		slog.Debug("tool loop: starting AI stream",
			"session_id", sessionID,
			"user_id", userID,
			"iteration", iteration,
			"messages", len(aiReq.Messages),
		)
		eventCh, err := s.provider.ChatStream(ctx, aiReq)
		if err != nil {
			outCh <- StreamEvent{Type: "error", Error: fmt.Sprintf("start chat stream: %v", err)}
			return ""
		}

		// Process this turn's events
		turnText, toolCalls, toolResults, stopReason, turnUsage, streamErr := s.processOneTurn(ctx, eventCh, outCh, extraTools)
		// Add separator between tool loop turns to avoid run-on text
		// (e.g., "...some text!More text" → "...some text! More text")
		if fullResponse.Len() > 0 && len(turnText) > 0 {
			last := fullResponse.String()[fullResponse.Len()-1]
			first := turnText[0]
			if last != ' ' && last != '\n' && first != ' ' && first != '\n' {
				fullResponse.WriteByte(' ')
			}
		}
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

			// Mid-loop persona swap: if suggest_expert fired, drain the channel
			// and rebuild the system prompt so the expert answers in the same
			// turn instead of requiring a follow-up user message (#175).
			var swappedPersona *persona.Persona
			if personaSwitchCh != nil && buildPrompt != nil {
				select {
				case newPersona := <-personaSwitchCh:
					if newPersona != nil {
						aiReq.SystemPrompt = buildPrompt(newPersona)
						swappedPersona = newPersona
						// Reset fabrication guard so the expert gets a fresh
						// chance to call its own tools without being treated
						// as a continuation of the previous persona's turn.
						fabricationRetried = false
						slog.Info("tool loop: persona swapped mid-turn",
							"session_id", sessionID,
							"new_persona_id", newPersona.ID,
							"new_persona_name", newPersona.Name,
						)
					}
				default:
					// no swap pending
				}
			}

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

			// After a persona swap, append a non-persisted nudge so the new
			// expert is forced to substantively answer the user's most recent
			// message instead of just introducing themselves and ending the
			// turn (#193). This message exists only in the AI request — it
			// is NOT written to chat history.
			if swappedPersona != nil {
				aiReq.Messages = append(aiReq.Messages, ai.Message{
					Role: "user",
					Content: fmt.Sprintf(
						"(System note: you are now responding as %s. The handoff is complete. Answer my most recent question DIRECTLY and substantively using your specialised expertise. Do NOT introduce yourself, do NOT defer to anyone else, and do NOT call suggest_expert again.)",
						swappedPersona.Name,
					),
				})
			}

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

		// AI finished (end_turn) — store and emit the final response.
		// If the response is empty (no text, no tool calls), suppress the
		// assistant message to avoid blank bubbles in chat history, and emit
		// an error event so the frontend can show a retry prompt. This is the
		// symmetric fix to the user-message rollback on AI failure (#167).
		responseText := fullResponse.String()
		if responseText == "" && len(toolCalls) == 0 {
			slog.Warn("suppressing empty assistant turn — no content and no tool calls",
				"session_id", sessionID,
				"iteration", iteration,
			)
			select {
			case outCh <- StreamEvent{Type: "error", Error: "no response from AI — please try again"}:
			case <-ctx.Done():
			}
			return ""
		}

		// Detect fabricated tool success: in planning mode the AI sometimes
		// claims it has added items to the itinerary without ever calling
		// create_itinerary_items. Inject a one-shot silent correction so the
		// AI actually calls the tool. The fabricated text is added to the AI
		// context for continuity but is NOT persisted to chat history (#171).
		if aiReq.Mode == "planning" && !fabricationRetried && len(toolCalls) == 0 && impliesItineraryCreation(responseText) {
			fabricationRetried = true
			slog.Warn("tool loop: fabricated tool success detected — re-prompting",
				"session_id", sessionID,
				"iteration", iteration,
				"response_preview", responseText[:min(len(responseText), 120)],
			)
			aiReq.Messages = append(aiReq.Messages,
				ai.Message{Role: "assistant", Content: responseText},
				ai.Message{Role: "user", Content: "You described adding items to the itinerary but did not call create_itinerary_items. Call it now with the items you just described."},
			)
			fullResponse.Reset()
			continue
		}

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
		s.logUsage(ctx, userID, aiReq.ModelTier, iteration+1, totalInputTokens, totalOutputTokens)
		return responseText
	}

	// Hit iteration limit — log, optionally append a brief note, then store what we have.
	slog.Warn("tool loop: hit max iterations",
		"session_id", sessionID,
		"iterations", maxToolLoopIterations,
	)

	const iterLimitNote = "\n\n*(Some details may be incomplete due to tool execution constraints.)*"
	responseText := fullResponse.String()
	if responseText != "" {
		// Emit a subtle note so the user knows the response may be partial.
		select {
		case outCh <- StreamEvent{Type: "text_delta", Text: iterLimitNote}:
		case <-ctx.Done():
		}
		responseText += iterLimitNote
	}

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

	s.logUsage(ctx, userID, aiReq.ModelTier, maxToolLoopIterations, totalInputTokens, totalOutputTokens)
	return responseText
}

// estimateCostUSD returns the estimated cost in USD for a request based on
// provider and model tier pricing. Rates are approximate and should be updated
// when pricing changes.
func estimateCostUSD(provider string, tier ai.ModelTier, inputTokens, outputTokens int) float64 {
	// Per-million-token rates (as of April 2026)
	var inputRate, outputRate float64
	switch provider {
	case "claude":
		switch tier {
		case ai.ModelTierFast:
			// Claude Haiku 4.5
			inputRate, outputRate = 0.80, 4.00
		case ai.ModelTierSmart, ai.ModelTierBest:
			// Claude Sonnet 4.6
			inputRate, outputRate = 3.00, 15.00
		default:
			inputRate, outputRate = 0.80, 4.00
		}
	case "gemini":
		switch tier {
		case ai.ModelTierFast:
			// Gemini 2.5 Flash-Lite
			inputRate, outputRate = 0.075, 0.30
		case ai.ModelTierSmart:
			// Gemini 2.5 Flash
			inputRate, outputRate = 0.15, 0.60
		case ai.ModelTierBest:
			// Gemini 2.5 Pro
			inputRate, outputRate = 1.25, 10.00
		default:
			inputRate, outputRate = 0.075, 0.30
		}
	default:
		return 0
	}
	return (float64(inputTokens)*inputRate + float64(outputTokens)*outputRate) / 1_000_000
}

// logUsage logs token usage with provider and environment labels for cost tracking,
// records against the daily token budget if configured, and persists the estimated
// cost in the daily_usage table via the usage service.
func (s *Service) logUsage(ctx context.Context, userID uuid.UUID, tier ai.ModelTier, iterations, inputTokens, outputTokens int) {
	if inputTokens == 0 && outputTokens == 0 {
		return
	}
	totalTokens := inputTokens + outputTokens
	costUSD := estimateCostUSD(s.provider.Name(), tier, inputTokens, outputTokens)
	slog.Info("ai_request_completed",
		"provider", s.provider.Name(),
		"env", os.Getenv("TARGET_ENV"),
		"input_tokens", inputTokens,
		"output_tokens", outputTokens,
		"total_tokens", totalTokens,
		"estimated_cost_usd", costUSD,
		"tool_loop_iterations", iterations,
		"model_tier", string(tier),
	)

	if s.budget != nil {
		s.budget.Record(totalTokens)
	}

	// Persist cost to database (convert USD to cents, round up to at least 1 cent
	// if there was any usage to avoid losing sub-cent costs).
	if s.usageSvc != nil {
		costCents := int32(costUSD*100 + 0.5) // round to nearest cent
		if costCents > 0 {
			if err := s.usageSvc.RecordAICost(ctx, userID, costCents); err != nil {
				slog.Error("failed to record AI cost", "error", err, "user_id", userID, "cost_cents", costCents)
			}
		}
	}
}

// processOneTurn drains a single AI stream, executing any tool calls.
// Returns the text produced, tool calls made, tool results, stop reason, usage, and any error.
func (s *Service) processOneTurn(ctx context.Context, eventCh <-chan ai.Event, outCh chan<- StreamEvent, extraTools map[string]tools.Tool) (text string, toolCalls []ai.ToolCall, toolResults []ai.ToolResult, stopReason string, usage *ai.Usage, err error) {
	var turnText strings.Builder
	var turnUsage *ai.Usage

	turnCtx, turnCancel := context.WithTimeout(ctx, turnTimeout)
	defer turnCancel()

	for {
		select {
		case <-turnCtx.Done():
			if ctx.Err() != nil {
				// Parent context cancelled (client disconnected).
				return turnText.String(), toolCalls, toolResults, "", turnUsage, ctx.Err()
			}
			// Per-turn timeout expired.
			return turnText.String(), toolCalls, toolResults, "", turnUsage,
				fmt.Errorf("turn timeout: AI provider did not respond within %s", turnTimeout)

		case event, ok := <-eventCh:
			if !ok {
				// Channel closed without an EventDone — log and return what we have.
				if stopReason == "" {
					slog.Warn("ai stream closed without EventDone",
						"text_len", turnText.Len(),
						"tool_calls", len(toolCalls),
					)
				}
				return turnText.String(), toolCalls, toolResults, stopReason, turnUsage, nil
			}

			switch event.Type {
			case ai.EventTextDelta:
				turnText.WriteString(event.Text)
				select {
				case outCh <- StreamEvent{Type: "text_delta", Text: event.Text}:
				case <-turnCtx.Done():
				}

			case ai.EventToolCall:
				if event.Tool != nil {
					select {
					case outCh <- StreamEvent{
						Type:      "tool_call",
						ToolName:  event.Tool.Name,
						ToolInput: event.Tool.Arguments,
					}:
					case <-turnCtx.Done():
					}

					// Track this tool call for the continuation message
					toolCalls = append(toolCalls, *event.Tool)

					// Execute tool — check extra tools first, then global registry.
					// We tolerate provider name mangling (e.g. Gemini emitting
					// "createItineraryItemsItems" for "create_itinerary_items") via
					// the canonicalized fallback; see tools.canonicalizeToolName.
					var result json.RawMessage
					var execErr error
					if extra, ok := lookupExtraTool(extraTools, event.Tool.Name); ok {
						result, execErr = extra.Execute(ctx, json.RawMessage(event.Tool.Arguments))
					} else {
						result, execErr = s.tools.Execute(ctx, event.Tool.Name, []byte(event.Tool.Arguments))
					}

					var resultStr string
					if execErr != nil {
						slog.Error("tool execution failed",
							"tool", event.Tool.Name,
							"error", execErr.Error(),
						)
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
					case <-turnCtx.Done():
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
				case <-turnCtx.Done():
				}
				return turnText.String(), toolCalls, toolResults, "", turnUsage, event.Error
			}
		}
	}
}

// impliesItineraryCreation returns true when the text contains confident
// past-tense assertions that itinerary items were persisted. Used to detect
// fabricated tool success: Gemini sometimes claims to have called
// create_itinerary_items without actually doing so (#171).
//
// The phrases are intentionally narrow — they must assert a completed
// add/save action on "the itinerary" to avoid false positives on normal
// planning prose ("here are some items you could add…").
func impliesItineraryCreation(text string) bool {
	lower := strings.ToLower(text)
	for _, phrase := range []string{
		"added to your itinerary",
		"added to the itinerary",
		"added them to your itinerary",
		"added those to your itinerary",
		"added these to your itinerary",
		"have been added to your itinerary",
		"have been added to the itinerary",
		"saved to your itinerary",
		"saved to the itinerary",
		"saved them to your itinerary",
		"saved those to your itinerary",
		"saved these to your itinerary",
		"created your itinerary",
		"i've already added",
		"i've put together a",
		"already been added to your itinerary",
	} {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	return false
}

// lookupExtraTool resolves a per-request tool by name, falling back to a
// canonicalized match so provider-side name mangling (e.g. Gemini returning
// "createItineraryItemsItems" for "create_itinerary_items") still finds the
// right tool.
func lookupExtraTool(extras map[string]tools.Tool, name string) (tools.Tool, bool) {
	if t, ok := extras[name]; ok {
		return t, true
	}
	if len(extras) == 0 {
		return nil, false
	}
	canon := tools.CanonicalToolName(name)
	for registered, t := range extras {
		if tools.CanonicalToolName(registered) == canon {
			return t, true
		}
	}
	return nil, false
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

// MoveSessionToTrip retroactively links a selection-mode session (stored under
// "_lobby") to the trip that was created during that conversation. This makes
// the initial conversation visible via ListSessions(tripID) and GetHistory.
func (s *Service) MoveSessionToTrip(ctx context.Context, userID uuid.UUID, sessionID, toTripID string) error {
	return s.chatStore.MoveSessionToTrip(ctx, userID.String(), "_lobby", toTripID, sessionID)
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
	all, err := s.chatStore.GetMessages(ctx, userID.String(), tripID, sessionID, limit)
	if err != nil {
		return nil, err
	}
	// Filter out synthetic tool-loop continuation messages: user messages with
	// no text content that exist only to carry tool results back to the AI.
	// These are internal protocol messages and should not appear in chat history.
	filtered := all[:0]
	for _, msg := range all {
		if msg.Role == "user" && msg.Content == "" && len(msg.ToolResults) > 0 {
			continue
		}
		filtered = append(filtered, msg)
	}
	return filtered, nil
}

// buildAttachmentBlocks converts the user's text content and attachments into
// multimodal content blocks suitable for the AI provider.
//
// Images are sent as base64-encoded image blocks (Claude vision / Gemini inlineData).
// Text files and CSVs are inlined as text blocks.
// PDFs are noted by filename (full PDF parsing is not yet implemented).
func buildAttachmentBlocks(textContent string, attachments []Attachment) []ai.ContentBlock {
	blocks := make([]ai.ContentBlock, 0, 1+len(attachments))

	// Lead with the user's text message
	if textContent != "" {
		blocks = append(blocks, ai.ContentBlock{
			Type: "text",
			Text: textContent,
		})
	}

	for _, att := range attachments {
		switch {
		case strings.HasPrefix(att.MediaType, "image/"):
			// Image attachments → vision content blocks
			blocks = append(blocks, ai.ContentBlock{
				Type: "image",
				Source: &ai.ImageSource{
					Type:      "base64",
					MediaType: att.MediaType,
					Data:      base64.StdEncoding.EncodeToString(att.Data),
				},
			})

		case att.MediaType == "text/plain" || att.MediaType == "text/csv":
			// Text/CSV attachments → inline text blocks
			label := "text file"
			if att.MediaType == "text/csv" {
				label = "CSV file"
			}
			blocks = append(blocks, ai.ContentBlock{
				Type: "text",
				Text: fmt.Sprintf("[Attached %s: %s]\n%s", label, att.Filename, string(att.Data)),
			})

		case att.MediaType == "application/pdf":
			// PDF attachments → extract text and inject as context.
			// Falls back to a filename-only note if extraction fails.
			extractedText, err := extractPDFText(att.Data)
			if err != nil || strings.TrimSpace(extractedText) == "" {
				slog.Warn("pdf text extraction failed, falling back to filename note",
					"filename", att.Filename,
					"error", err,
				)
				blocks = append(blocks, ai.ContentBlock{
					Type: "text",
					Text: fmt.Sprintf("[Attached PDF: %s (%d bytes) — the PDF content could not be extracted. Please ask the user to copy and paste the relevant text.]",
						att.Filename, len(att.Data)),
				})
			} else {
				blocks = append(blocks, ai.ContentBlock{
					Type: "text",
					Text: fmt.Sprintf("[Attached PDF: %s]\n%s", att.Filename, extractedText),
				})
			}

		default:
			slog.Warn("unsupported attachment media type in content block builder",
				"media_type", att.MediaType,
				"filename", att.Filename,
			)
		}
	}

	return blocks
}

// maxPDFTextBytes is the maximum number of characters extracted from a PDF
// to avoid overflowing the AI context window.
const maxPDFTextBytes = 10_000

// extractPDFText extracts plain text from a PDF given its raw bytes.
// It uses the ledongthuc/pdf library for pure-Go extraction.
// The returned text is capped at maxPDFTextBytes characters.
func extractPDFText(data []byte) (string, error) {
	r := bytes.NewReader(data)
	reader, err := pdf.NewReader(r, int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("open pdf: %w", err)
	}

	plainTextReader, err := reader.GetPlainText()
	if err != nil {
		return "", fmt.Errorf("get plain text: %w", err)
	}

	var sb strings.Builder
	buf := make([]byte, 4096)
	for {
		n, readErr := plainTextReader.Read(buf)
		if n > 0 {
			remaining := maxPDFTextBytes - sb.Len()
			if remaining <= 0 {
				sb.WriteString("\n[... content truncated to stay within context limit ...]")
				break
			}
			chunk := buf[:n]
			if len(chunk) > remaining {
				chunk = chunk[:remaining]
				sb.Write(chunk)
				sb.WriteString("\n[... content truncated to stay within context limit ...]")
				break
			}
			sb.Write(chunk)
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return sb.String(), fmt.Errorf("read pdf text: %w", readErr)
		}
	}

	return sb.String(), nil
}
