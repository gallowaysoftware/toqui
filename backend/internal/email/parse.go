package email

import (
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"regexp"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/text/encoding/htmlindex"
)

// ParsedEmail is the useful content extracted from a raw RFC 822 message
// (or, when the input has no headers, a bare body). FromAddress is the
// lowercased, angle-bracket-stripped sender address — the key the poller
// uses to look up which Toqui user forwarded the booking.
type ParsedEmail struct {
	FromAddress string
	Subject     string
	Body        string // best-effort plain text (text/plain preferred, HTML stripped)
}

// maxBodyBytes caps the extracted body to keep a malicious or runaway
// message from ballooning the AI parse request. The RPC already limits
// raw_email to 500KB; this is the post-extraction guard.
const maxBodyBytes = 200_000

// maxMultipartDepth bounds nested multipart recursion. A hand-crafted
// message can nest multiparts thousands deep within the 500KB cap, and an
// unbounded walk burns seconds of CPU per request. Real mail is 1–2 levels.
const maxMultipartDepth = 12

// Parse extracts the sender, subject, and a plain-text body from a raw
// email. It is deliberately lenient: input that doesn't parse as a
// structured message (no headers) is treated as a bare plain-text body so
// pasted booking text still works. It never returns an error — the worst
// case is an empty body, which the caller treats as "nothing to ingest".
func Parse(raw string) ParsedEmail {
	msg, err := mail.ReadMessage(strings.NewReader(raw))
	if err != nil {
		// Not a structured email — treat the whole thing as the body.
		return ParsedEmail{Body: truncate(collapseBlankLines(raw))}
	}

	dec := new(mime.WordDecoder)
	subject, _ := dec.DecodeHeader(msg.Header.Get("Subject"))
	from := parseFromAddress(msg.Header.Get("From"))

	body := extractBody(msg.Header.Get("Content-Type"), msg.Header.Get("Content-Transfer-Encoding"), msg.Body, 0)

	return ParsedEmail{
		FromAddress: from,
		// ToValidUTF8 is the last-line guard: charset decoding above handles
		// known encodings, but an unknown charset, a truncation mid-rune, or
		// base64 of arbitrary bytes can still yield invalid UTF-8 — which
		// would fail the Postgres TEXT insert (SQLSTATE 22021) downstream.
		Subject: strings.ToValidUTF8(strings.TrimSpace(subject), "�"),
		Body:    strings.ToValidUTF8(truncate(collapseBlankLines(body)), "�"),
	}
}

// parseFromAddress pulls the bare address out of a From header value
// ("Delta <no-reply@delta.com>" → "no-reply@delta.com"), lowercased.
func parseFromAddress(header string) string {
	if header == "" {
		return ""
	}
	if addr, err := mail.ParseAddress(header); err == nil {
		return strings.ToLower(strings.TrimSpace(addr.Address))
	}
	// Fall back to the last <...> group, then the raw value.
	if i := strings.LastIndex(header, "<"); i >= 0 {
		if j := strings.Index(header[i:], ">"); j > 0 {
			return strings.ToLower(strings.TrimSpace(header[i+1 : i+j]))
		}
	}
	return strings.ToLower(strings.TrimSpace(header))
}

// extractBody walks the message body and returns the best plain-text
// content it can find. For multipart/alternative it prefers text/plain and
// falls back to stripped text/html; for a single part it decodes and (if
// HTML) strips it.
func extractBody(contentType, transferEncoding string, body io.Reader, depth int) string {
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		// No / unparseable Content-Type — read as-is with the declared
		// transfer encoding (charset unknown → assume UTF-8/ASCII).
		return decodeText(readAll(body), transferEncoding, "")
	}

	if strings.HasPrefix(mediaType, "multipart/") {
		return extractMultipart(body, params["boundary"], depth)
	}

	text := decodeText(readAll(body), transferEncoding, params["charset"])
	if mediaType == "text/html" {
		return stripHTML(text)
	}
	return text
}

