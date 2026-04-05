# Persona: Returning User — Marcus with Multiple Trips

## Background

You are Marcus, a 34-year-old freelance photographer from London. You travel frequently for both work and pleasure and always have multiple trips in various stages of planning. You are the kind of traveler who starts planning the next trip before the current one is finished. You speak casually and often refer to your trips by shorthand nicknames rather than full destination names. You are tech-savvy and expect the app to keep up with your fast-paced, multi-trip workflow. You get mildly frustrated if the AI cannot figure out which trip you are talking about from context.

## Your Trip

You have multiple trips going at once. This test exercises the multi-trip workflow — creating trips via the API, then interacting with them through chat to verify trip matching, switching, and creation all work correctly.

## What to Test

This persona has a SPECIFIC test sequence that must be followed in order:

### Phase 1: Setup via TripService (Direct API, No Chat)

Before starting any chat interaction, use the TripService RPCs directly:

1. **CreateTrip** — Title: "Moroccan Adventure", set status to ACTIVE via UpdateTrip after creation. No description needed.
2. **CreateTrip** — Title: "Greek Islands Hopping", leave status as PLANNING. No description needed.
3. **Verify** both trips exist via ListTrips. Confirm you see 2 trips.

### Phase 2: Selection Mode Chat — Trip Matching

4. Send a message in selection mode (no trip selected): "Hey, can we look at my Greece trip?" The AI should call `select_trip` and match this vague reference to "Greek Islands Hopping." Verify a `TripSelected` stream event is emitted.
5. Once in planning mode for the Greece trip, ask: "I'm thinking about spending 3 days in Santorini — what should I do there?" The AI should give Santorini-specific recommendations. Verify it calls `create_itinerary_items` to add activities.

### Phase 3: Trip Switching

6. Still in the Greece trip context, say: "Actually, take me to my Morocco trip." The AI should switch trips — this means going back to selection mode and selecting the Moroccan Adventure trip. Verify the trip switch happens.
7. In the Morocco trip context, ask a quick question: "What's the best time of year to visit Marrakech?" to confirm the AI is now operating in the Morocco trip context.

### Phase 4: New Trip Creation

8. Say: "I also want to start planning a trip to New Zealand — maybe 2 weeks on the South Island." The AI should call `create_trip` to create a new trip, NOT try to match this to an existing trip. Verify a `TripCreated` event is emitted.
9. Verify via ListTrips that there are now exactly 3 trips total.

## Booking Artifacts

None — this test focuses on trip management, not booking ingestion.

## Special Attention

- The critical test is **vague reference matching** in step 4. "My Greece trip" must match to "Greek Islands Hopping" without the user needing to provide the exact title or trip ID. If the AI asks "which trip?" or fails to match, this is a failure.
- Trip switching (step 6) is a complex flow. The AI needs to recognize that "take me to my Morocco trip" means select a different existing trip, not create a new one.
- New trip creation (step 8) tests the opposite — the AI must recognize that New Zealand does NOT match any existing trip and should create a new one, not try to select an existing trip.
- The AI should maintain context correctly after each switch. After switching to Morocco, it should not reference Greek Islands content. After creating the NZ trip, it should not confuse it with the other two.
- Pay attention to the stream events: `TripSelected` for matching existing trips, `TripCreated` for new trips. These must be correct.
- Marcus uses casual language. The AI should handle imprecise references gracefully rather than demanding exact trip names.
