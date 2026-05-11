package handlers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gallowaysoftware/toqui-backend/internal/email"
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

// signSvix computes the Svix-style signature for `svix_id.svix_timestamp.body`
// given a base64-decoded HMAC key. Returns the value to put in the
// `svix-signature` header, including the `v1,` prefix.
func signSvix(t *testing.T, key []byte, svixID, svixTimestamp string, body []byte) string {
	t.Helper()
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(svixID))
	mac.Write([]byte("."))
	mac.Write([]byte(svixTimestamp))
	mac.Write([]byte("."))
	mac.Write(body)
	return "v1," + base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// makeResendRequest builds a Resend-style POST with valid Svix headers
// computed against the supplied secret. The secret is the *decoded* HMAC
// key bytes; the matching `whsec_<base64>` form is what the handler sees.
//
// If signWith is empty, no svix-signature header is set (used to test the
// missing-signature path).
func makeResendRequest(t *testing.T, payload []byte, signWith []byte, opts ...func(headers map[string]string)) testRequestResult {
	t.Helper()
	svixID := "msg_test_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	svixTimestamp := strconv.FormatInt(time.Now().UTC().Unix(), 10)

	headers := map[string]string{
		"Content-Type":   "application/json",
		"svix-id":        svixID,
		"svix-timestamp": svixTimestamp,
	}
	if len(signWith) > 0 {
		headers["svix-signature"] = signSvix(t, signWith, svixID, svixTimestamp, payload)
	}
	for _, o := range opts {
		o(headers)
	}
	return makeRequest(t, http.MethodPost, "/webhooks/email/inbound", bytes.NewReader(payload), headers)
}

// stubFetcher is a test double for ReceivedEmailFetcher.
type stubFetcher struct {
	called bool
	resp   *email.ReceivedEmail
	err    error
}

func (s *stubFetcher) FetchReceived(ctx context.Context, emailID string) (*email.ReceivedEmail, error) {
	s.called = true
	return s.resp, s.err
}

// keyForSecret turns a `whsec_<base64>` secret string into the decoded
// HMAC key bytes the handler will use internally.
func keyForSecret(t *testing.T, secret string) []byte {
	t.Helper()
	key, err := base64.StdEncoding.DecodeString(secret[len("whsec_"):])
	if err != nil {
		t.Fatalf("test secret invalid base64: %v", err)
	}
	return key
}

// validInboundEvent returns a marshalled email.received envelope with a
// fixed email_id, useful for tests that don't care about the body
// (because they're testing auth, not ingest).
func validInboundEvent(t *testing.T) []byte {
	t.Helper()
	body, err := json.Marshal(ResendInboundEvent{
		Type:      "email.received",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Data: ResendInboundData{
			EmailID: "rec_test_123",
			From:    "sender@example.com",
			To:      []string{"add@import.toqui.travel"},
			Subject: "Booking confirmation",
		},
	})
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	return body
}

// -- pure-function tests ------------------------------------------------------

func TestExtractEmailAddress(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"bare email", "user@example.com", "user@example.com"},
		{"name and email", "Jane Doe <jane@example.com>", "jane@example.com"},
		{"quoted name and email", `"Jane Doe" <jane@example.com>`, "jane@example.com"},
		{"email with spaces", "  user@example.com  ", "user@example.com"},
		{"empty string", "", ""},
		{"malformed angle brackets", "Jane <jane", "Jane <jane"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractEmailAddress(tt.input); got != tt.want {
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
		{"simple html", "<html><body><p>Hello</p></body></html>", "Hello"},
		{"nested tags", "<div><span>Nested <em>content</em></span></div>", "Nested content"},
		{"no tags", "Plain text", "Plain text"},
		{"empty", "", ""},
		{"only tags", "<br><br><br>", ""},
		{"with attributes", `<a href="https://example.com">Click here</a>`, "Click here"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stripHTMLTags(tt.input); got != tt.want {
				t.Errorf("stripHTMLTags(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// -- HandleInbound auth-path tests --------------------------------------------

func TestHandleInbound_MethodNotAllowed(t *testing.T) {
	handler := &EmailWebhookHandler{}
	rr := makeRequest(t, "GET", "/webhooks/email/inbound", nil, nil)
	handler.HandleInbound(rr.w, rr.r)
	if rr.w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.w.Code)
	}
}

func TestHandleInbound_NoSecretConfigured(t *testing.T) {
	handler := &EmailWebhookHandler{}
	rr := makeRequest(t, "POST", "/webhooks/email/inbound", bytes.NewReader([]byte("{}")), nil)
	handler.HandleInbound(rr.w, rr.r)
	if rr.w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rr.w.Code)
	}
}

func TestHandleInbound_InvalidJSON(t *testing.T) {
	handler := &EmailWebhookHandler{
		webhookSecret:        "whsec_dGVzdC1zZWNyZXQ=",
		skipSignatureForTest: true,
	}
	rr := makeRequest(t, "POST", "/webhooks/email/inbound",
		bytes.NewReader([]byte("not-json")),
		map[string]string{"Content-Type": "application/json"})
	handler.HandleInbound(rr.w, rr.r)
	if rr.w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.w.Code)
	}
}

