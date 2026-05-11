package booking

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gallowaysoftware/toqui-backend/internal/ai"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui-backend/internal/trip"
)

// bookingQueries is the slice of *dbgen.Queries that Service uses.
// Defining a small interface here lets unit tests inject a stub
// without spinning up Postgres. Mirrors paymentQueries (#418),
// subscriptionQueries (#424), tripQueries (#427), lifecycleQueries
// (#431). Same fail-loud test-double philosophy.
type bookingQueries interface {
	FindBookingByConfirmationCode(ctx context.Context, arg dbgen.FindBookingByConfirmationCodeParams) (dbgen.Booking, error)
	FindBookingFuzzy(ctx context.Context, arg dbgen.FindBookingFuzzyParams) (dbgen.Booking, error)
	MergeBooking(ctx context.Context, arg dbgen.MergeBookingParams) (dbgen.Booking, error)
	CreateBookingForOwnerOrEditor(ctx context.Context, arg dbgen.CreateBookingForOwnerOrEditorParams) (dbgen.Booking, error)
	UpdateBooking(ctx context.Context, arg dbgen.UpdateBookingParams) (dbgen.Booking, error)
	GetTripCostSummary(ctx context.Context, arg dbgen.GetTripCostSummaryParams) ([]dbgen.GetTripCostSummaryRow, error)
	ListBookingsByTrip(ctx context.Context, arg dbgen.ListBookingsByTripParams) ([]dbgen.Booking, error)
	GetBookingByID(ctx context.Context, arg dbgen.GetBookingByIDParams) (dbgen.Booking, error)
	DeleteBooking(ctx context.Context, arg dbgen.DeleteBookingParams) (int64, error)
	LinkBookingToTripForOwnerOrEditor(ctx context.Context, arg dbgen.LinkBookingToTripForOwnerOrEditorParams) (dbgen.Booking, error)

	// Used by post-ingest analysis (DetectConflicts + AnalyzeCoverage). Both
	// are best-effort — failures here log a warning but do not fail the
	// ingest, since the booking has already been written.
	GetTripByID(ctx context.Context, arg dbgen.GetTripByIDParams) (dbgen.Trip, error)
	ListItineraryItemsByTrip(ctx context.Context, tripID uuid.UUID) ([]dbgen.ItineraryItem, error)
}

var _ bookingQueries = (*dbgen.Queries)(nil)

type Service struct {
	queries    bookingQueries
	aiProvider ai.Provider
}

func NewService(pool *pgxpool.Pool, aiProvider ai.Provider) *Service {
	return &Service{
		queries:    dbgen.New(pool),
		aiProvider: aiProvider,
	}
}

// IngestResult is the result of an IngestText or IngestEmail call.
// WasUpdated is true when an existing booking was merged rather than a
// new one created. WasUpdated is false when (a) a brand-new booking was
// created, or (b) a duplicate was found but the merge was skipped
// because the user has manually edited the existing record (we never
// silently overwrite user edits). PreviousID is set in both merge cases
// — either the merged-into row's ID or the user-edited row's ID.
//
// Conflicts and CoverageGap are best-effort post-ingest feedback for the
// trip the booking belongs to. They surface scheduling problems
// (DetectConflicts) and the single highest-priority missing piece of
// the trip plan (AnalyzeCoverage) so the frontend / chat can prompt the
// user. Both are nil when the booking is unattached (no trip), when the
// trip lookup fails, or when no conflicts/gaps are detected.
type IngestResult struct {
	Booking     *dbgen.Booking
	WasUpdated  bool
	PreviousID  string
	Conflicts   []Conflict
	CoverageGap *CoverageGap
}

type ParsedBooking struct {
	Type              string          `json:"type"`
	ConfirmationCode  string          `json:"confirmation_code"`
	Provider          string          `json:"provider"`
	Title             string          `json:"title"`
	StartTime         string          `json:"start_time"`
	EndTime           string          `json:"end_time"`
	Address           string          `json:"address"`
	DepartureLocation string          `json:"departure_location"`
	ArrivalLocation   string          `json:"arrival_location"`
	NumGuests         int32           `json:"num_guests"`
	PriceCents        int64           `json:"price_cents"`
	Currency          string          `json:"currency"`
	Timezone          string          `json:"timezone"`
	Details           json.RawMessage `json:"details"`
}

