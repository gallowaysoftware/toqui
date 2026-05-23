# Persona: Structural — Launch Readiness Critical Path

## Background

This is the "would you bet the company on this?" test. It exercises the EXACT user journey a real customer takes from first touch to paid conversion, hitting every critical system in sequence. If any step fails, the product is not ready to launch.

## What to Test

Execute in STRICT order. Each step depends on the prior step succeeding.

### Phase 1: First-Time User Experience
1. **GetCurrentUser** — verify identity, confirm free tier
2. **Selection mode chat** — send "I want to plan a week in Japan for cherry blossom season next spring." Verify `create_trip` fires with destination JP.
3. **Verify trip created** — GetTrip, confirm title/description/destination
4. **Expert handoff** — the AI should proactively call `suggest_expert` for Japan. Verify persona switch event.
5. **Itinerary creation** — verify `create_itinerary_items` fires and items persist (GetItinerary)
6. **Weather tool** — ask "What's the weather like in Tokyo in late March?" Verify `get_weather` fires and returns data.
7. **Currency tool** — ask "How much is 10000 yen in US dollars?" Verify `currency_convert` fires.

### Phase 2: Trip Planning Depth
8. **Budget setting** — send "My budget is $3000 for the week." If update_trip fires with budget, great. Otherwise verify the AI acknowledges the budget.
9. **Dietary context** — send "I'm vegetarian." Verify preferences are captured.
10. **Itinerary modification** — send "Actually, move the temple visit to day 3." Verify `reorder_itinerary_items` or `delete_itinerary_items` + `create_itinerary_items` fires.
11. **Booking ingestion** — ingest the flight confirmation artifact. Verify it parses correctly and auto-links to itinerary.
12. **GetItinerary** — verify all items from steps 5, 10, 11 are present and consistent.

### Phase 3: Export & Sharing
13. **iCal export** — GET /api/trips/{id}/export/ical. Verify returns text/calendar with VEVENT entries.
14. **PDF export** — GET /api/trips/{id}/export/pdf. Verify returns application/pdf (starts with %PDF-).
15. **Enable sharing** — POST /api/trips/share. Verify token returned.
16. **Access shared view** — GET /shared/{token} without auth. Verify trip data visible.
17. **Offline bundle** — GET /api/trips/{id}/bundle. Verify returns trip + itinerary + bookings.

### Phase 4: Companion Mode
18. **Activate trip** — UpdateTrip status to ACTIVE.
19. **Companion query** — send "What's a good ramen place near Shinjuku?" in companion mode. Verify response is concise (<200 words), no itinerary modification.
20. **Explicit modify in companion** — send "Add that ramen place to my plan for tonight." Verify `create_itinerary_items` DOES fire (keyword pre-check should catch this).

### Phase 5: Account & Data
21. **Preferences** — PUT /api/preferences with {dietary: "vegetarian"}. GET /api/preferences, verify round-trip.
22. **Consents** — POST /api/privacy/consents with {consent_type: "terms"}. GET /api/privacy/consents, verify.
23. **Usage** — GET /api/usage. Verify used/limit/tier returned.
24. **Export data** — call ExportData RPC. Verify export includes trips, bookings, itinerary, preferences, consents.
25. **Search** — GET /api/search/itinerary?q=temple. Verify results include items from this trip.
26. **Destination search** — GET /api/destinations/search?q=jap. Verify returns Japan/JP.

### Phase 6: Cleanup
27. **Disable sharing** — POST /api/trips/unshare. Verify old token now returns 404.
28. **Delete trip** — DeleteTrip. Verify cascade (GetTrip returns NotFound, bookings gone, itinerary gone).

## Pass Criteria

ALL 28 steps must pass. Any failure means the product has a broken critical path. This is the test that decides whether we ship.

## Special Attention

- **Step 20 is the companion gate fix** — if the keyword pre-check from PR #324 works, this passes. If not, the user can't add items in companion mode.
- **Steps 13-14 are export features** — new, high-value, never tested in a full user journey.
- **Step 11 booking auto-link** — the auto-created itinerary item should appear in step 12.
- **Context retention** — the AI should remember "vegetarian" and "$3000 budget" across all messages without re-asking.
