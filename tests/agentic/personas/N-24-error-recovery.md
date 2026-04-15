# Persona: Structural — Error Recovery & Resilience

## Background

This test verifies the backend handles errors gracefully without data corruption or silent failures. In production, APIs fail, tokens expire, rate limits hit, and malformed data arrives. The backend must handle all of these without crashing or losing data.

## What to Test

### Phase 1: Graceful Degradation
1. **Send oversized message** — SendMessage with content at exactly 10000 chars (the max). Verify it succeeds (boundary condition).
2. **Send message at 10001 chars** — verify `InvalidArgument` error, not a crash or 500.
3. **Empty trip operations** — GetItinerary on a trip with no items. Verify empty array, not error.
4. **Empty booking list** — ListBookings on a trip with no bookings. Verify empty array.
5. **Double delete** — DeleteTrip twice. First should succeed, second should return NotFound (not 500).

### Phase 2: Booking Error Handling
6. **Ingest garbage text** — IngestBooking with "asdfghjkl random nonsense text 12345". The AI parser should still return a booking (probably type "other") rather than erroring.
7. **Ingest extremely long text** — IngestBooking with 5000 chars of repeated booking-like text. Should not OOM or timeout.
8. **ExtractBookingField with nonsense question** — ask "What color is the airplane's left wing?" for a hotel booking. Should return "not found in booking" rather than hallucinating.

### Phase 3: Concurrent-ish Operations
9. **Rapid-fire creates** — Send 3 CreateTrip requests as fast as possible (sequential, not truly concurrent). All 3 should succeed with unique IDs.
10. **Rapid-fire messages** — Send 3 SendMessage requests in quick succession to the same trip/session. All should succeed without data corruption.
11. **ListTrips after rapid creates** — verify all 3 trips from step 9 appear.

### Phase 4: Edge Case Data
12. **Trip with max-length title** — CreateTrip with a 512-character title (the max from buf.validate). Verify it saves and retrieves correctly.
13. **Trip with special characters in title** — CreateTrip with title containing `<script>alert('xss')</script> & "quotes" 'apostrophes'`. Verify it saves verbatim (no HTML escaping in storage, that's the frontend's job).
14. **Booking with empty optional fields** — IngestBooking where the AI returns minimal fields (just type + title). Verify no nil pointer panics.

### Phase 5: API Error Codes
15. **Auth errors** — call GetTrip without auth header. Verify `Unauthenticated`, not `Internal`.
16. **Permission errors** — call GetTrip with a valid token but someone else's trip_id. Verify `NotFound` (not `PermissionDenied` — we don't leak trip existence).
17. **Validation errors** — call CreateTrip with invalid data (empty title). Verify `InvalidArgument` with a helpful message.
18. **Rate limit** — if possible, trigger the per-user rate limiter. Verify `ResourceExhausted` with retry-after info.

## Pass Criteria

All 18 steps must return the expected error codes. Zero 500s, zero panics, zero data corruption.
