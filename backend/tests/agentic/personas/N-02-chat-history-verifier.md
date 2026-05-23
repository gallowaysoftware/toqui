# Persona: Structural — Chat History Data Integrity

## Background

This is a structural test persona, not a realistic traveler. The goal is to verify that chat message persistence (Firestore), session management, and history retrieval work correctly across multiple chat modes and sessions. You will create messages with known content and then verify that every message is persisted accurately, sessions are tracked correctly, and no data is lost or corrupted. Adopt a neutral tone — the message content itself is not important, only that it round-trips correctly through the system.

## Your Trip

A generic trip to Portugal. The trip exists solely as a vehicle for generating chat sessions across different modes. Create it through the natural selection mode flow so that the selection mode chat session is captured.

## What to Test

### Phase 1: Selection Mode Session

1. Start a new chat in CHAT_MODE_SELECTION. Send exactly one message: "I want to plan a trip to Portugal for 10 days." The AI should create a trip via `create_trip`. Record the session_id from the response.
2. Verify the trip was created by calling GetTrip. Record the trip_id.

### Phase 2: Planning Mode Session

3. Start a new chat in CHAT_MODE_PLANNING with the trip_id. Send exactly two messages:
   - Message A: "I want to visit Lisbon and Porto. Can you suggest a rough itinerary?"
   - Message B: "Add those to my itinerary please."
4. Record the session_id for this planning session.

### Phase 3: Companion Mode Session

5. Start a new chat in CHAT_MODE_COMPANION with the trip_id. Send exactly one message:
   - Message C: "I just arrived in Lisbon. Where should I eat dinner tonight near Alfama?"
6. Record the session_id for this companion session.

### Phase 4: Session Listing Verification

7. Call ListChatSessions with the trip_id. Verify:
   - At least 3 sessions are returned (selection, planning, companion). There may be more if the AI created additional sessions internally.
   - Each session has a non-empty `id` field.
   - Each session has a `mode` field matching the expected chat mode.
   - Each session has `created_at` and `updated_at` timestamps that are valid (non-zero, `updated_at` >= `created_at`).
   - Sessions are ordered by creation time (earliest first or most recent first — document whichever ordering the API uses).

### Phase 5: Message History Verification

8. GetChatHistory for **selection session**: at least 2 messages, first user message contains "Portugal", roles alternate, no empty content.
9. GetChatHistory for **planning session**: at least 4 messages, user messages contain "Lisbon and Porto" and "itinerary", roles alternate, no empty content.
10. GetChatHistory for **companion session**: at least 2 messages, user message contains "Lisbon"/"Alfama", roles alternate, no empty content.

### Phase 6: Cross-Session Isolation

11. Verify that messages from the planning session do NOT appear in the companion session history, and vice versa. Sessions must be isolated — no message bleed across sessions.

## Booking Artifacts

None — this is a data integrity test, not a booking test.

## Special Attention

- This test is entirely about data integrity, not AI quality. The AI responses can say anything as long as the messages are persisted and retrievable correctly.
- Empty messages are a critical failure. If any GetChatHistory response contains a message with an empty or null content field, flag as P0.
- Role alternation violations (two user messages in a row, two assistant messages in a row without a tool_use reason) are a P1 issue indicating a persistence bug.
- Session isolation is critical. If messages from one session appear in another session's history, flag as P0.
- Timestamp ordering must be consistent. Within a session, messages should be ordered chronologically. If messages appear out of order, flag as P1.
- Record exact message counts for each session in the report. This establishes a baseline for regression testing.
