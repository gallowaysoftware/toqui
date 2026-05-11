package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/net/html"

	"github.com/gallowaysoftware/toqui-backend/internal/audit"
	"github.com/gallowaysoftware/toqui-backend/internal/booking"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui-backend/internal/email"
	"github.com/gallowaysoftware/toqui-backend/internal/payment"
	"github.com/gallowaysoftware/toqui-backend/internal/trip"
)

// maxEmailBodySize bounds the inbound JSON payload to 1 MB. Resend's
// webhook bodies are tiny (metadata only — a few kilobytes), and the
// repo convention for every REST POST handler is 1 MB (CLAUDE.md
// "Security Hardening — Request body limits"). The previous 25 MB
// allowance was a DoS amplifier on a public, unauthenticated endpoint.
const maxEmailBodySize = 1 << 20

// svixReplayWindow is the maximum allowed clock skew between Resend's
// signing servers and this server. Anything older is rejected to
// mitigate replay attacks. Svix's official guidance is "within your
// tolerance"; 5 min matches what most receivers use.
const svixReplayWindow = 5 * time.Minute

// ResendInboundEvent is the JSON envelope Resend posts for inbound
// (`email.received`) webhooks. The webhook delivers metadata only —
// `data.email_id` is used to fetch the actual text/html body via the
// Resend Received Emails API (see internal/email/inbound.go).
//
// Per Resend docs (resend.com/docs/dashboard/webhooks):
//
//	{
//	  "type": "email.received",
//	  "created_at": "2026-...",
//	  "data": { "email_id": "...", "from": "...", "to": [...], ... }
//	}
//
// Authentication is via Svix signing headers:
//
//   - svix-id: unique message ID
//   - svix-timestamp: unix seconds (string)
//   - svix-signature: space-separated list of "v1,<base64-sig>" entries
//
// Signature is HMAC-SHA256 over `${svix-id}.${svix-timestamp}.${raw-body}`,
// keyed with the base64-decoded portion of the secret after the `whsec_`
// prefix. Verifier accepts a match against any v1 signature in the header.
type ResendInboundEvent struct {
	Type      string            `json:"type"`
	CreatedAt string            `json:"created_at"`
	Data      ResendInboundData `json:"data"`
}

// ResendInboundData mirrors the relevant fields of Resend's
// `data` object. We pull out only what we need for trip matching and
// the body fetch — full attachments etc. are intentionally ignored.
type ResendInboundData struct {
	EmailID   string   `json:"email_id"`
	From      string   `json:"from"`
	To        []string `json:"to"`
	Subject   string   `json:"subject"`
	MessageID string   `json:"message_id"`
}

// ReceivedEmailFetcher fetches the body of a received email by ID. The
// real implementation lives in internal/email; tests inject a fake.
type ReceivedEmailFetcher interface {
	FetchReceived(ctx context.Context, emailID string) (*email.ReceivedEmail, error)
}

// EmailWebhookHandler handles inbound email webhooks from Resend.
type EmailWebhookHandler struct {
	bookingSvc           *booking.Service
	tripSvc              *trip.Service
	paymentSvc           *payment.Service
	queries              *dbgen.Queries
	inbound              ReceivedEmailFetcher
	webhookSecret        string // Resend webhook signing secret (whsec_...)
	skipSignatureForTest bool   // test-only: skip signature verification

	// seenSvixIDs caches svix-id values within the replay window so that
	// a duplicate delivery (Resend retry of an already-processed message,
	// or an attacker replaying a captured-but-still-valid request)
	// short-circuits before booking ingest. Per-instance only — Cloud
	// Run scaleout means a request retried across instances inside the
	// 5-min window can still slip through. The signature timestamp gate
	// caps the residual exposure at 5 min; a full DB-backed dedup is the
	// next harden if cross-instance replay becomes a real risk.
	seenSvixIDs   map[string]time.Time
	seenSvixIDsMu sync.Mutex
}

