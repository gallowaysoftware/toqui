package ai

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

// SSEReader is shared by Claude + Gemini Vertex AI SSE streams. A bug
// here (truncated payloads, dropped [DONE], lost data lines) silently
// corrupts every chat response. Each test pins one specific contract.

func TestSSEReader_BasicDataLine(t *testing.T) {
	body := strings.NewReader("data: hello world\n\n")
	r := NewSSEReader(body)

	data, done, err := r.Next(context.Background())
	if err != nil {
		t.Fatalf("Next() errored: %v", err)
	}
	if done {
		t.Error("Next() returned done=true on first data line")
	}
	if data != "hello world" {
		t.Errorf("Next() data = %q, want %q", data, "hello world")
	}
}

func TestSSEReader_DoneSentinel(t *testing.T) {
	// Claude marks end-of-stream with `data: [DONE]`. The reader MUST
	// translate that to (done=true, "") rather than passing the literal
	// "[DONE]" through as a data payload — downstream parsers would
	// try to JSON-decode it and crash.
	body := strings.NewReader("data: first\n\ndata: [DONE]\n\n")
	r := NewSSEReader(body)

	// First call: real data
	data, done, _ := r.Next(context.Background())
	if data != "first" || done {
		t.Errorf("first call: data=%q done=%v, want first/false", data, done)
	}

	// Second call: [DONE] sentinel
	data, done, err := r.Next(context.Background())
	if err != nil {
		t.Errorf("Next() at [DONE] errored: %v", err)
	}
	if !done {
		t.Errorf("Next() at [DONE] done=%v, want true", done)
	}
	if data != "" {
		t.Errorf("Next() at [DONE] data=%q, want empty (NOT \"[DONE]\")", data)
	}
}

func TestSSEReader_EOFWithoutDoneSentinel(t *testing.T) {
	// Gemini doesn't send [DONE] — it just closes the connection.
	// EOF without a sentinel must return (done=true, nil err) so the
	// caller can finish gracefully rather than treating it as a stream
	// error.
	body := strings.NewReader("data: only line\n\n")
	r := NewSSEReader(body)

	// Drain the data
	_, _, _ = r.Next(context.Background())
	// Next call hits EOF
	data, done, err := r.Next(context.Background())
	if err != nil {
		t.Errorf("EOF without [DONE] errored: %v", err)
	}
	if !done {
		t.Errorf("EOF without [DONE] done=%v, want true", done)
	}
	if data != "" {
		t.Errorf("EOF data=%q, want empty", data)
	}
}

func TestSSEReader_SkipsNonDataLines(t *testing.T) {
	// SSE supports "event:", "id:", "retry:" lines plus blank
	// separators. Reader must skip all of those and only surface
	// data: payloads. Pin this so a future contributor doesn't
	// accidentally surface event-type lines as data.
	body := strings.NewReader(strings.Join([]string{
		"event: message_start",
		"id: 12345",
		"",
		": this is a comment",
		"data: payload-one",
		"",
		"event: content_block_delta",
		"data: payload-two",
		"",
	}, "\n") + "\n")
	r := NewSSEReader(body)

	got := []string{}
	for {
		data, done, err := r.Next(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if done {
			break
		}
		got = append(got, data)
	}
	want := []string{"payload-one", "payload-two"}
	if len(got) != len(want) {
		t.Fatalf("got %d data payloads, want %d: %v", len(got), len(want), got)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("data[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestSSEReader_ContextCancellation(t *testing.T) {
	// A cancelled context during Next() must surface ctx.Err() so the
	// chat handler can short-circuit cleanly rather than hanging on a
	// half-read scanner.
	body := strings.NewReader("data: would-be-payload\n\n")
	r := NewSSEReader(body)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel BEFORE the read

	_, _, err := r.Next(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled ctx error = %v, want context.Canceled", err)
	}
}

func TestSSEReader_LargePayload_NoTruncation(t *testing.T) {
	// Bug-class this defends against: bufio.Scanner's default 64KB
	// buffer silently truncates oversized lines without erroring.
	// Tool-call JSON for the recommend_booking tool can exceed 64KB
	// when a Pro user gets 5+ recommendations with rationale text.
	// The reader uses a 1MB buffer; pin that a 200KB line round-trips
	// intact so a future revert to the default buffer is caught.
	const targetSize = 200 * 1024
	bigData := strings.Repeat("X", targetSize)
	body := strings.NewReader("data: " + bigData + "\n\n")
	r := NewSSEReader(body)

	data, done, err := r.Next(context.Background())
	if err != nil {
		t.Fatalf("Next() on large payload errored: %v", err)
	}
	if done {
		t.Error("Next() returned done=true on large data line")
	}
	if len(data) != targetSize {
		t.Errorf("data len = %d, want %d (Scanner truncating?)", len(data), targetSize)
	}
}

// errorReader returns a fixed read error after delivering some bytes.
// Used to simulate a network drop mid-stream.
type errorReader struct {
	prefix string
	idx    int
	err    error
}

func (r *errorReader) Read(p []byte) (int, error) {
	if r.idx < len(r.prefix) {
		n := copy(p, r.prefix[r.idx:])
		r.idx += n
		return n, nil
	}
	return 0, r.err
}

func TestSSEReader_ReadError_WrappedAndPropagated(t *testing.T) {
	// A network drop mid-stream surfaces as scanner.Err() != nil. The
	// reader wraps it as `stream read: <err>` so logs / metrics can
	// distinguish stream errors from logic errors. Pin the wrap shape
	// so log queries that match on "stream read:" keep working.
	netErr := errors.New("connection reset by peer")
	body := &errorReader{prefix: "data: partial\n\n", err: netErr}
	r := NewSSEReader(body)

	// First call drains the partial data.
	_, _, _ = r.Next(context.Background())

	// Second call hits the reader error.
	_, _, err := r.Next(context.Background())
	if err == nil {
		t.Fatal("expected error from underlying read failure, got nil")
	}
	if !errors.Is(err, netErr) {
		t.Errorf("error chain doesn't include netErr: %v", err)
	}
	if !strings.Contains(err.Error(), "stream read:") {
		t.Errorf("error message = %q, want it to include 'stream read:' wrap prefix", err.Error())
	}
}

func TestSSEReader_EmptyBody(t *testing.T) {
	// Zero-length response body — return (done=true) cleanly.
	body := strings.NewReader("")
	r := NewSSEReader(body)
	_, done, err := r.Next(context.Background())
	if err != nil {
		t.Errorf("empty body errored: %v", err)
	}
	if !done {
		t.Errorf("empty body done=%v, want true", done)
	}
}

// _ ensures io is used in the test file (some tools insist on it).
var _ = io.EOF
