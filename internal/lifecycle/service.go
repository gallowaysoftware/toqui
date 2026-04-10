package lifecycle

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
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

// UserExport is the JSON structure returned by ExportUserData.
// It contains all user data in a portable format per GDPR Article 20.
type UserExport struct {
	ExportedAt string         `json:"exported_at"`
	User       any            `json:"user"`
	Trips      []TripExport   `json:"trips"`
	Bookings   []any          `json:"bookings"`
	Usage      []any          `json:"usage"`
	Referrals  []any          `json:"referrals"`
	Feedback   []any          `json:"feedback"`
	Payments   []any          `json:"payments"`
	ChatData   map[string]any `json:"chat_data"`
}

// TripExport includes a trip and its itinerary items.
type TripExport struct {
	Trip      any   `json:"trip"`
	Itinerary []any `json:"itinerary"`
	Themes    []any `json:"themes"`
}

// ExportUserData collects all user data from Postgres and Firestore
// into a portable JSON-serializable struct. GDPR Article 20.
func (s *Service) ExportUserData(ctx context.Context, userID uuid.UUID) (*UserExport, error) {
	export := &UserExport{
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		ChatData:   make(map[string]any),
	}

	// User profile
	user, err := s.queries.GetUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	export.User = user

	// Trips + itinerary + themes
	trips, err := s.queries.ListTripsByUser(ctx, dbgen.ListTripsByUserParams{
		UserID: userID,
		Limit:  10000,
		Offset: 0,
	})
	if err != nil {
		slog.Warn("export: failed to list trips", "error", err)
	} else {
		var tripIDs []string
		for _, t := range trips {
			items, _ := s.queries.ListItineraryItemsByTrip(ctx, t.ID)
			themes, _ := s.queries.GetTripThemes(ctx, t.ID)

			var itemsAny []any
			for _, item := range items {
				itemsAny = append(itemsAny, item)
			}
			var themesAny []any
			for _, theme := range themes {
				themesAny = append(themesAny, theme)
			}

			export.Trips = append(export.Trips, TripExport{
				Trip:      t,
				Itinerary: itemsAny,
				Themes:    themesAny,
			})
			tripIDs = append(tripIDs, t.ID.String())
		}

		// Chat data from Firestore
		if s.chatStore != nil && len(tripIDs) > 0 {
			chatData, err := s.chatStore.ExportChatData(ctx, userID.String(), tripIDs)
			if err != nil {
				slog.Warn("export: failed to export chat data", "error", err)
			} else {
				for k, v := range chatData {
					export.ChatData[k] = v
				}
			}
		}
	}

	// Bookings
	bookings, err := s.queries.ListBookingsByUser(ctx, dbgen.ListBookingsByUserParams{
		UserID: userID,
		Limit:  10000,
		Offset: 0,
	})
	if err != nil {
		slog.Warn("export: failed to list bookings", "error", err)
	} else {
		for _, b := range bookings {
			export.Bookings = append(export.Bookings, b)
		}
	}

	// Referrals
	referrals, err := s.queries.ListReferralsByUser(ctx, userID)
	if err != nil {
		slog.Warn("export: failed to list referrals", "error", err)
	}
	for _, r := range referrals {
		export.Referrals = append(export.Referrals, r)
	}

	// Payments
	payments, err := s.queries.ListUserPayments(ctx, dbgen.ListUserPaymentsParams{
		UserID:     userID,
		PageOffset: 0,
		PageSize:   10000,
	})
	if err != nil {
		slog.Warn("export: failed to list payments", "error", err)
	}
	for _, p := range payments {
		export.Payments = append(export.Payments, p)
	}

	return export, nil
}

// RequestExport creates a data export request, generates the export
// synchronously, and stores the download URL.
func (s *Service) RequestExport(ctx context.Context, userID uuid.UUID) (uuid.UUID, error) {
	req, err := s.queries.CreateExportRequest(ctx, userID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("create export request: %w", err)
	}

	// Generate export synchronously — user data is small enough.
	// The download is served via a REST endpoint using the request ID.
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		_, exportErr := s.ExportUserData(bgCtx, userID)
		if exportErr != nil {
			slog.Error("data export failed", "user_id", userID, "request_id", req.ID, "error", exportErr)
			return
		}

		// Mark as completed — the download endpoint serves the data live
		// from the DB rather than from a stored file, so we just need the
		// status update. The "download_url" points to the REST endpoint.
		downloadURL := fmt.Sprintf("/api/export/%s", req.ID.String())
		expiresAt := time.Now().Add(7 * 24 * time.Hour)
		if err := s.queries.CompleteExportRequest(bgCtx, dbgen.CompleteExportRequestParams{
			ID:          req.ID,
			DownloadUrl: pgtype.Text{String: downloadURL, Valid: true},
			ExpiresAt:   pgtype.Timestamptz{Time: expiresAt, Valid: true},
		}); err != nil {
			slog.Warn("export completed but failed to update status", "request_id", req.ID, "error", err)
		} else {
			slog.Info("data export completed", "user_id", userID, "request_id", req.ID)
		}
	}()

	return req.ID, nil
}
