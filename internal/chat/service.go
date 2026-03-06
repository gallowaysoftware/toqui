package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
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
}

func NewService(provider ai.Provider, chatStore *chatstore.Store, toolRegistry *tools.Registry, personas *persona.Registry) *Service {
	return &Service{
		provider:  provider,
		chatStore: chatStore,
		tools:     toolRegistry,
		personas:  personas,
	}
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

	// Build AI request
	messages := make([]ai.Message, 0, len(history))
	for _, msg := range history {
		messages = append(messages, ai.Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
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
		MaxTokens:    4096,
		Temperature:  0.7,
		Mode:         params.Mode,
	}

	// Classify the request to determine model tier. This uses deterministic
	// heuristics based on mode, message length, and tool availability.
	tier := ai.ClassifyRequest(aiReq)
	aiReq.ModelTier = tier

	// Apply tier-specific defaults for max tokens and temperature if the
	// caller did not explicitly set them.
	tierCfg := ai.ConfigForTier(tier)
	if aiReq.MaxTokens == 4096 {
		// Only override from the tier config when using the default value,
		// so explicit caller overrides are preserved.
		aiReq.MaxTokens = tierCfg.MaxTokens
	}

	slog.Info("chat request classified",
		"mode", params.Mode,
		"tier", tier,
		"provider", s.provider.Name(),
		"has_tools", len(toolDefs) > 0,
	)

	// Start streaming
	eventCh, err := s.provider.ChatStream(ctx, aiReq)
	if err != nil {
		return nil, "", fmt.Errorf("start chat stream: %w", err)
	}

	// Build extra tools map for execution
	extraToolsMap := make(map[string]tools.Tool, len(params.ExtraTools))
	for _, t := range params.ExtraTools {
		extraToolsMap[t.Definition().Name] = t
	}

	outCh := make(chan StreamEvent, 64)
	go func() {
		defer close(outCh)
		s.processEvents(ctx, eventCh, outCh, extraToolsMap, params.UserID, storeTripID, sessionID)
	}()

	return outCh, sessionID, nil
}

func (s *Service) processEvents(ctx context.Context, eventCh <-chan ai.Event, outCh chan<- StreamEvent, extraTools map[string]tools.Tool, userID uuid.UUID, tripID, sessionID string) {
	var fullResponse strings.Builder

	for event := range eventCh {
		select {
		case <-ctx.Done():
			return
		default:
		}

		switch event.Type {
		case ai.EventTextDelta:
			fullResponse.WriteString(event.Text)
			outCh <- StreamEvent{Type: "text_delta", Text: event.Text}

		case ai.EventToolCall:
			if event.Tool != nil {
				outCh <- StreamEvent{
					Type:      "tool_call",
					ToolName:  event.Tool.Name,
					ToolInput: event.Tool.Arguments,
				}

				// Execute tool — check extra tools first, then global registry
				var result json.RawMessage
				var execErr error
				if extra, ok := extraTools[event.Tool.Name]; ok {
					result, execErr = extra.Execute(ctx, json.RawMessage(event.Tool.Arguments))
				} else {
					result, execErr = s.tools.Execute(ctx, event.Tool.Name, []byte(event.Tool.Arguments))
				}

				if execErr != nil {
					outCh <- StreamEvent{
						Type:       "tool_result",
						ToolName:   event.Tool.Name,
						ToolResult: fmt.Sprintf(`{"error": "%s"}`, execErr.Error()),
					}
				} else {
					outCh <- StreamEvent{
						Type:       "tool_result",
						ToolName:   event.Tool.Name,
						ToolResult: string(result),
					}
				}
			}

		case ai.EventDone:
			// Store assistant message
			assistantMsg := &chatstore.ChatMessage{
				Role:    "assistant",
				Content: fullResponse.String(),
			}
			_ = s.chatStore.AddMessage(ctx, userID.String(), tripID, sessionID, assistantMsg)

			outCh <- StreamEvent{
				Type:      "message_complete",
				Text:      fullResponse.String(),
				SessionID: sessionID,
				MessageID: assistantMsg.ID,
			}

		case ai.EventError:
			outCh <- StreamEvent{Type: "error", Error: event.Error.Error()}
		}
	}
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
	// Verify session exists and belongs to user
	_, err := s.chatStore.GetSession(ctx, userID.String(), tripID, sessionID)
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}
	return s.chatStore.GetMessages(ctx, userID.String(), tripID, sessionID, limit)
}
