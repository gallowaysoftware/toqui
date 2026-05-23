//go:build integration

// Package integration — GDPR Article 17/20 end-to-end coverage for the
// lifecycle service.
//
// Scope notes for future readers:
//
// These tests cover the *currently wired* deletion + export fan-out:
//   - Postgres CASCADE across all user-keyed tables.
//   - Firestore chat purge under users/{uid}.
//   - The under_age_blocks anti-evasion preservation invariant.
//   - The export round-trip and multi-tenant isolation.
//
// Running:
//
//	make docker-up
//	make migrate-up
//	DATABASE_URL=postgres://... \
//	  FIRESTORE_EMULATOR_HOST=localhost:8080 \
//	  go test -tags=integration ./internal/integration/...
//
// TestEnv.CleanDB truncates every table touched here (see testhelper.go);
// it explicitly truncates `under_age_blocks` because lifecycle.DeleteUser
// must NOT cascade into that table — preserving rows across the test
// fixture would defeat the anti-evasion invariant we assert below.
package integration

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/gallowaysoftware/toqui/backend/internal/chatstore"
	"github.com/gallowaysoftware/toqui/backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui/backend/internal/lifecycle"
)

// e2eFixture stages a "fully populated user" — a row in every
// user-keyed Postgres table plus a Firestore chat session — so deletion
// fan-out has something to actually delete from each store. Returns the
// user, the trip, and the chat session for the caller to assert against.
type e2eFixture struct {
	user    dbgen.User
	trip    dbgen.Trip
	booking dbgen.Booking
	item    dbgen.ItineraryItem
	session *chatstore.ChatSession
	message *chatstore.ChatMessage
}

// stageUserFixture creates a user with a row in every user-keyed table
// the DeleteUser fan-out is supposed to cascade through. The label is
// embedded into the email + google_id so two fixtures in the same test
// (multi-tenant isolation) don't collide on UNIQUE constraints.
func stageUserFixture(t *testing.T, ctx context.Context, env *TestEnv, label string) e2eFixture {
	t.Helper()
	queries := dbgen.New(env.Pool)
	store := chatstore.New(env.Firestore)

	user, err := queries.UpsertUserByGoogleID(ctx, dbgen.UpsertUserByGoogleIDParams{
		GoogleID: pgtype.Text{String: "g_e2e_" + label, Valid: true},
		Email:    label + "@e2e.toqui-test.local",
		Name:     pgtype.Text{String: "E2E " + label, Valid: true},
	})
	if err != nil {
		t.Fatalf("[%s] upsert user: %v", label, err)
	}

	trip, err := queries.CreateTrip(ctx, dbgen.CreateTripParams{
		UserID:      user.ID,
		Title:       "Trip " + label,
		Description: pgtype.Text{String: "fixture trip for " + label, Valid: true},
	})
	if err != nil {
		t.Fatalf("[%s] create trip: %v", label, err)
	}

	item, err := queries.CreateItineraryItem(ctx, dbgen.CreateItineraryItemParams{
		TripID:     trip.ID,
		DayNumber:  pgtype.Int4{Int32: 1, Valid: true},
		OrderInDay: pgtype.Int4{Int32: 1, Valid: true},
		Type:       pgtype.Text{String: "activity", Valid: true},
		Title:      pgtype.Text{String: "Visit Forum (" + label + ")", Valid: true},
	})
	if err != nil {
		t.Fatalf("[%s] create itinerary item: %v", label, err)
	}

	booking, err := queries.CreateBooking(ctx, dbgen.CreateBookingParams{
		UserID:           user.ID,
		TripID:           pgtype.UUID{Bytes: trip.ID, Valid: true},
		Type:             "flight",
		Title:            "JFK→FCO " + label,
		ConfirmationCode: pgtype.Text{String: "CONF-" + strings.ToUpper(label), Valid: true},
		Source:           "manual",
	})
	if err != nil {
		t.Fatalf("[%s] create booking: %v", label, err)
	}

	// Theme link (themes table is migration-seeded; 'food' is always present).
	if err := queries.SetTripTheme(ctx, dbgen.SetTripThemeParams{
		TripID:     trip.ID,
		ThemeSlug:  "food",
		Confidence: 0.9,
		Source:     "ai",
	}); err != nil {
		t.Fatalf("[%s] set trip theme: %v", label, err)
	}

	if _, err := queries.CreateFeedback(ctx, dbgen.CreateFeedbackParams{
		UserID:  user.ID,
		Type:    "general",
		Message: "fixture feedback for " + label,
	}); err != nil {
		t.Fatalf("[%s] create feedback: %v", label, err)
	}

	// referrals.code is VARCHAR(16) UNIQUE — keep the prefix short and
	// truncate the label so a long fixture label (e.g. "exportthendelete")
	// doesn't silently overflow.
	refCode := "R-" + strings.ToUpper(label)
	if len(refCode) > 16 {
		refCode = refCode[:16]
	}
	if _, err := queries.CreateReferral(ctx, dbgen.CreateReferralParams{
		ReferrerID: user.ID,
		Code:       refCode,
	}); err != nil {
		t.Fatalf("[%s] create referral: %v", label, err)
	}

	if _, err := queries.RecordConsent(ctx, dbgen.RecordConsentParams{
		UserID:      user.ID,
		ConsentType: "terms",
		IpAddress:   pgtype.Text{String: "127.0.0.1", Valid: true},
		UserAgent:   pgtype.Text{String: "test/1.0", Valid: true},
	}); err != nil {
		t.Fatalf("[%s] record consent: %v", label, err)
	}

	if _, err := queries.UpsertPreference(ctx, dbgen.UpsertPreferenceParams{
		UserID: user.ID,
		Key:    "language",
		Value:  "en",
	}); err != nil {
		t.Fatalf("[%s] upsert preference: %v", label, err)
	}

	if _, err := queries.CreateRefreshToken(ctx, dbgen.CreateRefreshTokenParams{
		UserID:    user.ID,
		Jti:       "jti-" + label,
		Family:    uuid.New(),
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
	}); err != nil {
		t.Fatalf("[%s] create refresh token: %v", label, err)
	}

	if _, err := queries.IncrementDailyUsage(ctx, dbgen.IncrementDailyUsageParams{
		UserID:   user.ID,
		MaxCount: 100,
	}); err != nil {
		t.Fatalf("[%s] increment daily usage: %v", label, err)
	}

	// Firestore chat fixture under users/{uid}/trips/{tripId}/...
	session, err := store.CreateSession(ctx, user.ID.String(), trip.ID.String(), "planning")
	if err != nil {
		t.Fatalf("[%s] create chat session: %v", label, err)
	}
	msg := &chatstore.ChatMessage{
		Role:    "user",
		Content: "Plan my " + label + " trip",
	}
	if err := store.AddMessage(ctx, user.ID.String(), trip.ID.String(), session.ID, msg); err != nil {
		t.Fatalf("[%s] add chat message: %v", label, err)
	}

	return e2eFixture{
		user:    user,
		trip:    trip,
		booking: booking,
		item:    item,
		session: session,
		message: msg,
	}
}