func (s *Service) IngestText(ctx context.Context, userID uuid.UUID, tripID string, typeHint string, rawText string) (*IngestResult, error) {
	return s.ingest(ctx, userID, tripID, typeHint, rawText, "paste")
}

// IngestEmail parses raw email text with AI and creates a booking record with source="email".
func (s *Service) IngestEmail(ctx context.Context, userID uuid.UUID, tripID string, typeHint string, rawText string) (*IngestResult, error) {
	return s.ingest(ctx, userID, tripID, typeHint, rawText, "email")
}

func (s *Service) ingest(ctx context.Context, userID uuid.UUID, tripID string, typeHint string, rawText string, source string) (*IngestResult, error) {
	parsed, err := s.parseWithAI(ctx, rawText, typeHint)
	if err != nil {
		return nil, fmt.Errorf("parse booking: %w", err)
	}

	var tripUUID pgtype.UUID
	if tripID != "" {
		id, err := uuid.Parse(tripID)
		if err == nil {
			tripUUID = pgtype.UUID{Bytes: id, Valid: true}
		}
	}

	// Step 1 — exact confirmation code match: same user + trip + code → merge.
	if parsed.ConfirmationCode != "" && tripUUID.Valid {
		existing, err := s.queries.FindBookingByConfirmationCode(ctx, dbgen.FindBookingByConfirmationCodeParams{
			UserID:           userID,
			TripID:           tripUUID,
			ConfirmationCode: pgtype.Text{String: parsed.ConfirmationCode, Valid: true},
		})
		if err == nil {
			// Found an exact match — merge the new data in and return.
			result, err := s.tryMergeOrPreserve(ctx, userID, tripUUID, existing, parsed, rawText, "confirmation code")
			if err != nil {
				return nil, err
			}
			s.attachPostIngestAnalysis(ctx, userID, tripUUID, result)
			return result, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("check duplicate booking: %w", err)
		}
	}

	// Step 2 — fuzzy match fallback: same type + trip + date within 7 days + provider similarity.
	if tripUUID.Valid {
		candidate, found, err := s.findFuzzyMatch(ctx, userID, tripUUID, parsed)
		if err != nil {
			// Non-ErrNoRows error — propagate so the caller sees a real failure
			// instead of silently dropping into create-and-duplicate.
			return nil, fmt.Errorf("fuzzy match lookup: %w", err)
		}
		if found {
			result, err := s.tryMergeOrPreserve(ctx, userID, tripUUID, candidate, parsed, rawText, "fuzzy match")
			if err != nil {
				return nil, err
			}
			s.attachPostIngestAnalysis(ctx, userID, tripUUID, result)
			return result, nil
		}
	}

	// Step 3 — no match found, create a new booking.
	// Authz-gated insert: when a trip_id is supplied, the WHERE clause
	// re-checks the caller can edit that trip. A predicate miss (i.e.
	// the caller doesn't own the trip and isn't an accepted editor)
	// returns zero rows → pgx.ErrNoRows → trip.ErrNotOwnerOrEditor.
	// This closes the #361 P1 exploit where a client could plant a
	// booking onto any victim trip UUID by passing it in IngestBooking.
	// For unattached bookings (tripUUID.Valid == false) the gate is
	// a no-op — unattached bookings are self-scoped.
	var startTime, endTime pgtype.Timestamptz
	if t, err := time.Parse(time.RFC3339, parsed.StartTime); err == nil {
		startTime = pgtype.Timestamptz{Time: t, Valid: true}
	}
	if t, err := time.Parse(time.RFC3339, parsed.EndTime); err == nil {
		endTime = pgtype.Timestamptz{Time: t, Valid: true}
	}

	booking, err := s.queries.CreateBookingForOwnerOrEditor(ctx, dbgen.CreateBookingForOwnerOrEditorParams{
		UserID:            userID,
		TripID:            tripUUID,
		Type:              parsed.Type,
		ConfirmationCode:  pgtype.Text{String: parsed.ConfirmationCode, Valid: parsed.ConfirmationCode != ""},
		Provider:          pgtype.Text{String: parsed.Provider, Valid: parsed.Provider != ""},
		Title:             parsed.Title,
		StartTime:         startTime,
		EndTime:           endTime,
		DetailsJson:       parsed.Details,
		RawSource:         pgtype.Text{String: rawText, Valid: true},
		Source:            source,
		DepartureLocation: pgtype.Text{String: parsed.DepartureLocation, Valid: parsed.DepartureLocation != ""},
		ArrivalLocation:   pgtype.Text{String: parsed.ArrivalLocation, Valid: parsed.ArrivalLocation != ""},
		NumGuests:         pgtype.Int4{Int32: parsed.NumGuests, Valid: parsed.NumGuests > 0},
		PriceCents:        pgtype.Int8{Int64: parsed.PriceCents, Valid: parsed.PriceCents > 0},
		Currency:          pgtype.Text{String: parsed.Currency, Valid: parsed.Currency != ""},
		Timezone:          pgtype.Text{String: parsed.Timezone, Valid: parsed.Timezone != ""},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, trip.ErrNotOwnerOrEditor
		}
		// Race recovery: a unique-constraint violation on
		// (user_id, trip_id, confirmation_code) means a concurrent
		// request inserted a matching booking between our SELECT in
		// step 1 and our INSERT here. Re-fetch and merge so the second
		// caller still sees the de-duplicated outcome rather than
		// surfacing an opaque DB error.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" &&
			parsed.ConfirmationCode != "" && tripUUID.Valid {
			existing, ferr := s.queries.FindBookingByConfirmationCode(ctx, dbgen.FindBookingByConfirmationCodeParams{
				UserID:           userID,
				TripID:           tripUUID,
				ConfirmationCode: pgtype.Text{String: parsed.ConfirmationCode, Valid: true},
			})
			if ferr != nil {
				return nil, fmt.Errorf("create booking unique violation, refetch failed: %w", ferr)
			}
			slog.InfoContext(ctx, "booking ingest race recovered via unique constraint",
				"user_id", userID,
				"booking_id", existing.ID,
				"type", existing.Type,
			)
			result, mergeErr := s.tryMergeOrPreserve(ctx, userID, tripUUID, existing, parsed, rawText, "race recovery")
			if mergeErr != nil {
				return nil, mergeErr
			}
			s.attachPostIngestAnalysis(ctx, userID, tripUUID, result)
			return result, nil
		}
		return nil, fmt.Errorf("create booking: %w", err)
	}

	result := &IngestResult{Booking: &booking}
	s.attachPostIngestAnalysis(ctx, userID, tripUUID, result)
	return result, nil
}

