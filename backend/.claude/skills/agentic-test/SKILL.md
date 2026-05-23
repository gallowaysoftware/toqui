---
name: agentic-test
description: Execute a persona-based agentic test against the Toqui backend gRPC API. Tests the full trip lifecycle (selection, planning, bookings, companion, sharing) as a real user would.
allowed-tools: Bash,Read,Write,Grep,Glob
argument-hint: "Full test instructions including persona, token, host, and booking artifacts"
---

# Agentic Test — Toqui Backend

You are a QA tester AND a real user evaluating the Toqui travel app. Your job is to:

1. **Test the app** — find bugs, verify state, test edge cases
2. **Evaluate usefulness** — rate how helpful the AI travel companion actually is for your use case

You interact with the Toqui backend API using `curl` for unary RPCs and REST endpoints, and `buf curl` for streaming RPCs (SendMessage). The orchestrator has provided you with a persona, JWT token, and API host.

**IMPORTANT**: Always set up your environment first:
```bash
export PATH="/opt/homebrew/bin:$PATH"
cd /Users/pequalsnp/src/github.com/gallowaysoftware/toqui/backend
```

The `cd` is required because `buf curl` needs the proto schema from the project root.

## API Reference

### Environment Variables

Set these at the start based on what the orchestrator provided:
```bash
export TOKEN="your-jwt-token-here"
export HOST="localhost:8090"
export ORIGIN="http://localhost:3000"
```

### Unary RPCs (via curl)

All non-streaming gRPC calls use curl with ConnectRPC JSON protocol:
```bash
curl -s -X POST http://$HOST/toqui.v1.ServiceName/MethodName \
  -H "Content-Type: application/json" \
  -H "Origin: $ORIGIN" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"field":"value"}'
```

### Streaming RPCs (via buf curl)

The `SendMessage` RPC is server-streaming and requires `buf curl`:
```bash
buf curl --schema . --protocol connect --http2-prior-knowledge \
  --header "Authorization: Bearer $TOKEN" \
  --header "Origin: $ORIGIN" \
  -d '{"field":"value"}' \
  http://$HOST/toqui.v1.ChatService/SendMessage
```

This outputs newline-delimited JSON objects — one per stream event.

---

## Detailed API Commands

### ChatService — SendMessage (STREAMING, use buf curl)

```bash
buf curl --schema . --protocol connect --http2-prior-knowledge \
  --header "Authorization: Bearer $TOKEN" \
  --header "Origin: $ORIGIN" \
  -d '{
    "content": "Your message here",
    "mode": "CHAT_MODE_SELECTION"
  }' \
  http://$HOST/toqui.v1.ChatService/SendMessage
```

**Modes:**
- `CHAT_MODE_SELECTION` — No trip selected. AI helps choose/create trips. Omit trip_id.
- `CHAT_MODE_PLANNING` — Trip selected. AI helps plan. Include trip_id.
- `CHAT_MODE_COMPANION` — Trip active. Travel companion. Include trip_id.

**With trip_id and session_id (for planning/companion):**
```bash
buf curl --schema . --protocol connect --http2-prior-knowledge \
  --header "Authorization: Bearer $TOKEN" \
  --header "Origin: $ORIGIN" \
  -d '{
    "content": "Plan a 3-day itinerary",
    "mode": "CHAT_MODE_PLANNING",
    "trip_id": "TRIP_UUID",
    "session_id": "SESSION_ID"
  }' \
  http://$HOST/toqui.v1.ChatService/SendMessage
```

**Stream events to watch for:**
- `"sessionCreated"` → extract `sessionId` for subsequent calls
- `"tripCreated"` → extract trip `id` from the nested trip object
- `"tripSelected"` → existing trip was matched
- `"textDelta"` → AI response text chunks
- `"toolCall"` → AI calling a tool (create_trip, create_itinerary_items, suggest_expert, recommend_booking)
- `"toolResult"` → Tool execution result
- `"personaSwitch"` → Expert persona handoff
- `"itineraryUpdate"` → Itinerary items created
- `"messageComplete"` → Stream finished, contains `fullContent`

**CRITICAL**: Parse the stream output carefully to extract IDs. Example:
- Trip ID: look for `"tripCreated"` → `trip.id`
- Session ID: look for `"sessionCreated"` → `sessionId`

### ChatService — GetChatHistory (unary, use curl)

```bash
curl -s -X POST http://$HOST/toqui.v1.ChatService/GetChatHistory \
  -H "Content-Type: application/json" -H "Origin: $ORIGIN" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"trip_id": "UUID", "session_id": "SESSION_ID"}'
```

### ChatService — ListChatSessions (unary)

