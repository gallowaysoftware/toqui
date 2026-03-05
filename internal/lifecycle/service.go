package lifecycle

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/gallowaysoftware/toqui-backend/internal/chatstore"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
)

// Service handles data lifecycle operations: deletion, archival, export.
type Service struct {
	queries   *dbgen.Queries
	pool      *pgxpool.Pool
	chatStore *chatstore.Store
}

func NewService(pool *pgxpool.Pool, chatStore *chatstore.Store) *Service {
	return &Service{
		queries:   dbgen.New(pool),
		pool:      pool,
		chatStore: chatStore,
	}
}

// DeleteUser performs a full user data purge (GDPR Article 17).
// 1. Get all trip IDs for the user
// 2. Delete all Firestore chat data for each trip
// 3. Delete user from Postgres (CASCADE handles trips, bookings, itinerary, themes)
func (s *Service) DeleteUser(ctx context.Context, userID uuid.UUID) error {
	userIDStr := userID.String()

	// Get all trip IDs before deleting from Postgres
	tripIDs, err := s.queries.GetAllTripIDsForUser(ctx, userID)
	if err != nil {
		return fmt.Errorf("get trip IDs: %w", err)
	}

	// Delete all Firestore chat data
	for _, tripID := range tripIDs {
		if err := s.chatStore.DeleteAllForTrip(ctx, userIDStr, tripID.String()); err != nil {
			log.Printf("WARNING: failed to delete Firestore chat data for trip %s: %v", tripID, err)
			// Continue — don't fail the whole deletion for Firestore issues
		}
	}

	// Delete user from Postgres — CASCADE handles all related data
	if err := s.queries.DeleteUserByID(ctx, userID); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}

	return nil
}

// DeleteTrip purges a specific trip and all its associated data.
func (s *Service) DeleteTrip(ctx context.Context, userID uuid.UUID, tripID uuid.UUID) error {
	// Delete Firestore chat data
	if err := s.chatStore.DeleteAllForTrip(ctx, userID.String(), tripID.String()); err != nil {
		log.Printf("WARNING: failed to delete Firestore chat for trip %s: %v", tripID, err)
	}

	// Delete from Postgres — CASCADE handles itinerary, bookings, themes
	if err := s.queries.DeleteTripByUser(ctx, dbgen.DeleteTripByUserParams{
		ID:     tripID,
		UserID: userID,
	}); err != nil {
		return fmt.Errorf("delete trip: %w", err)
	}

	return nil
}

// ArchiveCompletedTrips finds trips past their archive date and archives them.
// Called by a scheduled job (e.g., Cloud Scheduler hitting an internal endpoint).
func (s *Service) ArchiveCompletedTrips(ctx context.Context) (int, error) {
	trips, err := s.queries.GetTripsToArchive(ctx)
	if err != nil {
		return 0, fmt.Errorf("get trips to archive: %w", err)
	}

	archived := 0
	for _, t := range trips {
		// Purge chat messages from Firestore
		if err := s.chatStore.DeleteAllForTrip(ctx, t.UserID.String(), t.ID.String()); err != nil {
			log.Printf("WARNING: failed to purge chat for trip %s: %v", t.ID, err)
			continue
		}

		// Mark as archived in Postgres
		if err := s.queries.ArchiveTrip(ctx, dbgen.ArchiveTripParams{
			ID:     t.ID,
			UserID: t.UserID,
		}); err != nil {
			log.Printf("WARNING: failed to archive trip %s: %v", t.ID, err)
			continue
		}

		archived++
	}

	return archived, nil
}

// SetChatTTL stamps an expireAt time on all Firestore chat data for a trip.
// Call this when a trip is marked completed to start the retention countdown.
func (s *Service) SetChatTTL(ctx context.Context, userID uuid.UUID, tripID uuid.UUID, retentionDays int) error {
	expireAt := time.Now().AddDate(0, 0, retentionDays)
	if err := s.chatStore.SetTTL(ctx, userID.String(), tripID.String(), expireAt); err != nil {
		return fmt.Errorf("set chat TTL: %w", err)
	}
	return nil
}

// SetChatTTLAsync fires TTL stamping in a background goroutine.
func (s *Service) SetChatTTLAsync(userID uuid.UUID, tripID uuid.UUID, retentionDays int) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if err := s.SetChatTTL(ctx, userID, tripID, retentionDays); err != nil {
			log.Printf("WARNING: failed to set chat TTL for trip %s: %v", tripID, err)
		}
	}()
}

// RequestDeletion creates a deletion request record (for audit trail)
// and initiates the deletion process.
func (s *Service) RequestDeletion(ctx context.Context, userID uuid.UUID) (uuid.UUID, error) {
	req, err := s.queries.CreateDeletionRequest(ctx, userID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("create deletion request: %w", err)
	}

	// Perform deletion immediately (within the same request for now;
	// in production, this should be an async job for large accounts)
	if err := s.DeleteUser(ctx, userID); err != nil {
		return uuid.Nil, fmt.Errorf("execute deletion: %w", err)
	}

	if err := s.queries.CompleteDeletionRequest(ctx, req.ID); err != nil {
		log.Printf("WARNING: deletion completed but failed to update request status: %v", err)
	}

	return req.ID, nil
}

// RequestExport creates a data export request.
// The actual export is done asynchronously by a worker.
func (s *Service) RequestExport(ctx context.Context, userID uuid.UUID) (uuid.UUID, error) {
	req, err := s.queries.CreateExportRequest(ctx, userID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("create export request: %w", err)
	}

	// TODO: Queue async export job
	// For now, return the request ID — the export worker will process it

	return req.ID, nil
}