func TestHandleInbound_MissingSvixHeaders(t *testing.T) {
	handler := &EmailWebhookHandler{webhookSecret: "whsec_dGVzdC1zZWNyZXQ="}
	rr := makeRequest(t, "POST", "/webhooks/email/inbound",
		bytes.NewReader(validInboundEvent(t)),
		map[string]string{"Content-Type": "application/json"})
	handler.HandleInbound(rr.w, rr.r)
	if rr.w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 missing svix headers, got %d", rr.w.Code)
	}
}

func TestHandleInbound_WrongSignature(t *testing.T) {
	secret := "whsec_dGVzdC1zZWNyZXQ=" // "test-secret"
	handler := &EmailWebhookHandler{webhookSecret: secret}

	body := validInboundEvent(t)
	wrongKey := []byte("totally-wrong")
	rr := makeResendRequest(t, body, wrongKey)
	handler.HandleInbound(rr.w, rr.r)
	if rr.w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 wrong signature, got %d", rr.w.Code)
	}
}

func TestHandleInbound_ExpiredTimestamp(t *testing.T) {
	secret := "whsec_dGVzdC1zZWNyZXQ="
	handler := &EmailWebhookHandler{webhookSecret: secret}
	key := keyForSecret(t, secret)

	body := validInboundEvent(t)
	// 10 minutes in the past — outside the 5-min window.
	staleTS := strconv.FormatInt(time.Now().UTC().Add(-10*time.Minute).Unix(), 10)
	svixID := "msg_stale"
	headers := map[string]string{
		"Content-Type":   "application/json",
		"svix-id":        svixID,
		"svix-timestamp": staleTS,
		"svix-signature": signSvix(t, key, svixID, staleTS, body),
	}
	rr := makeRequest(t, "POST", "/webhooks/email/inbound", bytes.NewReader(body), headers)
	handler.HandleInbound(rr.w, rr.r)
	if rr.w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 stale timestamp, got %d", rr.w.Code)
	}
}

func TestHandleInbound_FutureTimestampOutsideWindow(t *testing.T) {
	secret := "whsec_dGVzdC1zZWNyZXQ="
	handler := &EmailWebhookHandler{webhookSecret: secret}
	key := keyForSecret(t, secret)

	body := validInboundEvent(t)
	futureTS := strconv.FormatInt(time.Now().UTC().Add(10*time.Minute).Unix(), 10)
	svixID := "msg_future"
	headers := map[string]string{
		"Content-Type":   "application/json",
		"svix-id":        svixID,
		"svix-timestamp": futureTS,
		"svix-signature": signSvix(t, key, svixID, futureTS, body),
	}
	rr := makeRequest(t, "POST", "/webhooks/email/inbound", bytes.NewReader(body), headers)
	handler.HandleInbound(rr.w, rr.r)
	if rr.w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 future timestamp, got %d", rr.w.Code)
	}
}

func TestVerifySvixSignature_AcceptsValidSig(t *testing.T) {
	secret := "whsec_c2VjcmV0LWtleS0zMi1ieXRlcy1vZi1lbnRyb3B5" // "secret-key-32-bytes-of-entropy"
	key := keyForSecret(t, secret)
	h := &EmailWebhookHandler{webhookSecret: secret}

	body := validInboundEvent(t)
	svixID := "msg_valid"
	svixTimestamp := strconv.FormatInt(time.Now().UTC().Unix(), 10)
	sig := signSvix(t, key, svixID, svixTimestamp, body)

	r := httptest.NewRequest(http.MethodPost, "/webhooks/email/inbound", bytes.NewReader(body))
	r.Header.Set("svix-id", svixID)
	r.Header.Set("svix-timestamp", svixTimestamp)
	r.Header.Set("svix-signature", sig)

	if err := h.verifySvixSignature(r, body); err != nil {
		t.Errorf("expected verifySvixSignature to accept valid HMAC, got %v", err)
	}
}

