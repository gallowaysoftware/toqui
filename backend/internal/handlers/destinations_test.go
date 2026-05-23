package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDestinationSearch_ExactMatch(t *testing.T) {
	h := NewDestinationSearchHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/destinations/search?q=Japan", nil)
	w := httptest.NewRecorder()

	h.HandleSearch(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var results []DestinationResult
	if err := json.Unmarshal(w.Body.Bytes(), &results); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].Code != "JP" {
		t.Errorf("code = %q, want %q", results[0].Code, "JP")
	}
	if results[0].Name != "Japan" {
		t.Errorf("name = %q, want %q", results[0].Name, "Japan")
	}
}

func TestDestinationSearch_CaseInsensitive(t *testing.T) {
	h := NewDestinationSearchHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/destinations/search?q=japan", nil)
	w := httptest.NewRecorder()

	h.HandleSearch(w, req)

	var results []DestinationResult
	json.Unmarshal(w.Body.Bytes(), &results)

	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].Code != "JP" {
		t.Errorf("code = %q, want %q", results[0].Code, "JP")
	}
}

func TestDestinationSearch_PrefixMatch(t *testing.T) {
	h := NewDestinationSearchHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/destinations/search?q=ita", nil)
	w := httptest.NewRecorder()

	h.HandleSearch(w, req)

	var results []DestinationResult
	json.Unmarshal(w.Body.Bytes(), &results)

	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].Code != "IT" {
		t.Errorf("code = %q, want %q", results[0].Code, "IT")
	}
}

func TestDestinationSearch_Alias_UK(t *testing.T) {
	h := NewDestinationSearchHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/destinations/search?q=uk", nil)
	w := httptest.NewRecorder()

	h.HandleSearch(w, req)

	var results []DestinationResult
	json.Unmarshal(w.Body.Bytes(), &results)

	// "uk" matches the alias "UK" -> GB, and also "Ukraine" if present.
	// We only have UK alias in our list.
	found := false
	for _, r := range results {
		if r.Code == "GB" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected GB in results for query 'uk', got %v", results)
	}
}

func TestDestinationSearch_Alias_USA(t *testing.T) {
	h := NewDestinationSearchHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/destinations/search?q=usa", nil)
	w := httptest.NewRecorder()

	h.HandleSearch(w, req)

	var results []DestinationResult
	json.Unmarshal(w.Body.Bytes(), &results)

	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].Code != "US" {
		t.Errorf("code = %q, want %q", results[0].Code, "US")
	}
}

func TestDestinationSearch_Alias_America(t *testing.T) {
	h := NewDestinationSearchHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/destinations/search?q=america", nil)
	w := httptest.NewRecorder()

	h.HandleSearch(w, req)

	var results []DestinationResult
	json.Unmarshal(w.Body.Bytes(), &results)

	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].Code != "US" {
		t.Errorf("code = %q, want %q", results[0].Code, "US")
	}
}