// countRows returns the count of rows in the given table where user_id = $1.
// Used to assert that DeleteUser cascaded a clean sweep of the user-keyed
// tables. Trips/itinerary_items/bookings are kept around even though the
// generated query already returns []dbgen.Trip etc., because raw SQL
// cleanly answers the precise question the test is asking ("zero rows
// remain") without rerunning the full ORDER BY / LIMIT / pagination query
// path that ListTripsByUser et al. carry.
func countRows(t *testing.T, ctx context.Context, env *TestEnv, table, userColumn string, userID uuid.UUID) int {
	t.Helper()
	var count int
	q := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s = $1", table, userColumn)
	if err := env.Pool.QueryRow(ctx, q, userID).Scan(&count); err != nil {
		t.Fatalf("count %s.%s for user %s: %v", table, userColumn, userID, err)
	}
	return count
}

// hashEmailE2E mirrors handlers.sha256OfEmail. Inlined to keep this file
// from importing internal/handlers (and dragging in connect, audit, etc.).
// See under_age_blocks_test.go for the same inlining rationale.
func hashEmailE2E(email string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(email))))
	return hex.EncodeToString(sum[:])
}

// TestE2E_DeleteUser_FullPostgresFanout verifies that DeleteUser leaves
// no rows behind in any user-keyed Postgres table, and clears the
// Firestore subtree under users/{uid}/trips/{tripId}.
//
// This is the GDPR Article 17 ground-truth check — every table that
// inherits the user's lifetime should be empty after deletion.
func TestE2E_DeleteUser_FullPostgresFanout(t *testing.T) {
	env := NewTestEnv(t)
	env.CleanDB(t)
	ctx := context.Background()
	store := chatstore.New(env.Firestore)
	lifecycleSvc := lifecycle.NewService(env.Pool, store)

	fix := stageUserFixture(t, ctx, env, "fanout")

	if err := lifecycleSvc.DeleteUser(ctx, fix.user.ID); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}

	// Every user-keyed table must report zero rows for this user.
	tables := []struct {
		name   string
		column string
	}{
		{"users", "id"},
		{"trips", "user_id"},
		{"bookings", "user_id"},
		{"feedback", "user_id"},
		{"referrals", "referrer_id"},
		{"user_consents", "user_id"},
		{"user_preferences", "user_id"},
		{"refresh_tokens", "user_id"},
		{"daily_usage", "user_id"},
	}
	for _, tbl := range tables {
		if got := countRows(t, ctx, env, tbl.name, tbl.column, fix.user.ID); got != 0 {
			t.Errorf("expected 0 rows in %s for deleted user, got %d", tbl.name, got)
		}
	}

	// itinerary_items and trip_themes are keyed by trip_id (not user_id),
	// so they must be empty for the deleted user's trip.
	var itemCount int
	if err := env.Pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM itinerary_items WHERE trip_id = $1", fix.trip.ID,
	).Scan(&itemCount); err != nil {
		t.Fatalf("count itinerary_items: %v", err)
	}
	if itemCount != 0 {
		t.Errorf("expected 0 itinerary_items for deleted trip, got %d", itemCount)
	}
	var themeCount int
	if err := env.Pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM trip_themes WHERE trip_id = $1", fix.trip.ID,
	).Scan(&themeCount); err != nil {
		t.Fatalf("count trip_themes: %v", err)
	}
	if themeCount != 0 {
		t.Errorf("expected 0 trip_themes for deleted trip, got %d", themeCount)
	}

	// Firestore subtree: ListSessions on the deleted user's trip path
	// must return empty. (DeleteAllForTrip drops sessions and messages
	// under users/{uid}/trips/{tripId}.)
	sessions, err := store.ListSessions(ctx, fix.user.ID.String(), fix.trip.ID.String(), 100)
	if err != nil {
		t.Fatalf("list sessions after delete: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 chat sessions for deleted user, got %d", len(sessions))
	}
}

