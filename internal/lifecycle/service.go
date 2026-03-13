package lifecycle

import (
	"context"
	"fmt"
	"log/slog"
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
			slog.Warn("failed to delete Firestore chat data", "trip_id", tripID, "error", err)
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
		slog.Warn("failed to delete Firestore chat", "trip_id", tripID, "error", err)
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
			slog.Warn("failed to purge chat for trip", "trip_id", t.ID, "error", err)
			continue
		}

		// Mark as archived in Postgres
		if err := s.queries.ArchiveTrip(ctx, dbgen.ArchiveTripParams(t)); err != nil {
			slog.Warn("failed to archive trip", "trip_id", t.ID, "error", err)
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
// This intentionally uses a detached context with a 60-second timeout because
// TTL stamping must complete even after the originating request ends.
func (s *Service) SetChatTTLAsync(userID uuid.UUID, tripID uuid.UUID, retentionDays int) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if err := s.SetChatTTL(ctx, userID, tripID, retentionDays); err != nil {
			slog.Warn("failed to set chat TTL for trip", "trip_id", tripID, "error", err)
		}
	}()
}

// RequestDeletion creates a deletion request record (for audit trail)
// and launches the actual data purge asynchronously in a background goroutine
// with a 5-minute timeout. This prevents large accounts from causing HTTP
// request timeouts.
func (s *Service) RequestDeletion(ctx context.Context, userID uuid.UUID) (uuid.UUID, error) {
	req, err := s.queries.CreateDeletionRequest(ctx, userID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("create deletion request: %w", err)
	}

	// Launch deletion in a background goroutine so the HTTP response returns
	// immediately. Use a detached context with a generous timeout.
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		if err := s.DeleteUser(bgCtx, userID); err != nil {
			slog.Error("async user deletion failed",
				"user_id", userID,
				"request_id", req.ID,
				"error", err,
			)
			return
		}

		if err := s.queries.CompleteDeletionRequest(bgCtx, req.ID); err != nil {
			slog.Warn("deletion completed but failed to update request status",
				"request_id", req.ID,
				"error", err,
			)
		} else {
			slog.Info("user deletion completed", "user_id", userID, "request_id", req.ID)
		}
	}()

	return req.ID, nil
}

// RequestExport creates a data export request.
// Currently returns an error indicating the feature is not yet implemented,
// rather than returning a fake success with no actual export.
func (s *Service) RequestExport(_ context.Context, _ uuid.UUID) (uuid.UUID, error) {
	return uuid.Nil, fmt.Errorf("data export is not yet implemented")
}
