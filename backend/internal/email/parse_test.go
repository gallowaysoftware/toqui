package email

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestParse_PlainTextEmail(t *testing.T) {
	raw := "From: Delta Air Lines <no-reply@delta.com>\r\n" +
		"Subject: Your flight confirmation ABC123\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"\r\n" +
		"Confirmation: ABC123\r\nJFK to BCN, departs 2027-05-01.\r\n"

	got := Parse(raw)
	if got.FromAddress != "no-reply@delta.com" {
		t.Errorf("FromAddress = %q, want no-reply@delta.com", got.FromAddress)
	}
	if got.Subject != "Your flight confirmation ABC123" {
		t.Errorf("Subject = %q", got.Subject)
	}
	if !strings.Contains(got.Body, "Confirmation: ABC123") || !strings.Contains(got.Body, "JFK to BCN") {
		t.Errorf("Body missing content: %q", got.Body)
	}
}

func TestParse_BareBodyNoHeaders(t *testing.T) {
	// A pasted confirmation with no email headers must still yield a body.
	raw := "Hotel Arts Barcelona\nCheck-in 2027-05-02\nConfirmation HTL-9988"
	got := Parse(raw)
	if got.FromAddress != "" {
		t.Errorf("FromAddress = %q, want empty for a bare body", got.FromAddress)
	}
	if !strings.Contains(got.Body, "Hotel Arts Barcelona") {
		t.Errorf("Body = %q", got.Body)
	}
}

func TestParse_QuotedPrintable(t *testing.T) {
	raw := "From: a@b.com\r\n" +
		"Subject: Test\r\n" +
		"Content-Type: text/plain\r\n" +
		"Content-Transfer-Encoding: quoted-printable\r\n" +
		"\r\n" +
		"Caf=C3=A9 booking for=\r\n two guests\r\n"
	got := Parse(raw)
	if !strings.Contains(got.Body, "Café booking for two guests") {
		t.Errorf("quoted-printable not decoded: %q", got.Body)
	}
}

func TestParse_Base64(t *testing.T) {
	// "Booking OK" base64 = Qm9va2luZyBPSw==
	raw := "From: a@b.com\r\n" +
		"Subject: Test\r\n" +
		"Content-Type: text/plain\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"\r\n" +
		"Qm9va2luZyBPSw==\r\n"
	got := Parse(raw)
	if !strings.Contains(got.Body, "Booking OK") {
		t.Errorf("base64 not decoded: %q", got.Body)
	}
}

func TestParse_MultipartAlternativePrefersPlain(t *testing.T) {
	raw := "From: Hotel <res@hotel.com>\r\n" +
		"Subject: Reservation\r\n" +
		"Content-Type: multipart/alternative; boundary=\"BOUND\"\r\n" +
		"\r\n" +
		"--BOUND\r\n" +
		"Content-Type: text/plain\r\n\r\n" +
		"PLAIN: your room is booked\r\n" +
		"--BOUND\r\n" +
		"Content-Type: text/html\r\n\r\n" +
		"<html><body><p>HTML: your room is booked</p></body></html>\r\n" +
		"--BOUND--\r\n"
	got := Parse(raw)
	if !strings.Contains(got.Body, "PLAIN: your room is booked") {
		t.Errorf("expected plain part, got %q", got.Body)
	}
	if strings.Contains(got.Body, "HTML:") {
		t.Errorf("HTML part should have been ignored: %q", got.Body)
	}
}

func TestParse_MultipartHTMLFallbackStripped(t *testing.T) {
	// Only an HTML part present — it must be stripped to readable text.
	raw := "From: a@b.com\r\n" +
		"Subject: Reservation\r\n" +
		"Content-Type: multipart/alternative; boundary=\"B\"\r\n" +
		"\r\n" +
		"--B\r\n" +
		"Content-Type: text/html\r\n\r\n" +
		"<html><head><style>p{color:red}</style></head><body>" +
		"<p>Confirmation <b>XYZ789</b></p><script>alert(1)</script>" +
		"<div>Room 402</div></body></html>\r\n" +
		"--B--\r\n"
	got := Parse(raw)
	if !strings.Contains(got.Body, "Confirmation XYZ789") {
		t.Errorf("HTML text not extracted: %q", got.Body)
	}
	if !strings.Contains(got.Body, "Room 402") {
		t.Errorf("block element not separated: %q", got.Body)
	}
	if strings.Contains(got.Body, "alert(1)") || strings.Contains(got.Body, "color:red") {
		t.Errorf("script/style leaked into body: %q", got.Body)
	}
}

