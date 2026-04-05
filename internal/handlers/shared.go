package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gallowaysoftware/toqui-backend/internal/analytics"
	"github.com/gallowaysoftware/toqui-backend/internal/audit"
	"github.com/gallowaysoftware/toqui-backend/internal/auth"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui-backend/internal/trip"
)

// SharedHandler serves the public shared trip endpoint and authenticated
// enable/disable sharing endpoints.
type SharedHandler struct {
	tripSvc         *trip.Service
	authSvc         *auth.Service
	frontendURL     string
	analyticsClient *analytics.Client
}

// NewSharedHandler creates a new SharedHandler.
func NewSharedHandler(tripSvc *trip.Service, authSvc *auth.Service, frontendURL string) *SharedHandler {
	return &SharedHandler{
		tripSvc:     tripSvc,
		authSvc:     authSvc,
		frontendURL: frontendURL,
	}
}

// WithAnalytics configures the shared handler to send events to PostHog.
func (h *SharedHandler) WithAnalytics(client *analytics.Client) *SharedHandler {
	h.analyticsClient = client
	return h
}

// --- JSON response types ---

type sharedTripResponse struct {
	Trip      sharedTripInfo       `json:"trip"`
	Itinerary []sharedItineraryDay `json:"itinerary"`
}

type sharedTripInfo struct {
	Title              string `json:"title"`
	Description        string `json:"description,omitempty"`
	DestinationCountry string `json:"destination_country,omitempty"`
	Status             string `json:"status"`
	StartDate          string `json:"start_date,omitempty"`
	EndDate            string `json:"end_date,omitempty"`
}

type sharedItineraryDay struct {
	DayNumber int32                 `json:"day_number"`
	Items     []sharedItineraryItem `json:"items"`
}

type sharedItineraryItem struct {
	Title       string `json:"title"`
	Type        string `json:"type,omitempty"`
	Description string `json:"description,omitempty"`
}

type enableSharingRequest struct {
	TripID string `json:"trip_id"`
}

type enableSharingResponse struct {
	ShareToken string `json:"share_token"`
	ShareURL   string `json:"share_url"`
}

type disableSharingRequest struct {
	TripID string `json:"trip_id"`
}

// HandlePublicView handles GET /shared/{token} — returns trip + itinerary (no auth).
func (h *SharedHandler) HandlePublicView(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract token from path: /shared/{token}
	token := strings.TrimPrefix(r.URL.Path, "/shared/")
	if token == "" || strings.Contains(token, "/") {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	ctx := r.Context()

	t, err := h.tripSvc.GetByShareToken(ctx, token)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		slog.Error("get trip by share token failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	items, err := h.tripSvc.GetItinerary(ctx, t.ID)
	if err != nil {
		slog.Error("get itinerary for shared trip failed", "trip_id", t.ID, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Track shared trip view (async, non-blocking, no PII)
	if h.analyticsClient != nil {
		h.analyticsClient.Track("anonymous", "shared_trip_viewed", nil)
	}

	resp := sharedTripResponse{
		Trip:      buildSharedTripInfo(t),
		Itinerary: buildSharedItinerary(items),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("failed to encode shared trip response", "error", err)
	}
}

// HandleEnable handles POST /api/trips/share — enables sharing (authenticated).
func (h *SharedHandler) HandleEnable(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := h.authenticateRequest(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB limit
	var req enableSharingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	tripID, err := uuid.Parse(req.TripID)
	if err != nil {
		http.Error(w, "invalid trip_id", http.StatusBadRequest)
		return
	}

	token, err := h.tripSvc.EnableSharing(r.Context(), userID, tripID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "trip not found", http.StatusNotFound)
			return
		}
		slog.Error("enable sharing failed", "user_id", userID, "trip_id", tripID, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	audit.Log(audit.EventTripShare, "user_id", userID.String(), "trip_id", tripID.String())

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(enableSharingResponse{
		ShareToken: token,
		ShareURL:   h.frontendURL + "/shared/" + token,
	}); err != nil {
		slog.Error("failed to encode enable sharing response", "error", err)
	}
}

// HandleDisable handles POST /api/trips/unshare — disables sharing (authenticated).
func (h *SharedHandler) HandleDisable(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := h.authenticateRequest(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB limit
	var req disableSharingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	tripID, err := uuid.Parse(req.TripID)
	if err != nil {
		http.Error(w, "invalid trip_id", http.StatusBadRequest)
		return
	}

	if err := h.tripSvc.DisableSharing(r.Context(), userID, tripID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "trip not found", http.StatusNotFound)
			return
		}
		slog.Error("disable sharing failed", "user_id", userID, "trip_id", tripID, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	audit.Log(audit.EventTripUnshare, "user_id", userID.String(), "trip_id", tripID.String())

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
		slog.Error("failed to encode disable sharing response", "error", err)
	}
}

// authenticateRequest delegates to the shared authenticateRESTRequest helper.
func (h *SharedHandler) authenticateRequest(r *http.Request) (uuid.UUID, bool) {
	return authenticateRESTRequest(r, h.authSvc)
}

// buildSharedTripInfo converts a dbgen.Trip to the public-safe response format.
func buildSharedTripInfo(t *dbgen.Trip) sharedTripInfo {
	info := sharedTripInfo{
		Title:  t.Title,
		Status: t.Status,
	}
	if t.Description.Valid {
		info.Description = t.Description.String
	}
	if t.DestinationCountry.Valid {
		info.DestinationCountry = t.DestinationCountry.String
	}
	if t.StartDate.Valid {
		info.StartDate = t.StartDate.Time.Format("2006-01-02")
	}
	if t.EndDate.Valid {
		info.EndDate = t.EndDate.Time.Format("2006-01-02")
	}
	return info
}

// buildSharedItinerary groups itinerary items by day for the public response.
func buildSharedItinerary(items []dbgen.ItineraryItem) []sharedItineraryDay {
	dayMap := make(map[int32]*sharedItineraryDay)
	var dayOrder []int32

	for _, item := range items {
		dayNum := int32(0)
		if item.DayNumber.Valid {
			dayNum = item.DayNumber.Int32
		}

		day, ok := dayMap[dayNum]
		if !ok {
			day = &sharedItineraryDay{
				DayNumber: dayNum,
				Items:     []sharedItineraryItem{},
			}
			dayMap[dayNum] = day
			dayOrder = append(dayOrder, dayNum)
		}

		si := sharedItineraryItem{}
		if item.Title.Valid {
			si.Title = item.Title.String
		}
		if item.Type.Valid {
			si.Type = item.Type.String
		}
		if item.Description.Valid {
			si.Description = item.Description.String
		}

		day.Items = append(day.Items, si)
	}

	result := make([]sharedItineraryDay, 0, len(dayOrder))
	for _, dayNum := range dayOrder {
		result = append(result, *dayMap[dayNum])
	}
	return result
}
