package handlers

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

// testRequestResult holds both the recorder and the request for test assertions.
type testRequestResult struct {
	w *httptest.ResponseRecorder
	r *http.Request
}

// makeRequest creates a test HTTP request with the given method, path, body, and headers.
func makeRequest(t *testing.T, method, path string, body io.Reader, headers map[string]string) testRequestResult {
	t.Helper()
	r := httptest.NewRequest(method, path, body)
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	return testRequestResult{w: w, r: r}
}

// makeMultipartRequest creates a test HTTP POST request with multipart/form-data.
func makeMultipartRequest(t *testing.T, path string, fields map[string]string) testRequestResult {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	for key, val := range fields {
		if err := writer.WriteField(key, val); err != nil {
			t.Fatalf("write field %q: %v", key, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	r := httptest.NewRequest(http.MethodPost, path, &buf)
	r.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	return testRequestResult{w: w, r: r}
}

func TestExtractEmailAddress(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "bare email",
			input: "user@example.com",
			want:  "user@example.com",
		},
		{
			name:  "name and email",
			input: "Jane Doe <jane@example.com>",
			want:  "jane@example.com",
		},
		{
			name:  "quoted name and email",
			input: `"Jane Doe" <jane@example.com>`,
			want:  "jane@example.com",
		},
		{
			name:  "email with spaces",
			input: "  user@example.com  ",
			want:  "user@example.com",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "name only no brackets",
			input: "Jane Doe",
			want:  "Jane Doe",
		},
		{
			name:  "angle brackets with spaces",
			input: "Jane Doe < jane@example.com >",
			want:  "jane@example.com",
		},
		{
			name:  "multiple angle brackets uses last",
			input: "Jane <old@example.com> Doe <new@example.com>",
			want:  "new@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractEmailAddress(tt.input)
			if got != tt.want {
				t.Errorf("extractEmailAddress(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestStripHTMLTags(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain text",
			input: "Hello, world!",
			want:  "Hello, world!",
		},
		{
			name:  "simple html",
			input: "<p>Hello, <b>world</b>!</p>",
			want:  "Hello, world!",
		},
		{
			name:  "html with attributes",
			input: `<div class="content"><p>Hello</p></div>`,
			want:  "Hello",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "nested tags",
			input: "<html><body><h1>Title</h1><p>Content</p></body></html>",
			want:  "TitleContent",
		},
		{
			name:  "self closing tags",
			input: "Line one<br/>Line two",
			want:  "Line oneLine two",
		},
		{
			name:  "whitespace preservation",
			input: "<p>  Hello  </p>  <p>  World  </p>",
			want:  "Hello      World",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripHTMLTags(tt.input)
			if got != tt.want {
				t.Errorf("stripHTMLTags(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestHandleInbound_MethodNotAllowed(t *testing.T) {
	handler := &EmailWebhookHandler{}

	// GET should be rejected
	rr := makeRequest(t, "GET", "/webhooks/email/inbound", nil, nil)
	handler.HandleInbound(rr.w, rr.r)

	if rr.w.Code != 405 {
		t.Errorf("expected status 405, got %d", rr.w.Code)
	}
}

func TestHandleInbound_NoWebhookKey(t *testing.T) {
	handler := &EmailWebhookHandler{}

	rr := makeRequest(t, "POST", "/webhooks/email/inbound", nil, nil)
	handler.HandleInbound(rr.w, rr.r)

	if rr.w.Code != 503 {
		t.Errorf("expected status 503 when webhook key not configured, got %d", rr.w.Code)
	}
}

func TestHandleInbound_EmptyBody(t *testing.T) {
	handler := &EmailWebhookHandler{webhookKey: "test-key", skipSignatureForTest: true}

	// POST with multipart form but no text/html body
	fields := map[string]string{
		"from":    "sender@example.com",
		"subject": "Test booking",
		"text":    "",
		"html":    "",
	}
	rr := makeMultipartRequest(t, "/webhooks/email/inbound", fields)
	handler.HandleInbound(rr.w, rr.r)

	// Should return 200 OK (don't retry for empty bodies)
	if rr.w.Code != 200 {
		t.Errorf("expected status 200 for empty body, got %d", rr.w.Code)
	}
}
