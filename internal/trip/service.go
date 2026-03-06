package trip

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
)

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

func (s *Service) SetDestination(ctx context.Context, tripID uuid.UUID, countryCode string) error {
	return s.queries.UpdateTripDestination(ctx, dbgen.UpdateTripDestinationParams{
		ID:                 tripID,
		DestinationCountry: pgtype.Text{String: countryCode, Valid: true},
	})
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

func int4FromInt(n int) pgtype.Int4 {
	if n == 0 {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(n), Valid: true}
}
