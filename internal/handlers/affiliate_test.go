package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAffiliateHandler_HandleClick_Success(t *testing.T) {
	handler := NewAffiliateHandler(nil) // nil analytics — no-op tracking

	req := httptest.NewRequest(http.MethodGet,
		"/api/affiliate/click?url=https%3A%2F%2Fwww.skyscanner.com%2Ftransport%2Fflights%2FJFK%2FPRG&partner=skyscanner&category=flight",
		nil)
	rr := httptest.NewRecorder()

	handler.HandleClick(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("expected status %d, got %d", http.StatusFound, rr.Code)
	}

	location := rr.Header().Get("Location")
	if location != "https://www.skyscanner.com/transport/flights/JFK/PRG" {
		t.Errorf("expected redirect to skyscanner URL, got %q", location)
	}
}

func TestAffiliateHandler_HandleClick_MissingURL(t *testing.T) {
	handler := NewAffiliateHandler(nil)

	req := httptest.NewRequest(http.MethodGet,
		"/api/affiliate/click?partner=skyscanner&category=flight",
		nil)
	rr := httptest.NewRecorder()

	handler.HandleClick(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestAffiliateHandler_HandleClick_InvalidURL(t *testing.T) {
	handler := NewAffiliateHandler(nil)

	req := httptest.NewRequest(http.MethodGet,
		"/api/affiliate/click?url=not-a-valid-url",
		nil)
	rr := httptest.NewRecorder()

	handler.HandleClick(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestAffiliateHandler_HandleClick_InvalidScheme(t *testing.T) {
	handler := NewAffiliateHandler(nil)

	req := httptest.NewRequest(http.MethodGet,
		"/api/affiliate/click?url=javascript%3A%2F%2Falert(1)",
		nil)
	rr := httptest.NewRecorder()

	handler.HandleClick(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d for javascript scheme, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestAffiliateHandler_HandleClick_DisallowedDomain(t *testing.T) {
	handler := NewAffiliateHandler(nil)

	req := httptest.NewRequest(http.MethodGet,
		"/api/affiliate/click?url=https%3A%2F%2Fevil.example.com%2Fphishing",
		nil)
	rr := httptest.NewRecorder()

	handler.HandleClick(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d for disallowed domain, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestAffiliateHandler_HandleClick_WrongMethod(t *testing.T) {
	handler := NewAffiliateHandler(nil)

	req := httptest.NewRequest(http.MethodPost,
		"/api/affiliate/click?url=https%3A%2F%2Fwww.skyscanner.com%2F",
		nil)
	rr := httptest.NewRecorder()

	handler.HandleClick(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, rr.Code)
	}
}

func TestAffiliateHandler_HandleClick_AllAllowedDomains(t *testing.T) {
	handler := NewAffiliateHandler(nil)

	domains := []string{
		"www.skyscanner.com",
		"skyscanner.com",
		"www.booking.com",
		"booking.com",
		"www.getyourguide.com",
		"getyourguide.com",
		"www.viator.com",
		"viator.com",
		"www.discovercars.com",
		"discovercars.com",
		"safetywing.com",
		"www.safetywing.com",
	}

	for _, domain := range domains {
		t.Run(domain, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet,
				"/api/affiliate/click?url=https%3A%2F%2F"+domain+"%2Ftest",
				nil)
			rr := httptest.NewRecorder()

			handler.HandleClick(rr, req)

			if rr.Code != http.StatusFound {
				t.Errorf("expected status %d for domain %s, got %d", http.StatusFound, domain, rr.Code)
			}
		})
	}
}

func TestAffiliateHandler_HandleClick_MinimalParams(t *testing.T) {
	handler := NewAffiliateHandler(nil)

	// Only the url param is required; partner and category are optional
	req := httptest.NewRequest(http.MethodGet,
		"/api/affiliate/click?url=https%3A%2F%2Fwww.booking.com%2Fsearchresults.html",
		nil)
	rr := httptest.NewRecorder()

	handler.HandleClick(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("expected status %d, got %d", http.StatusFound, rr.Code)
	}
}

func TestIsAllowedAffiliateDomain(t *testing.T) {
	tests := []struct {
		host string
		want bool
	}{
		{"www.skyscanner.com", true},
		{"skyscanner.com", true},
		{"www.booking.com", true},
		{"booking.com", true},
		{"www.getyourguide.com", true},
		{"getyourguide.com", true},
		{"www.viator.com", true},
		{"viator.com", true},
		{"www.discovercars.com", true},
		{"discovercars.com", true},
		{"safetywing.com", true},
		{"www.safetywing.com", true},
		{"evil.com", false},
		{"skyscanner.evil.com", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			got := isAllowedAffiliateDomain(tt.host)
			if got != tt.want {
				t.Errorf("isAllowedAffiliateDomain(%q) = %v, want %v", tt.host, got, tt.want)
			}
		})
	}
}
