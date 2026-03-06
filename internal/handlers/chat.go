package handlers

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/gallowaysoftware/toqui-backend/internal/ai/tools"
	"github.com/gallowaysoftware/toqui-backend/internal/auth"
	"github.com/gallowaysoftware/toqui-backend/internal/chat"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui-backend/internal/persona"
	"github.com/gallowaysoftware/toqui-backend/internal/theme"
	"github.com/gallowaysoftware/toqui-backend/internal/trip"
	"google.golang.org/protobuf/types/known/timestamppb"

	toquiv1 "github.com/gallowaysoftware/toqui-backend/gen/toqui/v1"
)

type ChatHandler struct {
	chatSvc  *chat.Service
	tripSvc  *trip.Service
	themeSvc *theme.Service
}

func NewChatHandler(chatSvc *chat.Service, tripSvc *trip.Service, themeSvc *theme.Service) *ChatHandler {
	return &ChatHandler{chatSvc: chatSvc, tripSvc: tripSvc, themeSvc: themeSvc}
}

func (h *ChatHandler) SendMessage(ctx context.Context, req *connect.Request[toquiv1.SendMessageRequest], stream *connect.ServerStream[toquiv1.SendMessageResponse]) error {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return connect.NewError(connect.CodeUnauthenticated, nil)
	}

	isSelection := req.Msg.Mode == toquiv1.ChatMode_CHAT_MODE_SELECTION || req.Msg.TripId == ""

	mode := "planning"
	switch {
	case isSelection:
		mode = "selection"
	case req.Msg.Mode == toquiv1.ChatMode_CHAT_MODE_COMPANION:
		mode = "companion"
	}

	// Look up trip context for persona resolution and system prompt injection
	var destinationCountry string
	var tripThemes []string
	var tripTitle, tripDescription string
	if !isSelection {
		if tripID, err := uuid.Parse(req.Msg.TripId); err == nil {
			if t, err := h.tripSvc.GetByID(ctx, userID, tripID); err == nil {
				tripTitle = t.Title
				if t.Description.Valid {
					tripDescription = t.Description.String
				}
				if t.DestinationCountry.Valid {
					destinationCountry = t.DestinationCountry.String
				}
			}
			if h.themeSvc != nil {
				if themes, err := h.themeSvc.GetTripThemes(ctx, tripID); err == nil {
					tripThemes = themes
				}
			}
		}
	}

	params := chat.SendMessageParams{
		UserID:             userID,
		TripID:             req.Msg.TripId,
		SessionID:          req.Msg.SessionId,
		Content:            req.Msg.Content,
		Mode:               mode,
		PersonaID:          req.Msg.PersonaId,
		DestinationCountry: destinationCountry,
		TripThemes:         tripThemes,
	}

	// Inject ephemeral location (companion mode only)
	if req.Msg.UserLocation != nil {
		params.LocationLat = req.Msg.UserLocation.Latitude
		params.LocationLng = req.Msg.UserLocation.Longitude
	}

	// Use mutex-protected slices to collect events from tool callbacks
	var createdTrips []tripCreatedInfo
	var selectedTrips []tripCreatedInfo // reuse same struct — it has ID, Title, Description
	var itineraryItems []dbgen.ItineraryItem
	var pendingSwitch *personaSwitchInfo
	var mu sync.Mutex

	// Suggest expert tool is available in all modes — Toqui can hand off anytime
	suggestExpertTool := NewSuggestExpertTool(h.chatSvc.Personas(), destinationCountry,
		func(previous, expert *persona.Persona, handoffMessage string) {
			mu.Lock()
			pendingSwitch = &personaSwitchInfo{
				Previous:        previous,
				Expert:          expert,
				HandoffMessage:  handoffMessage,
			}
			mu.Unlock()
		},
	)

	// Selection mode: add create_trip + select_trip tools
	// Planning mode: add create_itinerary_items tool
	if isSelection {
		createTripTool := NewCreateTripTool(h.tripSvc, userID, func(tripID, title, description string) {
			mu.Lock()
			createdTrips = append(createdTrips, tripCreatedInfo{ID: tripID, Title: title, Description: description})
			mu.Unlock()
		})
		selectTripTool := NewSelectTripTool(h.tripSvc, userID, func(tripID, title, description string) {
			mu.Lock()
			selectedTrips = append(selectedTrips, tripCreatedInfo{ID: tripID, Title: title, Description: description})
			mu.Unlock()
		})
		params.ExtraTools = []tools.Tool{createTripTool, selectTripTool, suggestExpertTool}
		params.ExtraSystemContext = h.buildSelectionContext(ctx, userID)
	} else {
		// Planning/companion mode: inject trip metadata so the AI knows what trip it's working on
		params.ExtraSystemContext = buildTripContext(tripTitle, tripDescription, destinationCountry, tripThemes)
		params.ExtraTools = append(params.ExtraTools, suggestExpertTool)

		// Planning mode: inject itinerary creation tool
		if tripID, err := uuid.Parse(req.Msg.TripId); err == nil {
			itineraryTool := NewCreateItineraryTool(h.tripSvc, tripID, func(items []dbgen.ItineraryItem) {
				mu.Lock()
				itineraryItems = append(itineraryItems, items...)
				mu.Unlock()
			})
			params.ExtraTools = append(params.ExtraTools, itineraryTool)
		}
	}

	eventCh, sessionID, err := h.chatSvc.SendMessage(ctx, params)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	// Send session created event if new session
	if req.Msg.SessionId == "" {
		if err := stream.Send(&toquiv1.SendMessageResponse{
			Event: &toquiv1.SendMessageResponse_SessionCreated{
				SessionCreated: &toquiv1.SessionCreated{
					SessionId: sessionID,
				},
			},
		}); err != nil {
			return err
		}
	}

	var fullContent string
	for event := range eventCh {
		var chatEvent *toquiv1.SendMessageResponse

		switch event.Type {
		case "text_delta":
			chatEvent = &toquiv1.SendMessageResponse{
				Event: &toquiv1.SendMessageResponse_TextDelta{
					TextDelta: &toquiv1.TextDelta{Text: event.Text},
				},
			}
		case "tool_call":
			chatEvent = &toquiv1.SendMessageResponse{
				Event: &toquiv1.SendMessageResponse_ToolCall{
					ToolCall: &toquiv1.ToolCall{
						ToolName:  event.ToolName,
						InputJson: event.ToolInput,
					},
				},
			}
		case "tool_result":
			chatEvent = &toquiv1.SendMessageResponse{
				Event: &toquiv1.SendMessageResponse_ToolResult{
					ToolResult: &toquiv1.ToolResult{
						ToolName:   event.ToolName,
						ResultJson: event.ToolResult,
					},
				},
			}

			// Emit TripCreated / TripSelected events after tool execution
			mu.Lock()
			if event.ToolName == "create_trip" {
				for _, ct := range createdTrips {
					_ = stream.Send(&toquiv1.SendMessageResponse{
						Event: &toquiv1.SendMessageResponse_TripCreated{
							TripCreated: &toquiv1.TripCreated{
								Trip: &toquiv1.Trip{
									Id:          ct.ID,
									UserId:      userID.String(),
									Title:       ct.Title,
									Description: ct.Description,
									Status:      toquiv1.TripStatus_TRIP_STATUS_PLANNING,
								},
							},
						},
					})
				}
				createdTrips = nil
			}
			if event.ToolName == "select_trip" {
				for _, st := range selectedTrips {
					_ = stream.Send(&toquiv1.SendMessageResponse{
						Event: &toquiv1.SendMessageResponse_TripSelected{
							TripSelected: &toquiv1.TripSelected{
								Trip: &toquiv1.Trip{
									Id:          st.ID,
									UserId:      userID.String(),
									Title:       st.Title,
									Description: st.Description,
								},
							},
						},
					})
				}
				selectedTrips = nil
			}
			if event.ToolName == "suggest_expert" && pendingSwitch != nil {
				_ = stream.Send(&toquiv1.SendMessageResponse{
					Event: &toquiv1.SendMessageResponse_PersonaSwitch{
						PersonaSwitch: &toquiv1.PersonaSwitch{
							PreviousPersona: personaToProto(pendingSwitch.Previous),
							NewPersona:      personaToProto(pendingSwitch.Expert),
							HandoffMessage:  pendingSwitch.HandoffMessage,
						},
					},
				})
				pendingSwitch = nil
			}
			if event.ToolName == "create_itinerary_items" && len(itineraryItems) > 0 {
				if tripID, err := uuid.Parse(req.Msg.TripId); err == nil {
					if allItems, err := h.tripSvc.GetItinerary(ctx, tripID); err == nil {
						_ = stream.Send(&toquiv1.SendMessageResponse{
							Event: &toquiv1.SendMessageResponse_ItineraryUpdate{
								ItineraryUpdate: &toquiv1.ItineraryUpdate{
									TripId:    req.Msg.TripId,
									Itinerary: itineraryToProto(req.Msg.TripId, allItems),
								},
							},
						})
					}
				}
				itineraryItems = nil
			}
			mu.Unlock()

		case "message_complete":
			fullContent = event.Text
			chatEvent = &toquiv1.SendMessageResponse{
				Event: &toquiv1.SendMessageResponse_MessageComplete{
					MessageComplete: &toquiv1.MessageComplete{
						MessageId:   event.MessageID,
						SessionId:   event.SessionID,
						FullContent: event.Text,
					},
				},
			}
		case "error":
			chatEvent = &toquiv1.SendMessageResponse{
				Event: &toquiv1.SendMessageResponse_Error{
					Error: &toquiv1.ErrorEvent{Message: event.Error},
				},
			}
		default:
			continue
		}

		if err := stream.Send(chatEvent); err != nil {
			return err
		}
	}

	// Retag themes if the trip has none yet
	if !isSelection {
		if tripID, err := uuid.Parse(req.Msg.TripId); err == nil && h.themeSvc != nil && fullContent != "" {
			if len(tripThemes) == 0 {
				if t, err := h.tripSvc.GetByID(ctx, userID, tripID); err == nil {
					recentMessages := []string{req.Msg.Content, fullContent}
					go func() {
						if err := h.themeSvc.TagTrip(context.Background(), tripID, t.Title, t.Description.String, recentMessages); err != nil {
							log.Printf("chat retag trip %s: %v", tripID, err)
						}
					}()
				}
			}
		}
	}

	return nil
}

