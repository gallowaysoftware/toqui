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

	body := extractBody(msg.Header.Get("Content-Type"), msg.Header.Get("Content-Transfer-Encoding"), msg.Body)

	return ParsedEmail{
		FromAddress: from,
		Subject:     strings.TrimSpace(subject),
		Body:        truncate(collapseBlankLines(body)),
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
func extractBody(contentType, transferEncoding string, body io.Reader) string {
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		// No / unparseable Content-Type — read as-is with the declared
		// transfer encoding.
		return decodeText(readAll(body), transferEncoding)
	}

	if strings.HasPrefix(mediaType, "multipart/") {
		return extractMultipart(body, params["boundary"])
	}

	text := decodeText(readAll(body), transferEncoding)
	if mediaType == "text/html" {
		return stripHTML(text)
	}
	return text
}

// extractMultipart prefers a text/plain part; it keeps the first stripped
// text/html part as a fallback and recurses into nested multiparts.
func extractMultipart(body io.Reader, boundary string) string {
	if boundary == "" {
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
			if nested := extractMultipart(part, partParams["boundary"]); nested != "" {
				return nested
			}
		case partType == "text/plain":
			if text := decodeText(readAll(part), enc); strings.TrimSpace(text) != "" {
				return text // plain text wins immediately
			}
		case partType == "text/html":
			if htmlFallback == "" {
				htmlFallback = stripHTML(decodeText(readAll(part), enc))
			}
		}
		_ = part.Close()
	}
	return htmlFallback
}

func decodeText(raw, encoding string) string {
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "quoted-printable":
		if decoded, err := io.ReadAll(quotedprintable.NewReader(strings.NewReader(raw))); err == nil {
			return string(decoded)
		}
	case "base64":
		// Mail base64 is line-wrapped; strip whitespace before decoding.
		clean := strings.NewReplacer("\r", "", "\n", "", " ", "").Replace(raw)
		if decoded, err := base64.StdEncoding.DecodeString(clean); err == nil {
			return string(decoded)
		}
	}
	return raw
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
