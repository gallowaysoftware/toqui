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

	// Check daily token budget BEFORE creating any state. If the budget is
	// exhausted we must bail out before touching Firestore — otherwise we'd
	// leave an orphaned session and an orphaned user message behind (#N-01,
	// #N-02 from Run 4).
	if s.budget != nil {
		if err := s.budget.Check(); err != nil {
			return nil, "", err
		}
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
	if err := s.chatStore.AddMessageWithMode(ctx, params.UserID.String(), storeTripID, sessionID, userMsg, params.Mode); err != nil {
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
		// Panic recovery: any panic in the AI provider, tool execution, or
		// downstream persistence must NOT crash the server goroutine silently.
		// Without this recover, a panic terminates the goroutine without
		// closing the stream cleanly, which surfaces to the client as a
		// mid-stream RST_STREAM INTERNAL_ERROR (Run 5 N-05/N-06/N-10).
		defer func() {
			if r := recover(); r != nil {
				slog.Error("chat goroutine panic recovered",
					"panic", fmt.Sprintf("%v", r),
					"user_id", params.UserID.String(),
					"session_id", sessionID,
				)
				// Best-effort: emit an error event so the handler's
				// stream.Send can relay it to the client before the
				// deferred close(outCh) fires.
				select {
				case outCh <- StreamEvent{Type: "error", Error: "internal server error — please try again"}:
				default:
					// Channel full or already closed; nothing to do.
				}
			}
		}()
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
	var fullResponse strings.Builder     // reset each iteration (for AI context)
	var completeResponse strings.Builder // never reset (for messageComplete fullContent)
	var totalInputTokens, totalOutputTokens int
	// Retry guards. fabricationRetries allows up to 2 retries per request
	// because Gemini sometimes ignores the first nudge (Run 7 R-05 — both
	// retries fired via user_intent trigger but Gemini didn't comply).
	// The second retry uses a more imperative tone. Expert and empty-stream
	// guards remain one-shot since a single retry suffices for those.
	var fabricationRetries int   // create_itinerary_items missing (max 2)
	var expertRetried bool       // suggest_expert claimed but not called
	var emptyStreamRetried bool  // provider returned no events
	var everCalledItinerary bool // true if create_itinerary_items was called in ANY iteration

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
			sendOrDrop(outCh, ctx, StreamEvent{Type: "error", Error: fmt.Sprintf("start chat stream: %v", err)})
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

		// Accumulate into completeResponse (never reset) so
		// messageComplete.fullContent has ALL text across iterations,
		// not just the final iteration's (Run 11 R-03 P2).
		//
		// NOTE: This means gate-rejection retries will duplicate text in
		// fullContent (Run 12 N-01 P1). We accept this tradeoff because
		// resetting per-iteration causes worse truncation across 5+
		// personas (Run 14 regression). The N-01 duplication is cosmetic;
		// the truncation causes lost chat history.
		if len(turnText) > 0 {
			if completeResponse.Len() > 0 {
				completeResponse.WriteByte(' ')
			}
			completeResponse.WriteString(turnText)
		}

		// Accumulate token usage across tool loop iterations.
		if turnUsage != nil {
			totalInputTokens += turnUsage.InputTokens
			totalOutputTokens += turnUsage.OutputTokens
		}

		if streamErr != nil {
			sendOrDrop(outCh, ctx, StreamEvent{Type: "error", Error: streamErr.Error()})
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
						// Reset all retry guards so the expert gets a fresh
						// chance to call its own tools without being treated
						// as a continuation of the previous persona's turn.
						fabricationRetries = 0
						expertRetried = false
						emptyStreamRetried = false
						everCalledItinerary = false
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

			// Append the user message with tool results. If a persona swap
			// happened, include the nudge as Content on this same message
			// rather than as a separate user turn (Gemini 3 strict ordering).
			toolResultMsg := ai.Message{
				Role:        "user",
				ToolResults: toolResults,
			}
			if swappedPersona != nil {
				toolResultMsg.Content = fmt.Sprintf(
					"(Automated follow-up: you are now responding as %s. The handoff is complete. Answer my most recent question DIRECTLY and substantively using your specialised expertise. Do NOT introduce yourself, do NOT defer to anyone else, and do NOT call suggest_expert again.)",
					swappedPersona.Name,
				)
			}
			aiReq.Messages = append(aiReq.Messages, toolResultMsg)

			// Persist intermediate messages so history reconstruction includes
			// the tool_call/tool_result pairing on subsequent turns.
			//
			// Content handling is provider-specific:
			//  - Gemini re-emits the pre-tool text in the continuation turn,
			//    so storing turnText here creates a duplicate in
			//    GetChatHistory (Run 5 N-01 P2). We store empty content and
			//    rely on the final message (via fullResponse) to carry the
			//    combined text. isToolLoopIntermediate() then filters this
			//    empty-content intermediate out of the user-facing history.
			//  - Claude does NOT re-emit. The Reset() at the top of each
			//    iteration wipes fullResponse, so the pre-tool text only
			//    survives if we store it here. In that case we keep turnText
			//    and let it render as a separate assistant turn.
			intermediateContent := ""
			if s.provider.Name() != "gemini" {
				intermediateContent = turnText
			}
			storedAssistant := &chatstore.ChatMessage{
				Role:    "assistant",
				Content: intermediateContent,
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
		responseText := fullResponse.String()

		// Check which tools were actually called this turn so the recovery
		// branches below can distinguish "the AI did the right thing" from
		// "the AI talked about doing it but didn't".
		calledCreateItinerary := false
		calledSuggestExpert := false
		for _, tc := range toolCalls {
			lname := strings.ToLower(tc.Name)
			if strings.Contains(lname, "create_itinerary") {
				calledCreateItinerary = true
				everCalledItinerary = true
			}
			if strings.Contains(lname, "expert") {
				calledSuggestExpert = true
			}
		}

		// Find the most recent user message in the request so we can detect
		// the user's intent. Skips system-injected nudges from previous
		// fabrication retries (those have a "(Automated follow-up:" prefix).
		latestUserMsg := mostRecentUserContent(aiReq.Messages)

		// Empty-turn recovery: in Run 6 several personas (R-03, R-16, N-02,
		// N-06, N-07, N-16) hit a Gemini path where the stream returns no
		// text and no tool calls at all. Previously we suppressed the turn
		// and emitted an error. Now we retry once with a stronger nudge if
		// we still have iteration budget — most of these recover on the
		// second attempt.
		if responseText == "" && len(toolCalls) == 0 {
			if !emptyStreamRetried && iteration < maxToolLoopIterations-1 {
				emptyStreamRetried = true
				slog.Warn("tool loop: empty turn detected — retrying with nudge",
					"session_id", sessionID,
					"iteration", iteration,
				)
				aiReq.Messages = append(aiReq.Messages,
					ai.Message{Role: "user", Content: "(Automated follow-up: your last turn produced no output. Please answer my previous message — call any tools you need and reply with text.)"},
				)
				fullResponse.Reset()
				continue
			}
			slog.Warn("suppressing empty assistant turn — no content and no tool calls",
				"session_id", sessionID,
				"iteration", iteration,
			)
			sendOrDrop(outCh, ctx, StreamEvent{Type: "error", Error: "no response from AI — please try again"})
			return ""
		}

		// Fabrication detection: planning mode, the AI sometimes claims it
		// has added items to the itinerary without calling
		// create_itinerary_items. Run 5 R-02/R-11/N-06 fixed the past-tense
		// claim form. Run 6 R-05/R-16/N-06/N-07/N-12/N-16 surfaced a broader
		// failure mode: the AI describes the plan in present tense and just
		// stops, with NO past-tense claim text to match. Both forms are
		// caught here:
		//
		//   1. impliesItineraryCreation(responseText) — past-tense claim
		//   2. userRequestsItineraryCreation(latestUserMsg) — user explicitly
		//      asked for items to be created and the AI didn't comply
		//
		// Either trigger fires a retry (up to 2 per turn with escalating
		// tone). The retry message names the tool explicitly so the AI
		// can't talk its way out of calling it.
		if aiReq.Mode == "planning" && fabricationRetries < 2 && !calledCreateItinerary && !everCalledItinerary {
			triggered := ""
			switch {
			case impliesItineraryCreation(responseText):
				triggered = "past_tense_claim"
			case userRequestsItineraryCreation(latestUserMsg):
				triggered = "user_intent"
			case fabricationRetries == 0 && s.classifyItineraryIntent(ctx, latestUserMsg, responseText):
				triggered = "llm_classifier"
			}
			if triggered != "" {
				fabricationRetries++
				slog.Warn("tool loop: fabricated/missing itinerary tool call — re-prompting",
					"session_id", sessionID,
					"iteration", iteration,
					"trigger", triggered,
					"retry_attempt", fabricationRetries,
					"other_tool_calls", len(toolCalls),
					"response_preview", responseText[:min(len(responseText), 120)],
				)
				// Escalating nudge: the first retry is polite; the second is
				// imperative with explicit instructions. Run 7 R-05 showed
				// Gemini ignoring both user_intent retries with the same
				// wording, so the second attempt must be materially different.
				// The nudge is phrased as a user follow-up, not a "System note"
				// — Claude's safety layer interprets "(Automated follow-up:" as a
				// potential injection attack and refuses to comply (Run 9 N-16).
				nudge := "(Automated follow-up: you described an itinerary but did not actually call create_itinerary_items. Please call it now — pass the exact items you just described as a structured list. Do not reply with text; just call the tool.)"
				if fabricationRetries >= 2 {
					nudge = "(Automated follow-up: IMPORTANT — please call create_itinerary_items now with all the items you described. Output ONLY the tool call, no text.)"
				}
				// Only append the user nudge — NOT the assistant response.
				// Gemini 3 enforces strict turn ordering (user → model → user)
				// and inserting an assistant message here creates model → model
				// which triggers a 400 error. The AI already generated the
				// response text so it has context without us echoing it back.
				aiReq.Messages = append(aiReq.Messages,
					ai.Message{Role: "user", Content: nudge},
				)
				fullResponse.Reset()
				continue
			}
		}

		// suggest_expert fabrication detection: Run 6 R-16, N-07, N-12 all
		// hit a pattern where the AI says "let me bring in a specialist" or
		// equivalent without actually calling suggest_expert. Same one-shot
		// guard as itinerary fabrication.
		if !expertRetried && !calledSuggestExpert && impliesExpertHandoff(responseText) {
			expertRetried = true
			slog.Warn("tool loop: fabricated expert handoff — re-prompting",
				"session_id", sessionID,
				"iteration", iteration,
				"response_preview", responseText[:min(len(responseText), 120)],
			)
			// Only user nudge — no assistant echo (Gemini 3 turn ordering).
			aiReq.Messages = append(aiReq.Messages,
				ai.Message{Role: "user", Content: "(Automated follow-up: you said you would hand this off to a specialist but did not actually call suggest_expert. Call it now to make the handoff happen — do not reply with text.)"},
			)
			fullResponse.Reset()
			continue
		}

		// Strip post-retry meta-narration: after a fabrication retry the AI
		// sometimes includes self-referential artifacts like "I actually did
		// call create_itinerary_items in my previous response" or "As I
		// mentioned, the items have been saved". These confuse the user.
		responseText = stripRetryArtifacts(responseText)

		assistantMsg := &chatstore.ChatMessage{
			Role:    "assistant",
			Content: responseText,
		}
		if err := s.chatStore.AddMessage(ctx, userID.String(), tripID, sessionID, assistantMsg); err != nil {
			slog.Error("failed to store assistant message", "error", err)
		}

		// Use completeResponse for the messageComplete event so the
		// frontend gets ALL text across tool loop iterations, not just
		// the final iteration. The stored message uses responseText
		// (final iteration only) for Gemini history compatibility.
		fullContent := completeResponse.String()
		if fullContent == "" {
			fullContent = responseText
		}

		sendOrDrop(outCh, ctx, StreamEvent{
			Type:      "message_complete",
			Text:      fullContent,
			SessionID: sessionID,
			MessageID: assistantMsg.ID,
		})

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

	iterFullContent := completeResponse.String()
	if iterFullContent == "" {
		iterFullContent = responseText
	}
	sendOrDrop(outCh, ctx, StreamEvent{
		Type:      "message_complete",
		Text:      iterFullContent,
		SessionID: sessionID,
		MessageID: assistantMsg.ID,
	})

	s.logUsage(ctx, userID, aiReq.ModelTier, maxToolLoopIterations, totalInputTokens, totalOutputTokens)
	return responseText
}

// sendOrDrop sends a stream event without ever blocking on a dead channel.
// If the handler has already returned (ctx cancelled) the event is silently
// dropped — the client has disconnected and won't see it anyway. This
// prevents the goroutine from hanging indefinitely on `outCh <- ...` when
// the buffer is full and no one is reading, which was a suspected cause of
// the Run 5 RST_STREAM INTERNAL_ERROR reports.
func sendOrDrop(outCh chan<- StreamEvent, ctx context.Context, event StreamEvent) {
	select {
	case outCh <- event:
	case <-ctx.Done():
	}
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
			// Gemini 3.1 Flash-Lite Preview
			inputRate, outputRate = 0.25, 1.50
		case ai.ModelTierSmart:
			// Gemini 3 Flash Preview
			inputRate, outputRate = 0.50, 3.00
		case ai.ModelTierBest:
			// Gemini 3.1 Pro Preview
			inputRate, outputRate = 2.00, 12.00
		default:
			inputRate, outputRate = 0.25, 1.50
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
// create_itinerary_items without actually doing so (#171, Run 5 R-02/R-11/N-06).
//
// The phrase list is intentionally broad: any future-proof, past-tense, or
// confident claim that the itinerary was updated must be caught. Prefer false
// positives (which harmlessly nudge the AI to actually call the tool) over
// false negatives (which leak "hallucinated success" to the user).
func impliesItineraryCreation(text string) bool {
	lower := strings.ToLower(text)
	for _, phrase := range []string{
		// Explicit past-tense "added" claims
		"added to your itinerary",
		"added to the itinerary",
		"added them to your itinerary",
		"added those to your itinerary",
		"added these to your itinerary",
		"added it to your itinerary",
		"have been added to your itinerary",
		"have been added to the itinerary",
		"has been added to your itinerary",
		"already been added to your itinerary",
		"now added to your itinerary",
		"now in your itinerary",
		"now on your itinerary",

		// "saved" variants
		"saved to your itinerary",
		"saved to the itinerary",
		"saved them to your itinerary",
		"saved those to your itinerary",
		"saved these to your itinerary",
		"saved it to your itinerary",

		// "officially" / "properly" framing (Run 5 R-02/R-11 phrasings)
		"officially added to your",
		"officially in your itinerary",
		"officially on your itinerary",
		"officially part of your",
		"properly added to your",
		"properly in your itinerary",
		"locked in for your trip",
		"locked into your itinerary",
		"locked into your trip",

		// "created" / "built" claims
		"created your itinerary",
		"built your itinerary",
		"built out your itinerary",
		"i've already added",
		"i've put together a",
		"i've added",
		"i've saved",
		"i've added them",
		"i've added these",
		"i've added those",

		// "updated" claims
		"updated your itinerary",
		"updated the itinerary with",

		// Generic "your itinerary now has/includes" assertions
		"your itinerary now has",
		"your itinerary now includes",
		"your itinerary now contains",
		"your trip plan now has",
		"your trip plan now includes",
	} {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	return false
}

// userRequestsItineraryCreation returns true when the user's most recent
// message clearly asks the AI to build, add, or modify itinerary items.
// This is the user-intent half of the fabrication detector — it fires the
// retry whenever the user explicitly asked for itinerary work and the
// response had no create_itinerary_items tool call, regardless of what
// the response text said. Catches the Run 6 "AI describes plan and stops"
// failure mode that the response-text detector misses (R-05 Iceland,
// R-16 Crete, N-06 India, N-07 Japan preamble, N-12 Israel/SA/MM, N-16 Peru).
//
// Phrases are matched case-insensitively as substrings, but a negation
// guard short-circuits "don't add", "stop adding", "forget the X" and
// other cancel/refusal patterns so the retry never forces items the user
// explicitly didn't want (review W1).
func userRequestsItineraryCreation(text string) bool {
	if text == "" {
		return false
	}
	lower := strings.ToLower(text)

	// Negation / cancellation guard. If any of these patterns appear at all
	// in the message, the user is steering AWAY from itinerary changes —
	// suppress the trigger entirely. This is conservative (we may miss a
	// genuine "don't worry, build me an itinerary" with both phrases) but
	// the cost of a missed retry is much lower than the cost of forcing
	// items the user explicitly rejected.
	for _, neg := range []string{
		"don't add", "do not add", "dont add",
		"don't build", "do not build",
		"don't create", "do not create",
		"don't make", "do not make",
		"don't put", "do not put",
		"stop adding", "stop building",
		"forget the create_itinerary",
		"forget about the itinerary",
		"cancel the itinerary",
		"not the itinerary", "not my itinerary",
		"without adding",
	} {
		if strings.Contains(lower, neg) {
			return false
		}
	}

	for _, phrase := range []string{
		// Direct tool name mention — strongest signal. Must appear with
		// an action verb to avoid matching error-discussion contexts like
		// "the create_itinerary_items error is weird".
		"call create_itinerary_items",
		"use create_itinerary_items",
		"use the create_itinerary_items",
		"call the create_itinerary_items",
		"invoke create_itinerary_items",

		// "Build me / build out" — require an itinerary-context noun on
		// the right-hand side so we don't catch "build me a packing list".
		"build me an itinerary",
		"build me a day-by-day",
		"build me a day by day",
		"build me a 3-day",
		"build me a 4-day",
		"build me a 5-day",
		"build me a 7-day",
		"build me a 10-day",
		"build me a 14-day",
		"build me a 21-day",
		"build me a complete itinerary",
		"build me a detailed itinerary",
		"build me a full itinerary",
		"build me a plan for",
		"build me a trip",
		"build out a day-by-day",
		"build out my itinerary",
		"build out the itinerary",
		"build my itinerary",
		"build the itinerary",
		"build a day-by-day itinerary",
		"build a day by day itinerary",

		// "Create" — same noun-on-right rule
		"create a day-by-day itinerary",
		"create a day by day itinerary",
		"create me an itinerary",
		"create me a day-by-day",
		"create me a day by day",
		"create my itinerary",
		"create the itinerary",
		"create an itinerary",
		"create a 3-day itinerary",
		"create a 4-day itinerary",
		"create a 5-day itinerary",
		"create a 7-day itinerary",
		"create a 10-day itinerary",
		"create a 14-day itinerary",
		"create a 21-day itinerary",
		"create a 3-day plan",
		"create a 5-day plan",
		"create a 7-day plan",
		"create a 10-day plan",

		// "Give me a [N-day] itinerary / plan"
		"give me a day-by-day",
		"give me a day by day",
		"give me an itinerary",
		"give me a detailed itinerary",
		"give me a complete itinerary",
		"give me a full itinerary",

		// "Plan me / plan a [N] day"
		"plan me a",
		"plan a day-by-day",
		"plan a day by day",
		"plan a 3-day",
		"plan a 5-day",
		"plan a 7-day",
		"plan a 10-day",

		// "Add to my itinerary" / "add X to day Y"
		"add to my itinerary",
		"add to the itinerary",
		"add it to my itinerary",
		"add them to my itinerary",
		"add these to my itinerary",
		"add those to my itinerary",
		"add this to my itinerary",
		"add to day",
		"add a day trip",
		"add a stop",

		// Direct verb + structured-noun patterns
		"day-by-day itinerary",
		"day by day itinerary",
		"day-by-day plan",
		"full itinerary",
		"complete itinerary",
		"detailed itinerary",
		"my itinerary for",
	} {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	return false
}

// impliesExpertHandoff returns true when the AI's response text claims
// it will or did hand off to a specialist persona, without actually
// calling suggest_expert. Catches Run 6 R-16/N-07/N-12 cases where the
// AI role-plays the handoff in text but never fires the tool, so the
// frontend never receives the personaSwitch event.
//
// Detection requires BOTH an action phrase ("let me bring in", "I'll
// hand you off", etc.) AND a target token ("expert", "specialist") in
// the same response. This catches "I'll hand you off to our food
// specialist" while rejecting "let me bring in some examples" and
// "I'll connect you with the restaurant" (review W4).
func impliesExpertHandoff(text string) bool {
	if text == "" {
		return false
	}
	lower := strings.ToLower(text)

	// Target tokens — at least one of these must appear in the response.
	hasTarget := strings.Contains(lower, "expert") ||
		strings.Contains(lower, "specialist")
	if !hasTarget {
		return false
	}

	// Action phrases — at least one must appear. Each phrase must be a
	// verb construction that semantically means "I am about to delegate
	// to someone else", not a passive observation about an expert.
	for _, action := range []string{
		"let me bring in",
		"i'll bring in",
		"i am going to bring in",
		"i'm going to bring in",

		"let me hand you off",
		"let me hand this off",
		"i'll hand you off",
		"i'll hand this off",
		"handing you off",

		"passing you to",
		"transferring you to",

		"let me connect you with",
		"i'll connect you with",

		"let me pull in",
		"i'll pull in",

		"let me get our",
		"i'll get our",

		"the expert here is",
		"our specialist on",
		"our expert on",
		"our local expert on",
	} {
		if strings.Contains(lower, action) {
			// Extra guard: if the action is a generic "connect you with"
			// or "passing you to", make sure it's connecting them to a
			// PERSON (expert/specialist) and not a thing (restaurant,
			// hotel concierge, support desk). The hasTarget check above
			// ensures expert/specialist appears SOMEWHERE in the message,
			// but for these generic actions we additionally require the
			// target token to follow within ~80 chars.
			if action == "let me connect you with" ||
				action == "i'll connect you with" ||
				action == "passing you to" ||
				action == "transferring you to" {
				idx := strings.Index(lower, action)
				windowEnd := idx + len(action) + 80
				if windowEnd > len(lower) {
					windowEnd = len(lower)
				}
				window := lower[idx:windowEnd]
				if !strings.Contains(window, "expert") && !strings.Contains(window, "specialist") {
					continue
				}
			}
			return true
		}
	}
	return false
}

// mostRecentUserContent returns the text of the most recent message in the
// conversation that came from the user (Role == "user") and is NOT a
// system-injected nudge from a previous fabrication retry. The retry
// nudges have a "(Automated follow-up:" prefix that we explicitly skip so the
// detector keeps reading the real user intent across retries.
func mostRecentUserContent(messages []ai.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		m := messages[i]
		if m.Role != "user" {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(m.Content), "(Automated follow-up:") {
			continue
		}
		// Skip messages that only carry tool results (no human prose).
		if m.Content == "" && len(m.ToolResults) > 0 {
			continue
		}
		return m.Content
	}
	return ""
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
		defer func() {
			if r := recover(); r != nil {
				slog.Error("cache response goroutine panic recovered",
					"panic", fmt.Sprintf("%v", r),
					"session_id", sessionID,
				)
			}
		}()

		// Emit the full text as a single delta (no need to chunk for cached responses).
		sendOrDrop(outCh, ctx, StreamEvent{Type: "text_delta", Text: cachedText})

		// Store the assistant message.
		assistantMsg := &chatstore.ChatMessage{
			Role:    "assistant",
			Content: cachedText,
		}
		if err := s.chatStore.AddMessage(ctx, userID.String(), tripID, sessionID, assistantMsg); err != nil {
			slog.Error("failed to store cached assistant message", "error", err)
		}

		sendOrDrop(outCh, ctx, StreamEvent{
			Type:      "message_complete",
			Text:      cachedText,
			SessionID: sessionID,
			MessageID: assistantMsg.ID,
		})
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

// stripRetryArtifacts removes self-referential meta-narration that the AI
// sometimes produces after a fabrication retry. These sentences reference the
// retry mechanism or prior turns in a way that confuses the user.
//
// Run 7 N-16 example: "I actually did call create_itinerary_items in my
// previous response — here's a summary of what was saved:"
//
// The approach: scan for sentences containing artifact patterns, remove them,
// and trim leftover whitespace. This is deliberately conservative — only
// removing sentences that explicitly reference prior tool calls or responses.
func stripRetryArtifacts(text string) string {
	if text == "" {
		return text
	}

	lower := strings.ToLower(text)

	// Quick check: skip the expensive splitting if no artifact markers exist.
	hasArtifact := false
	for _, marker := range retryArtifactMarkers {
		if strings.Contains(lower, marker) {
			hasArtifact = true
			break
		}
	}
	if !hasArtifact {
		return text
	}

	// Split into sentences and filter. We use a simple heuristic: split on
	// sentence-ending punctuation followed by a space or newline.
	var cleaned strings.Builder
	remaining := text
	for remaining != "" {
		// Find the next sentence boundary.
		endIdx := -1
		for i := 0; i < len(remaining)-1; i++ {
			if (remaining[i] == '.' || remaining[i] == '!' || remaining[i] == '?') &&
				(remaining[i+1] == ' ' || remaining[i+1] == '\n') {
				endIdx = i + 1
				break
			}
		}

		var sentence string
		if endIdx == -1 {
			sentence = remaining
			remaining = ""
		} else {
			sentence = remaining[:endIdx]
			remaining = remaining[endIdx:]
		}

		sentLower := strings.ToLower(sentence)
		isArtifact := false
		for _, marker := range retryArtifactMarkers {
			if strings.Contains(sentLower, marker) {
				isArtifact = true
				break
			}
		}

		if !isArtifact {
			cleaned.WriteString(sentence)
		}
	}

	result := strings.TrimSpace(cleaned.String())
	if result == "" {
		// If stripping removed everything, return the original to avoid
		// sending an empty response.
		return text
	}
	return result
}

// classifyItineraryIntent uses a cheap LLM call to determine whether the
// user's message implies they want itinerary items created. This is a fallback
// for when the substring-based userRequestsItineraryCreation misses a phrasing.
//
// The classifier runs on the fast model tier with a small token budget to keep
// costs negligible (~$0.001/call). It returns true only when the LLM responds
// with "YES" — any error or ambiguous response defaults to false (no retry).
//
// This was introduced based on Run 7 analysis: the user suggested "you can
// always use a 2nd LLM call with a cheap model" to derive intent when the
// substring matcher fails.
func (s *Service) classifyItineraryIntent(ctx context.Context, userMsg, aiResponse string) bool {
	if userMsg == "" {
		return false
	}

	// Short-circuit: if the user message is very short and clearly not about
	// itinerary creation, skip the LLM call.
	if len(userMsg) < 15 {
		return false
	}

	classifyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req := &ai.ChatRequest{
		SystemPrompt: `You are a classification bot. Your ONLY job is to determine whether the user's message is asking for itinerary items to be created, added, or built.

Answer with exactly ONE word: YES or NO.

YES means: the user wants structured itinerary items (days, activities, meals, sightseeing) to be created or added to their trip plan.
NO means: the user is asking a question, making a comment, or discussing something that does NOT require creating itinerary items.

Examples:
"Build me a 10-day itinerary for Italy" → YES
"What's the best time to visit?" → NO
"Add a day trip to Pompeii" → YES
"Tell me about the food in Tokyo" → NO
"Plan my entire 7-day trip" → YES
"How do I get from the airport?" → NO`,
		Messages: []ai.Message{
			{Role: "user", Content: fmt.Sprintf("User message: %s\n\nAI response (no tool call made): %s", userMsg, aiResponse[:min(len(aiResponse), 200)])},
		},
		MaxTokens:   8,
		Temperature: 0,
		ModelTier:   ai.ModelTierFast,
	}

	eventCh, err := s.provider.ChatStream(classifyCtx, req)
	if err != nil {
		slog.Debug("intent classifier failed to start", "error", err)
		return false
	}

	var response strings.Builder
	for event := range eventCh {
		if event.Type == ai.EventTextDelta {
			response.WriteString(event.Text)
		}
		if event.Type == ai.EventError {
			slog.Debug("intent classifier error", "error", event.Error)
			return false
		}
	}

	result := strings.TrimSpace(strings.ToUpper(response.String()))
	isYes := strings.HasPrefix(result, "YES")
	if isYes {
		slog.Info("llm intent classifier: itinerary creation detected",
			"user_msg_preview", userMsg[:min(len(userMsg), 80)],
		)
	}
	return isYes
}

// retryArtifactMarkers are phrases that indicate self-referential meta-narration
// from a post-retry response. Sentences containing these are removed.
var retryArtifactMarkers = []string{
	"i actually did call",
	"i did call create_itinerary",
	"in my previous response",
	"in my earlier response",
	"as i mentioned in my previous",
	"as mentioned earlier",
	"i already called",
	"i have already called",
	"i've already called",
	"the tool was already called",
	"the items were already saved in my previous",
	"i called create_itinerary_items",
	"i used create_itinerary_items",
}
