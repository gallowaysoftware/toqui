package chatstore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/google/uuid"
	"google.golang.org/api/iterator"
)

type ChatSession struct {
	ID            string     `firestore:"id"`
	TripID        string     `firestore:"tripId"`
	Mode          string     `firestore:"mode"` // planning, companion
	CreatedAt     time.Time  `firestore:"createdAt"`
	LastMessageAt time.Time  `firestore:"lastMessageAt"`
	ExpireAt      *time.Time `firestore:"expireAt,omitempty"`
}

type ChatMessage struct {
	ID        string            `firestore:"id"`
	SessionID string            `firestore:"sessionId"`
	Role      string            `firestore:"role"` // user, assistant, system
	Content   string            `firestore:"content"`
	Metadata  map[string]string `firestore:"metadata"`
	CreatedAt time.Time         `firestore:"createdAt"`
	ExpireAt  *time.Time        `firestore:"expireAt,omitempty"`

	// ToolCalls stores tool calls made by the assistant in this message.
	// Each entry has ID, Name, and Arguments (JSON string).
	ToolCalls []StoredToolCall `firestore:"toolCalls,omitempty"`

	// ToolResults stores tool execution results returned to the AI.
	// Each entry has ToolCallID, Name, and Content (JSON string).
	ToolResults []StoredToolResult `firestore:"toolResults,omitempty"`
}

// StoredToolCall is a Firestore-friendly representation of an AI tool call.
type StoredToolCall struct {
	ID        string `firestore:"id"`
	Name      string `firestore:"name"`
	Arguments string `firestore:"arguments"` // JSON string
}

// StoredToolResult is a Firestore-friendly representation of a tool execution result.
type StoredToolResult struct {
	ToolCallID string `firestore:"toolCallId"`
	Name       string `firestore:"name"`
	Content    string `firestore:"content"` // JSON string
}

type Store struct {
	client *firestore.Client
}

func New(client *firestore.Client) *Store {
	return &Store{client: client}
}

func (s *Store) sessionsCol(userID string, tripID string) *firestore.CollectionRef {
	return s.client.Collection("users").Doc(userID).Collection("trips").Doc(tripID).Collection("chatSessions")
}

func (s *Store) messagesCol(userID, tripID, sessionID string) *firestore.CollectionRef {
	return s.sessionsCol(userID, tripID).Doc(sessionID).Collection("messages")
}

func (s *Store) CreateSession(ctx context.Context, userID, tripID, mode string) (*ChatSession, error) {
	session := &ChatSession{
		ID:            uuid.New().String(),
		TripID:        tripID,
		Mode:          mode,
		CreatedAt:     time.Now(),
		LastMessageAt: time.Now(),
	}

	_, err := s.sessionsCol(userID, tripID).Doc(session.ID).Set(ctx, session)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	return session, nil
}

func (s *Store) GetSession(ctx context.Context, userID, tripID, sessionID string) (*ChatSession, error) {
	doc, err := s.sessionsCol(userID, tripID).Doc(sessionID).Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}

	var session ChatSession
	if err := doc.DataTo(&session); err != nil {
		return nil, fmt.Errorf("decode session: %w", err)
	}
	return &session, nil
}

func (s *Store) ListSessions(ctx context.Context, userID, tripID string, limit int) ([]*ChatSession, error) {
	iter := s.sessionsCol(userID, tripID).OrderBy("lastMessageAt", firestore.Desc).Limit(limit).Documents(ctx)
	defer iter.Stop()

	var sessions []*ChatSession
	for {
		doc, err := iter.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("list sessions: %w", err)
		}

		var session ChatSession
		if err := doc.DataTo(&session); err != nil {
			return nil, fmt.Errorf("decode session: %w", err)
		}
		sessions = append(sessions, &session)
	}
	return sessions, nil
}

// AddMessage stores a message and updates the session's lastMessageAt timestamp.
// IMPORTANT: This mutates msg in-place, setting msg.ID, msg.SessionID, and
// msg.CreatedAt. Callers rely on msg.ID being populated after a successful call
// (e.g., to include the message ID in stream events).
func (s *Store) AddMessage(ctx context.Context, userID, tripID, sessionID string, msg *ChatMessage) error {
	msg.ID = uuid.New().String()
	msg.SessionID = sessionID
	msg.CreatedAt = time.Now()

	batch := s.client.Batch()
	batch.Set(s.messagesCol(userID, tripID, sessionID).Doc(msg.ID), msg)
	batch.Update(s.sessionsCol(userID, tripID).Doc(sessionID), []firestore.Update{
		{Path: "lastMessageAt", Value: time.Now()},
	})

	_, err := batch.Commit(ctx)
	if err != nil {
		return fmt.Errorf("add message: %w", err)
	}
	return nil
}