// TestE2E_DeleteUser_PreservesUnderAgeBlocks pins the most subtle
// correctness invariant in the file: the under_age_blocks table has NO
// foreign key to users (deliberate — it's anti-evasion data keyed on the
// SHA-256 of an email so refused users can't re-sign-up). DeleteUser
// must NOT touch it.
//
// If this test fails, that's a P1 GDPR/compliance bug — the refusal
// audit trail would silently disappear when a user submitted a deletion
// request, defeating the point of recording it in the first place.
func TestE2E_DeleteUser_PreservesUnderAgeBlocks(t *testing.T) {
	env := NewTestEnv(t)
	env.CleanDB(t)
	ctx := context.Background()
	queries := dbgen.New(env.Pool)
	store := chatstore.New(env.Firestore)
	lifecycleSvc := lifecycle.NewService(env.Pool, store)

	const blockedEmail = "preserved@e2e.toqui-test.local"
	emailHash := hashEmailE2E(blockedEmail)

	// Stage: under_age_blocks row exists (simulating a refused sign-up
	// from earlier — the email_sha256 is a stable identifier even after
	// the underlying user row is gone).
	if _, err := queries.RecordUnderAgeBlock(ctx, dbgen.RecordUnderAgeBlockParams{
		EmailSha256:   emailHash,
		OauthProvider: "google",
	}); err != nil {
		t.Fatalf("seed under_age_blocks: %v", err)
	}

	// Stage: a totally different user (different email) goes through the
	// normal create + delete flow. The block row above MUST be untouched.
	fix := stageUserFixture(t, ctx, env, "preserve")

	if err := lifecycleSvc.DeleteUser(ctx, fix.user.ID); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}

	// THE invariant.
	blocked, err := queries.IsEmailUnderAgeBlocked(ctx, emailHash)
	if err != nil {
		t.Fatalf("IsEmailUnderAgeBlocked: %v", err)
	}
	if !blocked {
		t.Fatal("under_age_blocks row was wiped by DeleteUser — this is a P1 compliance bug. " +
			"The table has no FK to users by design (it's anti-evasion data keyed by SHA-256 " +
			"of email, retained beyond the user's lifetime). If the deletion fan-out grew a " +
			"manual DELETE FROM under_age_blocks step, revert it.")
	}
}

