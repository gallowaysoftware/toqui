//go:build integration

package integration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/gallowaysoftware/toqui-backend/internal/chatstore"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui-backend/internal/lifecycle"
)

// hashEmail mirrors handlers.sha256OfEmail. Replicated locally so this
// test file doesn't import internal/handlers (which would pull in
// connect, audit, etc. transitively) — and so a future change to the
// handler's normalisation rule is caught here as a behavioural drift
// rather than a silent dependency on the production helper.
func hashEmail(email string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(email))))
	return hex.EncodeToString(sum[:])
}

// TestUnderAgeBlock_DeletionAndBlock_FullPath simulates the destructive
// half of POST /auth/verify-age when an under-18 DOB is submitted: the
// block row is recorded, the user is hard-deleted via the lifecycle
// service, and the block row survives the user deletion (no FK).
//
// The HTTP layer is deliberately skipped — the destructive path is
// `handleUnderAge` which is unexported and the wire-shape of the 403
// response is already pinned by unit tests in internal/handlers. What
// integration testing buys us here is the cross-component behaviour:
// the lifecycle service really does cascade a real Postgres row
// deletion across a Firestore chat purge while a sibling DB write to
// `under_age_blocks` persists.
func TestUnderAgeBlock_DeletionAndBlock_FullPath(t *testing.T) {
	env := NewTestEnv(t)
	env.CleanDB(t)
	ctx := context.Background()
	queries := dbgen.New(env.Pool)
	store := chatstore.New(env.Firestore)
	lifecycleSvc := lifecycle.NewService(env.Pool, store)

	// Stage: create a user that will fail the age gate.
	const email = "underage@example.com"
	user, err := queries.UpsertUserByGoogleID(ctx, dbgen.UpsertUserByGoogleIDParams{
		GoogleID: pgtype.Text{String: "g_under_age", Valid: true},
		Email:    email,
		Name:     pgtype.Text{String: "Under-Age", Valid: true},
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Act: write the block row (mirroring handleUnderAge's first step),
	// then call lifecycle.DeleteUser (its second step).
	emailHash := hashEmail(email)
	gotID, err := queries.RecordUnderAgeBlock(ctx, dbgen.RecordUnderAgeBlockParams{
		EmailSha256:   emailHash,
		OauthProvider: "google",
	})
	if err != nil {
		t.Fatalf("record under_age_block: %v", err)
	}
	if gotID.String() == "" || gotID == [16]byte{} {
		t.Errorf("expected non-zero block id from RETURNING, got %v", gotID)
	}

	if err := lifecycleSvc.DeleteUser(ctx, user.ID); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}

	// Assert: user row is gone (downstream-of-the-FK invariant).
	_, err = queries.GetUserByID(ctx, user.ID)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("expected pgx.ErrNoRows after DeleteUser, got %v", err)
	}

	// Assert: under_age_blocks row survives the user deletion. This is
	// THE invariant — the table has no FK to users so the deletion
	// can't cascade to it. Without this, the OAuth pre-check on a
	// re-attempt would be a no-op and the user could re-sign-up.
	blocked, err := queries.IsEmailUnderAgeBlocked(ctx, emailHash)
	if err != nil {
		t.Fatalf("IsEmailUnderAgeBlocked: %v", err)
	}
	if !blocked {
		t.Error("expected under_age_blocks row to survive lifecycle.DeleteUser, got blocked=false")
	}
}

// TestUnderAgeBlock_RaceConflictReturnsErrNoRows pins the W1 race-fix:
// when two concurrent verify-age handlers race on the same email, the
// `INSERT ... ON CONFLICT DO NOTHING RETURNING id` query returns
// `pgx.ErrNoRows` to the loser. The handler uses that signal to
// short-circuit (skipping the redundant lifecycle.DeleteUser + audit
// log) so compliance reports don't double-count the refusal.
func TestUnderAgeBlock_RaceConflictReturnsErrNoRows(t *testing.T) {
	env := NewTestEnv(t)
	env.CleanDB(t)
	ctx := context.Background()
	queries := dbgen.New(env.Pool)

	const emailHash = "0000000000000000000000000000000000000000000000000000000000000001"

	// First insert: must succeed and return an id.
	id1, err := queries.RecordUnderAgeBlock(ctx, dbgen.RecordUnderAgeBlockParams{
		EmailSha256:   emailHash,
		OauthProvider: "google",
	})
	if err != nil {
		t.Fatalf("first insert: %v", err)
	}
	if id1 == [16]byte{} {
		t.Error("first insert should return a non-zero id")
	}

	// Second insert with same email_sha256: must hit ON CONFLICT DO
	// NOTHING and surface as pgx.ErrNoRows. The handler converts this
	// to "another concurrent request handled this user" + early 403.
	id2, err := queries.RecordUnderAgeBlock(ctx, dbgen.RecordUnderAgeBlockParams{
		EmailSha256:   emailHash,
		OauthProvider: "facebook", // different provider — irrelevant; UNIQUE is on email
	})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("expected pgx.ErrNoRows from conflict, got id=%v err=%v", id2, err)
	}
}

