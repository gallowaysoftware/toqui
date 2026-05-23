# Persona: Structural — GDPR Data Rights Exerciser

## Background

This is a structural test that exercises GDPR Article 17 (right to deletion) and Article 20 (data portability). It creates a full user lifecycle with trips, bookings, itinerary items, and chat history, then exports all data and deletes the account, verifying complete data removal.

## Your Trip

A synthetic trip used for data lifecycle testing. Title: "GDPR Test — Portugal Surf Trip." Destination: Portugal.

## What to Test

Execute the following steps in order:

1. **GetCurrentUser**: Verify identity and record the user_id.
2. **Create trip via chat**: Send a message like "Plan a 5-day surf trip in Portugal." Verify `create_trip` fires.
3. **Add itinerary items**: Send a follow-up requesting specific activities. Verify `create_itinerary_items` fires and items are persisted via `GetItinerary`.
4. **IngestBooking**: Ingest the flight confirmation artifact. Verify via `ListBookings`.
5. **Record consent**: Call `POST /api/privacy/consents` with `{"consent_type":"terms","granted":true}`. Then `GET /api/privacy/consents` to verify it's recorded.
6. **Save preferences**: Call `PUT /api/preferences` with `{"dietary":"vegetarian","budget":"moderate"}`. Then `GET /api/preferences` to verify.
7. **Export data**: Call `ExportData` RPC. Verify the response contains a request_id or download URL. If a download endpoint exists (`/api/export/`), attempt to download and verify JSON contains trip, booking, itinerary, and chat data.
8. **Delete account**: Call `DeleteAccount` RPC with `confirm: true`. This should cascade-delete all data.
9. **Verify deletion**: 
   - `GetCurrentUser` should fail (user gone)
   - `ListTrips` should fail or return empty
   - `GetChatHistory` should fail or return not found
   - The user's data should be fully purged

## Special Attention

- **Cascade deletion is critical**: If any data survives after `DeleteAccount`, that's a GDPR violation (P0).
- **Export completeness**: The export should include ALL user data — trips, bookings, itinerary items, chat messages, preferences, consents.
- **Consent management**: Verify the new consent endpoints work correctly.
- **Preferences persistence**: Verify the new preferences endpoints work correctly.
- Report any data that survives deletion as a P0 bug.
