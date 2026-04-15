package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gallowaysoftware/toqui-backend/internal/auth"
	"github.com/gallowaysoftware/toqui-backend/internal/booking"
	"github.com/gallowaysoftware/toqui-backend/internal/chatstore"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui-backend/internal/theme"
	"github.com/gallowaysoftware/toqui-backend/internal/trip"
)

// BundleHandler serves GET /api/trips/{tripId}/bundle — returns a complete
// trip snapshot for offline use. Includes trip metadata, itinerary, bookings,
// recent chat messages, and destination guide data.
type BundleHandler struct {
	tripSvc    *trip.Service
	bookingSvc *booking.Service
	authSvc    *auth.Service
	chatStore  *chatstore.Store
	themeSvc   *theme.Service
	queries    *dbgen.Queries
	guides     *GuidesHandler
}

// NewBundleHandler creates a new BundleHandler.
func NewBundleHandler(
	tripSvc *trip.Service,
	bookingSvc *booking.Service,
	authSvc *auth.Service,
	chatStore *chatstore.Store,
	themeSvc *theme.Service,
	pool *pgxpool.Pool,
	guides *GuidesHandler,
) *BundleHandler {
	return &BundleHandler{
		tripSvc:    tripSvc,
		bookingSvc: bookingSvc,
		authSvc:    authSvc,
		chatStore:  chatStore,
		themeSvc:   themeSvc,
		queries:    dbgen.New(pool),
		guides:     guides,
	}
}

// --- JSON response types ---

type bundleResponse struct {
	BundleVersion string              `json:"bundle_version"`
	Modified      bool                `json:"modified"`
	Trip          *bundleTripInfo     `json:"trip,omitempty"`
	Itinerary     []bundleDay         `json:"itinerary,omitempty"`
	Bookings      []bundleBooking     `json:"bookings,omitempty"`
	ChatMessages  []bundleChatMessage `json:"chat_messages,omitempty"`
	Guides        []bundleGuide       `json:"destination_guides,omitempty"`
}

type bundleTripInfo struct {
	ID                   string   `json:"id"`
	Title                string   `json:"title"`
	Description          string   `json:"description,omitempty"`
	Status               string   `json:"status"`
	StartDate            string   `json:"start_date,omitempty"`
	EndDate              string   `json:"end_date,omitempty"`
	DestinationCountry   string   `json:"destination_country,omitempty"`
	DestinationCountries []string `json:"destination_countries,omitempty"`
	Themes               []string `json:"themes,omitempty"`
	BudgetCents          *int64   `json:"budget_cents,omitempty"`
	Currency             string   `json:"currency,omitempty"`
}

type bundleDay struct {
	DayNumber int32           `json:"day_number"`
	Date      string          `json:"date,omitempty"`
	Summary   string          `json:"summary,omitempty"`
	Items     []bundleDayItem `json:"items"`
}

type bundleDayItem struct {
	ID                 string  `json:"id"`
	OrderInDay         int32   `json:"order_in_day"`
	Type               string  `json:"type,omitempty"`
	Title              string  `json:"title"`
	Description        string  `json:"description,omitempty"`
	StartTime          string  `json:"start_time,omitempty"`
	EndTime            string  `json:"end_time,omitempty"`
	Latitude           float64 `json:"latitude,omitempty"`
	Longitude          float64 `json:"longitude,omitempty"`
	EstimatedCostCents *int64  `json:"estimated_cost_cents,omitempty"`
	CostCurrency       string  `json:"cost_currency,omitempty"`
}

type bundleBooking struct {
	ID               string `json:"id"`
	Type             string `json:"type"`
	Title            string `json:"title"`
	Provider         string `json:"provider,omitempty"`
	ConfirmationCode string `json:"confirmation_code,omitempty"`
	StartTime        string `json:"start_time,omitempty"`
	EndTime          string `json:"end_time,omitempty"`
	Address          string `json:"address,omitempty"`
	DetailsJSON      string `json:"details_json,omitempty"`
}

