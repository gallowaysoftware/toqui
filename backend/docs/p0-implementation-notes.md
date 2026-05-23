# P0 Implementation Notes

Covers the OAuth callback handler, persona persistence, chat history RPCs, Claude streaming fix, and frontend event handling changes shipped on 2026-03-05.

---

## 1. OAuth Callback Handler

### Flow

```
Frontend login()
  -> GET /auth/google/login (plain HTTP on backend, not ConnectRPC)
  -> Backend generates CSRF state, sets `oauth_state` cookie, 307 redirects to Google
  -> Google prompts user, redirects to GOOGLE_REDIRECT_URI
  -> GET /auth/google/callback (backend)
  -> Backend validates state cookie, exchanges code for Google user info
  -> Upserts user via `UpsertUserByGoogleID` (sqlc)
  -> Generates JWT access + refresh tokens
  -> 307 redirects to frontend /auth/callback?access_token=...&refresh_token=...&user_id=...&email=...&name=...
```

### CSRF State Cookie

`HandleLogin` generates a 16-byte random hex string and sets it as an `oauth_state` cookie:

- `HttpOnly: true` -- not readable by JS
- `SameSite: Lax` -- sent on top-level navigations (which is exactly what the OAuth redirect is)
- `MaxAge: 300` -- 5-minute window to complete the flow
- `Path: /` -- available to the callback endpoint

`HandleCallback` reads the cookie back, compares it to the `state` query param Google returns, and clears the cookie immediately (MaxAge: -1). If mismatched, redirects to frontend with `?error=invalid_state`.

### Token Delivery

Tokens are passed as query parameters on the redirect to `{FRONTEND_URL}/auth/callback`. The frontend callback page reads them from the URL and stores them (e.g., in memory or secure storage). Parameters:

| Param           | Value                            |
| --------------- | -------------------------------- |
| `access_token`  | Short-lived JWT                  |
| `refresh_token` | Long-lived JWT for token renewal |
| `user_id`       | UUID string                      |
| `email`         | User email                       |
| `name`          | Display name (omitted if empty)  |

### Config Change

