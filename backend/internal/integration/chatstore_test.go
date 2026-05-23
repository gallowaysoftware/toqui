//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/google/uuid"

	"github.com/gallowaysoftware/toqui/backend/internal/chatstore"
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

// TestChatStore_SessionIDFromDocRef pins the fix for #335 (Run 22 R-11 P2).
// If a session doc exists in Firestore without the denormalised "id" data
// field — whether from legacy data, a forgotten write path, or an external
// writer — GetSession and ListSessions must still return the session with
// the correct ID, populated from the doc's path component which is always
// authoritative.
//
// The test bypasses the store's public write API and writes a bare doc
// directly (no "id" field in the body) to simulate the hostile condition.
func TestChatStore_SessionIDFromDocRef(t *testing.T) {
	env := NewTestEnv(t)
	ctx := context.Background()
	store := chatstore.New(env.Firestore)

	// UUID-suffixed IDs so parallel / back-to-back runs on a persistent
	// emulator never collide — a time-based suffix loses to two test
	// invocations in the same second.
	suffix := uuid.NewString()
	userID := "test-user-id-from-ref-" + suffix
	tripID := "test-trip-id-from-ref-" + suffix
	sessionID := "session-without-id-field-" + suffix

	t.Cleanup(func() {
		_ = store.DeleteAllForTrip(context.Background(), userID, tripID)
	})

	// Write a session doc directly with NO "id" field in the body. This
	// mirrors the R-11 production scenario where the list endpoint returned
	// sessionId:null despite the doc existing and being queryable by ID.
	now := time.Now()
	sessionRef := env.Firestore.
		Collection("users").Doc(userID).
		Collection("trips").Doc(tripID).
		Collection("chatSessions").Doc(sessionID)
	if _, err := sessionRef.Set(ctx, map[string]interface{}{
		"mode":          "companion",
		"createdAt":     now,
		"lastMessageAt": now,
		"messageCount":  int64(20),
		// Deliberately NO "id" key AND NO "tripId" key. Both must be
		// repopulated from the doc path (doc.Ref.ID for session ID,
		// doc.Ref.Parent.Parent.ID for trip ID). This is the strictest
		// hostile-condition the prod bug could have produced.
	}, firestore.MergeAll); err != nil {
		t.Fatalf("write bare session doc: %v", err)
	}

	t.Run("GetSession populates ID and TripID from doc ref", func(t *testing.T) {
		got, err := store.GetSession(ctx, userID, tripID, sessionID)
		if err != nil {
			t.Fatalf("get session: %v", err)
		}
		if got.ID != sessionID {
			t.Errorf("got.ID = %q, want %q (doc had no id field; ID must come from doc.Ref.ID)", got.ID, sessionID)
		}
		if got.TripID != tripID {
			t.Errorf("got.TripID = %q, want %q (doc had no tripId field; TripID must come from doc.Ref.Parent.Parent.ID)", got.TripID, tripID)
		}
		if got.MessageCount != 20 {
			t.Errorf("got.MessageCount = %d, want 20 (sanity check that the rest of the doc decoded)", got.MessageCount)
		}
	})

	t.Run("ListSessions populates ID and TripID from doc ref", func(t *testing.T) {
		sessions, err := store.ListSessions(ctx, userID, tripID, 10)
		if err != nil {
			t.Fatalf("list sessions: %v", err)
		}
		if len(sessions) != 1 {
			t.Fatalf("got %d sessions, want 1", len(sessions))
		}
		if sessions[0].ID != sessionID {
			t.Errorf("sessions[0].ID = %q, want %q (this is the exact #335 regression)", sessions[0].ID, sessionID)
		}
		if sessions[0].TripID != tripID {
			t.Errorf("sessions[0].TripID = %q, want %q (TripID must come from doc.Ref.Parent.Parent.ID)", sessions[0].TripID, tripID)
		}
		if sessions[0].MessageCount != 20 {
			t.Errorf("sessions[0].MessageCount = %d, want 20", sessions[0].MessageCount)
		}
	})
}

// TestChatStore_MessageIDAndSessionIDFromDocRef pins #341 — the defence-in-
// depth extension of #339/#340 to ChatMessage. The message doc path is
// users/{uid}/trips/{tid}/chatSessions/{sid}/messages/{mid}, so doc.Ref.ID
// authoritatively holds the message ID and doc.Ref.Parent.Parent.ID holds
// the session ID. If any write path ever wrote a message doc without its
// "id" or "sessionId" fields, the read path must still return a fully
// identified message rather than `id: ""` / `sessionId: ""`.
func TestChatStore_MessageIDAndSessionIDFromDocRef(t *testing.T) {
	env := NewTestEnv(t)
	ctx := context.Background()
	store := chatstore.New(env.Firestore)

	suffix := uuid.NewString()
	userID := "test-user-msg-ref-" + suffix
	tripID := "test-trip-msg-ref-" + suffix
	sessionID := "test-session-msg-ref-" + suffix
	messageID := "bare-message-" + suffix

	t.Cleanup(func() {
		_ = store.DeleteAllForTrip(context.Background(), userID, tripID)
	})

	// Seed the session doc via the normal path so listing works.
	if _, err := store.CreateSession(ctx, userID, tripID, "planning"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	// Write a message doc directly with NO "id" and NO "sessionId" field.
	// This is the hostile scenario that #341 defends against.
	now := time.Now()
	messageRef := env.Firestore.
		Collection("users").Doc(userID).
		Collection("trips").Doc(tripID).
		Collection("chatSessions").Doc(sessionID).
		Collection("messages").Doc(messageID)
	if _, err := messageRef.Set(ctx, map[string]interface{}{
		"role":      "user",
		"content":   "Hello from a doc without id or sessionId",
		"createdAt": now,
		// Deliberately NO "id" and NO "sessionId" keys.
	}, firestore.MergeAll); err != nil {
		t.Fatalf("write bare message doc: %v", err)
	}

	t.Run("GetMessages populates ID and SessionID from doc ref", func(t *testing.T) {
		messages, err := store.GetMessages(ctx, userID, tripID, sessionID, 10)
		if err != nil {
			t.Fatalf("get messages: %v", err)
		}
		if len(messages) != 1 {
			t.Fatalf("got %d messages, want 1", len(messages))
		}
		if messages[0].ID != messageID {
			t.Errorf("messages[0].ID = %q, want %q (ID must come from doc.Ref.ID)", messages[0].ID, messageID)
		}
		if messages[0].SessionID != sessionID {
			t.Errorf("messages[0].SessionID = %q, want %q (SessionID must come from doc.Ref.Parent.Parent.ID)", messages[0].SessionID, sessionID)
		}
		if messages[0].Content != "Hello from a doc without id or sessionId" {
			t.Errorf("content roundtrip failed: got %q", messages[0].Content)
		}
	})

	t.Run("GetOldestMessages populates ID and SessionID from doc ref", func(t *testing.T) {
		messages, err := store.GetOldestMessages(ctx, userID, tripID, sessionID, 10)
		if err != nil {
			t.Fatalf("get oldest messages: %v", err)
		}
		if len(messages) != 1 {
			t.Fatalf("got %d messages, want 1", len(messages))
		}
		if messages[0].ID != messageID {
			t.Errorf("messages[0].ID = %q, want %q", messages[0].ID, messageID)
		}
		if messages[0].SessionID != sessionID {
			t.Errorf("messages[0].SessionID = %q, want %q", messages[0].SessionID, sessionID)
		}
	})
}
