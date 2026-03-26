// Package email provides transactional email sending via SMTP.
package email

import (
	"fmt"
	"log/slog"
	"net/smtp"
	"strings"
)

// Sender sends transactional emails via SMTP.
type Sender struct {
	host     string
	port     string
	username string
	password string
	from     string
}

// NewSender creates a new SMTP email sender.
// For Google Workspace: host=smtp.gmail.com, port=587, username=hello@toqui.travel, password=app-password.
func NewSender(host, port, username, password, from string) *Sender {
	return &Sender{
		host:     host,
		port:     port,
		username: username,
		password: password,
		from:     from,
	}
}

// Send sends a plain-text email.
func (s *Sender) Send(to, subject, body string) error {
	auth := smtp.PlainAuth("", s.username, s.password, s.host)

	msg := strings.Join([]string{
		"From: " + s.from,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		body,
	}, "\r\n")

	addr := s.host + ":" + s.port
	if err := smtp.SendMail(addr, auth, s.from, []string{to}, []byte(msg)); err != nil {
		slog.Error("email send failed", "to", to, "subject", subject, "error", err)
		return fmt.Errorf("send email: %w", err)
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
