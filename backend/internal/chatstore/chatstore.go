// Package chatstore persists chat sessions and messages in Postgres.
// It replaced the original Firestore implementation when the project
// became self-hostable — the public API (method signatures, struct
// JSON shapes) is unchanged so callers and the GDPR export format are
// unaffected.
package chatstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gallowaysoftware/toqui/backend/internal/dbgen"
)

// ErrNotFound is returned when a session or message doesn't exist under
// the given (user, trip) scope. A session that exists but belongs to a
// different user or trip is deliberately indistinguishable from one that
// doesn't exist.
var ErrNotFound = errors.New("chatstore: not found")

// ChatSession's json tags define the GDPR Article 20 export wire shape
// (snake_case) — do not change them.
type ChatSession struct {
	ID            string     `json:"id"`
	TripID        string     `json:"trip_id"`
	Mode          string     `json:"mode"` // planning, companion
	CreatedAt     time.Time  `json:"created_at"`
	LastMessageAt time.Time  `json:"last_message_at"`
	MessageCount  int        `json:"message_count"`
	ExpireAt      *time.Time `json:"expire_at,omitempty"`

	// Summary holds an AI-generated summary of older messages that fall
	// outside the 50-message recent window. Injected into the system prompt
	// so the AI retains context from earlier in the conversation.
	Summary string `json:"summary,omitempty"`

	// SummaryMessageCount records the session's MessageCount at the time the
	// summary was last generated. A new summary is triggered when
	// MessageCount - SummaryMessageCount > SummaryRefreshThreshold.
	SummaryMessageCount int `json:"summary_message_count,omitempty"`
}

type ChatMessage struct {
	ID        string            `json:"id"`
	SessionID string            `json:"session_id"`
	Role      string            `json:"role"` // user, assistant, system
	Content   string            `json:"content"`
	Metadata  map[string]string `json:"metadata"`
	CreatedAt time.Time         `json:"created_at"`

	// ToolCalls stores tool calls made by the assistant in this message.
	// Each entry has ID, Name, and Arguments (JSON string).
	ToolCalls []StoredToolCall `json:"tool_calls,omitempty"`

	// ToolResults stores tool execution results returned to the AI.
	// Each entry has ToolCallID, Name, and Content (JSON string).
	ToolResults []StoredToolResult `json:"tool_results,omitempty"`
}

// StoredToolCall is the persisted representation of an AI tool call.
type StoredToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
	// ThoughtSignature is a Gemini 3 opaque token for reasoning continuity
	// across tool-call turns. Empty for Gemini 2.5 and Claude.
	ThoughtSignature string `json:"thought_signature,omitempty"`
}

// StoredToolResult is the persisted representation of a tool execution result.
type StoredToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Name       string `json:"name"`
	Content    string `json:"content"` // JSON string
}

type Store struct {
	pool    *pgxpool.Pool
	queries *dbgen.Queries
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool, queries: dbgen.New(pool)}
}

func (s *Store) CreateSession(ctx context.Context, userID, tripID, mode string) (*ChatSession, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("create session: parse user id: %w", err)
	}
	row, err := s.queries.CreateChatSession(ctx, dbgen.CreateChatSessionParams{
		ID:        uuid.New(),
		UserID:    uid,
		TripID:    tripID,
		Mode:      mode,
		CreatedAt: time.Now(),
	})
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	return sessionFromRow(row), nil
}

