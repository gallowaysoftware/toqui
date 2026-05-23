package booking

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/gallowaysoftware/toqui/backend/internal/ai"
	"github.com/gallowaysoftware/toqui/backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui/backend/internal/trip"
)

// ---------------------------------------------------------------------------
// stubQueries — fail-loud test double for bookingQueries
// ---------------------------------------------------------------------------
//
// Same pattern as payment/subscription/trip/lifecycle stubQueries.
// Every method calls tb.Fatalf when called without an injected `*Fn`.

type stubQueries struct {
	tb testing.TB

	findBookingByConfirmationCodeFn     func(ctx context.Context, arg dbgen.FindBookingByConfirmationCodeParams) (dbgen.Booking, error)
	findBookingFuzzyFn                  func(ctx context.Context, arg dbgen.FindBookingFuzzyParams) (dbgen.Booking, error)
	mergeBookingFn                      func(ctx context.Context, arg dbgen.MergeBookingParams) (dbgen.Booking, error)
	createBookingForOwnerOrEditorFn     func(ctx context.Context, arg dbgen.CreateBookingForOwnerOrEditorParams) (dbgen.Booking, error)
	updateBookingFn                     func(ctx context.Context, arg dbgen.UpdateBookingParams) (dbgen.Booking, error)
	getTripCostSummaryFn                func(ctx context.Context, arg dbgen.GetTripCostSummaryParams) ([]dbgen.GetTripCostSummaryRow, error)
	listBookingsByTripFn                func(ctx context.Context, arg dbgen.ListBookingsByTripParams) ([]dbgen.Booking, error)
	getBookingByIDFn                    func(ctx context.Context, arg dbgen.GetBookingByIDParams) (dbgen.Booking, error)
	deleteBookingFn                     func(ctx context.Context, arg dbgen.DeleteBookingParams) (int64, error)
	linkBookingToTripForOwnerOrEditorFn func(ctx context.Context, arg dbgen.LinkBookingToTripForOwnerOrEditorParams) (dbgen.Booking, error)
	getTripByIDFn                       func(ctx context.Context, arg dbgen.GetTripByIDParams) (dbgen.Trip, error)
	listItineraryItemsByTripFn          func(ctx context.Context, tripID uuid.UUID) ([]dbgen.ItineraryItem, error)

	createBookingCalls []dbgen.CreateBookingForOwnerOrEditorParams
	deleteBookingCalls []dbgen.DeleteBookingParams
}

