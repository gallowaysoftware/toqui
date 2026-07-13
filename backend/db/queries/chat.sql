-- name: CreateChatSession :one
INSERT INTO chat_sessions (id, user_id, trip_id, mode, created_at, last_message_at)
VALUES ($1, $2, $3, $4, $5, $5)
RETURNING *;

-- name: GetChatSession :one
SELECT * FROM chat_sessions
WHERE id = $1 AND user_id = $2 AND trip_id = $3;

-- name: DeleteChatSession :exec
DELETE FROM chat_sessions
WHERE id = $1 AND user_id = $2 AND trip_id = $3;

-- name: ListChatSessions :many
SELECT * FROM chat_sessions
WHERE user_id = $1 AND trip_id = $2
ORDER BY last_message_at DESC
LIMIT $3;

-- UpsertChatSessionForMessage records message activity on a session,
-- creating the session implicitly if the client passed an unknown session ID
-- (mirrors the Firestore MergeAll upsert this port replaced). The WHERE guard
-- refuses to touch a session that exists under a different user or trip —
-- the caller treats the resulting no-row as "session not found".
-- name: UpsertChatSessionForMessage :one
INSERT INTO chat_sessions (id, user_id, trip_id, mode, created_at, last_message_at, message_count)
VALUES (
    sqlc.arg(id),
    sqlc.arg(user_id),
    sqlc.arg(trip_id),
    sqlc.arg(mode),
    sqlc.arg(last_message_at),
    sqlc.arg(last_message_at),
    CASE WHEN sqlc.arg(count_message)::bool THEN 1 ELSE 0 END
)
ON CONFLICT (id) DO UPDATE SET
    last_message_at = EXCLUDED.last_message_at,
    message_count = chat_sessions.message_count
        + CASE WHEN sqlc.arg(count_message)::bool THEN 1 ELSE 0 END,
    mode = COALESCE(NULLIF(sqlc.arg(mode)::text, ''), chat_sessions.mode)
WHERE chat_sessions.user_id = EXCLUDED.user_id
  AND chat_sessions.trip_id = EXCLUDED.trip_id
RETURNING id;

-- name: InsertChatMessage :exec
INSERT INTO chat_messages (id, session_id, role, content, metadata, tool_calls, tool_results, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: DeleteChatMessage :exec
DELETE FROM chat_messages m
USING chat_sessions s
WHERE m.id = $1 AND m.session_id = $2
  AND s.id = m.session_id AND s.user_id = $3 AND s.trip_id = $4;

-- GetNewestChatMessages returns the newest N messages in reverse-chronological
-- order; the store reverses them to chronological. The join enforces that the
-- session belongs to the given user and trip.
-- name: GetNewestChatMessages :many
SELECT m.* FROM chat_messages m
JOIN chat_sessions s ON s.id = m.session_id
WHERE m.session_id = $1 AND s.user_id = $2 AND s.trip_id = $3
ORDER BY m.seq DESC
LIMIT $4;

-- name: GetOldestChatMessages :many
SELECT m.* FROM chat_messages m
JOIN chat_sessions s ON s.id = m.session_id
WHERE m.session_id = $1 AND s.user_id = $2 AND s.trip_id = $3
ORDER BY m.seq ASC
LIMIT $4;

-- name: UpdateChatSessionSummary :execrows
UPDATE chat_sessions
SET summary = $4, summary_message_count = $5
WHERE id = $1 AND user_id = $2 AND trip_id = $3;

-- name: MoveChatSessionToTrip :execrows
UPDATE chat_sessions
SET trip_id = sqlc.arg(to_trip_id)
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id) AND trip_id = sqlc.arg(from_trip_id);

-- SetChatTTLForTrip stamps the retention deadline on ALL of a trip's
-- sessions, across every participant — collaborators' chat is trip data
-- and follows the trip's retention, not just the owner's sessions.
-- name: SetChatTTLForTrip :exec
UPDATE chat_sessions
SET expire_at = $2
WHERE trip_id = $1;

-- StampMissingChatTTLForEndedTrips is the hourly safety net: any session
-- belonging to a completed/archived trip that has no expire_at yet gets
-- one. Catches sessions created after the completion stamp, collaborator
-- sessions, and stamps that failed at completion time.
-- name: StampMissingChatTTLForEndedTrips :execrows
UPDATE chat_sessions cs
SET expire_at = NOW() + make_interval(days => sqlc.arg(retention_days)::int)
FROM trips t
WHERE t.id::text = cs.trip_id
  AND t.status IN ('completed', 'archived')
  AND cs.expire_at IS NULL;

-- DeleteChatForTrip removes ALL chat for a trip, across every participant.
-- Used when the trip itself is deleted.
-- name: DeleteChatForTrip :exec
DELETE FROM chat_sessions
WHERE trip_id = $1;

-- PurgeExpiredChatSessions enforces chat retention: sessions past their
-- expire_at are deleted (messages cascade). Run periodically by the
-- lifecycle background jobs.
-- name: PurgeExpiredChatSessions :execrows
DELETE FROM chat_sessions
WHERE expire_at IS NOT NULL AND expire_at < NOW();

-- PurgeStaleLobbyChatSessions deletes selection-mode ("_lobby") sessions
-- idle longer than the retention window. Lobby sessions never belong to a
-- trip, so they never get an expire_at stamp at trip completion — without
-- this they would live until account deletion.
-- name: PurgeStaleLobbyChatSessions :execrows
DELETE FROM chat_sessions
WHERE trip_id = '_lobby'
  AND last_message_at < NOW() - make_interval(days => sqlc.arg(retention_days)::int);
