//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/gallowaysoftware/toqui-backend/internal/chatstore"
)

func TestChatStoreSessionAndMessages(t *testing.T) {
	env := NewTestEnv(t)
	ctx := context.Background()
	store := chatstore.New(env.Firestore)

	userID := "test-user-chat"
	tripID := "test-trip-chat"

	// Create session
	session, err := store.CreateSession(ctx, userID, tripID, "planning")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if session.Mode != "planning" {
		t.Errorf("mode = %q, want %q", session.Mode, "planning")
	}

	// Get session
	got, err := store.GetSession(ctx, userID, tripID, session.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if got.ID != session.ID {
		t.Errorf("got ID = %q, want %q", got.ID, session.ID)
	}

	// Add messages
	userMsg := &chatstore.ChatMessage{Role: "user", Content: "Hello, where should I go in Tokyo?"}
	if err := store.AddMessage(ctx, userID, tripID, session.ID, userMsg); err != nil {
		t.Fatalf("add user message: %v", err)
	}

	assistantMsg := &chatstore.ChatMessage{Role: "assistant", Content: "I'd recommend Shinjuku and Shibuya!"}
	if err := store.AddMessage(ctx, userID, tripID, session.ID, assistantMsg); err != nil {
		t.Fatalf("add assistant message: %v", err)
	}

	// Get messages
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
}

func TestChatStoreTTL(t *testing.T) {
	env := NewTestEnv(t)
	ctx := context.Background()
	store := chatstore.New(env.Firestore)

	userID := "test-user-ttl"
	tripID := "test-trip-ttl"

	session, err := store.CreateSession(ctx, userID, tripID, "planning")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	msg := &chatstore.ChatMessage{Role: "user", Content: "Test TTL message"}
	if err := store.AddMessage(ctx, userID, tripID, session.ID, msg); err != nil {
		t.Fatalf("add message: %v", err)
	}

	// SetTTL should not error
	expiry := session.CreatedAt.AddDate(0, 0, 90)
	if err := store.SetTTL(ctx, userID, tripID, expiry); err != nil {
		t.Fatalf("set TTL: %v", err)
	}

	// Verify session still readable (TTL is just a field, Firestore handles deletion)
	got, err := store.GetSession(ctx, userID, tripID, session.ID)
	if err != nil {
		t.Fatalf("get session after TTL: %v", err)
	}
	if got.ExpireAt == nil {
		t.Error("expected expireAt to be set")
	}

	// Cleanup
	store.DeleteAllForTrip(ctx, userID, tripID)
}
