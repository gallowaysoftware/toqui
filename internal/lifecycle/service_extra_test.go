package lifecycle

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gallowaysoftware/toqui-backend/internal/chatstore"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
)

// ---------------------------------------------------------------------------
// stubQueries — fail-loud test double for lifecycleQueries
// ---------------------------------------------------------------------------
//
// Same pattern as internal/payment/, internal/subscription/,
// internal/trip/ stubQueries. Every method calls tb.Fatalf when called
// without an injected `*Fn`, so a test that forgets to configure a
// query path fails with a precise "set <fnName>" message rather than
// silently passing on a zero-value response. Lessons-learned from the
// PR #421 hardening of #418.

type stubQueries struct {
	tb testing.TB

	getAllTripIDsForUserFn         func(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error)
	deleteUserByIDFn               func(ctx context.Context, id uuid.UUID) error
	deleteTripByUserFn             func(ctx context.Context, arg dbgen.DeleteTripByUserParams) error
	getTripsToArchiveFn            func(ctx context.Context) ([]dbgen.GetTripsToArchiveRow, error)
	archiveTripFn                  func(ctx context.Context, arg dbgen.ArchiveTripParams) error
	createDeletionRequestFn        func(ctx context.Context, userID uuid.UUID) (dbgen.DeletionRequest, error)
	setDeletionRequestProcessingFn func(ctx context.Context, id uuid.UUID) error
	completeDeletionRequestFn      func(ctx context.Context, id uuid.UUID) error
	getStaleDeletionRequestsFn     func(ctx context.Context) ([]dbgen.GetStaleDeletionRequestsRow, error)
	incrementDeletionRetryCountFn  func(ctx context.Context, id uuid.UUID) error
	failDeletionRequestFn          func(ctx context.Context, id uuid.UUID) error
	getUserByIDFn                  func(ctx context.Context, id uuid.UUID) (dbgen.User, error)
	listTripsByUserFn              func(ctx context.Context, arg dbgen.ListTripsByUserParams) ([]dbgen.Trip, error)
	listItineraryItemsByTripFn     func(ctx context.Context, tripID uuid.UUID) ([]dbgen.ItineraryItem, error)
	getTripThemesFn                func(ctx context.Context, tripID uuid.UUID) ([]dbgen.GetTripThemesRow, error)
	listBookingsByUserFn           func(ctx context.Context, arg dbgen.ListBookingsByUserParams) ([]dbgen.Booking, error)
	listReferralsByUserFn          func(ctx context.Context, userID uuid.UUID) ([]dbgen.Referral, error)
	listFeedbackByUserFn           func(ctx context.Context, userID uuid.UUID) ([]dbgen.Feedback, error)
	listUserPaymentsFn             func(ctx context.Context, arg dbgen.ListUserPaymentsParams) ([]dbgen.ListUserPaymentsRow, error)
	getPreferencesFn               func(ctx context.Context, userID uuid.UUID) ([]dbgen.UserPreference, error)
	getActiveConsentsFn            func(ctx context.Context, userID uuid.UUID) ([]dbgen.UserConsent, error)
	createExportRequestFn          func(ctx context.Context, userID uuid.UUID) (dbgen.ExportRequest, error)
	completeExportRequestFn        func(ctx context.Context, arg dbgen.CompleteExportRequestParams) error

	// Captured calls.
	deleteUserByIDCalls               []uuid.UUID
	deleteTripByUserCalls             []dbgen.DeleteTripByUserParams
	archiveTripCalls                  []dbgen.ArchiveTripParams
	setDeletionRequestProcessingCalls []uuid.UUID
	completeDeletionRequestCalls      []uuid.UUID
	failDeletionRequestCalls          []uuid.UUID
	incrementDeletionRetryCalls       []uuid.UUID
	completeExportRequestCalls        []dbgen.CompleteExportRequestParams
}

func (s *stubQueries) GetAllTripIDsForUser(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	if s.getAllTripIDsForUserFn != nil {
		return s.getAllTripIDsForUserFn(ctx, userID)
	}
	s.tb.Fatalf("unexpected stubQueries.GetAllTripIDsForUser(%s) — set getAllTripIDsForUserFn", userID)
	return nil, nil
}

func (s *stubQueries) DeleteUserByID(ctx context.Context, id uuid.UUID) error {
	s.deleteUserByIDCalls = append(s.deleteUserByIDCalls, id)
	if s.deleteUserByIDFn != nil {
		return s.deleteUserByIDFn(ctx, id)
	}
	s.tb.Fatalf("unexpected stubQueries.DeleteUserByID(%s) — set deleteUserByIDFn", id)
	return nil
}

