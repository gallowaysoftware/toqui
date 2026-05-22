// Package email provides transactional email sending via Resend API.
package email

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// Sender sends transactional emails via Resend.
type Sender struct {
	apiKey string
	from   string
	client *http.Client
}

// NewSender creates a new Resend email sender.
func NewSender(apiKey, from string) *Sender {
	return &Sender{
		apiKey: apiKey,
		from:   from,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

type resendRequest struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Subject string `json:"subject"`
	Text    string `json:"text"`
}

// Send sends a plain-text email via Resend.
func (s *Sender) Send(to, subject, body string) error {
	payload, _ := json.Marshal(resendRequest{
		From:    s.from,
		To:      to,
		Subject: subject,
		Text:    body,
	})

	req, err := http.NewRequest(http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		// Recipient is masked because subject can include personal names
		// for collab invites ("X invited you to collaborate on ...") and
		// recipient is PII; both flow into Cloud Logging unredacted otherwise.
		slog.Error("email send failed", "to", MaskEmail(to), "error", err)
		return fmt.Errorf("send email: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		slog.Error("email send failed", "to", MaskEmail(to), "status", resp.StatusCode, "body", string(respBody))
		return fmt.Errorf("resend API error %d: %s", resp.StatusCode, string(respBody))
	}

	slog.Info("email sent", "to", MaskEmail(to))
	return nil
}

// SendWelcome sends a welcome email to new users.
func (s *Sender) SendWelcome(to, name, appURL string) error {
	greeting := "Hey there"
	if name != "" {
		greeting = fmt.Sprintf("Hey %s", name)
	}
	subject := "Welcome to Toqui — your AI travel companion"
	body := fmt.Sprintf(`%s!

Welcome to Toqui. Here's how to plan your first trip:

1. Tell Toqui where you want to go — just start chatting
2. Meet your expert personas — food guides, history buffs, adventure specialists matched to your destination
3. Get a day-by-day itinerary and export it to your calendar

Your first trip includes a 3-day Pro trial with unlimited expert access.

Start planning: %s

Happy travels!

— The Toqui Team
https://toqui.travel`, greeting, appURL)

	return s.Send(to, subject, body)
}

// SendCollabInvite sends a trip collaboration invite email.
func (s *Sender) SendCollabInvite(to, inviterName, tripTitle, acceptURL string) error {
	subject := fmt.Sprintf("%s invited you to collaborate on a trip", inviterName)
	body := fmt.Sprintf(`Hey there!

%s has invited you to collaborate on their trip "%s" on Toqui.

Click the link below to accept the invite:

%s

If you don't have a Toqui account yet, you'll be prompted to sign up first.

Happy travels!

— The Toqui Team
https://toqui.travel`, inviterName, tripTitle, acceptURL)

	return s.Send(to, subject, body)
}

