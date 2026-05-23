package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gallowaysoftware/toqui/backend/internal/auth"
	"github.com/gallowaysoftware/toqui/backend/internal/dbgen"
)

// stubFeedbackQueries is a fail-loud test double for feedbackQueries.
// Same pattern as the stubs in internal/booking, internal/lifecycle,
// internal/trip — every method calls tb.Fatalf if no *Fn hook is set,
// so a test that triggers an unexpected DB call fails with a precise
// error rather than silently exercising a zero-value path.

type stubFeedbackQueries struct {
	tb testing.TB

	getTripByIDOrCollaboratorFn func(ctx context.Context, arg dbgen.GetTripByIDOrCollaboratorParams) (dbgen.Trip, error)
	createFeedbackFn            func(ctx context.Context, arg dbgen.CreateFeedbackParams) (dbgen.Feedback, error)

	createFeedbackCalls []dbgen.CreateFeedbackParams
}

func (s *stubFeedbackQueries) GetTripByIDOrCollaborator(ctx context.Context, arg dbgen.GetTripByIDOrCollaboratorParams) (dbgen.Trip, error) {
	if s.getTripByIDOrCollaboratorFn != nil {
		return s.getTripByIDOrCollaboratorFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubFeedbackQueries.GetTripByIDOrCollaborator(%+v) — set getTripByIDOrCollaboratorFn", arg)
	return dbgen.Trip{}, nil
}

func (s *stubFeedbackQueries) CreateFeedback(ctx context.Context, arg dbgen.CreateFeedbackParams) (dbgen.Feedback, error) {
	s.createFeedbackCalls = append(s.createFeedbackCalls, arg)
	if s.createFeedbackFn != nil {
		return s.createFeedbackFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubFeedbackQueries.CreateFeedback(%+v) — set createFeedbackFn", arg)
	return dbgen.Feedback{}, nil
}

// authedRequest builds a POST /api/feedback request with a valid Bearer
// token. The auth.Service is real (not mocked) — using the production
// validator with a known JWT secret keeps the test honest about how
// authenticateRESTRequest actually parses the header.
func authedRequest(t *testing.T, authSvc *auth.Service, userID uuid.UUID, body any) *http.Request {
	t.Helper()
	token, err := authSvc.GenerateAccessToken(userID)
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	var b []byte
	if body != nil {
		var mErr error
		b, mErr = json.Marshal(body)
		if mErr != nil {
			t.Fatalf("marshal body: %v", mErr)
		}
	}
	req := httptest.NewRequest(http.MethodPost, "/api/feedback", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	return req
}

func newTestAuth(t *testing.T) *auth.Service {
	t.Helper()
	return auth.NewService("test-client", "test-secret", "http://localhost/callback", "test-jwt-secret")
}

func newFeedbackTestHandler(authSvc *auth.Service, q *stubFeedbackQueries) *FeedbackHandler {
	return &FeedbackHandler{authSvc: authSvc, queries: q}
}

// ---------------------------------------------------------------------------
// HTTP method gate
// ---------------------------------------------------------------------------

func TestHandleSubmitFeedback_RejectsNonPost(t *testing.T) {
	authSvc := newTestAuth(t)
	q := &stubFeedbackQueries{tb: t}
	h := newFeedbackTestHandler(authSvc, q)

	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/api/feedback", nil)
		h.HandleSubmitFeedback(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("method %s: status = %d, want 405", method, rec.Code)
		}
	}
	if len(q.createFeedbackCalls) != 0 {
		t.Errorf("non-POST methods must NOT reach the DB; got %d CreateFeedback calls", len(q.createFeedbackCalls))
	}
}

// ---------------------------------------------------------------------------
// Auth gate
// ---------------------------------------------------------------------------

func TestHandleSubmitFeedback_RejectsMissingAuth(t *testing.T) {
	authSvc := newTestAuth(t)
	q := &stubFeedbackQueries{tb: t}
	h := newFeedbackTestHandler(authSvc, q)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/feedback", strings.NewReader(`{"message":"x"}`))
	h.HandleSubmitFeedback(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
	if len(q.createFeedbackCalls) != 0 {
		t.Errorf("unauth requests must NOT reach the DB; got %d CreateFeedback calls", len(q.createFeedbackCalls))
	}
}

func TestHandleSubmitFeedback_RejectsInvalidToken(t *testing.T) {
	authSvc := newTestAuth(t)
	q := &stubFeedbackQueries{tb: t}
	h := newFeedbackTestHandler(authSvc, q)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/feedback", strings.NewReader(`{"message":"x"}`))
	req.Header.Set("Authorization", "Bearer not-a-real-token")
	h.HandleSubmitFeedback(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// Body validation
// ---------------------------------------------------------------------------

func TestHandleSubmitFeedback_RejectsMalformedJSON(t *testing.T) {
	authSvc := newTestAuth(t)
	userID := uuid.New()
	q := &stubFeedbackQueries{tb: t}
	h := newFeedbackTestHandler(authSvc, q)

	token, _ := authSvc.GenerateAccessToken(userID)
	req := httptest.NewRequest(http.MethodPost, "/api/feedback", strings.NewReader(`not json`))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	h.HandleSubmitFeedback(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandleSubmitFeedback_RejectsEmptyMessage(t *testing.T) {
	// `message` is the one required field — the handler treats absence
	// of a message as a bad request, not a silent empty submission.
	authSvc := newTestAuth(t)
	userID := uuid.New()
	q := &stubFeedbackQueries{tb: t}
	h := newFeedbackTestHandler(authSvc, q)

	rec := httptest.NewRecorder()
	h.HandleSubmitFeedback(rec, authedRequest(t, authSvc, userID, map[string]any{
		"type": "bug",
	}))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	if len(q.createFeedbackCalls) != 0 {
		t.Errorf("empty-message requests must NOT reach the DB; got %d CreateFeedback calls", len(q.createFeedbackCalls))
	}
}

// ---------------------------------------------------------------------------
// Happy path + type defaulting
// ---------------------------------------------------------------------------

func TestHandleSubmitFeedback_HappyPath_StoresFeedback(t *testing.T) {
	authSvc := newTestAuth(t)
	userID := uuid.New()
	feedbackID := uuid.New()

	q := &stubFeedbackQueries{tb: t,
		createFeedbackFn: func(_ context.Context, arg dbgen.CreateFeedbackParams) (dbgen.Feedback, error) {
			if arg.UserID != userID {
				t.Errorf("user_id mismatch: got %s, want %s", arg.UserID, userID)
			}
			if arg.Type != "bug" {
				t.Errorf("type = %q, want bug", arg.Type)
			}
			if arg.Message != "App crashes on launch" {
				t.Errorf("message = %q, want 'App crashes on launch'", arg.Message)
			}
			return dbgen.Feedback{ID: feedbackID, UserID: userID, Type: arg.Type, Message: arg.Message}, nil
		},
	}
	h := newFeedbackTestHandler(authSvc, q)

	rec := httptest.NewRecorder()
	h.HandleSubmitFeedback(rec, authedRequest(t, authSvc, userID, map[string]any{
		"type":    "bug",
		"message": "App crashes on launch",
		"page":    "/trips",
	}))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if resp["status"] != "received" {
		t.Errorf("response.status = %q, want received", resp["status"])
	}
	if resp["id"] != feedbackID.String() {
		t.Errorf("response.id = %q, want %s", resp["id"], feedbackID.String())
	}

	if len(q.createFeedbackCalls) != 1 {
		t.Fatalf("expected 1 CreateFeedback call, got %d", len(q.createFeedbackCalls))
	}
	args := q.createFeedbackCalls[0]
	if !args.Page.Valid || args.Page.String != "/trips" {
		t.Errorf("page = %+v, want valid '/trips'", args.Page)
	}
	if args.TripID.Valid {
		t.Errorf("trip_id should be invalid when not supplied, got %+v", args.TripID)
	}
	if len(args.Context) != 0 {
		t.Errorf("context should be empty bytes when not supplied, got %d bytes", len(args.Context))
	}
}

func TestHandleSubmitFeedback_DefaultsTypeToGeneral(t *testing.T) {
	// Type is optional — handler defaults to "general" when omitted.
	// Pinning the default value because admin filtering keys off it.
	authSvc := newTestAuth(t)
	userID := uuid.New()

	q := &stubFeedbackQueries{tb: t,
		createFeedbackFn: func(_ context.Context, arg dbgen.CreateFeedbackParams) (dbgen.Feedback, error) {
			if arg.Type != "general" {
				t.Errorf("type = %q, want general (default)", arg.Type)
			}
			return dbgen.Feedback{ID: uuid.New(), UserID: userID, Type: arg.Type}, nil
		},
	}
	h := newFeedbackTestHandler(authSvc, q)

	rec := httptest.NewRecorder()
	h.HandleSubmitFeedback(rec, authedRequest(t, authSvc, userID, map[string]any{
		"message": "great app",
	}))
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestHandleSubmitFeedback_EncodesContextBlob(t *testing.T) {
	// The optional `context` map is JSON-marshalled and stored as bytes.
	// Pin the encoding so a future struct-tag change doesn't silently
	// break the admin dashboard's context-rendering.
	authSvc := newTestAuth(t)
	userID := uuid.New()

	q := &stubFeedbackQueries{tb: t,
		createFeedbackFn: func(_ context.Context, arg dbgen.CreateFeedbackParams) (dbgen.Feedback, error) {
			if len(arg.Context) == 0 {
				t.Error("context bytes should be populated when context map is supplied")
			}
			var parsed map[string]any
			if err := json.Unmarshal(arg.Context, &parsed); err != nil {
				t.Fatalf("context not valid JSON: %v", err)
			}
			if parsed["app_version"] != "1.2.3" {
				t.Errorf("context['app_version'] = %v, want 1.2.3", parsed["app_version"])
			}
			return dbgen.Feedback{ID: uuid.New(), UserID: userID, Type: arg.Type}, nil
		},
	}
	h := newFeedbackTestHandler(authSvc, q)

	rec := httptest.NewRecorder()
	h.HandleSubmitFeedback(rec, authedRequest(t, authSvc, userID, map[string]any{
		"type":    "bug",
		"message": "x",
		"context": map[string]any{
			"app_version": "1.2.3",
			"os":          "iOS 18.2",
		},
	}))
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// trip_id verification (the #361 P3 regression check)
// ---------------------------------------------------------------------------

func TestHandleSubmitFeedback_AttachesVerifiedTripID(t *testing.T) {
	// When the user IS the owner-or-collaborator of the trip, the
	// trip_id must be persisted on the feedback row.
	authSvc := newTestAuth(t)
	userID := uuid.New()
	tripID := uuid.New()

	q := &stubFeedbackQueries{tb: t,
		getTripByIDOrCollaboratorFn: func(_ context.Context, arg dbgen.GetTripByIDOrCollaboratorParams) (dbgen.Trip, error) {
			if arg.ID != tripID || arg.UserID != userID {
				t.Errorf("authz check called with %+v, expected ID=%s UserID=%s", arg, tripID, userID)
			}
			return dbgen.Trip{ID: tripID, UserID: userID}, nil
		},
		createFeedbackFn: func(_ context.Context, arg dbgen.CreateFeedbackParams) (dbgen.Feedback, error) {
			if !arg.TripID.Valid {
				t.Error("trip_id should be valid after authz check passes")
			}
			if arg.TripID.Bytes != [16]byte(tripID) {
				t.Errorf("trip_id mismatch: got %v, want %s", arg.TripID, tripID)
			}
			return dbgen.Feedback{ID: uuid.New(), UserID: userID, Type: arg.Type}, nil
		},
	}
	h := newFeedbackTestHandler(authSvc, q)

	rec := httptest.NewRecorder()
	h.HandleSubmitFeedback(rec, authedRequest(t, authSvc, userID, map[string]any{
		"type":    "bug",
		"message": "trip-specific bug",
		"trip_id": tripID.String(),
	}))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
}

func TestHandleSubmitFeedback_DropsUnverifiedTripID(t *testing.T) {
	// #361 P3 regression: if the user is NOT the owner-or-collaborator,
	// the handler must silently drop the trip_id (returning success
	// without it) rather than persisting feedback against another
	// user's trip. Admin dashboards filter by trip_id, so a misdirected
	// row would surface in the wrong trip's feedback view.
	authSvc := newTestAuth(t)
	userID := uuid.New()
	otherUsersTrip := uuid.New()

	q := &stubFeedbackQueries{tb: t,
		getTripByIDOrCollaboratorFn: func(_ context.Context, _ dbgen.GetTripByIDOrCollaboratorParams) (dbgen.Trip, error) {
			return dbgen.Trip{}, pgx.ErrNoRows
		},
		createFeedbackFn: func(_ context.Context, arg dbgen.CreateFeedbackParams) (dbgen.Feedback, error) {
			if arg.TripID.Valid {
				t.Errorf("trip_id should have been dropped (user is not owner/collaborator), got %+v", arg.TripID)
			}
			return dbgen.Feedback{ID: uuid.New(), UserID: userID, Type: arg.Type}, nil
		},
	}
	h := newFeedbackTestHandler(authSvc, q)

	rec := httptest.NewRecorder()
	h.HandleSubmitFeedback(rec, authedRequest(t, authSvc, userID, map[string]any{
		"type":    "bug",
		"message": "trying to attach to someone else's trip",
		"trip_id": otherUsersTrip.String(),
	}))
	// The handler returns 200 on the feedback even when trip_id is
	// dropped — feedback without a trip_id is still a valid submission.
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (silent drop, not refusal)", rec.Code)
	}
	if len(q.createFeedbackCalls) != 1 {
		t.Errorf("expected 1 CreateFeedback call, got %d", len(q.createFeedbackCalls))
	}
}

func TestHandleSubmitFeedback_DropsMalformedTripID(t *testing.T) {
	// A non-UUID trip_id never reaches the authz check — uuid.Parse
	// fails first and the trip_id is silently dropped (same outcome as
	// the unverified case). This documents the current lenient
	// behavior; if we ever tighten it to 400, this test fails loud.
	authSvc := newTestAuth(t)
	userID := uuid.New()

	q := &stubFeedbackQueries{tb: t,
		// getTripByIDOrCollaborator must NOT be called — leaving the
		// hook unset means a fail-loud Fatalf if it is.
		createFeedbackFn: func(_ context.Context, arg dbgen.CreateFeedbackParams) (dbgen.Feedback, error) {
			if arg.TripID.Valid {
				t.Errorf("trip_id should be dropped on uuid.Parse failure, got %+v", arg.TripID)
			}
			return dbgen.Feedback{ID: uuid.New(), UserID: userID, Type: arg.Type}, nil
		},
	}
	h := newFeedbackTestHandler(authSvc, q)

	rec := httptest.NewRecorder()
	h.HandleSubmitFeedback(rec, authedRequest(t, authSvc, userID, map[string]any{
		"type":    "bug",
		"message": "x",
		"trip_id": "not-a-uuid",
	}))
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// DB error paths
// ---------------------------------------------------------------------------

func TestHandleSubmitFeedback_CreateFeedbackErrorReturns500(t *testing.T) {
	authSvc := newTestAuth(t)
	userID := uuid.New()

	q := &stubFeedbackQueries{tb: t,
		createFeedbackFn: func(_ context.Context, _ dbgen.CreateFeedbackParams) (dbgen.Feedback, error) {
			return dbgen.Feedback{}, errors.New("db down")
		},
	}
	h := newFeedbackTestHandler(authSvc, q)

	rec := httptest.NewRecorder()
	h.HandleSubmitFeedback(rec, authedRequest(t, authSvc, userID, map[string]any{
		"type":    "general",
		"message": "x",
	}))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}