func (s *Store) GetSession(ctx context.Context, userID, tripID, sessionID string) (*ChatSession, error) {
	uid, sid, err := parseIDs(userID, sessionID)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}
	row, err := s.queries.GetChatSession(ctx, dbgen.GetChatSessionParams{
		ID:     sid,
		UserID: uid,
		TripID: tripID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("get session: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}
	return sessionFromRow(row), nil
}

// DeleteSession deletes a single session (its messages cascade).
// Used to clean up orphaned sessions that were created but never received messages.
func (s *Store) DeleteSession(ctx context.Context, userID, tripID, sessionID string) error {
	uid, sid, err := parseIDs(userID, sessionID)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	if err := s.queries.DeleteChatSession(ctx, dbgen.DeleteChatSessionParams{
		ID:     sid,
		UserID: uid,
		TripID: tripID,
	}); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// DeleteMessage deletes a single message.
// Used to roll back a persisted user message when the AI response fails,
// preventing orphaned messages from appearing in chat history.
func (s *Store) DeleteMessage(ctx context.Context, userID, tripID, sessionID, messageID string) error {
	uid, sid, err := parseIDs(userID, sessionID)
	if err != nil {
		return fmt.Errorf("delete message: %w", err)
	}
	mid, err := uuid.Parse(messageID)
	if err != nil {
		return fmt.Errorf("delete message: parse message id: %w", err)
	}
	if err := s.queries.DeleteChatMessage(ctx, dbgen.DeleteChatMessageParams{
		ID:        mid,
		SessionID: sid,
		UserID:    uid,
		TripID:    tripID,
	}); err != nil {
		return fmt.Errorf("delete message: %w", err)
	}
	return nil
}

func (s *Store) ListSessions(ctx context.Context, userID, tripID string, limit int) ([]*ChatSession, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("list sessions: parse user id: %w", err)
	}
	rows, err := s.queries.ListChatSessions(ctx, dbgen.ListChatSessionsParams{
		UserID: uid,
		TripID: tripID,
		Limit:  int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	var sessions []*ChatSession
	for _, row := range rows {
		sessions = append(sessions, sessionFromRow(row))
	}
	return sessions, nil
}

// AddMessage stores a message and updates the session's lastMessageAt timestamp.
// IMPORTANT: This mutates msg in-place, setting msg.ID, msg.SessionID, and
// msg.CreatedAt. Callers rely on msg.ID being populated after a successful call
// (e.g., to include the message ID in stream events).
func (s *Store) AddMessage(ctx context.Context, userID, tripID, sessionID string, msg *ChatMessage) error {
	return s.AddMessageWithMode(ctx, userID, tripID, sessionID, msg, "")
}

// AddMessageWithMode stores a message and updates the session metadata in a
// single transaction. If the session doesn't exist yet (client passed an
// unknown session ID), it is created implicitly. If mode is non-empty, the
// session's mode field is updated — this ensures that a session originally
// created in "selection" mode gets updated to "planning" or "companion"
// when subsequent messages use a different mode (Run 8 R-05, N-12 P2).
func (s *Store) AddMessageWithMode(ctx context.Context, userID, tripID, sessionID string, msg *ChatMessage, mode string) error {
	uid, sid, err := parseIDs(userID, sessionID)
	if err != nil {
		return fmt.Errorf("add message: %w", err)
	}

	msg.ID = uuid.New().String()
	msg.SessionID = sessionID
	msg.CreatedAt = time.Now()

	metadata, toolCalls, toolResults, err := marshalMessageJSON(msg)
	if err != nil {
		return fmt.Errorf("add message: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("add message: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit

	q := s.queries.WithTx(tx)

	// Only count user/assistant messages with content — not tool-loop
	// intermediates (empty assistant with tool_calls, user with tool_results).
	// This makes messageCount match what GetChatHistory returns after
	// isToolLoopIntermediate filtering (Run 17 N-02 P2).
	countMessage := msg.Content != "" && (msg.Role == "user" || msg.Role == "assistant")

	if _, err := q.UpsertChatSessionForMessage(ctx, dbgen.UpsertChatSessionForMessageParams{
		ID:            sid,
		UserID:        uid,
		TripID:        tripID,
		Mode:          mode,
		LastMessageAt: msg.CreatedAt,
		CountMessage:  countMessage,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// The session ID exists under a different user or trip. Firestore's
			// path-keyed layout made this collision structurally impossible;
			// here we refuse rather than attach messages across scopes.
			return fmt.Errorf("add message: %w", ErrNotFound)
		}
		return fmt.Errorf("add message: upsert session: %w", err)
	}

	if err := q.InsertChatMessage(ctx, dbgen.InsertChatMessageParams{
		ID:          uuid.MustParse(msg.ID),
		SessionID:   sid,
		Role:        msg.Role,
		Content:     msg.Content,
		Metadata:    metadata,
		ToolCalls:   toolCalls,
		ToolResults: toolResults,
		CreatedAt:   msg.CreatedAt,
	}); err != nil {
		return fmt.Errorf("add message: insert: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("add message: commit: %w", err)
	}
	return nil
}

// GetMessages returns the newest `limit` messages in chronological order.
// Fetching the NEWEST messages ensures that in tool-heavy conversations
// (where each tool call generates multiple intermediate messages), the
// most recent user messages are never silently dropped.
func (s *Store) GetMessages(ctx context.Context, userID, tripID, sessionID string, limit int) ([]*ChatMessage, error) {
	uid, sid, err := parseIDs(userID, sessionID)
	if err != nil {
		return nil, fmt.Errorf("get messages: %w", err)
	}
	rows, err := s.queries.GetNewestChatMessages(ctx, dbgen.GetNewestChatMessagesParams{
		SessionID: sid,
		UserID:    uid,
		TripID:    tripID,
		Limit:     int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("get messages: %w", err)
	}

	messages := make([]*ChatMessage, 0, len(rows))
	for _, row := range rows {
		msg, err := messageFromRow(row)
		if err != nil {
			return nil, fmt.Errorf("get messages: %w", err)
		}
		messages = append(messages, msg)
	}

	// Reverse to chronological order (oldest first) for the AI provider.
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
	return messages, nil
}

// GetOldestMessages returns the oldest messages in a session, ordered
// chronologically. Used to fetch the messages that fall outside the
// recent-message window for summarization.
func (s *Store) GetOldestMessages(ctx context.Context, userID, tripID, sessionID string, limit int) ([]*ChatMessage, error) {
	uid, sid, err := parseIDs(userID, sessionID)
	if err != nil {
		return nil, fmt.Errorf("get oldest messages: %w", err)
	}
	rows, err := s.queries.GetOldestChatMessages(ctx, dbgen.GetOldestChatMessagesParams{
		SessionID: sid,
		UserID:    uid,
		TripID:    tripID,
		Limit:     int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("get oldest messages: %w", err)
	}
	messages := make([]*ChatMessage, 0, len(rows))
	for _, row := range rows {
		msg, err := messageFromRow(row)
		if err != nil {
			return nil, fmt.Errorf("get oldest messages: %w", err)
		}
		messages = append(messages, msg)
	}
	return messages, nil
}

// UpdateSummary writes a conversation summary and the messageCount at which
// it was generated to the session. The summary is used to retain context
// from older messages that fall outside the 50-message window.
func (s *Store) UpdateSummary(ctx context.Context, userID, tripID, sessionID, summary string, messageCount int) error {
	uid, sid, err := parseIDs(userID, sessionID)
	if err != nil {
		return fmt.Errorf("update summary: %w", err)
	}
	if err := s.queries.UpdateChatSessionSummary(ctx, dbgen.UpdateChatSessionSummaryParams{
		ID:                  sid,
		UserID:              uid,
		TripID:              tripID,
		Summary:             summary,
		SummaryMessageCount: int32(messageCount),
	}); err != nil {
		return fmt.Errorf("update summary: %w", err)
	}
	return nil
}

// ExportChatData exports all chat sessions and messages for a user across
// all their trips. Used for GDPR Article 20 (data portability).
type ExportedSession struct {
	Session  *ChatSession   `json:"session"`
	Messages []*ChatMessage `json:"messages"`
}

func (s *Store) ExportChatData(ctx context.Context, userID string, tripIDs []string) (map[string][]ExportedSession, error) {
	result := make(map[string][]ExportedSession)

	for _, tripID := range tripIDs {
		sessions, err := s.ListSessions(ctx, userID, tripID, 1000)
		if err != nil {
			slog.Warn("export: failed to list sessions for trip", "trip_id", tripID, "error", err)
			continue
		}

		var exported []ExportedSession
		for _, session := range sessions {
			messages, err := s.GetMessages(ctx, userID, tripID, session.ID, 10000)
			if err != nil {
				slog.Warn("export: failed to get messages", "session_id", session.ID, "error", err)
				continue
			}
			exported = append(exported, ExportedSession{
				Session:  session,
				Messages: messages,
			})
		}
		if len(exported) > 0 {
			result[tripID] = exported
		}
	}

	return result, nil
}

// MoveSessionToTrip moves a session (and, implicitly, all its messages) from
// one trip scope to another. Used when a selection-mode session needs to be
// retroactively linked to a trip that was created mid-conversation.
func (s *Store) MoveSessionToTrip(ctx context.Context, userID, fromTripID, toTripID, sessionID string) error {
	uid, sid, err := parseIDs(userID, sessionID)
	if err != nil {
		return fmt.Errorf("move session: %w", err)
	}
	rows, err := s.queries.MoveChatSessionToTrip(ctx, dbgen.MoveChatSessionToTripParams{
		ToTripID:   toTripID,
		ID:         sid,
		UserID:     uid,
		FromTripID: fromTripID,
	})
	if err != nil {
		return fmt.Errorf("move session: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("move session: %w", ErrNotFound)
	}
	return nil
}

// SetTTL stamps an expireAt time on all sessions for a trip. The lifecycle
// background purge job deletes expired sessions (messages cascade).
func (s *Store) SetTTL(ctx context.Context, userID, tripID string, expireAt time.Time) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("set ttl: parse user id: %w", err)
	}
	if err := s.queries.SetChatTTLForTrip(ctx, dbgen.SetChatTTLForTripParams{
		UserID:   uid,
		TripID:   tripID,
		ExpireAt: pgtype.Timestamptz{Time: expireAt, Valid: true},
	}); err != nil {
		return fmt.Errorf("set ttl: %w", err)
	}
	return nil
}

// DeleteAllForTrip deletes all chat sessions and messages for a trip.
// Used for trip deletion and data lifecycle archival.
func (s *Store) DeleteAllForTrip(ctx context.Context, userID, tripID string) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("delete all for trip: parse user id: %w", err)
	}
	if err := s.queries.DeleteChatForTrip(ctx, dbgen.DeleteChatForTripParams{
		UserID: uid,
		TripID: tripID,
	}); err != nil {
		return fmt.Errorf("delete all for trip: %w", err)
	}
	return nil
}

func parseIDs(userID, sessionID string) (uuid.UUID, uuid.UUID, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("parse user id: %w", err)
	}
	sid, err := uuid.Parse(sessionID)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("parse session id: %w", err)
	}
	return uid, sid, nil
}

func sessionFromRow(row dbgen.ChatSession) *ChatSession {
	session := &ChatSession{
		ID:                  row.ID.String(),
		TripID:              row.TripID,
		Mode:                row.Mode,
		CreatedAt:           row.CreatedAt,
		LastMessageAt:       row.LastMessageAt,
		MessageCount:        int(row.MessageCount),
		Summary:             row.Summary,
		SummaryMessageCount: int(row.SummaryMessageCount),
	}
	if row.ExpireAt.Valid {
		t := row.ExpireAt.Time
		session.ExpireAt = &t
	}
	return session
}

func marshalMessageJSON(msg *ChatMessage) (metadata, toolCalls, toolResults []byte, err error) {
	if len(msg.Metadata) > 0 {
		if metadata, err = json.Marshal(msg.Metadata); err != nil {
			return nil, nil, nil, fmt.Errorf("marshal metadata: %w", err)
		}
	}
	if len(msg.ToolCalls) > 0 {
		if toolCalls, err = json.Marshal(msg.ToolCalls); err != nil {
			return nil, nil, nil, fmt.Errorf("marshal tool calls: %w", err)
		}
	}
	if len(msg.ToolResults) > 0 {
		if toolResults, err = json.Marshal(msg.ToolResults); err != nil {
			return nil, nil, nil, fmt.Errorf("marshal tool results: %w", err)
		}
	}
	return metadata, toolCalls, toolResults, nil
}

func messageFromRow(row dbgen.ChatMessage) (*ChatMessage, error) {
	msg := &ChatMessage{
		ID:        row.ID.String(),
		SessionID: row.SessionID.String(),
		Role:      row.Role,
		Content:   row.Content,
		CreatedAt: row.CreatedAt,
	}
	if len(row.Metadata) > 0 {
		if err := json.Unmarshal(row.Metadata, &msg.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal metadata for message %s: %w", msg.ID, err)
		}
	}
	if len(row.ToolCalls) > 0 {
		if err := json.Unmarshal(row.ToolCalls, &msg.ToolCalls); err != nil {
			return nil, fmt.Errorf("unmarshal tool calls for message %s: %w", msg.ID, err)
		}
	}
	if len(row.ToolResults) > 0 {
		if err := json.Unmarshal(row.ToolResults, &msg.ToolResults); err != nil {
			return nil, fmt.Errorf("unmarshal tool results for message %s: %w", msg.ID, err)
		}
	}
	return msg, nil
}