// extractMultipart prefers a text/plain part; it keeps the first stripped
// text/html part as a fallback and recurses into nested multiparts up to
// maxMultipartDepth.
func extractMultipart(body io.Reader, boundary string, depth int) string {
	if boundary == "" || depth >= maxMultipartDepth {
		return ""
	}
	mr := multipart.NewReader(body, boundary)
	var htmlFallback string
	for {
		part, err := mr.NextPart()
		if err != nil {
			break
		}
		partType, partParams, _ := mime.ParseMediaType(part.Header.Get("Content-Type"))
		enc := part.Header.Get("Content-Transfer-Encoding")

		switch {
		case strings.HasPrefix(partType, "multipart/"):
			if nested := extractMultipart(part, partParams["boundary"], depth+1); nested != "" {
				_ = part.Close()
				return nested
			}
		case partType == "text/plain":
			if text := decodeText(readAll(part), enc, partParams["charset"]); strings.TrimSpace(text) != "" {
				_ = part.Close()
				return text // plain text wins immediately
			}
		case partType == "text/html":
			if htmlFallback == "" {
				htmlFallback = stripHTML(decodeText(readAll(part), enc, partParams["charset"]))
			}
		}
		_ = part.Close()
	}
	return htmlFallback
}

// decodeText undoes the Content-Transfer-Encoding, then converts the bytes
// from the part's charset to UTF-8 so non-UTF-8 confirmations (latin-1,
// windows-1252, shift_jis, gb2312, …) don't reach the DB as invalid bytes.
func decodeText(raw, encoding, charset string) string {
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "quoted-printable":
		if decoded, err := io.ReadAll(quotedprintable.NewReader(strings.NewReader(raw))); err == nil {
			raw = string(decoded)
		}
	case "base64":
		// Mail base64 is line-wrapped; strip whitespace before decoding.
		clean := strings.NewReplacer("\r", "", "\n", "", " ", "").Replace(raw)
		if decoded, err := base64.StdEncoding.DecodeString(clean); err == nil {
			raw = string(decoded)
		}
	}
	return toUTF8(raw, charset)
}

// toUTF8 converts s from the named charset to UTF-8. Unknown/empty charsets
// and already-UTF-8 content pass through unchanged (the ToValidUTF8 guard in
// Parse scrubs anything still invalid).
func toUTF8(s, charset string) string {
	charset = strings.ToLower(strings.TrimSpace(charset))
	if charset == "" || charset == "utf-8" || charset == "utf8" || charset == "us-ascii" || charset == "ascii" {
		return s
	}
	enc, err := htmlindex.Get(charset)
	if err != nil || enc == nil {
		return s
	}
	decoded, err := enc.NewDecoder().String(s)
	if err != nil {
		return s
	}
	return decoded
}

func readAll(r io.Reader) string {
	// Bound the read so a huge part can't exhaust memory — the caller
	// truncates again, but stop early here.
	b, _ := io.ReadAll(io.LimitReader(r, maxBodyBytes*2))
	return string(b)
}

// stripHTML renders an HTML fragment to readable plain text: drops
// script/style, turns block elements and <br> into line breaks, and
// collapses whitespace runs.
func stripHTML(input string) string {
	node, err := html.Parse(strings.NewReader(input))
	if err != nil {
		return input
	}
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "script", "style", "head":
				return
			case "br":
				sb.WriteByte('\n')
			}
		}
		if n.Type == html.TextNode {
			sb.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
		if n.Type == html.ElementNode && isBlockElement(n.Data) {
			sb.WriteByte('\n')
		}
	}
	walk(node)
	return html.UnescapeString(sb.String())
}

func isBlockElement(tag string) bool {
	switch tag {
	case "p", "div", "tr", "table", "li", "ul", "ol", "h1", "h2", "h3", "h4", "h5", "h6", "section", "article", "header", "footer", "blockquote":
		return true
	}
	return false
}

var blankLineRun = regexp.MustCompile(`\n[ \t]*\n([ \t]*\n)+`)

// collapseBlankLines trims trailing whitespace per line and squeezes runs
// of blank lines to a single blank line — HTML stripping leaves a lot of
// empty lines that waste AI tokens.
func collapseBlankLines(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t\r")
	}
	joined := strings.Join(lines, "\n")
	joined = blankLineRun.ReplaceAllString(joined, "\n\n")
	return strings.TrimSpace(joined)
}

func truncate(s string) string {
	if len(s) <= maxBodyBytes {
		return s
	}
	return s[:maxBodyBytes]
}