`GoogleRedirectURI` now defaults to `http://localhost:8090/auth/google/callback` (the backend's own callback endpoint). Previously it pointed at the frontend. This is required because Google sends the auth code to the backend, which needs to exchange it server-side.

### Relationship to ConnectRPC Auth RPCs

The existing `AuthService.GoogleLogin` and `AuthService.RefreshToken` ConnectRPC RPCs remain functional. They serve programmatic use cases (e.g., mobile clients or tests that already have a Google ID token). The new OAuth handler is the browser-friendly flow where the backend orchestrates the full redirect dance.

### Wiring (main.go)

```go
oauthHandler := handlers.NewOAuthHandler(authSvc, pool, cfg.FrontendURL)

mux.HandleFunc("/auth/google/login", oauthHandler.HandleLogin)
mux.HandleFunc("/auth/google/callback", oauthHandler.HandleCallback)
```

These are plain `http.HandleFunc` routes registered on the same mux as the ConnectRPC handlers, but outside the interceptor chain (no auth interceptor -- the user isn't authenticated yet).

---

## 2. SetDefaultPersona DB Persistence

### Migration

`db/migrations/20260305000005_user_default_persona.up.sql`:

```sql
ALTER TABLE users ADD COLUMN default_persona_id VARCHAR(256);
```

Nullable column -- `NULL` means "use system default (Toqui)".

### sqlc Queries

Added to `db/queries/users.sql`:

```sql
-- name: SetUserDefaultPersona :one
UPDATE users SET default_persona_id = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: GetUserDefaultPersona :one
SELECT default_persona_id FROM users WHERE id = $1;
```

Both are `:one` queries. `SetUserDefaultPersona` returns the full user row so the handler can confirm the write. `GetUserDefaultPersona` returns only the nullable `default_persona_id` column.

### Handler Change

`PersonaHandler` now accepts a `*pgxpool.Pool` in its constructor and creates a `dbgen.Queries` instance internally:

```go
func NewPersonaHandler(registry *persona.Registry, pool *pgxpool.Pool) *PersonaHandler
```

`SetDefaultPersona` validates the persona exists in the registry before persisting:

1. Extract `userID` from auth context
2. Call `registry.Get(personaId)` -- returns `CodeNotFound` if invalid
3. Call `queries.SetUserDefaultPersona` with `pgtype.Text{String: personaId, Valid: true}`
4. Return the validated persona proto

This means you cannot set a default persona ID that doesn't exist in the registry, preventing stale references.

### main.go Wiring Change

```go
personaHandler := handlers.NewPersonaHandler(personaRegistry, pool)
```

Previously `NewPersonaHandler` only took the registry. Now it also takes the pool for DB access.

---

## 3. GetChatHistory and ListChatSessions

### Proto Change

`GetChatHistoryRequest` now includes a `trip_id` field. This is required because Firestore documents are stored under a path that includes the trip:

```
users/{uid}/trips/{tripId}/chatSessions/{sessionId}/messages/{messageId}
```

Without `trip_id`, the backend cannot construct the Firestore document path to reach messages.

### Service Methods

**`ListSessions(ctx, userID, tripID, limit)`**

- Clamps limit to range [1, 100], defaults to 20
- Delegates to `chatStore.ListSessions`
- Returns `[]*chatstore.ChatSession` with fields: `ID`, `TripID`, `Mode`, `CreatedAt`, `LastMessageAt`

**`GetHistory(ctx, userID, tripID, sessionID, limit)`**

- Clamps limit to range [1, 100], defaults to 50
- Verifies session exists and belongs to user via `chatStore.GetSession` before reading messages
- Returns `[]*chatstore.ChatMessage` with fields: `ID`, `SessionID`, `Role`, `Content`, `Metadata`, `CreatedAt`

### Handler Implementations

**`ChatHandler.ListChatSessions`**

- Reads `page_size` from pagination proto, defaults to 20
- Converts `chatstore.ChatSession` to `toquiv1.ChatSession` proto
- Maps string mode ("planning"/"companion") to `ChatMode` enum via `chatModeFromString`

**`ChatHandler.GetChatHistory`**

- Reads `page_size` from pagination proto, defaults to 50
- Passes `req.Msg.TripId` and `req.Msg.SessionId` to service
- Converts messages to `toquiv1.ChatMessage` protos with `timestamppb.New` for timestamps

### Firestore Path Structure

```
users/{uid}/trips/{tripId}/chatSessions/{sessionId}          -- session document
users/{uid}/trips/{tripId}/chatSessions/{sessionId}/messages  -- subcollection
```

All chat operations (create session, add message, list sessions, get messages) require the full path components: `userID`, `tripID`, and `sessionID`. This structure supports the data lifecycle system where trip archival (90 days after completion) can efficiently purge all associated chat data by deleting the trip subtree.

---

## 4. Claude Tool Input Accumulation Fix

### The Bug

The Anthropic streaming API sends tool use across multiple SSE events:

1. `content_block_start` with `type: "tool_use"` -- contains `id` and `name` but empty `input`
2. One or more `content_block_delta` with `type: "input_json_delta"` -- contains `partial_json` fragments
3. `content_block_stop` -- signals the block is complete

The old code emitted an `EventToolCall` immediately on `content_block_start`, which meant the tool call had empty arguments. The `input_json_delta` events were being ignored entirely, so the accumulated arguments were never collected.

### The Fix

Introduced a `pendingTool` struct and a `toolBlocks` map keyed by content block index:

```go
type pendingTool struct {
    id   string
    name string
    args strings.Builder
}
```

The flow is now:

1. **`content_block_start`** (type `tool_use`): Create a `pendingTool` entry in `toolBlocks[index]` with the tool's `id` and `name`. Do NOT emit an event yet.

2. **`content_block_delta`** (type `input_json_delta`): Look up the pending tool by `event.Index`, append `partial_json` to its `args` builder.

3. **`content_block_stop`**: Look up the pending tool by `event.Index`, emit `EventToolCall` with the fully accumulated `args` string (defaulting to `"{}"` if empty), then delete the entry from the map.

### Integration with Chat Service

When `chat.Service.processEvents` receives an `EventToolCall`, it now gets a complete, parseable JSON arguments string. It:

1. Emits a `StreamEvent{Type: "tool_call"}` to the handler (which sends it to the frontend)
2. Calls `tools.Execute(ctx, name, arguments)` synchronously
3. Emits a `StreamEvent{Type: "tool_result"}` with the result or error

The frontend can now show tool activity indicators with correct tool names, and the tool execution receives valid input instead of empty JSON.

---

## 5. Frontend Streaming Event Handling

### New Events in useChat

The `useChat` hook (`src/lib/hooks/useChat.ts`) now handles these additional `SendMessageResponse` event cases:

| Case              | Action                                                                              |
| ----------------- | ----------------------------------------------------------------------------------- |
| `toolCall`        | Sets `toolActivity` state to `{ toolName, status: "calling" }`                      |
| `toolResult`      | Updates `toolActivity` to `{ toolName, status: "done" }`                            |
| `messageComplete` | Clears `toolActivity`, optionally overrides `fullText` with server-provided content |
| `error`           | Logs to console                                                                     |

The hook exports a new `toolActivity` state (`ToolActivity | null`) alongside existing `messages`, `streamingText`, `isStreaming`, and `activePersona`.

### ToolActivity Type

```typescript
export interface ToolActivity {
  toolName: string;
  status: "calling" | "done";
}
```

### ToolActivityIndicator Component

Defined inline in `ChatContainer.tsx`. Renders a small status line below the message list:

- **`calling`**: Shows a spinning animation + human-readable label (e.g., "Searching places...")
- **`done`**: Shows a green checkmark + label without ellipsis

Tool names are mapped to display labels via a `toolDisplayNames` lookup:

```typescript
const toolDisplayNames: Record<string, string> = {
  places_search: "Searching places",
  web_search: "Searching the web",
};
```

Unknown tools fall back to `"Using {toolName}"`.

### Rendering Flow in ChatContainer

```
ChatContainer
  -> useChat(tripId, mode) returns { ..., toolActivity }
  -> if (isStreaming && toolActivity):    render <ToolActivityIndicator>
  -> if (isStreaming && !streamingText && !toolActivity): render <TypingIndicator>
  -> if (streamingText):                  render streaming <MessageBubble>
```

The `ToolActivityIndicator` replaces the `TypingIndicator` while a tool is active, giving users visibility into what the AI is doing (searching, looking up data, etc.) rather than showing a generic typing animation.

### PersonaBar

Also in `ChatContainer.tsx`, the `PersonaBar` component renders above the message list when an `activePersona` is set. It shows the persona's initial (colored circle), name, and up to 3 specialties. This is driven by the `personaSwitch` streaming event handled in `useChat`.
