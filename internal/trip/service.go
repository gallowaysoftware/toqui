package trip

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
)

// ErrInvalidStatusTransition is returned by Update and CreateWithStatus when
// the requested status change is not allowed by the trip state machine.
// Handlers should map this to connect.CodeFailedPrecondition so clients can
// distinguish validation failures from server errors (Run 5 R-07/N-08 P2).
var ErrInvalidStatusTransition = errors.New("invalid trip status transition")

// ErrInvalidInitialStatus is returned by CreateWithStatus when the caller
// requests an initial status that is not permitted at creation time (e.g.
// TRIP_STATUS_COMPLETED). Handlers should map this to
// connect.CodeInvalidArgument.
var ErrInvalidInitialStatus = errors.New("invalid initial trip status")

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
	if startDate != nil && endDate != nil && endDate.Before(*startDate) {
		return nil, fmt.Errorf("end_date (%s) cannot be before start_date (%s)", endDate.Format("2006-01-02"), startDate.Format("2006-01-02"))
	}
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

// CreateWithStatus is like Create but allows the caller to set the initial
// trip status (e.g. "active" for travellers who are already on the ground
// when they create the trip). When status is "" or "planning" the default
// database value is used. Other values are applied atomically in the same
// transaction as the INSERT so an invalid status or transient update
// failure cannot leave an orphaned planning-state trip behind (Run 4
// N-01 P1, addresses adversarial review WARNING #1).
func (s *Service) CreateWithStatus(ctx context.Context, userID uuid.UUID, title, description string, startDate, endDate *time.Time, status string) (*dbgen.Trip, error) {
	// Validate BEFORE doing any writes so a bad status never reaches the DB.
	if status != "" && status != "planning" && !isValidInitialStatus(status) {
		return nil, fmt.Errorf("%w: %q", ErrInvalidInitialStatus, status)
	}

	// Fast path: default status means no second write is needed.
	if status == "" || status == "planning" {
		return s.Create(ctx, userID, title, description, startDate, endDate)
	}

	// Non-default status: run INSERT + status UPDATE in one transaction so
	// a failure on the second write rolls back the first.
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := s.queries.WithTx(tx)

	trip, err := qtx.CreateTrip(ctx, dbgen.CreateTripParams{
		UserID:      userID,
		Title:       title,
		Description: textFromString(description),
		StartDate:   dateFromTime(startDate),
		EndDate:     dateFromTime(endDate),
	})
	if err != nil {
		return nil, fmt.Errorf("create trip: %w", err)
	}
	updated, err := qtx.UpdateTrip(ctx, dbgen.UpdateTripParams{
		ID:     trip.ID,
		UserID: userID,
		Status: status,
	})
	if err != nil {
		return nil, fmt.Errorf("set initial status: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}
	return &updated, nil
}

// isValidInitialStatus reports whether status is a value a client is allowed
// to specify at trip creation time. We deliberately refuse "completed" — a
// trip cannot start in the completed state without first going through
// planning or active (see the status machine in Update).
func isValidInitialStatus(status string) bool {
	switch status {
	case "planning", "active":
		return true
	default:
		return false
	}
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

// SearchByUser performs a full-text search across the user's trips, matching
// against title, description, and destination country. Results are ranked by
// relevance.
func (s *Service) SearchByUser(ctx context.Context, userID uuid.UUID, query string, limit int32) ([]dbgen.Trip, error) {
	trips, err := s.queries.SearchTripsByUser(ctx, dbgen.SearchTripsByUserParams{
		UserID:     userID,
		Query:      query,
		MaxResults: limit,
	})
	if err != nil {
		return nil, fmt.Errorf("search trips: %w", err)
	}
	return trips, nil
}

func (s *Service) Update(ctx context.Context, userID, tripID uuid.UUID, title, description, status string, startDate, endDate *time.Time) (*dbgen.Trip, error) {
	if startDate != nil && endDate != nil && endDate.Before(*startDate) {
		return nil, fmt.Errorf("end_date (%s) cannot be before start_date (%s)", endDate.Format("2006-01-02"), startDate.Format("2006-01-02"))
	}
	// Enforce the documented trip status machine: planning → active → completed
	// is the canonical forward path, planning → completed is allowed as a
	// shortcut ("this was a quick trip, mark it done"), and any transition
	// back from completed is forbidden. Run 4 N-08 found that the server
	// previously accepted arbitrary status transitions (P2).
	if status != "" {
		current, err := s.queries.GetTripByID(ctx, dbgen.GetTripByIDParams{ID: tripID, UserID: userID})
		if err != nil {
			return nil, fmt.Errorf("load trip for status check: %w", err)
		}
		if !isValidStatusTransition(current.Status, status) {
			return nil, fmt.Errorf("%w: %s → %s", ErrInvalidStatusTransition, current.Status, status)
		}
	}
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

// isValidStatusTransition reports whether moving from `current` to `next`
// is a legal trip lifecycle step. A same-status "update" is always allowed
// so that non-status UpdateTrip calls that happen to round-trip the current
// status field aren't blocked.
func isValidStatusTransition(current, next string) bool {
	if current == next {
		return true
	}
	switch current {
	case "planning":
		return next == "active" || next == "completed"
	case "active":
		return next == "completed"
	case "completed":
		// Terminal state.
		return false
	default:
		// Unknown current status — allow the update so we don't lock out
		// trips created before the state machine existed.
		return true
	}
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

// CloneTrip duplicates an existing trip owned by userID, creating a new trip
// with the same description, destination countries, and itinerary items. The
// cloned trip always starts in "planning" status. Bookings, chat history,
// share tokens, and trial/unlock state are NOT copied.
//
// If newTitle is empty it defaults to "Copy of <original title>".
func (s *Service) CloneTrip(ctx context.Context, userID, sourceTripID uuid.UUID, newTitle string) (*dbgen.Trip, error) {
	// Load source trip and verify ownership.
	source, err := s.queries.GetTripByID(ctx, dbgen.GetTripByIDParams{
		ID:     sourceTripID,
		UserID: userID,
	})
	if err != nil {
		return nil, fmt.Errorf("get source trip: %w", err)
	}

	if newTitle == "" {
		newTitle = "Copy of " + source.Title
	}

	// Run the clone in a transaction so a failure in any step rolls back.
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := s.queries.WithTx(tx)

	// Create the new trip with the same description and dates.
	cloned, err := qtx.CreateTrip(ctx, dbgen.CreateTripParams{
		UserID:      userID,
		Title:       newTitle,
		Description: source.Description,
		StartDate:   source.StartDate,
		EndDate:     source.EndDate,
	})
	if err != nil {
		return nil, fmt.Errorf("create cloned trip: %w", err)
	}

	// Copy destination countries if present.
	if len(source.DestinationCountries) > 0 {
		primary := source.DestinationCountries[0]
		_, err = qtx.UpdateTripDestinations(ctx, dbgen.UpdateTripDestinationsParams{
			ID:                   cloned.ID,
			UserID:               userID,
			DestinationCountries: source.DestinationCountries,
			PrimaryCountry:       primary,
		})
		if err != nil {
			return nil, fmt.Errorf("set cloned destinations: %w", err)
		}
	} else if source.DestinationCountry.Valid && source.DestinationCountry.String != "" {
		_, err = qtx.UpdateTripDestination(ctx, dbgen.UpdateTripDestinationParams{
			ID:                 cloned.ID,
			DestinationCountry: source.DestinationCountry,
			UserID:             userID,
		})
		if err != nil {
			return nil, fmt.Errorf("set cloned destination: %w", err)
		}
	}

	// Clone itinerary items.
	if err := qtx.CloneItineraryItems(ctx, dbgen.CloneItineraryItemsParams{
		NewTripID:    cloned.ID,
		SourceTripID: sourceTripID,
	}); err != nil {
		return nil, fmt.Errorf("clone itinerary items: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	// Re-read the trip after commit to pick up any changes from the
	// destination update (the CreateTrip return won't have them).
	final, err := s.queries.GetTripByID(ctx, dbgen.GetTripByIDParams{
		ID:     cloned.ID,
		UserID: userID,
	})
	if err != nil {
		return nil, fmt.Errorf("reload cloned trip: %w", err)
	}

	return &final, nil
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

// DeleteItineraryItems removes itinerary items by their IDs. Only items owned
// by the given user (via the trip's user_id) are deleted. Returns the list of
// actually-deleted IDs so the caller can report accurately, and an error if
// ALL deletions failed (partial success returns the successful IDs with nil).
func (s *Service) DeleteItineraryItems(ctx context.Context, userID uuid.UUID, itemIDs []uuid.UUID) ([]uuid.UUID, error) {
	var deleted []uuid.UUID
	var lastErr error
	for _, id := range itemIDs {
		err := s.queries.DeleteItineraryItem(ctx, dbgen.DeleteItineraryItemParams{
			ID:     id,
			UserID: userID,
		})
		if err != nil {
			slog.Error("delete itinerary item", "item_id", id, "error", err)
			lastErr = err
			continue
		}
		deleted = append(deleted, id)
	}
	// Return error only when ALL deletions failed.
	if len(deleted) == 0 && lastErr != nil {
		return nil, fmt.Errorf("all %d deletions failed: %w", len(itemIDs), lastErr)
	}
	return deleted, nil
}

func (s *Service) GetItinerary(ctx context.Context, tripID uuid.UUID) ([]dbgen.ItineraryItem, error) {
	items, err := s.queries.ListItineraryItemsByTrip(ctx, tripID)
	if err != nil {
		return nil, fmt.Errorf("get itinerary: %w", err)
	}
	return items, nil
}

// ReplaceItineraryItem describes a single item in a bulk itinerary rewrite.
// This is a minimal projection of the proto ItineraryItem that only carries
// the fields we actually persist through the sqlc query path.
//
// DaySummary and DayDate are day-level fields from the proto Itinerary.
// The itinerary_items table has no dedicated day-level row, so these values
// are stored in the items' metadata JSONB under the "day_summary" and
// "day_date" keys and reconstructed by itineraryToProto() at read time.
// This avoids a schema migration while making the UpdateItinerary RPC
// round-trip day-level data correctly (Run 5 R-07/N-05 P2).
type ReplaceItineraryItem struct {
	DayNumber   int
	OrderInDay  int
	Type        string
	Title       string
	Description string
	DaySummary  string
	DayDate     string
}

// ReplaceItinerary deletes all existing itinerary items for a trip and
// inserts the provided set in one transaction. Used by the public
// TripService/UpdateItinerary RPC so clients have a non-AI path to manage
// itinerary content (Run 4 N-05 P1). Handlers flatten their proto
// Itinerary into []ReplaceItineraryItem before calling.
func (s *Service) ReplaceItinerary(ctx context.Context, userID, tripID uuid.UUID, items []ReplaceItineraryItem) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := s.queries.WithTx(tx)

	if err := qtx.DeleteItineraryItemsByTrip(ctx, dbgen.DeleteItineraryItemsByTripParams{
		TripID: tripID,
		UserID: userID,
	}); err != nil {
		return fmt.Errorf("delete existing: %w", err)
	}

	for _, item := range items {
		// Encode day-level summary/date in the item metadata JSONB so
		// UpdateItinerary round-trips them without requiring a new table
		// (Run 5 R-07/N-05 P2). itineraryToProto reads them back.
		var metadata []byte
		if item.DaySummary != "" || item.DayDate != "" {
			md := make(map[string]string, 2)
			if item.DaySummary != "" {
				md["day_summary"] = item.DaySummary
			}
			if item.DayDate != "" {
				md["day_date"] = item.DayDate
			}
			if b, err := json.Marshal(md); err == nil {
				metadata = b
			}
		}
		if _, err := qtx.CreateItineraryItem(ctx, dbgen.CreateItineraryItemParams{
			TripID:      tripID,
			DayNumber:   int4FromInt(item.DayNumber),
			OrderInDay:  int4FromInt(item.OrderInDay),
			Type:        textFromString(item.Type),
			Title:       textFromString(item.Title),
			Description: textFromString(item.Description),
			Metadata:    metadata,
		}); err != nil {
			return fmt.Errorf("insert item %q: %w", item.Title, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
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
