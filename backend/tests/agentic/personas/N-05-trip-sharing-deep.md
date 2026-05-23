# Persona: Structural — Trip Sharing Lifecycle

## Background

This is a structural test persona that deeply exercises the trip sharing feature. The goal is to verify the full sharing lifecycle: enable, access, disable, verify revocation, re-enable, and confirm that shared views contain meaningful trip data (not just a title). This covers both the happy path and the critical security path — ensuring that revoked shares are actually inaccessible.

## Your Trip

A trip to Japan with some itinerary content and a booking, created through the normal planning flow so the shared view has something substantive to display. Title: "Tokyo and Kyoto Adventure", destination: Japan, 7 days.

## What to Test

### Phase 1: Trip Setup with Content

1. Create the trip via selection mode chat: "I want to plan a 7-day trip to Tokyo and Kyoto in Japan." Wait for `create_trip` to fire.
2. Send a planning message: "Build me a day-by-day itinerary with 2 days in Tokyo, a day trip to Hakone, and 4 days in Kyoto." Wait for `create_itinerary_items` to fire. Record the number of itinerary items created.
3. Ingest one booking artifact: `tests/agentic/artifacts/ryokan-booking.txt` as BOOKING_TYPE_HOTEL. This gives the shared view a booking to display.

### Phase 2: Enable Sharing

4. `POST /api/trips/share` with the trip_id. Verify:
   - Response contains a `share_token` or `token` field.
   - Token is a non-empty string.
   - Record the token as `token_1`.

### Phase 3: Access Shared View

5. `GET /shared/{token_1}` — Verify the response contains:
   - The trip title ("Tokyo and Kyoto Adventure" or similar).
   - Itinerary items are present in the response. At minimum, item titles and day numbers should be visible.
   - The response does NOT contain the user's auth token, user_id, or any private account information. Shared views must be read-only public projections.
6. Verify the shared view has itinerary items with recognizable content (Tokyo, Kyoto, Hakone references). If the shared view returns only a title and no itinerary, flag as P1 — the shared view is incomplete.

### Phase 4: Disable Sharing

7. `POST /api/trips/unshare` with the trip_id. Verify response indicates success.
8. `GET /shared/{token_1}` — Verify this now returns an error (HTTP 404 or a JSON error response). The old token MUST be invalidated. If the shared view is still accessible after unsharing, flag as P0 security issue — the user explicitly revoked sharing and the data is still public.

### Phase 5: Re-Enable Sharing

9. `POST /api/trips/share` with the trip_id again. Verify:
   - Response contains a new token. Record as `token_2`.
   - `token_2` is different from `token_1`. If the same token is reissued, flag as P1 — token reuse after revocation is a security concern (cached/bookmarked links from the old share would work again).

### Phase 6: Verify New Share

10. `GET /shared/{token_2}` — Verify the trip data is accessible with the new token. Same content checks as Phase 3.
11. `GET /shared/{token_1}` — Verify the OLD token is STILL invalid. Re-enabling sharing must not resurrect old tokens.

### Phase 7: Content Completeness

12. In the shared view from step 10, verify itinerary has day numbers, item titles, and at least 5 items for a 7-day trip. Check whether the ryokan booking appears. Document the shared view's content scope.

## Booking Artifacts

- `tests/agentic/artifacts/ryokan-booking.txt` — Ingest as BOOKING_TYPE_HOTEL

## Special Attention

- Token revocation is the most critical test. Shared data accessible after unsharing is P0 security.
- Token reuse on re-share is P1 security — old bookmarked URLs would work again.
- Shared view must be read-only. Any auth tokens, user IDs, or account details in the response is P0 data leak.
- Shared view without itinerary content is functionally broken (P1). Old tokens must stay invalid after re-share (Phase 6, step 11).
- Document the exact response shape for frontend baseline.