func (s *stubQueries) DeleteTripByUser(ctx context.Context, arg dbgen.DeleteTripByUserParams) error {
	s.deleteTripByUserCalls = append(s.deleteTripByUserCalls, arg)
	if s.deleteTripByUserFn != nil {
		return s.deleteTripByUserFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubQueries.DeleteTripByUser(%+v) — set deleteTripByUserFn", arg)
	return nil
}

func (s *stubQueries) GetTripsToArchive(ctx context.Context) ([]dbgen.GetTripsToArchiveRow, error) {
	if s.getTripsToArchiveFn != nil {
		return s.getTripsToArchiveFn(ctx)
	}
	s.tb.Fatalf("unexpected stubQueries.GetTripsToArchive() — set getTripsToArchiveFn")
	return nil, nil
}

func (s *stubQueries) ArchiveTrip(ctx context.Context, arg dbgen.ArchiveTripParams) error {
	s.archiveTripCalls = append(s.archiveTripCalls, arg)
	if s.archiveTripFn != nil {
		return s.archiveTripFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubQueries.ArchiveTrip(%+v) — set archiveTripFn", arg)
	return nil
}

func (s *stubQueries) CreateDeletionRequest(ctx context.Context, userID uuid.UUID) (dbgen.DeletionRequest, error) {
	if s.createDeletionRequestFn != nil {
		return s.createDeletionRequestFn(ctx, userID)
	}
	s.tb.Fatalf("unexpected stubQueries.CreateDeletionRequest(%s) — set createDeletionRequestFn", userID)
	return dbgen.DeletionRequest{}, nil
}

func (s *stubQueries) SetDeletionRequestProcessing(ctx context.Context, id uuid.UUID) error {
	s.setDeletionRequestProcessingCalls = append(s.setDeletionRequestProcessingCalls, id)
	if s.setDeletionRequestProcessingFn != nil {
		return s.setDeletionRequestProcessingFn(ctx, id)
	}
	s.tb.Fatalf("unexpected stubQueries.SetDeletionRequestProcessing(%s) — set setDeletionRequestProcessingFn", id)
	return nil
}

func (s *stubQueries) CompleteDeletionRequest(ctx context.Context, id uuid.UUID) error {
	s.completeDeletionRequestCalls = append(s.completeDeletionRequestCalls, id)
	if s.completeDeletionRequestFn != nil {
		return s.completeDeletionRequestFn(ctx, id)
	}
	s.tb.Fatalf("unexpected stubQueries.CompleteDeletionRequest(%s) — set completeDeletionRequestFn", id)
	return nil
}

func (s *stubQueries) GetStaleDeletionRequests(ctx context.Context) ([]dbgen.GetStaleDeletionRequestsRow, error) {
	if s.getStaleDeletionRequestsFn != nil {
		return s.getStaleDeletionRequestsFn(ctx)
	}
	s.tb.Fatalf("unexpected stubQueries.GetStaleDeletionRequests() — set getStaleDeletionRequestsFn")
	return nil, nil
}

func (s *stubQueries) IncrementDeletionRetryCount(ctx context.Context, id uuid.UUID) error {
	s.incrementDeletionRetryCalls = append(s.incrementDeletionRetryCalls, id)
	if s.incrementDeletionRetryCountFn != nil {
		return s.incrementDeletionRetryCountFn(ctx, id)
	}
	s.tb.Fatalf("unexpected stubQueries.IncrementDeletionRetryCount(%s) — set incrementDeletionRetryCountFn", id)
	return nil
}

func (s *stubQueries) FailDeletionRequest(ctx context.Context, id uuid.UUID) error {
	s.failDeletionRequestCalls = append(s.failDeletionRequestCalls, id)
	if s.failDeletionRequestFn != nil {
		return s.failDeletionRequestFn(ctx, id)
	}
	s.tb.Fatalf("unexpected stubQueries.FailDeletionRequest(%s) — set failDeletionRequestFn", id)
	return nil
}

func (s *stubQueries) GetUserByID(ctx context.Context, id uuid.UUID) (dbgen.User, error) {
	if s.getUserByIDFn != nil {
		return s.getUserByIDFn(ctx, id)
	}
	s.tb.Fatalf("unexpected stubQueries.GetUserByID(%s) — set getUserByIDFn", id)
	return dbgen.User{}, nil
}

func (s *stubQueries) ListTripsByUser(ctx context.Context, arg dbgen.ListTripsByUserParams) ([]dbgen.Trip, error) {
	if s.listTripsByUserFn != nil {
		return s.listTripsByUserFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubQueries.ListTripsByUser(%+v) — set listTripsByUserFn", arg)
	return nil, nil
}

func (s *stubQueries) ListItineraryItemsByTrip(ctx context.Context, tripID uuid.UUID) ([]dbgen.ItineraryItem, error) {
	if s.listItineraryItemsByTripFn != nil {
		return s.listItineraryItemsByTripFn(ctx, tripID)
	}
	s.tb.Fatalf("unexpected stubQueries.ListItineraryItemsByTrip(%s) — set listItineraryItemsByTripFn", tripID)
	return nil, nil
}

func (s *stubQueries) GetTripThemes(ctx context.Context, tripID uuid.UUID) ([]dbgen.GetTripThemesRow, error) {
	if s.getTripThemesFn != nil {
		return s.getTripThemesFn(ctx, tripID)
	}
	s.tb.Fatalf("unexpected stubQueries.GetTripThemes(%s) — set getTripThemesFn", tripID)
	return nil, nil
}

func (s *stubQueries) ListBookingsByUser(ctx context.Context, arg dbgen.ListBookingsByUserParams) ([]dbgen.Booking, error) {
	if s.listBookingsByUserFn != nil {
		return s.listBookingsByUserFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubQueries.ListBookingsByUser(%+v) — set listBookingsByUserFn", arg)
	return nil, nil
}

func (s *stubQueries) ListReferralsByUser(ctx context.Context, userID uuid.UUID) ([]dbgen.Referral, error) {
	if s.listReferralsByUserFn != nil {
		return s.listReferralsByUserFn(ctx, userID)
	}
	s.tb.Fatalf("unexpected stubQueries.ListReferralsByUser(%s) — set listReferralsByUserFn", userID)
	return nil, nil
}

func (s *stubQueries) ListFeedbackByUser(ctx context.Context, userID uuid.UUID) ([]dbgen.Feedback, error) {
	if s.listFeedbackByUserFn != nil {
		return s.listFeedbackByUserFn(ctx, userID)
	}
	s.tb.Fatalf("unexpected stubQueries.ListFeedbackByUser(%s) — set listFeedbackByUserFn", userID)
	return nil, nil
}

func (s *stubQueries) ListUserPayments(ctx context.Context, arg dbgen.ListUserPaymentsParams) ([]dbgen.ListUserPaymentsRow, error) {
	if s.listUserPaymentsFn != nil {
		return s.listUserPaymentsFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubQueries.ListUserPayments(%+v) — set listUserPaymentsFn", arg)
	return nil, nil
}

func (s *stubQueries) GetPreferences(ctx context.Context, userID uuid.UUID) ([]dbgen.UserPreference, error) {
	if s.getPreferencesFn != nil {
		return s.getPreferencesFn(ctx, userID)
	}
	s.tb.Fatalf("unexpected stubQueries.GetPreferences(%s) — set getPreferencesFn", userID)
	return nil, nil
}

func (s *stubQueries) GetActiveConsents(ctx context.Context, userID uuid.UUID) ([]dbgen.UserConsent, error) {
	if s.getActiveConsentsFn != nil {
		return s.getActiveConsentsFn(ctx, userID)
	}
	s.tb.Fatalf("unexpected stubQueries.GetActiveConsents(%s) — set getActiveConsentsFn", userID)
	return nil, nil
}

func (s *stubQueries) CreateExportRequest(ctx context.Context, userID uuid.UUID) (dbgen.ExportRequest, error) {
	if s.createExportRequestFn != nil {
		return s.createExportRequestFn(ctx, userID)
	}
	s.tb.Fatalf("unexpected stubQueries.CreateExportRequest(%s) — set createExportRequestFn", userID)
	return dbgen.ExportRequest{}, nil
}

func (s *stubQueries) CompleteExportRequest(ctx context.Context, arg dbgen.CompleteExportRequestParams) error {
	s.completeExportRequestCalls = append(s.completeExportRequestCalls, arg)
	if s.completeExportRequestFn != nil {
		return s.completeExportRequestFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubQueries.CompleteExportRequest(%+v) — set completeExportRequestFn", arg)
	return nil
}

// ---------------------------------------------------------------------------
// stubChatStore — fail-loud test double for lifecycleChatStore
// ---------------------------------------------------------------------------

type stubChatStore struct {
	tb testing.TB

	deleteAllForTripFn func(ctx context.Context, userID, tripID string) error
	setTTLFn           func(ctx context.Context, userID, tripID string, expireAt time.Time) error
	exportChatDataFn   func(ctx context.Context, userID string, tripIDs []string) (map[string][]chatstore.ExportedSession, error)

	deleteAllForTripCalls []struct{ UserID, TripID string }
	setTTLCalls           []struct {
		UserID, TripID string
		ExpireAt       time.Time
	}
}

func (s *stubChatStore) DeleteAllForTrip(ctx context.Context, userID, tripID string) error {
	s.deleteAllForTripCalls = append(s.deleteAllForTripCalls, struct{ UserID, TripID string }{userID, tripID})
	if s.deleteAllForTripFn != nil {
		return s.deleteAllForTripFn(ctx, userID, tripID)
	}
	s.tb.Fatalf("unexpected stubChatStore.DeleteAllForTrip(%s, %s) — set deleteAllForTripFn", userID, tripID)
	return nil
}

func (s *stubChatStore) SetTTL(ctx context.Context, userID, tripID string, expireAt time.Time) error {
	s.setTTLCalls = append(s.setTTLCalls, struct {
		UserID, TripID string
		ExpireAt       time.Time
	}{userID, tripID, expireAt})
	if s.setTTLFn != nil {
		return s.setTTLFn(ctx, userID, tripID, expireAt)
	}
	s.tb.Fatalf("unexpected stubChatStore.SetTTL(%s, %s, %v) — set setTTLFn", userID, tripID, expireAt)
	return nil
}

func (s *stubChatStore) ExportChatData(ctx context.Context, userID string, tripIDs []string) (map[string][]chatstore.ExportedSession, error) {
	if s.exportChatDataFn != nil {
		return s.exportChatDataFn(ctx, userID, tripIDs)
	}
	s.tb.Fatalf("unexpected stubChatStore.ExportChatData(%s, %v) — set exportChatDataFn", userID, tripIDs)
	return nil, nil
}

// newTestService builds a Service literal directly (no NewService — that
// requires a *pgxpool.Pool we don't have at unit-test time). The pool
// field is only used by code paths the unit tests don't exercise (the
// ones that need a real Postgres landed in internal/integration/).
func newTestService(q *stubQueries, c *stubChatStore) *Service {
	return &Service{
		queries:   q,
		chatStore: c,
	}
}

// ---------------------------------------------------------------------------
// DeleteUser
// ---------------------------------------------------------------------------

func TestDeleteUser_HappyPath_DeletesFirestorePerTripThenPostgres(t *testing.T) {
	userID := uuid.New()
	trip1 := uuid.New()
	trip2 := uuid.New()

	q := &stubQueries{tb: t,
		getAllTripIDsForUserFn: func(_ context.Context, id uuid.UUID) ([]uuid.UUID, error) {
			if id != userID {
				t.Errorf("expected userID=%s, got %s", userID, id)
			}
			return []uuid.UUID{trip1, trip2}, nil
		},
		deleteUserByIDFn: func(_ context.Context, _ uuid.UUID) error { return nil },
	}
	c := &stubChatStore{tb: t,
		deleteAllForTripFn: func(_ context.Context, _, _ string) error { return nil },
	}
	svc := newTestService(q, c)

	if err := svc.DeleteUser(context.Background(), userID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Each trip's Firestore data was deleted.
	if len(c.deleteAllForTripCalls) != 2 {
		t.Errorf("expected 2 Firestore deletes, got %d", len(c.deleteAllForTripCalls))
	}
	// Postgres user delete fired exactly once with the right ID.
	if len(q.deleteUserByIDCalls) != 1 || q.deleteUserByIDCalls[0] != userID {
		t.Errorf("expected single DeleteUserByID(%s), got %v", userID, q.deleteUserByIDCalls)
	}
}

func TestDeleteUser_FirestoreFailureIsNonFatal(t *testing.T) {
	// Per the function comment: "Continue — don't fail the whole
	// deletion for Firestore issues". The Postgres user delete must
	// still fire.
	userID := uuid.New()
	q := &stubQueries{tb: t,
		getAllTripIDsForUserFn: func(_ context.Context, _ uuid.UUID) ([]uuid.UUID, error) {
			return []uuid.UUID{uuid.New()}, nil
		},
		deleteUserByIDFn: func(_ context.Context, _ uuid.UUID) error { return nil },
	}
	c := &stubChatStore{tb: t,
		deleteAllForTripFn: func(_ context.Context, _, _ string) error {
			return errors.New("firestore down")
		},
	}
	svc := newTestService(q, c)

	if err := svc.DeleteUser(context.Background(), userID); err != nil {
		t.Errorf("Firestore failure must not fail the whole deletion, got %v", err)
	}
	if len(q.deleteUserByIDCalls) != 1 {
		t.Errorf("Postgres delete must still fire — got %d calls", len(q.deleteUserByIDCalls))
	}
}

func TestDeleteUser_GetTripIDsErrorPropagates(t *testing.T) {
	wantErr := errors.New("db down")
	q := &stubQueries{tb: t,
		getAllTripIDsForUserFn: func(_ context.Context, _ uuid.UUID) ([]uuid.UUID, error) {
			return nil, wantErr
		},
	}
	c := &stubChatStore{tb: t}
	svc := newTestService(q, c)

	err := svc.DeleteUser(context.Background(), uuid.New())
	if !errors.Is(err, wantErr) {
		t.Errorf("expected wantErr to wrap, got %v", err)
	}
}

func TestDeleteUser_PostgresDeleteErrorPropagates(t *testing.T) {
	wantErr := errors.New("constraint violation")
	q := &stubQueries{tb: t,
		getAllTripIDsForUserFn: func(_ context.Context, _ uuid.UUID) ([]uuid.UUID, error) {
			return nil, nil
		},
		deleteUserByIDFn: func(_ context.Context, _ uuid.UUID) error { return wantErr },
	}
	c := &stubChatStore{tb: t}
	svc := newTestService(q, c)

	err := svc.DeleteUser(context.Background(), uuid.New())
	if !errors.Is(err, wantErr) {
		t.Errorf("expected wantErr to wrap, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// DeleteTrip
// ---------------------------------------------------------------------------

func TestDeleteTrip_HappyPath(t *testing.T) {
	userID := uuid.New()
	tripID := uuid.New()

	q := &stubQueries{tb: t,
		deleteTripByUserFn: func(_ context.Context, arg dbgen.DeleteTripByUserParams) error {
			if arg.ID != tripID || arg.UserID != userID {
				t.Errorf("DeleteTripByUser: got %+v, want trip=%s user=%s", arg, tripID, userID)
			}
			return nil
		},
	}
	c := &stubChatStore{tb: t,
		deleteAllForTripFn: func(_ context.Context, _, _ string) error { return nil },
	}
	svc := newTestService(q, c)

	if err := svc.DeleteTrip(context.Background(), userID, tripID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(c.deleteAllForTripCalls) != 1 {
		t.Errorf("expected 1 Firestore delete, got %d", len(c.deleteAllForTripCalls))
	}
}

func TestDeleteTrip_FirestoreFailureNonFatal(t *testing.T) {
	q := &stubQueries{tb: t,
		deleteTripByUserFn: func(_ context.Context, _ dbgen.DeleteTripByUserParams) error { return nil },
	}
	c := &stubChatStore{tb: t,
		deleteAllForTripFn: func(_ context.Context, _, _ string) error {
			return errors.New("firestore failed")
		},
	}
	svc := newTestService(q, c)

	if err := svc.DeleteTrip(context.Background(), uuid.New(), uuid.New()); err != nil {
		t.Errorf("Firestore failure must not fail trip deletion, got %v", err)
	}
}

func TestDeleteTrip_PostgresDeleteErrorPropagates(t *testing.T) {
	wantErr := errors.New("trip not found")
	q := &stubQueries{tb: t,
		deleteTripByUserFn: func(_ context.Context, _ dbgen.DeleteTripByUserParams) error { return wantErr },
	}
	c := &stubChatStore{tb: t,
		deleteAllForTripFn: func(_ context.Context, _, _ string) error { return nil },
	}
	svc := newTestService(q, c)

	if err := svc.DeleteTrip(context.Background(), uuid.New(), uuid.New()); !errors.Is(err, wantErr) {
		t.Errorf("expected wantErr to wrap, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// ArchiveCompletedTrips — loop with continue-on-error semantics
// ---------------------------------------------------------------------------

func TestArchiveCompletedTrips_HappyPath_ArchivesAllReturned(t *testing.T) {
	t1 := dbgen.GetTripsToArchiveRow{ID: uuid.New(), UserID: uuid.New()}
	t2 := dbgen.GetTripsToArchiveRow{ID: uuid.New(), UserID: uuid.New()}

	q := &stubQueries{tb: t,
		getTripsToArchiveFn: func(_ context.Context) ([]dbgen.GetTripsToArchiveRow, error) {
			return []dbgen.GetTripsToArchiveRow{t1, t2}, nil
		},
		archiveTripFn: func(_ context.Context, _ dbgen.ArchiveTripParams) error { return nil },
	}
	c := &stubChatStore{tb: t,
		deleteAllForTripFn: func(_ context.Context, _, _ string) error { return nil },
	}
	svc := newTestService(q, c)

	count, err := svc.ArchiveCompletedTrips(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 archived, got %d", count)
	}
	if len(q.archiveTripCalls) != 2 {
		t.Errorf("expected 2 ArchiveTrip calls, got %d", len(q.archiveTripCalls))
	}
}

func TestArchiveCompletedTrips_FirestoreFailureSkipsTrip(t *testing.T) {
	// A trip whose chat purge fails is NOT archived (continue) — the
	// archived counter and ArchiveTrip call list should reflect only
	// the trips that succeeded.
	t1 := dbgen.GetTripsToArchiveRow{ID: uuid.New(), UserID: uuid.New()}
	t2 := dbgen.GetTripsToArchiveRow{ID: uuid.New(), UserID: uuid.New()}

	q := &stubQueries{tb: t,
		getTripsToArchiveFn: func(_ context.Context) ([]dbgen.GetTripsToArchiveRow, error) {
			return []dbgen.GetTripsToArchiveRow{t1, t2}, nil
		},
		archiveTripFn: func(_ context.Context, _ dbgen.ArchiveTripParams) error { return nil },
	}
	calls := 0
	c := &stubChatStore{tb: t,
		deleteAllForTripFn: func(_ context.Context, _, _ string) error {
			calls++
			if calls == 1 {
				return errors.New("firestore down for trip 1")
			}
			return nil
		},
	}
	svc := newTestService(q, c)

	count, err := svc.ArchiveCompletedTrips(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 archived (the second trip), got %d", count)
	}
	if len(q.archiveTripCalls) != 1 {
		t.Errorf("expected 1 ArchiveTrip call (skipped on Firestore failure), got %d", len(q.archiveTripCalls))
	}
}

func TestArchiveCompletedTrips_GetTripsErrorPropagates(t *testing.T) {
	wantErr := errors.New("db down")
	q := &stubQueries{tb: t,
		getTripsToArchiveFn: func(_ context.Context) ([]dbgen.GetTripsToArchiveRow, error) {
			return nil, wantErr
		},
	}
	c := &stubChatStore{tb: t}
	svc := newTestService(q, c)

	count, err := svc.ArchiveCompletedTrips(context.Background())
	if !errors.Is(err, wantErr) {
		t.Errorf("expected wantErr to wrap, got %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 archived on error, got %d", count)
	}
}

// ---------------------------------------------------------------------------
// SetChatTTL
// ---------------------------------------------------------------------------

func TestSetChatTTL_StampsExpireAtRetentionDaysOut(t *testing.T) {
	userID := uuid.New()
	tripID := uuid.New()

	c := &stubChatStore{tb: t,
		setTTLFn: func(_ context.Context, _, _ string, expireAt time.Time) error {
			// Must be ~retentionDays in the future. Allow a 1-min slack
			// to absorb test execution time.
			expected := time.Now().AddDate(0, 0, 90)
			if expireAt.Before(expected.Add(-1*time.Minute)) || expireAt.After(expected.Add(1*time.Minute)) {
				t.Errorf("expireAt off: got %v, expected ~%v", expireAt, expected)
			}
			return nil
		},
	}
	svc := newTestService(&stubQueries{tb: t}, c)

	if err := svc.SetChatTTL(context.Background(), userID, tripID, 90); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(c.setTTLCalls) != 1 {
		t.Fatalf("expected 1 SetTTL call, got %d", len(c.setTTLCalls))
	}
	got := c.setTTLCalls[0]
	if got.UserID != userID.String() || got.TripID != tripID.String() {
		t.Errorf("SetTTL got user=%s trip=%s, want %s/%s", got.UserID, got.TripID, userID, tripID)
	}
}

func TestSetChatTTL_ChatStoreErrorPropagates(t *testing.T) {
	wantErr := errors.New("ttl set failed")
	c := &stubChatStore{tb: t,
		setTTLFn: func(_ context.Context, _, _ string, _ time.Time) error { return wantErr },
	}
	svc := newTestService(&stubQueries{tb: t}, c)

	if err := svc.SetChatTTL(context.Background(), uuid.New(), uuid.New(), 90); !errors.Is(err, wantErr) {
		t.Errorf("expected wantErr to wrap, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// RetryFailedDeletions
// ---------------------------------------------------------------------------

func TestRetryFailedDeletions_RetriesAndCompletes(t *testing.T) {
	stale := dbgen.GetStaleDeletionRequestsRow{
		ID:         uuid.New(),
		UserID:     uuid.New(),
		RetryCount: 1,
	}
	q := &stubQueries{tb: t,
		getStaleDeletionRequestsFn: func(_ context.Context) ([]dbgen.GetStaleDeletionRequestsRow, error) {
			return []dbgen.GetStaleDeletionRequestsRow{stale}, nil
		},
		incrementDeletionRetryCountFn: func(_ context.Context, _ uuid.UUID) error { return nil },
		// DeleteUser branch — return no trips so we don't need
		// chatStore configured for this test.
		getAllTripIDsForUserFn:    func(_ context.Context, _ uuid.UUID) ([]uuid.UUID, error) { return nil, nil },
		deleteUserByIDFn:          func(_ context.Context, _ uuid.UUID) error { return nil },
		completeDeletionRequestFn: func(_ context.Context, _ uuid.UUID) error { return nil },
	}
	c := &stubChatStore{tb: t} // no Firestore calls expected with empty trip list
	svc := newTestService(q, c)

	retried, failed, err := svc.RetryFailedDeletions(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if retried != 1 || failed != 0 {
		t.Errorf("expected retried=1 failed=0, got retried=%d failed=%d", retried, failed)
	}
	if len(q.incrementDeletionRetryCalls) != 1 {
		t.Errorf("expected 1 IncrementDeletionRetryCount call, got %d", len(q.incrementDeletionRetryCalls))
	}
	if len(q.completeDeletionRequestCalls) != 1 {
		t.Errorf("expected 1 CompleteDeletionRequest call, got %d", len(q.completeDeletionRequestCalls))
	}
}

func TestRetryFailedDeletions_MaxedOutFails(t *testing.T) {
	// A request that's already at maxDeletionRetries (5) must NOT be
	// retried — instead it's marked as permanently failed for manual
	// intervention. Pin maxDeletionRetries since it's a behavioural
	// constant.
	stale := dbgen.GetStaleDeletionRequestsRow{
		ID:         uuid.New(),
		UserID:     uuid.New(),
		RetryCount: maxDeletionRetries, // already at the cap
	}
	q := &stubQueries{tb: t,
		getStaleDeletionRequestsFn: func(_ context.Context) ([]dbgen.GetStaleDeletionRequestsRow, error) {
			return []dbgen.GetStaleDeletionRequestsRow{stale}, nil
		},
		failDeletionRequestFn: func(_ context.Context, _ uuid.UUID) error { return nil },
	}
	svc := newTestService(q, &stubChatStore{tb: t})

	retried, failed, err := svc.RetryFailedDeletions(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if retried != 0 || failed != 1 {
		t.Errorf("expected retried=0 failed=1, got retried=%d failed=%d", retried, failed)
	}
	if len(q.failDeletionRequestCalls) != 1 {
		t.Errorf("expected 1 FailDeletionRequest call, got %d", len(q.failDeletionRequestCalls))
	}
	// Must NOT have incremented retry count or attempted deletion.
	if len(q.incrementDeletionRetryCalls) != 0 {
		t.Errorf("must not increment retry count past max")
	}
}

func TestRetryFailedDeletions_NoStaleReturnsZeros(t *testing.T) {
	q := &stubQueries{tb: t,
		getStaleDeletionRequestsFn: func(_ context.Context) ([]dbgen.GetStaleDeletionRequestsRow, error) {
			return nil, nil
		},
	}
	svc := newTestService(q, &stubChatStore{tb: t})

	retried, failed, err := svc.RetryFailedDeletions(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if retried != 0 || failed != 0 {
		t.Errorf("expected zero counts, got retried=%d failed=%d", retried, failed)
	}
}

func TestRetryFailedDeletions_GetStaleErrorPropagates(t *testing.T) {
	wantErr := errors.New("db down")
	q := &stubQueries{tb: t,
		getStaleDeletionRequestsFn: func(_ context.Context) ([]dbgen.GetStaleDeletionRequestsRow, error) {
			return nil, wantErr
		},
	}
	svc := newTestService(q, &stubChatStore{tb: t})

	_, _, err := svc.RetryFailedDeletions(context.Background())
	if !errors.Is(err, wantErr) {
		t.Errorf("expected wantErr to wrap, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// RequestDeletion — synchronous half of the spawn-goroutine path
// ---------------------------------------------------------------------------

func TestRequestDeletion_CreatesAndTransitionsToProcessing(t *testing.T) {
	// RequestDeletion creates the request row and marks it processing
	// before returning. The actual DeleteUser fires in a background
	// goroutine; we wait for it via a sync.WaitGroup-like channel
	// driven by the queries we configure.
	userID := uuid.New()
	reqID := uuid.New()

	deletionDone := make(chan struct{}, 1)

	q := &stubQueries{tb: t,
		createDeletionRequestFn: func(_ context.Context, _ uuid.UUID) (dbgen.DeletionRequest, error) {
			return dbgen.DeletionRequest{ID: reqID, UserID: userID}, nil
		},
		setDeletionRequestProcessingFn: func(_ context.Context, _ uuid.UUID) error { return nil },
		// Background goroutine paths:
		getAllTripIDsForUserFn: func(_ context.Context, _ uuid.UUID) ([]uuid.UUID, error) { return nil, nil },
		deleteUserByIDFn:       func(_ context.Context, _ uuid.UUID) error { return nil },
		completeDeletionRequestFn: func(_ context.Context, _ uuid.UUID) error {
			select {
			case deletionDone <- struct{}{}:
			default:
			}
			return nil
		},
	}
	c := &stubChatStore{tb: t} // no Firestore calls expected with empty trip list
	svc := newTestService(q, c)

	got, err := svc.RequestDeletion(context.Background(), userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != reqID {
		t.Errorf("RequestDeletion returned %s, expected %s", got, reqID)
	}
	if len(q.setDeletionRequestProcessingCalls) != 1 {
		t.Errorf("expected SetDeletionRequestProcessing fired once, got %d", len(q.setDeletionRequestProcessingCalls))
	}

	// Wait for the async goroutine to finish (or 2 seconds, whichever).
	select {
	case <-deletionDone:
	case <-time.After(2 * time.Second):
		t.Error("background DeleteUser did not complete within 2 seconds")
	}
	if len(q.completeDeletionRequestCalls) != 1 {
		t.Errorf("expected background completion to fire CompleteDeletionRequest, got %d", len(q.completeDeletionRequestCalls))
	}
}

func TestRequestDeletion_CreateRequestErrorPropagates(t *testing.T) {
	wantErr := errors.New("db down")
	q := &stubQueries{tb: t,
		createDeletionRequestFn: func(_ context.Context, _ uuid.UUID) (dbgen.DeletionRequest, error) {
			return dbgen.DeletionRequest{}, wantErr
		},
	}
	svc := newTestService(q, &stubChatStore{tb: t})

	_, err := svc.RequestDeletion(context.Background(), uuid.New())
	if !errors.Is(err, wantErr) {
		t.Errorf("expected wantErr to wrap, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// ExportUserData
// ---------------------------------------------------------------------------

func TestExportUserData_ReturnsExportShape(t *testing.T) {
	// The function does many best-effort reads, swallowing errors with
	// slog.Warn for non-essential collections (bookings, referrals,
	// payments, preferences, consents, chat). It only fails the
	// overall call if the user lookup itself fails.
	userID := uuid.New()
	user := dbgen.User{ID: userID, Email: "test@example.com"}
	trip := dbgen.Trip{ID: uuid.New(), UserID: userID, Title: "Test Trip"}

	q := &stubQueries{tb: t,
		getUserByIDFn: func(_ context.Context, _ uuid.UUID) (dbgen.User, error) { return user, nil },
		listTripsByUserFn: func(_ context.Context, _ dbgen.ListTripsByUserParams) ([]dbgen.Trip, error) {
			return []dbgen.Trip{trip}, nil
		},
		listItineraryItemsByTripFn: func(_ context.Context, _ uuid.UUID) ([]dbgen.ItineraryItem, error) { return nil, nil },
		getTripThemesFn:            func(_ context.Context, _ uuid.UUID) ([]dbgen.GetTripThemesRow, error) { return nil, nil },
		listBookingsByUserFn:       func(_ context.Context, _ dbgen.ListBookingsByUserParams) ([]dbgen.Booking, error) { return nil, nil },
		listReferralsByUserFn:      func(_ context.Context, _ uuid.UUID) ([]dbgen.Referral, error) { return nil, nil },
		listFeedbackByUserFn:       func(_ context.Context, _ uuid.UUID) ([]dbgen.Feedback, error) { return nil, nil },
		listUserPaymentsFn: func(_ context.Context, _ dbgen.ListUserPaymentsParams) ([]dbgen.ListUserPaymentsRow, error) {
			return nil, nil
		},
		getPreferencesFn:    func(_ context.Context, _ uuid.UUID) ([]dbgen.UserPreference, error) { return nil, nil },
		getActiveConsentsFn: func(_ context.Context, _ uuid.UUID) ([]dbgen.UserConsent, error) { return nil, nil },
	}
	c := &stubChatStore{tb: t,
		exportChatDataFn: func(_ context.Context, _ string, _ []string) (map[string][]chatstore.ExportedSession, error) {
			return nil, nil
		},
	}
	svc := newTestService(q, c)

	got, err := svc.ExportUserData(context.Background(), userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil export")
	}
	if got.User == nil {
		t.Error("export.User should be populated")
	}
	if len(got.Trips) != 1 {
		t.Errorf("expected 1 trip, got %d", len(got.Trips))
	}
	if got.ExportedAt == "" {
		t.Error("ExportedAt should be set")
	}
	// ChatData map should always be initialised (non-nil) so consumer
	// JSON marshalling produces "{}" rather than "null".
	if got.ChatData == nil {
		t.Error("ChatData map should be non-nil even when empty")
	}
}

func TestExportUserData_GetUserErrorIsTheOnlyFatalPath(t *testing.T) {
	// The contract: GetUserByID is the one query whose failure aborts
	// the export. All other lookups are best-effort.
	wantErr := errors.New("user not found")
	q := &stubQueries{tb: t,
		getUserByIDFn: func(_ context.Context, _ uuid.UUID) (dbgen.User, error) {
			return dbgen.User{}, wantErr
		},
	}
	svc := newTestService(q, &stubChatStore{tb: t})

	_, err := svc.ExportUserData(context.Background(), uuid.New())
	if !errors.Is(err, wantErr) {
		t.Errorf("expected wantErr to wrap, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// RequestExport
// ---------------------------------------------------------------------------

func TestRequestExport_CreatesRequestAndKicksOffBackgroundExport(t *testing.T) {
	userID := uuid.New()
	reqID := uuid.New()

	var bgWG sync.WaitGroup
	bgWG.Add(1)

	q := &stubQueries{tb: t,
		createExportRequestFn: func(_ context.Context, _ uuid.UUID) (dbgen.ExportRequest, error) {
			return dbgen.ExportRequest{ID: reqID, UserID: userID}, nil
		},
		// Background ExportUserData paths — minimal implementations.
		getUserByIDFn: func(_ context.Context, _ uuid.UUID) (dbgen.User, error) {
			return dbgen.User{ID: userID}, nil
		},
		listTripsByUserFn:     func(_ context.Context, _ dbgen.ListTripsByUserParams) ([]dbgen.Trip, error) { return nil, nil },
		listBookingsByUserFn:  func(_ context.Context, _ dbgen.ListBookingsByUserParams) ([]dbgen.Booking, error) { return nil, nil },
		listReferralsByUserFn: func(_ context.Context, _ uuid.UUID) ([]dbgen.Referral, error) { return nil, nil },
		listFeedbackByUserFn:  func(_ context.Context, _ uuid.UUID) ([]dbgen.Feedback, error) { return nil, nil },
		listUserPaymentsFn: func(_ context.Context, _ dbgen.ListUserPaymentsParams) ([]dbgen.ListUserPaymentsRow, error) {
			return nil, nil
		},
		getPreferencesFn:    func(_ context.Context, _ uuid.UUID) ([]dbgen.UserPreference, error) { return nil, nil },
		getActiveConsentsFn: func(_ context.Context, _ uuid.UUID) ([]dbgen.UserConsent, error) { return nil, nil },
		completeExportRequestFn: func(_ context.Context, arg dbgen.CompleteExportRequestParams) error {
			// Pin the wire shape: download_url is a REST endpoint
			// when no exportStore is configured.
			if !arg.DownloadUrl.Valid || arg.DownloadUrl.String == "" {
				t.Errorf("expected DownloadUrl populated, got %+v", arg.DownloadUrl)
			}
			if !arg.ExpiresAt.Valid {
				t.Errorf("expected ExpiresAt valid, got %+v", arg.ExpiresAt)
			}
			// Expiry must be ~7 days out.
			expected := time.Now().Add(7 * 24 * time.Hour)
			if arg.ExpiresAt.Time.Before(expected.Add(-1*time.Minute)) || arg.ExpiresAt.Time.After(expected.Add(1*time.Minute)) {
				t.Errorf("ExpiresAt off: got %v, expected ~%v", arg.ExpiresAt.Time, expected)
			}
			bgWG.Done()
			return nil
		},
	}
	svc := newTestService(q, &stubChatStore{tb: t})

	got, err := svc.RequestExport(context.Background(), userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != reqID {
		t.Errorf("expected reqID=%s, got %s", reqID, got)
	}

	// Wait for the background goroutine to land.
	done := make(chan struct{})
	go func() { bgWG.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("background export did not complete within 2 seconds")
	}

	if len(q.completeExportRequestCalls) != 1 {
		t.Errorf("expected 1 CompleteExportRequest call, got %d", len(q.completeExportRequestCalls))
	}
}

func TestRequestExport_CreateRequestErrorPropagates(t *testing.T) {
	wantErr := errors.New("db down")
	q := &stubQueries{tb: t,
		createExportRequestFn: func(_ context.Context, _ uuid.UUID) (dbgen.ExportRequest, error) {
			return dbgen.ExportRequest{}, wantErr
		},
	}
	svc := newTestService(q, &stubChatStore{tb: t})

	_, err := svc.RequestExport(context.Background(), uuid.New())
	if !errors.Is(err, wantErr) {
		t.Errorf("expected wantErr to wrap, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// HasLocalExport — defensive nil check
// ---------------------------------------------------------------------------

func TestHasLocalExport_ReturnsFalseWhenNoExportStore(t *testing.T) {
	// When exportStore is nil, HasLocalExport must NOT panic and must
	// return false. Pinning this because the handler downstream
	// branches on it.
	svc := newTestService(&stubQueries{tb: t}, &stubChatStore{tb: t})
	// Use a known UUID; the function should short-circuit before ever
	// touching the store.
	if svc.HasLocalExport(uuid.New()) {
		t.Error("expected false when exportStore is nil")
	}
}
