# Persona: Structural — Full Lifecycle CRUD Stress Test

## Background

This is not a traveler persona. This is a comprehensive structural test that exercises the complete trip lifecycle from creation through deletion, verifying data integrity at every step. The agent should execute each step in strict order, calling a read/verify operation after every write operation to confirm the backend persisted the expected state. This is the most thorough CRUD test in the suite and catches data loss, COALESCE bugs, status machine errors, and cascade deletion issues.

## Your Trip

A synthetic trip used purely for lifecycle testing. Title: "Lifecycle Test — Kyoto Spring." Description: "Cherry blossom season trip for lifecycle validation." Destination: Japan (Kyoto).

## What to Test

Execute the following 14 steps in strict order. Every write must be followed by a read that verifies the expected state. If any verification fails, continue executing remaining steps but flag the failure.

1. **CreateTrip**: Create a trip with title "Lifecycle Test — Kyoto Spring", description "Cherry blossom season trip for lifecycle validation", and appropriate dates. Record the returned `trip_id`.
2. **GetTrip (verify create)**: Call `GetTrip` with the returned ID. Verify: title matches, description matches, status is `TRIP_STATUS_PLANNING`, dates are set. If any field is missing or wrong, flag it.
3. **UpdateTrip title only**: Update ONLY the title to "Lifecycle Test — Kyoto Autumn". Do NOT send description or dates in the update. Call `GetTrip` to verify: title changed to the new value, description is STILL "Cherry blossom season trip for lifecycle validation" (not nulled out), dates are preserved. This tests COALESCE partial update logic.
4. **UpdateTrip description only**: Update ONLY the description to "Updated: autumn foliage trip." Call `GetTrip` to verify: description changed, title is STILL "Lifecycle Test — Kyoto Autumn" (preserved from step 3), dates preserved.
5. **UpdateTrip status to ACTIVE**: Update the trip status to `TRIP_STATUS_ACTIVE`. Call `GetTrip` to verify: status is now ACTIVE, title and description are preserved from previous steps.
6. **Send a planning message**: Send a chat message like "Add a visit to Kinkaku-ji temple on day 1." The AI should call `create_itinerary_items`. Verify the response contains itinerary-related content.
7. **IngestBooking**: Ingest a flight booking using the flight confirmation artifact text. Call `ListBookings` for this trip. Verify: at least one booking exists, it has a type (flight), and it references this trip.
8. **GetItinerary**: Call `GetItinerary` for the trip. Verify: at least one itinerary item exists from step 6. Record the item count.
9. **Enable sharing**: Call `POST /api/trips/share` for this trip. Record the returned share token. Call `GET /shared/{token}` (unauthenticated) to verify the shared view returns trip data with the correct title.
10. **UpdateTrip status to COMPLETED**: Update status to `TRIP_STATUS_COMPLETED`. Call `GetTrip` to verify: status is COMPLETED, title is "Lifecycle Test — Kyoto Autumn", description is "Updated: autumn foliage trip." All fields from prior updates must be preserved through the status change.
11. **ListTrips**: Call `ListTrips`. Verify: the test trip appears in the list with status `TRIP_STATUS_COMPLETED` and the correct title.
12. **Disable sharing**: Call `POST /api/trips/unshare` for this trip. Attempt `GET /shared/{token}` again — it should now fail or return an error indicating the share is no longer active.
13. **DeleteTrip**: Call `DeleteTrip` for the test trip.
14. **Verify deletion**: Call `GetTrip` with the trip ID — expect `NOT_FOUND`. Call `ListTrips` — verify the trip no longer appears. If bookings had their own list endpoint, verify those are also gone (cascade delete).

## Booking Artifacts

Use `tests/agentic/artifacts/flight-confirmation.txt` for the IngestBooking step. The flight confirmation text should be ingested as raw text and parsed by the AI booking parser.

## Special Attention

- **COALESCE partial update bugs** are the highest-priority catch. Steps 3 and 4 specifically test that updating one field does not null out other fields. This is a historically common bug in UPDATE queries that use COALESCE. If the description disappears when you update the title, that is a P0 regression.
- **Status transition preservation**: Step 10 verifies that changing the status does not reset other fields. Some implementations accidentally overwrite non-status fields during status transitions.
- **Cascade deletion**: Step 14 should verify that deleting a trip also removes its bookings and itinerary items. Orphaned records are a data integrity bug.
- **Share token lifecycle**: Steps 9 and 12 test that sharing can be toggled on and off, and that the public endpoint respects the current sharing state. A share token that still works after unsharing is a security bug.
- **Ordering matters**: These steps must execute in order because each builds on state from the previous step. Parallelization is not allowed for this test.
- Report results as a step-by-step log with pass/fail for each verification point. Include the actual values observed, not just pass/fail, so that partial failures can be diagnosed.
