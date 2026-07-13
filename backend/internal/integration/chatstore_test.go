//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/gallowaysoftware/toqui/backend/internal/chatstore"
	"github.com/gallowaysoftware/toqui/backend/internal/dbgen"
)

// newChatFixtureUser creates a users row (chat_sessions has a FK to users)
// and returns its ID as a string, the form the chatstore API takes.
func newChatFixtureUser(t *testing.T, ctx context.Context, env *TestEnv, label string) string {
	t.Helper()
	queries := dbgen.New(env.Pool)
	user, err := queries.UpsertUserByGoogleID(ctx, dbgen.UpsertUserByGoogleIDParams{
		GoogleID: pgtype.Text{String: "g_chat_" + label + "_" + uuid.NewString(), Valid: true},
		Email:    label + "-" + uuid.NewString() + "@chat.toqui-test.local",
		Name:     pgtype.Text{String: "Chat " + label, Valid: true},
	})
	if err != nil {
		t.Fatalf("[%s] create fixture user: %v", label, err)
	}
	return user.ID.String()
}

func TestChatStoreSessionAndMessages(t *testing.T) {
	env := NewTestEnv(t)
	env.CleanDB(t)
	ctx := context.Background()
	store := chatstore.New(env.Pool)

	userID := newChatFixtureUser(t, ctx, env, "basic")
	tripID := uuid.NewString()

	// Create session
	session, err := store.CreateSession(ctx, userID, tripID, "planning")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if session.Mode != "planning" {
		t.Errorf("mode = %q, want %q", session.Mode, "planning")
	}
	if session.CreatedAt.IsZero() {
		t.Error("CreatedAt must be set on a freshly created session")
	}

	// Get session
	got, err := store.GetSession(ctx, userID, tripID, session.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if got.ID != session.ID {
		t.Errorf("got ID = %q, want %q", got.ID, session.ID)
	}
	if got.TripID != tripID {
		t.Errorf("got TripID = %q, want %q", got.TripID, tripID)
	}

	// Add messages
	userMsg := &chatstore.ChatMessage{Role: "user", Content: "Hello, where should I go in Tokyo?"}
	if err := store.AddMessage(ctx, userID, tripID, session.ID, userMsg); err != nil {
		t.Fatalf("add user message: %v", err)
	}
	if userMsg.ID == "" || userMsg.SessionID != session.ID {
		t.Errorf("AddMessage must populate msg.ID (%q) and msg.SessionID (%q)", userMsg.ID, userMsg.SessionID)
	}

	assistantMsg := &chatstore.ChatMessage{Role: "assistant", Content: "I'd recommend Shinjuku and Shibuya!"}
	if err := store.AddMessage(ctx, userID, tripID, session.ID, assistantMsg); err != nil {
		t.Fatalf("add assistant message: %v", err)
	}

	// Get messages — chronological order
	messages, err := store.GetMessages(ctx, userID, tripID, session.ID, 100)
	if err != nil {
		t.Fatalf("get messages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("got %d messages, want 2", len(messages))
	}
	if messages[0].Role != "user" {
		t.Errorf("first message role = %q, want %q", messages[0].Role, "user")
	}
	if messages[1].Role != "assistant" {
		t.Errorf("second message role = %q, want %q", messages[1].Role, "assistant")
	}

	// messageCount counts content-bearing user/assistant messages
	got, err = store.GetSession(ctx, userID, tripID, session.ID)
	if err != nil {
		t.Fatalf("get session after messages: %v", err)
	}
	if got.MessageCount != 2 {
		t.Errorf("MessageCount = %d, want 2", got.MessageCount)
	}

	// List sessions
	sessions, err := store.ListSessions(ctx, userID, tripID, 10)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Errorf("got %d sessions, want 1", len(sessions))
	}

	// Delete all for trip
	if err := store.DeleteAllForTrip(ctx, userID, tripID); err != nil {
		t.Fatalf("delete all: %v", err)
	}

	sessions, err = store.ListSessions(ctx, userID, tripID, 10)
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("got %d sessions after delete, want 0", len(sessions))
	}

	// Messages must have cascaded with their sessions
	var orphaned int
	if err := env.Pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM chat_messages WHERE session_id = $1", session.ID,
	).Scan(&orphaned); err != nil {
		t.Fatalf("count orphaned messages: %v", err)
	}
	if orphaned != 0 {
		t.Errorf("got %d orphaned messages after DeleteAllForTrip, want 0", orphaned)
	}
}

