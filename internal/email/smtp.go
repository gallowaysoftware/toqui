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
		slog.Error("email send failed", "to", to, "subject", subject, "error", err)
		return fmt.Errorf("send email: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		slog.Error("email send failed", "to", to, "subject", subject, "status", resp.StatusCode, "body", string(respBody))
		return fmt.Errorf("resend API error %d: %s", resp.StatusCode, string(respBody))
	}

	slog.Info("email sent", "to", to, "subject", subject)
	return nil
}

// SendInvite sends an invite email with a link to sign up.
func (s *Sender) SendInvite(to, inviteCode, appURL string) error {
	subject := "You're invited to Toqui!"
	body := fmt.Sprintf(`You've been invited to try Toqui — your AI travel companion!

Use your personal invite code to get started:

    %s

Or click this link to sign up directly:

%s/waitlist?invite_code=%s

See you on the road!

— The Toqui Team
https://toqui.travel`, inviteCode, appURL, inviteCode)

	return s.Send(to, subject, body)
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

// SendVerification sends a waitlist verification email.
func (s *Sender) SendVerification(to, verifyURL string) error {
	subject := "Verify your Toqui waitlist signup"
	body := fmt.Sprintf(`Hey there!

Thanks for joining the Toqui waitlist. Please verify your email by clicking the link below:

%s

If you didn't sign up for Toqui, you can safely ignore this email.

— The Toqui Team
https://toqui.travel`, verifyURL)

	return s.Send(to, subject, body)
}