// NewEmailWebhookHandler creates the inbound webhook handler.
//
//   - webhookSecret is the Resend webhook signing secret in `whsec_...`
//     form (loaded from EMAIL_WEBHOOK_SECRET in config).
//   - inbound is the client used to retrieve the email body after a
//     verified webhook arrives. Pass email.NewInbound(cfg.ResendAPIKey).
func NewEmailWebhookHandler(
	bookingSvc *booking.Service,
	tripSvc *trip.Service,
	paymentSvc *payment.Service,
	pool *pgxpool.Pool,
	inbound ReceivedEmailFetcher,
	webhookSecret string,
) *EmailWebhookHandler {
	return &EmailWebhookHandler{
		bookingSvc:    bookingSvc,
		tripSvc:       tripSvc,
		paymentSvc:    paymentSvc,
		queries:       dbgen.New(pool),
		inbound:       inbound,
		webhookSecret: webhookSecret,
		seenSvixIDs:   make(map[string]time.Time),
	}
}

// markAndCheckSvixID records the svix-id and returns true if this is the
// first time we've seen it within the replay window. Returns false on a
// duplicate (retry/replay). Old entries are evicted opportunistically.
//
// Safe to call on a handler built without NewEmailWebhookHandler — the
// internal map is lazily initialised under the mutex so tests that
// construct the struct directly don't panic.
func (h *EmailWebhookHandler) markAndCheckSvixID(id string) bool {
	if id == "" {
		// Defensive: the signature path already rejects empty svix-id;
		// treat empty-id callers as fresh so we don't accidentally
		// dedup multiple legitimate-but-malformed requests onto each
		// other.
		return true
	}
	now := time.Now()
	cutoff := now.Add(-svixReplayWindow)

	h.seenSvixIDsMu.Lock()
	defer h.seenSvixIDsMu.Unlock()

	if h.seenSvixIDs == nil {
		h.seenSvixIDs = make(map[string]time.Time)
	}

	// Opportunistic eviction — bounded by the small set of in-flight
	// IDs in any 5-min window, so the linear scan is cheap.
	for k, t := range h.seenSvixIDs {
		if t.Before(cutoff) {
			delete(h.seenSvixIDs, k)
		}
	}

	if seen, ok := h.seenSvixIDs[id]; ok && !seen.Before(cutoff) {
		return false
	}
	h.seenSvixIDs[id] = now
	return true
}

