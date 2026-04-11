package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gallowaysoftware/toqui-backend/internal/affiliate"
	"github.com/gallowaysoftware/toqui-backend/internal/ai"
	"github.com/gallowaysoftware/toqui-backend/internal/ai/tools"
	"github.com/gallowaysoftware/toqui-backend/internal/analytics"
	"github.com/gallowaysoftware/toqui-backend/internal/auth"
	"github.com/gallowaysoftware/toqui-backend/internal/chat"
	"github.com/gallowaysoftware/toqui-backend/internal/chatstore"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui-backend/internal/location"
	"github.com/gallowaysoftware/toqui-backend/internal/payment"
	"github.com/gallowaysoftware/toqui-backend/internal/persona"
	"github.com/gallowaysoftware/toqui-backend/internal/theme"
	"github.com/gallowaysoftware/toqui-backend/internal/tier"
	"github.com/gallowaysoftware/toqui-backend/internal/trip"
	"github.com/gallowaysoftware/toqui-backend/internal/usage"

	toquiv1 "github.com/gallowaysoftware/toqui-backend/gen/toqui/v1"
)

// dateFormatLong is the Go reference-time layout for human-readable dates
// like dateFormatLong. Used in system prompts and trip date formatting.
const dateFormatLong = "January 2, 2006"

type ChatHandler struct {
	chatSvc       *chat.Service
	tripSvc       *trip.Service
	themeSvc      *theme.Service
	locationCache *location.Cache
	locationSvc   *location.Service
	linkBuilder   *affiliate.LinkBuilder
	usageSvc      *usage.Service
	paymentSvc    *payment.Service
	aiProvider    ai.Provider // for companion gate LLM classifier
	queries       *dbgen.Queries
	pool          *pgxpool.Pool
	placesAPIKey  string
	adminEmails   map[string]bool
	analytics     *analytics.Client
}

func NewChatHandler(chatSvc *chat.Service, tripSvc *trip.Service, themeSvc *theme.Service, locationCache *location.Cache, locationSvc *location.Service, linkBuilder *affiliate.LinkBuilder, usageSvc *usage.Service, paymentSvc *payment.Service, pool *pgxpool.Pool, adminEmails []string) *ChatHandler {
	emailSet := make(map[string]bool, len(adminEmails))
	for _, e := range adminEmails {
		emailSet[strings.ToLower(strings.TrimSpace(e))] = true
	}
	return &ChatHandler{
		chatSvc:       chatSvc,
		tripSvc:       tripSvc,
		themeSvc:      themeSvc,
		locationCache: locationCache,
		locationSvc:   locationSvc,
		paymentSvc:    paymentSvc,
		linkBuilder:   linkBuilder,
		usageSvc:      usageSvc,
		queries:       dbgen.New(pool),
		pool:          pool,
		adminEmails:   emailSet,
	}
}

// WithPlacesAPIKey configures the chat handler to geocode itinerary item
// locations using the Google Places/Geocoding API. If the key is empty,
// geocoding is silently skipped.
func (h *ChatHandler) WithPlacesAPIKey(key string) *ChatHandler {
	h.placesAPIKey = key
	return h
}

// WithAnalytics configures the chat handler to send events to PostHog.
func (h *ChatHandler) WithAnalytics(client *analytics.Client) *ChatHandler {
	h.analytics = client
	return h
}

// WithAIProvider configures the AI provider for the companion mode intent
// gate. The gate uses a fast-tier LLM call to classify user intent.
func (h *ChatHandler) WithAIProvider(provider ai.Provider) *ChatHandler {
	h.aiProvider = provider
	return h
}

func (h *ChatHandler) isAdmin(ctx context.Context, userID uuid.UUID) bool {
	if len(h.adminEmails) == 0 {
		return false
	}
	user, err := h.queries.GetUserByID(ctx, userID)
	if err != nil {
		return false
	}
	return h.adminEmails[strings.ToLower(user.Email)]
}

