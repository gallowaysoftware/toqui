# Agent Feedback: Alice (Solo Backpacker)

## Persona

Solo female backpacker, budget traveler, first-time Toqui user

## Journey Summary

Alice is a first-time user who wants to backpack through Vietnam for a month on a tight budget. Her journey tests the full trip lifecycle: discovery (selection mode), planning (two turns of route and budget advice), status transitions (planning -> active -> completed), and on-trip assistance (companion mode). This test exercises the create_trip tool, persona resolution, theme tagging, session continuity, and the data lifecycle hooks.

**Note:** This feedback is based on deep source code analysis of the entire backend codebase. The sandbox environment prevented live API execution. A test runner script (`feedback/alice-test-runner.py`) has been provided for live validation. All findings below are from code-level review and describe expected behavior, confirmed bugs, and architectural concerns.

## Step-by-Step Results

### 1. Selection Mode -- Trip Creation

- **Input**: `SendMessage(mode=CHAT_MODE_SELECTION, tripId="", content="I'm thinking about backpacking through Vietnam for a month. Street food, motorbikes, the whole deal")`
- **Expected AI Response**: Toqui (default persona) responds enthusiastically about Vietnam, acknowledges the backpacking/street-food vibe, and proactively calls `create_trip` since Alice expressed clear interest in a specific destination.
- **Expected Tool Calls**: `create_trip(title="Vietnam Backpacking", description="Month-long backpacking trip through Vietnam...")` -- the system prompt explicitly instructs the AI to create trips proactively when users express interest.
- **Trip Created**: Yes (expected). The trip starts in `TRIP_STATUS_PLANNING`. The `TripCreated` event is emitted on the stream after the `tool_result` event.
- **Session Handling**: A new session is created in Firestore under `users/{uid}/trips/_lobby/chatSessions/{sessionId}`. The `SessionCreated` event is the first event on the stream.
- **Theme Tagging**: After the trip is created via `CreateTripTool.Execute()`, `TagTripAsync` fires in the background. It should detect themes like "food", "adventure", and "budget", and set `destination_country` to "VN".
- **Selection Context**: `buildSelectionContext()` lists Alice's existing trips (none for a new user), so the system prompt says "The user has no existing trips yet. Help them get started!"
- **Verdict**: Should work correctly. The selection mode flow is well-implemented with the create_trip and select_trip tools properly injected.

### 2. Planning Mode -- Route Planning

- **Input**: `SendMessage(mode=CHAT_MODE_PLANNING, tripId="<trip_id_from_step_1>", content="What's the best route? I want to start in Hanoi and end in Ho Chi Minh City")`
- **Expected AI Response**: Toqui suggests a north-to-south route through Vietnam -- likely Hanoi -> Ninh Binh -> Ha Long Bay -> Hue -> Hoi An -> Da Nang -> Nha Trang -> Da Lat -> HCMC (or similar). Planning mode prompt adds "Suggest specific places, experiences, and a structured itinerary when you have enough context."
- **Expected Tool Calls**: Possibly `web_search` or `places_search` if the AI decides it needs current information, but likely none for general route advice.
- **Persona Resolution**: The handler calls `tripSvc.GetByID()` to get the trip, checks `DestinationCountry` and trip themes. If async tagging has completed (race condition), it resolves to a VN expert persona. If not, Toqui handles it.
- **Session Handling**: `sessionId` is empty in the request, so a new planning session is created under the trip path. This is a NEW session separate from the selection mode session.
- **Theme Retagging**: After this message completes, the chat handler checks if `len(tripThemes) == 0` and if so, re-tags using the recent messages. This is a nice safety net for the async race condition.
- **Verdict**: Should work. The race condition with persona resolution is mitigated by the retag-on-empty logic. However, Alice won't get the Vietnam expert persona on this first planning turn.

### 3. Planning Mode -- Budget Advice

