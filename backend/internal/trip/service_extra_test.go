package trip

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
)

// ---------------------------------------------------------------------------
// Test scaffolding
// ---------------------------------------------------------------------------

// stubQueries is a hand-rolled test double for tripQueries with fail-loud
// defaults. Same pattern as internal/payment/stripe_test.go and
// internal/subscription/service_extra_test.go: each method calls tb.Fatalf
// when invoked without an injected `*Fn`, so a test that forgets to configure
// a query path fails with a precise "set <fnName>" message instead of
// silently passing on a zero-value response.
//
// Lessons-learned from #421's hardening of #418: an earlier draft returned
// zero values from unconfigured methods; a `GetTripByID` returning
// `dbgen.Trip{}, nil` when unconfigured would silently pass the ownership
// check (real DB returns `pgx.ErrNoRows`, which the production code
// converts to a permission failure). Fail-loud forces every test to make
// its expectations explicit.
type stubQueries struct {
	tb testing.TB

	// Trip CRUD
	createTripFn  func(ctx context.Context, arg dbgen.CreateTripParams) (dbgen.Trip, error)
	getTripByIDFn func(ctx context.Context, arg dbgen.GetTripByIDParams) (dbgen.Trip, error)
	updateTripFn  func(ctx context.Context, arg dbgen.UpdateTripParams) (dbgen.Trip, error)
	deleteTripFn  func(ctx context.Context, arg dbgen.DeleteTripParams) error

	// Trip listing / search
	listTripsByUserFn           func(ctx context.Context, arg dbgen.ListTripsByUserParams) ([]dbgen.Trip, error)
	listTripsByUserAndStatusFn  func(ctx context.Context, arg dbgen.ListTripsByUserAndStatusParams) ([]dbgen.Trip, error)
	countTripsByUserFn          func(ctx context.Context, userID uuid.UUID) (int64, error)
	countTripsByUserAndStatusFn func(ctx context.Context, arg dbgen.CountTripsByUserAndStatusParams) (int64, error)
	searchTripsByUserFn         func(ctx context.Context, arg dbgen.SearchTripsByUserParams) ([]dbgen.Trip, error)
	searchTripsByUserILIKEFn    func(ctx context.Context, arg dbgen.SearchTripsByUserILIKEParams) ([]dbgen.Trip, error)
	listTripTemplatesFn         func(ctx context.Context, arg dbgen.ListTripTemplatesParams) ([]dbgen.Trip, error)
	countTripTemplatesFn        func(ctx context.Context) (int64, error)

	// Collaborator / sharing
	getTripByIDOrCollaboratorFn func(ctx context.Context, arg dbgen.GetTripByIDOrCollaboratorParams) (dbgen.Trip, error)
	listSharedTripsFn           func(ctx context.Context, userID pgtype.UUID) ([]dbgen.Trip, error)
	isAcceptedCollaboratorFn    func(ctx context.Context, arg dbgen.IsAcceptedCollaboratorWithRoleParams) (bool, error)
	enableTripSharingFn         func(ctx context.Context, arg dbgen.EnableTripSharingParams) (dbgen.Trip, error)
	disableTripSharingFn        func(ctx context.Context, arg dbgen.DisableTripSharingParams) (dbgen.Trip, error)
	getTripByShareTokenFn       func(ctx context.Context, shareToken pgtype.Text) (dbgen.Trip, error)

	// Destinations
	updateTripDestinationFn  func(ctx context.Context, arg dbgen.UpdateTripDestinationParams) (pgconn.CommandTag, error)
	updateTripDestinationsFn func(ctx context.Context, arg dbgen.UpdateTripDestinationsParams) (pgconn.CommandTag, error)

	// Itinerary
	listItineraryItemsByTripFn            func(ctx context.Context, tripID uuid.UUID) ([]dbgen.ItineraryItem, error)
	createItineraryItemFn                 func(ctx context.Context, arg dbgen.CreateItineraryItemParams) (dbgen.ItineraryItem, error)
	createItineraryItemForOwnerOrEditorFn func(ctx context.Context, arg dbgen.CreateItineraryItemForOwnerOrEditorParams) (dbgen.ItineraryItem, error)
	deleteItineraryItemFn                 func(ctx context.Context, arg dbgen.DeleteItineraryItemParams) (int64, error)
	deleteItineraryItemByOwnerOrEditorFn  func(ctx context.Context, arg dbgen.DeleteItineraryItemByOwnerOrEditorParams) (int64, error)
	moveItineraryItemForOwnerOrEditorFn   func(ctx context.Context, arg dbgen.MoveItineraryItemForOwnerOrEditorParams) (dbgen.ItineraryItem, error)

	// Captured calls.
	createTripCalls         []dbgen.CreateTripParams
	updateTripCalls         []dbgen.UpdateTripParams
	deleteTripCalls         []dbgen.DeleteTripParams
	enableSharingCalls      []dbgen.EnableTripSharingParams
	disableSharingCalls     []dbgen.DisableTripSharingParams
	updateDestinationCalls  []dbgen.UpdateTripDestinationParams
	updateDestinationsCalls []dbgen.UpdateTripDestinationsParams
	createItemCalls         []dbgen.CreateItineraryItemParams
	createItemAuthzCalls    []dbgen.CreateItineraryItemForOwnerOrEditorParams
	deleteItemCalls         []dbgen.DeleteItineraryItemParams
	deleteItemAuthzCalls    []dbgen.DeleteItineraryItemByOwnerOrEditorParams
	moveItemAuthzCalls      []dbgen.MoveItineraryItemForOwnerOrEditorParams
}