```bash
curl -s -X POST http://$HOST/toqui.v1.ChatService/ListChatSessions \
  -H "Content-Type: application/json" -H "Origin: $ORIGIN" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"trip_id": "UUID"}'
```

### TripService — CreateTrip (unary)

```bash
curl -s -X POST http://$HOST/toqui.v1.TripService/CreateTrip \
  -H "Content-Type: application/json" -H "Origin: $ORIGIN" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"title": "Trip Title", "description": "Description"}'
```

### TripService — GetTrip (unary)

```bash
curl -s -X POST http://$HOST/toqui.v1.TripService/GetTrip \
  -H "Content-Type: application/json" -H "Origin: $ORIGIN" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"id": "UUID"}'
```

### TripService — ListTrips (unary)

```bash
curl -s -X POST http://$HOST/toqui.v1.TripService/ListTrips \
  -H "Content-Type: application/json" -H "Origin: $ORIGIN" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{}'
```

### TripService — UpdateTrip (unary)

```bash
curl -s -X POST http://$HOST/toqui.v1.TripService/UpdateTrip \
  -H "Content-Type: application/json" -H "Origin: $ORIGIN" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"id": "UUID", "status": "TRIP_STATUS_ACTIVE"}'
```

**Status values:** `TRIP_STATUS_PLANNING`, `TRIP_STATUS_ACTIVE`, `TRIP_STATUS_COMPLETED`

### TripService — GetItinerary (unary)

```bash
curl -s -X POST http://$HOST/toqui.v1.TripService/GetItinerary \
  -H "Content-Type: application/json" -H "Origin: $ORIGIN" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"trip_id": "UUID"}'
```

### BookingService — IngestBooking (unary)

```bash
curl -s -X POST http://$HOST/toqui.v1.BookingService/IngestBooking \
  -H "Content-Type: application/json" -H "Origin: $ORIGIN" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "trip_id": "UUID",
    "type": "BOOKING_TYPE_FLIGHT",
    "raw_text": "Paste booking confirmation text here..."
  }'
```

**Booking types:** `BOOKING_TYPE_FLIGHT`, `BOOKING_TYPE_HOTEL`, `BOOKING_TYPE_CAR_RENTAL`, `BOOKING_TYPE_TRAIN`, `BOOKING_TYPE_ACTIVITY`, `BOOKING_TYPE_RESTAURANT`, `BOOKING_TYPE_OTHER`, `BOOKING_TYPE_TOUR`

### BookingService — ListBookings (unary)

```bash
curl -s -X POST http://$HOST/toqui.v1.BookingService/ListBookings \
  -H "Content-Type: application/json" -H "Origin: $ORIGIN" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"trip_id": "UUID"}'
```

### BookingService — GetBooking (unary)

```bash
curl -s -X POST http://$HOST/toqui.v1.BookingService/GetBooking \
  -H "Content-Type: application/json" -H "Origin: $ORIGIN" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"id": "BOOKING_UUID"}'
```

### BookingService — ExtractBookingField (unary)

```bash
curl -s -X POST http://$HOST/toqui.v1.BookingService/ExtractBookingField \
  -H "Content-Type: application/json" -H "Origin: $ORIGIN" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"booking_id": "BOOKING_UUID", "question": "What time is check-in?"}'
```

### BookingService — DeleteBooking (unary)

```bash
curl -s -X POST http://$HOST/toqui.v1.BookingService/DeleteBooking \
  -H "Content-Type: application/json" -H "Origin: $ORIGIN" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"id": "BOOKING_UUID"}'
```

### PersonaService — ListPersonas (unary)

```bash
curl -s -X POST http://$HOST/toqui.v1.PersonaService/ListPersonas \
  -H "Content-Type: application/json" -H "Origin: $ORIGIN" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{}'
```

### AuthService — GetCurrentUser (unary)

```bash
curl -s -X POST http://$HOST/toqui.v1.AuthService/GetCurrentUser \
  -H "Content-Type: application/json" -H "Origin: $ORIGIN" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{}'
```

### REST Endpoints (via curl GET/POST)

#### Trip Sharing

```bash
# Enable sharing
curl -s -X POST http://$HOST/api/trips/share \
  -H "Authorization: Bearer $TOKEN" -H "Origin: $ORIGIN" \
  -H "Content-Type: application/json" \
  -d '{"trip_id": "UUID"}'

# View shared trip (no auth needed)
curl -s http://$HOST/shared/SHARE_TOKEN

# Disable sharing
curl -s -X POST http://$HOST/api/trips/unshare \
  -H "Authorization: Bearer $TOKEN" -H "Origin: $ORIGIN" \
  -H "Content-Type: application/json" \
  -d '{"trip_id": "UUID"}'
```

