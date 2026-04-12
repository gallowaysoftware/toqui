package booking

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gallowaysoftware/toqui-backend/internal/ai"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
)

type Service struct {
	queries    *dbgen.Queries
	aiProvider ai.Provider
}

func NewService(pool *pgxpool.Pool, aiProvider ai.Provider) *Service {
	return &Service{
		queries:    dbgen.New(pool),
		aiProvider: aiProvider,
	}
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
	Details           json.RawMessage `json:"details"`
}

func (s *Service) IngestText(ctx context.Context, userID uuid.UUID, tripID string, typeHint string, rawText string) (*dbgen.Booking, error) {
	return s.ingest(ctx, userID, tripID, typeHint, rawText, "paste")
}

// IngestEmail parses raw email text with AI and creates a booking record with source="email".
func (s *Service) IngestEmail(ctx context.Context, userID uuid.UUID, tripID string, typeHint string, rawText string) (*dbgen.Booking, error) {
	return s.ingest(ctx, userID, tripID, typeHint, rawText, "email")
}

func (s *Service) ingest(ctx context.Context, userID uuid.UUID, tripID string, typeHint string, rawText string, source string) (*dbgen.Booking, error) {
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

	// Duplicate detection: if same user + same trip + same confirmation code, return existing booking.
	if parsed.ConfirmationCode != "" && tripUUID.Valid {
		existing, err := s.queries.FindBookingByConfirmationCode(ctx, dbgen.FindBookingByConfirmationCodeParams{
			UserID:           userID,
			TripID:           tripUUID,
			ConfirmationCode: pgtype.Text{String: parsed.ConfirmationCode, Valid: true},
		})
		if err == nil {
			slog.InfoContext(ctx, "duplicate booking detected, returning existing",
				"booking_id", existing.ID,
			)
			return &existing, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("check duplicate booking: %w", err)
		}
	}

	booking, err := s.queries.CreateBooking(ctx, dbgen.CreateBookingParams{
		UserID:            userID,
		TripID:            tripUUID,
		Type:              parsed.Type,
		ConfirmationCode:  pgtype.Text{String: parsed.ConfirmationCode, Valid: parsed.ConfirmationCode != ""},
		Provider:          pgtype.Text{String: parsed.Provider, Valid: parsed.Provider != ""},
		Title:             parsed.Title,
		DetailsJson:       parsed.Details,
		RawSource:         pgtype.Text{String: rawText, Valid: true},
		Source:            source,
		DepartureLocation: pgtype.Text{String: parsed.DepartureLocation, Valid: parsed.DepartureLocation != ""},
		ArrivalLocation:   pgtype.Text{String: parsed.ArrivalLocation, Valid: parsed.ArrivalLocation != ""},
		NumGuests:         pgtype.Int4{Int32: parsed.NumGuests, Valid: parsed.NumGuests > 0},
	})
	if err != nil {
		return nil, fmt.Errorf("create booking: %w", err)
	}

	return &booking, nil
}

func (s *Service) parseWithAI(ctx context.Context, rawText string, typeHint string) (*ParsedBooking, error) {
	prompt := rawText
	if typeHint != "" {
		prompt = fmt.Sprintf("Type hint: %s\n\n%s", typeHint, rawText)
	}

	req := &ai.ChatRequest{
		SystemPrompt: `You are a booking confirmation parser. Extract structured booking information from the text provided.
Return a JSON object with these fields:
- type: one of "flight", "hotel", "car_rental", "train", "tour", "activity", "restaurant", "other"
- confirmation_code: the booking/confirmation number
- provider: the company name (airline, hotel chain, etc.)
- title: a brief description of the booking
- start_time: ISO 8601 datetime if available. For flights, use the OUTBOUND departure time. For hotels, use check-in date+time. For tours/activities, use the start time. ALWAYS populate this if any time info is present.
- end_time: ISO 8601 datetime if available. For flights, use the OUTBOUND arrival time. For hotels, use check-out date+time. For tours, use the end time.
- address: the location/address if available
- departure_location: departure city/airport/station (for flights, trains, tours). For round-trip flights, use the OUTBOUND leg's departure.
- arrival_location: arrival city/airport/station (for flights, trains, tours). For round-trip flights, use the OUTBOUND leg's arrival (the destination), NOT the return leg's arrival.
- num_guests: number of guests/passengers if available
- details: a type-specific JSON object with the following schema based on type:

flight: {"airline":"","flight_number":"","departure_airport":"","arrival_airport":"","departure_terminal":"","arrival_terminal":"","seat":"","cabin_class":"","passengers":[],"legs":[{"flight_number":"","airline":"","departure_airport":"","arrival_airport":"","departure_time":"","arrival_time":"","cabin":""}]}
  For multi-segment, connecting, or round-trip flights, populate the "legs" array with one entry per flight segment (outbound leg first, then connecting segments, then return leg). Always keep the top-level flight fields (airline, flight_number, departure_airport, arrival_airport, etc.) populated with the FIRST outbound leg's values for backward compatibility. For single non-stop one-way flights, omit the "legs" array.
hotel: {"hotel_name":"","check_in_date":"","check_out_date":"","room_type":"","num_guests":0,"address":"","phone":""}
  For multi-property bookings (hostels, chains): use a "properties" array instead: {"properties":[{"hotel_name":"","address":"","check_in_date":"","check_out_date":"","room_type":"","nights":0,"rate_per_night":0}]}
car_rental: {"company":"","pickup_location":"","dropoff_location":"","pickup_time":"","dropoff_time":"","car_type":"","driver_name":""}
train: {"operator":"","train_number":"","departure_station":"","arrival_station":"","seat":"","car_number":"","class":""}
tour: {"tour_operator":"","tour_name":"","num_participants":0,"meeting_point":"","date":"YYYY-MM-DD","start_time":"HH:MM","duration":"","guide_name":"","price":"","stops":[{"name":"","location":"","duration":"","notes":""}]}
activity: {"operator":"","activity_name":"","location":"","num_guests":0,"date":"YYYY-MM-DD","start_time":"HH:MM","duration":"","guide_name":"","price":"","notes":""}
restaurant: {"restaurant_name":"","cuisine":"","party_size":0,"notes":""}

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
	"flight":     true,
	"hotel":      true,
	"car_rental": true,
	"train":      true,
	"tour":       true,
	"activity":   true,
	"restaurant": true,
	"ferry":      true,
	"bus":        true,
	"cruise":     true,
	"transfer":   true,
	"other":      true,
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

func (s *Service) Delete(ctx context.Context, userID, bookingID uuid.UUID) error {
	return s.queries.DeleteBooking(ctx, dbgen.DeleteBookingParams{
		ID:     bookingID,
		UserID: userID,
	})
}

func (s *Service) LinkToTrip(ctx context.Context, userID, bookingID, tripID uuid.UUID) (*dbgen.Booking, error) {
	booking, err := s.queries.LinkBookingToTrip(ctx, dbgen.LinkBookingToTripParams{
		ID:     bookingID,
		TripID: pgtype.UUID{Bytes: tripID, Valid: true},
		UserID: userID,
	})
	if err != nil {
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
