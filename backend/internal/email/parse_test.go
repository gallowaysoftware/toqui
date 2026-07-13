package email

import (
	"strings"
	"testing"
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
