package booking

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
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

	booking, err := s.queries.CreateBooking(ctx, dbgen.CreateBookingParams{
		UserID:            userID,
		TripID:            tripUUID,
		Type:              parsed.Type,
		ConfirmationCode:  pgtype.Text{String: parsed.ConfirmationCode, Valid: parsed.ConfirmationCode != ""},
		Provider:          pgtype.Text{String: parsed.Provider, Valid: parsed.Provider != ""},
		Title:             parsed.Title,
		DetailsJson:       parsed.Details,
		RawSource:         pgtype.Text{String: rawText, Valid: true},
		Source:            "paste",
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
- start_time: ISO 8601 datetime if available
- end_time: ISO 8601 datetime if available
- address: the location/address if available
- departure_location: departure city/airport/station (for flights, trains, tours)
- arrival_location: arrival city/airport/station (for flights, trains, tours)
- num_guests: number of guests/passengers if available
- details: a type-specific JSON object with the following schema based on type:

flight: {"airline":"","flight_number":"","departure_airport":"","arrival_airport":"","departure_terminal":"","arrival_terminal":"","seat":"","cabin_class":"","passengers":[]}
hotel: {"hotel_name":"","check_in_date":"","check_out_date":"","room_type":"","num_guests":0,"address":"","phone":""}
car_rental: {"company":"","pickup_location":"","dropoff_location":"","pickup_time":"","dropoff_time":"","car_type":"","driver_name":""}
train: {"operator":"","train_number":"","departure_station":"","arrival_station":"","seat":"","car_number":"","class":""}
tour: {"tour_operator":"","tour_name":"","num_participants":0,"meeting_point":"","stops":[{"name":"","location":"","duration":"","notes":""}]}
activity: {"operator":"","activity_name":"","location":"","num_guests":0,"notes":""}
restaurant: {"restaurant_name":"","cuisine":"","party_size":0,"notes":""}

Only include fields that are present in the source text. Return ONLY valid JSON, no other text.`,
		Messages: []ai.Message{
			{Role: "user", Content: prompt},
		},
		MaxTokens:   2048,
		Temperature: 0,
	}

	eventCh, err := s.aiProvider.ChatStream(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("AI parse: %w", err)
	}

	var response strings.Builder
	for event := range eventCh {
		if event.Type == ai.EventTextDelta {
			response.WriteString(event.Text)
		}
	}

	var parsed ParsedBooking
	if err := json.Unmarshal([]byte(response.String()), &parsed); err != nil {
		return nil, fmt.Errorf("unmarshal AI response: %w", err)
	}

	return &parsed, nil
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
	Answer          string            `json:"answer"`
	ExtractedFields map[string]string `json:"extracted_fields,omitempty"`
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
		MaxTokens:   1024,
		Temperature: 0,
	}

	eventCh, err := s.aiProvider.ChatStream(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("AI extract: %w", err)
	}

	var response strings.Builder
	for event := range eventCh {
		if event.Type == ai.EventTextDelta {
			response.WriteString(event.Text)
		}
	}

	var result ExtractFieldResult
	if err := json.Unmarshal([]byte(response.String()), &result); err != nil {
		return nil, fmt.Errorf("unmarshal AI response: %w", err)
	}

	return &result, nil
}