// attachPostIngestAnalysis populates result.Conflicts and result.CoverageGap
// using DetectConflicts and AnalyzeCoverage. Best-effort: any DB error here
// is logged at warn but does not propagate — the booking has already been
// written and should be returned regardless. When the booking is unattached
// (no trip_id) we skip analysis entirely since both detectors are
// trip-scoped.
func (s *Service) attachPostIngestAnalysis(ctx context.Context, userID uuid.UUID, tripUUID pgtype.UUID, result *IngestResult) {
	if !tripUUID.Valid {
		return
	}
	tripID := uuid.UUID(tripUUID.Bytes)

	bookings, err := s.queries.ListBookingsByTrip(ctx, dbgen.ListBookingsByTripParams{
		TripID: tripUUID,
		UserID: userID,
	})
	if err != nil {
		slog.WarnContext(ctx, "post-ingest analysis: list bookings failed",
			"user_id", userID,
			"error", err,
		)
		return
	}

	conflicts := DetectConflicts(bookings)
	if len(conflicts) > 0 {
		result.Conflicts = conflicts
		// Privacy: log type counts, never the messages or IDs of bookings.
		// The conflict messages are user-facing and don't carry travel
		// content, but we still keep operational logs minimal.
		typeCounts := map[string]int{}
		for _, c := range conflicts {
			typeCounts[c.Type]++
		}
		slog.InfoContext(ctx, "post-ingest conflicts detected",
			"user_id", userID,
			"trip_id", tripID,
			"type_counts", typeCounts,
		)
	}

	trip, err := s.queries.GetTripByID(ctx, dbgen.GetTripByIDParams{
		ID:     tripID,
		UserID: userID,
	})
	if err != nil {
		// Trip lookup failure is not fatal — we just skip coverage analysis.
		// This typically means the booking is on a trip the user no longer
		// owns (e.g. collaborator), in which case coverage suggestions are
		// not the user's call to act on anyway.
		slog.WarnContext(ctx, "post-ingest analysis: get trip failed",
			"user_id", userID,
			"trip_id", tripID,
			"error", err,
		)
		return
	}

	itineraryItems, err := s.queries.ListItineraryItemsByTrip(ctx, tripID)
	if err != nil {
		slog.WarnContext(ctx, "post-ingest analysis: list itinerary failed",
			"user_id", userID,
			"trip_id", tripID,
			"error", err,
		)
		// Don't return — AnalyzeCoverage tolerates 0 itinerary items, just
		// won't fire the sparse_itinerary check. Better to surface
		// no_accommodation than to bail entirely on a transient itinerary
		// query failure.
	}

	if gap := AnalyzeCoverage(trip, bookings, len(itineraryItems)); gap != nil {
		result.CoverageGap = gap
		slog.InfoContext(ctx, "post-ingest coverage gap detected",
			"user_id", userID,
			"trip_id", tripID,
			"gap_type", gap.Type,
			"priority", gap.Priority,
		)
	}
}

