package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"connectrpc.com/connect"

	"github.com/gallowaysoftware/toqui-backend/internal/requestid"
)

// internalError logs the real error with context and returns a generic error
// message to the client. This prevents leaking internal implementation details
// (database errors, stack traces, etc.) while preserving debuggability via logs.
func internalError(ctx context.Context, operation string, err error) *connect.Error {
	reqID := requestid.FromContext(ctx)
	slog.Error(operation, "error", err, "request_id", reqID)
	return connect.NewError(connect.CodeInternal, fmt.Errorf("an internal error occurred"))
}

// maskEmail obscures the local part of an email address for safe logging.
// "john.doe@example.com" → "j***@example.com"
// Returns the input unchanged if it doesn't contain "@".
func maskEmail(email string) string {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return email
	}
	local := parts[0]
	if len(local) == 0 {
		return email
	}
	return string(local[0]) + "***@" + parts[1]
}
