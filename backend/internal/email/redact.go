package email

import "strings"

// MaskEmail obscures the local part of an email address for safe logging.
// "john.doe@example.com" → "j***@example.com"
// Returns the input unchanged if it doesn't contain "@".
//
// Splits on the LAST "@" to keep the domain stable even for adversarial
// quoted-localpart inputs like "a@b@example.com" (RFC-5321 allows this).
// Indexes the localpart by rune so multi-byte first characters
// ("é***@…") aren't truncated mid-codepoint.
func MaskEmail(email string) string {
	at := strings.LastIndex(email, "@")
	if at <= 0 || at == len(email)-1 {
		return email
	}
	local := email[:at]
	domain := email[at+1:]
	runes := []rune(local)
	if len(runes) == 0 {
		return email
	}
	return string(runes[0]) + "***@" + domain
}