func (s *stubQueries) FindBookingByConfirmationCode(ctx context.Context, arg dbgen.FindBookingByConfirmationCodeParams) (dbgen.Booking, error) {
	if s.findBookingByConfirmationCodeFn != nil {
		return s.findBookingByConfirmationCodeFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubQueries.FindBookingByConfirmationCode(%+v) — set findBookingByConfirmationCodeFn", arg)
	return dbgen.Booking{}, nil
}

func (s *stubQueries) FindBookingFuzzy(ctx context.Context, arg dbgen.FindBookingFuzzyParams) (dbgen.Booking, error) {
	if s.findBookingFuzzyFn != nil {
		return s.findBookingFuzzyFn(ctx, arg)
	}
	// Default: no fuzzy match found. This keeps existing tests passing without
	// requiring every test to opt into fuzzy lookup.
	return dbgen.Booking{}, pgx.ErrNoRows
}

func (s *stubQueries) MergeBooking(ctx context.Context, arg dbgen.MergeBookingParams) (dbgen.Booking, error) {
	if s.mergeBookingFn != nil {
		return s.mergeBookingFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubQueries.MergeBooking(%+v) — set mergeBookingFn", arg)
	return dbgen.Booking{}, nil
}

func (s *stubQueries) CreateBookingForOwnerOrEditor(ctx context.Context, arg dbgen.CreateBookingForOwnerOrEditorParams) (dbgen.Booking, error) {
	s.createBookingCalls = append(s.createBookingCalls, arg)
	if s.createBookingForOwnerOrEditorFn != nil {
		return s.createBookingForOwnerOrEditorFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubQueries.CreateBookingForOwnerOrEditor(%+v) — set createBookingForOwnerOrEditorFn", arg)
	return dbgen.Booking{}, nil
}

func (s *stubQueries) UpdateBooking(ctx context.Context, arg dbgen.UpdateBookingParams) (dbgen.Booking, error) {
	if s.updateBookingFn != nil {
		return s.updateBookingFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubQueries.UpdateBooking(%+v) — set updateBookingFn", arg)
	return dbgen.Booking{}, nil
}

func (s *stubQueries) GetTripCostSummary(ctx context.Context, arg dbgen.GetTripCostSummaryParams) ([]dbgen.GetTripCostSummaryRow, error) {
	if s.getTripCostSummaryFn != nil {
		return s.getTripCostSummaryFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubQueries.GetTripCostSummary(%+v) — set getTripCostSummaryFn", arg)
	return nil, nil
}

func (s *stubQueries) ListBookingsByTrip(ctx context.Context, arg dbgen.ListBookingsByTripParams) ([]dbgen.Booking, error) {
	if s.listBookingsByTripFn != nil {
		return s.listBookingsByTripFn(ctx, arg)
	}
	// Soft default: an empty list is correct for tests that don't care
	// about post-ingest analysis. Originally fail-loud, but the post-ingest
	// path (DetectConflicts + AnalyzeCoverage) now calls this after every
	// successful ingest as a best-effort feedback fetch — making it
	// fail-loud here would force every ingest test to opt in. Tests that
	// want to assert on listing behavior set listBookingsByTripFn.
	return nil, nil
}

func (s *stubQueries) GetBookingByID(ctx context.Context, arg dbgen.GetBookingByIDParams) (dbgen.Booking, error) {
	if s.getBookingByIDFn != nil {
		return s.getBookingByIDFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubQueries.GetBookingByID(%+v) — set getBookingByIDFn", arg)
	return dbgen.Booking{}, nil
}

func (s *stubQueries) DeleteBooking(ctx context.Context, arg dbgen.DeleteBookingParams) (int64, error) {
	s.deleteBookingCalls = append(s.deleteBookingCalls, arg)
	if s.deleteBookingFn != nil {
		return s.deleteBookingFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubQueries.DeleteBooking(%+v) — set deleteBookingFn", arg)
	return 0, nil
}

func (s *stubQueries) LinkBookingToTripForOwnerOrEditor(ctx context.Context, arg dbgen.LinkBookingToTripForOwnerOrEditorParams) (dbgen.Booking, error) {
	if s.linkBookingToTripForOwnerOrEditorFn != nil {
		return s.linkBookingToTripForOwnerOrEditorFn(ctx, arg)
	}
	s.tb.Fatalf("unexpected stubQueries.LinkBookingToTripForOwnerOrEditor(%+v) — set linkBookingToTripForOwnerOrEditorFn", arg)
	return dbgen.Booking{}, nil
}

// GetTripByID and ListItineraryItemsByTrip back the post-ingest analysis
// (DetectConflicts + AnalyzeCoverage). Both default to "no data" so
// existing tests that don't care about post-ingest feedback don't have to
// opt in. Tests that want to exercise the analysis can set the *Fn fields.
func (s *stubQueries) GetTripByID(ctx context.Context, arg dbgen.GetTripByIDParams) (dbgen.Trip, error) {
	if s.getTripByIDFn != nil {
		return s.getTripByIDFn(ctx, arg)
	}
	return dbgen.Trip{ID: arg.ID, UserID: arg.UserID}, nil
}

func (s *stubQueries) ListItineraryItemsByTrip(ctx context.Context, tripID uuid.UUID) ([]dbgen.ItineraryItem, error) {
	if s.listItineraryItemsByTripFn != nil {
		return s.listItineraryItemsByTripFn(ctx, tripID)
	}
	return nil, nil
}

// stubAIProvider satisfies ai.Provider. Each test injects a chatStream
// function that emits the events it wants the service code to see.
type stubAIProvider struct {
	tb         testing.TB
	chatStream func(ctx context.Context, req *ai.ChatRequest) (<-chan ai.Event, error)
}

func (p *stubAIProvider) ChatStream(ctx context.Context, req *ai.ChatRequest) (<-chan ai.Event, error) {
	if p.chatStream != nil {
		return p.chatStream(ctx, req)
	}
	p.tb.Fatalf("unexpected stubAIProvider.ChatStream(...) — set chatStream")
	return nil, nil
}

func (p *stubAIProvider) Name() string { return "stub" }

// streamingResponse is a tiny helper that returns an event channel
// emitting the given text as a single TextDelta then closing. Most
// booking AI calls collapse the streamed response into a single string
// before parsing, so emitting it as one delta is faithful to what the
// service code does in production.
func streamingResponse(text string) func(ctx context.Context, req *ai.ChatRequest) (<-chan ai.Event, error) {
	return func(ctx context.Context, req *ai.ChatRequest) (<-chan ai.Event, error) {
		ch := make(chan ai.Event, 2)
		ch <- ai.Event{Type: ai.EventTextDelta, Text: text}
		close(ch)
		return ch, nil
	}
}

func newTestService(q *stubQueries, p *stubAIProvider) *Service {
	return &Service{queries: q, aiProvider: p}
}

// ---------------------------------------------------------------------------
// normalizeBookingType
// ---------------------------------------------------------------------------

func TestNormalizeBookingType_Known(t *testing.T) {
	for _, in := range []string{"flight", "hotel", "vacation_rental", "car_rental", "train", "tour", "activity", "restaurant", "ferry", "bus", "cruise", "transfer", "other"} {
		if got := normalizeBookingType(in); got != in {
			t.Errorf("normalizeBookingType(%q) = %q, want %q", in, got, in)
		}
	}
}

func TestNormalizeBookingType_CaseInsensitive(t *testing.T) {
	if got := normalizeBookingType("FLIGHT"); got != "flight" {
		t.Errorf("uppercase should normalize to lowercase, got %q", got)
	}
	if got := normalizeBookingType("  Hotel  "); got != "hotel" {
		t.Errorf("whitespace + casing should normalize, got %q", got)
	}
}

func TestNormalizeBookingType_UnknownMapsToOther(t *testing.T) {
	// AI hallucinations like "spacecraft" or "submarine" must not
	// land in the DB as arbitrary strings — they map to "other".
	for _, in := range []string{"spacecraft", "submarine", "", "unknown_thing"} {
		if got := normalizeBookingType(in); got != "other" {
			t.Errorf("unknown %q should map to 'other', got %q", in, got)
		}
	}
}

// ---------------------------------------------------------------------------
// stripCodeFences
// ---------------------------------------------------------------------------

func TestStripCodeFences_NoFence(t *testing.T) {
	if got := stripCodeFences(`{"a": 1}`); got != `{"a": 1}` {
		t.Errorf("plain JSON should be unchanged, got %q", got)
	}
}

func TestStripCodeFences_JSONFence(t *testing.T) {
	// Common AI output: ```json\n...\n```
	got := stripCodeFences("```json\n{\"a\": 1}\n```")
	if got != `{"a": 1}` {
		t.Errorf("expected fence stripped, got %q", got)
	}
}

func TestStripCodeFences_BareFence(t *testing.T) {
	// Some models use ``` without a language tag.
	got := stripCodeFences("```\n{\"a\": 1}\n```")
	if got != `{"a": 1}` {
		t.Errorf("expected bare fence stripped, got %q", got)
	}
}

func TestStripCodeFences_LeadingWhitespace(t *testing.T) {
	got := stripCodeFences("  \n```json\n{\"a\": 1}\n```\n  ")
	if got != `{"a": 1}` {
		t.Errorf("expected leading/trailing whitespace stripped, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// IngestText / ingest — the AI parse → DB insert pipeline
// ---------------------------------------------------------------------------

func TestIngestText_HappyPath(t *testing.T) {
	userID := uuid.New()
	tripUUID := uuid.New()
	resultID := uuid.New()

	parsedJSON := `{
		"type": "hotel",
		"confirmation_code": "ABC123",
		"provider": "Hilton",
		"title": "Hilton Tokyo",
		"price_cents": 25000,
		"currency": "USD",
		"details": {"hotel_name": "Hilton Tokyo"}
	}`

	q := &stubQueries{tb: t,
		findBookingByConfirmationCodeFn: func(_ context.Context, _ dbgen.FindBookingByConfirmationCodeParams) (dbgen.Booking, error) {
			return dbgen.Booking{}, pgx.ErrNoRows // no duplicate
		},
		createBookingForOwnerOrEditorFn: func(_ context.Context, arg dbgen.CreateBookingForOwnerOrEditorParams) (dbgen.Booking, error) {
			// Pin a few invariants on what gets written.
			if arg.UserID != userID {
				t.Errorf("UserID: got %s, want %s", arg.UserID, userID)
			}
			if arg.Type != "hotel" {
				t.Errorf("Type: got %q, want hotel", arg.Type)
			}
			if !arg.ConfirmationCode.Valid || arg.ConfirmationCode.String != "ABC123" {
				t.Errorf("ConfirmationCode: got %+v", arg.ConfirmationCode)
			}
			if arg.Source != "paste" {
				t.Errorf("Source: got %q, want paste (IngestText)", arg.Source)
			}
			return dbgen.Booking{ID: resultID, Type: "hotel"}, nil
		},
	}
	p := &stubAIProvider{tb: t, chatStream: streamingResponse(parsedJSON)}
	svc := newTestService(q, p)

	got, err := svc.IngestText(context.Background(), userID, tripUUID.String(), "hotel", "raw text")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Booking.ID != resultID {
		t.Errorf("expected returned booking id=%s, got %s", resultID, got.Booking.ID)
	}
}

func TestIngestEmail_PassesEmailSource(t *testing.T) {
	parsedJSON := `{"type": "flight", "confirmation_code": "F1"}`
	q := &stubQueries{tb: t,
		findBookingByConfirmationCodeFn: func(_ context.Context, _ dbgen.FindBookingByConfirmationCodeParams) (dbgen.Booking, error) {
			return dbgen.Booking{}, pgx.ErrNoRows
		},
		createBookingForOwnerOrEditorFn: func(_ context.Context, arg dbgen.CreateBookingForOwnerOrEditorParams) (dbgen.Booking, error) {
			if arg.Source != "email" {
				t.Errorf("IngestEmail must set Source=email, got %q", arg.Source)
			}
			return dbgen.Booking{ID: uuid.New()}, nil
		},
	}
	p := &stubAIProvider{tb: t, chatStream: streamingResponse(parsedJSON)}
	svc := newTestService(q, p)

	if _, err := svc.IngestEmail(context.Background(), uuid.New(), uuid.New().String(), "", "raw"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestIngest_DuplicateConfirmationCodeReturnsExisting(t *testing.T) {
	// Same user + same trip + same confirmation code → return the
	// existing row, do NOT create a new one. The `createBooking*Fn`
	// being unset makes this fail-loud if the function tries to
	// insert.
	existingID := uuid.New()
	parsedJSON := `{"type": "hotel", "confirmation_code": "DUP123"}`
	q := &stubQueries{tb: t,
		findBookingByConfirmationCodeFn: func(_ context.Context, _ dbgen.FindBookingByConfirmationCodeParams) (dbgen.Booking, error) {
			return dbgen.Booking{ID: existingID, Type: "hotel"}, nil
		},
		mergeBookingFn: func(_ context.Context, arg dbgen.MergeBookingParams) (dbgen.Booking, error) {
			if arg.ID != existingID {
				t.Errorf("MergeBooking called with id=%s, want %s", arg.ID, existingID)
			}
			return dbgen.Booking{ID: existingID, Type: "hotel"}, nil
		},
		// createBookingForOwnerOrEditorFn deliberately not set —
		// triggers fail-loud if duplicate-detection is broken.
	}
	p := &stubAIProvider{tb: t, chatStream: streamingResponse(parsedJSON)}
	svc := newTestService(q, p)

	got, err := svc.IngestText(context.Background(), uuid.New(), uuid.New().String(), "", "raw")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Booking.ID != existingID {
		t.Errorf("expected existing booking %s, got %s", existingID, got.Booking.ID)
	}
	if !got.WasUpdated {
		t.Errorf("expected WasUpdated=true on duplicate-code merge")
	}
	if got.PreviousID != existingID.String() {
		t.Errorf("expected PreviousID=%s, got %q", existingID, got.PreviousID)
	}
	if len(q.createBookingCalls) != 0 {
		t.Errorf("must not call CreateBooking on duplicate, got %d calls", len(q.createBookingCalls))
	}
}

func TestIngest_AuthzGateMapsErrNoRowsToErrNotOwnerOrEditor(t *testing.T) {
	// THE #361 P1 regression test. The ingest path uses a gated query
	// that returns pgx.ErrNoRows when the caller doesn't own/can't
	// edit the trip. Service must convert this to
	// trip.ErrNotOwnerOrEditor so the handler maps it to
	// PermissionDenied. Regression: a previous version of this code
	// silently created bookings on victim trip UUIDs.
	parsedJSON := `{"type": "hotel"}`
	q := &stubQueries{tb: t,
		findBookingByConfirmationCodeFn: func(_ context.Context, _ dbgen.FindBookingByConfirmationCodeParams) (dbgen.Booking, error) {
			return dbgen.Booking{}, pgx.ErrNoRows
		},
		createBookingForOwnerOrEditorFn: func(_ context.Context, _ dbgen.CreateBookingForOwnerOrEditorParams) (dbgen.Booking, error) {
			return dbgen.Booking{}, pgx.ErrNoRows
		},
	}
	p := &stubAIProvider{tb: t, chatStream: streamingResponse(parsedJSON)}
	svc := newTestService(q, p)

	_, err := svc.IngestText(context.Background(), uuid.New(), uuid.New().String(), "", "raw")
	if !errors.Is(err, trip.ErrNotOwnerOrEditor) {
		t.Errorf("expected trip.ErrNotOwnerOrEditor, got %v", err)
	}
}

func TestIngest_OtherCreateErrorWrapped(t *testing.T) {
	// Inverse of the authz test — non-pgx.ErrNoRows must NOT match
	// trip.ErrNotOwnerOrEditor (a future refactor that returned the
	// sentinel for any error would silently flip 500s to 403s and
	// hide the underlying DB issue).
	parsedJSON := `{"type": "hotel"}`
	wantErr := errors.New("constraint violation")
	q := &stubQueries{tb: t,
		findBookingByConfirmationCodeFn: func(_ context.Context, _ dbgen.FindBookingByConfirmationCodeParams) (dbgen.Booking, error) {
			return dbgen.Booking{}, pgx.ErrNoRows
		},
		createBookingForOwnerOrEditorFn: func(_ context.Context, _ dbgen.CreateBookingForOwnerOrEditorParams) (dbgen.Booking, error) {
			return dbgen.Booking{}, wantErr
		},
	}
	p := &stubAIProvider{tb: t, chatStream: streamingResponse(parsedJSON)}
	svc := newTestService(q, p)

	_, err := svc.IngestText(context.Background(), uuid.New(), uuid.New().String(), "", "raw")
	if errors.Is(err, trip.ErrNotOwnerOrEditor) {
		t.Errorf("non-pgx.ErrNoRows must NOT match ErrNotOwnerOrEditor, got %v", err)
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("expected wantErr to wrap, got %v", err)
	}
}

func TestIngest_AIErrorPropagates(t *testing.T) {
	q := &stubQueries{tb: t}
	p := &stubAIProvider{tb: t,
		chatStream: func(_ context.Context, _ *ai.ChatRequest) (<-chan ai.Event, error) {
			return nil, errors.New("ai down")
		},
	}
	svc := newTestService(q, p)

	_, err := svc.IngestText(context.Background(), uuid.New(), uuid.New().String(), "", "raw")
	if err == nil {
		t.Fatal("expected error when AI provider fails")
	}
	if !strings.Contains(err.Error(), "parse booking") {
		t.Errorf("expected error wrapped with 'parse booking', got %v", err)
	}
}

func TestIngest_AIStreamErrorPropagates(t *testing.T) {
	q := &stubQueries{tb: t}
	p := &stubAIProvider{tb: t,
		chatStream: func(_ context.Context, _ *ai.ChatRequest) (<-chan ai.Event, error) {
			ch := make(chan ai.Event, 1)
			ch <- ai.Event{Type: ai.EventError, Error: errors.New("stream broke")}
			close(ch)
			return ch, nil
		},
	}
	svc := newTestService(q, p)

	_, err := svc.IngestText(context.Background(), uuid.New(), uuid.New().String(), "", "raw")
	if err == nil || !strings.Contains(err.Error(), "stream") {
		t.Errorf("expected stream error to propagate, got %v", err)
	}
}

func TestIngest_MalformedJSONResponseErrors(t *testing.T) {
	q := &stubQueries{tb: t}
	p := &stubAIProvider{tb: t, chatStream: streamingResponse("not json at all")}
	svc := newTestService(q, p)

	_, err := svc.IngestText(context.Background(), uuid.New(), uuid.New().String(), "", "raw")
	if err == nil || !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("expected unmarshal error, got %v", err)
	}
}

func TestIngest_AIHallucinatedTypeMapsToOther(t *testing.T) {
	// AI response has type=spacecraft. normalizeBookingType maps to
	// "other" before insert. Verify the create-call argument.
	parsedJSON := `{"type": "spacecraft", "confirmation_code": "X"}`
	q := &stubQueries{tb: t,
		findBookingByConfirmationCodeFn: func(_ context.Context, _ dbgen.FindBookingByConfirmationCodeParams) (dbgen.Booking, error) {
			return dbgen.Booking{}, pgx.ErrNoRows
		},
		createBookingForOwnerOrEditorFn: func(_ context.Context, arg dbgen.CreateBookingForOwnerOrEditorParams) (dbgen.Booking, error) {
			if arg.Type != "other" {
				t.Errorf("AI hallucinated type should be normalized to 'other', got %q", arg.Type)
			}
			return dbgen.Booking{ID: uuid.New()}, nil
		},
	}
	p := &stubAIProvider{tb: t, chatStream: streamingResponse(parsedJSON)}
	svc := newTestService(q, p)

	if _, err := svc.IngestText(context.Background(), uuid.New(), uuid.New().String(), "", "raw"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestIngest_FencedJSONResponseStripped(t *testing.T) {
	// Some models wrap JSON in ```json fences. The stripCodeFences
	// pre-parser should handle it — pin via the integration-style
	// path rather than relying on the unit test for stripCodeFences
	// alone.
	parsedJSON := "```json\n" + `{"type": "flight", "confirmation_code": "F"}` + "\n```"
	q := &stubQueries{tb: t,
		findBookingByConfirmationCodeFn: func(_ context.Context, _ dbgen.FindBookingByConfirmationCodeParams) (dbgen.Booking, error) {
			return dbgen.Booking{}, pgx.ErrNoRows
		},
		createBookingForOwnerOrEditorFn: func(_ context.Context, arg dbgen.CreateBookingForOwnerOrEditorParams) (dbgen.Booking, error) {
			if arg.Type != "flight" {
				t.Errorf("expected fenced JSON to parse correctly, got Type=%q", arg.Type)
			}
			return dbgen.Booking{ID: uuid.New()}, nil
		},
	}
	p := &stubAIProvider{tb: t, chatStream: streamingResponse(parsedJSON)}
	svc := newTestService(q, p)

	if _, err := svc.IngestText(context.Background(), uuid.New(), uuid.New().String(), "", "raw"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestIngest_DifferentUserSameConfirmationCodeNoMerge regresses the
// authorization invariant in FindBookingByConfirmationCode: the filter
// is `user_id = $1 AND trip_id = $2 AND confirmation_code = $3`, so a
// different user's matching code MUST return ErrNoRows (no cross-user
// merge). This test pins the contract from the service side — if a
// future schema migration drops the user_id predicate, this fails.
func TestIngest_DifferentUserSameConfirmationCodeNoMerge(t *testing.T) {
	parsedJSON := `{"type": "hotel", "confirmation_code": "SHARED-CODE"}`
	var capturedFindParams dbgen.FindBookingByConfirmationCodeParams
	q := &stubQueries{tb: t,
		findBookingByConfirmationCodeFn: func(_ context.Context, arg dbgen.FindBookingByConfirmationCodeParams) (dbgen.Booking, error) {
			capturedFindParams = arg
			return dbgen.Booking{}, pgx.ErrNoRows
		},
		createBookingForOwnerOrEditorFn: func(_ context.Context, _ dbgen.CreateBookingForOwnerOrEditorParams) (dbgen.Booking, error) {
			return dbgen.Booking{ID: uuid.New(), Type: "hotel"}, nil
		},
	}
	p := &stubAIProvider{tb: t, chatStream: streamingResponse(parsedJSON)}
	svc := newTestService(q, p)

	userA := uuid.New()
	tripID := uuid.New()
	if _, err := svc.IngestText(context.Background(), userA, tripID.String(), "", "raw"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedFindParams.UserID != userA {
		t.Errorf("FindBookingByConfirmationCode must filter on caller's UserID, got %s want %s", capturedFindParams.UserID, userA)
	}
	if capturedFindParams.TripID.Bytes != tripID || !capturedFindParams.TripID.Valid {
		t.Errorf("FindBookingByConfirmationCode must filter on caller's TripID, got %+v want %s", capturedFindParams.TripID, tripID)
	}
}

// TestIngest_FuzzyMatchSucceeds_MergesIntoCandidate exercises the fuzzy
// path which the original test suite never set findBookingFuzzyFn for —
// dead-code-tested before this regression added explicit coverage.
func TestIngest_FuzzyMatchSucceeds_MergesIntoCandidate(t *testing.T) {
	candidateID := uuid.New()
	parsedJSON := `{"type": "hotel", "provider": "Hilton Tokyo", "start_time": "2026-06-15T14:00:00Z"}`
	q := &stubQueries{tb: t,
		// No exact code match → falls through to fuzzy path.
		findBookingByConfirmationCodeFn: func(_ context.Context, _ dbgen.FindBookingByConfirmationCodeParams) (dbgen.Booking, error) {
			return dbgen.Booking{}, pgx.ErrNoRows
		},
		findBookingFuzzyFn: func(_ context.Context, _ dbgen.FindBookingFuzzyParams) (dbgen.Booking, error) {
			return dbgen.Booking{
				ID:       candidateID,
				Type:     "hotel",
				Provider: pgtype.Text{String: "Hilton", Valid: true},
			}, nil
		},
		mergeBookingFn: func(_ context.Context, arg dbgen.MergeBookingParams) (dbgen.Booking, error) {
			if arg.ID != candidateID {
				t.Errorf("fuzzy merge called for id=%s, want %s", arg.ID, candidateID)
			}
			return dbgen.Booking{ID: candidateID, Type: "hotel"}, nil
		},
	}
	p := &stubAIProvider{tb: t, chatStream: streamingResponse(parsedJSON)}
	svc := newTestService(q, p)

	got, err := svc.IngestText(context.Background(), uuid.New(), uuid.New().String(), "", "raw")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.WasUpdated || got.PreviousID != candidateID.String() {
		t.Errorf("expected fuzzy merge result, got %+v", got)
	}
}

// TestIngest_FuzzyMatchProviderMismatch_FallsThroughToCreate covers the
// tightening branch in findFuzzyMatch where a candidate is rejected on
// provider similarity. The flow must continue to the create path
// rather than returning the wrong booking.
func TestIngest_FuzzyMatchProviderMismatch_FallsThroughToCreate(t *testing.T) {
	parsedJSON := `{"type": "flight", "provider": "Delta", "start_time": "2026-06-15T14:00:00Z"}`
	q := &stubQueries{tb: t,
		findBookingByConfirmationCodeFn: func(_ context.Context, _ dbgen.FindBookingByConfirmationCodeParams) (dbgen.Booking, error) {
			return dbgen.Booking{}, pgx.ErrNoRows
		},
		findBookingFuzzyFn: func(_ context.Context, _ dbgen.FindBookingFuzzyParams) (dbgen.Booking, error) {
			// Same date+type but a totally different carrier.
			return dbgen.Booking{
				ID:       uuid.New(),
				Type:     "flight",
				Provider: pgtype.Text{String: "United", Valid: true},
			}, nil
		},
		createBookingForOwnerOrEditorFn: func(_ context.Context, _ dbgen.CreateBookingForOwnerOrEditorParams) (dbgen.Booking, error) {
			return dbgen.Booking{ID: uuid.New(), Type: "flight"}, nil
		},
		// mergeBookingFn deliberately unset — calling it would mean we
		// merged across carriers, which would fail-loud.
	}
	p := &stubAIProvider{tb: t, chatStream: streamingResponse(parsedJSON)}
	svc := newTestService(q, p)

	if _, err := svc.IngestText(context.Background(), uuid.New(), uuid.New().String(), "", "raw"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestIngest_FuzzyMatchDBErrorPropagates regresses the original error-
// swallow behaviour where ANY error from FindBookingFuzzy was treated
// as no-match and the service silently created a duplicate. Now,
// non-ErrNoRows errors must propagate to the caller.
func TestIngest_FuzzyMatchDBErrorPropagates(t *testing.T) {
	parsedJSON := `{"type": "hotel", "provider": "Hilton", "start_time": "2026-06-15T14:00:00Z"}`
	wantErr := errors.New("connection reset")
	q := &stubQueries{tb: t,
		findBookingByConfirmationCodeFn: func(_ context.Context, _ dbgen.FindBookingByConfirmationCodeParams) (dbgen.Booking, error) {
			return dbgen.Booking{}, pgx.ErrNoRows
		},
		findBookingFuzzyFn: func(_ context.Context, _ dbgen.FindBookingFuzzyParams) (dbgen.Booking, error) {
			return dbgen.Booking{}, wantErr
		},
		// createBookingForOwnerOrEditorFn deliberately unset — fail-loud
		// if the service falls through to create on a transient error.
	}
	p := &stubAIProvider{tb: t, chatStream: streamingResponse(parsedJSON)}
	svc := newTestService(q, p)

	_, err := svc.IngestText(context.Background(), uuid.New(), uuid.New().String(), "", "raw")
	if !errors.Is(err, wantErr) {
		t.Errorf("expected fuzzy DB error to propagate, got %v", err)
	}
}

// TestIngest_RaceRecoveryOnUniqueViolation regresses the TOCTOU window
// between SELECT-by-code and INSERT. Two concurrent requests both saw
// no row and both tried to INSERT; the unique partial index on
// (user_id, trip_id, confirmation_code) makes one of them fail with
// 23505. The service must catch that, re-fetch the row that won the
// race, and merge into it instead of bubbling an opaque DB error.
func TestIngest_RaceRecoveryOnUniqueViolation(t *testing.T) {
	winnerID := uuid.New()
	parsedJSON := `{"type": "hotel", "confirmation_code": "RACE-CODE"}`
	findCalls := 0
	q := &stubQueries{tb: t,
		findBookingByConfirmationCodeFn: func(_ context.Context, _ dbgen.FindBookingByConfirmationCodeParams) (dbgen.Booking, error) {
			findCalls++
			if findCalls == 1 {
				// First call (the dedup SELECT before INSERT) — no row yet.
				return dbgen.Booking{}, pgx.ErrNoRows
			}
			// Second call (post-23505 re-fetch) — the winner is there.
			return dbgen.Booking{ID: winnerID, Type: "hotel"}, nil
		},
		createBookingForOwnerOrEditorFn: func(_ context.Context, _ dbgen.CreateBookingForOwnerOrEditorParams) (dbgen.Booking, error) {
			return dbgen.Booking{}, &pgconn.PgError{Code: "23505", ConstraintName: "bookings_user_trip_confirmation_unique"}
		},
		mergeBookingFn: func(_ context.Context, arg dbgen.MergeBookingParams) (dbgen.Booking, error) {
			if arg.ID != winnerID {
				t.Errorf("race recovery merge id=%s, want %s", arg.ID, winnerID)
			}
			return dbgen.Booking{ID: winnerID, Type: "hotel"}, nil
		},
	}
	p := &stubAIProvider{tb: t, chatStream: streamingResponse(parsedJSON)}
	svc := newTestService(q, p)

	got, err := svc.IngestText(context.Background(), uuid.New(), uuid.New().String(), "", "raw")
	if err != nil {
		t.Fatalf("expected race recovery to succeed, got error: %v", err)
	}
	if !got.WasUpdated || got.PreviousID != winnerID.String() {
		t.Errorf("expected race recovery to surface winner via merge, got %+v", got)
	}
	if findCalls != 2 {
		t.Errorf("expected exactly 2 FindBookingByConfirmationCode calls (SELECT + race recovery), got %d", findCalls)
	}
}

// TestIngest_PreservesUserEditsOnMerge regresses the silent data-loss
// bug where a re-imported confirmation email overwrote a user's
// manual edits. The MergeBooking SQL WHERE clause refuses to update
// rows whose updated_at has moved past created_at + 1s; pgx returns
// ErrNoRows for that case and the service must surface the existing
// (untouched) record with WasUpdated=false.
func TestIngest_PreservesUserEditsOnMerge(t *testing.T) {
	editedID := uuid.New()
	editedTitle := "Hilton Tokyo - anniversary trip"
	parsedJSON := `{"type": "hotel", "confirmation_code": "EDITED", "title": "Hilton Tokyo"}`
	q := &stubQueries{tb: t,
		findBookingByConfirmationCodeFn: func(_ context.Context, _ dbgen.FindBookingByConfirmationCodeParams) (dbgen.Booking, error) {
			return dbgen.Booking{ID: editedID, Type: "hotel", Title: editedTitle}, nil
		},
		mergeBookingFn: func(_ context.Context, _ dbgen.MergeBookingParams) (dbgen.Booking, error) {
			// SQL guard rejects the update because updated_at >
			// created_at + 1s — pgx returns ErrNoRows.
			return dbgen.Booking{}, pgx.ErrNoRows
		},
		// createBookingForOwnerOrEditorFn deliberately unset — must NOT
		// fall through to create when the dedup SELECT already found
		// a (user-edited) row.
	}
	p := &stubAIProvider{tb: t, chatStream: streamingResponse(parsedJSON)}
	svc := newTestService(q, p)

	got, err := svc.IngestText(context.Background(), uuid.New(), uuid.New().String(), "", "raw")
	if err != nil {
		t.Fatalf("expected user-edit preservation to succeed, got error: %v", err)
	}
	if got.WasUpdated {
		t.Errorf("expected WasUpdated=false when user-edit guard fires, got %+v", got)
	}
	if got.PreviousID != editedID.String() {
		t.Errorf("expected PreviousID=%s, got %q", editedID, got.PreviousID)
	}
	if got.Booking.Title != editedTitle {
		t.Errorf("expected user's edited title preserved (%q), got %q", editedTitle, got.Booking.Title)
	}
}

// TestIngest_PassesTripIDToMerge confirms B5 defense-in-depth: the
// service must pass the tripUUID through to MergeBookingParams.TripID
// so the SQL WHERE clause can enforce trip_id parity even if a future
// SELECT-phase change relaxed the trip_id filter.
func TestIngest_PassesTripIDToMerge(t *testing.T) {
	parsedJSON := `{"type": "hotel", "confirmation_code": "TID-CHECK"}`
	tripID := uuid.New()
	existingID := uuid.New()
	var capturedMerge dbgen.MergeBookingParams
	q := &stubQueries{tb: t,
		findBookingByConfirmationCodeFn: func(_ context.Context, _ dbgen.FindBookingByConfirmationCodeParams) (dbgen.Booking, error) {
			return dbgen.Booking{ID: existingID, Type: "hotel"}, nil
		},
		mergeBookingFn: func(_ context.Context, arg dbgen.MergeBookingParams) (dbgen.Booking, error) {
			capturedMerge = arg
			return dbgen.Booking{ID: existingID, Type: "hotel"}, nil
		},
	}
	p := &stubAIProvider{tb: t, chatStream: streamingResponse(parsedJSON)}
	svc := newTestService(q, p)

	if _, err := svc.IngestText(context.Background(), uuid.New(), tripID.String(), "", "raw"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !capturedMerge.TripID.Valid || capturedMerge.TripID.Bytes != tripID {
		t.Errorf("MergeBookingParams.TripID not propagated: got %+v want %s", capturedMerge.TripID, tripID)
	}
}

// TestProviderSimilar covers the dedup-gate matcher. Most of these cases
// were previous false positives from the strings.Contains era — "Air"
// matching "AirBnB" silently merged a hotel into a flight; "Hilton Tokyo"
// matching "Hilton Osaka" silently merged two hotels on a multi-city trip.
func TestProviderSimilar(t *testing.T) {
	cases := []struct {
		name string
		a, b string
		want bool
	}{
		// Allowed: natural extensions of the same brand.
		{"identical", "Delta", "Delta", true},
		{"prefix extension", "BC Ferries", "BC Ferries Ltd", true},
		{"reverse prefix extension", "BC Ferries Ltd", "BC Ferries", true},
		{"short vs long airline", "Delta", "Delta Air Lines", true},
		{"case difference", "delta", "Delta", true},
		{"punctuation difference", "BC Ferries, Inc.", "BC Ferries", true},
		{"hyphen vs space", "Air-Canada", "Air Canada", true},

		// Rejected: substring matches across different brands.
		{"air vs airbnb", "Air", "AirBnB", false},
		{"airbnb vs air canada", "AirBnB", "Air Canada", false},

		// Rejected: same first token but divergent rest (multi-city
		// false positives).
		{"hilton tokyo vs hilton osaka", "Hilton Tokyo", "Hilton Osaka", false},
		{"hotel arts vs hotel indigo", "Hotel Arts", "Hotel Indigo", false},

		// Rejected: totally different carriers.
		{"delta vs united", "Delta", "United", false},

		// Edge case: empty / non-alphanumeric-only inputs are treated
		// as "no opinion" so callers can rely on their own emptiness
		// gating without us flipping the answer back to false.
		{"empty a", "", "Delta", true},
		{"empty b", "Delta", "", true},
		{"both empty", "", "", true},
		{"non-alphanumeric only", "***", "Delta", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := providerSimilar(tc.a, tc.b); got != tc.want {
				t.Errorf("providerSimilar(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// CRUD methods
// ---------------------------------------------------------------------------

func TestUpdate_HappyPath(t *testing.T) {
	userID := uuid.New()
	bookingID := uuid.New()
	q := &stubQueries{tb: t,
		updateBookingFn: func(_ context.Context, arg dbgen.UpdateBookingParams) (dbgen.Booking, error) {
			if arg.ID != bookingID || arg.UserID != userID {
				t.Errorf("Update params mismatch: got %+v, want id=%s user=%s", arg, bookingID, userID)
			}
			return dbgen.Booking{ID: bookingID, Title: "Updated"}, nil
		},
	}
	svc := newTestService(q, &stubAIProvider{tb: t})

	got, err := svc.Update(context.Background(), userID, bookingID, dbgen.UpdateBookingParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Title != "Updated" {
		t.Errorf("expected updated booking, got %+v", got)
	}
}

func TestUpdate_ErrorWrapped(t *testing.T) {
	wantErr := errors.New("not found")
	q := &stubQueries{tb: t,
		updateBookingFn: func(_ context.Context, _ dbgen.UpdateBookingParams) (dbgen.Booking, error) {
			return dbgen.Booking{}, wantErr
		},
	}
	svc := newTestService(q, &stubAIProvider{tb: t})

	_, err := svc.Update(context.Background(), uuid.New(), uuid.New(), dbgen.UpdateBookingParams{})
	if !errors.Is(err, wantErr) {
		t.Errorf("expected wantErr to wrap, got %v", err)
	}
}

func TestGetByID_HappyPath(t *testing.T) {
	userID := uuid.New()
	bookingID := uuid.New()
	q := &stubQueries{tb: t,
		getBookingByIDFn: func(_ context.Context, arg dbgen.GetBookingByIDParams) (dbgen.Booking, error) {
			if arg.ID != bookingID || arg.UserID != userID {
				t.Errorf("GetBookingByID params mismatch: got %+v", arg)
			}
			return dbgen.Booking{ID: bookingID}, nil
		},
	}
	svc := newTestService(q, &stubAIProvider{tb: t})

	got, err := svc.GetByID(context.Background(), userID, bookingID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != bookingID {
		t.Errorf("got %s, want %s", got.ID, bookingID)
	}
}

func TestGetByID_NotFoundWrapped(t *testing.T) {
	q := &stubQueries{tb: t,
		getBookingByIDFn: func(_ context.Context, _ dbgen.GetBookingByIDParams) (dbgen.Booking, error) {
			return dbgen.Booking{}, pgx.ErrNoRows
		},
	}
	svc := newTestService(q, &stubAIProvider{tb: t})

	_, err := svc.GetByID(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("expected pgx.ErrNoRows to wrap, got %v", err)
	}
}

func TestListByTrip_PassesParams(t *testing.T) {
	userID := uuid.New()
	tripID := uuid.New()
	q := &stubQueries{tb: t,
		listBookingsByTripFn: func(_ context.Context, arg dbgen.ListBookingsByTripParams) ([]dbgen.Booking, error) {
			if arg.UserID != userID {
				t.Errorf("UserID: got %s, want %s", arg.UserID, userID)
			}
			if arg.TripID.Bytes != tripID || !arg.TripID.Valid {
				t.Errorf("TripID: got %+v, want valid bytes=%s", arg.TripID, tripID)
			}
			return []dbgen.Booking{{ID: uuid.New()}, {ID: uuid.New()}}, nil
		},
	}
	svc := newTestService(q, &stubAIProvider{tb: t})

	got, err := svc.ListByTrip(context.Background(), userID, tripID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 bookings, got %d", len(got))
	}
}

func TestDelete_RowsAffected(t *testing.T) {
	// HTTP idempotent DELETE semantics: a non-existent ID returns
	// (false, nil), not an error. This is the contract the handler
	// relies on for the audit-miss path.
	cases := []struct {
		name     string
		rows     int64
		expected bool
	}{
		{"existing row deleted", 1, true},
		{"missing row no-op", 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q := &stubQueries{tb: t,
				deleteBookingFn: func(_ context.Context, _ dbgen.DeleteBookingParams) (int64, error) {
					return tc.rows, nil
				},
			}
			svc := newTestService(q, &stubAIProvider{tb: t})

			deleted, err := svc.Delete(context.Background(), uuid.New(), uuid.New())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if deleted != tc.expected {
				t.Errorf("Delete returned %v, want %v", deleted, tc.expected)
			}
		})
	}
}

func TestDelete_DBErrorPropagates(t *testing.T) {
	wantErr := errors.New("db down")
	q := &stubQueries{tb: t,
		deleteBookingFn: func(_ context.Context, _ dbgen.DeleteBookingParams) (int64, error) {
			return 0, wantErr
		},
	}
	svc := newTestService(q, &stubAIProvider{tb: t})

	_, err := svc.Delete(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, wantErr) {
		t.Errorf("expected wantErr to wrap, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// LinkToTrip — the other #361 P1 authz site
// ---------------------------------------------------------------------------

func TestLinkToTrip_AuthzGateMapsErrNoRowsToErrNotOwnerOrEditor(t *testing.T) {
	// The original LinkBookingToTrip checked booking ownership but
	// not trip edit rights, so any user could re-associate their own
	// booking with a victim's trip. The gated query closes the gap
	// by requiring both — predicate miss → ErrNoRows → service-side
	// conversion to trip.ErrNotOwnerOrEditor.
	q := &stubQueries{tb: t,
		linkBookingToTripForOwnerOrEditorFn: func(_ context.Context, _ dbgen.LinkBookingToTripForOwnerOrEditorParams) (dbgen.Booking, error) {
			return dbgen.Booking{}, pgx.ErrNoRows
		},
	}
	svc := newTestService(q, &stubAIProvider{tb: t})

	_, err := svc.LinkToTrip(context.Background(), uuid.New(), uuid.New(), uuid.New())
	if !errors.Is(err, trip.ErrNotOwnerOrEditor) {
		t.Errorf("expected trip.ErrNotOwnerOrEditor, got %v", err)
	}
}

func TestLinkToTrip_OtherErrorWrapped(t *testing.T) {
	wantErr := errors.New("constraint violation")
	q := &stubQueries{tb: t,
		linkBookingToTripForOwnerOrEditorFn: func(_ context.Context, _ dbgen.LinkBookingToTripForOwnerOrEditorParams) (dbgen.Booking, error) {
			return dbgen.Booking{}, wantErr
		},
	}
	svc := newTestService(q, &stubAIProvider{tb: t})

	_, err := svc.LinkToTrip(context.Background(), uuid.New(), uuid.New(), uuid.New())
	if errors.Is(err, trip.ErrNotOwnerOrEditor) {
		t.Errorf("non-pgx.ErrNoRows must NOT match ErrNotOwnerOrEditor")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("expected wantErr to wrap, got %v", err)
	}
}

func TestLinkToTrip_HappyPath(t *testing.T) {
	userID := uuid.New()
	bookingID := uuid.New()
	tripID := uuid.New()
	q := &stubQueries{tb: t,
		linkBookingToTripForOwnerOrEditorFn: func(_ context.Context, arg dbgen.LinkBookingToTripForOwnerOrEditorParams) (dbgen.Booking, error) {
			if arg.ID != bookingID || arg.UserID != userID {
				t.Errorf("Link params mismatch: %+v", arg)
			}
			if arg.TripID.Bytes != tripID || !arg.TripID.Valid {
				t.Errorf("TripID mismatch: %+v", arg.TripID)
			}
			return dbgen.Booking{ID: bookingID, TripID: pgtype.UUID{Bytes: tripID, Valid: true}}, nil
		},
	}
	svc := newTestService(q, &stubAIProvider{tb: t})

	got, err := svc.LinkToTrip(context.Background(), userID, bookingID, tripID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != bookingID {
		t.Errorf("got %s, want %s", got.ID, bookingID)
	}
}

// ---------------------------------------------------------------------------
// GetTripCostSummary
// ---------------------------------------------------------------------------

func TestGetTripCostSummary_MapsRowsToCostSummary(t *testing.T) {
	userID := uuid.New()
	tripID := uuid.New()
	q := &stubQueries{tb: t,
		getTripCostSummaryFn: func(_ context.Context, _ dbgen.GetTripCostSummaryParams) ([]dbgen.GetTripCostSummaryRow, error) {
			return []dbgen.GetTripCostSummaryRow{
				{Currency: "USD", TotalCents: 10000, BookingCount: 2},
				{Currency: "EUR", TotalCents: 5000, BookingCount: 1},
			}, nil
		},
	}
	svc := newTestService(q, &stubAIProvider{tb: t})

	got, err := svc.GetTripCostSummary(context.Background(), userID, tripID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 currencies, got %d", len(got))
	}
	if got[0].Currency != "USD" || got[0].TotalCents != 10000 || got[0].BookingCount != 2 {
		t.Errorf("USD row mapped wrong: %+v", got[0])
	}
}

func TestGetTripCostSummary_DBErrorPropagates(t *testing.T) {
	wantErr := errors.New("db down")
	q := &stubQueries{tb: t,
		getTripCostSummaryFn: func(_ context.Context, _ dbgen.GetTripCostSummaryParams) ([]dbgen.GetTripCostSummaryRow, error) {
			return nil, wantErr
		},
	}
	svc := newTestService(q, &stubAIProvider{tb: t})

	_, err := svc.GetTripCostSummary(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, wantErr) {
		t.Errorf("expected wantErr to wrap, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// ExtractField
// ---------------------------------------------------------------------------

func TestExtractField_HappyPath(t *testing.T) {
	q := &stubQueries{tb: t,
		getBookingByIDFn: func(_ context.Context, _ dbgen.GetBookingByIDParams) (dbgen.Booking, error) {
			return dbgen.Booking{
				ID:        uuid.New(),
				RawSource: pgtype.Text{String: "Hilton Tokyo, check-in 2026-06-15", Valid: true},
			}, nil
		},
	}
	p := &stubAIProvider{tb: t,
		chatStream: streamingResponse(`{"answer": "2026-06-15", "extracted_fields": {"check_in": "2026-06-15"}}`),
	}
	svc := newTestService(q, p)

	got, err := svc.ExtractField(context.Background(), uuid.New(), uuid.New(), "When is check-in?")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Answer != "2026-06-15" {
		t.Errorf("Answer: got %q, want 2026-06-15", got.Answer)
	}
	if got.ExtractedFields["check_in"] != "2026-06-15" {
		t.Errorf("ExtractedFields: got %+v", got.ExtractedFields)
	}
}

func TestExtractField_NoRawSourceErrors(t *testing.T) {
	// A booking created without an email/paste raw source can't be
	// re-extracted from. Pin the explicit error rather than letting
	// the AI call run with an empty prompt.
	q := &stubQueries{tb: t,
		getBookingByIDFn: func(_ context.Context, _ dbgen.GetBookingByIDParams) (dbgen.Booking, error) {
			return dbgen.Booking{ID: uuid.New(), RawSource: pgtype.Text{Valid: false}}, nil
		},
	}
	svc := newTestService(q, &stubAIProvider{tb: t})

	_, err := svc.ExtractField(context.Background(), uuid.New(), uuid.New(), "anything")
	if err == nil || !strings.Contains(err.Error(), "no raw source") {
		t.Errorf("expected 'no raw source' error, got %v", err)
	}
}

func TestExtractField_BookingNotFoundErrors(t *testing.T) {
	q := &stubQueries{tb: t,
		getBookingByIDFn: func(_ context.Context, _ dbgen.GetBookingByIDParams) (dbgen.Booking, error) {
			return dbgen.Booking{}, pgx.ErrNoRows
		},
	}
	svc := newTestService(q, &stubAIProvider{tb: t})

	_, err := svc.ExtractField(context.Background(), uuid.New(), uuid.New(), "anything")
	if err == nil {
		t.Fatal("expected error when booking is missing")
	}
}

func TestExtractField_NonJSONResponseUsedAsAnswer(t *testing.T) {
	// If the AI returns plain text instead of JSON, the function
	// degrades to using the whole response as the Answer (with a
	// slog.Warn). Pin this graceful degradation.
	q := &stubQueries{tb: t,
		getBookingByIDFn: func(_ context.Context, _ dbgen.GetBookingByIDParams) (dbgen.Booking, error) {
			return dbgen.Booking{
				ID:        uuid.New(),
				RawSource: pgtype.Text{String: "raw", Valid: true},
			}, nil
		},
	}
	p := &stubAIProvider{tb: t, chatStream: streamingResponse("just a plain text answer")}
	svc := newTestService(q, p)

	got, err := svc.ExtractField(context.Background(), uuid.New(), uuid.New(), "anything")
	if err != nil {
		t.Fatalf("unexpected error (non-JSON should degrade gracefully): %v", err)
	}
	if got.Answer != "just a plain text answer" {
		t.Errorf("expected raw text as Answer, got %q", got.Answer)
	}
}

// ---------------------------------------------------------------------------
// Post-ingest analysis (DetectConflicts + AnalyzeCoverage attached to the
// IngestResult by attachPostIngestAnalysis).
// ---------------------------------------------------------------------------

func TestIngest_AttachesConflictsAndCoverage(t *testing.T) {
	// After ingest, the service fetches the trip's bookings and runs the
	// detectors. Here we set up a tight-layover scenario so DetectConflicts
	// has something to flag, and a flight-without-hotel scenario for
	// AnalyzeCoverage. Both should appear on the result.
	userID := uuid.New()
	tripUUID := uuid.New()

	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	flightA := dbgen.Booking{
		ID:                uuid.New(),
		Type:              "flight",
		DepartureLocation: pgtype.Text{String: "JFK", Valid: true},
		ArrivalLocation:   pgtype.Text{String: "FRA", Valid: true},
		StartTime:         pgtype.Timestamptz{Time: base.Add(-7 * time.Hour), Valid: true},
		EndTime:           pgtype.Timestamptz{Time: base, Valid: true},
	}
	flightB := dbgen.Booking{
		ID:                uuid.New(),
		Type:              "flight",
		DepartureLocation: pgtype.Text{String: "FRA", Valid: true},
		ArrivalLocation:   pgtype.Text{String: "ATH", Valid: true},
		StartTime:         pgtype.Timestamptz{Time: base.Add(45 * time.Minute), Valid: true},
		EndTime:           pgtype.Timestamptz{Time: base.Add(45*time.Minute + 3*time.Hour), Valid: true},
	}

	parsedJSON := `{"type": "flight", "confirmation_code": "FX1"}`

	q := &stubQueries{tb: t,
		findBookingByConfirmationCodeFn: func(_ context.Context, _ dbgen.FindBookingByConfirmationCodeParams) (dbgen.Booking, error) {
			return dbgen.Booking{}, pgx.ErrNoRows
		},
		createBookingForOwnerOrEditorFn: func(_ context.Context, _ dbgen.CreateBookingForOwnerOrEditorParams) (dbgen.Booking, error) {
			return flightA, nil
		},
		listBookingsByTripFn: func(_ context.Context, _ dbgen.ListBookingsByTripParams) ([]dbgen.Booking, error) {
			// Return both flights so the tight_layover check fires AND
			// no_accommodation fires (flights but no hotel).
			return []dbgen.Booking{flightA, flightB}, nil
		},
	}
	p := &stubAIProvider{tb: t, chatStream: streamingResponse(parsedJSON)}
	svc := newTestService(q, p)

	got, err := svc.IngestText(context.Background(), userID, tripUUID.String(), "flight", "raw")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Conflicts) == 0 {
		t.Errorf("expected post-ingest conflicts, got none")
	}
	foundLayover := false
	for _, c := range got.Conflicts {
		if c.Type == ConflictTightLayover {
			foundLayover = true
		}
	}
	if !foundLayover {
		t.Errorf("expected tight_layover conflict, got %+v", got.Conflicts)
	}
	if got.CoverageGap == nil {
		t.Errorf("expected post-ingest coverage gap (no accommodation), got nil")
	} else if got.CoverageGap.Type != GapNoAccommodation {
		t.Errorf("expected no_accommodation gap, got %s", got.CoverageGap.Type)
	}
}

func TestIngest_UnattachedBookingSkipsAnalysis(t *testing.T) {
	// Bookings without a trip_id should not trigger the post-ingest
	// analysis (nothing to analyze against). The list query must NOT be
	// called for unattached bookings.
	parsedJSON := `{"type": "flight", "confirmation_code": "FX2"}`

	listCalled := false
	q := &stubQueries{tb: t,
		findBookingByConfirmationCodeFn: func(_ context.Context, _ dbgen.FindBookingByConfirmationCodeParams) (dbgen.Booking, error) {
			return dbgen.Booking{}, pgx.ErrNoRows
		},
		createBookingForOwnerOrEditorFn: func(_ context.Context, _ dbgen.CreateBookingForOwnerOrEditorParams) (dbgen.Booking, error) {
			return dbgen.Booking{ID: uuid.New(), Type: "flight"}, nil
		},
		listBookingsByTripFn: func(_ context.Context, _ dbgen.ListBookingsByTripParams) ([]dbgen.Booking, error) {
			listCalled = true
			return nil, nil
		},
	}
	p := &stubAIProvider{tb: t, chatStream: streamingResponse(parsedJSON)}
	svc := newTestService(q, p)

	got, err := svc.IngestText(context.Background(), uuid.New(), "" /* no trip */, "flight", "raw")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if listCalled {
		t.Errorf("ListBookingsByTrip must not be called for unattached bookings")
	}
	if len(got.Conflicts) != 0 {
		t.Errorf("expected no conflicts on unattached booking, got %+v", got.Conflicts)
	}
	if got.CoverageGap != nil {
		t.Errorf("expected no coverage gap on unattached booking, got %+v", got.CoverageGap)
	}
}

func TestIngest_ListBookingsErrorIsBestEffort(t *testing.T) {
	// If ListBookingsByTrip fails, the ingest still succeeds — analysis is
	// best-effort. The result has no conflicts/gap but the booking is
	// returned successfully.
	parsedJSON := `{"type": "flight", "confirmation_code": "FX3"}`
	resultID := uuid.New()
	tripUUID := uuid.New()

	q := &stubQueries{tb: t,
		findBookingByConfirmationCodeFn: func(_ context.Context, _ dbgen.FindBookingByConfirmationCodeParams) (dbgen.Booking, error) {
			return dbgen.Booking{}, pgx.ErrNoRows
		},
		createBookingForOwnerOrEditorFn: func(_ context.Context, _ dbgen.CreateBookingForOwnerOrEditorParams) (dbgen.Booking, error) {
			return dbgen.Booking{ID: resultID, Type: "flight"}, nil
		},
		listBookingsByTripFn: func(_ context.Context, _ dbgen.ListBookingsByTripParams) ([]dbgen.Booking, error) {
			return nil, fmt.Errorf("simulated DB failure")
		},
	}
	p := &stubAIProvider{tb: t, chatStream: streamingResponse(parsedJSON)}
	svc := newTestService(q, p)

	got, err := svc.IngestText(context.Background(), uuid.New(), tripUUID.String(), "flight", "raw")
	if err != nil {
		t.Fatalf("post-ingest analysis errors must not fail the ingest: %v", err)
	}
	if got.Booking == nil || got.Booking.ID != resultID {
		t.Errorf("expected booking returned despite analysis failure")
	}
	if len(got.Conflicts) != 0 || got.CoverageGap != nil {
		t.Errorf("expected empty analysis on DB failure, got conflicts=%d gap=%+v", len(got.Conflicts), got.CoverageGap)
	}
}
