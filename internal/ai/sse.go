package ai

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
)

// SSEReader reads Server-Sent Events from an HTTP response body.
// Both Claude and Gemini Vertex AI use the SSE format with "data: " prefixed lines.
// This shared implementation ensures consistent parsing and proper error checking.
type SSEReader struct {
	scanner *bufio.Scanner
}

// NewSSEReader creates a new SSE reader from an io.Reader (typically an HTTP response body).
func NewSSEReader(r io.Reader) *SSEReader {
	return &SSEReader{scanner: bufio.NewScanner(r)}
}

// Next returns the next SSE data payload as a raw string.
// It skips blank lines, event type lines, and handles the [DONE] sentinel.
//
// Returns:
//   - (data, false, nil) for a normal data line
//   - ("", true, nil) when the stream is finished ([DONE] or EOF)
//   - ("", false, err) on context cancellation or read error
func (s *SSEReader) Next(ctx context.Context) (data string, done bool, err error) {
	for s.scanner.Scan() {
		select {
		case <-ctx.Done():
			return "", false, ctx.Err()
		default:
		}

		line := s.scanner.Text()

		// Skip blank lines and non-data lines (e.g., "event: ..." or "id: ...").
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		payload := strings.TrimPrefix(line, "data: ")

		// Claude uses [DONE] as a sentinel. Gemini just closes the connection.
		if payload == "[DONE]" {
			return "", true, nil
		}

		return payload, false, nil
	}

	// Scanner finished — check for read errors.
	if err := s.scanner.Err(); err != nil {
		return "", false, fmt.Errorf("stream read: %w", err)
	}

	// Clean EOF — stream ended normally.
	return "", true, nil
}
