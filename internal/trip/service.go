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
	"github.com/jackc/pgx/v5"
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

// ErrNotOwnerOrEditor is returned by service methods that require owner or
// editor-role collaborator authz when the caller has neither. Handlers
// should map this to connect.CodePermissionDenied (#343).
var ErrNotOwnerOrEditor = errors.New("user is not trip owner or editor-role collaborator")

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
	// Use tsvector full-text search for ASCII queries. For non-ASCII
	// (CJK, Arabic, Cyrillic, etc.), fall back to ILIKE since PostgreSQL's
	// tsvector doesn't tokenize these scripts correctly (N-23 P1).
	if isNonASCII(query) {
		trips, err := s.queries.SearchTripsByUserILIKE(ctx, dbgen.SearchTripsByUserILIKEParams{
			UserID:     userID,
			Query:      pgtype.Text{String: query, Valid: true},
			MaxResults: limit,
		})
		if err != nil {
			return nil, fmt.Errorf("search trips (ilike): %w", err)
		}
		return trips, nil
	}

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

// isNonASCII returns true if the string contains any non-ASCII characters.
func isNonASCII(s string) bool {
	for _, r := range s {
		if r > 127 {
			return true
		}
	}
	return false
}

func (s *Service) Update(ctx context.Context, userID, tripID uuid.UUID, title, description, status string, startDate, endDate *time.Time, budgetCents *int64, currency, notes, coverImageURL, timezone string) (*dbgen.Trip, error) {
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
		ID:            tripID,
		UserID:        userID,
		Title:         title,
		Description:   textFromString(description),
		Status:        status,
		StartDate:     dateFromTime(startDate),
		EndDate:       dateFromTime(endDate),
		BudgetCents:   int8FromPtr(budgetCents),
		Currency:      currency,
		Notes:         textFromString(notes),
		CoverImageUrl: coverImageURL,
		Timezone:      timezone,
	})
	if err != nil {
		return nil, fmt.Errorf("update trip: %w", err)
	}
	return &trip, nil
}

// ListTemplates returns pre-built trip templates available for cloning.
func (s *Service) ListTemplates(ctx context.Context, limit, offset int32) ([]dbgen.Trip, int64, error) {
	templates, err := s.queries.ListTripTemplates(ctx, dbgen.ListTripTemplatesParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list templates: %w", err)
	}
	count, err := s.queries.CountTripTemplates(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count templates: %w", err)
	}
	return templates, count, nil
}

// MoveItineraryItem moves a single itinerary item to a new day/position.
func (s *Service) MoveItineraryItem(ctx context.Context, userID, tripID, itemID uuid.UUID, targetDay, targetPos int) (*dbgen.ItineraryItem, error) {
	if targetPos <= 0 {
		targetPos = 1
	}
	item, err := s.queries.MoveItineraryItem(ctx, dbgen.MoveItineraryItemParams{
		DayNumber:  pgtype.Int4{Int32: int32(targetDay), Valid: true},
		OrderInDay: pgtype.Int4{Int32: int32(targetPos), Valid: true},
		ID:         itemID,
		UserID:     userID,
	})
	if err != nil {
		return nil, fmt.Errorf("move itinerary item: %w", err)
	}
	return &item, nil
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
	return s.CreateItineraryItemWithCost(ctx, tripID, dayNumber, orderInDay, itemType, title, description, nil, "")
}

// CreateItineraryItemWithCost creates an itinerary item with optional cost
// fields (estimated_cost_cents, cost_currency). When costCents is nil or
// currency is empty the item is created without cost data.
func (s *Service) CreateItineraryItemWithCost(ctx context.Context, tripID uuid.UUID, dayNumber, orderInDay int, itemType, title, description string, costCents *int64, costCurrency string) (dbgen.ItineraryItem, error) {
	item, err := s.queries.CreateItineraryItem(ctx, dbgen.CreateItineraryItemParams{
		TripID:             tripID,
		DayNumber:          int4FromInt(dayNumber),
		OrderInDay:         int4FromInt(orderInDay),
		Type:               textFromString(itemType),
		Title:              textFromString(title),
		Description:        textFromString(description),
		EstimatedCostCents: int8FromPtr(costCents),
		CostCurrency:       textFromString(costCurrency),
	})
	if err != nil {
		return dbgen.ItineraryItem{}, fmt.Errorf("create itinerary item: %w", err)
	}
	return item, nil
}