// TestE2E_DeleteUser_Idempotent verifies that re-calling DeleteUser on an
// already-deleted user returns nil (no error). This is the safety net for
// the retry loop in RetryFailedDeletions and for the user-initiated retry
// path: a partial failure on a previous attempt followed by a re-issue
// shouldn't blow up the second call.
func TestE2E_DeleteUser_Idempotent(t *testing.T) {
	env := NewTestEnv(t)
	env.CleanDB(t)
	ctx := context.Background()
	store := chatstore.New(env.Firestore)
	lifecycleSvc := lifecycle.NewService(env.Pool, store)

	fix := stageUserFixture(t, ctx, env, "idempotent")

	if err := lifecycleSvc.DeleteUser(ctx, fix.user.ID); err != nil {
		t.Fatalf("first DeleteUser: %v", err)
	}

	if err := lifecycleSvc.DeleteUser(ctx, fix.user.ID); err != nil {
		t.Errorf("second (idempotent) DeleteUser must succeed, got: %v", err)
	}

	// Calling once more for good measure — the retry loop can fire it
	// repeatedly under stale-deletion-request handling.
	if err := lifecycleSvc.DeleteUser(ctx, fix.user.ID); err != nil {
		t.Errorf("third DeleteUser must succeed, got: %v", err)
	}
}

// TestE2E_ExportUserData_RoundTrip verifies the GDPR Article 20 export
// path: the export contains every category of user data, JSON-serialises
// losslessly, and round-trips back through json.Unmarshal.
func TestE2E_ExportUserData_RoundTrip(t *testing.T) {
	env := NewTestEnv(t)
	env.CleanDB(t)
	ctx := context.Background()
	store := chatstore.New(env.Firestore)
	lifecycleSvc := lifecycle.NewService(env.Pool, store)

	fix := stageUserFixture(t, ctx, env, "roundtrip")

	export, err := lifecycleSvc.ExportUserData(ctx, fix.user.ID)
	if err != nil {
		t.Fatalf("ExportUserData: %v", err)
	}
	if export == nil {
		t.Fatal("ExportUserData returned nil export with nil error")
	}

	if export.ExportedAt == "" {
		t.Error("ExportedAt timestamp must be set")
	}
	if len(export.Trips) != 1 {
		t.Errorf("expected 1 trip in export, got %d", len(export.Trips))
	}
	if len(export.Bookings) != 1 {
		t.Errorf("expected 1 booking in export, got %d", len(export.Bookings))
	}
	if len(export.Referrals) != 1 {
		t.Errorf("expected 1 referral in export, got %d", len(export.Referrals))
	}
	// Feedback the user submitted IS their own personal data and per
	// GDPR Art. 20 must appear in their export. Wired in #438 — the
	// fixture writes one feedback row, the export must include it.
	if len(export.Feedback) != 1 {
		t.Errorf("expected 1 feedback row in export.Feedback (Art. 20 includes user-submitted feedback), got %d", len(export.Feedback))
	}
	if len(export.Consents) != 1 {
		t.Errorf("expected 1 consent in export, got %d", len(export.Consents))
	}
	if len(export.Preferences) != 1 {
		t.Errorf("expected 1 preference in export, got %d", len(export.Preferences))
	}

	// Trip details: the itinerary item we staged must show up.
	if got := len(export.Trips[0].Itinerary); got != 1 {
		t.Errorf("expected 1 itinerary item on trip[0], got %d", got)
	}
	if got := len(export.Trips[0].Themes); got != 1 {
		t.Errorf("expected 1 theme on trip[0], got %d", got)
	}

	// Chat data: the staged Firestore session must appear keyed by
	// trip ID.
	chatForTrip, ok := export.ChatData[fix.trip.ID.String()]
	if !ok {
		t.Errorf("expected chat data for trip %s in export.ChatData, got keys %v",
			fix.trip.ID.String(), keysOf(export.ChatData))
	} else if chatForTrip == nil {
		t.Error("chat data for trip is nil")
	}

	// JSON round-trip: the export must marshal cleanly and parse back to
	// a generic map. The chatstore types now carry json: tags alongside
	// their firestore: tags (#438), so the chat slice serialises in the
	// same snake_case shape as the rest of the export.
	raw, err := json.Marshal(export)
	if err != nil {
		t.Fatalf("json.Marshal export: %v", err)
	}
	if len(raw) < 100 {
		t.Errorf("marshalled export is suspiciously small (%d bytes)", len(raw))
	}
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("json.Unmarshal export: %v", err)
	}
	for _, key := range []string{"exported_at", "user", "trips", "bookings", "referrals", "feedback", "consents", "preferences", "chat_data"} {
		if _, ok := parsed[key]; !ok {
			t.Errorf("parsed export missing top-level key %q (have: %v)", key, keysOf(parsed))
		}
	}

	// Wire-shape regression check: chat data must serialise with
	// snake_case keys (json: tags on chatstore.ChatSession /
	// chatstore.ChatMessage), not Go's default PascalCase. Before #438
	// these types only had firestore: tags so the export contained
	// `"ID"`, `"TripID"`, `"Content"` — inconsistent with the rest of
	// the export. Now they should match.
	rawStr := string(raw)
	if strings.Contains(rawStr, `"TripID"`) || strings.Contains(rawStr, `"SessionID"`) {
		t.Errorf("export contains PascalCase chat field names — chatstore.ChatMessage / ChatSession json: tags missing. Sample: %.200s", rawStr)
	}
}