func TestDestinationSearch_NoResults(t *testing.T) {
	h := NewDestinationSearchHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/destinations/search?q=zzznomatch", nil)
	w := httptest.NewRecorder()

	h.HandleSearch(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var results []DestinationResult
	json.Unmarshal(w.Body.Bytes(), &results)

	if len(results) != 0 {
		t.Errorf("len(results) = %d, want 0", len(results))
	}
}

func TestDestinationSearch_EmptyQuery(t *testing.T) {
	h := NewDestinationSearchHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/destinations/search?q=", nil)
	w := httptest.NewRecorder()

	h.HandleSearch(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var results []DestinationResult
	json.Unmarshal(w.Body.Bytes(), &results)

	if len(results) != 0 {
		t.Errorf("len(results) = %d, want 0 for empty query", len(results))
	}
}

func TestDestinationSearch_NoQueryParam(t *testing.T) {
	h := NewDestinationSearchHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/destinations/search", nil)
	w := httptest.NewRecorder()

	h.HandleSearch(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var results []DestinationResult
	json.Unmarshal(w.Body.Bytes(), &results)

	if len(results) != 0 {
		t.Errorf("len(results) = %d, want 0", len(results))
	}
}

func TestDestinationSearch_MaxResults(t *testing.T) {
	h := NewDestinationSearchHandler()

	// Query with a single character that matches many entries.
	req := httptest.NewRequest(http.MethodGet, "/api/destinations/search?q=s", nil)
	w := httptest.NewRecorder()

	h.HandleSearch(w, req)

	var results []DestinationResult
	json.Unmarshal(w.Body.Bytes(), &results)

	if len(results) > 10 {
		t.Errorf("len(results) = %d, want <= 10", len(results))
	}
}

func TestDestinationSearch_NoDuplicateCodes(t *testing.T) {
	h := NewDestinationSearchHandler()

	// "united" should match "United Kingdom" (primary) and potentially
	// "United States" (primary) — but not duplicate GB from alias.
	req := httptest.NewRequest(http.MethodGet, "/api/destinations/search?q=united", nil)
	w := httptest.NewRecorder()

	h.HandleSearch(w, req)

	var results []DestinationResult
	json.Unmarshal(w.Body.Bytes(), &results)

	codes := make(map[string]bool)
	for _, r := range results {
		if codes[r.Code] {
			t.Errorf("duplicate code %q in results", r.Code)
		}
		codes[r.Code] = true
	}
}

func TestDestinationSearch_MethodNotAllowed(t *testing.T) {
	h := NewDestinationSearchHandler()

	req := httptest.NewRequest(http.MethodPost, "/api/destinations/search?q=japan", nil)
	w := httptest.NewRecorder()

	h.HandleSearch(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestDestinationSearch_WhitespaceQuery(t *testing.T) {
	h := NewDestinationSearchHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/destinations/search?q=++++", nil)
	w := httptest.NewRecorder()

	h.HandleSearch(w, req)

	var results []DestinationResult
	json.Unmarshal(w.Body.Bytes(), &results)

	if len(results) != 0 {
		t.Errorf("len(results) = %d, want 0 for whitespace-only query", len(results))
	}
}

func TestDestinationSearch_Alias_Korea(t *testing.T) {
	h := NewDestinationSearchHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/destinations/search?q=korea", nil)
	w := httptest.NewRecorder()

	h.HandleSearch(w, req)

	var results []DestinationResult
	json.Unmarshal(w.Body.Bytes(), &results)

	if len(results) == 0 {
		t.Fatal("expected results for 'korea'")
	}
	found := false
	for _, r := range results {
		if r.Code == "KR" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected KR in results for 'korea', got %v", results)
	}
}

func TestDestinationSearch_Alias_Czechia(t *testing.T) {
	h := NewDestinationSearchHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/destinations/search?q=czechia", nil)
	w := httptest.NewRecorder()

	h.HandleSearch(w, req)

	var results []DestinationResult
	json.Unmarshal(w.Body.Bytes(), &results)

	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].Code != "CZ" {
		t.Errorf("code = %q, want %q", results[0].Code, "CZ")
	}
}

func TestDestinationSearch_Alias_Bali(t *testing.T) {
	h := NewDestinationSearchHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/destinations/search?q=bali", nil)
	w := httptest.NewRecorder()

	h.HandleSearch(w, req)

	var results []DestinationResult
	json.Unmarshal(w.Body.Bytes(), &results)

	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].Code != "ID" {
		t.Errorf("code = %q, want %q", results[0].Code, "ID")
	}
}

func TestBuildDestinationEntries_AllLocations(t *testing.T) {
	entries := buildDestinationEntries()

	// Verify all 43 locations are present.
	codes := make(map[string]bool)
	for _, e := range entries {
		codes[e.code] = true
	}

	expectedCodes := []string{
		"IT", "JP", "FR", "GB", "US", "ES", "DE", "PT", "GR", "TH",
		"MX", "AU", "BR", "IN", "KR", "VN", "MA", "PE", "NZ", "TR",
		"HR", "ZA", "CO", "EG", "ID", "PH", "CN", "CZ", "AT", "CH",
		"IE", "SE", "AR", "CL", "JO", "TZ", "IS", "SG", "HK", "KH",
		"TW", "NO", "LK",
	}

	for _, code := range expectedCodes {
		if !codes[code] {
			t.Errorf("missing destination code %q", code)
		}
	}
}

func TestBuildDestinationEntries_Sorted(t *testing.T) {
	entries := buildDestinationEntries()

	for i := 1; i < len(entries); i++ {
		if entries[i].name < entries[i-1].name {
			t.Errorf("entries not sorted: %q < %q at index %d", entries[i].name, entries[i-1].name, i)
		}
	}
}
