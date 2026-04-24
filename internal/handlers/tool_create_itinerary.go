package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gallowaysoftware/toqui-backend/internal/ai"
	"github.com/gallowaysoftware/toqui-backend/internal/analytics"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui-backend/internal/trip"
)

// itineraryItemWithLocation pairs a created DB item with an optional location
// name that will be resolved to coordinates in the background.
type itineraryItemWithLocation struct {
	item         dbgen.ItineraryItem
	locationName string
}

// CreateItineraryTool is a chat tool that lets the AI add structured itinerary items
// to the current trip. It's injected into planning mode chat so the AI can say
// "Let me add that to your itinerary" and actually create the items.
//
// In selection mode the tool is instantiated with a deferred trip-ID provider
// that resolves to the trip created earlier in the same turn (#181). When
// neither tripID nor a provider yields a UUID, Execute returns an error so
// the AI can recover gracefully.
//
// callerID is the authenticated user whose authz is re-checked on every
// INSERT via the SQL-gated CreateItineraryItemForOwnerOrEditor query
// (#353). This closes a TOCTOU window: chat sessions last tens of
// seconds to minutes, and a collaborator revoked mid-stream would
// otherwise still land inserts past the handler pre-check. It also
// hardens the selection-mode req.Msg.TripId fallback — if a caller
// lies about a trip_id they don't own/edit, the gated insert misses
// and the tool returns a clean "forbidden" error rather than silently
// writing to someone else's trip.
type CreateItineraryTool struct {
	tripSvc         *trip.Service
	tripID          uuid.UUID
	tripIDProvider  func() (uuid.UUID, bool)
	callerID        uuid.UUID // authz subject for per-insert SQL gate (#353)
	userID          string    // for analytics only (hashed before sending)
	onCreated       func(items []dbgen.ItineraryItem)
	pool            *pgxpool.Pool
	placesAPIKey    string
	analyticsClient *analytics.Client
}

type createItineraryArgs struct {
	Items []createItineraryItemArg `json:"items"`
}

type createItineraryItemArg struct {
	DayNumber          int    `json:"day_number"`
	OrderInDay         int    `json:"order_in_day"`
	Type               string `json:"type"`
	Title              string `json:"title"`
	Description        string `json:"description"`
	LocationName       string `json:"location_name"`
	EstimatedCostCents *int64 `json:"estimated_cost_cents,omitempty"`
	CostCurrency       string `json:"cost_currency,omitempty"`
}

// NewCreateItineraryTool constructs the chat tool. callerID MUST be the
// authenticated user ID from the request context — it's the authz
// subject for the SQL-gated INSERT and cannot be spoofed by AI tool
// args or untrusted request fields (#353).
func NewCreateItineraryTool(tripSvc *trip.Service, tripID, callerID uuid.UUID, onCreated func(items []dbgen.ItineraryItem)) *CreateItineraryTool {
	return &CreateItineraryTool{tripSvc: tripSvc, tripID: tripID, callerID: callerID, onCreated: onCreated}
}

// WithDeferredTripID returns a copy of the tool that resolves the target trip
// ID lazily at execution time via the provided function. Used in selection
// mode where the trip is created earlier in the same turn by create_trip and
// the resulting UUID isn't known when the tool is constructed (#181).
func (t *CreateItineraryTool) WithDeferredTripID(provider func() (uuid.UUID, bool)) *CreateItineraryTool {
	cp := *t
	cp.tripIDProvider = provider
	return &cp
}

// WithGeocoding returns a copy of the tool configured to geocode location_name
// values for each created item in a background goroutine.
func (t *CreateItineraryTool) WithGeocoding(pool *pgxpool.Pool, placesAPIKey string) *CreateItineraryTool {
	cp := *t
	cp.pool = pool
	cp.placesAPIKey = placesAPIKey
	return &cp
}

