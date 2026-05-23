//go:build integration

package integration

import (
	"context"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgtype"

	toquiv1 "github.com/gallowaysoftware/toqui-backend/gen/toqui/v1"
	"github.com/gallowaysoftware/toqui-backend/internal/auth"
	"github.com/gallowaysoftware/toqui-backend/internal/chatstore"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui-backend/internal/handlers"
	"github.com/gallowaysoftware/toqui-backend/internal/lifecycle"
)

// TestRefreshTokenConcurrentRotation pins the TOCTOU fix from
// toqui-backend#369 (P1 #2). Before the fix, two concurrent RefreshToken
// RPCs carrying the same JTI could both read revoked=false, both revoke
// the old token, and both issue new tokens — neither one trips reuse
// detection. The fix wraps lookup+revoke+insert in a transaction with
// SELECT ... FOR UPDATE on the refresh_tokens row, serialising them so
// the second request sees revoked=true and the family is revoked.
func TestRefreshTokenConcurrentRotation(t *testing.T) {
	env := NewTestEnv(t)
	env.CleanDB(t)
	ctx := context.Background()
	queries := dbgen.New(env.Pool)
	store := chatstore.New(env.Firestore)
	lifecycleSvc := lifecycle.NewService(env.Pool, store)

	// 32+ char JWT secret (config rejects shorter in non-local envs; keeps
	// this test aligned with prod behavior).
	authSvc := auth.NewService("test-client-id", "test-secret", "http://localhost/callback", "test-jwt-secret-concurrency-32chars!!")

	// Minimal allowed-domains and no auth-limiter: we're testing rotation,
	// not domain-allowlist or lockout.
	h := handlers.NewAuthHandler(authSvc, env.Pool, lifecycleSvc, nil, nil)

	user, err := queries.UpsertUserByGoogleID(ctx, dbgen.UpsertUserByGoogleIDParams{
		GoogleID: pgtype.Text{String: "refresh-concurrent-google", Valid: true},
		Email:    "refresh-concurrent@example.com",
		Name:     pgtype.Text{String: "Rotator", Valid: true},
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Issue an initial refresh token and persist it the way the real login
	// flow does — same DB row the handler will race on.
	initial, err := authSvc.GenerateRefreshToken(user.ID, [16]byte{})
	if err != nil {
		t.Fatalf("generate refresh token: %v", err)
	}
	if _, err := queries.CreateRefreshToken(ctx, dbgen.CreateRefreshTokenParams{
		UserID:    user.ID,
		Jti:       initial.JTI,
		Family:    initial.Family,
		ExpiresAt: initial.ExpiresAt,
	}); err != nil {
		t.Fatalf("store initial refresh token: %v", err)
	}

	// Fire two concurrent RefreshToken RPCs with the same token. Exactly
	// one should succeed; the other must get Unauthenticated.
	const n = 2
	var wg sync.WaitGroup
	wg.Add(n)
	results := make([]error, n)
	successTokens := make([]string, n)

	start := make(chan struct{})
	for i := range n {
		go func(i int) {
			defer wg.Done()
			<-start // align the two goroutines as closely as possible
			reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			resp, err := h.RefreshToken(reqCtx, connect.NewRequest(&toquiv1.RefreshTokenRequest{
				RefreshToken: initial.Token,
			}))
			results[i] = err
			if err == nil && resp != nil && resp.Msg != nil {
				successTokens[i] = resp.Msg.RefreshToken
			}
		}(i)
	}
	close(start)
	wg.Wait()

	// Exactly one success, exactly one failure. Either ordering is fine.
	successes := 0
	failures := 0
	for i, err := range results {
		if err == nil {
			successes++
			if successTokens[i] == "" {
				t.Errorf("goroutine %d: success with empty token", i)
			}
		} else {
			failures++
			// The failure path returns CodeUnauthenticated — either
			// "token not in DB" (loser's qtx read after winner committed)
			// or "revoked" (row observed revoked=true).
			if connect.CodeOf(err) != connect.CodeUnauthenticated {
				t.Errorf("goroutine %d: unexpected error code %v: %v", i, connect.CodeOf(err), err)
			}
		}
	}
	if successes != 1 || failures != 1 {
		t.Fatalf("want exactly 1 success + 1 failure, got %d success + %d failure (errs=%v)",
			successes, failures, results)
	}

	// After the race, the entire token family must be revoked — the loser
	// presented what it thought was a live token and got rejected, so
	// reuse detection should have fired and tripped a family revoke on
	// BOTH the original token and the winner's newly issued token.
	tokens, err := env.Pool.Query(ctx, `SELECT jti, revoked, family FROM refresh_tokens WHERE user_id = $1`, user.ID)
	if err != nil {
		t.Fatalf("list refresh tokens: %v", err)
	}
	defer tokens.Close()

	var totalRows, revokedRows int
	for tokens.Next() {
		var jti string
		var revoked bool
		var family [16]byte
		if err := tokens.Scan(&jti, &revoked, &family); err != nil {
			t.Fatalf("scan refresh token row: %v", err)
		}
		totalRows++
		if revoked {
			revokedRows++
		}
	}
	if err := tokens.Err(); err != nil {
		t.Fatalf("iterate refresh tokens: %v", err)
	}

	if totalRows == 0 {
		t.Fatal("no refresh_tokens rows for user after race — rotation dropped the record")
	}
	if revokedRows != totalRows {
		t.Fatalf("family revoke on reuse did not cover all rows: got %d/%d revoked", revokedRows, totalRows)
	}
}
