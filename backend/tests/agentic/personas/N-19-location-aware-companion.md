# Persona: Suki — Location-Aware Walking Tour (Barcelona)

## Background

You are Suki, a 28-year-old architect from Tokyo exploring Barcelona on foot with GPS active. You're interested in Gaudí's architecture, local tapas, and street photography. You have an active trip in companion mode and are sending messages with your real GPS coordinates.

## Your Trip

Title: "Barcelona Architecture Walk." Duration: 3 days. Status: ACTIVE (companion mode). Destination: Spain (ES).

## What to Test

1. **Create trip**: Create the trip via RPC (not chat), then update status to ACTIVE.
2. **Send companion message with location**: Send a chat message "What's interesting near me?" with `user_location` set to `{latitude: 41.4036, longitude: 2.1744}` (Sagrada Família area). Verify the response mentions nearby landmarks.
3. **Test nearby_places tool**: The message should trigger the `nearby_places` tool. Verify it returns results (not empty — this was a P0 stub in Run 19, now implemented). If no Google Places API key is configured, expect a graceful error rather than empty results.
4. **UpdateLocation RPC**: Call `UpdateLocation` directly with coordinates `{latitude: 41.3851, longitude: 2.1734}` (Gothic Quarter). Verify the response is successful.
5. **GetNearby RPC**: Call `GetNearby` with the same coordinates and category "restaurant". Verify structured place results are returned (or a clear "no API key" error in test env).
6. **Second companion message with different location**: Send "Where should I eat around here?" with `user_location` set to `{latitude: 41.3851, longitude: 2.1734}` (Gothic Quarter — different from first message). Verify the AI's response changes to reflect the new location.
7. **ResolvePersona RPC**: Call `ResolvePersona` with trip_id, latitude 41.4036, longitude 2.1744, and themes ["architecture"]. Verify a Spain + architecture expert persona is returned.
8. **Weather tool**: Ask "What's the weather like today?" — verify the `get_weather` tool fires and returns Barcelona weather data.
9. **Currency tool**: Ask "How much is 50 euros in yen?" — verify the `currency_convert` tool fires and returns a conversion.

## Special Attention

- **`nearby_places` must not return empty**: This was the P0 bug from the audit (#236, now fixed). If it still returns empty with no error, that's a P0 regression.
- **LocationService RPCs**: UpdateLocation and GetNearby had zero test coverage before. This persona fills that gap.
- **PersonaService**: ResolvePersona also had zero coverage. Test it here.
- **New tools**: Weather and currency tools are brand new (#267). Verify they work in companion mode.
- **GPS coordinate changes**: The AI should give different recommendations for Sagrada Família vs Gothic Quarter.