func TestVerifySvixSignature_AcceptsMultipleSigsAnyMatch(t *testing.T) {
	secret := "whsec_c2VjcmV0LWtleS0zMi1ieXRlcy1vZi1lbnRyb3B5"
	key := keyForSecret(t, secret)
	h := &EmailWebhookHandler{webhookSecret: secret}

	body := validInboundEvent(t)
	svixID := "msg_multi"
	svixTimestamp := strconv.FormatInt(time.Now().UTC().Unix(), 10)
	good := signSvix(t, key, svixID, svixTimestamp, body)
	// Multiple signatures separated by spaces; one bogus, one valid.
	combined := "v1,bogusbogusbogus " + good

	r := httptest.NewRequest(http.MethodPost, "/webhooks/email/inbound", bytes.NewReader(body))
	r.Header.Set("svix-id", svixID)
	r.Header.Set("svix-timestamp", svixTimestamp)
	r.Header.Set("svix-signature", combined)

	if err := h.verifySvixSignature(r, body); err != nil {
		t.Errorf("expected acceptance when at least one v1 sig matches, got %v", err)
	}
}

// -- HandleInbound dispatch tests --------------------------------------------

func TestHandleInbound_NonInboundEventTypeIs200NoOp(t *testing.T) {
	// A delivery/bounce event accidentally pointed at this URL should
	// be a 200 no-op (so Resend doesn't retry forever) without trying
	// to fetch a body.
	fetcher := &stubFetcher{}
	handler := &EmailWebhookHandler{
		webhookSecret:        "whsec_dGVzdC1zZWNyZXQ=",
		skipSignatureForTest: true,
		inbound:              fetcher,
	}

	body, _ := json.Marshal(map[string]any{
		"type":       "email.delivered",
		"created_at": time.Now().UTC().Format(time.RFC3339),
		"data":       map[string]any{"email_id": "rec_456"},
	})
	rr := makeRequest(t, "POST", "/webhooks/email/inbound",
		bytes.NewReader(body),
		map[string]string{"Content-Type": "application/json"})
	handler.HandleInbound(rr.w, rr.r)

	if rr.w.Code != http.StatusOK {
		t.Errorf("expected 200 for non-inbound event, got %d", rr.w.Code)
	}
	if fetcher.called {
		t.Error("FetchReceived must not be called for non-inbound events")
	}
}

func TestHandleInbound_MissingEmailIDIs200NoOp(t *testing.T) {
	fetcher := &stubFetcher{}
	handler := &EmailWebhookHandler{
		webhookSecret:        "whsec_dGVzdC1zZWNyZXQ=",
		skipSignatureForTest: true,
		inbound:              fetcher,
	}

	body, _ := json.Marshal(ResendInboundEvent{
		Type: "email.received",
		Data: ResendInboundData{From: "sender@example.com"}, // no email_id
	})
	rr := makeRequest(t, "POST", "/webhooks/email/inbound",
		bytes.NewReader(body),
		map[string]string{"Content-Type": "application/json"})
	handler.HandleInbound(rr.w, rr.r)

	if rr.w.Code != http.StatusOK {
		t.Errorf("expected 200 with missing email_id, got %d", rr.w.Code)
	}
	if fetcher.called {
		t.Error("FetchReceived must not be called when email_id is empty")
	}
}

// -- secret decoding edge cases ----------------------------------------------

func TestVerifySvixSignature_RejectsInvalidSecretFormat(t *testing.T) {
	// A secret without `whsec_` prefix is treated as the raw base64;
	// non-base64 input should be rejected at decode time.
	h := &EmailWebhookHandler{webhookSecret: "whsec_!!!not-base64!!!"}
	r := httptest.NewRequest(http.MethodPost, "/webhooks/email/inbound", bytes.NewReader([]byte(`{}`)))
	r.Header.Set("svix-id", "x")
	r.Header.Set("svix-timestamp", strconv.FormatInt(time.Now().Unix(), 10))
	r.Header.Set("svix-signature", "v1,xxx")

	if err := h.verifySvixSignature(r, []byte(`{}`)); err == nil {
		t.Error("expected decode error for malformed secret")
	}
}