type tripCreatedInfo struct {
	ID, Title, Description string
}

type personaSwitchInfo struct {
	Previous       *persona.Persona
	Expert         *persona.Persona
	HandoffMessage string
}

// buildTripContext returns system prompt context for planning/companion mode:
// the trip's metadata so the AI knows what it's helping with.
func buildTripContext(title, description, destinationCountry string, themes []string) string {
	if title == "" && description == "" && destinationCountry == "" {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("CURRENT TRIP CONTEXT:\n")
	if title != "" {
		sb.WriteString(fmt.Sprintf("- Title: %s\n", title))
	}
	if description != "" {
		sb.WriteString(fmt.Sprintf("- Description: %s\n", description))
	}
	if destinationCountry != "" {
		sb.WriteString(fmt.Sprintf("- Destination country: %s\n", destinationCountry))
	}
	if len(themes) > 0 {
		sb.WriteString(fmt.Sprintf("- Trip themes: %s\n", strings.Join(themes, ", ")))
	}
	sb.WriteString("\nUse this context to give specific, relevant advice. Do NOT ask the user where they are going — you already know from the trip details above.")
	sb.WriteString("\n\nWhen you have specific activities, meals, or experiences to suggest, use the create_itinerary_items tool to add them to the itinerary. Don't just describe what the user could do — actually add it to their plan. You can add multiple items across multiple days in a single call.")
	return sb.String()
}

// buildSelectionContext returns system prompt context for selection mode:
// the user's existing trips so Toqui can help them find or create one.
func (h *ChatHandler) buildSelectionContext(ctx context.Context, userID uuid.UUID) string {
	trips, _, err := h.tripSvc.ListByUser(ctx, userID, "", 20, 0)
	if err != nil || len(trips) == 0 {
		return `You are in SELECTION mode — no trip is selected yet.

Help the user decide on a trip. You can:
- Help them brainstorm destinations and trip ideas
- Create a trip for them when they're ready (use the create_trip tool)

The user has no existing trips yet. Help them get started!

When the user expresses interest in a specific destination or trip idea, proactively create the trip for them using the create_trip tool. Don't wait for them to explicitly say "create a trip" — if they say something like "I want to go to Japan" or "planning a weekend in Paris", go ahead and create it.`
	}

	var sb strings.Builder
	sb.WriteString(`You are in SELECTION mode — no trip is selected yet.

Help the user decide on a trip. You can:
- Help them brainstorm destinations and trip ideas
- Select an existing trip if the user refers to one (use the select_trip tool with the trip_id)
- Create a NEW trip when they're ready (use the create_trip tool)

IMPORTANT: When the user vaguely refers to an existing trip (e.g., "that Japan thing", "continue planning my Europe trip", "the one from last week"), use your best judgment to match it to a trip from the list below and call select_trip. Always briefly acknowledge which trip you're selecting before calling the tool, e.g., "Let me pull up your Greek Islands trip!" If you're unsure which trip they mean, ask them to clarify.

When the user expresses interest in a NEW destination or trip idea, proactively create the trip using create_trip. Don't wait for them to explicitly say "create a trip" — if they say something like "I want to go to Japan" or "planning a weekend in Paris" and there's no matching existing trip, go ahead and create it.

The user's existing trips:
`)
	for _, t := range trips {
		status := t.Status
		sb.WriteString(fmt.Sprintf("- %s (id: %s, status: %s", t.Title, t.ID, status))
		if t.DestinationCountry.Valid && t.DestinationCountry.String != "" {
			sb.WriteString(fmt.Sprintf(", destination: %s", t.DestinationCountry.String))
		}
		sb.WriteString(")")
		if t.Description.Valid && t.Description.String != "" {
			sb.WriteString(fmt.Sprintf(" — %s", t.Description.String))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func (h *ChatHandler) GetChatHistory(ctx context.Context, req *connect.Request[toquiv1.GetChatHistoryRequest]) (*connect.Response[toquiv1.GetChatHistoryResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	limit := int(req.Msg.GetPagination().GetPageSize())
	if limit == 0 {
		limit = 50
	}

	messages, err := h.chatSvc.GetHistory(ctx, userID, req.Msg.TripId, req.Msg.SessionId, limit)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	protoMessages := make([]*toquiv1.ChatMessage, len(messages))
	for i, m := range messages {
		protoMessages[i] = &toquiv1.ChatMessage{
			Id:        m.ID,
			SessionId: m.SessionID,
			Role:      m.Role,
			Content:   m.Content,
			Metadata:  m.Metadata,
			CreatedAt: timestamppb.New(m.CreatedAt),
		}
	}

	return connect.NewResponse(&toquiv1.GetChatHistoryResponse{
		Messages: protoMessages,
	}), nil
}

func (h *ChatHandler) ListChatSessions(ctx context.Context, req *connect.Request[toquiv1.ListChatSessionsRequest]) (*connect.Response[toquiv1.ListChatSessionsResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	limit := int(req.Msg.GetPagination().GetPageSize())
	if limit == 0 {
		limit = 20
	}

	sessions, err := h.chatSvc.ListSessions(ctx, userID, req.Msg.TripId, limit)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	protoSessions := make([]*toquiv1.ChatSession, len(sessions))
	for i, s := range sessions {
		protoSessions[i] = &toquiv1.ChatSession{
			Id:            s.ID,
			TripId:        s.TripID,
			Mode:          chatModeFromString(s.Mode),
			CreatedAt:     timestamppb.New(s.CreatedAt),
			LastMessageAt: timestamppb.New(s.LastMessageAt),
		}
	}

	return connect.NewResponse(&toquiv1.ListChatSessionsResponse{
		Sessions: protoSessions,
	}), nil
}

func chatModeFromString(mode string) toquiv1.ChatMode {
	switch mode {
	case "planning":
		return toquiv1.ChatMode_CHAT_MODE_PLANNING
	case "companion":
		return toquiv1.ChatMode_CHAT_MODE_COMPANION
	case "selection":
		return toquiv1.ChatMode_CHAT_MODE_SELECTION
	default:
		return toquiv1.ChatMode_CHAT_MODE_UNSPECIFIED
	}
}
