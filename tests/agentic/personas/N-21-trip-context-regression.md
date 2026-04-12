# Persona: Structural — Trip Context Persistence Regression Canary

## Background

This is a fast-failing regression test for the Run 19 P1 `no_trip_selected` bug. It specifically tests that `create_itinerary_items` works across multiple messages in the same session, and that the trip ID is correctly resolved after trip creation and selection.

## Your Trip

Title: "Regression Canary — Greek Islands." Destination: Greece (GR). Created via chat in selection mode.

## What to Test

Execute these steps in strict order, verifying itinerary persistence after EACH message:

1. **Selection mode — create trip**: Send "I want to plan a 7-day trip to the Greek islands." Verify `create_trip` fires and a trip is created. Record the trip_id.

2. **Immediate follow-up — add items**: In the SAME session, send "Start with 3 days in Santorini — add sunset viewing at Oia, a boat tour to the caldera, and wine tasting." Verify `create_itinerary_items` fires. Call `GetItinerary` — verify 3+ items exist. **This is where the Run 19 bug manifested** — if `no_trip_selected` is returned here, the fix regressed.

3. **Second follow-up — more items**: Send "Then 2 days in Mykonos — add beach day at Paradise Beach and dinner in Little Venice." Verify items are created. Call `GetItinerary` — verify total is 5+ items (previous + new).

4. **Third follow-up — modify**: Send "Actually, move the wine tasting to day 4 in Mykonos." Verify `reorder_itinerary_items` fires (new tool from #247). Call `GetItinerary` to verify the item moved.

5. **Fourth follow-up — delete**: Send "Remove the boat tour." Verify `delete_itinerary_items` fires. Call `GetItinerary` — verify the boat tour item is gone and total count decreased.

6. **Fifth follow-up — update trip**: Send "Change the trip title to 'Santorini & Mykonos Adventure'." Verify `update_trip` fires (tool from #245). Call `GetTrip` to verify title changed.

7. **New session**: Start a NEW chat session for the same trip (send trip_id in the request). Send "Add a day trip to Delos." Verify `create_itinerary_items` works in the new session too.

8. **Verify final state**: Call `GetItinerary` one final time. Verify all items from steps 2, 3, 7 are present (minus the deleted one from step 5), and the moved item is on the correct day.

## Also Check For

- **Text stutter**: Check all AI responses for duplicated phrases (Run 19 N-01 P2, fixed in #233).
- **Template placeholders**: Check for `{{...}}` patterns in responses (Run 19 N-12 P2, fixed in #231).
- **messageCount**: Compare `ListChatSessions` messageCount with actual `GetChatHistory` count.
- **Context retention**: The AI should remember Santorini/Mykonos context across all messages without re-asking.

## Special Attention

- **Step 2 is the critical check**: If `create_itinerary_items` fails with `no_trip_selected` on the immediate follow-up to trip creation, the `selectedTripID` persistence fix from PR #231 has regressed.
- **Step 7 tests cross-session**: The trip_id must be passed correctly in new sessions.
- **Steps 4-6 test new tools**: reorder (#247), delete, and update_trip (#245) are all new since Run 19.
- This persona should fail fast (within 2 messages) if the core regression is present, rather than completing a long test that incidentally catches it.