// Sanity check: ensure the local signing helper produces a value the
// handler accepts. This protects against signSvix and verifySvixSignature
// drifting apart.
func TestSignAndVerifyRoundTrip(t *testing.T) {
	secret := "whsec_c2VjcmV0LWtleS0zMi1ieXRlcy1vZi1lbnRyb3B5"
	key := keyForSecret(t, secret)
	body := []byte(fmt.Sprintf(`{"hello":"world","ts":%d}`, time.Now().Unix()))

	svixID := "msg_roundtrip"
	svixTimestamp := strconv.FormatInt(time.Now().Unix(), 10)
	sig := signSvix(t, key, svixID, svixTimestamp, body)

	r := httptest.NewRequest(http.MethodPost, "/webhooks/email/inbound", bytes.NewReader(body))
	r.Header.Set("svix-id", svixID)
	r.Header.Set("svix-timestamp", svixTimestamp)
	r.Header.Set("svix-signature", sig)

	h := &EmailWebhookHandler{webhookSecret: secret}
	if err := h.verifySvixSignature(r, body); err != nil {
		t.Fatalf("round-trip failed: %v", err)
	}
}

// TestVerifySvixSignature_RejectsMalformedBase64SigEntry pins B1: the
// verifier must base64-decode each signature entry and compare raw HMAC
// bytes, so a malformed-base64 entry is silently skipped instead of
// being fed straight to hmac.Equal (which would have leaked the
// expected-signature length via the unequal-length comparison
// short-circuit).
func TestVerifySvixSignature_RejectsMalformedBase64SigEntry(t *testing.T) {
	secret := "whsec_c2VjcmV0LWtleS0zMi1ieXRlcy1vZi1lbnRyb3B5"
	h := &EmailWebhookHandler{webhookSecret: secret}

	body := []byte(`{"type":"email.received"}`)
	svixID := "msg_malformed"
	svixTimestamp := strconv.FormatInt(time.Now().Unix(), 10)

	r := httptest.NewRequest(http.MethodPost, "/webhooks/email/inbound", bytes.NewReader(body))
	r.Header.Set("svix-id", svixID)
	r.Header.Set("svix-timestamp", svixTimestamp)
	// "v1,!!!not-base64!!!" — illegal base64 should be rejected, not
	// fed into hmac.Equal as raw bytes against the expected base64
	// string.
	r.Header.Set("svix-signature", "v1,!!!not-base64!!!")

	err := h.verifySvixSignature(r, body)
	if err == nil {
		t.Fatal("expected verification to fail on malformed base64 signature")
	}
}

// TestMarkAndCheckSvixID_DedupesWithinWindow regresses B2: a duplicate
// svix-id seen within the replay window must short-circuit. Booking
// ingestion is not idempotent, so without this gate a Resend retry
// (or attacker replay) creates a duplicate booking each time.
func TestMarkAndCheckSvixID_DedupesWithinWindow(t *testing.T) {
	h := &EmailWebhookHandler{}

	if !h.markAndCheckSvixID("msg_first") {
		t.Fatal("first sighting must return true")
	}
	if h.markAndCheckSvixID("msg_first") {
		t.Errorf("second sighting of same id within window must return false (dedup)")
	}
	if !h.markAndCheckSvixID("msg_other") {
		t.Errorf("a different id must NOT collide with the first one")
	}
}

// TestMarkAndCheckSvixID_EmptyIDPassesThrough — defensive: empty IDs
// are not deduped because they don't uniquely identify a delivery
// attempt and we don't want all empty-id requests to collapse onto
// each other.
func TestMarkAndCheckSvixID_EmptyIDPassesThrough(t *testing.T) {
	h := &EmailWebhookHandler{}
	if !h.markAndCheckSvixID("") {
		t.Errorf("empty id must NOT be treated as a duplicate")
	}
	if !h.markAndCheckSvixID("") {
		t.Errorf("repeated empty id must still pass through (no dedup)")
	}
}

// TestMarkAndCheckSvixID_EvictsExpiredEntries — opportunistic eviction
// keeps the seen-id map bounded. Once an entry is older than the
// replay window, it must be dropped so the same id can be reused
// freely (won't happen in practice, but proves the eviction works).
func TestMarkAndCheckSvixID_EvictsExpiredEntries(t *testing.T) {
	h := &EmailWebhookHandler{}
	// Manually plant an expired entry to simulate a request that
	// landed outside the replay window. Eviction happens on the next
	// call.
	h.seenSvixIDsMu.Lock()
	h.seenSvixIDs = map[string]time.Time{
		"msg_expired": time.Now().Add(-2 * svixReplayWindow),
	}
	h.seenSvixIDsMu.Unlock()

	if !h.markAndCheckSvixID("msg_other") {
		t.Fatal("unrelated fresh id must pass")
	}

	h.seenSvixIDsMu.Lock()
	_, stillThere := h.seenSvixIDs["msg_expired"]
	h.seenSvixIDsMu.Unlock()
	if stillThere {
		t.Errorf("expected expired id to be evicted on next access")
	}
}
