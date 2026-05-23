package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gallowaysoftware/toqui/backend/internal/auth"
	"github.com/gallowaysoftware/toqui/backend/internal/middleware"
)

// captureHandler records the Authorization header value the middleware
// chain forwarded — that's the only observable effect of CookieAuth.
type captureHandler struct {
	got string
}

func (h *captureHandler) ServeHTTP(_ http.ResponseWriter, r *http.Request) {
	h.got = r.Header.Get("Authorization")
}

func TestCookieAuth_AuthorizationHeaderTakesPrecedence(t *testing.T) {
	// Native apps and ConnectRPC clients set Authorization: Bearer
	// directly. The middleware MUST NOT overwrite an existing header
	// even if a cookie is also present — that would let a stale or
	// mismatched cookie clobber an explicit native-token.
	cap := &captureHandler{}
	mw := middleware.CookieAuth(cap)

	req := httptest.NewRequest(http.MethodGet, "/api/anything", nil)
	req.Header.Set("Authorization", "Bearer native-token-from-app")
	req.AddCookie(&http.Cookie{Name: auth.AccessTokenCookie, Value: "stale-cookie-token"})

	mw.ServeHTTP(httptest.NewRecorder(), req)

	if cap.got != "Bearer native-token-from-app" {
		t.Errorf("Authorization = %q, want Bearer native-token-from-app (cookie should NOT override)", cap.got)
	}
}

func TestCookieAuth_CookieFillsMissingAuthorization(t *testing.T) {
	// Web browser path: server-set HttpOnly cookie, no Authorization
	// header (browsers can't easily set arbitrary headers cross-origin).
	// Middleware bridges the cookie value into the Bearer header so
	// downstream interceptors and REST handlers see the same shape
	// regardless of client type.
	cap := &captureHandler{}
	mw := middleware.CookieAuth(cap)

	req := httptest.NewRequest(http.MethodGet, "/api/anything", nil)
	req.AddCookie(&http.Cookie{Name: auth.AccessTokenCookie, Value: "browser-cookie-token"})

	mw.ServeHTTP(httptest.NewRecorder(), req)

	if cap.got != "Bearer browser-cookie-token" {
		t.Errorf("Authorization = %q, want Bearer browser-cookie-token", cap.got)
	}
}

func TestCookieAuth_NoCookieNoHeader_PassThrough(t *testing.T) {
	// Anonymous request (e.g. /healthz, /livez, /shared/{token}). The
	// middleware must NOT synthesize an Authorization header — a
	// blank request must remain blank so downstream auth gates can
	// correctly classify it as unauthenticated rather than seeing a
	// "Bearer " prefix that's actually empty.
	cap := &captureHandler{}
	mw := middleware.CookieAuth(cap)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)

	mw.ServeHTTP(httptest.NewRecorder(), req)

	if cap.got != "" {
		t.Errorf("Authorization = %q, want empty (anonymous request must not gain a header)", cap.got)
	}
}

func TestCookieAuth_EmptyCookieValue_IsTreatedAsAbsent(t *testing.T) {
	// A cookie with an empty value is semantically the same as no
	// cookie at all. Auth.AccessTokenFromCookie returns "" for both
	// cases; CookieAuth must NOT set "Bearer " (a header with just
	// the prefix and no token) since downstream code splits on space
	// and would treat that as a malformed token rather than no auth.
	cap := &captureHandler{}
	mw := middleware.CookieAuth(cap)

	req := httptest.NewRequest(http.MethodGet, "/api/anything", nil)
	req.AddCookie(&http.Cookie{Name: auth.AccessTokenCookie, Value: ""})

	mw.ServeHTTP(httptest.NewRecorder(), req)

	if cap.got != "" {
		t.Errorf("Authorization = %q, want empty (empty cookie ≡ no cookie)", cap.got)
	}
}

func TestCookieAuth_DifferentCookieName_Ignored(t *testing.T) {
	// Cookies with names other than AccessTokenCookie (e.g. third-party
	// analytics cookies, the refresh-token cookie at /auth path scope,
	// OAuth state cookies) must be ignored. Tightens the contract that
	// only the specific access-token cookie is bridged to Authorization.
	cap := &captureHandler{}
	mw := middleware.CookieAuth(cap)

	req := httptest.NewRequest(http.MethodGet, "/api/anything", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "not-an-access-token"})
	req.AddCookie(&http.Cookie{Name: "toqui_refresh", Value: "refresh-token-not-access"})

	mw.ServeHTTP(httptest.NewRecorder(), req)

	if cap.got != "" {
		t.Errorf("Authorization = %q, want empty (only AccessTokenCookie should be bridged)", cap.got)
	}
}