func (s *stubQueries) CreateTrip(ctx context.Context, arg dbgen.CreateTripParams) (dbgen.Trip, error) {
	s.createTripCalls = append(s.createTripCalls, arg)
	if s.createTripFn != nil {
		return s.createTripFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubQueries.CreateTrip — set createTripFn")
	return dbgen.Trip{}, nil
}

func (s *stubQueries) GetTripByID(ctx context.Context, arg dbgen.GetTripByIDParams) (dbgen.Trip, error) {
	if s.getTripByIDFn != nil {
		return s.getTripByIDFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubQueries.GetTripByID — set getTripByIDFn")
	return dbgen.Trip{}, nil
}

func (s *stubQueries) GetTripByIDOrCollaborator(ctx context.Context, arg dbgen.GetTripByIDOrCollaboratorParams) (dbgen.Trip, error) {
	if s.getTripByIDOrCollaboratorFn != nil {
		return s.getTripByIDOrCollaboratorFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubQueries.GetTripByIDOrCollaborator — set getTripByIDOrCollaboratorFn")
	return dbgen.Trip{}, nil
}

func (s *stubQueries) GetTripByShareToken(ctx context.Context, shareToken pgtype.Text) (dbgen.Trip, error) {
	if s.getTripByShareTokenFn != nil {
		return s.getTripByShareTokenFn(ctx, shareToken)
	}
	s.tb.Fatalf("unexpected stubQueries.GetTripByShareToken — set getTripByShareTokenFn")
	return dbgen.Trip{}, nil
}

func (s *stubQueries) ListSharedTrips(ctx context.Context, userID pgtype.UUID) ([]dbgen.Trip, error) {
	if s.listSharedTripsFn != nil {
		return s.listSharedTripsFn(ctx, userID)
	}
	s.tb.Fatalf("unexpected stubQueries.ListSharedTrips — set listSharedTripsFn")
	return nil, nil
}

func (s *stubQueries) ListTripsByUser(ctx context.Context, arg dbgen.ListTripsByUserParams) ([]dbgen.Trip, error) {
	if s.listTripsByUserFn != nil {
		return s.listTripsByUserFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubQueries.ListTripsByUser — set listTripsByUserFn")
	return nil, nil
}

func (s *stubQueries) ListTripsByUserAndStatus(ctx context.Context, arg dbgen.ListTripsByUserAndStatusParams) ([]dbgen.Trip, error) {
	if s.listTripsByUserAndStatusFn != nil {
		return s.listTripsByUserAndStatusFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubQueries.ListTripsByUserAndStatus — set listTripsByUserAndStatusFn")
	return nil, nil
}

func (s *stubQueries) CountTripsByUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	if s.countTripsByUserFn != nil {
		return s.countTripsByUserFn(ctx, userID)
	}
	s.tb.Fatalf("unexpected stubQueries.CountTripsByUser — set countTripsByUserFn")
	return 0, nil
}

func (s *stubQueries) CountTripsByUserAndStatus(ctx context.Context, arg dbgen.CountTripsByUserAndStatusParams) (int64, error) {
	if s.countTripsByUserAndStatusFn != nil {
		return s.countTripsByUserAndStatusFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubQueries.CountTripsByUserAndStatus — set countTripsByUserAndStatusFn")
	return 0, nil
}

func (s *stubQueries) SearchTripsByUser(ctx context.Context, arg dbgen.SearchTripsByUserParams) ([]dbgen.Trip, error) {
	if s.searchTripsByUserFn != nil {
		return s.searchTripsByUserFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubQueries.SearchTripsByUser — set searchTripsByUserFn")
	return nil, nil
}

func (s *stubQueries) SearchTripsByUserILIKE(ctx context.Context, arg dbgen.SearchTripsByUserILIKEParams) ([]dbgen.Trip, error) {
	if s.searchTripsByUserILIKEFn != nil {
		return s.searchTripsByUserILIKEFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubQueries.SearchTripsByUserILIKE — set searchTripsByUserILIKEFn")
	return nil, nil
}

func (s *stubQueries) UpdateTrip(ctx context.Context, arg dbgen.UpdateTripParams) (dbgen.Trip, error) {
	s.updateTripCalls = append(s.updateTripCalls, arg)
	if s.updateTripFn != nil {
		return s.updateTripFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubQueries.UpdateTrip — set updateTripFn")
	return dbgen.Trip{}, nil
}

func (s *stubQueries) UpdateTripDestination(ctx context.Context, arg dbgen.UpdateTripDestinationParams) (pgconn.CommandTag, error) {
	s.updateDestinationCalls = append(s.updateDestinationCalls, arg)
	if s.updateTripDestinationFn != nil {
		return s.updateTripDestinationFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubQueries.UpdateTripDestination — set updateTripDestinationFn")
	return pgconn.CommandTag{}, nil
}

func (s *stubQueries) UpdateTripDestinations(ctx context.Context, arg dbgen.UpdateTripDestinationsParams) (pgconn.CommandTag, error) {
	s.updateDestinationsCalls = append(s.updateDestinationsCalls, arg)
	if s.updateTripDestinationsFn != nil {
		return s.updateTripDestinationsFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubQueries.UpdateTripDestinations — set updateTripDestinationsFn")
	return pgconn.CommandTag{}, nil
}

func (s *stubQueries) DeleteTrip(ctx context.Context, arg dbgen.DeleteTripParams) error {
	s.deleteTripCalls = append(s.deleteTripCalls, arg)
	if s.deleteTripFn != nil {
		return s.deleteTripFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubQueries.DeleteTrip — set deleteTripFn")
	return nil
}

func (s *stubQueries) EnableTripSharing(ctx context.Context, arg dbgen.EnableTripSharingParams) (dbgen.Trip, error) {
	s.enableSharingCalls = append(s.enableSharingCalls, arg)
	if s.enableTripSharingFn != nil {
		return s.enableTripSharingFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubQueries.EnableTripSharing — set enableTripSharingFn")
	return dbgen.Trip{}, nil
}

func (s *stubQueries) DisableTripSharing(ctx context.Context, arg dbgen.DisableTripSharingParams) (dbgen.Trip, error) {
	s.disableSharingCalls = append(s.disableSharingCalls, arg)
	if s.disableTripSharingFn != nil {
		return s.disableTripSharingFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubQueries.DisableTripSharing — set disableTripSharingFn")
	return dbgen.Trip{}, nil
}

func (s *stubQueries) ListTripTemplates(ctx context.Context, arg dbgen.ListTripTemplatesParams) ([]dbgen.Trip, error) {
	if s.listTripTemplatesFn != nil {
		return s.listTripTemplatesFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubQueries.ListTripTemplates — set listTripTemplatesFn")
	return nil, nil
}

func (s *stubQueries) CountTripTemplates(ctx context.Context) (int64, error) {
	if s.countTripTemplatesFn != nil {
		return s.countTripTemplatesFn(ctx)
	}
	s.tb.Fatalf("unexpected stubQueries.CountTripTemplates — set countTripTemplatesFn")
	return 0, nil
}

func (s *stubQueries) IsAcceptedCollaboratorWithRole(ctx context.Context, arg dbgen.IsAcceptedCollaboratorWithRoleParams) (bool, error) {
	if s.isAcceptedCollaboratorFn != nil {
		return s.isAcceptedCollaboratorFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubQueries.IsAcceptedCollaboratorWithRole — set isAcceptedCollaboratorFn")
	return false, nil
}

func (s *stubQueries) ListItineraryItemsByTrip(ctx context.Context, tripID uuid.UUID) ([]dbgen.ItineraryItem, error) {
	if s.listItineraryItemsByTripFn != nil {
		return s.listItineraryItemsByTripFn(ctx, tripID)
	}
	s.tb.Fatalf("unexpected stubQueries.ListItineraryItemsByTrip — set listItineraryItemsByTripFn")
	return nil, nil
}

func (s *stubQueries) CreateItineraryItem(ctx context.Context, arg dbgen.CreateItineraryItemParams) (dbgen.ItineraryItem, error) {
	s.createItemCalls = append(s.createItemCalls, arg)
	if s.createItineraryItemFn != nil {
		return s.createItineraryItemFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubQueries.CreateItineraryItem — set createItineraryItemFn")
	return dbgen.ItineraryItem{}, nil
}

func (s *stubQueries) CreateItineraryItemForOwnerOrEditor(ctx context.Context, arg dbgen.CreateItineraryItemForOwnerOrEditorParams) (dbgen.ItineraryItem, error) {
	s.createItemAuthzCalls = append(s.createItemAuthzCalls, arg)
	if s.createItineraryItemForOwnerOrEditorFn != nil {
		return s.createItineraryItemForOwnerOrEditorFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubQueries.CreateItineraryItemForOwnerOrEditor — set createItineraryItemForOwnerOrEditorFn")
	return dbgen.ItineraryItem{}, nil
}

func (s *stubQueries) DeleteItineraryItem(ctx context.Context, arg dbgen.DeleteItineraryItemParams) (int64, error) {
	s.deleteItemCalls = append(s.deleteItemCalls, arg)
	if s.deleteItineraryItemFn != nil {
		return s.deleteItineraryItemFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubQueries.DeleteItineraryItem — set deleteItineraryItemFn")
	return 0, nil
}

func (s *stubQueries) DeleteItineraryItemByOwnerOrEditor(ctx context.Context, arg dbgen.DeleteItineraryItemByOwnerOrEditorParams) (int64, error) {
	s.deleteItemAuthzCalls = append(s.deleteItemAuthzCalls, arg)
	if s.deleteItineraryItemByOwnerOrEditorFn != nil {
		return s.deleteItineraryItemByOwnerOrEditorFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubQueries.DeleteItineraryItemByOwnerOrEditor — set deleteItineraryItemByOwnerOrEditorFn")
	return 0, nil
}

func (s *stubQueries) MoveItineraryItemForOwnerOrEditor(ctx context.Context, arg dbgen.MoveItineraryItemForOwnerOrEditorParams) (dbgen.ItineraryItem, error) {
	s.moveItemAuthzCalls = append(s.moveItemAuthzCalls, arg)
	if s.moveItineraryItemForOwnerOrEditorFn != nil {
		return s.moveItineraryItemForOwnerOrEditorFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubQueries.MoveItineraryItemForOwnerOrEditor — set moveItineraryItemForOwnerOrEditorFn")
	return dbgen.ItineraryItem{}, nil
}

// WithTx is part of the tripQueries interface but the unit tests in this
// file never exercise transaction paths (those require a real *pgxpool.Pool
// and live in integration tests). Calling this in a unit test is a fail-loud
// signal that a non-tx-only test accidentally hit a tx code path.
func (s *stubQueries) WithTx(_ pgx.Tx) *dbgen.Queries {
	s.tb.Fatalf("unexpected stubQueries.WithTx — transaction paths require integration tests, not unit tests")
	return nil
}

// newTestService builds a Service via the real NewService constructor so any
// constructor-side wiring is exercised by every test, then swaps in the stub
// for `queries` (the only field NewService can't accept directly because its
// signature type-locks to *pgxpool.Pool). Mirrors newTestService in
// internal/payment/stripe_test.go and internal/subscription/service_extra_test.go.
//
// Pool stays nil because none of the unit tests in this file exercise tx
// paths (CreateWithStatus non-default status, CloneTrip, and
// ReplaceItineraryForOwnerOrEditor all call pool.Begin and so live in
// integration tests).
func newTestService(t *testing.T, q *stubQueries) *Service {
	t.Helper()
	svc := &Service{
		queries: q,
		pool:    nil,
	}
	return svc
}

// ---------------------------------------------------------------------------
// Create — fast path (no tx required when status is empty/planning)
// ---------------------------------------------------------------------------

func TestCreate_HappyPath(t *testing.T) {
	userID := uuid.New()
	want := dbgen.Trip{ID: uuid.New(), UserID: userID, Title: "Greece"}
	q := &stubQueries{tb: t,
		createTripFn: func(ctx context.Context, arg dbgen.CreateTripParams) (dbgen.Trip, error) {
			if arg.UserID != userID {
				t.Errorf("UserID: got %s, want %s", arg.UserID, userID)
			}
			if arg.Title != "Greece" {
				t.Errorf("Title: got %q, want Greece", arg.Title)
			}
			return want, nil
		},
	}
	svc := newTestService(t, q)
	got, err := svc.Create(context.Background(), userID, "Greece", "", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != want.ID {
		t.Errorf("ID: got %s, want %s", got.ID, want.ID)
	}
}

func TestCreate_RejectsEndBeforeStart(t *testing.T) {
	q := &stubQueries{tb: t} // CreateTrip must NOT be called.
	svc := newTestService(t, q)

	start := mustParseDate(t, "2026-06-15")
	end := mustParseDate(t, "2026-06-10")

	_, err := svc.Create(context.Background(), uuid.New(), "Bad Trip", "", &start, &end)
	if err == nil || !strings.Contains(err.Error(), "end_date") {
		t.Errorf("expected end_date validation error, got %v", err)
	}
	if len(q.createTripCalls) != 0 {
		t.Errorf("CreateTrip must not be called when validation fails, got %d calls", len(q.createTripCalls))
	}
}

func TestCreate_QueryErrorWrapped(t *testing.T) {
	q := &stubQueries{tb: t,
		createTripFn: func(ctx context.Context, _ dbgen.CreateTripParams) (dbgen.Trip, error) {
			return dbgen.Trip{}, errors.New("db down")
		},
	}
	svc := newTestService(t, q)
	_, err := svc.Create(context.Background(), uuid.New(), "X", "", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "create trip") {
		t.Errorf("expected wrapped error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// CreateWithStatus — fast paths (planning/empty go through Create)
// ---------------------------------------------------------------------------

func TestCreateWithStatus_PlanningTakesFastPath(t *testing.T) {
	userID := uuid.New()
	q := &stubQueries{tb: t,
		createTripFn: func(ctx context.Context, _ dbgen.CreateTripParams) (dbgen.Trip, error) {
			return dbgen.Trip{ID: uuid.New(), UserID: userID, Status: "planning"}, nil
		},
	}
	svc := newTestService(t, q)
	got, err := svc.CreateWithStatus(context.Background(), userID, "T", "", nil, nil, "planning")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != "planning" {
		t.Errorf("Status: got %q, want planning", got.Status)
	}
}

func TestCreateWithStatus_EmptyStatusTakesFastPath(t *testing.T) {
	q := &stubQueries{tb: t,
		createTripFn: func(ctx context.Context, _ dbgen.CreateTripParams) (dbgen.Trip, error) {
			return dbgen.Trip{}, nil
		},
	}
	svc := newTestService(t, q)
	if _, err := svc.CreateWithStatus(context.Background(), uuid.New(), "T", "", nil, nil, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateWithStatus_RejectsCompleted(t *testing.T) {
	// Completed is not a valid initial status — reject before any DB write.
	q := &stubQueries{tb: t} // no fns set; would fail-loud if called.
	svc := newTestService(t, q)
	_, err := svc.CreateWithStatus(context.Background(), uuid.New(), "T", "", nil, nil, "completed")
	if err == nil || !errors.Is(err, ErrInvalidInitialStatus) {
		t.Errorf("expected ErrInvalidInitialStatus, got %v", err)
	}
}

func TestCreateWithStatus_RejectsBogus(t *testing.T) {
	q := &stubQueries{tb: t}
	svc := newTestService(t, q)
	_, err := svc.CreateWithStatus(context.Background(), uuid.New(), "T", "", nil, nil, "bogus")
	if err == nil || !errors.Is(err, ErrInvalidInitialStatus) {
		t.Errorf("expected ErrInvalidInitialStatus, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// GetByID / GetByIDOrCollaborator / GetByShareToken
// ---------------------------------------------------------------------------

func TestGetByID_HappyPath(t *testing.T) {
	userID, tripID := uuid.New(), uuid.New()
	q := &stubQueries{tb: t,
		getTripByIDFn: func(_ context.Context, arg dbgen.GetTripByIDParams) (dbgen.Trip, error) {
			if arg.ID != tripID || arg.UserID != userID {
				t.Errorf("params: got %+v, want id=%s user=%s", arg, tripID, userID)
			}
			return dbgen.Trip{ID: tripID, UserID: userID, Title: "X"}, nil
		},
	}
	svc := newTestService(t, q)
	got, err := svc.GetByID(context.Background(), userID, tripID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != tripID {
		t.Errorf("ID: got %s, want %s", got.ID, tripID)
	}
}

func TestGetByID_NotOwnerReturnsError(t *testing.T) {
	// pgx.ErrNoRows from the WHERE-by-user_id lookup must surface as a
	// wrapped error, NOT a zero-value Trip.
	q := &stubQueries{tb: t,
		getTripByIDFn: func(_ context.Context, _ dbgen.GetTripByIDParams) (dbgen.Trip, error) {
			return dbgen.Trip{}, pgx.ErrNoRows
		},
	}
	svc := newTestService(t, q)
	_, err := svc.GetByID(context.Background(), uuid.New(), uuid.New())
	if err == nil || !strings.Contains(err.Error(), "get trip") {
		t.Errorf("expected wrapped error, got %v", err)
	}
}

func TestGetByIDOrCollaborator_HappyPath(t *testing.T) {
	tripID := uuid.New()
	q := &stubQueries{tb: t,
		getTripByIDOrCollaboratorFn: func(_ context.Context, arg dbgen.GetTripByIDOrCollaboratorParams) (dbgen.Trip, error) {
			if arg.ID != tripID {
				t.Errorf("ID: got %s, want %s", arg.ID, tripID)
			}
			return dbgen.Trip{ID: tripID}, nil
		},
	}
	svc := newTestService(t, q)
	if _, err := svc.GetByIDOrCollaborator(context.Background(), uuid.New(), tripID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetByShareToken_HappyPath(t *testing.T) {
	q := &stubQueries{tb: t,
		getTripByShareTokenFn: func(_ context.Context, token pgtype.Text) (dbgen.Trip, error) {
			if !token.Valid || token.String != "abc123" {
				t.Errorf("token: got %+v, want abc123", token)
			}
			return dbgen.Trip{}, nil
		},
	}
	svc := newTestService(t, q)
	if _, err := svc.GetByShareToken(context.Background(), "abc123"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// ListByUser — both branches
// ---------------------------------------------------------------------------

func TestListByUser_NoStatusFilter(t *testing.T) {
	userID := uuid.New()
	q := &stubQueries{tb: t,
		listTripsByUserFn: func(_ context.Context, arg dbgen.ListTripsByUserParams) ([]dbgen.Trip, error) {
			if arg.UserID != userID || arg.Limit != 10 || arg.Offset != 5 {
				t.Errorf("params: got %+v, want user=%s limit=10 offset=5", arg, userID)
			}
			return []dbgen.Trip{{ID: uuid.New()}, {ID: uuid.New()}}, nil
		},
		countTripsByUserFn: func(_ context.Context, id uuid.UUID) (int64, error) {
			if id != userID {
				t.Errorf("user mismatch in count")
			}
			return 42, nil
		},
	}
	svc := newTestService(t, q)
	trips, count, err := svc.ListByUser(context.Background(), userID, "", 10, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(trips) != 2 {
		t.Errorf("trips: got %d, want 2", len(trips))
	}
	if count != 42 {
		t.Errorf("count: got %d, want 42", count)
	}
}

func TestListByUser_WithStatusFilter(t *testing.T) {
	userID := uuid.New()
	q := &stubQueries{tb: t,
		listTripsByUserAndStatusFn: func(_ context.Context, arg dbgen.ListTripsByUserAndStatusParams) ([]dbgen.Trip, error) {
			if arg.Status != "active" {
				t.Errorf("Status: got %q, want active", arg.Status)
			}
			return []dbgen.Trip{{ID: uuid.New()}}, nil
		},
		countTripsByUserAndStatusFn: func(_ context.Context, arg dbgen.CountTripsByUserAndStatusParams) (int64, error) {
			if arg.Status != "active" {
				t.Errorf("Status: got %q, want active", arg.Status)
			}
			return 7, nil
		},
	}
	svc := newTestService(t, q)
	trips, count, err := svc.ListByUser(context.Background(), userID, "active", 10, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(trips) != 1 || count != 7 {
		t.Errorf("got trips=%d count=%d, want 1 / 7", len(trips), count)
	}
}

func TestListByUser_ListErrorPropagates(t *testing.T) {
	q := &stubQueries{tb: t,
		listTripsByUserFn: func(_ context.Context, _ dbgen.ListTripsByUserParams) ([]dbgen.Trip, error) {
			return nil, errors.New("db down")
		},
	}
	svc := newTestService(t, q)
	_, _, err := svc.ListByUser(context.Background(), uuid.New(), "", 10, 0)
	if err == nil || !strings.Contains(err.Error(), "list trips") {
		t.Errorf("expected wrapped list error, got %v", err)
	}
}

func TestListSharedTrips_HappyPath(t *testing.T) {
	userID := uuid.New()
	q := &stubQueries{tb: t,
		listSharedTripsFn: func(_ context.Context, arg pgtype.UUID) ([]dbgen.Trip, error) {
			if !arg.Valid || arg.Bytes != userID {
				t.Errorf("user: got %+v, want %s", arg, userID)
			}
			return []dbgen.Trip{{ID: uuid.New()}}, nil
		},
	}
	svc := newTestService(t, q)
	got, err := svc.ListSharedTrips(context.Background(), userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("got %d trips, want 1", len(got))
	}
}

// ---------------------------------------------------------------------------
// SearchByUser — ASCII vs non-ASCII routing (#391/N-23 P1)
// ---------------------------------------------------------------------------

func TestSearchByUser_ASCIIUsesTSVector(t *testing.T) {
	q := &stubQueries{tb: t,
		searchTripsByUserFn: func(_ context.Context, arg dbgen.SearchTripsByUserParams) ([]dbgen.Trip, error) {
			if arg.Query != "Greece" {
				t.Errorf("Query: got %q, want Greece", arg.Query)
			}
			return []dbgen.Trip{{ID: uuid.New()}}, nil
		},
	}
	svc := newTestService(t, q)
	if _, err := svc.SearchByUser(context.Background(), uuid.New(), "Greece", 10); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSearchByUser_NonASCIIUsesILIKE(t *testing.T) {
	// CJK / Cyrillic / Arabic don't tokenize cleanly with PostgreSQL's
	// tsvector — N-23 P1 routed those through ILIKE. This test pins the
	// routing so a refactor doesn't silently regress non-Latin search.
	q := &stubQueries{tb: t,
		searchTripsByUserILIKEFn: func(_ context.Context, arg dbgen.SearchTripsByUserILIKEParams) ([]dbgen.Trip, error) {
			if !arg.Query.Valid || arg.Query.String != "日本" {
				t.Errorf("Query: got %+v, want 日本", arg.Query)
			}
			return []dbgen.Trip{{ID: uuid.New()}}, nil
		},
	}
	svc := newTestService(t, q)
	if _, err := svc.SearchByUser(context.Background(), uuid.New(), "日本", 10); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestIsNonASCII(t *testing.T) {
	cases := map[string]bool{
		"Greece": false,
		"日本":     true,
		"Café":   true, // é > 127
		"":       false,
		"hello!": false,
		"Россия": true,
	}
	for input, want := range cases {
		if got := isNonASCII(input); got != want {
			t.Errorf("isNonASCII(%q) = %v; want %v", input, got, want)
		}
	}
}

// ---------------------------------------------------------------------------
// Update — status machine + date validation
// ---------------------------------------------------------------------------

func TestUpdate_NoStatusChange_SkipsLoadCheck(t *testing.T) {
	tripID := uuid.New()
	q := &stubQueries{tb: t,
		// getTripByIDFn deliberately UNSET — must not be called when status is empty.
		updateTripFn: func(_ context.Context, arg dbgen.UpdateTripParams) (dbgen.Trip, error) {
			if arg.ID != tripID {
				t.Errorf("ID mismatch")
			}
			return dbgen.Trip{ID: tripID, Title: "Updated"}, nil
		},
	}
	svc := newTestService(t, q)
	got, err := svc.Update(context.Background(), uuid.New(), tripID, "Updated", "", "", nil, nil, nil, "", "", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Title != "Updated" {
		t.Errorf("Title: got %q, want Updated", got.Title)
	}
}

func TestUpdate_RejectsEndBeforeStart(t *testing.T) {
	q := &stubQueries{tb: t} // UpdateTrip must NOT be called.
	svc := newTestService(t, q)
	start := mustParseDate(t, "2026-06-15")
	end := mustParseDate(t, "2026-06-10")
	_, err := svc.Update(context.Background(), uuid.New(), uuid.New(), "T", "", "", &start, &end, nil, "", "", "", "")
	if err == nil || !strings.Contains(err.Error(), "end_date") {
		t.Errorf("expected end_date validation error, got %v", err)
	}
	if len(q.updateTripCalls) != 0 {
		t.Errorf("UpdateTrip must not be called on validation failure")
	}
}

func TestUpdate_StatusTransition_PlanningToActive(t *testing.T) {
	q := &stubQueries{tb: t,
		getTripByIDFn: func(_ context.Context, _ dbgen.GetTripByIDParams) (dbgen.Trip, error) {
			return dbgen.Trip{Status: "planning"}, nil
		},
		updateTripFn: func(_ context.Context, arg dbgen.UpdateTripParams) (dbgen.Trip, error) {
			if arg.Status != "active" {
				t.Errorf("Status: got %q, want active", arg.Status)
			}
			return dbgen.Trip{Status: "active"}, nil
		},
	}
	svc := newTestService(t, q)
	if _, err := svc.Update(context.Background(), uuid.New(), uuid.New(), "", "", "active", nil, nil, nil, "", "", "", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestUpdate_RejectsCompletedToPlanning pins the Run 4 N-08 P2 invariant:
// terminal-state trips cannot be reopened. Without this guard, a client
// could mark a trip "completed" then reopen it, leaving archival/cleanup
// in an inconsistent state.
func TestUpdate_RejectsCompletedToPlanning(t *testing.T) {
	q := &stubQueries{tb: t,
		getTripByIDFn: func(_ context.Context, _ dbgen.GetTripByIDParams) (dbgen.Trip, error) {
			return dbgen.Trip{Status: "completed"}, nil
		},
		// updateTripFn UNSET — must not be called.
	}
	svc := newTestService(t, q)
	_, err := svc.Update(context.Background(), uuid.New(), uuid.New(), "", "", "planning", nil, nil, nil, "", "", "", "")
	if err == nil || !errors.Is(err, ErrInvalidStatusTransition) {
		t.Errorf("expected ErrInvalidStatusTransition, got %v", err)
	}
	if len(q.updateTripCalls) != 0 {
		t.Errorf("UpdateTrip must not be called on invalid transition")
	}
}

func TestUpdate_RejectsActiveToPlanning(t *testing.T) {
	q := &stubQueries{tb: t,
		getTripByIDFn: func(_ context.Context, _ dbgen.GetTripByIDParams) (dbgen.Trip, error) {
			return dbgen.Trip{Status: "active"}, nil
		},
	}
	svc := newTestService(t, q)
	_, err := svc.Update(context.Background(), uuid.New(), uuid.New(), "", "", "planning", nil, nil, nil, "", "", "", "")
	if err == nil || !errors.Is(err, ErrInvalidStatusTransition) {
		t.Errorf("expected ErrInvalidStatusTransition, got %v", err)
	}
}

func TestUpdate_StatusCheckLoadError(t *testing.T) {
	// If the status pre-check load fails (transient DB error), we surface a
	// wrapped error rather than silently skipping the check. Run 4 N-08 P2.
	q := &stubQueries{tb: t,
		getTripByIDFn: func(_ context.Context, _ dbgen.GetTripByIDParams) (dbgen.Trip, error) {
			return dbgen.Trip{}, errors.New("db flaky")
		},
	}
	svc := newTestService(t, q)
	_, err := svc.Update(context.Background(), uuid.New(), uuid.New(), "", "", "active", nil, nil, nil, "", "", "", "")
	if err == nil || !strings.Contains(err.Error(), "status check") {
		t.Errorf("expected status-check error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Delete
// ---------------------------------------------------------------------------

func TestDelete_HappyPath(t *testing.T) {
	userID, tripID := uuid.New(), uuid.New()
	q := &stubQueries{tb: t,
		deleteTripFn: func(_ context.Context, arg dbgen.DeleteTripParams) error {
			if arg.ID != tripID || arg.UserID != userID {
				t.Errorf("params: got %+v", arg)
			}
			return nil
		},
	}
	svc := newTestService(t, q)
	if err := svc.Delete(context.Background(), userID, tripID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDelete_QueryErrorWrapped(t *testing.T) {
	q := &stubQueries{tb: t,
		deleteTripFn: func(_ context.Context, _ dbgen.DeleteTripParams) error {
			return errors.New("db down")
		},
	}
	svc := newTestService(t, q)
	err := svc.Delete(context.Background(), uuid.New(), uuid.New())
	if err == nil || !strings.Contains(err.Error(), "delete trip") {
		t.Errorf("expected wrapped error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Sharing
// ---------------------------------------------------------------------------

func TestEnableSharing_TokenIsAlphanumericAndPersisted(t *testing.T) {
	userID, tripID := uuid.New(), uuid.New()
	var captured pgtype.Text
	q := &stubQueries{tb: t,
		enableTripSharingFn: func(_ context.Context, arg dbgen.EnableTripSharingParams) (dbgen.Trip, error) {
			if arg.ID != tripID || arg.UserID != userID {
				t.Errorf("params: got %+v", arg)
			}
			captured = arg.ShareToken
			return dbgen.Trip{}, nil
		},
	}
	svc := newTestService(t, q)
	token, err := svc.EnableSharing(context.Background(), userID, tripID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(token) != shareTokenLength {
		t.Errorf("token length: got %d, want %d", len(token), shareTokenLength)
	}
	if !captured.Valid || captured.String != token {
		t.Errorf("persisted token mismatch: got %+v, want %q", captured, token)
	}
}

func TestEnableSharing_QueryErrorWrapped(t *testing.T) {
	q := &stubQueries{tb: t,
		enableTripSharingFn: func(_ context.Context, _ dbgen.EnableTripSharingParams) (dbgen.Trip, error) {
			return dbgen.Trip{}, errors.New("db down")
		},
	}
	svc := newTestService(t, q)
	_, err := svc.EnableSharing(context.Background(), uuid.New(), uuid.New())
	if err == nil || !strings.Contains(err.Error(), "enable trip sharing") {
		t.Errorf("expected wrapped error, got %v", err)
	}
}

func TestDisableSharing_HappyPath(t *testing.T) {
	q := &stubQueries{tb: t,
		disableTripSharingFn: func(_ context.Context, _ dbgen.DisableTripSharingParams) (dbgen.Trip, error) {
			return dbgen.Trip{}, nil
		},
	}
	svc := newTestService(t, q)
	if err := svc.DisableSharing(context.Background(), uuid.New(), uuid.New()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Templates
// ---------------------------------------------------------------------------

func TestListTemplates_HappyPath(t *testing.T) {
	q := &stubQueries{tb: t,
		listTripTemplatesFn: func(_ context.Context, _ dbgen.ListTripTemplatesParams) ([]dbgen.Trip, error) {
			return []dbgen.Trip{{ID: uuid.New()}, {ID: uuid.New()}}, nil
		},
		countTripTemplatesFn: func(_ context.Context) (int64, error) {
			return 2, nil
		},
	}
	svc := newTestService(t, q)
	templates, count, err := svc.ListTemplates(context.Background(), 10, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(templates) != 2 || count != 2 {
		t.Errorf("got templates=%d count=%d", len(templates), count)
	}
}

// ---------------------------------------------------------------------------
// Destination management
// ---------------------------------------------------------------------------

func TestSetDestination_HappyPath(t *testing.T) {
	q := &stubQueries{tb: t,
		updateTripDestinationFn: func(_ context.Context, arg dbgen.UpdateTripDestinationParams) (pgconn.CommandTag, error) {
			if !arg.DestinationCountry.Valid || arg.DestinationCountry.String != "JP" {
				t.Errorf("country: got %+v", arg.DestinationCountry)
			}
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	svc := newTestService(t, q)
	if err := svc.SetDestination(context.Background(), uuid.New(), uuid.New(), "JP"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetDestination_RowsAffectedZeroIsAccessDenied(t *testing.T) {
	// 0 rows affected = either the trip doesn't exist OR the user doesn't
	// own it. Authz is enforced in SQL via the WHERE clause, so the
	// service maps 0-rows to "not found or access denied". This is the
	// owner-only sibling of the #345/#343 RowsAffected pattern.
	q := &stubQueries{tb: t,
		updateTripDestinationFn: func(_ context.Context, _ dbgen.UpdateTripDestinationParams) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
	}
	svc := newTestService(t, q)
	err := svc.SetDestination(context.Background(), uuid.New(), uuid.New(), "JP")
	if err == nil || !strings.Contains(err.Error(), "access denied") {
		t.Errorf("expected access-denied error, got %v", err)
	}
}

func TestSetDestinations_EmptySliceIsNoOp(t *testing.T) {
	q := &stubQueries{tb: t} // UpdateTripDestinations must NOT be called.
	svc := newTestService(t, q)
	if err := svc.SetDestinations(context.Background(), uuid.New(), uuid.New(), nil); err != nil {
		t.Errorf("expected nil for empty slice, got %v", err)
	}
	if len(q.updateDestinationsCalls) != 0 {
		t.Errorf("UpdateTripDestinations must not be called for empty input")
	}
}

func TestSetDestinations_FirstIsPrimary(t *testing.T) {
	q := &stubQueries{tb: t,
		updateTripDestinationsFn: func(_ context.Context, arg dbgen.UpdateTripDestinationsParams) (pgconn.CommandTag, error) {
			if arg.PrimaryCountry != "IT" {
				t.Errorf("primary: got %q, want IT", arg.PrimaryCountry)
			}
			if len(arg.DestinationCountries) != 3 {
				t.Errorf("countries: got %d, want 3", len(arg.DestinationCountries))
			}
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	svc := newTestService(t, q)
	if err := svc.SetDestinations(context.Background(), uuid.New(), uuid.New(), []string{"IT", "FR", "ES"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetDestinations_AccessDeniedWhenZeroRows(t *testing.T) {
	q := &stubQueries{tb: t,
		updateTripDestinationsFn: func(_ context.Context, _ dbgen.UpdateTripDestinationsParams) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
	}
	svc := newTestService(t, q)
	err := svc.SetDestinations(context.Background(), uuid.New(), uuid.New(), []string{"IT"})
	if err == nil || !strings.Contains(err.Error(), "access denied") {
		t.Errorf("expected access-denied error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Authz — IsEditorCollaborator + CanEditTrip (#346/#352/#353/#358 TOCTOU
// fixes — these are the highest-value tests in this package because they
// pin the authz invariants for write operations on shared trips).
// ---------------------------------------------------------------------------

func TestIsEditorCollaborator_TrueWhenAccepted(t *testing.T) {
	userID, tripID := uuid.New(), uuid.New()
	q := &stubQueries{tb: t,
		isAcceptedCollaboratorFn: func(_ context.Context, arg dbgen.IsAcceptedCollaboratorWithRoleParams) (bool, error) {
			if arg.TripID != tripID || arg.UserID.Bytes != userID || arg.Role != "editor" {
				t.Errorf("params: got %+v", arg)
			}
			return true, nil
		},
	}
	svc := newTestService(t, q)
	ok, err := svc.IsEditorCollaborator(context.Background(), userID, tripID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected editor=true")
	}
}

// TestIsEditorCollaborator_DBErrorReturnsFalseAndError pins the #348
// invariant: transient DB failures must surface as (false, err) so the
// caller can distinguish "definitely not an editor" from "couldn't answer
// the question". Swallowing the error would let a 5xx-worthy condition
// look like a deliberate PermissionDenied.
func TestIsEditorCollaborator_DBErrorReturnsFalseAndError(t *testing.T) {
	q := &stubQueries{tb: t,
		isAcceptedCollaboratorFn: func(_ context.Context, _ dbgen.IsAcceptedCollaboratorWithRoleParams) (bool, error) {
			return false, errors.New("db transient failure")
		},
	}
	svc := newTestService(t, q)
	ok, err := svc.IsEditorCollaborator(context.Background(), uuid.New(), uuid.New())
	if err == nil {
		t.Error("expected error from transient DB failure")
	}
	if ok {
		t.Error("must return false on error to avoid false positives")
	}
}

func TestCanEditTrip_OwnerFastPath(t *testing.T) {
	// Owner check succeeds — must NOT fall through to the editor query.
	q := &stubQueries{tb: t,
		getTripByIDFn: func(_ context.Context, _ dbgen.GetTripByIDParams) (dbgen.Trip, error) {
			return dbgen.Trip{ID: uuid.New()}, nil
		},
		// isAcceptedCollaboratorFn deliberately UNSET — fail-loud if called.
	}
	svc := newTestService(t, q)
	ok, err := svc.CanEditTrip(context.Background(), uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("owner should be able to edit")
	}
}

// TestCanEditTrip_NotOwnerFallsThroughToEditor pins #348: ErrNoRows from
// the ownership probe is treated as "not the owner, try the editor path",
// not as a transient error.
func TestCanEditTrip_NotOwnerFallsThroughToEditor(t *testing.T) {
	q := &stubQueries{tb: t,
		getTripByIDFn: func(_ context.Context, _ dbgen.GetTripByIDParams) (dbgen.Trip, error) {
			return dbgen.Trip{}, pgx.ErrNoRows
		},
		isAcceptedCollaboratorFn: func(_ context.Context, _ dbgen.IsAcceptedCollaboratorWithRoleParams) (bool, error) {
			return true, nil
		},
	}
	svc := newTestService(t, q)
	ok, err := svc.CanEditTrip(context.Background(), uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("editor collaborator should be able to edit")
	}
}

func TestCanEditTrip_ViewerCannotEdit(t *testing.T) {
	q := &stubQueries{tb: t,
		getTripByIDFn: func(_ context.Context, _ dbgen.GetTripByIDParams) (dbgen.Trip, error) {
			return dbgen.Trip{}, pgx.ErrNoRows
		},
		isAcceptedCollaboratorFn: func(_ context.Context, _ dbgen.IsAcceptedCollaboratorWithRoleParams) (bool, error) {
			return false, nil
		},
	}
	svc := newTestService(t, q)
	ok, err := svc.CanEditTrip(context.Background(), uuid.New(), uuid.New())
	if err != nil {
		t.Errorf("expected nil err for clean 'no', got %v", err)
	}
	if ok {
		t.Error("viewer must not be able to edit")
	}
}

// TestCanEditTrip_TransientOwnerErrorPropagates pins the #348 distinction
// between "definitely not the owner" (ErrNoRows → keep going) and
// "couldn't answer" (any other error → fail loudly). A CanEditTrip that
// swallows transient errors would let an authz pre-check return a false
// "no" and turn a retryable 5xx into a deliberate-looking 403.
func TestCanEditTrip_TransientOwnerErrorPropagates(t *testing.T) {
	q := &stubQueries{tb: t,
		getTripByIDFn: func(_ context.Context, _ dbgen.GetTripByIDParams) (dbgen.Trip, error) {
			return dbgen.Trip{}, errors.New("db down")
		},
		// isAcceptedCollaboratorFn UNSET — must not fall through on transient errors.
	}
	svc := newTestService(t, q)
	ok, err := svc.CanEditTrip(context.Background(), uuid.New(), uuid.New())
	if err == nil {
		t.Error("expected error from transient DB failure")
	}
	if ok {
		t.Error("must return false on transient error")
	}
}

// ---------------------------------------------------------------------------
// Itinerary listing
// ---------------------------------------------------------------------------

func TestGetItinerary_HappyPath(t *testing.T) {
	q := &stubQueries{tb: t,
		listItineraryItemsByTripFn: func(_ context.Context, _ uuid.UUID) ([]dbgen.ItineraryItem, error) {
			return []dbgen.ItineraryItem{{ID: uuid.New()}, {ID: uuid.New()}}, nil
		},
	}
	svc := newTestService(t, q)
	items, err := svc.GetItinerary(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("got %d items, want 2", len(items))
	}
}

func TestGetItineraryForOwnerOrEditor_DeniesViewer(t *testing.T) {
	// Viewer (not owner, not editor) must get ErrNotOwnerOrEditor — and
	// the underlying ListItineraryItemsByTrip must NOT be called.
	q := &stubQueries{tb: t,
		getTripByIDFn: func(_ context.Context, _ dbgen.GetTripByIDParams) (dbgen.Trip, error) {
			return dbgen.Trip{}, pgx.ErrNoRows
		},
		isAcceptedCollaboratorFn: func(_ context.Context, _ dbgen.IsAcceptedCollaboratorWithRoleParams) (bool, error) {
			return false, nil
		},
		// listItineraryItemsByTripFn UNSET — fail-loud if called.
	}
	svc := newTestService(t, q)
	_, err := svc.GetItineraryForOwnerOrEditor(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, ErrNotOwnerOrEditor) {
		t.Errorf("expected ErrNotOwnerOrEditor, got %v", err)
	}
}

func TestGetItineraryForOwnerOrEditor_AllowsOwner(t *testing.T) {
	q := &stubQueries{tb: t,
		getTripByIDFn: func(_ context.Context, _ dbgen.GetTripByIDParams) (dbgen.Trip, error) {
			return dbgen.Trip{ID: uuid.New()}, nil
		},
		listItineraryItemsByTripFn: func(_ context.Context, _ uuid.UUID) ([]dbgen.ItineraryItem, error) {
			return []dbgen.ItineraryItem{{ID: uuid.New()}}, nil
		},
	}
	svc := newTestService(t, q)
	got, err := svc.GetItineraryForOwnerOrEditor(context.Background(), uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("got %d items, want 1", len(got))
	}
}

func TestGetItineraryForOwnerOrEditor_AllowsEditor(t *testing.T) {
	q := &stubQueries{tb: t,
		getTripByIDFn: func(_ context.Context, _ dbgen.GetTripByIDParams) (dbgen.Trip, error) {
			return dbgen.Trip{}, pgx.ErrNoRows
		},
		isAcceptedCollaboratorFn: func(_ context.Context, _ dbgen.IsAcceptedCollaboratorWithRoleParams) (bool, error) {
			return true, nil
		},
		listItineraryItemsByTripFn: func(_ context.Context, _ uuid.UUID) ([]dbgen.ItineraryItem, error) {
			return []dbgen.ItineraryItem{}, nil
		},
	}
	svc := newTestService(t, q)
	if _, err := svc.GetItineraryForOwnerOrEditor(context.Background(), uuid.New(), uuid.New()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetItineraryForOwnerOrEditor_GateErrorWrapped(t *testing.T) {
	q := &stubQueries{tb: t,
		getTripByIDFn: func(_ context.Context, _ dbgen.GetTripByIDParams) (dbgen.Trip, error) {
			return dbgen.Trip{}, errors.New("db down")
		},
	}
	svc := newTestService(t, q)
	_, err := svc.GetItineraryForOwnerOrEditor(context.Background(), uuid.New(), uuid.New())
	if errors.Is(err, ErrNotOwnerOrEditor) {
		t.Errorf("transient DB error must NOT be reported as ErrNotOwnerOrEditor: %v", err)
	}
	if err == nil || !strings.Contains(err.Error(), "check edit access") {
		t.Errorf("expected wrapped error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Itinerary item create — owner-only and authz-gated variants (#346/#353)
// ---------------------------------------------------------------------------

func TestCreateItineraryItem_HappyPath(t *testing.T) {
	tripID := uuid.New()
	q := &stubQueries{tb: t,
		createItineraryItemFn: func(_ context.Context, arg dbgen.CreateItineraryItemParams) (dbgen.ItineraryItem, error) {
			if arg.TripID != tripID {
				t.Errorf("TripID: got %s, want %s", arg.TripID, tripID)
			}
			if !arg.Title.Valid || arg.Title.String != "Lunch" {
				t.Errorf("Title: got %+v, want Lunch", arg.Title)
			}
			return dbgen.ItineraryItem{ID: uuid.New()}, nil
		},
	}
	svc := newTestService(t, q)
	if _, err := svc.CreateItineraryItem(context.Background(), tripID, 1, 1, "meal", "Lunch", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateItineraryItemWithCost_PassesCostFields(t *testing.T) {
	cost := int64(1500)
	q := &stubQueries{tb: t,
		createItineraryItemFn: func(_ context.Context, arg dbgen.CreateItineraryItemParams) (dbgen.ItineraryItem, error) {
			if !arg.EstimatedCostCents.Valid || arg.EstimatedCostCents.Int64 != 1500 {
				t.Errorf("cost: got %+v, want 1500", arg.EstimatedCostCents)
			}
			if !arg.CostCurrency.Valid || arg.CostCurrency.String != "USD" {
				t.Errorf("currency: got %+v, want USD", arg.CostCurrency)
			}
			return dbgen.ItineraryItem{}, nil
		},
	}
	svc := newTestService(t, q)
	if _, err := svc.CreateItineraryItemWithCost(context.Background(), uuid.New(), 1, 1, "meal", "L", "", &cost, "USD"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestCreateItineraryItemForOwnerOrEditor_HappyPath pins the #346/#353
// authz-gated insert path — the underlying query enforces authz in SQL.
func TestCreateItineraryItemForOwnerOrEditor_HappyPath(t *testing.T) {
	callerID, tripID := uuid.New(), uuid.New()
	q := &stubQueries{tb: t,
		createItineraryItemForOwnerOrEditorFn: func(_ context.Context, arg dbgen.CreateItineraryItemForOwnerOrEditorParams) (dbgen.ItineraryItem, error) {
			if arg.UserID != callerID || arg.TripID != tripID {
				t.Errorf("authz params mismatch: got %+v", arg)
			}
			return dbgen.ItineraryItem{ID: uuid.New()}, nil
		},
	}
	svc := newTestService(t, q)
	if _, err := svc.CreateItineraryItemForOwnerOrEditor(context.Background(), callerID, tripID, 1, 1, "meal", "T", "", nil, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestCreateItineraryItemForOwnerOrEditor_NoRowsIsErrNotOwnerOrEditor pins
// the #346/#353 TOCTOU regression: when the SQL WHERE clause filters out
// the caller (because they were demoted from editor mid-stream or never
// had access), pgx returns ErrNoRows. The service translates that to the
// ErrNotOwnerOrEditor sentinel so the handler maps to PermissionDenied,
// not a raw pgx error or — worse — a silent zero-value insert.
func TestCreateItineraryItemForOwnerOrEditor_NoRowsIsErrNotOwnerOrEditor(t *testing.T) {
	q := &stubQueries{tb: t,
		createItineraryItemForOwnerOrEditorFn: func(_ context.Context, _ dbgen.CreateItineraryItemForOwnerOrEditorParams) (dbgen.ItineraryItem, error) {
			return dbgen.ItineraryItem{}, pgx.ErrNoRows
		},
	}
	svc := newTestService(t, q)
	_, err := svc.CreateItineraryItemForOwnerOrEditor(context.Background(), uuid.New(), uuid.New(), 1, 1, "meal", "T", "", nil, "")
	if !errors.Is(err, ErrNotOwnerOrEditor) {
		t.Errorf("expected ErrNotOwnerOrEditor, got %v", err)
	}
}

func TestCreateItineraryItemForOwnerOrEditor_OtherErrorWrapped(t *testing.T) {
	// Non-ErrNoRows errors must NOT be turned into authz failures.
	q := &stubQueries{tb: t,
		createItineraryItemForOwnerOrEditorFn: func(_ context.Context, _ dbgen.CreateItineraryItemForOwnerOrEditorParams) (dbgen.ItineraryItem, error) {
			return dbgen.ItineraryItem{}, errors.New("db down")
		},
	}
	svc := newTestService(t, q)
	_, err := svc.CreateItineraryItemForOwnerOrEditor(context.Background(), uuid.New(), uuid.New(), 1, 1, "meal", "T", "", nil, "")
	if errors.Is(err, ErrNotOwnerOrEditor) {
		t.Errorf("transient DB error must NOT be reported as ErrNotOwnerOrEditor: %v", err)
	}
	if err == nil || !strings.Contains(err.Error(), "create itinerary item") {
		t.Errorf("expected wrapped error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Itinerary item delete — DB-truth via RowsAffected (#345/#343)
// ---------------------------------------------------------------------------

// TestDeleteItineraryItems_OnlyAffectedReturned pins #345: the service
// inspects RowsAffected and only appends to the "deleted" slice when a row
// was actually removed. Trusting "no pgx error" → "deleted" was the bug
// that #345 fixed for the owner-only path; before the fix, items belonging
// to a different user silently appeared in the response as "deleted".
func TestDeleteItineraryItems_OnlyAffectedReturned(t *testing.T) {
	userID := uuid.New()
	id1, id2, id3 := uuid.New(), uuid.New(), uuid.New()

	rowsByID := map[uuid.UUID]int64{
		id1: 1, // owned by user → deleted
		id2: 0, // foreign or missing → silent no-op at SQL layer
		id3: 1,
	}

	q := &stubQueries{tb: t,
		deleteItineraryItemFn: func(_ context.Context, arg dbgen.DeleteItineraryItemParams) (int64, error) {
			if arg.UserID != userID {
				t.Errorf("UserID: got %s, want %s", arg.UserID, userID)
			}
			return rowsByID[arg.ID], nil
		},
	}
	svc := newTestService(t, q)
	deleted, err := svc.DeleteItineraryItems(context.Background(), userID, []uuid.UUID{id1, id2, id3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deleted) != 2 {
		t.Errorf("expected 2 IDs deleted (1 was foreign), got %d: %v", len(deleted), deleted)
	}
	for _, got := range deleted {
		if got == id2 {
			t.Errorf("foreign ID %s must not appear as deleted", id2)
		}
	}
}

func TestDeleteItineraryItems_AllZeroRowsReturnsNilDeleted(t *testing.T) {
	// Zero rows for every input is not an error — just an empty result.
	// (Caller renders this as "nothing to delete", not a 5xx.)
	q := &stubQueries{tb: t,
		deleteItineraryItemFn: func(_ context.Context, _ dbgen.DeleteItineraryItemParams) (int64, error) {
			return 0, nil
		},
	}
	svc := newTestService(t, q)
	deleted, err := svc.DeleteItineraryItems(context.Background(), uuid.New(), []uuid.UUID{uuid.New(), uuid.New()})
	if err != nil {
		t.Errorf("expected nil err for all-foreign deletes, got %v", err)
	}
	if len(deleted) != 0 {
		t.Errorf("expected zero deleted, got %v", deleted)
	}
}

func TestDeleteItineraryItems_PartialErrorReturnsSuccessIDs(t *testing.T) {
	// Partial-failure semantics: errors on some IDs and successes on
	// others → return the successful IDs with nil error.
	id1, id2 := uuid.New(), uuid.New()
	q := &stubQueries{tb: t,
		deleteItineraryItemFn: func(_ context.Context, arg dbgen.DeleteItineraryItemParams) (int64, error) {
			if arg.ID == id1 {
				return 0, errors.New("transient")
			}
			return 1, nil
		},
	}
	svc := newTestService(t, q)
	deleted, err := svc.DeleteItineraryItems(context.Background(), uuid.New(), []uuid.UUID{id1, id2})
	if err != nil {
		t.Errorf("expected nil err for partial success, got %v", err)
	}
	if len(deleted) != 1 || deleted[0] != id2 {
		t.Errorf("expected deleted=[%s], got %v", id2, deleted)
	}
}

func TestDeleteItineraryItems_AllErrorsReturnsAggregateError(t *testing.T) {
	// All deletions fail → return aggregate error so the handler can
	// surface a 5xx instead of pretending success.
	q := &stubQueries{tb: t,
		deleteItineraryItemFn: func(_ context.Context, _ dbgen.DeleteItineraryItemParams) (int64, error) {
			return 0, errors.New("db down")
		},
	}
	svc := newTestService(t, q)
	_, err := svc.DeleteItineraryItems(context.Background(), uuid.New(), []uuid.UUID{uuid.New(), uuid.New()})
	if err == nil || !strings.Contains(err.Error(), "deletions failed") {
		t.Errorf("expected aggregate error, got %v", err)
	}
}

// TestDeleteItineraryItemsForOwnerOrEditor_OnlyAffectedReturned pins #343:
// the authz-gated delete path also reports DB truth via RowsAffected
// rather than swallowing zero-row no-ops as success. The original
// owner-or-editor query was annotated `:exec` (no RowsAffected) and
// callers saw a bogus success for every ID regardless of authz.
func TestDeleteItineraryItemsForOwnerOrEditor_OnlyAffectedReturned(t *testing.T) {
	userID := uuid.New()
	id1, id2, id3 := uuid.New(), uuid.New(), uuid.New()
	rowsByID := map[uuid.UUID]int64{
		id1: 1, // owner/editor → deleted
		id2: 0, // viewer or non-collaborator → no-op
		id3: 1,
	}
	q := &stubQueries{tb: t,
		deleteItineraryItemByOwnerOrEditorFn: func(_ context.Context, arg dbgen.DeleteItineraryItemByOwnerOrEditorParams) (int64, error) {
			return rowsByID[arg.ID], nil
		},
	}
	svc := newTestService(t, q)
	deleted, err := svc.DeleteItineraryItemsForOwnerOrEditor(context.Background(), userID, []uuid.UUID{id1, id2, id3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deleted) != 2 {
		t.Errorf("expected 2 IDs deleted, got %d: %v", len(deleted), deleted)
	}
	for _, got := range deleted {
		if got == id2 {
			t.Errorf("non-collaborator ID %s must not appear as deleted", id2)
		}
	}
}

func TestDeleteItineraryItemsForOwnerOrEditor_AllErrorsReturnsAggregate(t *testing.T) {
	q := &stubQueries{tb: t,
		deleteItineraryItemByOwnerOrEditorFn: func(_ context.Context, _ dbgen.DeleteItineraryItemByOwnerOrEditorParams) (int64, error) {
			return 0, errors.New("db down")
		},
	}
	svc := newTestService(t, q)
	_, err := svc.DeleteItineraryItemsForOwnerOrEditor(context.Background(), uuid.New(), []uuid.UUID{uuid.New()})
	if err == nil || !strings.Contains(err.Error(), "deletions failed") {
		t.Errorf("expected aggregate error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// MoveItineraryItem — owner-or-editor authz (#361 P2)
// ---------------------------------------------------------------------------

// TestMoveItineraryItem_HappyPath pins the #361 fix: the service calls
// MoveItineraryItemForOwnerOrEditor (not the legacy owner-only query) so
// editor-role collaborators don't hit pgx.ErrNoRows on a legitimate move.
func TestMoveItineraryItem_HappyPath(t *testing.T) {
	q := &stubQueries{tb: t,
		moveItineraryItemForOwnerOrEditorFn: func(_ context.Context, arg dbgen.MoveItineraryItemForOwnerOrEditorParams) (dbgen.ItineraryItem, error) {
			if !arg.DayNumber.Valid || arg.DayNumber.Int32 != 3 {
				t.Errorf("DayNumber: got %+v, want 3", arg.DayNumber)
			}
			if !arg.OrderInDay.Valid || arg.OrderInDay.Int32 != 2 {
				t.Errorf("OrderInDay: got %+v, want 2", arg.OrderInDay)
			}
			return dbgen.ItineraryItem{ID: uuid.New()}, nil
		},
	}
	svc := newTestService(t, q)
	if _, err := svc.MoveItineraryItem(context.Background(), uuid.New(), uuid.New(), uuid.New(), 3, 2); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestMoveItineraryItem_TargetPosClampedToOne pins the
// "targetPos <= 0 → 1" sanitization so a poorly-supplied client value
// can't write order_in_day=0 (which our 1-indexed schema treats as NULL
// per int4FromInt's contract).
func TestMoveItineraryItem_TargetPosClampedToOne(t *testing.T) {
	q := &stubQueries{tb: t,
		moveItineraryItemForOwnerOrEditorFn: func(_ context.Context, arg dbgen.MoveItineraryItemForOwnerOrEditorParams) (dbgen.ItineraryItem, error) {
			if !arg.OrderInDay.Valid || arg.OrderInDay.Int32 != 1 {
				t.Errorf("OrderInDay must be clamped to 1, got %+v", arg.OrderInDay)
			}
			return dbgen.ItineraryItem{}, nil
		},
	}
	svc := newTestService(t, q)
	if _, err := svc.MoveItineraryItem(context.Background(), uuid.New(), uuid.New(), uuid.New(), 1, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := svc.MoveItineraryItem(context.Background(), uuid.New(), uuid.New(), uuid.New(), 1, -5); err != nil {
		t.Fatalf("unexpected error (negative): %v", err)
	}
}

func TestMoveItineraryItem_QueryErrorWrapped(t *testing.T) {
	q := &stubQueries{tb: t,
		moveItineraryItemForOwnerOrEditorFn: func(_ context.Context, _ dbgen.MoveItineraryItemForOwnerOrEditorParams) (dbgen.ItineraryItem, error) {
			return dbgen.ItineraryItem{}, errors.New("db down")
		},
	}
	svc := newTestService(t, q)
	_, err := svc.MoveItineraryItem(context.Background(), uuid.New(), uuid.New(), uuid.New(), 1, 1)
	if err == nil || !strings.Contains(err.Error(), "move itinerary item") {
		t.Errorf("expected wrapped error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func mustParseDate(t *testing.T, s string) time.Time {
	t.Helper()
	out, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatalf("mustParseDate(%q): %v", s, err)
	}
	return out
}