// HandleInbound processes a Resend `email.received` webhook.
// Route: POST /webhooks/email/inbound
//
// Pipeline:
//  1. Verify Svix signature + timestamp window
//  2. Decode the JSON envelope; ignore non-`email.received` events
//  3. Fetch the body from Resend's Received Emails API using data.email_id
//  4. Look up the user by sender email
//  5. Match the email to a trip by subject (existing logic)
//  6. Ingest as a booking
func (h *EmailWebhookHandler) HandleInbound(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.webhookSecret == "" {
		slog.Warn("email webhook rejected: EMAIL_WEBHOOK_SECRET not configured")
		http.Error(w, "webhook not configured", http.StatusServiceUnavailable)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxEmailBodySize)
	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Warn("email webhook read body failed", "error", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if !h.skipSignatureForTest {
		if err := h.verifySvixSignature(r, rawBody); err != nil {
			slog.Warn("email webhook signature verification failed", "error", err)
			audit.Log(audit.EventWebhookAuthFailed,
				"reason", "signature_verification_failed",
				"remote_addr", r.RemoteAddr,
				"svix_id_present", r.Header.Get("svix-id") != "",
				"error", err.Error(),
			)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		// Replay protection: a duplicate svix-id within the replay
		// window means either a Resend retry of an already-processed
		// message or an attacker replaying a captured request. Either
		// way, ignore. Booking ingestion is NOT idempotent — without
		// this gate a replay creates a duplicate booking each time.
		if !h.markAndCheckSvixID(r.Header.Get("svix-id")) {
			slog.Info("email webhook duplicate svix-id, ignoring",
				"svix_id", r.Header.Get("svix-id"),
			)
			audit.Log(audit.EventWebhookAuthFailed,
				"reason", "replay_duplicate_svix_id",
				"remote_addr", r.RemoteAddr,
				"svix_id_present", true,
			)
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	var event ResendInboundEvent
	if err := json.Unmarshal(rawBody, &event); err != nil {
		slog.Warn("email webhook invalid JSON", "error", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// Resend may deliver other event types over the same endpoint
	// (delivered, bounced, etc.) if a misconfiguration ever occurs.
	// Anything that's not an inbound event is a 200 no-op so Resend
	// doesn't retry it.
	if event.Type != "email.received" {
		slog.Info("email webhook ignoring non-inbound event type", "type", event.Type)
		w.WriteHeader(http.StatusOK)
		return
	}

	if event.Data.EmailID == "" {
		slog.Warn("email webhook missing data.email_id")
		w.WriteHeader(http.StatusOK)
		return
	}

	senderEmail := extractEmailAddress(event.Data.From)
	subject := event.Data.Subject

	slog.Info("email webhook received",
		"from", maskEmail(senderEmail),
		"email_id", event.Data.EmailID,
	)

	// Look up the user by sender email FIRST — this is the cheapest
	// gate and lets us 200-no-op on unknown senders without spending an
	// API call to fetch the body of an email we'd discard anyway.
	user, err := h.queries.GetUserByEmail(r.Context(), senderEmail)
	if err != nil {
		slog.Warn("email webhook unknown sender",
			"email", maskEmail(senderEmail),
			"error", err,
		)
		w.WriteHeader(http.StatusOK)
		return
	}

	// Fetch the actual body from Resend. Resend webhooks intentionally
	// omit the body to keep payloads small — see
	// resend.com/docs/dashboard/webhooks/emails/received.
	received, err := h.inbound.FetchReceived(r.Context(), event.Data.EmailID)
	if err != nil {
		slog.Error("email webhook fetch body failed",
			"user_id", user.ID,
			"email_id", event.Data.EmailID,
			"error", err,
		)
		// Return 500 so Resend retries; transient API failures are
		// recoverable on their next retry.
		http.Error(w, "fetch body failed", http.StatusInternalServerError)
		return
	}

	body := received.Text
	if body == "" {
		body = stripHTMLTags(received.HTML)
	}
	if body == "" {
		slog.Warn("email webhook received empty body",
			"user_id", user.ID,
			"email_id", event.Data.EmailID,
		)
		w.WriteHeader(http.StatusOK)
		return
	}

	slog.Info("email webhook matched user",
		"user_id", user.ID,
		"email", maskEmail(user.Email),
	)

	// Try to match to an existing trip.
	tripID := h.matchTrip(r, user, subject)

	// Email forwarding is available to all users (free and Pro).
	// This is a key utility feature that creates lock-in — once
	// bookings are in Toqui, the AI can build context around them.

	// Include subject line as context for AI parsing.
	fullText := body
	if subject != "" {
		fullText = fmt.Sprintf("Subject: %s\n\n%s", subject, body)
	}

	// Ingest via booking service with source="email".
	result, err := h.bookingSvc.IngestEmail(r.Context(), user.ID, tripID, "", fullText)
	if err != nil {
		slog.Error("email webhook ingest failed",
			"user_id", user.ID,
			"error", err,
		)
		// Return 500 so Resend retries.
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	b := result.Booking

	// Do NOT log booking title — it contains travel content (hotel
	// name, destination, etc). Type + ids are sufficient for
	// operational debugging. See toqui-backend#369 P1 #10.
	slog.Info("email webhook booking ingested",
		"booking_id", b.ID,
		"user_id", user.ID,
		"trip_id", tripID,
		"type", b.Type,
		"was_updated", result.WasUpdated,
	)

	w.WriteHeader(http.StatusOK)
}

// matchTrip attempts to find a trip for the user to associate the booking with.
// Strategy:
//  1. Try to match the email subject against trip titles and destinations.
//  2. Fall back to the most recently created "planning" trip.
//  3. Fall back to the most recently created trip regardless of status.
//  4. If no trips exist, return empty string (booking will be unlinked).
func (h *EmailWebhookHandler) matchTrip(r *http.Request, user dbgen.User, subject string) string {
	ctx := r.Context()

	// Load recent trips for subject-based matching.
	allTrips, _, err := h.tripSvc.ListByUser(ctx, user.ID, "", 20, 0)
	if err != nil || len(allTrips) == 0 {
		slog.Info("email webhook no trip matched, booking will be unlinked",
			"user_id", user.ID,
		)
		return ""
	}

	// Try to match the email subject against trip titles or destinations.
	// A booking confirmation for "Paris" should match a trip titled "Paris Weekend".
	subjectLower := strings.ToLower(subject)
	if subjectLower != "" {
		for _, t := range allTrips {
			titleLower := strings.ToLower(t.Title)
			destLower := ""
			if t.DestinationCountry.Valid {
				destLower = strings.ToLower(t.DestinationCountry.String)
			}
			if (titleLower != "" && strings.Contains(subjectLower, titleLower)) ||
				(destLower != "" && strings.Contains(subjectLower, destLower)) {
				tripID := t.ID.String()
				// `trip_title` and `subject` both leak travel content.
				// Use trip_id + user_id; the match is traceable via DB.
				slog.Info("email webhook matched trip by subject",
					"trip_id", tripID,
					"user_id", user.ID,
				)
				return tripID
			}
		}
	}

	// Fall back: prefer the most recent planning trip.
	for _, t := range allTrips {
		if t.Status == "planning" {
			tripID := t.ID.String()
			slog.Info("email webhook matched planning trip (fallback)",
				"trip_id", tripID,
				"user_id", user.ID,
			)
			return tripID
		}
	}

	// Last resort: most recent trip of any status.
	tripID := allTrips[0].ID.String()
	slog.Info("email webhook matched recent trip (fallback)",
		"trip_id", tripID,
		"user_id", user.ID,
	)
	return tripID
}

// verifySvixSignature implements Svix's webhook verification scheme as
// documented at docs.svix.com/receiving/verifying-payloads/how-manual.
// Resend uses Svix as its webhook delivery layer.
//
// Steps:
//  1. Read svix-id, svix-timestamp, svix-signature headers
//  2. Reject if timestamp is outside ±svixReplayWindow
//  3. Strip "whsec_" prefix from configured secret, base64-decode the
//     remainder to get the HMAC key
//  4. Compute HMAC-SHA256(key, svix_id + "." + svix_timestamp + "." +
//     raw_body), base64-encode
//  5. The svix-signature header is space-separated "v1,<sig>" entries;
//     accept if any matches via constant-time compare
func (h *EmailWebhookHandler) verifySvixSignature(r *http.Request, rawBody []byte) error {
	svixID := r.Header.Get("svix-id")
	svixTimestamp := r.Header.Get("svix-timestamp")
	svixSignature := r.Header.Get("svix-signature")
	if svixID == "" || svixTimestamp == "" || svixSignature == "" {
		return errors.New("missing svix-id / svix-timestamp / svix-signature header")
	}

	tsSeconds, err := strconv.ParseInt(svixTimestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("parse svix-timestamp: %w", err)
	}
	skew := time.Since(time.Unix(tsSeconds, 0))
	if skew < 0 {
		skew = -skew
	}
	if skew > svixReplayWindow {
		return fmt.Errorf("timestamp outside replay window: skew=%s", skew)
	}

	// Decode the secret. Resend secrets ship with a `whsec_` prefix
	// followed by base64. The HMAC key is the decoded bytes.
	secret := strings.TrimPrefix(h.webhookSecret, "whsec_")
	key, err := base64.StdEncoding.DecodeString(secret)
	if err != nil {
		return fmt.Errorf("decode webhook secret (expected whsec_<base64>): %w", err)
	}

	// Compute expected signature (raw HMAC bytes, NOT the base64 form —
	// we compare in raw to keep both sides exactly 32 bytes for
	// constant-time hmac.Equal).
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(svixID))
	mac.Write([]byte("."))
	mac.Write([]byte(svixTimestamp))
	mac.Write([]byte("."))
	mac.Write(rawBody)
	expected := mac.Sum(nil)

	// svix-signature is space-delimited "v1,<base64>" entries. Decode
	// each entry's base64 portion and compare against `expected` in
	// constant time. Comparing the raw 32-byte HMAC outputs (rather
	// than their base64-encoded strings of attacker-controlled length)
	// avoids the length-leak and malformed-input footguns that
	// hmac.Equal does NOT protect against.
	for _, entry := range strings.Fields(svixSignature) {
		_, sigB64, ok := strings.Cut(entry, ",")
		if !ok {
			continue
		}
		sig, err := base64.StdEncoding.DecodeString(sigB64)
		if err != nil {
			// Skip malformed entries — Svix only emits valid base64.
			continue
		}
		if hmac.Equal(sig, expected) {
			return nil
		}
	}
	return errors.New("no svix-signature entry matched expected HMAC")
}

// extractEmailAddress extracts a bare email address from a "Name <email>" string.
// If the input is already a bare email, it is returned as-is.
func extractEmailAddress(from string) string {
	from = strings.TrimSpace(from)
	if from == "" {
		return ""
	}

	// Try to extract from "Name <email>" format.
	if start := strings.LastIndex(from, "<"); start != -1 {
		if end := strings.LastIndex(from, ">"); end > start {
			return strings.TrimSpace(from[start+1 : end])
		}
	}

	return from
}

// stripHTMLTags extracts plain text from an HTML document. Used as a
// fallback when an inbound email has no text/plain part. The input is
// attacker-controlled (anyone can send the user an email), so the
// previous char-by-char tag stripper had real gaps that flowed into
// the AI parsing pipeline:
//
//   - <script>/<style> bodies were emitted as visible text, leaking
//     attacker-controlled JS / CSS source into the AI prompt;
//   - HTML entities (&amp;, &#x...;) were preserved verbatim instead
//     of decoded, so confirmation codes and dates rendered wrong;
//   - HTML comments (<!-- ... -->) were treated as tags rather than
//     skipped, but their content leaked back out when a comment
//     contained '>' chars.
//
// The tokenizer-based pass uses golang.org/x/net/html (already a
// transitive dep). It honours entity decoding for free, suppresses
// the content of script/style/head/title elements, and skips
// comments cleanly. Whitespace runs between tokens are collapsed to
// a single space so the resulting plain text is grep-friendly when
// the AI parser searches for confirmation codes.
func stripHTMLTags(htmlSrc string) string {
	if htmlSrc == "" {
		return ""
	}
	tok := html.NewTokenizer(strings.NewReader(htmlSrc))
	var out strings.Builder
	// Tags whose text content should NOT be emitted. Lowercase.
	skipContent := map[string]bool{
		"script": true,
		"style":  true,
		"head":   true,
		"title":  true,
	}
	var skipDepth int
loop:
	for {
		switch tok.Next() {
		case html.ErrorToken:
			break loop
		case html.StartTagToken:
			name, _ := tok.TagName()
			if skipContent[string(name)] {
				skipDepth++
			}
		case html.EndTagToken:
			name, _ := tok.TagName()
			if skipContent[string(name)] && skipDepth > 0 {
				skipDepth--
			}
		case html.TextToken:
			if skipDepth > 0 {
				continue
			}
			// tok.Text() returns the decoded text — entities are
			// already resolved (&amp; → &, &#x1F389; → 🎉, etc).
			out.Write(tok.Text())
			out.WriteByte(' ')
		case html.SelfClosingTagToken, html.CommentToken, html.DoctypeToken:
			// no-op
		}
	}
	// Collapse repeated whitespace so the AI prompt isn't drowned
	// in formatting artefacts.
	return strings.Join(strings.Fields(out.String()), " ")
}
