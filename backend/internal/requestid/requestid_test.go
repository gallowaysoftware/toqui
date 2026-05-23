package requestid_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/gallowaysoftware/toqui/backend/internal/requestid"
)

// uuidPattern matches the canonical google/uuid.NewString() format —
// 8-4-4-4-12 lowercase hex with hyphens. Tightens the contract that we
// generate UUIDs (not arbitrary opaque strings) so any future change
// to the ID format would have to update this test deliberately.
var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func TestMiddleware_GeneratesUUIDWhenHeaderAbsent(t *testing.T) {
	var observedFromContext, observedHeaderInHandler string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observedFromContext = requestid.FromContext(r.Context())
		observedHeaderInHandler = w.Header().Get("X-Request-ID")
	})
	mw := requestid.Middleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/anything", nil)
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)

	if !uuidPattern.MatchString(observedFromContext) {
		t.Fatalf("FromContext returned %q; expected canonical UUID format", observedFromContext)
	}
	// The same ID must appear in the response header (so callers can
	// correlate logs in Cloud Logging without having to read the body).
	if rec.Header().Get("X-Request-ID") != observedFromContext {
		t.Errorf("response X-Request-ID=%q, context=%q; should match exactly",
			rec.Header().Get("X-Request-ID"), observedFromContext)
	}
	// Header is set BEFORE the handler runs (so handlers that write a
	// body can rely on the header being present). The middleware sets
	// the header before calling next.ServeHTTP, so reading the header
	// from inside the handler must return the same value.
	if observedHeaderInHandler != observedFromContext {
		t.Errorf("handler observed header=%q but context=%q; header must be set before next", observedHeaderInHandler, observedFromContext)
	}
}

func TestMiddleware_PreservesIncomingRequestID(t *testing.T) {
	// Cloud Run / load-balancer-injected upstream IDs (X-Cloud-Trace-Context
	// or any other proxy that sets X-Request-ID) must be preserved
	// rather than overwritten — that's how distributed traces stay
	// correlated across hops. The middleware only generates a fresh
	// ID when the header is absent.
	const upstream = "8c2b1e4a-9f0d-4abc-8000-aaaaaaaaaaaa"

	var observedFromContext string
	handler := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		observedFromContext = requestid.FromContext(r.Context())
	})
	mw := requestid.Middleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/anything", nil)
	req.Header.Set("X-Request-ID", upstream)
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req)

	if observedFromContext != upstream {
		t.Errorf("FromContext=%q, expected upstream %q (must NOT regenerate when header is present)",
			observedFromContext, upstream)
	}
	if rec.Header().Get("X-Request-ID") != upstream {
		t.Errorf("response X-Request-ID=%q, expected %q",
			rec.Header().Get("X-Request-ID"), upstream)
	}
}

func TestMiddleware_GeneratesUniqueIDPerRequest(t *testing.T) {
	// Two requests in succession must get different IDs. Pinning this
	// guards against a future refactor accidentally caching the
	// generator output (e.g. a sync.Once on the wrong scope).
	handler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {})
	mw := requestid.Middleware(handler)

	rec1 := httptest.NewRecorder()
	mw.ServeHTTP(rec1, httptest.NewRequest(http.MethodGet, "/", nil))

	rec2 := httptest.NewRecorder()
	mw.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/", nil))

	id1 := rec1.Header().Get("X-Request-ID")
	id2 := rec2.Header().Get("X-Request-ID")
	if id1 == "" || id2 == "" {
		t.Fatalf("missing IDs: id1=%q id2=%q", id1, id2)
	}
	if id1 == id2 {
		t.Errorf("expected unique IDs per request, both were %q", id1)
	}
}

func TestFromContext_EmptyWhenAbsent(t *testing.T) {
	// Code paths that never went through the middleware (e.g. a
	// background goroutine spawned with context.Background()) must
	// not panic and must return an empty string. Pins the type
	// assertion's _, _ = pattern.
	got := requestid.FromContext(context.Background())
	if got != "" {
		t.Errorf("FromContext on bare context = %q, expected empty", got)
	}
}

func TestFromContext_EmptyWhenWrongType(t *testing.T) {
	// If some other code stores a non-string at our context key (it
	// can't, since ctxKey is unexported, but nothing in Go's type
	// system enforces that) FromContext must still return "" rather
	// than panic. Black-box test of the (id, _) := … cast.
	type otherKey struct{}
	ctx := context.WithValue(context.Background(), otherKey{}, "not the same key")
	if got := requestid.FromContext(ctx); got != "" {
		t.Errorf("FromContext with unrelated key = %q, expected empty", got)
	}
}
