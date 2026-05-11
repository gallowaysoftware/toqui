package email

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// ReceivedEmail is the parsed body of an inbound email retrieved from
// Resend's GET /emails/receiving/{id} API. Resend's webhooks deliver
// metadata only — to actually parse a booking we have to fetch the
// body separately, which this struct represents.
//
// `Text` and `HTML` may both be empty; callers should fall back from
// text → html → "" the same way the email webhook handler does.
type ReceivedEmail struct {
	ID      string   `json:"id"`
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	Text    string   `json:"text"`
	HTML    string   `json:"html"`
}

// Inbound is a thin Resend client for retrieving received emails. The
// webhook handler uses this after verifying a Resend webhook signature
// to fetch the body that Resend's webhook intentionally omits.
type Inbound struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewInbound creates a Resend received-email fetcher.
func NewInbound(apiKey string) *Inbound {
	return &Inbound{
		apiKey:  apiKey,
		baseURL: "https://api.resend.com",
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

// FetchReceived retrieves a received email by its Resend ID. Network
// failures and non-2xx statuses are returned as errors so the caller
// can decide whether to retry (return 5xx to Resend) or bail out (200
// no-op).
func (i *Inbound) FetchReceived(ctx context.Context, emailID string) (*ReceivedEmail, error) {
	if i.apiKey == "" {
		return nil, fmt.Errorf("resend api key not configured")
	}
	if emailID == "" {
		return nil, fmt.Errorf("email id required")
	}

	url := fmt.Sprintf("%s/emails/receiving/%s", i.baseURL, emailID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+i.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := i.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch received email: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 25<<20))
	if err != nil {
		return nil, fmt.Errorf("read received email body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Don't include the raw body in the error since an upstream HTML
		// error page could contain unrelated content. Caller logs the
		// status code only.
		slog.WarnContext(ctx, "resend received-email fetch non-2xx",
			"status", resp.StatusCode,
			"email_id", emailID,
		)
		return nil, fmt.Errorf("resend status %d", resp.StatusCode)
	}

	var out ReceivedEmail
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("decode received email: %w", err)
	}
	return &out, nil
}