- **Input**: `SendMessage(mode=CHAT_MODE_PLANNING, tripId="<trip_id>", sessionId="<session_from_step_2>", content="I'm on a tight budget, maybe $30/day. What kind of hostels should I look for?")`
- **Expected AI Response**: Budget-specific advice for Vietnam -- mentions hostel chains, dorm pricing ($5-10/night), guesthouses, street food economics, and practical money-saving tips.
- **Expected Tool Calls**: Possibly `web_search` for current hostel pricing, but general knowledge should suffice.
- **Session Continuity**: Uses the same `sessionId` from step 2. The chat history from step 2 is loaded from Firestore (up to 50 messages). The AI sees the full conversation context including the route planning question and response.
- **Persona Resolution**: By now, the async theme tagger from step 1 has very likely completed. If themes include "budget" and "food", and `destination_country` is "VN", the persona resolver composes an expert from the VN location profile + matched theme profiles. The VN profile flavor includes great context about Vietnamese culture, street food, and motorbike chaos.
- **Verdict**: Should work well. The multi-turn session continuity via Firestore is solid, and persona resolution should kick in by this turn.

### 4. Status Transition -- Start Traveling

- **Input**: `UpdateTrip(id="<trip_id>", status=TRIP_STATUS_ACTIVE)`
- **Expected API Response**: HTTP 200 with the updated trip object showing `status: TRIP_STATUS_ACTIVE`.
- **BUG FOUND**: The `UpdateTrip` SQL query (`UPDATE trips SET title = $3, description = $4, status = $5, start_date = $6, end_date = $7, updated_at = NOW() WHERE id = $1 AND user_id = $2 RETURNING *`) overwrites ALL fields. If the `UpdateTripRequest` only sets `id` and `status`, the handler passes empty strings for `title` and `description`, and nil for dates. This means:
  - `title` gets overwritten to `""` (empty string)
  - `description` gets overwritten to NULL (empty pgtype.Text)
  - `start_date` and `end_date` get overwritten to NULL
  - Only `status` is correctly set to "active"
  - **This is a data-loss bug.** The trip title Alice set during creation ("Vietnam Backpacking" or similar) is destroyed.
- **Workaround**: The client must re-send all existing fields when updating status. But this is fragile and unexpected.
- **Verdict**: BUG -- UpdateTrip should use partial updates (COALESCE or field masks) to only update fields that are explicitly set.

### 5. Companion Mode -- On Arrival

- **Input**: `SendMessage(mode=CHAT_MODE_COMPANION, tripId="<trip_id>", content="I just arrived at Hanoi airport. Where should I go first?")`
- **Expected AI Response**: Concise, actionable advice for getting from Noi Bai airport to Hanoi old quarter -- mention Grab app, airport bus 86, taxi scam warnings, and a suggestion for first stop (e.g., get pho, check into hostel area around Ma May street).
- **Companion Mode Prompt**: System prompt adds "Be concise and actionable. Prioritize immediate, practical information."
- **Persona Resolution**: Same as step 3. If themes and destination_country are set, the expert persona is used.
- **Session Handling**: New session created (no sessionId in request). This is a separate companion mode session.
- **Location Context**: No `user_location` sent in this request, so no GPS-based recommendations. If it were sent, coordinates would be injected into the system prompt but never stored (privacy-preserving).
- **Note**: If the step 4 bug wiped the trip title, `GetByID` still returns the trip but with empty title. The persona resolution checks `DestinationCountry` (which was set by the theme tagger directly, not via UpdateTrip), so persona resolution should still work.
- **Verdict**: Should work correctly. Companion mode is well-designed for on-trip conciseness.

### 6. Status Transition -- Complete Trip

- **Input**: `UpdateTrip(id="<trip_id>", status=TRIP_STATUS_COMPLETED)`
- **Expected API Response**: HTTP 200 with trip showing `status: TRIP_STATUS_COMPLETED`.
- **Same data-loss bug as step 4**: Title, description, dates all overwritten to empty.
- **Data Lifecycle**: When status is "completed", the handler calls `lifecycleSvc.SetChatTTLAsync(userID, tripID, 90)` -- sets a 90-day TTL on chat data in Firestore. This is a good privacy/retention feature.
- **Verdict**: BUG (same as step 4) -- but the lifecycle trigger works correctly.

## What Went Well

