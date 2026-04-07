package csrf

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

var okHandler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
})

// addAuthCookie attaches a fake toqui_access cookie so the CSRF middleware
// treats the request as a cookie-based session and applies the Origin/Referer
// checks. Bearer-token (cookie-less) requests bypass CSRF entirely (#179).
func addAuthCookie(req *http.Request) {
	req.AddCookie(&http.Cookie{Name: "toqui_access", Value: "test-token"})
}

func TestSafeMethodsPassThrough(t *testing.T) {
	h := Middleware(okHandler, []string{"https://app.example.com"}, nil)

	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		req := httptest.NewRequest(method, "/api/data", nil)
		req.Header.Set("Origin", "https://evil.example.com") // should be ignored
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: expected 200, got %d", method, rec.Code)
		}
	}
}

func TestAllowedOrigin(t *testing.T) {
	h := Middleware(okHandler, []string{"https://app.example.com"}, nil)

	req := httptest.NewRequest(http.MethodPost, "/auth/exchange", nil)
	req.Header.Set("Origin", "https://app.example.com")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestAllowedOriginCaseInsensitive(t *testing.T) {
	h := Middleware(okHandler, []string{"https://App.Example.Com"}, nil)

	req := httptest.NewRequest(http.MethodPost, "/waitlist", nil)
	req.Header.Set("Origin", "https://app.example.com")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestUntrustedOriginRejected(t *testing.T) {
	h := Middleware(okHandler, []string{"https://app.example.com"}, nil)

	req := httptest.NewRequest(http.MethodPost, "/auth/exchange", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	addAuthCookie(req)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}

func TestNullOriginRejected(t *testing.T) {
	h := Middleware(okHandler, []string{"https://app.example.com"}, nil)

	req := httptest.NewRequest(http.MethodPost, "/waitlist", nil)
	req.Header.Set("Origin", "null")
	addAuthCookie(req)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for null origin, got %d", rec.Code)
	}
}

func TestRefererFallback(t *testing.T) {
	h := Middleware(okHandler, []string{"https://app.example.com"}, nil)

	// No Origin, but trusted Referer — should pass.
	req := httptest.NewRequest(http.MethodPost, "/waitlist", nil)
	req.Header.Set("Referer", "https://app.example.com/some-page")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 with trusted referer, got %d", rec.Code)
	}
}

func TestUntrustedRefererRejected(t *testing.T) {
	h := Middleware(okHandler, []string{"https://app.example.com"}, nil)

	req := httptest.NewRequest(http.MethodPost, "/waitlist", nil)
	req.Header.Set("Referer", "https://evil.example.com/attack")
	addAuthCookie(req)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 with untrusted referer, got %d", rec.Code)
	}
}

func TestNoOriginNoRefererWithCookieRejected(t *testing.T) {
	h := Middleware(okHandler, []string{"https://app.example.com"}, nil)

	// Cookie-authenticated request with no Origin/Referer is still rejected
	// because the browser would normally send Origin on POST/PUT/DELETE.
	req := httptest.NewRequest(http.MethodPost, "/waitlist", nil)
	addAuthCookie(req)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for cookie-auth no-origin request, got %d", rec.Code)
	}
}

// Bearer-token clients (no auth cookie) bypass CSRF entirely. CSRF only
// applies to cookie-based sessions where the browser auto-attaches cookies
// to cross-origin POSTs (#179).
func TestBearerClientNoCookieBypassesCSRF(t *testing.T) {
	h := Middleware(okHandler, []string{"https://app.example.com"}, nil)

	// No auth cookie, no Origin, no Referer — should pass through.
	req := httptest.NewRequest(http.MethodPost, "/toqui.v1.TripService/ListTrips", nil)
	req.Header.Set("Authorization", "Bearer fake-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for bearer-only client, got %d", rec.Code)
	}
}

func TestBearerClientUntrustedOriginBypassesCSRF(t *testing.T) {
	h := Middleware(okHandler, []string{"https://app.example.com"}, nil)

	// No auth cookie even with an "untrusted" origin (e.g. a curl test that
	// fakes Origin) — bypass, because there's no cookie for an attacker to
	// hijack.
	req := httptest.NewRequest(http.MethodPost, "/toqui.v1.TripService/ListTrips", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for bearer-only client even with untrusted origin, got %d", rec.Code)
	}
}

func TestExemptPrefixes(t *testing.T) {
	h := Middleware(okHandler, []string{"https://app.example.com"}, []string{"/webhooks/"})

	// Webhook path with untrusted origin — should pass (exempt).
	req := httptest.NewRequest(http.MethodPost, "/webhooks/email/inbound", nil)
	req.Header.Set("Origin", "https://sendgrid.example.com")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for exempt webhook path, got %d", rec.Code)
	}
}

func TestMultipleAllowedOrigins(t *testing.T) {
	h := Middleware(okHandler, []string{
		"https://app.example.com",
		"http://localhost:3000",
	}, nil)

	for _, origin := range []string{"https://app.example.com", "http://localhost:3000"} {
		req := httptest.NewRequest(http.MethodPost, "/api/data", nil)
		req.Header.Set("Origin", origin)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("origin %s: expected 200, got %d", origin, rec.Code)
		}
	}
}

func TestDeleteAndPutProtected(t *testing.T) {
	h := Middleware(okHandler, []string{"https://app.example.com"}, nil)

	for _, method := range []string{http.MethodPut, http.MethodDelete, "PATCH"} {
		req := httptest.NewRequest(method, "/api/data", nil)
		req.Header.Set("Origin", "https://evil.example.com")
		addAuthCookie(req)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s: expected 403, got %d", method, rec.Code)
		}
	}
}