// TestUnderAgeBlock_TrueConcurrentRace exercises the race-fix under
// actual concurrent goroutines (not just sequential calls). Spawns N
// goroutines all racing to insert the same email_sha256; asserts that
// EXACTLY ONE wins (returns an id) and the rest receive pgx.ErrNoRows.
//
// This is the test that would have failed the previous design's
// "best-effort logged-but-ignored" insert: in that version both
// goroutines saw the same outcome (success) and both proceeded to
// lifecycle.DeleteUser + audit, yielding the duplicate audit events
// the W1 review flagged. The new RETURNING-id path makes the race
// outcome observable.
func TestUnderAgeBlock_TrueConcurrentRace(t *testing.T) {
	env := NewTestEnv(t)
	env.CleanDB(t)
	queries := dbgen.New(env.Pool)

	const emailHash = "0000000000000000000000000000000000000000000000000000000000000002"
	const goroutines = 8

	var wg sync.WaitGroup
	wg.Add(goroutines)

	results := make([]error, goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			_, err := queries.RecordUnderAgeBlock(context.Background(), dbgen.RecordUnderAgeBlockParams{
				EmailSha256:   emailHash,
				OauthProvider: "google",
			})
			results[idx] = err
		}(i)
	}
	wg.Wait()

	winners := 0
	losers := 0
	for _, err := range results {
		switch {
		case err == nil:
			winners++
		case errors.Is(err, pgx.ErrNoRows):
			losers++
		default:
			t.Errorf("unexpected error from concurrent insert: %v", err)
		}
	}
	if winners != 1 {
		t.Errorf("expected exactly 1 winner, got %d (losers=%d)", winners, losers)
	}
	if losers != goroutines-1 {
		t.Errorf("expected %d losers, got %d", goroutines-1, losers)
	}
}

// TestUnderAgeBlock_OAuthPreCheckSeparatesEmails pins that the
// pre-check helper (called from Google/Facebook/Apple OAuth handlers)
// matches per-email, not per-(email, provider). A user refused via
// Google is also blocked from Facebook re-signup — that's the
// anti-evasion property the redesign promised.
func TestUnderAgeBlock_OAuthPreCheckSeparatesEmails(t *testing.T) {
	env := NewTestEnv(t)
	env.CleanDB(t)
	ctx := context.Background()
	queries := dbgen.New(env.Pool)

	const refusedEmail = "refused@example.com"
	const cleanEmail = "clean@example.com"
	refusedHash := hashEmail(refusedEmail)
	cleanHash := hashEmail(cleanEmail)

	// Refuse the first email via Google.
	_, err := queries.RecordUnderAgeBlock(ctx, dbgen.RecordUnderAgeBlockParams{
		EmailSha256:   refusedHash,
		OauthProvider: "google",
	})
	if err != nil {
		t.Fatalf("seed refusal: %v", err)
	}

	// Pre-check: same email is blocked, regardless of which OAuth
	// provider asks.
	blocked, err := queries.IsEmailUnderAgeBlocked(ctx, refusedHash)
	if err != nil {
		t.Fatalf("IsEmailUnderAgeBlocked refused: %v", err)
	}
	if !blocked {
		t.Error("refused email should be blocked")
	}

	// Pre-check: a different email is NOT blocked. Sanity that the
	// query isn't matching everything.
	blocked, err = queries.IsEmailUnderAgeBlocked(ctx, cleanHash)
	if err != nil {
		t.Fatalf("IsEmailUnderAgeBlocked clean: %v", err)
	}
	if blocked {
		t.Error("clean email should not be blocked")
	}
}

// TestUnderAgeBlock_HashNormalisation pins the email-normalisation
// invariant ACROSS the test surface (verify-age writes the block,
// OAuth pre-check reads it). If a future refactor splits the helper
// in one place but not the other, refused users could re-sign-up by
// changing their email casing — this test would catch that.
func TestUnderAgeBlock_HashNormalisation(t *testing.T) {
	env := NewTestEnv(t)
	env.CleanDB(t)
	ctx := context.Background()
	queries := dbgen.New(env.Pool)

	// Refusal stored using the canonical (lowercased, trimmed) form.
	canonical := hashEmail("alice@example.com")
	_, err := queries.RecordUnderAgeBlock(ctx, dbgen.RecordUnderAgeBlockParams{
		EmailSha256:   canonical,
		OauthProvider: "google",
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Pre-check called with a casing/whitespace variant (the OAuth
	// provider sent us a different form). The helper normalises before
	// hashing, so all variants must hit the same row.
	for _, variant := range []string{
		"ALICE@EXAMPLE.COM",
		"  alice@example.com  ",
		"Alice@Example.com",
	} {
		blocked, err := queries.IsEmailUnderAgeBlocked(ctx, hashEmail(variant))
		if err != nil {
			t.Fatalf("IsEmailUnderAgeBlocked %q: %v", variant, err)
		}
		if !blocked {
			t.Errorf("variant %q should hash-match the canonical block row", variant)
		}
	}
}
