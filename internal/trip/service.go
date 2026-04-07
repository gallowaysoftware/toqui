package trip

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
)

// shareTokenAlphabet is the character set for generating share tokens.
const shareTokenAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// shareTokenLength is the length of generated share tokens.
// 22 characters from a 62-char alphabet provides ~131 bits of entropy (>128-bit minimum).
const shareTokenLength = 22

type Service struct {
	queries *dbgen.Queries
	pool    *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{
		queries: dbgen.New(pool),
		pool:    pool,
	}
}

func (s *Service) Create(ctx context.Context, userID uuid.UUID, title, description string, startDate, endDate *time.Time) (*dbgen.Trip, error) {
	trip, err := s.queries.CreateTrip(ctx, dbgen.CreateTripParams{
		UserID:      userID,
		Title:       title,
		Description: textFromString(description),
		StartDate:   dateFromTime(startDate),
		EndDate:     dateFromTime(endDate),
	})
	if err != nil {
		return nil, fmt.Errorf("create trip: %w", err)
	}
	return &trip, nil
}

func (s *Service) GetByID(ctx context.Context, userID, tripID uuid.UUID) (*dbgen.Trip, error) {
	trip, err := s.queries.GetTripByID(ctx, dbgen.GetTripByIDParams{
		ID:     tripID,
		UserID: userID,
	})
	if err != nil {
		return nil, fmt.Errorf("get trip: %w", err)
	}
	return &trip, nil
}

// GetByIDOrCollaborator returns a trip if the user owns it or is an accepted collaborator.
func (s *Service) GetByIDOrCollaborator(ctx context.Context, userID, tripID uuid.UUID) (*dbgen.Trip, error) {
	trip, err := s.queries.GetTripByIDOrCollaborator(ctx, dbgen.GetTripByIDOrCollaboratorParams{
		ID:     tripID,
		UserID: userID,
	})
	if err != nil {
		return nil, fmt.Errorf("get trip: %w", err)
	}
	return &trip, nil
}