func (s *Store) GetMessages(ctx context.Context, userID, tripID, sessionID string, limit int) ([]*ChatMessage, error) {
	iter := s.messagesCol(userID, tripID, sessionID).OrderBy("createdAt", firestore.Asc).Limit(limit).Documents(ctx)
	defer iter.Stop()

	var messages []*ChatMessage
	for {
		doc, err := iter.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("get messages: %w", err)
		}

		var msg ChatMessage
		if err := doc.DataTo(&msg); err != nil {
			return nil, fmt.Errorf("decode message: %w", err)
		}
		messages = append(messages, &msg)
	}
	return messages, nil
}

// SetTTL stamps an expireAt time on all sessions and messages for a trip.
// Firestore's TTL policy will automatically delete expired documents.
// Configure the TTL policy: gcloud firestore fields ttls update expireAt --collection-group=messages
// and: gcloud firestore fields ttls update expireAt --collection-group=chatSessions
func (s *Store) SetTTL(ctx context.Context, userID, tripID string, expireAt time.Time) error {
	sessions, err := s.ListSessions(ctx, userID, tripID, 1000)
	if err != nil {
		return fmt.Errorf("list sessions for TTL: %w", err)
	}

	for _, session := range sessions {
		batch := s.client.Batch()
		batchCount := 0

		// Stamp session
		batch.Update(s.sessionsCol(userID, tripID).Doc(session.ID), []firestore.Update{
			{Path: "expireAt", Value: expireAt},
		})
		batchCount++

		// Stamp messages
		iter := s.messagesCol(userID, tripID, session.ID).Documents(ctx)
		for {
			doc, iterErr := iter.Next()
			if errors.Is(iterErr, iterator.Done) {
				break
			}
			if iterErr != nil {
				iter.Stop()
				return fmt.Errorf("iterate messages for TTL: %w", iterErr)
			}
			batch.Update(doc.Ref, []firestore.Update{
				{Path: "expireAt", Value: expireAt},
			})
			batchCount++

			// Firestore batch limit is 500
			if batchCount >= 490 {
				if _, commitErr := batch.Commit(ctx); commitErr != nil {
					iter.Stop()
					return fmt.Errorf("batch TTL update: %w", commitErr)
				}
				batch = s.client.Batch()
				batchCount = 0
			}
		}
		iter.Stop()

		if batchCount > 0 {
			if _, commitErr := batch.Commit(ctx); commitErr != nil {
				return fmt.Errorf("batch TTL update: %w", commitErr)
			}
		}
	}

	return nil
}

// DeleteAllForTrip deletes all chat sessions and messages for a trip.
// Used for trip deletion and data lifecycle archival.
func (s *Store) DeleteAllForTrip(ctx context.Context, userID, tripID string) error {
	sessions, err := s.ListSessions(ctx, userID, tripID, 1000)
	if err != nil {
		return fmt.Errorf("list sessions for deletion: %w", err)
	}

	for _, session := range sessions {
		// Delete all messages in this session
		if err := s.deleteCollection(ctx, s.messagesCol(userID, tripID, session.ID)); err != nil {
			return fmt.Errorf("delete messages for session %s: %w", session.ID, err)
		}

		// Delete the session document
		if _, err := s.sessionsCol(userID, tripID).Doc(session.ID).Delete(ctx); err != nil {
			return fmt.Errorf("delete session %s: %w", session.ID, err)
		}
	}

	return nil
}

// deleteCollection deletes all documents in a Firestore collection in batches.
func (s *Store) deleteCollection(ctx context.Context, col *firestore.CollectionRef) error {
	const batchSize = 100

	for {
		iter := col.Limit(batchSize).Documents(ctx)
		batch := s.client.Batch()
		count := 0

		for {
			doc, err := iter.Next()
			if errors.Is(err, iterator.Done) {
				break
			}
			if err != nil {
				iter.Stop()
				return fmt.Errorf("iterate for deletion: %w", err)
			}
			batch.Delete(doc.Ref)
			count++
		}
		iter.Stop()

		if count == 0 {
			return nil
		}

		if _, err := batch.Commit(ctx); err != nil {
			return fmt.Errorf("batch delete: %w", err)
		}
	}
}