// tryMergeOrPreserve runs MergeBooking against an existing record. The
// SQL WHERE clause refuses to overwrite a booking the user has touched
// (updated_at > created_at + 1s), so a pgx.ErrNoRows return here means
// "user-edited, leave it alone" — we surface the existing record with
// WasUpdated=false. Real DB errors are wrapped and returned. The
// `viaPath` argument is for log tagging only.
func (s *Service) tryMergeOrPreserve(
	ctx context.Context,
	userID uuid.UUID,
	tripUUID pgtype.UUID,
	existing dbgen.Booking,
	parsed *ParsedBooking,
	rawText string,
	viaPath string,
) (*IngestResult, error) {
	merged, err := s.mergeIntoExisting(ctx, userID, tripUUID, existing.ID, parsed, rawText)
	if err == nil {
		slog.InfoContext(ctx, "booking merged",
			"user_id", userID,
			"booking_id", merged.ID,
			"type", merged.Type,
			"via", viaPath,
		)
		return &IngestResult{Booking: merged, WasUpdated: true, PreviousID: existing.ID.String()}, nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		// User has edited this booking since creation (or trip_id
		// guard rejected it). Preserve their work — return the
		// existing record without modification.
		preserved := existing
		slog.InfoContext(ctx, "booking re-import preserved user edits",
			"user_id", userID,
			"booking_id", existing.ID,
			"type", existing.Type,
			"via", viaPath,
		)
		return &IngestResult{Booking: &preserved, WasUpdated: false, PreviousID: existing.ID.String()}, nil
	}
	return nil, err
}

// mergeIntoExisting updates an existing booking with the parsed fields from
// a re-import. Non-empty values from the new import win; existing values are
// preserved when the new import has nothing (COALESCE pattern in SQL).
//
// The MergeBooking SQL WHERE clause includes trip_id (defense-in-depth)
// and an updated_at <= created_at + 1s guard that prevents clobbering
// user edits — see db/queries/bookings.sql for the rationale. A
// predicate miss returns pgx.ErrNoRows; the caller (tryMergeOrPreserve)
// is responsible for distinguishing that from a true error.
func (s *Service) mergeIntoExisting(ctx context.Context, userID uuid.UUID, tripUUID pgtype.UUID, bookingID uuid.UUID, parsed *ParsedBooking, rawText string) (*dbgen.Booking, error) {
	var startTime, endTime pgtype.Timestamptz
	if t, err := time.Parse(time.RFC3339, parsed.StartTime); err == nil {
		startTime = pgtype.Timestamptz{Time: t, Valid: true}
	}
	if t, err := time.Parse(time.RFC3339, parsed.EndTime); err == nil {
		endTime = pgtype.Timestamptz{Time: t, Valid: true}
	}

	merged, err := s.queries.MergeBooking(ctx, dbgen.MergeBookingParams{
		ID:                bookingID,
		UserID:            userID,
		TripID:            tripUUID,
		Type:              parsed.Type,
		ConfirmationCode:  parsed.ConfirmationCode,
		Provider:          parsed.Provider,
		Title:             parsed.Title,
		StartTime:         startTime,
		EndTime:           endTime,
		Address:           parsed.Address,
		DepartureLocation: parsed.DepartureLocation,
		ArrivalLocation:   parsed.ArrivalLocation,
		NumGuests:         pgtype.Int4{Int32: parsed.NumGuests, Valid: parsed.NumGuests > 0},
		PriceCents:        pgtype.Int8{Int64: parsed.PriceCents, Valid: parsed.PriceCents > 0},
		Currency:          parsed.Currency,
		Timezone:          parsed.Timezone,
		DetailsJson:       parsed.Details,
		RawSource:         rawText,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Caller distinguishes user-edited from real error via
			// errors.Is — propagate the sentinel unwrapped.
			return nil, err
		}
		return nil, fmt.Errorf("merge booking: %w", err)
	}
	return &merged, nil
}

