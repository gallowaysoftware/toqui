-- Chat persistence moves from Firestore to Postgres. Sessions are scoped to
-- (user, trip); trip_id is TEXT (not a FK) because selection-mode chats use
-- the '_lobby' sentinel before a trip exists.
CREATE TABLE chat_sessions (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    trip_id TEXT NOT NULL,
    mode TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_message_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    message_count INT NOT NULL DEFAULT 0,
    -- Retention: stamped when the trip completes; the lifecycle purge job
    -- deletes expired sessions (messages cascade). Replaces Firestore TTL.
    expire_at TIMESTAMPTZ,
    summary TEXT NOT NULL DEFAULT '',
    summary_message_count INT NOT NULL DEFAULT 0
);

CREATE INDEX idx_chat_sessions_user_trip ON chat_sessions (user_id, trip_id, last_message_at DESC);
CREATE INDEX idx_chat_sessions_expire ON chat_sessions (expire_at) WHERE expire_at IS NOT NULL;

CREATE TABLE chat_messages (
    id UUID PRIMARY KEY,
    session_id UUID NOT NULL REFERENCES chat_sessions(id) ON DELETE CASCADE,
    -- Monotonic insertion order. Ordering by created_at alone can tie at
    -- microsecond resolution during fast tool loops; seq breaks the tie.
    seq BIGINT GENERATED ALWAYS AS IDENTITY,
    role TEXT NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    metadata JSONB,
    tool_calls JSONB,
    tool_results JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_chat_messages_session_seq ON chat_messages (session_id, seq);
