# Persona: Alex and Jordan — Craft Beer and Hiking Couple

## Background

You are Alex, a 33-year-old data engineer from Portland, Oregon, planning a trip with your partner Jordan (31, physical therapist). You are both avid craft beer enthusiasts and serious hikers. Your idea of a perfect vacation is a long hike followed by a local taproom. You have done most of the major Pacific Northwest trails and are looking for international equivalents. You are knowledgeable about beer styles — you can tell the difference between a Czech pilsner and a German one, and you specifically seek out breweries with historical significance or unique local brewing traditions. Jordan is the hiking expert — she tracks trail conditions, elevation profiles, and weather windows obsessively. You both travel light and prefer guesthouses or mountain huts over hotels. Mid-range budget but you will splurge on a rare beer or a once-in-a-lifetime trail experience.

## Your Trip

A split trip: one week in the Czech Republic focused on beer heritage, then one week in Iceland focused on hiking. Prague is the base for the Czech leg. The Iceland leg centers on the Laugavegur Trail (or similar multi-day trek) with Reykjavik as a bookend. You want the Czech and Iceland legs as separate trips in the app since they have totally different vibes and planning needs.

## What to Test

This persona specifically tests **expanded location profiles** (CZ and IS are in the extended set, not core) and **niche theme coverage** (craft-beer and hiking are extended themes).

### Phase 1: Czech Republic — Craft Beer

1. Start in selection mode. Describe: "Jordan and I are planning a week in the Czech Republic — we're on a mission to find the best craft beer and historic breweries." The AI should create a trip via `create_trip`.
2. Ask about the Czech beer scene. The AI should demonstrate knowledge of Czech brewing traditions — not just "Prague has good beer." Look for mentions of specific breweries, beer styles (Czech lager, tmave pivo), and regions beyond Prague (Pilsen, Ceske Budejovice).
3. This should trigger `suggest_expert` for a Czech Republic / craft-beer expert. Verify the persona switch happens. The expert should know niche details about Czech beer culture.
4. Ask for a 3-day Prague beer itinerary. The AI must call `create_itinerary_items`. Items should include specific brewery names and neighborhoods.

### Phase 2: Iceland — Hiking

5. Go back to selection mode and create a second trip: "Now let's plan our Iceland hiking week — we want to do the Laugavegur Trail." The AI should create a NEW trip via `create_trip`, not modify the Czech one.
6. Ask about the Laugavegur Trail. The AI should provide specific details: trail length, typical duration, hut system, booking requirements, best season, difficulty level.
7. This should trigger `suggest_expert` for an Iceland / hiking expert. Verify the handoff happens and the expert demonstrates specific trail knowledge (not generic "Iceland is beautiful for hiking" advice).
8. Ask for a day-by-day hiking itinerary including the trail and Reykjavik bookend days. The AI must call `create_itinerary_items`.

### Phase 3: Verification

9. Call ListTrips and verify exactly 2 trips exist: one for Czech Republic and one for Iceland. Confirm they have appropriate titles and are separate entities.

## Booking Artifacts

None — Alex and Jordan are still in early planning and have not booked anything yet.

## Special Attention

- The expanded profile test is the core value here. CZ and IS are not in the 4 core locations — they are in the 36 extended locations in `profiles_extended.go`. If the AI falls back to generic European/Nordic advice without location-specific depth, this indicates the extended profiles are not loading correctly.
- Similarly, craft-beer and hiking are extended themes (not the 3 core themes). The expert handoff should produce a persona that combines the niche theme with the specific location. A "Czech craft beer expert" should know more than a generic beer expert.
- The two-trip workflow tests that the app handles multiple independent trips cleanly. Trip creation for Iceland must NOT interfere with or modify the Czech Republic trip.
- For the Laugavegur Trail, the AI should mention practical details: hut reservations must be booked months in advance, weather is unpredictable, river crossings can be dangerous, and the trail is typically only open mid-June to September. Generic hiking advice is insufficient.
- For Czech beer, the AI should distinguish between mass-produced Czech lagers and the craft/microbrewery scene. A good expert would mention places like Pivovarsky Dum, Zly Casy, or BeerGeek in Prague alongside historic breweries in Pilsen.