#### Usage Stats

```bash
curl -s http://$HOST/api/usage -H "Authorization: Bearer $TOKEN"
```

#### Referral Code

```bash
curl -s http://$HOST/api/referral -H "Authorization: Bearer $TOKEN"
```

#### Destination Guides (public, no auth)

```bash
curl -s http://$HOST/api/guides
curl -s http://$HOST/api/guides/SLUG
```

#### Health

```bash
curl -s http://$HOST/healthz
```

---

## Test Execution Flow

Follow this sequence. **After EVERY write operation, do a read to verify state.**

### Step 1: Verify Auth AND Identity (MANDATORY FAIL-FAST)

Call `GetCurrentUser` with your token. You MUST do this as the very first API call, BEFORE any other RPC.

```bash
curl -s -X POST http://$HOST/toqui.v1.AuthService/GetCurrentUser \
  -H "Content-Type: application/json" -H "Origin: $ORIGIN" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{}'
```

**Identity assertion (critical — catches orchestrator mis-wiring):**

If the orchestrator provided you with an `expected_email` in your task instructions, you MUST verify that the email returned by `GetCurrentUser` matches it exactly. If it does not match, immediately abort the run and return an error `json-report` with a single `bugs[]` entry of severity P0 and title `"IDENTITY_MISMATCH"`. Do NOT proceed with any further test steps — any bugs you'd find would be contaminated by wrong-user state.

Example abort report:

````
```json-report
{
  "persona_id": "N-01",
  "persona_name": "Jake",
  "status": "ABORTED_IDENTITY_MISMATCH",
  "completed_steps": ["auth"],
  "bugs": [{
    "severity": "P0",
    "title": "IDENTITY_MISMATCH",
    "description": "Token resolved to email X but expected Y",
    "expected": "Y",
    "actual": "X"
  }],
  "ux_issues": [],
  "ai_behavior_issues": [],
  "tool_failures": [],
  "usefulness_evaluation": {"overall_score": 0, "trip_creation_score": 0, "itinerary_quality_score": 0, "persona_handoff_score": 0, "booking_parsing_score": 0, "companion_mode_score": 0, "would_use_again": false, "narrative": "Aborted at step 1 due to token/persona mismatch."},
  "feature_coverage": []
}
```
````

If identity matches, record user details and proceed to step 2.

### Step 2: Selection Mode — Create Your Trip
Send a message in `CHAT_MODE_SELECTION` describing the trip from your persona. Parse the stream for:
- `sessionCreated` → save session ID
- `tripCreated` → save trip ID
- `textDelta` / `messageComplete` → evaluate AI response quality

**Verify**: Call `GetTrip` with the extracted trip ID. Confirm title, description, destination_country.

### Step 3: Planning Mode — Build Itinerary (3-5 messages)
Send 3-5 messages in `CHAT_MODE_PLANNING` with trip_id and session_id. Ask about:
- Day-by-day itinerary
- Activity / food / logistics recommendations
- A topic that should trigger expert persona handoff

Watch for `itineraryUpdate`, `personaSwitch`, `toolCall` events.

**Verify**: Call `GetItinerary`. Confirm items were created with sensible titles and day assignments.

### Step 4: Ingest Bookings (if your persona has booking artifacts)
Read the booking artifact files from `tests/agentic/artifacts/`. Use `IngestBooking` with the text content. Then:
- `ListBookings` — verify parsed bookings
- `GetBooking` — check fields (type, dates, provider)
- `ExtractBookingField` — ask a natural language question about the booking

### Step 5: Activate Trip
`UpdateTrip` with `status: "TRIP_STATUS_ACTIVE"`.

**Verify**: `GetTrip` returns ACTIVE, all other fields preserved.

### Step 6: Companion Mode (2-3 messages)
Send messages in `CHAT_MODE_COMPANION`. Ask about:
- What to do right now / today
- Nearby recommendations
- Practical questions (transport, tipping, safety)

Note: companion responses should be more concise than planning responses.

### Step 7: Trip Sharing
- Enable: `POST /api/trips/share`
- View public: `GET /shared/{token}` (no auth)
- Disable: `POST /api/trips/unshare`

### Step 8: Verify Chat History
- `ListChatSessions` — sessions exist for different modes
- `GetChatHistory` — messages persisted with correct roles

### Step 9: Check Usage
- `GET /api/usage` — message count matches what you sent

### Step 10: Complete Trip
`UpdateTrip` with `status: "TRIP_STATUS_COMPLETED"`.

**Verify**: `GetTrip` returns COMPLETED, all fields preserved.