// TestE2E_ExportUserData_MultiTenantIsolation is the strongest privacy
// guarantee in this file: an export for user A must not contain user B's
// data, anywhere. We rely on the user IDs being unique random UUIDs and
// scan the whole serialised export for B's id — if it appears, the
// tenant boundary has been breached.
func TestE2E_ExportUserData_MultiTenantIsolation(t *testing.T) {
	env := NewTestEnv(t)
	env.CleanDB(t)
	ctx := context.Background()
	store := chatstore.New(env.Firestore)
	lifecycleSvc := lifecycle.NewService(env.Pool, store)

	a := stageUserFixture(t, ctx, env, "tenantA")
	b := stageUserFixture(t, ctx, env, "tenantB")

	exportA, err := lifecycleSvc.ExportUserData(ctx, a.user.ID)
	if err != nil {
		t.Fatalf("ExportUserData(A): %v", err)
	}

	raw, err := json.Marshal(exportA)
	if err != nil {
		t.Fatalf("marshal exportA: %v", err)
	}
	rawStr := string(raw)

	// User B identifiers must not appear in user A's export, in any
	// form. We check every primary identifier we can reach for from B's
	// fixture: user id, trip id, booking id, itinerary item id, chat
	// session id, email, and the referral code (which embeds the label).
	type leak struct {
		field, value string
	}
	leaks := []leak{
		{"B user_id", b.user.ID.String()},
		{"B trip_id", b.trip.ID.String()},
		{"B booking_id", b.booking.ID.String()},
		{"B item_id", b.item.ID.String()},
		{"B chat session_id", b.session.ID},
		{"B email", b.user.Email},
		{"B referral code", "R-TENANTB"},
	}
	for _, l := range leaks {
		if strings.Contains(rawStr, l.value) {
			t.Errorf("tenant isolation breach: %s value %q appears in tenant A's export", l.field, l.value)
		}
	}

	// Sanity: A's own identifiers are present (otherwise the contains
	// check above is vacuously true).
	if !strings.Contains(rawStr, a.user.ID.String()) {
		t.Error("sanity check failed: tenant A's own user_id is missing from its export")
	}
}

// TestE2E_ExportThenDelete is the realistic compliance flow: a user
// requests their data export, then deletes their account. Both must
// succeed, and the export's contents must remain intact in memory after
// the delete (no nil pointers from fields the deletion would have
// invalidated — important because the export struct embeds dbgen rows
// by value, so this is mostly belt-and-braces, but a future refactor
// could grow lazy fetches).
func TestE2E_ExportThenDelete(t *testing.T) {
	env := NewTestEnv(t)
	env.CleanDB(t)
	ctx := context.Background()
	store := chatstore.New(env.Firestore)
	lifecycleSvc := lifecycle.NewService(env.Pool, store)

	fix := stageUserFixture(t, ctx, env, "expdel")

	export, err := lifecycleSvc.ExportUserData(ctx, fix.user.ID)
	if err != nil {
		t.Fatalf("ExportUserData (pre-delete): %v", err)
	}
	preDeleteJSON, err := json.Marshal(export)
	if err != nil {
		t.Fatalf("marshal pre-delete export: %v", err)
	}

	if err := lifecycleSvc.DeleteUser(ctx, fix.user.ID); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}

	// User is gone in the DB now.
	queries := dbgen.New(env.Pool)
	if _, err := queries.GetUserByID(ctx, fix.user.ID); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("expected pgx.ErrNoRows after DeleteUser, got %v", err)
	}

	// Export struct stays intact in memory and re-marshals to the same
	// bytes — proving the deletion didn't invalidate any reference held
	// by the export.
	postDeleteJSON, err := json.Marshal(export)
	if err != nil {
		t.Fatalf("marshal post-delete export: %v", err)
	}
	if !bytes.Equal(preDeleteJSON, postDeleteJSON) {
		t.Error("in-memory export changed after DeleteUser — the export struct must not hold lazy references to deleted DB rows")
	}
}

// keysOf is a tiny helper for clearer assertion failure messages when a
// map key is missing.
func keysOf[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
