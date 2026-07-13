package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func decodeResults(t *testing.T, raw json.RawMessage) []searchResult {
	t.Helper()
	var out struct {
		Results []searchResult `json:"results"`
		Status  string         `json:"status"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal tool output: %v", err)
	}
	return out.Results
}

func TestWebSearch_StubReturnsNoWebAccessWithoutError(t *testing.T) {
	raw, err := NewWebSearchStub().Execute(context.Background(), json.RawMessage(`{"query":"paris"}`))
	if err != nil {
		t.Fatalf("stub returned a Go error: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// The stub MUST NOT include an "error" key (Gemini treats it as failure).
	if _, hasErr := payload["error"]; hasErr {
		t.Error("stub payload contains an 'error' key")
	}
	if payload["status"] != "no_web_access" {
		t.Errorf("status = %v, want no_web_access", payload["status"])
	}
}

func TestWebSearch_SearXNGBackend(t *testing.T) {
	var gotQuery, gotFormat string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("q")
		gotFormat = r.URL.Query().Get("format")
		_, _ = w.Write([]byte(`{"results":[
			{"title":"Louvre","url":"https://louvre.fr","content":"art museum"},
			{"title":"Eiffel Tower","url":"https://toureiffel.paris","content":"iron tower"}
		]}`))
	}))
	defer srv.Close()

	tool := NewWebSearchSearXNG(srv.URL)
	raw, err := tool.Execute(context.Background(), json.RawMessage(`{"query":"paris landmarks"}`))
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotQuery != "paris landmarks" {
		t.Errorf("query forwarded = %q", gotQuery)
	}
	if gotFormat != "json" {
		t.Errorf("format = %q, want json", gotFormat)
	}
	results := decodeResults(t, raw)
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	if results[0].Title != "Louvre" || results[0].URL != "https://louvre.fr" || results[0].Snippet != "art museum" {
		t.Errorf("first result mismapped: %+v (SearXNG 'content' → snippet)", results[0])
	}
}

func TestWebSearch_SearXNGCapsResults(t *testing.T) {
	// SearXNG returns many results; the tool caps at 5.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		var b strings.Builder
		b.WriteString(`{"results":[`)
		for i := 0; i < 20; i++ {
			if i > 0 {
				b.WriteString(",")
			}
			b.WriteString(`{"title":"t","url":"https://x","content":"c"}`)
		}
		b.WriteString(`]}`)
		_, _ = w.Write([]byte(b.String()))
	}))
	defer srv.Close()

	raw, err := NewWebSearchSearXNG(srv.URL).Execute(context.Background(), json.RawMessage(`{"query":"x"}`))
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if n := len(decodeResults(t, raw)); n != 5 {
		t.Errorf("got %d results, want capped at 5", n)
	}
}

func TestWebSearch_SearXNGTrailingSlashTrimmed(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer srv.Close()

	// Pass a base URL with a trailing slash — the tool must not produce //search.
	_, err := NewWebSearchSearXNG(srv.URL+"/").Execute(context.Background(), json.RawMessage(`{"query":"x"}`))
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotPath != "/search" {
		t.Errorf("request path = %q, want /search", gotPath)
	}
}

func TestWebSearch_BackendHTTPErrorPropagates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := NewWebSearchSearXNG(srv.URL).Execute(context.Background(), json.RawMessage(`{"query":"x"}`))
	if err == nil {
		t.Error("expected an error on a 500 from the backend")
	}
}

func TestWebSearch_GoogleBackendParsing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"items":[{"title":"T","link":"https://l","snippet":"S"}]}`))
	}))
	defer srv.Close()

	// Point the Google backend at the test server by overriding its URL host
	// is not possible without a seam; instead validate the SearXNG-independent
	// mapping via a direct googleSearch with a stubbed client transport.
	g := &googleSearch{apiKey: "k", cx: "c", client: srv.Client()}
	// Rewrite requests to the test server.
	g.client.Transport = rewriteTransport{to: srv.URL}
	results, err := g.search(context.Background(), "q")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 1 || results[0].Title != "T" || results[0].URL != "https://l" || results[0].Snippet != "S" {
		t.Errorf("google mapping wrong: %+v", results)
	}
}

// rewriteTransport redirects every request to a fixed base URL (the test
// server), preserving the query string.
type rewriteTransport struct{ to string }

func (rt rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base, _ := url.Parse(rt.to)
	req.URL.Scheme = base.Scheme
	req.URL.Host = base.Host
	return http.DefaultTransport.RoundTrip(req)
}

func TestPlaceLookup_StubReturnsNoErrorPayload(t *testing.T) {
	raw, err := NewPlaceLookupStub().Execute(context.Background(), json.RawMessage(`{"query":"eiffel tower"}`))
	if err != nil {
		t.Fatalf("stub returned a Go error: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, hasErr := payload["error"]; hasErr {
		t.Error("place_lookup stub payload contains an 'error' key")
	}
	if payload["status"] != "no_place_data" {
		t.Errorf("status = %v, want no_place_data", payload["status"])
	}
}