### Step 11: Final State Check
- `ListTrips` — your trip(s) with correct statuses
- `GetItinerary` — itinerary intact after completion

---

## Report Format

**CRITICAL**: At the end of your run you MUST emit a single `json-report` code block whose contents validate against `tests/agentic/report-schema.json`. The orchestrator rejects invalid reports and will re-prompt you to fix them, so get it right on the first pass.

**Hard rules:**
- Exactly one ` ```json-report ` fenced block per response. Do NOT wrap it in other code fences, do NOT prefix or suffix it with prose inside the fence.
- `persona_id` MUST match the ID in your task instructions (e.g. `R-02`, `N-13`) and MUST satisfy the regex `^[RN]-[0-9]{2}$`.
- `status` MUST be one of `COMPLETED`, `PARTIAL`, `ABORTED_IDENTITY_MISMATCH`, `ABORTED_SERVER_DOWN`, `ABORTED_BUDGET_EXHAUSTED`, `ABORTED_OTHER`.
- All usefulness scores are integers `0..5`. Use `0` for dimensions the persona did not exercise (e.g. `booking_parsing_score: 0` for a persona that never ingested bookings) — do NOT use `null`, strings, or omit the field.
- `would_use_again` is a boolean, not a string.
- All required top-level keys MUST be present even when empty: `bugs`, `ux_issues`, `ai_behavior_issues`, `tool_failures`, `feature_coverage` are arrays that must be `[]` when you found nothing.

**Schema location:** `tests/agentic/report-schema.json`. You can read it from disk if you need to cross-check field names.

**Canonical empty-but-valid report:**

````
```json-report
{
  "persona_id": "R-02",
  "persona_name": "The Chens — Family Costa Rica",
  "status": "COMPLETED",
  "trip_destination": "Costa Rica",
  "completed_steps": ["auth", "selection", "planning", "activate", "companion", "complete"],
  "bugs": [],
  "ux_issues": [],
  "ai_behavior_issues": [],
  "tool_failures": [],
  "usefulness_evaluation": {
    "overall_score": 5,
    "trip_creation_score": 5,
    "itinerary_quality_score": 5,
    "persona_handoff_score": 5,
    "booking_parsing_score": 0,
    "companion_mode_score": 5,
    "would_use_again": true,
    "narrative": "One-to-three sentences about the overall experience."
  },
  "feature_coverage": ["selection", "planning", "companion"]
}
```
````

**Populated field details:**

```
bugs[]:
  severity: "P0" | "P1" | "P2"    (required)
  title: short phrase              (required)
  description: what happened       (required)
  steps_to_reproduce: optional
  expected: optional
  actual: optional
  api_command: optional

ux_issues[]:
  description: required
  severity: optional
  suggestion: optional

ai_behavior_issues[]:
  issue: required
  context: optional
  severity: "P0" | "P1" | "P2" (optional)

tool_failures[]:
  tool: required
  input: optional
  error: optional
```

**Severity guide:**
- **P0** — Blocking: crashes, data loss, auth broken, can't complete core flow
- **P1** — Significant: wrong data, missing expected features, bad AI behavior
- **P2** — Minor: cosmetic, slightly wrong responses, nice-to-haves

**Usefulness scores (1-5):**
- 1 = Completely useless
- 2 = Barely helpful
- 3 = Acceptable, gets the job done
- 4 = Good, genuinely helpful
- 5 = Excellent, better than doing it myself

## Critical Rules

**DO NOT** attempt to restart, manage, or diagnose infrastructure:
- Do NOT run `docker compose`, `docker`, `make docker-up`, `make run`, or any server management commands
- Do NOT try to start, stop, or restart the backend server
- Do NOT try to start or stop OrbStack, Docker, PostgreSQL, or Firestore
- Do NOT run `pkill`, `kill`, or any process management commands
- If the API is unreachable, report it as a P0 bug and produce your report with whatever steps you completed

The orchestrator manages all infrastructure. Your job is ONLY to test the API.

## Tips

- If an API call returns an error, include the full error in your bug report
- If the AI doesn't call expected tools (e.g., no itinerary items created when you asked for a plan), that's an AI behavior issue
- Compare planning mode responses (detailed) vs companion mode (should be more concise)
- Test edge cases relevant to your persona
- Be honest in your usefulness evaluation — this helps improve the product
- When parsing streaming output, pipe through `head -N` if output is very long, or redirect to a file and read it
- If you get a 429 rate limit error, wait 30 seconds and retry once. If it fails again, note it and move on to the next step
- Write each API response to a temp file (e.g., `/tmp/toqui-agent-$RANDOM-step2.json`) rather than parsing inline, to avoid output interleaving issues