// TestChatStoreToolLoopRoundTrip verifies that tool calls and tool results
// survive the JSONB round-trip — this data is what lets the AI reconstruct
// tool context when continuing a conversation.
func TestChatStoreToolLoopRoundTrip(t *testing.T) {
	env := NewTestEnv(t)
	env.CleanDB(t)
	ctx := context.Background()
	store := chatstore.New(env.Pool)

	userID := newChatFixtureUser(t, ctx, env, "toolloop")
	tripID := uuid.NewString()

	session, err := store.CreateSession(ctx, userID, tripID, "planning")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	toolCallMsg := &chatstore.ChatMessage{
		Role: "assistant",
		ToolCalls: []chatstore.StoredToolCall{{
			ID:               "call_1",
			Name:             "create_itinerary_items",
			Arguments:        `{"items":[{"title":"Sushi at Tsukiji","day":2}]}`,
			ThoughtSignature: "opaque-gemini-token",
		}},
	}
	if err := store.AddMessage(ctx, userID, tripID, session.ID, toolCallMsg); err != nil {
		t.Fatalf("add tool call message: %v", err)
	}

	toolResultMsg := &chatstore.ChatMessage{
		Role:     "user",
		Metadata: map[string]string{"synthetic": "true"},
		ToolResults: []chatstore.StoredToolResult{{
			ToolCallID: "call_1",
			Name:       "create_itinerary_items",
			Content:    `{"created":1}`,
		}},
	}
	if err := store.AddMessage(ctx, userID, tripID, session.ID, toolResultMsg); err != nil {
		t.Fatalf("add tool result message: %v", err)
	}

	messages, err := store.GetMessages(ctx, userID, tripID, session.ID, 10)
	if err != nil {
		t.Fatalf("get messages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("got %d messages, want 2", len(messages))
	}

	tc := messages[0].ToolCalls
	if len(tc) != 1 || tc[0].ID != "call_1" || tc[0].Name != "create_itinerary_items" ||
		tc[0].Arguments != `{"items":[{"title":"Sushi at Tsukiji","day":2}]}` ||
		tc[0].ThoughtSignature != "opaque-gemini-token" {
		t.Errorf("tool calls did not round-trip: %+v", tc)
	}
	tr := messages[1].ToolResults
	if len(tr) != 1 || tr[0].ToolCallID != "call_1" || tr[0].Content != `{"created":1}` {
		t.Errorf("tool results did not round-trip: %+v", tr)
	}
	if messages[1].Metadata["synthetic"] != "true" {
		t.Errorf("metadata did not round-trip: %+v", messages[1].Metadata)
	}

	// Neither message carries user-visible content, so messageCount stays 0.
	session, err = store.GetSession(ctx, userID, tripID, session.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if session.MessageCount != 0 {
		t.Errorf("MessageCount = %d, want 0 (tool-loop intermediates must not count)", session.MessageCount)
	}
}

// TestChatStoreImplicitSessionCreation pins the AddMessage upsert path: a
// message addressed to an unknown (but valid-UUID) session ID implicitly
// creates the session, matching the old Firestore MergeAll behaviour.
func TestChatStoreImplicitSessionCreation(t *testing.T) {
	env := NewTestEnv(t)
	env.CleanDB(t)
	ctx := context.Background()
	store := chatstore.New(env.Pool)

	userID := newChatFixtureUser(t, ctx, env, "implicit")
	tripID := uuid.NewString()
	sessionID := uuid.NewString() // never created via CreateSession

	msg := &chatstore.ChatMessage{Role: "user", Content: "Hello out of nowhere"}
	if err := store.AddMessageWithMode(ctx, userID, tripID, sessionID, msg, "companion"); err != nil {
		t.Fatalf("add message to unknown session: %v", err)
	}

	session, err := store.GetSession(ctx, userID, tripID, sessionID)
	if err != nil {
		t.Fatalf("get implicitly created session: %v", err)
	}
	if session.Mode != "companion" {
		t.Errorf("mode = %q, want %q", session.Mode, "companion")
	}
	if session.MessageCount != 1 {
		t.Errorf("MessageCount = %d, want 1", session.MessageCount)
	}
	if session.CreatedAt.IsZero() {
		t.Error("implicitly created session must have CreatedAt set")
	}
}

// TestChatStoreCrossUserSessionIsolation verifies that a session ID owned
// by one user cannot be read, written to, or deleted through another
// user's scope. Firestore's path-keyed layout gave this for free; the
// Postgres schema enforces it with (user_id, trip_id) guards.
func TestChatStoreCrossUserSessionIsolation(t *testing.T) {
	env := NewTestEnv(t)
	env.CleanDB(t)
	ctx := context.Background()
	store := chatstore.New(env.Pool)

	victimID := newChatFixtureUser(t, ctx, env, "victim")
	attackerID := newChatFixtureUser(t, ctx, env, "attacker")
	tripID := uuid.NewString()

	session, err := store.CreateSession(ctx, victimID, tripID, "planning")
	if err != nil {
		t.Fatalf("create victim session: %v", err)
	}
	if err := store.AddMessage(ctx, victimID, tripID, session.ID, &chatstore.ChatMessage{
		Role: "user", Content: "private plans",
	}); err != nil {
		t.Fatalf("add victim message: %v", err)
	}

	// Reads under the attacker's scope must come back empty / not found.
	if _, err := store.GetSession(ctx, attackerID, tripID, session.ID); !errors.Is(err, chatstore.ErrNotFound) {
		t.Errorf("GetSession under attacker scope: got err %v, want ErrNotFound", err)
	}
	msgs, err := store.GetMessages(ctx, attackerID, tripID, session.ID, 10)
	if err != nil {
		t.Fatalf("get messages under attacker scope: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("attacker read %d messages from victim session, want 0", len(msgs))
	}

	// A write to the victim's session ID under the attacker's scope must be
	// rejected — not silently attached to the victim's session.
	err = store.AddMessage(ctx, attackerID, tripID, session.ID, &chatstore.ChatMessage{
		Role: "user", Content: "injected",
	})
	if !errors.Is(err, chatstore.ErrNotFound) {
		t.Errorf("AddMessage under attacker scope: got err %v, want ErrNotFound", err)
	}

	// The victim's history must be unchanged.
	msgs, err = store.GetMessages(ctx, victimID, tripID, session.ID, 10)
	if err != nil {
		t.Fatalf("get victim messages: %v", err)
	}
	if len(msgs) != 1 || msgs[0].Content != "private plans" {
		t.Errorf("victim history changed: %+v", msgs)
	}

	// Deletes under the attacker's scope must be no-ops.
	if err := store.DeleteSession(ctx, attackerID, tripID, session.ID); err != nil {
		t.Fatalf("delete under attacker scope errored: %v", err)
	}
	if _, err := store.GetSession(ctx, victimID, tripID, session.ID); err != nil {
		t.Errorf("victim session disappeared after attacker delete attempt: %v", err)
	}
}

// TestChatStoreMoveSessionToTrip covers the selection-mode flow: a chat
// starts in the "_lobby" scope, the AI creates a trip mid-conversation,
// and the session is retroactively linked to the new trip.
func TestChatStoreMoveSessionToTrip(t *testing.T) {
	env := NewTestEnv(t)
	env.CleanDB(t)
	ctx := context.Background()
	store := chatstore.New(env.Pool)

	userID := newChatFixtureUser(t, ctx, env, "mover")
	toTripID := uuid.NewString()

	session, err := store.CreateSession(ctx, userID, "_lobby", "selection")
	if err != nil {
		t.Fatalf("create lobby session: %v", err)
	}
	if err := store.AddMessage(ctx, userID, "_lobby", session.ID, &chatstore.ChatMessage{
		Role: "user", Content: "I want to go to Portugal in May",
	}); err != nil {
		t.Fatalf("add lobby message: %v", err)
	}

	if err := store.MoveSessionToTrip(ctx, userID, "_lobby", toTripID, session.ID); err != nil {
		t.Fatalf("move session: %v", err)
	}

	// Session and messages are now visible under the trip scope...
	moved, err := store.GetSession(ctx, userID, toTripID, session.ID)
	if err != nil {
		t.Fatalf("get moved session: %v", err)
	}
	if moved.TripID != toTripID {
		t.Errorf("moved TripID = %q, want %q", moved.TripID, toTripID)
	}
	msgs, err := store.GetMessages(ctx, userID, toTripID, session.ID, 10)
	if err != nil {
		t.Fatalf("get moved messages: %v", err)
	}
	if len(msgs) != 1 {
		t.Errorf("got %d messages under new trip, want 1", len(msgs))
	}

	// ...and gone from the lobby.
	lobbySessions, err := store.ListSessions(ctx, userID, "_lobby", 10)
	if err != nil {
		t.Fatalf("list lobby sessions: %v", err)
	}
	if len(lobbySessions) != 0 {
		t.Errorf("got %d sessions still in lobby, want 0", len(lobbySessions))
	}

	// Moving a session that isn't in the source scope errors.
	if err := store.MoveSessionToTrip(ctx, userID, "_lobby", toTripID, session.ID); !errors.Is(err, chatstore.ErrNotFound) {
		t.Errorf("second move: got err %v, want ErrNotFound", err)
	}
}

// TestChatStoreTTLAndPurge covers the retention path end-to-end: SetTTL
// stamps expire_at on the trip's sessions, and PurgeExpiredChatSessions
// (the lifecycle job's query) deletes them once expired.
func TestChatStoreTTLAndPurge(t *testing.T) {
	env := NewTestEnv(t)
	env.CleanDB(t)
	ctx := context.Background()
	store := chatstore.New(env.Pool)
	queries := dbgen.New(env.Pool)

	userID := newChatFixtureUser(t, ctx, env, "ttl")
	expiredTripID := uuid.NewString()
	keptTripID := uuid.NewString()

	expiredSession, err := store.CreateSession(ctx, userID, expiredTripID, "planning")
	if err != nil {
		t.Fatalf("create expired session: %v", err)
	}
	if err := store.AddMessage(ctx, userID, expiredTripID, expiredSession.ID, &chatstore.ChatMessage{
		Role: "user", Content: "old chat",
	}); err != nil {
		t.Fatalf("add message: %v", err)
	}
	keptSession, err := store.CreateSession(ctx, userID, keptTripID, "planning")
	if err != nil {
		t.Fatalf("create kept session: %v", err)
	}

	// Stamp one trip as already expired, the other 90 days out.
	if err := store.SetTTL(ctx, userID, expiredTripID, time.Now().Add(-time.Hour)); err != nil {
		t.Fatalf("set expired TTL: %v", err)
	}
	if err := store.SetTTL(ctx, userID, keptTripID, time.Now().AddDate(0, 0, 90)); err != nil {
		t.Fatalf("set future TTL: %v", err)
	}

	got, err := store.GetSession(ctx, userID, keptTripID, keptSession.ID)
	if err != nil {
		t.Fatalf("get kept session: %v", err)
	}
	if got.ExpireAt == nil {
		t.Error("expected ExpireAt to be set after SetTTL")
	}

	purged, err := queries.PurgeExpiredChatSessions(ctx)
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if purged != 1 {
		t.Errorf("purged %d sessions, want 1", purged)
	}

	if _, err := store.GetSession(ctx, userID, expiredTripID, expiredSession.ID); !errors.Is(err, chatstore.ErrNotFound) {
		t.Errorf("expired session still readable: err = %v, want ErrNotFound", err)
	}
	if _, err := store.GetSession(ctx, userID, keptTripID, keptSession.ID); err != nil {
		t.Errorf("kept session was purged early: %v", err)
	}

	// The expired session's messages must be gone too (cascade).
	var remaining int
	if err := env.Pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM chat_messages WHERE session_id = $1", expiredSession.ID,
	).Scan(&remaining); err != nil {
		t.Fatalf("count messages: %v", err)
	}
	if remaining != 0 {
		t.Errorf("got %d messages left after purge, want 0", remaining)
	}
}

// TestChatStoreSummary covers UpdateSummary + GetOldestMessages, the two
// halves of the conversation-summarization path.
func TestChatStoreSummary(t *testing.T) {
	env := NewTestEnv(t)
	env.CleanDB(t)
	ctx := context.Background()
	store := chatstore.New(env.Pool)

	userID := newChatFixtureUser(t, ctx, env, "summary")
	tripID := uuid.NewString()

	session, err := store.CreateSession(ctx, userID, tripID, "planning")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	for _, content := range []string{"first", "second", "third"} {
		if err := store.AddMessage(ctx, userID, tripID, session.ID, &chatstore.ChatMessage{
			Role: "user", Content: content,
		}); err != nil {
			t.Fatalf("add message %q: %v", content, err)
		}
	}

	oldest, err := store.GetOldestMessages(ctx, userID, tripID, session.ID, 2)
	if err != nil {
		t.Fatalf("get oldest: %v", err)
	}
	if len(oldest) != 2 || oldest[0].Content != "first" || oldest[1].Content != "second" {
		t.Errorf("oldest messages wrong: %+v", oldest)
	}

	if err := store.UpdateSummary(ctx, userID, tripID, session.ID, "They discussed three things.", 3); err != nil {
		t.Fatalf("update summary: %v", err)
	}
	got, err := store.GetSession(ctx, userID, tripID, session.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if got.Summary != "They discussed three things." || got.SummaryMessageCount != 3 {
		t.Errorf("summary round-trip failed: %q / %d", got.Summary, got.SummaryMessageCount)
	}
}
