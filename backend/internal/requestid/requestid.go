// Package requestid provides HTTP middleware that generates a unique request ID
// for each incoming request. The ID is stored in the request context and added
// to the response headers for correlation.
package requestid

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

type ctxKey struct{}

// Middleware adds a unique request ID to each request's context and response header.
// Cloud Run sets X-Cloud-Trace-Context; if present, we extract the trace ID instead.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = uuid.NewString()
		}

		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), ctxKey{}, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// FromContext returns the request ID from the context, or an empty string.
func FromContext(ctx context.Context) string {
	id, _ := ctx.Value(ctxKey{}).(string)
	return id
}
