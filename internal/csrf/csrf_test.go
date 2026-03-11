package csrf

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

var okHandler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
})

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
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 with untrusted referer, got %d", rec.Code)
	}
}

func TestNoOriginNoRefererAllowed(t *testing.T) {
	h := Middleware(okHandler, []string{"https://app.example.com"}, nil)

	// No Origin, no Referer — non-browser client (curl, server-to-server).
	req := httptest.NewRequest(http.MethodPost, "/waitlist", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for no-origin request, got %d", rec.Code)
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
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s: expected 403, got %d", method, rec.Code)
		}
	}
}