type bundleChatMessage struct {
	ID        string `json:"id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

type bundleGuide struct {
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	PersonaName string `json:"persona_name"`
	Destination string `json:"destination"`
	Country     string `json:"country"`
	Theme       string `json:"theme"`
	Content     string `json:"content"`
}

// HandleBundle handles GET /api/trips/{tripId}/bundle.
// Supports conditional fetch via If-Modified-Since header — returns 304 when
// no data has changed, saving bandwidth on metered mobile connections.
func (h *BundleHandler) HandleBundle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := authenticateRESTRequest(r, h.authSvc)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	tripID, ok := parseTripIDFromBundlePath(r.URL.Path)
	if !ok {
		http.Error(w, "invalid trip ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Load trip (verifies ownership or collaborator access).
	t, err := h.tripSvc.GetByIDOrCollaborator(ctx, userID, tripID)
	if err != nil {
		slog.Warn("bundle: trip not found", "trip_id", tripID, "user_id", userID, "error", err)
		http.Error(w, "trip not found", http.StatusNotFound)
		return
	}

	// Determine the latest modification timestamp across the trip.
	bundleVersion := t.UpdatedAt

	// Conditional fetch: if the client sent If-Modified-Since and the trip
	// hasn't changed since then, return 304.
	if ims := r.Header.Get("If-Modified-Since"); ims != "" {
		if parsed, err := time.Parse(http.TimeFormat, ims); err == nil {
			if !bundleVersion.After(parsed) {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}
	}

	// --- Assemble the full bundle ---

	// Trip metadata with themes.
	var themes []string
	if h.themeSvc != nil {
		themes, _ = h.themeSvc.GetTripThemes(ctx, tripID)
	}
	tripInfo := buildBundleTripInfo(t, themes)

	// Itinerary.
	items, err := h.tripSvc.GetItinerary(ctx, tripID)
	if err != nil {
		slog.Error("bundle: failed to load itinerary", "trip_id", tripID, "error", err)
		http.Error(w, "failed to load itinerary", http.StatusInternalServerError)
		return
	}

	coordsMap := make(map[uuid.UUID]trip.ItineraryItemCoords)
	if coords, err := h.tripSvc.GetItineraryCoords(ctx, tripID); err == nil {
		for _, c := range coords {
			coordsMap[c.ID] = c
		}
	}
	itinerary := buildBundleItinerary(items, coordsMap)

	// Bookings.
	var bookings []bundleBooking
	if h.bookingSvc != nil {
		dbBookings, err := h.bookingSvc.ListByTrip(ctx, userID, tripID)
		if err != nil {
			slog.Warn("bundle: failed to load bookings", "trip_id", tripID, "error", err)
		} else {
			bookings = buildBundleBookings(dbBookings)
		}
	}

	// Chat messages — fetch the latest session's recent messages.
	var chatMessages []bundleChatMessage
	if h.chatStore != nil {
		chatMessages = h.fetchRecentChatMessages(ctx, userID, tripID)
	}

	// Destination guides — match by country code.
	var guides []bundleGuide
	if h.guides != nil {
		countries := t.DestinationCountries
		if len(countries) == 0 && t.DestinationCountry.Valid && t.DestinationCountry.String != "" {
			countries = []string{t.DestinationCountry.String}
		}
		guides = h.matchGuides(countries)
	}

	resp := bundleResponse{
		BundleVersion: bundleVersion.UTC().Format(time.RFC3339),
		Modified:      true,
		Trip:          tripInfo,
		Itinerary:     itinerary,
		Bookings:      bookings,
		ChatMessages:  chatMessages,
		Guides:        guides,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Last-Modified", bundleVersion.UTC().Format(http.TimeFormat))
	w.Header().Set("Cache-Control", "private, no-cache")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("bundle: failed to encode response", "error", err)
	}
}

// parseTripIDFromBundlePath extracts the trip UUID from /api/trips/{id}/bundle.
func parseTripIDFromBundlePath(path string) (uuid.UUID, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	// parts: [api, trips, <uuid>, bundle]
	if len(parts) != 4 || parts[0] != "api" || parts[1] != "trips" || parts[3] != "bundle" {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(parts[2])
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

// fetchRecentChatMessages loads recent messages from the latest chat session.
// Returns up to 50 messages. Best-effort — returns empty on any error.
func (h *BundleHandler) fetchRecentChatMessages(ctx context.Context, userID uuid.UUID, tripID uuid.UUID) []bundleChatMessage {
	sessions, err := h.chatStore.ListSessions(ctx, userID.String(), tripID.String(), 1)
	if err != nil || len(sessions) == 0 {
		return nil
	}

	latestSession := sessions[0]
	messages, err := h.chatStore.GetMessages(ctx, userID.String(), tripID.String(), latestSession.ID, 50)
	if err != nil {
		slog.Warn("bundle: failed to load chat messages", "trip_id", tripID, "session_id", latestSession.ID, "error", err)
		return nil
	}

	result := make([]bundleChatMessage, 0, len(messages))
	for _, msg := range messages {
		// Skip system messages and tool-related messages — only include
		// user and assistant messages for offline reading.
		if msg.Role != "user" && msg.Role != "assistant" {
			continue
		}
		if msg.Content == "" {
			continue
		}
		result = append(result, bundleChatMessage{
			ID:        msg.ID,
			Role:      msg.Role,
			Content:   msg.Content,
			CreatedAt: msg.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	return result
}

// matchGuides returns guides whose country code matches any of the trip's
// destination countries.
func (h *BundleHandler) matchGuides(countries []string) []bundleGuide {
	if len(countries) == 0 {
		return nil
	}

	countrySet := make(map[string]bool, len(countries))
	for _, c := range countries {
		countrySet[strings.ToUpper(c)] = true
	}

	var matched []bundleGuide
	for _, g := range h.guides.guides {
		if countrySet[strings.ToUpper(g.Country)] {
			matched = append(matched, bundleGuide{
				Slug:        g.Slug,
				Title:       g.Title,
				PersonaName: g.PersonaName,
				Destination: g.Destination,
				Country:     g.Country,
				Theme:       g.Theme,
				Content:     g.Content,
			})
		}
	}
	return matched
}

// buildBundleTripInfo converts a dbgen.Trip to the bundle's trip info format.
func buildBundleTripInfo(t *dbgen.Trip, themes []string) *bundleTripInfo {
	info := &bundleTripInfo{
		ID:     t.ID.String(),
		Title:  t.Title,
		Status: t.Status,
		Themes: themes,
	}
	if t.Description.Valid {
		info.Description = t.Description.String
	}
	if t.DestinationCountry.Valid {
		info.DestinationCountry = t.DestinationCountry.String
	}
	if len(t.DestinationCountries) > 0 {
		info.DestinationCountries = t.DestinationCountries
	}
	if t.StartDate.Valid {
		info.StartDate = t.StartDate.Time.Format("2006-01-02")
	}
	if t.EndDate.Valid {
		info.EndDate = t.EndDate.Time.Format("2006-01-02")
	}
	if t.BudgetCents.Valid {
		info.BudgetCents = &t.BudgetCents.Int64
	}
	if t.Currency.Valid && t.Currency.String != "" {
		info.Currency = t.Currency.String
	}
	return info
}

// buildBundleItinerary groups itinerary items into days for the bundle.
func buildBundleItinerary(items []dbgen.ItineraryItem, coordsMap map[uuid.UUID]trip.ItineraryItemCoords) []bundleDay {
	dayMap := make(map[int32]*bundleDay)
	var dayOrder []int32

	for _, item := range items {
		dayNum := int32(0)
		if item.DayNumber.Valid {
			dayNum = item.DayNumber.Int32
		}

		day, ok := dayMap[dayNum]
		if !ok {
			summary := ""
			date := ""
			if len(item.Metadata) > 0 {
				var md map[string]json.RawMessage
				if err := json.Unmarshal(item.Metadata, &md); err == nil {
					if raw, ok := md["day_summary"]; ok {
						var v string
						if json.Unmarshal(raw, &v) == nil {
							summary = v
						}
					}
					if raw, ok := md["day_date"]; ok {
						var v string
						if json.Unmarshal(raw, &v) == nil {
							date = v
						}
					}
				}
			}
			day = &bundleDay{
				DayNumber: dayNum,
				Summary:   summary,
				Date:      date,
				Items:     []bundleDayItem{},
			}
			dayMap[dayNum] = day
			dayOrder = append(dayOrder, dayNum)
		}

		bi := bundleDayItem{
			ID: item.ID.String(),
		}
		if item.Title.Valid {
			bi.Title = item.Title.String
		}
		if item.OrderInDay.Valid {
			bi.OrderInDay = item.OrderInDay.Int32
		}
		if item.Type.Valid {
			bi.Type = item.Type.String
		}
		if item.Description.Valid {
			bi.Description = item.Description.String
		}
		if item.StartTime.Valid {
			bi.StartTime = item.StartTime.Time.UTC().Format(time.RFC3339)
		}
		if item.EndTime.Valid {
			bi.EndTime = item.EndTime.Time.UTC().Format(time.RFC3339)
		}
		if c, ok := coordsMap[item.ID]; ok && (c.Latitude != 0 || c.Longitude != 0) {
			bi.Latitude = c.Latitude
			bi.Longitude = c.Longitude
		}
		if item.EstimatedCostCents.Valid {
			bi.EstimatedCostCents = &item.EstimatedCostCents.Int64
		}
		if item.CostCurrency.Valid {
			bi.CostCurrency = item.CostCurrency.String
		}

		day.Items = append(day.Items, bi)
	}

	result := make([]bundleDay, 0, len(dayOrder))
	for _, dayNum := range dayOrder {
		result = append(result, *dayMap[dayNum])
	}
	return result
}

// buildBundleBookings converts DB bookings to the bundle's booking format.
// Unlike shared trip views, this is an authenticated endpoint for the trip
// owner — confirmation codes ARE included since the traveler needs them offline.
func buildBundleBookings(bookings []dbgen.Booking) []bundleBooking {
	result := make([]bundleBooking, 0, len(bookings))
	for _, b := range bookings {
		bb := bundleBooking{
			ID:    b.ID.String(),
			Type:  b.Type,
			Title: b.Title,
		}
		if b.Provider.Valid {
			bb.Provider = b.Provider.String
		}
		if b.ConfirmationCode.Valid {
			bb.ConfirmationCode = b.ConfirmationCode.String
		}
		if b.StartTime.Valid {
			bb.StartTime = b.StartTime.Time.UTC().Format(time.RFC3339)
		}
		if b.EndTime.Valid {
			bb.EndTime = b.EndTime.Time.UTC().Format(time.RFC3339)
		}
		if b.Address.Valid {
			bb.Address = b.Address.String
		}
		if len(b.DetailsJson) > 0 {
			bb.DetailsJSON = string(b.DetailsJson)
		}
		result = append(result, bb)
	}
	return result
}