// findFuzzyMatch checks whether a fuzzy candidate exists for parsed and, if
// so, applies a provider name similarity filter. Returns
// (candidate, found, error). `found` is true only on a real match;
// (zero, false, nil) means no candidate exists or a candidate was rejected
// by the provider-similarity filter; a non-nil error means the lookup
// itself failed and the caller should propagate it rather than silently
// falling through to create-and-duplicate.
//
// Provider similarity: one provider string must contain the other
// (case-insensitive). This handles minor variations like "BC Ferries" vs
// "BC Ferries Ltd" while blocking cross-carrier false positives (e.g.
// "Delta" vs "United").
//
// When either provider string is empty the provider check is skipped so that
// bookings with no provider field can still be fuzzy-matched by date alone.
func (s *Service) findFuzzyMatch(ctx context.Context, userID uuid.UUID, tripUUID pgtype.UUID, parsed *ParsedBooking) (dbgen.Booking, bool, error) {
	startTime, err := time.Parse(time.RFC3339, parsed.StartTime)
	if err != nil {
		// Can't fuzzy-match without a parseable start time. Not an
		// error — many legitimate bookings have no start time (manual
		// paste, partial confirmations); the caller proceeds to create.
		return dbgen.Booking{}, false, nil //nolint:nilerr // missing start_time is a no-fuzzy signal, not a failure
	}

	candidate, err := s.queries.FindBookingFuzzy(ctx, dbgen.FindBookingFuzzyParams{
		UserID:    userID,
		TripID:    tripUUID,
		Type:      parsed.Type,
		StartTime: pgtype.Timestamptz{Time: startTime, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dbgen.Booking{}, false, nil
		}
		// Real DB error — propagate. Previously this path silently
		// fell through and created a duplicate booking, defeating the
		// dedup feature on flaky-DB conditions.
		return dbgen.Booking{}, false, err
	}

	// Provider name similarity check: skip when either is empty.
	if parsed.Provider != "" && candidate.Provider.Valid && candidate.Provider.String != "" {
		a := strings.ToLower(parsed.Provider)
		b := strings.ToLower(candidate.Provider.String)
		if !strings.Contains(a, b) && !strings.Contains(b, a) {
			return dbgen.Booking{}, false, nil
		}
	}

	return candidate, true, nil
}

func (s *Service) parseWithAI(ctx context.Context, rawText string, typeHint string) (*ParsedBooking, error) {
	prompt := rawText
	if typeHint != "" {
		prompt = fmt.Sprintf("Type hint: %s\n\n%s", typeHint, rawText)
	}

	req := &ai.ChatRequest{
		SystemPrompt: `You are a booking confirmation parser. Extract structured booking information from the text provided.
Return a JSON object with these fields:
- type: one of "flight", "hotel", "vacation_rental", "car_rental", "train", "tour", "activity", "restaurant", "ferry", "bus", "cruise", "transfer", "other". Use "hotel" for traditional hotels, hostels, B&Bs, inns, and resorts (commercial properties with a front desk). Use "vacation_rental" for whole-property rentals such as houses, cabins, villas, apartments, condos, cottages — typically from VRBO, Airbnb, or similar platforms where a guest rents an entire dwelling.
- confirmation_code: the booking/confirmation number
- provider: the company name (airline, hotel chain, ferry operator, bus company, etc.)
- title: a brief description of the booking
- start_time: ISO 8601 datetime if available. For flights, use the OUTBOUND departure time. For hotels, use check-in date+time. For tours/activities, use the start time. For ferries/buses, use departure time. ALWAYS populate this if any time info is present.
- end_time: ISO 8601 datetime if available. For flights, use the OUTBOUND arrival time. For hotels, use check-out date+time. For tours, use the end time. For ferries/buses, use arrival time.
- address: the location/address if available
- departure_location: departure city/airport/station/port (for flights, trains, ferries, buses, tours). For round-trip flights, use the OUTBOUND leg's departure.
- arrival_location: arrival city/airport/station/port (for flights, trains, ferries, buses, tours). For round-trip flights, use the OUTBOUND leg's arrival (the destination), NOT the return leg's arrival.
- num_guests: number of guests/passengers if available
- price_cents: total price in the smallest currency unit (e.g. cents). Convert dollars to cents (multiply by 100). If the price is "EUR 45.50", set price_cents to 4550. If no price is found, omit or set to 0.
- currency: ISO 4217 currency code (e.g. "USD", "EUR", "CAD", "GBP"). Infer from currency symbols: $=USD, €=EUR, £=GBP, C$=CAD, A$=AUD, ¥=JPY. If ambiguous, use "USD".
- timezone: IANA timezone identifier for the booking's local time (e.g. "America/New_York", "Europe/London", "Asia/Tokyo"). Infer from the location if not explicitly stated. If unknown, omit.
- details: a type-specific JSON object with the following schema based on type:

flight: {"airline":"","flight_number":"","departure_airport":"","arrival_airport":"","departure_terminal":"","arrival_terminal":"","seat":"","cabin_class":"","passengers":[],"legs":[{"flight_number":"","airline":"","departure_airport":"","arrival_airport":"","departure_time":"","arrival_time":"","cabin":""}]}
  For multi-segment, connecting, or round-trip flights, populate the "legs" array with one entry per flight segment (outbound leg first, then connecting segments, then return leg). Always keep the top-level flight fields (airline, flight_number, departure_airport, arrival_airport, etc.) populated with the FIRST outbound leg's values for backward compatibility. For single non-stop one-way flights, omit the "legs" array.
hotel: {"hotel_name":"","check_in_date":"","check_out_date":"","room_type":"","num_guests":0,"address":"","phone":""}
  For multi-property bookings (hostels, chains): use a "properties" array instead: {"properties":[{"hotel_name":"","address":"","check_in_date":"","check_out_date":"","room_type":"","nights":0,"rate_per_night":0}]}
vacation_rental: reuse the hotel schema ({"hotel_name":"","check_in_date":"","check_out_date":"","room_type":"","num_guests":0,"address":"","phone":""}). Put the property name/listing title in "hotel_name" and the unit/room description in "room_type" (e.g. "Entire cabin", "2BR villa").
car_rental: {"company":"","pickup_location":"","dropoff_location":"","pickup_time":"","dropoff_time":"","car_type":"","driver_name":""}
train: {"operator":"","train_number":"","departure_station":"","arrival_station":"","seat":"","car_number":"","class":""}
tour: {"tour_operator":"","tour_name":"","num_participants":0,"meeting_point":"","date":"YYYY-MM-DD","start_time":"HH:MM","duration":"","guide_name":"","price":"","stops":[{"name":"","location":"","duration":"","notes":""}]}
activity: {"operator":"","activity_name":"","location":"","num_guests":0,"date":"YYYY-MM-DD","start_time":"HH:MM","duration":"","guide_name":"","price":"","notes":""}
restaurant: {"restaurant_name":"","cuisine":"","party_size":0,"notes":""}
ferry: {"operator":"","vessel_name":"","departure_port":"","arrival_port":"","departure_time":"","arrival_time":"","cabin_type":"","deck":"","num_passengers":0,"vehicle_included":false}
bus: {"operator":"","route_number":"","departure_station":"","arrival_station":"","departure_time":"","arrival_time":"","seat":"","class":"","platform":""}
cruise: {"cruise_line":"","ship_name":"","departure_port":"","arrival_port":"","cabin_number":"","cabin_type":"","deck":"","num_passengers":0,"ports_of_call":[]}
transfer: {"operator":"","vehicle_type":"","pickup_location":"","dropoff_location":"","pickup_time":"","num_passengers":0,"driver_name":"","flight_number":""}

Only include fields that are present in the source text. Return ONLY valid JSON, no other text.`,
		Messages: []ai.Message{
			{Role: "user", Content: prompt},
		},
		// 4096 handled most single-property artifacts, but multi-stop hostel
		// bookings (3+ properties) produce larger JSON and were hitting MAX_TOKENS
		// (see toqui-backend#169). 16384 gives headroom for the largest multi-
		// property artifacts without breaking single-property cost economics.
		MaxTokens:   16384,
		Temperature: 0,
	}

	eventCh, err := s.aiProvider.ChatStream(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("AI parse: %w", err)
	}

	var response strings.Builder
	for event := range eventCh {
		switch event.Type {
		case ai.EventTextDelta:
			response.WriteString(event.Text)
		case ai.EventError:
			return nil, fmt.Errorf("AI parse stream error: %w", event.Error)
		}
	}

	raw := stripCodeFences(response.String())

	var parsed ParsedBooking
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, fmt.Errorf("unmarshal AI response: %w", err)
	}

	// Validate booking type against known values. The AI may hallucinate
	// unknown types; normalize them to "other" for safe storage.
	parsed.Type = normalizeBookingType(parsed.Type)

	return &parsed, nil
}

