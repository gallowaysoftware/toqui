# Persona: Structural — Adversarial Edge Cases

## Background

This is not a traveler persona. This is a structural and adversarial test that exercises error handling and boundary conditions across the API surface. The agent should execute each test case in order, recording the expected versus actual behavior for every call. There is no trip narrative — this is pure API validation. The agent should be methodical and precise, reporting exact error codes and messages.

## Your Trip

No trip narrative. This test creates and manipulates trips solely to exercise validation logic and error paths.

## What to Test

Execute the following 10 test cases in order. For each, record: test number, description, expected result, actual result, and pass/fail.

1. **GetTrip with non-existent UUID**: Call `GetTrip` with a properly formatted but non-existent UUID (e.g., `00000000-0000-0000-0000-000000000000`). Expected: `NOT_FOUND` error code. The server must not panic or return a 500.
2. **GetTrip with malformed ID**: Call `GetTrip` with `trip_id` set to the literal string `not-a-uuid`. Expected: `INVALID_ARGUMENT` error from buf.validate constraints (UUID format validation). If the server returns `NOT_FOUND` instead, the validation interceptor is not catching malformed IDs.
3. **CreateTrip with empty title**: Call `CreateTrip` with an empty string for `title`. Expected: `INVALID_ARGUMENT` error. Trips must have a non-empty title.
4. **CreateTrip with oversized title**: Call `CreateTrip` with a `title` that is 513 characters long (one over the 512-character limit). Expected: `INVALID_ARGUMENT` error from buf.validate string length constraint. If the trip is created successfully, the validation constraint is missing or misconfigured.
5. **Invalid status transition**: First create a valid trip. Then call `UpdateTrip` to set its status to `TRIP_STATUS_COMPLETED`. Then attempt to update the status back to `TRIP_STATUS_PLANNING`. Record the actual behavior — the backend may reject this as an invalid state transition, or it may allow it. Document which behavior occurs and whether it seems intentional.
6. **SendMessage with empty content**: In a chat session for a valid trip, call `SendMessage` with an empty `content` string. Expected: `INVALID_ARGUMENT` error. The AI should not be invoked with empty input.
7. **SendMessage with oversized content**: Call `SendMessage` with a `content` string that is 10,001 characters long (one over the 10,000 limit). Expected: `INVALID_ARGUMENT` error from buf.validate constraint. If the message is accepted, the constraint is missing.
8. **DeleteBooking on non-existent ID**: Call `DeleteBooking` with a properly formatted UUID that does not correspond to any booking. Record whether the server returns `NOT_FOUND`, silently succeeds (idempotent delete), or returns an unexpected error.
9. **IngestBooking with empty raw_text**: Call `IngestBooking` with `raw_text` set to an empty string. Expected: `INVALID_ARGUMENT` error. Ingesting an empty booking confirmation makes no sense and should be rejected.
10. **ListTrips with zero trips**: After ensuring the test user has no trips (or before creating any), call `ListTrips`. Expected: a successful response with an empty `trips` array (not an error). Some APIs incorrectly return `NOT_FOUND` for empty collections.

## Booking Artifacts

None. Test case 8 uses a fabricated non-existent booking UUID. Test case 9 uses an empty string.

## Special Attention

- **Error code precision matters**: The difference between `NOT_FOUND` and `INVALID_ARGUMENT` is significant. `INVALID_ARGUMENT` means the input was malformed and never reached business logic. `NOT_FOUND` means the input was valid but the resource does not exist. If the API returns the wrong category of error, that is a bug worth reporting.
- **No panics or 500s**: None of these edge cases should produce a server panic or HTTP 500. If any does, report it as P0.
- **Validation interceptor coverage**: Tests 2, 3, 4, 6, 7, and 9 specifically test whether buf.validate constraints are properly defined in the proto files and enforced by the validation interceptor. Missing constraints are P1 bugs.
- **Idempotency**: Test 8 checks delete idempotency. Both `NOT_FOUND` and silent success are acceptable — but a 500 error is not.
- **State machine enforcement**: Test 5 checks whether trip status transitions are validated. If the backend allows COMPLETED to PLANNING, note whether this seems intentional (some systems allow reactivation) or accidental (missing validation).
- Report results in a structured table format for easy scanning. Each row: test number, description, expected, actual, pass/fail, notes.