// ListSharedTrips returns trips shared with the user as a collaborator.
func (s *Service) ListSharedTrips(ctx context.Context, userID uuid.UUID) ([]dbgen.Trip, error) {
	trips, err := s.queries.ListSharedTrips(ctx, pgtype.UUID{Bytes: userID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("list shared trips: %w", err)
	}
	return trips, nil
}

func (s *Service) ListByUser(ctx context.Context, userID uuid.UUID, status string, limit, offset int32) ([]dbgen.Trip, int64, error) {
	var trips []dbgen.Trip
	var count int64
	var err error

	if status != "" {
		trips, err = s.queries.ListTripsByUserAndStatus(ctx, dbgen.ListTripsByUserAndStatusParams{
			UserID: userID,
			Status: status,
			Limit:  limit,
			Offset: offset,
		})
		if err != nil {
			return nil, 0, fmt.Errorf("list trips: %w", err)
		}
		count, err = s.queries.CountTripsByUserAndStatus(ctx, dbgen.CountTripsByUserAndStatusParams{
			UserID: userID,
			Status: status,
		})
	} else {
		trips, err = s.queries.ListTripsByUser(ctx, dbgen.ListTripsByUserParams{
			UserID: userID,
			Limit:  limit,
			Offset: offset,
		})
		if err != nil {
			return nil, 0, fmt.Errorf("list trips: %w", err)
		}
		count, err = s.queries.CountTripsByUser(ctx, userID)
	}
	if err != nil {
		return nil, 0, fmt.Errorf("count trips: %w", err)
	}

	return trips, count, nil
}

func (s *Service) Update(ctx context.Context, userID, tripID uuid.UUID, title, description, status string, startDate, endDate *time.Time) (*dbgen.Trip, error) {
	trip, err := s.queries.UpdateTrip(ctx, dbgen.UpdateTripParams{
		ID:          tripID,
		UserID:      userID,
		Title:       title,
		Description: textFromString(description),
		Status:      status,
		StartDate:   dateFromTime(startDate),
		EndDate:     dateFromTime(endDate),
	})
	if err != nil {
		return nil, fmt.Errorf("update trip: %w", err)
	}
	return &trip, nil
}

func (s *Service) SetDestination(ctx context.Context, userID, tripID uuid.UUID, countryCode string) error {
	result, err := s.queries.UpdateTripDestination(ctx, dbgen.UpdateTripDestinationParams{
		ID:                 tripID,
		DestinationCountry: pgtype.Text{String: countryCode, Valid: true},
		UserID:             userID,
	})
	if err != nil {
		return fmt.Errorf("set destination: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("trip not found or access denied")
	}
	return nil
}

// SetDestinations updates the full set of destination countries for a trip
// (multi-country support, #133). The first entry of countryCodes is also
// stored as the primary destination_country for backward compatibility with
// single-country persona resolution. Pass a single-element slice for the
// usual single-country case.
func (s *Service) SetDestinations(ctx context.Context, userID, tripID uuid.UUID, countryCodes []string) error {
	if len(countryCodes) == 0 {
		return nil
	}
	primary := countryCodes[0]
	result, err := s.queries.UpdateTripDestinations(ctx, dbgen.UpdateTripDestinationsParams{
		ID:                   tripID,
		UserID:               userID,
		DestinationCountries: countryCodes,
		PrimaryCountry:       primary,
	})
	if err != nil {
		return fmt.Errorf("set destinations: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("trip not found or access denied")
	}
	return nil
}

func (s *Service) Delete(ctx context.Context, userID, tripID uuid.UUID) error {
	if err := s.queries.DeleteTrip(ctx, dbgen.DeleteTripParams{
		ID:     tripID,
		UserID: userID,
	}); err != nil {
		return fmt.Errorf("delete trip: %w", err)
	}
	return nil
}

func (s *Service) EnableSharing(ctx context.Context, userID, tripID uuid.UUID) (string, error) {
	token, err := generateShareToken()
	if err != nil {
		return "", fmt.Errorf("generate share token: %w", err)
	}

	_, err = s.queries.EnableTripSharing(ctx, dbgen.EnableTripSharingParams{
		ShareToken: pgtype.Text{String: token, Valid: true},
		ID:         tripID,
		UserID:     userID,
	})
	if err != nil {
		return "", fmt.Errorf("enable trip sharing: %w", err)
	}

	return token, nil
}

func (s *Service) DisableSharing(ctx context.Context, userID, tripID uuid.UUID) error {
	_, err := s.queries.DisableTripSharing(ctx, dbgen.DisableTripSharingParams{
		ID:     tripID,
		UserID: userID,
	})
	if err != nil {
		return fmt.Errorf("disable trip sharing: %w", err)
	}
	return nil
}

func (s *Service) GetByShareToken(ctx context.Context, token string) (*dbgen.Trip, error) {
	trip, err := s.queries.GetTripByShareToken(ctx, pgtype.Text{String: token, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("get trip by share token: %w", err)
	}
	return &trip, nil
}

// generateShareToken produces a cryptographically random 22-character alphanumeric string (~131 bits of entropy).
func generateShareToken() (string, error) {
	result := make([]byte, shareTokenLength)
	alphabetLen := big.NewInt(int64(len(shareTokenAlphabet)))
	for i := range result {
		n, err := rand.Int(rand.Reader, alphabetLen)
		if err != nil {
			return "", err
		}
		result[i] = shareTokenAlphabet[n.Int64()]
	}
	return string(result), nil
}

func (s *Service) CreateItineraryItem(ctx context.Context, tripID uuid.UUID, dayNumber, orderInDay int, itemType, title, description string) (dbgen.ItineraryItem, error) {
	item, err := s.queries.CreateItineraryItem(ctx, dbgen.CreateItineraryItemParams{
		TripID:      tripID,
		DayNumber:   int4FromInt(dayNumber),
		OrderInDay:  int4FromInt(orderInDay),
		Type:        textFromString(itemType),
		Title:       textFromString(title),
		Description: textFromString(description),
	})
	if err != nil {
		return dbgen.ItineraryItem{}, fmt.Errorf("create itinerary item: %w", err)
	}
	return item, nil
}

func (s *Service) GetItinerary(ctx context.Context, tripID uuid.UUID) ([]dbgen.ItineraryItem, error) {
	items, err := s.queries.ListItineraryItemsByTrip(ctx, tripID)
	if err != nil {
		return nil, fmt.Errorf("get itinerary: %w", err)
	}
	return items, nil
}

// ItineraryItemCoords holds the lat/lng coordinates for a single itinerary item.
type ItineraryItemCoords struct {
	ID        uuid.UUID
	Latitude  float64
	Longitude float64
}

// GetItineraryCoords returns lat/lng for all geocoded itinerary items in a trip.
// Items whose location is NULL (not yet geocoded) are omitted from the result.
// Uses PostGIS ST_X/ST_Y to extract coordinates from the GEOGRAPHY column.
func (s *Service) GetItineraryCoords(ctx context.Context, tripID uuid.UUID) ([]ItineraryItemCoords, error) {
	const q = `
		SELECT id,
		       ST_Y(location::geometry) AS latitude,
		       ST_X(location::geometry) AS longitude
		  FROM itinerary_items
		 WHERE trip_id = $1
		   AND location IS NOT NULL`

	rows, err := s.pool.Query(ctx, q, tripID)
	if err != nil {
		return nil, fmt.Errorf("get itinerary coords: %w", err)
	}
	defer rows.Close()

	var result []ItineraryItemCoords
	for rows.Next() {
		var c ItineraryItemCoords
		if err := rows.Scan(&c.ID, &c.Latitude, &c.Longitude); err != nil {
			return nil, fmt.Errorf("scan itinerary coord: %w", err)
		}
		result = append(result, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate itinerary coords: %w", err)
	}
	return result, nil
}

func textFromString(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

func dateFromTime(t *time.Time) pgtype.Date {
	if t == nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: *t, Valid: true}
}

// int4FromInt converts a Go int to a nullable pgtype.Int4.
// Zero maps to NULL (Valid=false) because our 1-indexed fields (day_number,
// order_in_day) treat 0 as "unset". If you need to store a literal zero,
// construct pgtype.Int4{Int32: 0, Valid: true} directly.
func int4FromInt(n int) pgtype.Int4 {
	if n == 0 {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(n), Valid: true}
}