// CreateItineraryItemForOwnerOrEditor is the authz-gated variant used by
// code paths where the caller's right to edit the trip may change between
// request time and the moment of write. Chat sessions last tens of
// seconds to minutes; an editor-role collaborator revoked mid-stream
// would otherwise still land inserts via the un-gated helpers (#353).
//
// The underlying CreateItineraryItemForOwnerOrEditor SQL re-checks
// ownership on every INSERT. A predicate miss → pgx.ErrNoRows, which
// we translate into ErrNotOwnerOrEditor so handlers (and chat tools)
// can report "forbidden" with a clean sentinel rather than a raw
// pgx error (#346 pattern, #353 coverage).
func (s *Service) CreateItineraryItemForOwnerOrEditor(ctx context.Context, callerID, tripID uuid.UUID, dayNumber, orderInDay int, itemType, title, description string, costCents *int64, costCurrency string) (dbgen.ItineraryItem, error) {
	item, err := s.queries.CreateItineraryItemForOwnerOrEditor(ctx, dbgen.CreateItineraryItemForOwnerOrEditorParams{
		TripID:             tripID,
		DayNumber:          int4FromInt(dayNumber),
		OrderInDay:         int4FromInt(orderInDay),
		Type:               textFromString(itemType),
		Title:              textFromString(title),
		Description:        textFromString(description),
		EstimatedCostCents: int8FromPtr(costCents),
		CostCurrency:       textFromString(costCurrency),
		UserID:             callerID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dbgen.ItineraryItem{}, ErrNotOwnerOrEditor
		}
		return dbgen.ItineraryItem{}, fmt.Errorf("create itinerary item (owner/editor): %w", err)
	}
	return item, nil
}

