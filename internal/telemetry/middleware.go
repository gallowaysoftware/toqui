package telemetry

import (
	"net/http"
	"strconv"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// statusWriter wraps http.ResponseWriter to capture the status code.
type statusWriter struct {
	http.ResponseWriter
	status int
	wrote  bool
}

// Compile-time guard: statusWriter MUST implement http.Flusher. This is
// load-bearing — ConnectRPC's server-streaming transport type-asserts the
// response writer to http.Flusher before sending any frames; if this
// assertion stops holding, every streaming RPC (e.g. ChatService/SendMessage)
// dies immediately with
// "*telemetry.statusWriter does not implement http.Flusher". This line
// makes that regression a compile error rather than a runtime one, catching
// it the moment anyone removes the Flush() method below.
var _ http.Flusher = (*statusWriter)(nil)

func (w *statusWriter) WriteHeader(code int) {
	if !w.wrote {
		w.status = code
		w.wrote = true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if !w.wrote {
		w.status = http.StatusOK
		w.wrote = true
	}
	return w.ResponseWriter.Write(b)
}

// Unwrap supports http.ResponseController and other standard unwrap patterns.
func (w *statusWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// Flush implements http.Flusher by delegating to the underlying
// ResponseWriter when it supports flushing. ConnectRPC's server-streaming
// transport (used by ChatService/SendMessage) type-asserts the response
// writer to http.Flusher before sending any frames; without this
// passthrough every streaming RPC fails immediately with
// "*telemetry.statusWriter does not implement http.Flusher" and the
// stream is killed before the first event reaches the client. Observed
// in agentic test runs 21 and 22 as a recurring P0. The no-op fallback
// (when the underlying writer is not itself a Flusher) preserves the
// previous non-streaming behaviour for unary RPCs and plain HTTP.
func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Middleware returns an http.Handler that records request duration and count
// metrics for every HTTP request. It should wrap the outermost layer of the
// middleware chain (or just inside otelhttp) so it captures all requests.
func Middleware(m *Metrics, next http.Handler) http.Handler {
	if m == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(sw, r)

		duration := time.Since(start).Seconds()
		attrs := metric.WithAttributes(
			attribute.String("http.request.method", r.Method),
			attribute.String("http.route", r.URL.Path),
			attribute.String("http.response.status_code", strconv.Itoa(sw.status)),
		)
		m.RequestDuration.Record(r.Context(), duration, attrs)
		m.RequestCount.Add(r.Context(), 1, attrs)
	})
}