func (h *ChatHandler) SendMessage(ctx context.Context, req *connect.Request[toquiv1.SendMessageRequest], stream *connect.ServerStream[toquiv1.SendMessageResponse]) error {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return connect.NewError(connect.CodeUnauthenticated, nil)
	}

	// Validate request — the protovalidate interceptor only covers unary RPCs,
	// so server-streaming RPCs need explicit validation here.
	if err := protovalidate.Validate(req.Msg); err != nil {
		var ve *protovalidate.ValidationError
		if errors.As(err, &ve) {
			return connect.NewError(connect.CodeInvalidArgument, ve)
		}
		return connect.NewError(connect.CodeInternal, err)
	}

	// Validate attachments
	if err := validateAttachments(req.Msg.Attachments); err != nil {
		return connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Admin users have unlimited AI interactions.
	isAdmin := h.isAdmin(ctx, userID)

	// Look up the user's subscription tier for usage limits and booking recommendation gating.
	// Default to free tier if the lookup fails so we never block the chat flow.
	userTier := tier.Free
	if h.queries != nil {
		if raw, err := h.queries.GetUserSubscriptionTier(ctx, userID); err == nil {
			userTier = tier.Parse(raw)
		} else {
			slog.Warn("failed to look up user subscription tier, defaulting to free",
				"user_id", userID, "error", err)
		}
	}

	// Companion mode can work without a trip (standalone), so don't force it to selection.
	isSelection := req.Msg.Mode == toquiv1.ChatMode_CHAT_MODE_SELECTION ||
		(req.Msg.TripId == "" && req.Msg.Mode != toquiv1.ChatMode_CHAT_MODE_COMPANION)

	mode := "planning"
	switch {
	case isSelection:
		mode = "selection"
	case req.Msg.Mode == toquiv1.ChatMode_CHAT_MODE_COMPANION:
		mode = "companion"
	}

	// Check daily message limit before processing (skip for admins).
	// Selection-mode messages are exempt from the quota — they're orientation
	// interactions before any trip exists, and counting them against the
	// daily cap meant first-time users could be blocked mid-planning by
	// throwaway "where should I go?" turns (#191).
	if h.usageSvc != nil && !isAdmin && !isSelection {
		limit := h.usageSvc.LimitForTier(userTier)
		remaining, err := h.usageSvc.IncrementAndCheckTier(ctx, userID, userTier)
		if err != nil {
			if errors.Is(err, usage.ErrDailyLimitExceeded) {
				upgradeHint := ""
				if userTier.IsFree() {
					upgradeHint = " Upgrade to Explorer or Voyager for unlimited messages — visit your account settings to learn more."
				} else if !userTier.IsUnlimited() {
					upgradeHint = " Upgrade to Explorer or Voyager for unlimited messages."
				}
				return connect.NewError(
					connect.CodeResourceExhausted,
					fmt.Errorf("you have reached your daily message limit of %d messages; it resets at %s.%s",
						limit, usage.ResetTime().Format("2006-01-02T15:04:05Z"), upgradeHint),
				)
			}
			// Log but don't block on usage tracking errors
			slog.Error("usage tracking failed", "user_id", userID, "error", err)
		} else {
			slog.Debug("daily usage tracked", "user_id", userID, "remaining", remaining, "tier", string(userTier))
		}
	}

	// Track chat message (async, non-blocking, privacy-safe — no message content)
	if h.analytics != nil {
		h.analytics.Track(userID.String(), "chat_message_sent", map[string]any{
			"mode": mode,
		})
	}

	// Look up trip context for persona resolution and system prompt injection
	var destinationCountry string
	var destinationCountries []string
	var tripThemes []string
	var tripTitle, tripDescription string
	var tripStartDate, tripEndDate string
	var tripStartDateISO, tripEndDateISO string // YYYY-MM-DD for affiliate URLs
	var tripStatus string
	var existingItinerary []dbgen.ItineraryItem
	var existingBookings []dbgen.Booking
	var collaboratorCount int64
	if !isSelection {
		if tripID, err := uuid.Parse(req.Msg.TripId); err == nil {
			if t, err := h.tripSvc.GetByID(ctx, userID, tripID); err == nil {
				tripTitle = t.Title
				tripStatus = t.Status
				if t.Description.Valid {
					tripDescription = t.Description.String
				}
				if t.DestinationCountry.Valid {
					destinationCountry = t.DestinationCountry.String
				}
				// Multi-country trips: load all destinations for the system prompt (#133)
				if len(t.DestinationCountries) > 0 {
					destinationCountries = t.DestinationCountries
				} else if destinationCountry != "" {
					destinationCountries = []string{destinationCountry}
				}
				if t.StartDate.Valid {
					tripStartDate = t.StartDate.Time.Format(dateFormatLong)
					tripStartDateISO = t.StartDate.Time.Format("2006-01-02")
				}
				if t.EndDate.Valid {
					tripEndDate = t.EndDate.Time.Format(dateFormatLong)
					tripEndDateISO = t.EndDate.Time.Format("2006-01-02")
				}
			}
			if h.themeSvc != nil {
				if themes, err := h.themeSvc.GetTripThemes(ctx, tripID); err == nil {
					tripThemes = themes
				}
			}
			// Load existing itinerary items for AI context
			if items, err := h.tripSvc.GetItinerary(ctx, tripID); err == nil {
				existingItinerary = items
			}
			// Load existing bookings for AI context
			if h.queries != nil {
				if bk, err := h.queries.ListBookingsByTrip(ctx, dbgen.ListBookingsByTripParams{
					TripID: pgtype.UUID{Bytes: tripID, Valid: true},
					UserID: userID,
				}); err == nil {
					existingBookings = bk
				}
			}
			// Load collaborator count for AI context
			if h.queries != nil {
				if count, err := h.queries.CountCollaboratorsByTrip(ctx, tripID); err == nil {
					collaboratorCount = count
				}
			}
		}
	}

	// Convert proto attachments to chat service attachments
	var attachments []chat.Attachment
	for _, a := range req.Msg.Attachments {
		attachments = append(attachments, chat.Attachment{
			Filename:  a.Filename,
			MediaType: a.MediaType,
			Data:      a.Data,
		})
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
		Attachments:        attachments,
	}

	// Inject ephemeral location (companion mode only).
	// Priority: request-level location > cached location from UpdateLocation RPC.
	if req.Msg.UserLocation != nil {
		params.LocationLat = req.Msg.UserLocation.Latitude
		params.LocationLng = req.Msg.UserLocation.Longitude
	} else if mode == "companion" && h.locationCache != nil {
		if cached := h.locationCache.Get(userID); cached != nil {
			params.LocationLat = cached.Lat
			params.LocationLng = cached.Lng
			slog.Debug("injected cached location into companion chat",
				"user_id", userID,
				"lat", cached.Lat,
				"lng", cached.Lng,
			)
		}
	}

	// Use mutex-protected slices to collect events from tool callbacks
	var createdTrips []tripCreatedInfo
	var selectedTrips []tripCreatedInfo // reuse same struct — it has ID, Title, Description
	var updatedTrips []tripUpdatedInfo
	var itineraryItems []dbgen.ItineraryItem
	var pendingSwitch *personaSwitchInfo
	var recommendations []affiliate.Recommendation
	var createdTripID string  // first trip created this session (for session relinking, #153)
	var selectedTripID string // most recent trip selected this session (survives copy-and-clear)
	var mu sync.Mutex

	// Check if this trip is unlocked (Trip Pro purchased or trial active)
	var tripUnlocked bool
	if parsedTripID, parseErr := uuid.Parse(req.Msg.TripId); parseErr == nil {
		if h.paymentSvc != nil {
			tripUnlocked, _ = h.paymentSvc.IsTripUnlocked(ctx, userID, parsedTripID)
		}
		if !tripUnlocked && h.queries != nil {
			if active, err := h.queries.IsTripTrialActive(ctx, parsedTripID); err == nil && active {
				tripUnlocked = true
			}
		}
	}

	// personaSwitchCh signals a mid-turn handoff to the chat service so the
	// expert can answer in the same turn (#175). Buffered so the suggest_expert
	// callback never blocks; only the most recent switch wins per turn.
	personaSwitchCh := make(chan *persona.Persona, 1)
	params.PersonaSwitchCh = personaSwitchCh

	// Suggest expert tool — free users get 3 teaser messages, then upgrade prompt
	suggestExpertTool := NewSuggestExpertTool(h.chatSvc.Personas(), destinationCountry,
		func(previous, expert *persona.Persona, handoffMessage string) {
			mu.Lock()
			pendingSwitch = &personaSwitchInfo{
				Previous:       previous,
				Expert:         expert,
				HandoffMessage: handoffMessage,
			}
			mu.Unlock()

			// Notify the chat service to swap the system prompt for the next
			// tool loop iteration. Non-blocking — if the channel is already
			// full, the previous switch is overwritten by draining first.
			select {
			case personaSwitchCh <- expert:
			default:
				<-personaSwitchCh
				personaSwitchCh <- expert
			}
		},
	)

	// In selection mode, add a deferred destination resolver so
	// suggest_expert can resolve the country after create_trip fires
	// in the same turn (Run 12 R-16 P2: no_destination error).
	if isSelection {
		suggestExpertTool = suggestExpertTool.WithDeferredDestination(func() string {
			mu.Lock()
			defer mu.Unlock()
			if createdTripID != "" {
				if id, parseErr := uuid.Parse(createdTripID); parseErr == nil {
					if t, getErr := h.tripSvc.GetByID(context.Background(), userID, id); getErr == nil && t.DestinationCountry.Valid {
						return t.DestinationCountry.String
					}
				}
			}
			return ""
		})
	}

	// Wrap suggest_expert for free-tier users to enforce per-trip expert limit.
	// The counter is persisted in the DB (trips.expert_calls) so it survives
	// across messages and sessions, unlike the old per-RPC atomic counter.
	var expertTool tools.Tool = suggestExpertTool
	if !userTier.IsPro() && !tripUnlocked {
		gateTripID := uuid.Nil
		if id, err := uuid.Parse(req.Msg.TripId); err == nil {
			gateTripID = id
		}
		expertTool = newExpertTeaserGate(suggestExpertTool, h.queries, gateTripID, userID)
	}

	// Recommend booking tool is available in all modes. Free-tier users get
	// affiliate-linked results; Pro-tier users get unbiased recommendations.
	var recommendBookingTool tools.Tool
	if h.linkBuilder != nil {
		rbt := NewRecommendBookingTool(h.linkBuilder, userTier, func(rec affiliate.Recommendation) {
			mu.Lock()
			recommendations = append(recommendations, rec)
			mu.Unlock()
		})
		if destinationCountry != "" || tripStartDateISO != "" {
			rbt = rbt.WithTripContext(destinationCountry, tripStartDateISO, tripEndDateISO)
		}
		recommendBookingTool = rbt
	}

	// Selection mode: add create_trip + select_trip tools
	// Planning mode: add create_itinerary_items tool
	if isSelection {
		createTripTool := NewCreateTripTool(h.tripSvc, userID, func(tripID, title, description string) {
			mu.Lock()
			createdTrips = append(createdTrips, tripCreatedInfo{ID: tripID, Title: title, Description: description})
			if createdTripID == "" {
				createdTripID = tripID // save for session relinking after stream ends
			}
			mu.Unlock()
		})
		selectTripTool := NewSelectTripTool(h.tripSvc, userID, func(tripID, title, description string) {
			mu.Lock()
			selectedTrips = append(selectedTrips, tripCreatedInfo{ID: tripID, Title: title, Description: description})
			selectedTripID = tripID // persist across copy-and-clear (#230)
			mu.Unlock()
		})

		// Inject create_itinerary_items into selection mode with a deferred
		// trip-ID provider. The provider returns the trip created earlier in
		// the same turn (or, failing that, the most recently selected trip),
		// so an expert handed off after create_trip can immediately persist
		// itinerary items in the same turn (#181).
		deferredItineraryTool := NewCreateItineraryTool(h.tripSvc, uuid.Nil, func(items []dbgen.ItineraryItem) {
			mu.Lock()
			itineraryItems = append(itineraryItems, items...)
			mu.Unlock()
		}).
			WithGeocoding(h.pool, h.placesAPIKey).
			WithAnalytics(h.analytics, userID.String()).
			WithDeferredTripID(func() (uuid.UUID, bool) {
				mu.Lock()
				defer mu.Unlock()
				// 1. Trip created in this turn (never cleared)
				if createdTripID != "" {
					if id, err := uuid.Parse(createdTripID); err == nil {
						return id, true
					}
				}
				// 2. Trip selected in this turn (survives copy-and-clear, #230)
				if selectedTripID != "" {
					if id, err := uuid.Parse(selectedTripID); err == nil {
						return id, true
					}
				}
				// 3. Pending select_trip result (may be cleared by event handler)
				if len(selectedTrips) > 0 {
					if id, err := uuid.Parse(selectedTrips[len(selectedTrips)-1].ID); err == nil {
						return id, true
					}
				}
				// 4. Trip ID from request (subsequent messages in selection mode)
				if req.Msg.TripId != "" {
					if id, err := uuid.Parse(req.Msg.TripId); err == nil {
						return id, true
					}
				}
				return uuid.Nil, false
			})

		params.ExtraTools = []tools.Tool{createTripTool, selectTripTool, expertTool, deferredItineraryTool}
		if recommendBookingTool != nil {
			params.ExtraTools = append(params.ExtraTools, recommendBookingTool)
		}
		params.ExtraSystemContext = h.buildSelectionContext(ctx, userID, userTier)
	} else {
		// Planning/companion mode: inject trip metadata so the AI knows what trip it's working on
		params.ExtraSystemContext = buildTripContext(tripTitle, tripDescription, destinationCountry, destinationCountries, tripStartDate, tripEndDate, tripStatus, tripThemes, existingItinerary, existingBookings, collaboratorCount, userTier)
	}

	// Inject user preferences into the system context (all modes).
	// Preferences are appended after mode-specific context so the AI
	// always has access to remembered dietary, budget, pace, etc.
	if h.queries != nil {
		if prefMap, err := loadPreferencesMap(ctx, h.queries, userID); err != nil {
			slog.Warn("failed to load user preferences for chat context", "user_id", userID, "error", err)
		} else if len(prefMap) > 0 {
			params.ExtraSystemContext += buildPreferencesContext(prefMap)
		}
	}

	if !isSelection {
		params.ExtraTools = append(params.ExtraTools, expertTool)
		if recommendBookingTool != nil {
			params.ExtraTools = append(params.ExtraTools, recommendBookingTool)
		}

		// Inject itinerary creation + deletion tools.
		//
		// Planning mode: real tools that persist/delete items. Only
		// registered when tripID parses.
		//
		// Companion mode: same real tools but wrapped in CompanionGate
		// which blocks calls unless the user explicitly asks to modify
		// the itinerary. This prevents the Run 5/Run 8 over-eagerness
		// regression where Gemini interprets "recommend a lunch spot"
		// as "add a lunch spot to the itinerary".
		if mode == "planning" || mode == "companion" {
			if tripID, err := uuid.Parse(req.Msg.TripId); err == nil {
				var createTool tools.Tool = NewCreateItineraryTool(h.tripSvc, tripID, func(items []dbgen.ItineraryItem) {
					mu.Lock()
					itineraryItems = append(itineraryItems, items...)
					mu.Unlock()
				}).WithGeocoding(h.pool, h.placesAPIKey).
					WithAnalytics(h.analytics, userID.String())

				var deleteTool tools.Tool = NewDeleteItineraryTool(h.tripSvc, tripID, userID, func(deletedIDs []string) {
					slog.Info("itinerary items deleted via chat", "count", len(deletedIDs), "trip_id", tripID)
				})

				updateTripTool := NewUpdateTripTool(h.tripSvc, tripID, userID, func(id, title, description string, countries []string) {
					mu.Lock()
					updatedTrips = append(updatedTrips, tripUpdatedInfo{
						ID:          id,
						Title:       title,
						Description: description,
						Countries:   countries,
					})
					mu.Unlock()
				})

				// In companion mode, wrap tools with an intent gate that
				// only allows calls when the user explicitly requests
				// itinerary changes ("add this to my plan", "remove the
				// museum visit"). Info queries pass through ungated.
				if mode == "companion" && h.aiProvider != nil {
					userMsg := req.Msg.Content
					getUserMsg := func() string { return userMsg }
					createTool = NewCompanionGate(createTool, h.aiProvider, getUserMsg)
					deleteTool = NewCompanionGate(deleteTool, h.aiProvider, getUserMsg)
				}

				params.ExtraTools = append(params.ExtraTools, createTool, deleteTool, updateTripTool)
			}
		}

		// Companion mode: inject nearby_places tool with user's cached location
		if mode == "companion" && h.locationSvc != nil {
			nearbyTool := NewNearbyPlacesTool(h.locationSvc, params.LocationLat, params.LocationLng)
			params.ExtraTools = append(params.ExtraTools, nearbyTool)
		}
	}

	eventCh, sessionID, err := h.chatSvc.SendMessage(ctx, params)
	if err != nil {
		if errors.Is(err, ai.ErrBudgetExhausted) {
			return connect.NewError(
				connect.CodeResourceExhausted,
				fmt.Errorf("our AI service has reached its daily capacity — please try again tomorrow"),
			)
		}
		return internalError(ctx, "send message", err)
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
	hadContent := false // Track if any AI content was received (for usage refund on failure)
	for event := range eventCh {
		var chatEvent *toquiv1.SendMessageResponse

		switch event.Type {
		case "text_delta":
			hadContent = true
			chatEvent = &toquiv1.SendMessageResponse{
				Event: &toquiv1.SendMessageResponse_TextDelta{
					TextDelta: &toquiv1.TextDelta{Text: event.Text},
				},
			}
		case "tool_call":
			hadContent = true
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

			// Copy-and-clear under the lock, then send outside it.
			// This avoids holding the mutex during stream I/O and prevents
			// duplicate proto events across tool loop iterations (#52).
			mu.Lock()
			var localCreated []tripCreatedInfo
			var localSelected []tripCreatedInfo
			var localSwitch *personaSwitchInfo
			var localItinerary []dbgen.ItineraryItem
			var localRecs []affiliate.Recommendation

			if event.ToolName == "create_trip" {
				localCreated = createdTrips
				createdTrips = nil
			}
			if event.ToolName == "select_trip" {
				localSelected = selectedTrips
				selectedTrips = nil
			}
			if event.ToolName == "suggest_expert" {
				localSwitch = pendingSwitch
				pendingSwitch = nil
			}
			if event.ToolName == "create_itinerary_items" {
				localItinerary = itineraryItems
				itineraryItems = nil
			}
			if event.ToolName == "recommend_booking" {
				localRecs = recommendations
				recommendations = nil
			}
			var localUpdated []tripUpdatedInfo
			if event.ToolName == "update_trip" {
				localUpdated = updatedTrips
				updatedTrips = nil
			}
			mu.Unlock()

			// Emit proto events outside the lock
			for _, ct := range localCreated {
				if err := stream.Send(&toquiv1.SendMessageResponse{
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
				}); err != nil {
					slog.Warn("stream.Send TripCreated failed", "error", err)
				}
			}
			for _, st := range localSelected {
				if err := stream.Send(&toquiv1.SendMessageResponse{
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
				}); err != nil {
					slog.Warn("stream.Send TripSelected failed", "error", err)
				}
			}
			if localSwitch != nil {
				if err := stream.Send(&toquiv1.SendMessageResponse{
					Event: &toquiv1.SendMessageResponse_PersonaSwitch{
						PersonaSwitch: &toquiv1.PersonaSwitch{
							PreviousPersona: personaToProto(localSwitch.Previous),
							NewPersona:      personaToProto(localSwitch.Expert),
							HandoffMessage:  localSwitch.HandoffMessage,
						},
					},
				}); err != nil {
					slog.Warn("stream.Send PersonaSwitch failed", "error", err)
				}
			}
			if len(localItinerary) > 0 {
				if tripID, err := uuid.Parse(req.Msg.TripId); err == nil {
					if allItems, err := h.tripSvc.GetItinerary(ctx, tripID); err == nil {
						slog.Info("itinerary update: sending items to client",
							"trip_id", tripID, "callback_count", len(localItinerary), "db_count", len(allItems))
						// Best-effort: fetch coordinates; empty map is fine (map pins
						// may appear after next itinerary fetch once geocoding completes).
						coordsMap := make(map[uuid.UUID]trip.ItineraryItemCoords)
						if coords, err := h.tripSvc.GetItineraryCoords(ctx, tripID); err == nil {
							for _, c := range coords {
								coordsMap[c.ID] = c
							}
						}
						if err := stream.Send(&toquiv1.SendMessageResponse{
							Event: &toquiv1.SendMessageResponse_ItineraryUpdate{
								ItineraryUpdate: &toquiv1.ItineraryUpdate{
									TripId:    req.Msg.TripId,
									Itinerary: itineraryToProto(req.Msg.TripId, allItems, coordsMap),
								},
							},
						}); err != nil {
							slog.Warn("stream.Send ItineraryUpdate failed", "error", err)
						}
					}
				}
			}
			for _, rec := range localRecs {
				slog.Info("affiliate recommendation generated",
					"partner", rec.Partner,
					"category", rec.Category,
					"title", rec.Title,
					"user_id", userID,
				)
			}
			for _, ut := range localUpdated {
				tripProto := &toquiv1.Trip{
					Id:          ut.ID,
					UserId:      userID.String(),
					Title:       ut.Title,
					Description: ut.Description,
				}
				if err := stream.Send(&toquiv1.SendMessageResponse{
					Event: &toquiv1.SendMessageResponse_TripUpdated{
						TripUpdated: &toquiv1.TripUpdated{
							Trip: tripProto,
						},
					},
				}); err != nil {
					slog.Warn("stream.Send TripUpdated failed", "error", err)
				}
			}

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

	// Retroactively link the selection session to the newly created trip (#153).
	// This makes the initial conversation visible in ListSessions(tripID) and
	// GetHistory. Runs after the stream so we don't block the client response.
	if isSelection && createdTripID != "" && sessionID != "" {
		if err := h.chatSvc.MoveSessionToTrip(ctx, userID, sessionID, createdTripID); err != nil {
			slog.Error("failed to relink selection session to created trip",
				"session_id", sessionID, "trip_id", createdTripID, "error", err)
		}
	}

	// Retag themes if the trip has none yet.
	// This intentionally outlives the request context: theme tagging is a
	// best-effort background job that should complete even after the SSE
	// stream is closed. We use a separate 30-second timeout to bound it.
	if !isSelection {
		if tripID, err := uuid.Parse(req.Msg.TripId); err == nil && h.themeSvc != nil && fullContent != "" {
			if len(tripThemes) == 0 {
				if t, err := h.tripSvc.GetByID(ctx, userID, tripID); err == nil {
					recentMessages := []string{req.Msg.Content, fullContent}
					go func() {
						bgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
						defer cancel()
						if err := h.themeSvc.TagTrip(bgCtx, userID, tripID, t.Title, t.Description.String, recentMessages); err != nil {
							slog.Error("chat retag trip failed", "trip_id", tripID, "error", err)
						}
					}()
				}
			}
		}
	}

	// Refund daily usage if the AI produced no content (e.g., 429 rate limit
	// killed the stream). Selection-mode messages were never counted, so
	// nothing to refund (#191).
	if !hadContent && h.usageSvc != nil && !isAdmin && !isSelection {
		if err := h.queries.DecrementDailyUsage(ctx, userID); err != nil {
			slog.Error("failed to decrement daily usage after empty AI response", "user_id", userID, "error", err)
		}
	}

	return nil
}

type tripCreatedInfo struct {
	ID, Title, Description string
}

type tripUpdatedInfo struct {
	ID, Title, Description string
	Countries              []string
}

type personaSwitchInfo struct {
	Previous       *persona.Persona
	Expert         *persona.Persona
	HandoffMessage string
}

// sanitizeForPrompt strips control characters and truncates user-controlled text
// before injection into AI system prompts. This prevents prompt injection via
// crafted trip titles/descriptions.
func sanitizeForPrompt(s string, maxLen int) string {
	// Replace newlines, tabs, and other control characters with spaces
	var b strings.Builder
	for _, r := range s {
		if r == '\n' || r == '\r' || r == '\t' || r < 0x20 {
			b.WriteRune(' ')
		} else {
			b.WriteRune(r)
		}
	}
	// Collapse multiple spaces in a single pass.
	raw := strings.TrimSpace(b.String())
	var out strings.Builder
	out.Grow(len(raw))
	prevSpace := false
	for _, r := range raw {
		if r == ' ' {
			if prevSpace {
				continue
			}
			prevSpace = true
		} else {
			prevSpace = false
		}
		out.WriteRune(r)
	}
	result := out.String()
	// Truncate by rune count, not byte count, to avoid splitting multi-byte
	// UTF-8 characters (e.g., CJK, emoji) at the boundary.
	if maxLen > 0 {
		runes := []rune(result)
		if len(runes) > maxLen {
			result = string(runes[:maxLen])
		}
	}
	return result
}

// buildTripContext returns system prompt context for planning/companion mode:
// the trip's metadata so the AI knows what it's helping with.
func buildTripContext(title, description, destinationCountry string, destinationCountries []string, startDate, endDate, status string, themes []string, itineraryItems []dbgen.ItineraryItem, bookings []dbgen.Booking, collaboratorCount int64, userTier tier.UserTier) string {
	if title == "" && description == "" && destinationCountry == "" && len(destinationCountries) == 0 {
		return ""
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Today's date is %s.\n\n", time.Now().Format(dateFormatLong))
	sb.WriteString("CURRENT TRIP CONTEXT:\n")
	if title != "" {
		statusLabel := status
		if statusLabel == "" {
			statusLabel = "planning"
		}
		fmt.Fprintf(&sb, "- Trip: %s (%s)\n", sanitizeForPrompt(title, 200), statusLabel)
	}
	if description != "" {
		fmt.Fprintf(&sb, "- Description: %s\n", sanitizeForPrompt(description, 500))
	}
	// Multi-country trips: list all destinations. Single-country trips fall
	// back to the legacy single field for the same display (#133).
	if len(destinationCountries) > 1 {
		sanitized := make([]string, len(destinationCountries))
		for i, c := range destinationCountries {
			sanitized[i] = sanitizeForPrompt(c, 10)
		}
		fmt.Fprintf(&sb, "- Destinations: %s\n", strings.Join(sanitized, ", "))
	} else if destinationCountry != "" {
		fmt.Fprintf(&sb, "- Destination: %s\n", sanitizeForPrompt(destinationCountry, 100))
	} else if len(destinationCountries) == 1 {
		fmt.Fprintf(&sb, "- Destination: %s\n", sanitizeForPrompt(destinationCountries[0], 100))
	}
	if startDate != "" && endDate != "" {
		fmt.Fprintf(&sb, "- Travel dates: %s to %s\n", startDate, endDate)
	} else if startDate != "" {
		fmt.Fprintf(&sb, "- Start date: %s\n", startDate)
	} else if endDate != "" {
		fmt.Fprintf(&sb, "- End date: %s\n", endDate)
	}
	if len(themes) > 0 {
		sanitized := make([]string, len(themes))
		for i, t := range themes {
			sanitized[i] = sanitizeForPrompt(t, 50)
		}
		fmt.Fprintf(&sb, "- Trip themes: %s\n", strings.Join(sanitized, ", "))
	}
	if collaboratorCount > 0 {
		fmt.Fprintf(&sb, "- Collaborators: %d people on this trip\n", collaboratorCount+1) // +1 for the owner
	}

	// Itinerary summary (capped at 20 items)
	if len(itineraryItems) > 0 {
		sb.WriteString("\nExisting itinerary")
		// Group items by day number
		dayItems := make(map[int32][]dbgen.ItineraryItem)
		var dayNums []int32
		for _, item := range itineraryItems {
			day := int32(0)
			if item.DayNumber.Valid {
				day = item.DayNumber.Int32
			}
			if _, exists := dayItems[day]; !exists {
				dayNums = append(dayNums, day)
			}
			dayItems[day] = append(dayItems[day], item)
		}
		fmt.Fprintf(&sb, " (%d items):\n", len(itineraryItems))
		// Show up to 60 items (enough for a 12-day trip at 5 items/day).
		// Uses a compact one-line-per-day format to minimize token usage
		// while giving the AI full visibility of the itinerary.
		const maxContextItems = 60
		itemCount := 0
		for _, day := range dayNums {
			items := dayItems[day]
			if day > 0 {
				fmt.Fprintf(&sb, "  Day %d:", day)
			} else {
				sb.WriteString("  Unscheduled:")
			}
			titles := make([]string, 0, len(items))
			for _, item := range items {
				if itemCount >= maxContextItems {
					break
				}
				if item.Title.Valid && item.Title.String != "" {
					titles = append(titles, sanitizeForPrompt(item.Title.String, 80))
				}
				itemCount++
			}
			fmt.Fprintf(&sb, " %s\n", strings.Join(titles, ", "))
			if itemCount >= maxContextItems {
				sb.WriteString("  ... (more items not shown)\n")
				break
			}
		}
	}

	// Bookings summary (rich detail so AI can answer questions about bookings)
	if len(bookings) > 0 {
		sb.WriteString("\nExisting bookings:\n")
		for i, b := range bookings {
			if i >= 20 {
				sb.WriteString("  ... (more bookings not shown)\n")
				break
			}
			bookingType := sanitizeForPrompt(b.Type, 50)
			bookingTitle := sanitizeForPrompt(b.Title, 150)
			fmt.Fprintf(&sb, "  - %s: %s", bookingType, bookingTitle)
			if b.Provider.Valid && b.Provider.String != "" {
				fmt.Fprintf(&sb, " [%s]", sanitizeForPrompt(b.Provider.String, 100))
			}
			if b.StartTime.Valid {
				fmt.Fprintf(&sb, " (%s", b.StartTime.Time.Format("Jan 2"))
				if b.EndTime.Valid {
					fmt.Fprintf(&sb, " to %s", b.EndTime.Time.Format("Jan 2"))
				}
				sb.WriteString(")")
			}
			if b.ConfirmationCode.Valid && b.ConfirmationCode.String != "" {
				fmt.Fprintf(&sb, " Confirmation: %s", sanitizeForPrompt(b.ConfirmationCode.String, 50))
			}
			if b.DepartureLocation.Valid && b.DepartureLocation.String != "" {
				fmt.Fprintf(&sb, " From: %s", sanitizeForPrompt(b.DepartureLocation.String, 100))
			}
			if b.ArrivalLocation.Valid && b.ArrivalLocation.String != "" {
				fmt.Fprintf(&sb, " To: %s", sanitizeForPrompt(b.ArrivalLocation.String, 100))
			}
			if b.Address.Valid && b.Address.String != "" {
				fmt.Fprintf(&sb, " Address: %s", sanitizeForPrompt(b.Address.String, 200))
			}
			// Include key fields from details_json if present
			if len(b.DetailsJson) > 0 {
				var details map[string]interface{}
				if err := json.Unmarshal(b.DetailsJson, &details); err == nil {
					var extras []string
					for _, key := range []string{"airline", "flight_number", "hotel_name", "room_type", "check_in_time", "check_out_time", "meeting_point", "notes"} {
						if v, ok := details[key]; ok && v != nil && fmt.Sprintf("%v", v) != "" {
							extras = append(extras, fmt.Sprintf("%s: %v", key, v))
						}
					}
					if len(extras) > 0 {
						fmt.Fprintf(&sb, " | %s", strings.Join(extras, ", "))
					}
				}
			}
			// Include truncated raw source for bookings so AI can answer
			// detailed questions (terminal, check-in time, etc.)
			if b.RawSource.Valid && b.RawSource.String != "" {
				raw := sanitizeForPrompt(b.RawSource.String, 500)
				fmt.Fprintf(&sb, "\n    Raw details: %s", raw)
			}
			sb.WriteString("\n")
		}
	}

	// Smart planning advice based on trip context
	if startDate != "" && endDate != "" {
		start, errS := time.Parse(dateFormatLong, startDate)
		end, errE := time.Parse(dateFormatLong, endDate)
		if errS == nil && errE == nil {
			tripDays := int(end.Sub(start).Hours()/24) + 1
			if tripDays > 7 {
				fmt.Fprintf(&sb, "\nThis is a longer trip (%d days). Build in rest/flex days every 3-4 days of activities — travelers burn out on packed schedules. Don't over-schedule every day.\n", tripDays)
			}
		}
	}

	if collaboratorCount > 0 {
		sb.WriteString("\nThis is a group trip. Suggest activities that work for groups and include some free time for individual exploration. Note group-friendly logistics (e.g., shared transport, group discounts, restaurants that take large parties).\n")
	}

	// Booking-aware planning: if there are accommodation bookings, suggest nearby activities
	hasAccommodation := false
	for _, b := range bookings {
		if strings.EqualFold(b.Type, "accommodation") || strings.EqualFold(b.Type, "hotel") || strings.EqualFold(b.Type, "lodging") {
			hasAccommodation = true
			break
		}
	}
	if hasAccommodation {
		sb.WriteString("\nThe traveler has accommodation bookings listed above. When planning daily activities, consider proximity to their hotel/accommodation and suggest activities in nearby neighborhoods first.\n")
	}

	sb.WriteString("\nYou already know the trip destination, dates, existing itinerary, bookings, and group size. Do NOT ask for this information again. If USER PREFERENCES are provided below, use them without asking again — only ask about preferences that are NOT already listed. For any unlisted preferences (interests, mobility needs, travel style, etc.), DO ask clarifying questions to give better recommendations.")

	// Differentiate itinerary tool behavior between planning and companion mode.
	// In planning mode, the AI should proactively create items. In companion mode
	// (status=active), the user is traveling and asking questions — only create
	// items when they explicitly ask (Run 19 N-13 regression).
	if strings.EqualFold(status, "active") {
		sb.WriteString("\n\nCOMPANION MODE — ITINERARY TOOL USAGE: You are in companion mode (the trip is active). The traveler is on the ground and asking you questions. Do NOT proactively modify the itinerary. Only call create_itinerary_items or delete_itinerary_items when the user EXPLICITLY asks to add or remove something from their plan (e.g., \"add that to my itinerary\", \"save this for tomorrow\", \"remove the museum visit\"). For all other queries — recommendations, suggestions, directions, opinions, tips, \"what should I do\", \"where should I eat\" — just answer in text. The user's itinerary already exists; don't clutter it with every suggestion you make.")
	} else {
		sb.WriteString("\n\nITINERARY TOOL USAGE: ALWAYS use the create_itinerary_items tool when you suggest specific activities, meals, sightseeing, or experiences for the trip. If you mention a concrete place or activity the traveler should do, save it to the itinerary — don't just describe it in prose. The user expects items to appear in their itinerary view. Only skip the tool for abstract questions about transport logistics, safety, budgets, or general destination info where no specific activity is being recommended.")
		sb.WriteString("\nCRITICAL: NEVER describe an itinerary plan in text without also calling create_itinerary_items to save it. If you mention specific activities, restaurants, or attractions for specific days, you MUST create itinerary items for them. The user's itinerary is only useful if it's saved — text descriptions alone are not visible in their trip plan.")
	}
	sb.WriteString("\n\n")
	sb.WriteString(bookingInstructionsForTier(userTier))
	return sb.String()
}

// buildSelectionContext returns system prompt context for selection mode:
// the user's existing trips so Toqui can help them find or create one.
func (h *ChatHandler) buildSelectionContext(ctx context.Context, userID uuid.UUID, userTier tier.UserTier) string {
	today := time.Now().Format(dateFormatLong)

	trips, _, err := h.tripSvc.ListByUser(ctx, userID, "", 20, 0)
	if err != nil || len(trips) == 0 {
		return fmt.Sprintf(`Today's date is %s.

You are in SELECTION mode — no trip is selected yet.

Help the user decide on a trip. You can:
- Help them brainstorm destinations and trip ideas
- Create a trip for them when they're ready (use the create_trip tool)

The user has no existing trips yet. Help them get started!

When the user expresses interest in a specific destination or trip idea, proactively create the trip for them using the create_trip tool. Don't wait for them to explicitly say "create a trip" — if they say something like "I want to go to Japan" or "planning a weekend in Paris", go ahead and create it.

%s`, today, bookingInstructionsForTier(userTier))
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Today's date is %s.\n\n", today)
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
		fmt.Fprintf(&sb, "- %s (id: %s, status: %s", sanitizeForPrompt(t.Title, 200), t.ID, status)
		if t.DestinationCountry.Valid && t.DestinationCountry.String != "" {
			fmt.Fprintf(&sb, ", destination: %s", sanitizeForPrompt(t.DestinationCountry.String, 100))
		}
		sb.WriteString(")")
		if t.Description.Valid && t.Description.String != "" {
			fmt.Fprintf(&sb, " — %s", sanitizeForPrompt(t.Description.String, 500))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	sb.WriteString(bookingInstructionsForTier(userTier))

	return sb.String()
}

// bookingInstructionsForTier returns the system prompt snippet that tells the
// AI how to handle booking recommendations based on the user's subscription tier.
func bookingInstructionsForTier(userTier tier.UserTier) string {
	triggerPhrases := `
When the user asks about ANY of the following, you MUST call the recommend_booking tool:
- "book a flight", "find flights", "search for flights"
- "book a hotel", "find accommodation", "where to stay"
- "book a tour", "book an activity", "things to book", "tickets"
- "day trip", "excursion", "can I book"
- "car rental", "rent a car"
- Any question about purchasing, reserving, or booking travel services
Do NOT respond with text-only suggestions when the user is asking to BOOK something. Use the tool.`

	if userTier.IsPro() {
		return "BOOKING RECOMMENDATIONS: Recommend the best options regardless of affiliate partnerships. Use the recommend_booking tool when the user asks about flights, hotels, activities, car rentals, or travel insurance." + triggerPhrases
	}
	return "BOOKING RECOMMENDATIONS: When the user asks about flights, hotels, activities, car rentals, or travel insurance, use the recommend_booking tool. For international trips, proactively suggest travel insurance if the user hasn't mentioned it. For destinations that benefit from driving (rural areas, road trips), suggest car rentals. IMPORTANT: You MUST include the disclosure text from the tool result in your response to the user — this is a legal requirement. Present the recommendation with the search link and the full disclosure statement." + triggerPhrases
}

func (h *ChatHandler) GetChatHistory(ctx context.Context, req *connect.Request[toquiv1.GetChatHistoryRequest]) (*connect.Response[toquiv1.GetChatHistoryResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	limit := int(clampPageSize(req.Msg.GetPagination().GetPageSize(), 50, 100))

	messages, err := h.chatSvc.GetHistory(ctx, userID, req.Msg.TripId, req.Msg.SessionId, limit)
	if err != nil {
		return nil, internalError(ctx, "get chat history", err)
	}

	// Filter out tool-loop intermediates so clients never see empty-content
	// bubbles. These messages are retained in Firestore because they are
	// required to reconstruct tool_call/tool_result context when the AI
	// continues a conversation, but they must not leak into the user-facing
	// history view (Run 4 #N-02 P0).
	protoMessages := make([]*toquiv1.ChatMessage, 0, len(messages))
	for _, m := range messages {
		if isToolLoopIntermediate(m) {
			continue
		}
		protoMessages = append(protoMessages, &toquiv1.ChatMessage{
			Id:        m.ID,
			SessionId: m.SessionID,
			Role:      m.Role,
			Content:   m.Content,
			Metadata:  m.Metadata,
			CreatedAt: timestamppb.New(m.CreatedAt),
		})
	}

	return connect.NewResponse(&toquiv1.GetChatHistoryResponse{
		Messages: protoMessages,
	}), nil
}

// isToolLoopIntermediate reports whether a stored chat message is an
// AI-internal tool-loop turn that should be hidden from the user-facing
// history view. These are either:
//   - assistant messages with empty content that carry only tool_calls, OR
//   - user-role messages that carry only tool_results and no content, OR
//   - any message with empty content and no tool data (blank bubble).
//
// They are necessary for AI history reconstruction but render as blank
// bubbles for end users.
func isToolLoopIntermediate(m *chatstore.ChatMessage) bool {
	if m == nil {
		return false
	}
	if strings.TrimSpace(m.Content) != "" {
		return false
	}
	// Never filter user-role messages — even empty ones may be meaningful
	// (e.g., attachment-only messages).
	if m.Role == "user" && len(m.ToolResults) == 0 {
		return false
	}
	// Empty assistant content: filter if it has tool data (intermediate turn)
	// or if it's completely blank (blank bubble from truncation/provider error).
	// Empty user messages carrying only tool_results are also intermediates.
	return true
}

func (h *ChatHandler) ListChatSessions(ctx context.Context, req *connect.Request[toquiv1.ListChatSessionsRequest]) (*connect.Response[toquiv1.ListChatSessionsResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	limit := int(clampPageSize(req.Msg.GetPagination().GetPageSize(), 20, 100))

	sessions, err := h.chatSvc.ListSessions(ctx, userID, req.Msg.TripId, limit)
	if err != nil {
		return nil, internalError(ctx, "list sessions", err)
	}

	protoSessions := make([]*toquiv1.ChatSession, len(sessions))
	for i, s := range sessions {
		protoSessions[i] = &toquiv1.ChatSession{
			Id:            s.ID,
			TripId:        s.TripID,
			Mode:          chatModeFromString(s.Mode),
			CreatedAt:     timestamppb.New(s.CreatedAt),
			LastMessageAt: timestamppb.New(s.LastMessageAt),
			MessageCount:  int32(s.MessageCount),
		}
	}

	return connect.NewResponse(&toquiv1.ListChatSessionsResponse{
		Sessions: protoSessions,
	}), nil
}

// Attachment validation constants.
const (
	maxAttachments     = 5
	maxAttachmentBytes = 10 * 1024 * 1024 // 10 MB
)

// allowedAttachmentTypes is the set of media types accepted for chat attachments.
var allowedAttachmentTypes = map[string]bool{
	"image/jpeg":      true,
	"image/png":       true,
	"image/gif":       true,
	"image/webp":      true,
	"application/pdf": true,
	"text/plain":      true,
	"text/csv":        true,
}

// validateAttachments checks that attachments conform to size, count, and type limits.
func validateAttachments(attachments []*toquiv1.Attachment) error {
	if len(attachments) == 0 {
		return nil
	}
	if len(attachments) > maxAttachments {
		return fmt.Errorf("too many attachments: %d (max %d)", len(attachments), maxAttachments)
	}
	for i, a := range attachments {
		if a.Filename == "" {
			return fmt.Errorf("attachment %d: filename is required", i)
		}
		if a.MediaType == "" {
			return fmt.Errorf("attachment %d: media_type is required", i)
		}
		if !allowedAttachmentTypes[a.MediaType] {
			return fmt.Errorf("attachment %d: unsupported media type %q", i, a.MediaType)
		}
		if len(a.Data) == 0 {
			return fmt.Errorf("attachment %d: data is empty", i)
		}
		if int64(len(a.Data)) > maxAttachmentBytes {
			return fmt.Errorf("attachment %d: size %d bytes exceeds maximum %d bytes", i, len(a.Data), maxAttachmentBytes)
		}
	}
	return nil
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