// DeleteItineraryItems removes itinerary items by their IDs. Only items owned
// by the given user (via the trip's user_id) are deleted. Returns the list of
// actually-deleted IDs so the caller can report accurately, and an error if
// ALL deletions failed (partial success returns the successful IDs with nil).
//
// The underlying DeleteItineraryItem query filters on trip.user_id, so items
// owned by a different user are a silent no-op at the SQL layer. The service
// inspects RowsAffected and only appends to the "deleted" slice when a row
// was actually removed — matching DB truth instead of pgx's nil-on-zero-row
// Exec semantics (#345, same defence as #343 applied to the owner-only path).
func (s *Service) DeleteItineraryItems(ctx context.Context, userID uuid.UUID, itemIDs []uuid.UUID) ([]uuid.UUID, error) {
	var deleted []uuid.UUID
	var lastErr error
	for _, id := range itemIDs {
		rows, err := s.queries.DeleteItineraryItem(ctx, dbgen.DeleteItineraryItemParams{
			ID:     id,
			UserID: userID,
		})
		if err != nil {
			slog.Error("delete itinerary item", "item_id", id, "error", err)
			lastErr = err
			continue
		}
		// Zero rows means the item doesn't exist or belongs to a
		// different user. Either way, do not claim to have deleted it
		// — handler and AI-tool paths use the return slice to build
		// user-visible confirmations.
		if rows == 0 {
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

// DeleteItineraryItemsForOwnerOrEditor is like DeleteItineraryItems but also
// allows accepted editor-role collaborators to delete items (#263). The
// underlying query enforces authz in SQL (trip owner OR editor-role
// collaborator), so a viewer or non-collaborator sees zero rows affected.
// Callers receive only the IDs that were actually deleted — matching the
// DB truth, not the mere absence of a pgx error (#343).
func (s *Service) DeleteItineraryItemsForOwnerOrEditor(ctx context.Context, userID uuid.UUID, itemIDs []uuid.UUID) ([]uuid.UUID, error) {
	var deleted []uuid.UUID
	var lastErr error
	for _, id := range itemIDs {
		rows, err := s.queries.DeleteItineraryItemByOwnerOrEditor(ctx, dbgen.DeleteItineraryItemByOwnerOrEditorParams{
			ID:     id,
			UserID: userID,
		})
		if err != nil {
			slog.Error("delete itinerary item (owner/editor)", "item_id", id, "error", err)
			lastErr = err
			continue
		}
		// Zero rows means either the item doesn't exist or the caller is
		// not owner/editor — both should be reported as "not deleted"
		// rather than silently succeeding. Previously the query was
		// annotated :exec (no RowsAffected) and callers saw a bogus
		// success for every ID regardless of authz (#343).
		if rows == 0 {
			continue
		}
		deleted = append(deleted, id)
	}
	if len(deleted) == 0 && lastErr != nil {
		return nil, fmt.Errorf("all %d deletions failed: %w", len(itemIDs), lastErr)
	}
	return deleted, nil
}

// IsEditorCollaborator reports whether the given user is an accepted
// collaborator with editor role on the specified trip. Returns
// (false, err) on any DB error so callers can distinguish "definitely
// not an editor" from "couldn't answer the question" — an authz
// pre-check that swallows transient DB errors converts what should be
// a retryable 5xx into a deliberate-looking `PermissionDenied` (#348).
func (s *Service) IsEditorCollaborator(ctx context.Context, userID, tripID uuid.UUID) (bool, error) {
	ok, err := s.queries.IsAcceptedCollaboratorWithRole(ctx, dbgen.IsAcceptedCollaboratorWithRoleParams{
		TripID: tripID,
		UserID: pgtype.UUID{Bytes: userID, Valid: true},
		Role:   "editor",
	})
	if err != nil {
		return false, fmt.Errorf("is editor collaborator: %w", err)
	}
	return ok, nil
}

// CanEditTrip reports whether the user is the trip owner or an accepted
// editor-role collaborator. Used by handlers to gate write operations on
// itineraries and chat (#263).
//
// Returns (false, err) only on transient DB failures. A clean "no, this
// user cannot edit this trip" is (false, nil); ErrNoRows from the
// ownership probe is treated as "not the owner, try the editor path"
// not as a transient error (#348).
func (s *Service) CanEditTrip(ctx context.Context, userID, tripID uuid.UUID) (bool, error) {
	// Check owner first (fast path via GetByID).
	_, err := s.queries.GetTripByID(ctx, dbgen.GetTripByIDParams{
		ID:     tripID,
		UserID: userID,
	})
	if err == nil {
		return true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return false, fmt.Errorf("check trip ownership: %w", err)
	}
	return s.IsEditorCollaborator(ctx, userID, tripID)
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
	DayNumber          int
	OrderInDay         int
	Type               string
	Title              string
	Description        string
	DaySummary         string
	DayDate            string
	EstimatedCostCents *int64
	CostCurrency       string
}

// ReplaceItineraryForOwnerOrEditor deletes all existing itinerary items
// for a trip and inserts the provided set in one transaction. Used by
// the public TripService/UpdateItinerary RPC so clients have a non-AI
// path to manage itinerary content (Run 4 N-05 P1). Handlers flatten
// their proto Itinerary into []ReplaceItineraryItem before calling.
//
// Supersedes the older owner-only ReplaceItinerary which was deleted
// in #353 — the owner path is a proper subset of the owner-or-editor
// path, so a single entry point with layered SQL authz covers both
// cases without the risk of Delete/Insert predicates drifting apart.
//
// Also allows accepted editor-role collaborators to replace the
// itinerary (#263).
//
// Authz is enforced in three layers as defence-in-depth:
//  1. An explicit CanEditTrip pre-check before the transaction begins —
//     fails fast for obviously-unauthorised callers before allocating a
//     transaction.
//  2. The DeleteItineraryItemsByTripForOwnerOrEditor query filters in
//     SQL, so a caller who slips past the pre-check deletes nothing.
//  3. CreateItineraryItemForOwnerOrEditor filters in SQL on every insert
//     (#346). This closes a TOCTOU window: a collaborator demoted from
//     editor to viewer between the pre-check and the inserts would
//     otherwise sneak new items into someone else's trip, because the
//     delete step silently no-ops on the demoted snapshot while the old
//     un-gated CreateItineraryItem had no authz of its own. When the
//     WHERE predicate misses the INSERT matches zero rows and pgx
//     returns ErrNoRows; we translate that to ErrNotOwnerOrEditor and
//     roll back the whole transaction.
func (s *Service) ReplaceItineraryForOwnerOrEditor(ctx context.Context, userID, tripID uuid.UUID, items []ReplaceItineraryItem) error {
	canEdit, err := s.CanEditTrip(ctx, userID, tripID)
	if err != nil {
		return fmt.Errorf("check edit access: %w", err)
	}
	if !canEdit {
		return ErrNotOwnerOrEditor
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := s.queries.WithTx(tx)

	if _, err := qtx.DeleteItineraryItemsByTripForOwnerOrEditor(ctx, dbgen.DeleteItineraryItemsByTripForOwnerOrEditorParams{
		TripID: tripID,
		UserID: userID,
	}); err != nil {
		return fmt.Errorf("delete existing: %w", err)
	}

	for _, item := range items {
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
		if _, err := qtx.CreateItineraryItemForOwnerOrEditor(ctx, dbgen.CreateItineraryItemForOwnerOrEditorParams{
			TripID:             tripID,
			DayNumber:          int4FromInt(item.DayNumber),
			OrderInDay:         int4FromInt(item.OrderInDay),
			Type:               textFromString(item.Type),
			Title:              textFromString(item.Title),
			Description:        textFromString(item.Description),
			Metadata:           metadata,
			EstimatedCostCents: int8FromPtr(item.EstimatedCostCents),
			CostCurrency:       textFromString(item.CostCurrency),
			UserID:             userID,
		}); err != nil {
			// ErrNoRows means the WHERE predicate didn't match —
			// the caller was demoted (or lost ownership) between
			// the pre-check and this insert. Translate to the
			// authz sentinel so the handler maps to
			// PermissionDenied, and roll back the transaction.
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotOwnerOrEditor
			}
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

// int8FromPtr converts a *int64 to a nullable pgtype.Int8.
// A nil pointer maps to NULL (Valid=false). A non-nil pointer sets
// the value including zero (so callers can explicitly clear a budget to 0).
func int8FromPtr(p *int64) pgtype.Int8 {
	if p == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *p, Valid: true}
}
