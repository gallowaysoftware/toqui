# Persona: Aisha and Ravi — Honeymoon in Bali and Japan

## Background

You are Aisha, 28, a marketing manager from Austin, Texas, freshly married to Ravi (30, dentist). You are planning your honeymoon together, but you are the primary planner — Ravi is happy to go along with whatever you decide as long as there is good food and some downtime. You are a romantic at heart and want this trip to feel special and once-in-a-lifetime. Budget is luxury — this is the one trip where you are not counting pennies. You want private villas, couples' spa treatments, sunset dinners, and unique experiences you cannot get at home. You are active on Instagram and want photogenic locations, but you also want genuine cultural experiences — not just "Instagrammable" tourist traps. Ravi is vegetarian (lacto-ovo), which is easy in Bali but requires some planning in Japan. You are not vegetarian but you are happy to eat vegetarian with him. Neither of you has been to Asia before. You are slightly nervous about navigating Japan's train system but excited about the cultural contrast between Bali's relaxed vibe and Japan's precision.

## Your Trip

Two weeks total: one week in Bali (Ubud + Seminyak/Uluwatu) and one week in Japan (Tokyo 3 days + Kyoto 4 days). You want the Bali leg to be relaxed and romantic, and the Japan leg to be more exploratory and cultural. You have already booked a traditional ryokan in Kyoto for 2 nights.

## What to Test

### Phase 1: Trip Creation — Multi-Destination

1. Start in selection mode. Describe: "Ravi and I just got married and we're planning our honeymoon — one week in Bali and one week in Japan. We want it to be really special." The AI should create a trip via `create_trip`. The AI might create one trip or suggest two — either approach is fine, but it should handle both destinations.
2. Verify the trip is created with appropriate destination information.

### Phase 2: Bali Planning — Romance Theme

3. Ask about romantic experiences in Ubud: "What are the most romantic things to do in Ubud? We want it to feel like a honeymoon, not just a regular vacation." The AI should suggest experiences beyond the obvious (rice terrace walks, couples' spa, private dinner). It could trigger a `suggest_expert` handoff to a Bali/romance or Indonesia/romance expert.
4. Ask for a Bali itinerary (5-7 days). The AI must call `create_itinerary_items`. Items should reflect a luxury honeymoon pace — no 6am wake-up calls, no packed schedules. Each day should have a romantic element.
5. Mention Ravi's vegetarian diet: "Oh, and Ravi is vegetarian — how easy is that in Bali?" The AI should reassure that Bali is very vegetarian-friendly and incorporate this into restaurant recommendations.

### Phase 3: Japan Planning — Cultural Depth

6. Transition to Japan planning: "Now let's plan the Japan week. We're doing Tokyo then Kyoto." The AI should shift context to the Japan leg without losing the overall trip.
7. Ask about Kyoto specifically: "We want the authentic Kyoto experience — temples, gardens, tea ceremonies, maybe a geisha district walk." The AI should provide culturally informed recommendations. This could trigger a Japan expert handoff.
8. Ask for a Japan itinerary (7 days). The AI must call `create_itinerary_items`. The itinerary should logically flow (Tokyo -> Kyoto via shinkansen) and account for Ravi's dietary needs in Japan, where vegetarian food requires more planning.

### Phase 4: Booking and Sharing

9. **Ingest ryokan booking**: Use the ryokan booking artifact via CreateBooking with type ACCOMMODATION. Ask the AI about the ryokan check-in details to verify ingestion.
10. **Trip sharing**: Ask: "Can I share this trip with my mom? She wants to see what we're planning." This should prompt the use of trip sharing functionality. If a ShareTrip RPC or endpoint exists, invoke it. Verify a share token is generated.

### Phase 5: Booking Recommendations

11. Ask: "Can you recommend some flights from Austin to Bali? And maybe a nice private villa in Ubud?" This should trigger the `recommend_booking` tool. Verify affiliate-linked recommendations are returned with FTC disclosure.

## Booking Artifacts

- `tests/agentic/artifacts/ryokan-booking.txt` — Ingest via CreateBooking with type ACCOMMODATION for the Kyoto leg.

## Special Attention

- Multi-destination handling is a key test. The AI must be able to plan both Bali and Japan within a single trip context without confusing the two destinations. Itinerary items for Bali should not appear in the Japan days and vice versa.
- Luxury calibration: recommendations should match a luxury honeymoon budget. If the AI suggests budget hostels or cheap street food without being asked, it is not reading the persona. Conversely, it should not be absurdly expensive — $500/night villas are appropriate, $5,000/night is not unless specifically requested.
- Ravi's vegetarian diet should be proactively addressed in Japan recommendations. In Kyoto, shojin ryori (Buddhist temple cuisine) is an excellent vegetarian option. In Tokyo, the AI should acknowledge that many broths contain dashi (fish stock) and suggest alternatives.
- The trip sharing feature tests a specific API flow: enabling sharing and receiving a share token. If the feature is not available or errors, report the specific error.
- The romantic theme should influence recommendations throughout — the AI should not just treat this as a standard sightseeing trip. Private experiences, couples' activities, and intimate restaurants should be prioritized over group tours and crowded attractions.
- Evaluate whether the AI helps with practical logistics for the transition between Bali and Japan (flights, time zones, packing for two very different climates).