// WithAnalytics returns a copy of the tool configured to send events to PostHog.
func (t *CreateItineraryTool) WithAnalytics(client *analytics.Client, userID string) *CreateItineraryTool {
	cp := *t
	cp.analyticsClient = client
	cp.userID = userID
	return &cp
}

func (t *CreateItineraryTool) Definition() ai.ToolDefinition {
	return ai.ToolDefinition{
		Name:        "create_itinerary_items",
		Description: "Add structured itinerary items to the current trip. Call this when you have specific activities, meals, or experiences to suggest. You can add multiple items at once across multiple days. Group items by neighborhood to minimize transit. Use order_in_day to reflect the natural flow of a day (morning sightseeing, lunch, afternoon activities, dinner, evening). Each item's description should include: estimated duration, a practical tip, and transit notes to the next stop when locations are far apart.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"items": {
					"type": "array",
					"description": "List of itinerary items to add. Order items logically within each day: morning first, evening last. Group nearby locations together.",
					"items": {
						"type": "object",
						"properties": {
							"day_number": {
								"type": "integer",
								"description": "Day number (1-indexed)"
							},
							"order_in_day": {
								"type": "integer",
								"description": "Order within the day (1-indexed). Use this to sequence items chronologically: 1 for first morning activity, 2 for mid-morning, etc."
							},
							"type": {
								"type": "string",
								"enum": ["activity", "meal", "transport", "accommodation", "sightseeing", "shopping", "nightlife"],
								"description": "Type of itinerary item"
							},
							"title": {
								"type": "string",
								"description": "Short title, e.g. 'Visit Fushimi Inari Shrine' or 'Lunch at Nishiki Market'"
							},
							"description": {
								"type": "string",
								"description": "Include: (1) estimated duration (e.g., 'Allow 2-3 hours'), (2) a practical tip (e.g., 'Go early to avoid crowds', 'Book 2 weeks ahead'), (3) transit note to next stop if far (e.g., '15 min walk to next stop' or 'Take metro Line 2, 20 min'). Example: 'Allow 2-3 hours. The shrine is stunning at sunrise with fewer crowds. 10 min taxi to Gion district afterward.'"
							},
							"location_name": {
								"type": "string",
								"description": "Specific, geocodable place name including city/region, e.g. 'Fushimi Inari Shrine, Kyoto, Japan' or 'Eiffel Tower, Paris, France'. Be precise enough to place on a map."
							},
							"estimated_cost_cents": {
								"type": "integer",
								"description": "Estimated cost in cents (e.g. 2500 = $25.00). Only set when the user has a trip budget or asks about costs."
							},
							"cost_currency": {
								"type": "string",
								"description": "ISO 4217 currency code for the cost (e.g. 'USD', 'EUR'). Use the trip's budget currency when available."
							}
						},
						"required": ["day_number", "title", "type"]
					}
				}
			},
			"required": ["items"]
		}`),
	}
}

func (t *CreateItineraryTool) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	var params createItineraryArgs
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("parse args: %w", err)
	}

	if len(params.Items) == 0 {
		return nil, fmt.Errorf("at least one item is required")
	}

	// Resolve target trip: prefer the static tripID; fall back to the deferred
	// provider used by selection mode (#181). When neither is set, surface a
	// clear error so the AI can recover instead of getting an unknown-tool
	// confusion that triggers retry loops.
	tripID := t.tripID
	if tripID == uuid.Nil && t.tripIDProvider != nil {
		if id, ok := t.tripIDProvider(); ok {
			tripID = id
		}
	}
	if tripID == uuid.Nil {
		return json.Marshal(map[string]any{
			"error":   "no_trip_selected",
			"message": "Cannot add itinerary items: no trip is currently selected. Create a trip first using create_trip, then call this tool again.",
		})
	}

	// Load existing items for deduplication through the authz-gated
	// helper so the dedup peek never leaks another user's itinerary
	// metadata back to the AI (and from there into the response). Dedup
	// is an optimisation, not a correctness feature — on any error
	// (authz denied, transient DB) we fall through with existing=nil;
	// the subsequent SQL-gated INSERT still blocks unauthorised writes.
	existing, err := t.tripSvc.GetItineraryForOwnerOrEditor(ctx, t.callerID, tripID)
	if err != nil {
		if errors.Is(err, trip.ErrNotOwnerOrEditor) {
			slog.Info("dedup peek skipped: caller is not owner/editor", "trip_id", tripID, "caller_id", t.callerID)
		} else {
			slog.Warn("failed to load existing itinerary for dedup, proceeding without dedup", "trip_id", tripID, "error", err)
		}
		existing = nil
	}

	var created []itineraryItemWithLocation
	var failed []string
	var skipped int
	for _, item := range params.Items {
		if item.Title == "" {
			continue
		}
		if isDuplicateItem(existing, item.DayNumber, item.Title) {
			// Do NOT log item title — it's travel content (e.g. "Visit
			// Yad Vashem"). CLAUDE.md bans that from logs.
			// See toqui-backend#369 P1 #11.
			slog.Info("skipped duplicate itinerary item", "trip_id", tripID, "day", item.DayNumber)
			skipped++
			continue
		}
		dbItem, err := t.tripSvc.CreateItineraryItemForOwnerOrEditor(ctx, t.callerID, tripID, item.DayNumber, item.OrderInDay, item.Type, item.Title, item.Description, item.EstimatedCostCents, item.CostCurrency)
		if err != nil {
			// Authz gate missed — caller lost edit rights since the
			// handler pre-check (revocation mid-stream, or a spoofed
			// trip_id in selection mode). Short-circuit the whole
			// tool call so the AI reports the denial cleanly instead
			// of pretending every item was a shape bug. No further
			// inserts in this batch can succeed either, so bailing
			// out early also avoids dozens of identical DB queries.
			if errors.Is(err, trip.ErrNotOwnerOrEditor) {
				slog.Warn("create_itinerary_items authz denied",
					"trip_id", tripID,
					"caller_id", t.callerID,
					"pending_items", len(params.Items)-len(created)-skipped,
				)
				return json.Marshal(map[string]any{
					"error":   "forbidden",
					"message": "You don't have edit access to this trip anymore. Tell the user their access was revoked or they need to select a trip they own. Do NOT retry.",
				})
			}
			// Do NOT include item title in the log.
			slog.Error("create itinerary item", "trip_id", tripID, "day", item.DayNumber, "error", err)
			failed = append(failed, item.Title)
			continue
		}
		created = append(created, itineraryItemWithLocation{item: dbItem, locationName: item.LocationName})
	}

	if len(created) == 0 {
		// Differentiate the failure modes so the AI can recover instead of
		// blindly retrying with the same payload (#183, Run 4 R-20).
		//
		// For the all-duplicates case we specifically return a SUCCESS-shaped
		// response (no "error" key) because the AI interprets any JSON with
		// an "error" field as a failure and retries — which produced the
		// Run 4 retry-loop regression. Telling the AI "these items are
		// already present" is a success outcome, not an error.
		//
		// Field-naming matters for the narration bug (#190 LB-3, run22
		// N-01): the AI previously saw `created_count: 0` + a polite
		// message and still wrote "I've added it to your itinerary" in the
		// user-facing reply. The new shape spells out BOTH counts with
		// names the model can't misread — `newly_created_count` and
		// `already_present_count` — and the status + message are worded
		// to disallow any "added" narration.
		switch {
		case skipped > 0 && len(failed) == 0:
			return json.Marshal(map[string]any{
				"status":                "nothing_added_already_present",
				"newly_created_count":   0,
				"already_present_count": skipped,
				"persisted":             false,
				"message":               fmt.Sprintf("NOTHING WAS ADDED. All %d requested items are ALREADY in the user's itinerary on the specified days. Do NOT tell the user you added, created, scheduled, or saved anything. Instead, tell them the items are already on their plan. Do NOT call this tool again with the same items.", skipped),
			})
		case len(failed) > 0:
			return json.Marshal(map[string]any{
				"error":               "all_failed",
				"newly_created_count": 0,
				"persisted":           false,
				"message":             "Every item failed to persist. NOTHING WAS ADDED. Check that day_number is a positive integer and that title is non-empty, then call the tool again with corrected items. Do NOT tell the user you added anything.",
				"failed_items":        failed,
			})
		default:
			return json.Marshal(map[string]any{
				"error":               "no_valid_items",
				"newly_created_count": 0,
				"persisted":           false,
				"message":             "No valid items were provided. NOTHING WAS ADDED. Each item needs a non-empty title and a positive day_number. Retry with at least one item. Do NOT tell the user you added anything.",
			})
		}
	}

	dbItems := make([]dbgen.ItineraryItem, len(created))
	toGeocode := make([]itineraryItemWithLocation, 0, len(created))
	for i, c := range created {
		dbItems[i] = c.item
		if c.locationName != "" {
			toGeocode = append(toGeocode, c)
		}
	}

	if t.onCreated != nil {
		t.onCreated(dbItems)
	}

	// Track itinerary generation (async, non-blocking, no content — just counts)
	if t.analyticsClient != nil {
		daySet := make(map[int]struct{})
		for _, c := range created {
			if c.item.DayNumber.Valid {
				daySet[int(c.item.DayNumber.Int32)] = struct{}{}
			}
		}
		t.analyticsClient.Track(t.userID, "itinerary_generated", map[string]any{
			"item_count": len(created),
			"day_count":  len(daySet),
		})
	}

	// Fire-and-forget background geocoding so it never delays the streaming response.
	if t.pool != nil && t.placesAPIKey != "" && len(toGeocode) > 0 {
		go t.geocodeItems(toGeocode)
	}

	// Build summary for the AI
	summary := make([]map[string]any, len(created))
	for i, c := range created {
		entry := map[string]any{
			"id":    c.item.ID.String(),
			"title": c.item.Title.String,
		}
		if c.item.DayNumber.Valid {
			entry["day_number"] = c.item.DayNumber.Int32
		}
		if c.item.Type.Valid {
			entry["type"] = c.item.Type.String
		}
		summary[i] = entry
	}

	msg := fmt.Sprintf("Successfully added %d items to the itinerary.", len(created))
	if skipped > 0 || len(failed) > 0 {
		parts := []string{fmt.Sprintf("Added %d items to the itinerary.", len(created))}
		if skipped > 0 {
			parts = append(parts, fmt.Sprintf("Skipped %d duplicate(s) already in the itinerary.", skipped))
		}
		if len(failed) > 0 {
			parts = append(parts, fmt.Sprintf("%d items failed: %s.", len(failed), strings.Join(failed, ", ")))
		}
		msg = strings.Join(parts, " ")
	}

	// `newly_created_count` is the authoritative "did we actually persist
	// anything?" counter for the AI. Naming it the same across the
	// success and already-present paths lets the AI key off a single
	// field when narrating. Keep `created_count` as an alias for
	// backward-compat with any older prompt snippets / telemetry until
	// all call sites migrate.
	result := map[string]any{
		"newly_created_count": len(created),
		"created_count":       len(created), // legacy alias — remove once prompts migrate
		"persisted":           true,
		"items":               summary,
		"message":             msg,
	}
	if skipped > 0 {
		result["already_present_count"] = skipped
		result["skipped_duplicates"] = skipped // legacy alias
	}
	if len(failed) > 0 {
		result["failed_count"] = len(failed)
		result["failed_items"] = failed
	}
	return json.Marshal(result)
}

// normalizeTitle lowercases and collapses whitespace for fuzzy comparison.
func normalizeTitle(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// dedupOverlapThreshold is the fraction of significant words that must
// overlap between two titles on the same day for the new item to be
// treated as a duplicate. Raised from 0.6 → 0.7 to reduce false
// positives like "Visit Wat Saket" vs "Visit Wat Arun" (both share
// "visit" and "temple"-style words, ≥60% overlap, but refer to
// different places). 70% still catches the containment cases #322 /
// Run 16 originally fixed: "Day Trip: Montserrat" is a substring of
// "Day Trip to Montserrat Monastery" and trips the containment
// short-circuit before overlap even matters. See #190 LB-3.
const dedupOverlapThreshold = 0.7

// isDuplicateItem checks whether a new item on the given day has a title
// similar enough to an existing item to be considered a duplicate.
// Uses containment + word overlap. The threshold is deliberately high
// (see dedupOverlapThreshold) because a false-positive here is much
// worse than a false-negative: a false-positive returns
// `nothing_added_already_present` when the user DID want a new item,
// which was the #190 LB-3 narration bug.
func isDuplicateItem(existing []dbgen.ItineraryItem, dayNumber int, title string) bool {
	norm := normalizeTitle(title)
	if norm == "" {
		return false
	}
	newWords := significantWords(norm)
	for _, e := range existing {
		if !e.DayNumber.Valid || int(e.DayNumber.Int32) != dayNumber {
			continue
		}
		if !e.Title.Valid {
			continue
		}
		existNorm := normalizeTitle(e.Title.String)
		// Exact or containment match
		if existNorm == norm || strings.Contains(existNorm, norm) || strings.Contains(norm, existNorm) {
			return true
		}
		// Word overlap on significant words.
		existWords := significantWords(existNorm)
		if wordOverlapRatio(newWords, existWords) >= dedupOverlapThreshold ||
			wordOverlapRatio(existWords, newWords) >= dedupOverlapThreshold {
			return true
		}
	}
	return false
}

// significantWords returns the set of words longer than 3 chars (skips
// articles, prepositions, conjunctions that don't carry meaning).
func significantWords(s string) map[string]bool {
	words := make(map[string]bool)
	for _, w := range strings.Fields(s) {
		if len(w) > 3 {
			words[w] = true
		}
	}
	return words
}

// wordOverlapRatio returns the fraction of words in `a` that also appear in `b`.
func wordOverlapRatio(a, b map[string]bool) float64 {
	if len(a) == 0 {
		return 0
	}
	overlap := 0
	for w := range a {
		if b[w] {
			overlap++
		}
	}
	return float64(overlap) / float64(len(a))
}

// geocodeItems resolves location names to coordinates and persists them.
// Runs in a background goroutine — failures are logged and never surface to the user.
// Cap at 20 items to avoid excessive API calls in a single batch.
func (t *CreateItineraryTool) geocodeItems(items []itineraryItemWithLocation) {
	const maxBatch = 20
	if len(items) > maxBatch {
		items = items[:maxBatch]
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, c := range items {
		lat, lng, err := geocodeLocation(ctx, t.placesAPIKey, c.locationName)
		if err != nil {
			slog.Warn("geocode itinerary item failed",
				"item_id", c.item.ID,
				"location_name", c.locationName,
				"error", err,
			)
			continue
		}
		if lat == 0 && lng == 0 {
			continue
		}

		const updateSQL = `UPDATE itinerary_items
			SET location = ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography
			WHERE id = $3`
		if _, err := t.pool.Exec(ctx, updateSQL, lng, lat, c.item.ID); err != nil {
			slog.Warn("update itinerary item location failed",
				"item_id", c.item.ID,
				"lat", lat,
				"lng", lng,
				"error", err,
			)
		} else {
			slog.Debug("geocoded itinerary item",
				"item_id", c.item.ID,
				"location_name", c.locationName,
				"lat", lat,
				"lng", lng,
			)
		}
	}
}
