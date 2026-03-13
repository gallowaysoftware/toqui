package handlers

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gallowaysoftware/toqui-backend/internal/booking"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui-backend/internal/trip"
)

// maxEmailBodySize limits the multipart form data to 25 MB (SendGrid max attachment size).
const maxEmailBodySize = 25 << 20

// EmailWebhookHandler handles inbound email webhooks from SendGrid Inbound Parse.
type EmailWebhookHandler struct {
	bookingSvc *booking.Service
	tripSvc    *trip.Service
	queries    *dbgen.Queries
	webhookKey string // SendGrid Inbound Parse webhook verification public key (PEM)
}

// NewEmailWebhookHandler creates a new handler for inbound email webhooks.
func NewEmailWebhookHandler(bookingSvc *booking.Service, tripSvc *trip.Service, pool *pgxpool.Pool, webhookKey string) *EmailWebhookHandler {
	return &EmailWebhookHandler{
		bookingSvc: bookingSvc,
		tripSvc:    tripSvc,
		queries:    dbgen.New(pool),
		webhookKey: webhookKey,
	}
}

// HandleInbound processes inbound email webhooks from SendGrid Inbound Parse.
// Route: POST /webhooks/email/inbound
//
// SendGrid sends multipart/form-data with fields:
//   - from: sender email (e.g. "Jane Doe <jane@example.com>")
//   - to: recipient email
//   - subject: email subject line
//   - text: plain text body
//   - html: HTML body
//   - envelope: JSON with "from" and "to" arrays
func (h *EmailWebhookHandler) HandleInbound(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Verify webhook signature if key is configured.
	if h.webhookKey != "" {
		if err := h.verifySignature(r); err != nil {
			slog.Warn("email webhook signature verification failed", "error", err)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}

	// Parse multipart form data.
	if err := r.ParseMultipartForm(maxEmailBodySize); err != nil {
		slog.Error("email webhook parse form failed", "error", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	senderEmail := extractEmailAddress(r.FormValue("from"))
	subject := r.FormValue("subject")
	textBody := r.FormValue("text")
	htmlBody := r.FormValue("html")

	slog.Info("email webhook received",
		"from", senderEmail,
		"subject", subject,
		"text_len", len(textBody),
		"html_len", len(htmlBody),
	)

	// Determine the email body to use (prefer plain text, fall back to HTML).
	body := textBody
	if body == "" {
		body = stripHTMLTags(htmlBody)
	}
	if body == "" {
		slog.Warn("email webhook received empty body", "from", senderEmail, "subject", subject)
		// Return 200 so SendGrid does not retry.
		w.WriteHeader(http.StatusOK)
		return
	}

	// Look up the user by sender email.
	user, err := h.queries.GetUserByEmail(r.Context(), senderEmail)
	if err != nil {
		slog.Warn("email webhook unknown sender",
			"email", maskEmail(senderEmail),
			"error", err,
		)
		// Return 200 to prevent SendGrid from retrying for unknown senders.
		w.WriteHeader(http.StatusOK)
		return
	}

	slog.Info("email webhook matched user",
		"user_id", user.ID,
		"email", maskEmail(user.Email),
	)

	// Try to match to an existing trip.
	tripID := h.matchTrip(r, user, subject)

	// Include subject line as context for AI parsing.
	fullText := body
	if subject != "" {
		fullText = fmt.Sprintf("Subject: %s\n\n%s", subject, body)
	}

	// Ingest via booking service with source="email".
	b, err := h.bookingSvc.IngestEmail(r.Context(), user.ID, tripID, "", fullText)
	if err != nil {
		slog.Error("email webhook ingest failed",
			"user_id", user.ID,
			"error", err,
		)
		// Return 500 so SendGrid retries.
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	slog.Info("email webhook booking created",
		"booking_id", b.ID,
		"user_id", user.ID,
		"trip_id", tripID,
		"type", b.Type,
		"title", b.Title,
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
				slog.Info("email webhook matched trip by subject",
					"trip_id", tripID,
					"trip_title", t.Title,
					"subject", subject,
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
				"trip_title", t.Title,
				"user_id", user.ID,
			)
			return tripID
		}
	}

	// Last resort: most recent trip of any status.
	tripID := allTrips[0].ID.String()
	slog.Info("email webhook matched recent trip (fallback)",
		"trip_id", tripID,
		"trip_title", allTrips[0].Title,
		"user_id", user.ID,
	)
	return tripID
}

// verifySignature validates the SendGrid Event Webhook ECDSA signature.
// The signature is computed over: timestamp + raw_body.
//
// This method reads the raw body for verification, then restores r.Body
// so ParseMultipartForm can re-read it.
//
// Headers:
//   - X-Twilio-Email-Event-Webhook-Signature: base64-encoded ECDSA signature
//   - X-Twilio-Email-Event-Webhook-Timestamp: timestamp string
func (h *EmailWebhookHandler) verifySignature(r *http.Request) error {
	signature := r.Header.Get("X-Twilio-Email-Event-Webhook-Signature")
	timestamp := r.Header.Get("X-Twilio-Email-Event-Webhook-Timestamp")

	if signature == "" || timestamp == "" {
		return fmt.Errorf("missing signature headers")
	}

	// Read the raw body for signature verification (bounded to max size).
	rawBody, err := io.ReadAll(io.LimitReader(r.Body, maxEmailBodySize))
	if err != nil {
		return fmt.Errorf("read body for verification: %w", err)
	}
	r.Body.Close()
	// Restore body so ParseMultipartForm can re-read it.
	r.Body = io.NopCloser(bytes.NewReader(rawBody))

	// Decode the PEM public key.
	block, _ := pem.Decode([]byte(h.webhookKey))
	if block == nil {
		return fmt.Errorf("invalid PEM public key")
	}

	pubKeyInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("parse public key: %w", err)
	}

	ecdsaKey, ok := pubKeyInterface.(*ecdsa.PublicKey)
	if !ok {
		return fmt.Errorf("public key is not ECDSA")
	}

	// Decode the base64 signature.
	sigBytes, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}

	// The signed payload is: timestamp + raw_body.
	payload := make([]byte, 0, len(timestamp)+len(rawBody))
	payload = append(payload, timestamp...)
	payload = append(payload, rawBody...)
	hash := sha256.Sum256(payload)

	// Parse the ECDSA signature as r || s concatenated values.
	keySize := (ecdsaKey.Params().BitSize + 7) / 8
	if len(sigBytes) != 2*keySize {
		return fmt.Errorf("unexpected signature length: got %d, expected %d", len(sigBytes), 2*keySize)
	}

	rVal := new(big.Int).SetBytes(sigBytes[:keySize])
	sVal := new(big.Int).SetBytes(sigBytes[keySize:])

	if !ecdsa.Verify(ecdsaKey, hash[:], rVal, sVal) {
		return fmt.Errorf("ECDSA signature verification failed")
	}

	return nil
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

// stripHTMLTags removes HTML tags from a string, providing a basic plain-text
// extraction. This is a simple implementation for fallback when plain text is
// not available; it does not handle all HTML edge cases.
func stripHTMLTags(html string) string {
	if html == "" {
		return ""
	}

	var result strings.Builder
	inTag := false
	for _, r := range html {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			result.WriteRune(r)
		}
	}
	return strings.TrimSpace(result.String())
}