func TestParse_EncodedSubject(t *testing.T) {
	raw := "From: a@b.com\r\n" +
		"Subject: =?utf-8?q?Caf=C3=A9_Booking?=\r\n" +
		"Content-Type: text/plain\r\n\r\nbody\r\n"
	got := Parse(raw)
	if got.Subject != "Café Booking" {
		t.Errorf("encoded subject not decoded: %q", got.Subject)
	}
}

func TestParse_FromAddressBareAndAngle(t *testing.T) {
	cases := []struct{ header, want string }{
		{"From: plain@example.com\r\nSubject: s\r\n\r\nb\r\n", "plain@example.com"},
		{"From: Name <UPPER@Example.COM>\r\nSubject: s\r\n\r\nb\r\n", "upper@example.com"},
	}
	for _, c := range cases {
		if got := Parse(c.header).FromAddress; got != c.want {
			t.Errorf("Parse(%q).FromAddress = %q, want %q", c.header, got, c.want)
		}
	}
}

func TestParse_CollapsesBlankLines(t *testing.T) {
	raw := "line1\n\n\n\n\nline2\n\n\n"
	got := Parse(raw)
	if got.Body != "line1\n\nline2" {
		t.Errorf("blank lines not collapsed: %q", got.Body)
	}
}

func TestParse_Latin1CharsetDecoded(t *testing.T) {
	// "Café" in ISO-8859-1: 'é' is the single byte 0xE9.
	raw := "From: a@b.com\r\n" +
		"Subject: Test\r\n" +
		"Content-Type: text/plain; charset=iso-8859-1\r\n" +
		"\r\n" +
		"Caf\xe9 reservation confirmed\r\n"
	got := Parse(raw)
	if !strings.Contains(got.Body, "Café reservation confirmed") {
		t.Errorf("latin-1 not decoded to UTF-8: %q", got.Body)
	}
	if !utf8.ValidString(got.Body) {
		t.Errorf("body is not valid UTF-8: %q", got.Body)
	}
}

func TestParse_Windows1252QuotedPrintable(t *testing.T) {
	raw := "From: a@b.com\r\n" +
		"Subject: Test\r\n" +
		"Content-Type: text/plain; charset=windows-1252\r\n" +
		"Content-Transfer-Encoding: quoted-printable\r\n" +
		"\r\n" +
		"Caf=E9 booking\r\n"
	got := Parse(raw)
	if !strings.Contains(got.Body, "Café booking") {
		t.Errorf("windows-1252 QP not decoded: %q", got.Body)
	}
	if !utf8.ValidString(got.Body) {
		t.Errorf("body not valid UTF-8: %q", got.Body)
	}
}

func TestParse_Base64BinaryStaysValidUTF8(t *testing.T) {
	// base64 of the two bytes 0xFF 0xFE — invalid UTF-8. Must not reach
	// the caller as invalid bytes (would crash the Postgres TEXT insert).
	raw := "From: a@b.com\r\n" +
		"Subject: Test\r\n" +
		"Content-Type: text/plain\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"\r\n" +
		"//4=\r\n"
	got := Parse(raw)
	if !utf8.ValidString(got.Body) {
		t.Errorf("base64 binary produced invalid UTF-8: %q", got.Body)
	}
	if !utf8.ValidString(got.Subject) {
		t.Errorf("subject not valid UTF-8: %q", got.Subject)
	}
}

func TestParse_DeeplyNestedMultipartBounded(t *testing.T) {
	// Nest multiparts far deeper than the cap; Parse must return quickly
	// (recursion is bounded) without hanging or crashing. We can't easily
	// assert wall-clock here, but reaching the assertion at all proves the
	// depth guard terminated the walk.
	var sb strings.Builder
	sb.WriteString("From: a@b.com\r\nSubject: nested\r\n")
	depth := 40
	for i := 0; i < depth; i++ {
		sb.WriteString("Content-Type: multipart/mixed; boundary=\"b")
		sb.WriteString(string(rune('A' + i%26)))
		sb.WriteString("\"\r\n\r\n--b")
		sb.WriteString(string(rune('A' + i%26)))
		sb.WriteString("\r\n")
	}
	sb.WriteString("Content-Type: text/plain\r\n\r\ndeep body\r\n")
	got := Parse(sb.String())
	// Body past the depth cap is not extracted — that's the intended
	// trade-off; the point is it returns rather than burning CPU forever.
	if !utf8.ValidString(got.Body) {
		t.Errorf("nested body not valid UTF-8: %q", got.Body)
	}
}