// validBookingTypes is the set of booking types supported by the system.
var validBookingTypes = map[string]bool{
	"flight":          true,
	"hotel":           true,
	"vacation_rental": true,
	"car_rental":      true,
	"train":           true,
	"tour":            true,
	"activity":        true,
	"restaurant":      true,
	"ferry":           true,
	"bus":             true,
	"cruise":          true,
	"transfer":        true,
	"other":           true,
}

// normalizeBookingType validates the AI-generated booking type against known
// values. Unknown types are mapped to "other" to prevent arbitrary string
// values from being stored in the database.
func normalizeBookingType(t string) string {
	t = strings.ToLower(strings.TrimSpace(t))
	if validBookingTypes[t] {
		return t
	}
	return "other"
}

// stripCodeFences removes markdown code fences (```json ... ```) that AI
// models often wrap around JSON output.
func stripCodeFences(s string) string {
	trimmed := strings.TrimSpace(s)
	// Handle ```json ... ``` or ``` ... ```
	if strings.HasPrefix(trimmed, "```") {
		// Remove opening fence (```json or ```)
		idx := strings.Index(trimmed, "\n")
		if idx != -1 {
			trimmed = trimmed[idx+1:]
		}
		// Remove closing fence
		trimmed = strings.TrimSuffix(trimmed, "```")
		trimmed = strings.TrimSpace(trimmed)
	}
	return trimmed
}

