package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"connectrpc.com/connect"

	"github.com/gallowaysoftware/toqui-backend/internal/audit"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
)

// errEmailUnderAgeBlocked is returned by checkUnderAgeBlock when the
// caller's email previously failed the 18+ age verification gate. The
// OAuth login handlers convert this to connect.CodePermissionDenied
// with a user-visible message; the sentinel is exported so tests can
// assert on it.
var errEmailUnderAgeBlocked = errors.New("account creation refused: previously failed age verification")

// checkUnderAgeBlock is the OAuth-side enforcement of the under-18
// refusal recorded by /auth/verify-age (see age_verify.go's
// handleUnderAge). Each OAuth login flow (Google, Facebook, Apple)
// must call this AFTER it has validated the upstream token (so we
// know the email is real) and BEFORE it upserts the user (so a
// refused person can't simply sign in again to re-create their row).
//
// Behaviour:
//   - email empty (Apple's silent-relay subsequent login) → no-op,
//     returns nil. Apple only ships email on first sign-in; for
//     subsequent logins we have the apple_sub which we cross-checked
//     at first sign-in time, so a returning Apple user has already
//     passed (or been refused) and either way isn't a re-creation
//     attempt.
//   - email present, hashed and found in under_age_blocks → returns
//     a connect error with code PermissionDenied and a message the
//     frontend can show verbatim. Audit-logs the refusal.
//   - email present, NOT in under_age_blocks → returns nil; the
//     caller proceeds with the normal upsert.
//   - DB error → returned wrapped. The caller treats this as an
//     internal error (CodeInternal) — we deliberately don't fail
//     OPEN on a query failure since that would silently disable the
//     anti-evasion check.
//
// Provider is the OAuth provider tag ("google", "facebook", "apple")
// for the audit log only; the under_age_blocks table records its own
// provider at refusal time (which may differ if the same user retries
// via a different OAuth identity).
func checkUnderAgeBlock(ctx context.Context, q *dbgen.Queries, email, provider string) error {
	// Trim before the empty check so a hypothetical whitespace-only
	// email from a misbehaving OAuth provider doesn't hash as the
	// constant SHA-256 of "" and collide with future block-list rows.
	// (W5 from the PR #420 adversarial review.)
	if strings.TrimSpace(email) == "" {
		return nil
	}

	hash := sha256OfEmail(email)
	blocked, err := q.IsEmailUnderAgeBlocked(ctx, hash)
	if err != nil {
		return fmt.Errorf("check under_age_blocks: %w", err)
	}
	if !blocked {
		return nil
	}

	audit.Log(audit.EventLoginDeniedUnderAge,
		"email", maskEmail(email),
		"oauth_provider", provider,
	)
	slog.Info("oauth login refused: email previously failed age verification",
		"oauth_provider", provider,
	)

	return connect.NewError(
		connect.CodePermissionDenied,
		errEmailUnderAgeBlocked,
	)
}
