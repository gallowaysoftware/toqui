package lifecycle

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gallowaysoftware/toqui-backend/internal/chatstore"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui-backend/internal/exportstorage"
)

// lifecycleQueries is the slice of *dbgen.Queries that Service depends
// on. Defining a small interface here lets unit tests inject a stub
// without spinning up Postgres. Mirrors the `paymentQueries` /
// `subscriptionQueries` / `tripQueries` patterns in their respective
// packages — same fail-loud test-double philosophy. *dbgen.Queries
// satisfies this interface naturally; the compile-time guard below
// catches sqlc method-signature drift.
type lifecycleQueries interface {
	GetAllTripIDsForUser(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error)
	DeleteUserByID(ctx context.Context, id uuid.UUID) error
	DeleteTripByUser(ctx context.Context, arg dbgen.DeleteTripByUserParams) error
	GetTripsToArchive(ctx context.Context) ([]dbgen.GetTripsToArchiveRow, error)
	ArchiveTrip(ctx context.Context, arg dbgen.ArchiveTripParams) error
	CreateDeletionRequest(ctx context.Context, userID uuid.UUID) (dbgen.DeletionRequest, error)
	SetDeletionRequestProcessing(ctx context.Context, id uuid.UUID) error
	CompleteDeletionRequest(ctx context.Context, id uuid.UUID) error
	GetStaleDeletionRequests(ctx context.Context) ([]dbgen.GetStaleDeletionRequestsRow, error)
	IncrementDeletionRetryCount(ctx context.Context, id uuid.UUID) error
	FailDeletionRequest(ctx context.Context, id uuid.UUID) error
	GetUserByID(ctx context.Context, id uuid.UUID) (dbgen.User, error)
	ListTripsByUser(ctx context.Context, arg dbgen.ListTripsByUserParams) ([]dbgen.Trip, error)
	ListItineraryItemsByTrip(ctx context.Context, tripID uuid.UUID) ([]dbgen.ItineraryItem, error)
	GetTripThemes(ctx context.Context, tripID uuid.UUID) ([]dbgen.GetTripThemesRow, error)
	ListBookingsByUser(ctx context.Context, arg dbgen.ListBookingsByUserParams) ([]dbgen.Booking, error)
	ListFeedbackByUser(ctx context.Context, userID uuid.UUID) ([]dbgen.Feedback, error)
	GetPreferences(ctx context.Context, userID uuid.UUID) ([]dbgen.UserPreference, error)
	GetActiveConsents(ctx context.Context, userID uuid.UUID) ([]dbgen.UserConsent, error)
	CreateExportRequest(ctx context.Context, userID uuid.UUID) (dbgen.ExportRequest, error)
	CompleteExportRequest(ctx context.Context, arg dbgen.CompleteExportRequestParams) error
}

var _ lifecycleQueries = (*dbgen.Queries)(nil)

// lifecycleChatStore is the slice of *chatstore.Store that Service
// depends on. Same rationale as lifecycleQueries — unit tests inject
// a stub instead of standing up the Firestore emulator.
type lifecycleChatStore interface {
	DeleteAllForTrip(ctx context.Context, userID, tripID string) error
	SetTTL(ctx context.Context, userID, tripID string, expireAt time.Time) error
	ExportChatData(ctx context.Context, userID string, tripIDs []string) (map[string][]chatstore.ExportedSession, error)
}

var _ lifecycleChatStore = (*chatstore.Store)(nil)

// Service handles data lifecycle operations: deletion, archival, export.
type Service struct {
	queries     lifecycleQueries
	pool        *pgxpool.Pool
	chatStore   lifecycleChatStore
	exportStore exportstorage.Store
}

func NewService(pool *pgxpool.Pool, chatStore *chatstore.Store) *Service {
	return &Service{
		queries:   dbgen.New(pool),
		pool:      pool,
		chatStore: chatStore,
	}
}

// SetExportStore configures durable storage for GDPR data exports.
// When set, exports are persisted at generation time for point-in-time
// consistency. When nil, the download endpoint regenerates data live.
func (s *Service) SetExportStore(store exportstorage.Store) {
	s.exportStore = store
}