func (s *Service) Update(ctx context.Context, userID, bookingID uuid.UUID, params dbgen.UpdateBookingParams) (*dbgen.Booking, error) {
	params.ID = bookingID
	params.UserID = userID
	booking, err := s.queries.UpdateBooking(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("update booking: %w", err)
	}
	return &booking, nil
}

type CostSummary struct {
	Currency     string
	TotalCents   int64
	BookingCount int64
}

func (s *Service) GetTripCostSummary(ctx context.Context, userID, tripID uuid.UUID) ([]CostSummary, error) {
	rows, err := s.queries.GetTripCostSummary(ctx, dbgen.GetTripCostSummaryParams{
		TripID: pgtype.UUID{Bytes: tripID, Valid: true},
		UserID: userID,
	})
	if err != nil {
		return nil, fmt.Errorf("get trip cost summary: %w", err)
	}

	summaries := make([]CostSummary, len(rows))
	for i, r := range rows {
		summaries[i] = CostSummary{
			Currency:     r.Currency,
			TotalCents:   r.TotalCents,
			BookingCount: r.BookingCount,
		}
	}
	return summaries, nil
}

func (s *Service) ListByTrip(ctx context.Context, userID uuid.UUID, tripID uuid.UUID) ([]dbgen.Booking, error) {
	return s.queries.ListBookingsByTrip(ctx, dbgen.ListBookingsByTripParams{
		TripID: pgtype.UUID{Bytes: tripID, Valid: true},
		UserID: userID,
	})
}