- **Selection mode tool injection** is elegantly implemented. The `create_trip` and `select_trip` tools are only available in selection mode, cleanly scoped via `ExtraTools`.
- **Proactive trip creation** -- the system prompt instructs the AI to create trips when users express interest, not waiting for an explicit "create a trip" command. This is great UX.
- **Persona composition system** is sophisticated and well-thought-out. The location + theme profile composition, AI-generated identities with template fallback, and consistent caching via composite keys is production-quality.
- **Vietnam location profile** exists with culturally accurate flavor text covering street food, motorbike culture, and the north-south contrast.
- **Privacy-preserving location handling** -- companion mode GPS coordinates are injected into the AI prompt but never persisted to chat history or Firestore. Explicitly documented in code comments.
- **Theme retagging safety net** -- if async tagging hasn't completed by the first planning message, the chat handler re-tags using the recent conversation. Clever race condition mitigation.
- **Data lifecycle on trip completion** -- 90-day TTL on chat data when a trip is completed is a thoughtful retention policy.
- **Selection context building** -- existing trips are listed in the system prompt so the AI can help select them. Good for returning users.
- **Session isolation** -- selection mode chats go to `_lobby` path, planning/companion get their own sessions per trip.

## What Went Poorly

- **UpdateTrip is destructive** -- sending a status-only update wipes title, description, and dates. This is the most critical bug found.
- **Persona resolution race condition** -- on the first planning message after trip creation, async theme tagging may not have completed. The user gets Toqui instead of a Vietnam expert. The retag-on-empty safety net helps for the second message, but the first planning turn loses personalization.
- **No destination_country on create_trip tool** -- the `create_trip` tool only accepts title and description. The destination country is only set by the async theme tagger. This means there's always at least one turn of delay before persona resolution can use the country code.
- **Chat mode not stored on session creation** -- the `ChatSession` proto has a `mode` field and sessions are created with a mode, but switching modes (e.g., planning to companion) creates a new session. This is correct behavior but means conversation context doesn't carry across mode changes.

## Bugs Found

- **CRITICAL: UpdateTrip overwrites all fields** -- `UpdateTrip` SQL (`trips.sql:27-29`) sets title, description, status, start_date, end_date unconditionally. If the API request only sets `id` and `status`, other fields are overwritten to empty/null. This causes data loss on status transitions. The fix should use COALESCE:
  ```sql
  UPDATE trips SET
    title = COALESCE(NULLIF($3, ''), title),
    description = COALESCE($4, description),
    status = COALESCE(NULLIF($5, ''), status),
    start_date = COALESCE($6, start_date),
    end_date = COALESCE($7, end_date),
    updated_at = NOW()
  WHERE id = $1 AND user_id = $2
  RETURNING *;
  ```
  Or use proto field masks to determine which fields were explicitly set.
- **MINOR: tripStatusToString defaults to "planning" for UNSPECIFIED** -- `tripStatusToString(0)` returns "planning" (handlers/trip.go:239). If UpdateTrip is called with status 0 (not set), it would change status to "planning". Should return empty string for UNSPECIFIED and skip the status update.

## Suggestions

- **Use COALESCE or field masks in UpdateTrip** to prevent data loss on partial updates. This is the highest-priority fix.
- **Add destination_country to create_trip tool** -- let the AI set the ISO country code at creation time. This eliminates the race condition where the first planning message can't resolve an expert persona.
- **Consider a "trip context" cache** -- after theme tagging completes, store the resolved persona ID on the trip so subsequent requests don't need to re-resolve. This avoids the composer lookup on every chat message.
- **Add a web_search tool for budget/hostel queries** -- in the current tool registry, only places and web_search tools are available. If API keys aren't configured, the AI falls back to training data. For a budget travel app, real-time pricing is valuable.
- **Stream event ordering documentation** -- document the expected order of events (SessionCreated -> TextDelta* -> ToolCall -> ToolResult -> TripCreated -> TextDelta* -> MessageComplete). The client needs to understand this to build the UI correctly.
- **Rate limiting on chat** -- the interceptor limits to 10 requests per 60 seconds. For a chat interface with streaming, this is fine, but during rapid planning conversations it could be hit. Consider per-mode rate limits (selection: lower, planning: higher).
- **Integration test for the full cycle** -- add an integration test that exercises the selection -> planning -> companion flow end-to-end, similar to what this feedback documents.

## Test Runner

A Python test runner script has been provided at `feedback/alice-test-runner.py`. Run it with:

```bash
cd /Users/pequalsnp/src/github.com/gallowaysoftware/toqui-backend
python3 feedback/alice-test-runner.py
```

Requirements:

- Backend running at `localhost:8090`
- Firestore emulator running
- PostgreSQL with migrations applied
- AI provider configured (ANTHROPIC_API_KEY or OPENAI_API_KEY)

The script generates a JWT, runs all 6 steps sequentially, captures raw events, and writes results to `feedback/alice-test-results.json`.