// DeleteUser performs a full user data purge (GDPR Article 17) on the
// data-of-record stores.
//  1. Get all trip IDs for the user
//  2. Delete all Firestore chat data for each trip
//  3. Delete user from Postgres (CASCADE handles trips, bookings, itinerary, themes)
//
// Note: PostHog and Sentry deletion (the "no shadow profiles, no retained
// analytics" privacy-policy promise) is NOT yet wired up in this path.
// Two prerequisites need to land first:
//   - Frontend Sentry.setUser({id: hashedUserID}) so Sentry records are
//     keyed to a stable identifier the backend can target on delete
//     (today they're anonymous, so a delete call would 404).
//   - PostHog person-deletion via the documented two-step API
//     (resolve distinct_id → numeric person_id, then DELETE).
//
// Until that follow-up PR ships, account deletion is complete on the
// primary stores but operators must run a periodic Sentry/PostHog scrub
// out of band. Tracked at TODO(privacy-fanout).
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

	// Transition to processing so RetryFailedDeletions can detect stale entries.
	if err := s.queries.SetDeletionRequestProcessing(ctx, req.ID); err != nil {
		slog.Warn("failed to set deletion request to processing", "request_id", req.ID, "error", err)
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

// maxDeletionRetries is the maximum number of retry attempts before a deletion
// request is marked as permanently failed.
const maxDeletionRetries = 5

// RetryFailedDeletions finds deletion requests stuck in "processing" status
// for over 1 hour and retries them. After maxDeletionRetries attempts, the
// request is marked as "failed" for manual intervention.
func (s *Service) RetryFailedDeletions(ctx context.Context) (retried int, failed int, err error) {
	stale, err := s.queries.GetStaleDeletionRequests(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("get stale deletion requests: %w", err)
	}

	for _, req := range stale {
		if req.RetryCount >= maxDeletionRetries {
			if err := s.queries.FailDeletionRequest(ctx, req.ID); err != nil {
				slog.Error("failed to mark deletion request as failed",
					"request_id", req.ID,
					"error", err,
				)
			} else {
				slog.Warn("deletion request permanently failed after max retries",
					"request_id", req.ID,
					"user_id", req.UserID,
					"retry_count", req.RetryCount,
				)
				failed++
			}
			continue
		}

		// Increment retry count before attempting deletion.
		if err := s.queries.IncrementDeletionRetryCount(ctx, req.ID); err != nil {
			slog.Error("failed to increment retry count",
				"request_id", req.ID,
				"error", err,
			)
			continue
		}

		slog.Info("retrying stale deletion request",
			"request_id", req.ID,
			"user_id", req.UserID,
			"retry_count", req.RetryCount+1,
		)

		if err := s.DeleteUser(ctx, req.UserID); err != nil {
			slog.Error("deletion retry failed",
				"request_id", req.ID,
				"user_id", req.UserID,
				"retry_count", req.RetryCount+1,
				"error", err,
			)
			continue
		}

		if err := s.queries.CompleteDeletionRequest(ctx, req.ID); err != nil {
			slog.Warn("deletion retry completed but failed to update status",
				"request_id", req.ID,
				"error", err,
			)
		} else {
			slog.Info("deletion retry succeeded",
				"request_id", req.ID,
				"user_id", req.UserID,
				"retry_count", req.RetryCount+1,
			)
			retried++
		}
	}

	return retried, failed, nil
}

// UserExport is the JSON structure returned by ExportUserData.
// It contains all user data in a portable format per GDPR Article 20.
type UserExport struct {
	ExportedAt  string         `json:"exported_at"`
	User        any            `json:"user"`
	Trips       []TripExport   `json:"trips"`
	Bookings    []any          `json:"bookings"`
	Usage       []any          `json:"usage"`
	Referrals   []any          `json:"referrals"`
	Feedback    []any          `json:"feedback"`
	Payments    []any          `json:"payments"`
	Preferences []any          `json:"preferences"`
	Consents    []any          `json:"consents"`
	ChatData    map[string]any `json:"chat_data"`
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

	// Feedback (GDPR Article 20 — feedback the user submitted is their
	// own personal data and must be included in their export). Previously
	// this field was allocated on UserExport but never populated; the
	// feedback was only retrievable through the admin handlers, leaving
	// a silent gap in the Article 20 promise. See toqui-backend#438.
	feedback, err := s.queries.ListFeedbackByUser(ctx, userID)
	if err != nil {
		slog.Warn("export: failed to list feedback", "error", err)
	}
	for _, f := range feedback {
		export.Feedback = append(export.Feedback, f)
	}

	// Preferences
	preferences, err := s.queries.GetPreferences(ctx, userID)
	if err != nil {
		slog.Warn("export: failed to list preferences", "error", err)
	}
	for _, p := range preferences {
		export.Preferences = append(export.Preferences, p)
	}

	// Consents (GDPR Article 20 — must include consent records)
	consents, err := s.queries.GetActiveConsents(ctx, userID)
	if err != nil {
		slog.Warn("export: failed to list consents", "error", err)
	}
	for _, c := range consents {
		export.Consents = append(export.Consents, c)
	}

	return export, nil
}

// RequestExport creates a data export request, generates the export
// in a background goroutine, and stores the download URL on completion.
//
// When an export store is configured (GCS or local filesystem), the export
// payload is persisted at generation time for point-in-time consistency. The
// download URL is either a signed GCS URL or a local REST endpoint path.
// When no store is configured, the download endpoint regenerates data live.
func (s *Service) RequestExport(ctx context.Context, userID uuid.UUID) (uuid.UUID, error) {
	req, err := s.queries.CreateExportRequest(ctx, userID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("create export request: %w", err)
	}

	// Generate export in background — user data is small enough for a single pass.
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		exportData, exportErr := s.ExportUserData(bgCtx, userID)
		if exportErr != nil {
			slog.Error("data export failed", "user_id", userID, "request_id", req.ID, "error", exportErr)
			return
		}

		// Persist export to durable storage if configured, otherwise fall
		// back to a REST endpoint that regenerates data live.
		var downloadURL string
		if s.exportStore != nil {
			url, uploadErr := s.exportStore.Upload(bgCtx, req.ID, exportData)
			if uploadErr != nil {
				slog.Error("export upload failed", "user_id", userID, "request_id", req.ID, "error", uploadErr)
				return
			}
			downloadURL = url
		} else {
			downloadURL = fmt.Sprintf("/api/export/%s", req.ID.String())
		}

		expiresAt := time.Now().Add(7 * 24 * time.Hour)
		if err := s.queries.CompleteExportRequest(bgCtx, dbgen.CompleteExportRequestParams{
			ID:          req.ID,
			DownloadUrl: pgtype.Text{String: downloadURL, Valid: true},
			ExpiresAt:   pgtype.Timestamptz{Time: expiresAt, Valid: true},
		}); err != nil {
			slog.Warn("export completed but failed to update status", "request_id", req.ID, "error", err)
		} else {
			slog.Info("data export completed", "user_id", userID, "request_id", req.ID, "download_url", downloadURL)
		}
	}()

	return req.ID, nil
}

// HasLocalExport returns true if the export store is a local filesystem store
// and the export file exists. Used by the download handler to determine
// whether to serve a persisted local file or regenerate live.
func (s *Service) HasLocalExport(requestID uuid.UUID) bool {
	if s.exportStore == nil {
		return false
	}
	localStore, ok := s.exportStore.(*exportstorage.LocalStore)
	if !ok {
		return false
	}
	rc, err := localStore.OpenExport(requestID)
	if err != nil {
		return false
	}
	rc.Close()
	return true
}

// OpenLocalExport opens a locally persisted export file for reading.
// Returns an error if the export store is not local or the file doesn't exist.
func (s *Service) OpenLocalExport(requestID uuid.UUID) (io.ReadCloser, error) {
	if s.exportStore == nil {
		return nil, fmt.Errorf("no export store configured")
	}
	localStore, ok := s.exportStore.(*exportstorage.LocalStore)
	if !ok {
		return nil, fmt.Errorf("export store is not local")
	}
	return localStore.OpenExport(requestID)
}