func (s *Service) GetByID(ctx context.Context, userID, bookingID uuid.UUID) (*dbgen.Booking, error) {
	booking, err := s.queries.GetBookingByID(ctx, dbgen.GetBookingByIDParams{
		ID:     bookingID,
		UserID: userID,
	})
	if err != nil {
		return nil, fmt.Errorf("get booking: %w", err)
	}
	return &booking, nil
}

// Delete removes a booking owned by userID. Returns (deleted, error) where
// deleted is true when the row actually existed and belonged to userID.
// When deleted is false and error is nil, the booking either did not exist
// or belonged to a different user — callers should treat this as a no-op
// (HTTP idempotent DELETE semantics) but may audit the miss.
func (s *Service) Delete(ctx context.Context, userID, bookingID uuid.UUID) (bool, error) {
	rows, err := s.queries.DeleteBooking(ctx, dbgen.DeleteBookingParams{
		ID:     bookingID,
		UserID: userID,
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

func (s *Service) LinkToTrip(ctx context.Context, userID, bookingID, tripID uuid.UUID) (*dbgen.Booking, error) {
	// Authz-gated re-link (#361 P1 fix). The old LinkBookingToTrip
	// query verified booking ownership but not the target trip, so
	// any user could re-associate their own booking with a victim's
	// trip. The gated variant requires both booking ownership AND
	// edit rights on the target trip. Predicate miss → ErrNoRows →
	// trip.ErrNotOwnerOrEditor.
	booking, err := s.queries.LinkBookingToTripForOwnerOrEditor(ctx, dbgen.LinkBookingToTripForOwnerOrEditorParams{
		ID:     bookingID,
		TripID: pgtype.UUID{Bytes: tripID, Valid: true},
		UserID: userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, trip.ErrNotOwnerOrEditor
		}
		return nil, fmt.Errorf("link booking: %w", err)
	}
	return &booking, nil
}

type ExtractFieldResult struct {
	Answer          string         `json:"answer"`
	ExtractedFields map[string]any `json:"extracted_fields,omitempty"`
}

func (s *Service) ExtractField(ctx context.Context, userID, bookingID uuid.UUID, question string) (*ExtractFieldResult, error) {
	b, err := s.queries.GetBookingByID(ctx, dbgen.GetBookingByIDParams{
		ID:     bookingID,
		UserID: userID,
	})
	if err != nil {
		return nil, fmt.Errorf("get booking: %w", err)
	}

	rawSource := ""
	if b.RawSource.Valid {
		rawSource = b.RawSource.String
	}
	if rawSource == "" {
		return nil, fmt.Errorf("no raw source available for re-extraction")
	}

	req := &ai.ChatRequest{
		SystemPrompt: `You are a booking information extractor. Given raw booking source text and a question, answer the question based on the source.
Return a JSON object with:
- answer: your direct answer to the question
- extracted_fields: a map of field names to values for any structured data you extracted while answering

Return ONLY valid JSON, no other text.`,
		Messages: []ai.Message{
			{Role: "user", Content: fmt.Sprintf("Source:\n%s\n\nQuestion: %s", rawSource, question)},
		},
		// Bumped from 1024 alongside the parser bump (#150) to give multi-field
		// extractions room to breathe, since Gemini can produce chattier JSON
		// than Claude for the same prompt.
		MaxTokens:   2048,
		Temperature: 0,
	}

	eventCh, err := s.aiProvider.ChatStream(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("AI extract: %w", err)
	}

	var response strings.Builder
	for event := range eventCh {
		switch event.Type {
		case ai.EventTextDelta:
			response.WriteString(event.Text)
		case ai.EventError:
			return nil, fmt.Errorf("AI extract stream error: %w", event.Error)
		}
	}

	raw := stripCodeFences(response.String())

	var result ExtractFieldResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		// If JSON parsing fails, treat the entire response as the answer
		slog.Warn("failed to parse AI extract response as JSON, using raw text",
			"booking_id", bookingID, "error", err)
		return &ExtractFieldResult{Answer: raw}, nil
	}

	return &result, nil
}
