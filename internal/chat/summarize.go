package chat

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gallowaysoftware/toqui-backend/internal/ai"
	"github.com/gallowaysoftware/toqui-backend/internal/chatstore"
)

const (
	// summaryMessageThreshold is the minimum number of messages in a session
	// before summarization is considered. Sessions with fewer messages do not
	// need a summary because the full 50-message window already covers them.
	summaryMessageThreshold = 50

	// summaryRefreshInterval controls how many new messages must accumulate
	// since the last summary before a re-summarization is triggered. This
	// prevents the expensive summarization call from firing on every message.
	summaryRefreshInterval = 20

	// summaryMaxTokens caps the output of the summarization call. The summary
	// should be concise — it is injected into the system prompt on every
	// subsequent request, so brevity saves tokens on every turn.
	summaryMaxTokens = 512

	// recentMessageWindow is the number of most-recent messages loaded as
	// full history for the AI request. Messages older than this window are
	// covered by the summary instead.
	recentMessageWindow = 50
)

// summarizationPrompt is the system prompt used for the summarization call.
// It instructs the model to produce a concise factual summary suitable for
// injection as conversation context.
const summarizationPrompt = `You are a conversation summarizer for a travel planning assistant. Summarize the following conversation, focusing on:

1. Key facts about the traveler (preferences, dietary restrictions, budget, travel dates, group composition)
2. Decisions already made (destinations chosen, activities confirmed, bookings made)
3. Important context (trip themes, accommodation preferences, transportation plans)
4. Any questions the traveler has already answered (so the assistant does not re-ask them)

Be concise and factual. Output only the summary, no preamble. Maximum 300 words.`

// NeedsSummary determines whether a session requires a new or refreshed
// conversation summary. Returns true when:
//   - The session has more messages than summaryMessageThreshold, AND
//   - Either no summary exists yet, or enough new messages have accumulated
//     since the last summary (controlled by summaryRefreshInterval).
func NeedsSummary(session *chatstore.ChatSession) bool {
	if session == nil {
		return false
	}
	if session.MessageCount <= summaryMessageThreshold {
		return false
	}
	// No summary yet — generate one.
	if session.Summary == "" {
		return true
	}
	// Summary exists — check if enough new messages have accumulated.
	return session.MessageCount-session.SummaryMessageCount >= summaryRefreshInterval
}

// OlderMessageCount returns the number of messages that fall outside the
// recent-message window (and therefore should be summarized). Returns 0 if
// the total count fits within the window.
func OlderMessageCount(totalMessages int) int {
	if totalMessages <= recentMessageWindow {
		return 0
	}
	return totalMessages - recentMessageWindow
}

// GenerateSummary calls the AI provider with a fast-tier model to produce
// a concise summary of the provided messages. The summary captures key
// facts, preferences, and decisions for context continuity.
func (s *Service) GenerateSummary(ctx context.Context, messages []*chatstore.ChatMessage) (string, error) {
	if len(messages) == 0 {
		return "", nil
	}

	// Build a conversation transcript from the messages.
	var transcript strings.Builder
	for _, msg := range messages {
		if msg.Content == "" {
			continue
		}
		switch msg.Role {
		case "user":
			transcript.WriteString("User: ")
		case "assistant":
			transcript.WriteString("Assistant: ")
		default:
			continue
		}
		transcript.WriteString(msg.Content)
		transcript.WriteByte('\n')
	}

	if transcript.Len() == 0 {
		return "", nil
	}

	// Build the summarization request using the fast tier.
	req := &ai.ChatRequest{
		SystemPrompt: summarizationPrompt,
		Messages: []ai.Message{
			{
				Role:    "user",
				Content: transcript.String(),
			},
		},
		MaxTokens:   summaryMaxTokens,
		Temperature: 0.3, // Low temperature for factual summary
		ModelTier:   ai.ModelTierFast,
	}

	eventCh, err := s.provider.ChatStream(ctx, req)
	if err != nil {
		return "", fmt.Errorf("start summarization stream: %w", err)
	}

	var summary strings.Builder
	for event := range eventCh {
		switch event.Type {
		case ai.EventTextDelta:
			summary.WriteString(event.Text)
		case ai.EventError:
			return "", fmt.Errorf("summarization error: %w", event.Error)
		case ai.EventDone:
			// done
		}
	}

	result := strings.TrimSpace(summary.String())
	if result == "" {
		return "", fmt.Errorf("summarization produced empty result")
	}

	return result, nil
}

// MaybeRefreshSummary checks if the session needs a summary update and, if so,
// loads the older messages, generates a summary, and persists it. This is
// designed to be called early in SendMessage, before the AI request is built.
//
// The function is best-effort: summarization failures are logged but do not
// block the chat flow. The conversation will work without a summary — it just
// may lose older context.
func (s *Service) MaybeRefreshSummary(ctx context.Context, userID, tripID, sessionID string, session *chatstore.ChatSession) {
	if !NeedsSummary(session) {
		return
	}

	olderCount := OlderMessageCount(session.MessageCount)
	if olderCount == 0 {
		return
	}

	slog.Info("conversation summary: generating",
		"session_id", sessionID,
		"message_count", session.MessageCount,
		"summary_message_count", session.SummaryMessageCount,
		"older_messages", olderCount,
	)

	// Load the older messages (those outside the recent window).
	olderMessages, err := s.chatStore.GetOldestMessages(ctx, userID, tripID, sessionID, olderCount)
	if err != nil {
		slog.Error("conversation summary: failed to load older messages",
			"session_id", sessionID,
			"error", err,
		)
		return
	}

	if len(olderMessages) == 0 {
		slog.Warn("conversation summary: no older messages found despite count indicating otherwise",
			"session_id", sessionID,
			"expected_older", olderCount,
		)
		return
	}

	summary, err := s.GenerateSummary(ctx, olderMessages)
	if err != nil {
		slog.Error("conversation summary: generation failed",
			"session_id", sessionID,
			"error", err,
		)
		return
	}

	// Persist the summary and the current message count.
	if err := s.chatStore.UpdateSummary(ctx, userID, tripID, sessionID, summary, session.MessageCount); err != nil {
		slog.Error("conversation summary: failed to persist",
			"session_id", sessionID,
			"error", err,
		)
		return
	}

	// Update the in-memory session so the caller can use the summary
	// immediately without re-reading from Firestore.
	session.Summary = summary
	session.SummaryMessageCount = session.MessageCount

	slog.Info("conversation summary: saved",
		"session_id", sessionID,
		"summary_len", len(summary),
	)
}

// BuildSummaryContext returns the summary context string to prepend to the
// system prompt, or empty string if no summary is available.
func BuildSummaryContext(session *chatstore.ChatSession) string {
	if session == nil || session.Summary == "" {
		return ""
	}
	return fmt.Sprintf("PREVIOUS CONVERSATION SUMMARY (messages 1-%d):\n%s\n\nThe above summarizes earlier parts of this conversation. The user may have already shared preferences, made decisions, or answered questions covered in that summary. Do not re-ask questions that are already answered above.",
		session.SummaryMessageCount, session.Summary)
}
