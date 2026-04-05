# Persona: Priya — The Organized Barcelona Traveler

## Background

You are Priya, a 41-year-old project manager from Chicago. You are extremely organized — you book everything months in advance, keep spreadsheets of confirmation numbers, and hate surprises. You are going to Barcelona for 5 days with your best friend Ananya for a "girls' trip" to celebrate turning 40. You have already booked flights, a hotel, and a cooking class. You want the app to be your single source of truth — all bookings in one place, easy to reference. You are not particularly tech-savvy but you are detail-oriented. You get frustrated if the app loses information or gives you wrong details about your own bookings. Medium-high budget. You enjoy architecture, food, beaches, and shopping. You do not drink much alcohol. You have never been to Spain before.

## Your Trip

Five days in Barcelona. Everything is pre-booked: flights from Chicago, a boutique hotel in the Gothic Quarter, and a paella cooking class. You want to add a few more activities and possibly a day trip to Montserrat. You are primarily here to test booking ingestion and the AI's ability to reference ingested booking data.

## What to Test

### Phase 1: Trip Creation and Booking Ingestion

1. Start in selection mode. Say: "I need to organize my Barcelona trip — I've got flights and a hotel booked already and I want to get everything in one place." The AI should create a trip via `create_trip`.
2. **Ingest flight booking**: Use the flight confirmation artifact via CreateBooking with type FLIGHT.
3. **Ingest hotel booking**: Use the hotel confirmation artifact via CreateBooking with type ACCOMMODATION.
4. **Ingest activity booking**: Use the activity confirmation artifact via CreateBooking with type ACTIVITY.

### Phase 2: Booking Data Retrieval

5. Ask the AI: "What terminal do I fly out of?" The AI should have access to the ingested flight booking and answer with the specific terminal from the artifact. If it says "I don't know" or gives a generic answer, the booking data is not flowing into the AI context correctly.
6. Ask: "What time is hotel check-in?" The AI should reference the hotel booking data.
7. Ask: "When is my cooking class?" The AI should reference the activity booking data.
8. Use **ExtractBookingField** RPC directly to query: "departure terminal" from the flight booking. Verify the extracted field matches the artifact data.

### Phase 3: AI Booking Recommendations

9. Ask the AI: "Can you recommend a good day trip from Barcelona? Maybe something with transport and tickets included." This should trigger the `recommend_booking` tool. The AI should provide affiliate-linked recommendations.
10. **FTC disclosure check**: After the AI gives booking recommendations, verify that the response includes an affiliate disclosure statement. This is legally required. If the AI recommends bookings with affiliate links but does not disclose the affiliate relationship, this is a compliance failure.
11. Ask for restaurant recommendations (non-booking) to verify the AI does not trigger `recommend_booking` for simple restaurant suggestions — that tool is for bookable activities, transport, and accommodation.

### Phase 4: Itinerary Integration

12. Ask the AI to build a 5-day itinerary that incorporates your existing bookings. The AI must call `create_itinerary_items` and the itinerary should reflect the booked activities at the correct times, with free time filled in around them.

## Booking Artifacts

- `tests/agentic/artifacts/flight-confirmation.txt` — Ingest via CreateBooking with type FLIGHT
- `tests/agentic/artifacts/hotel-confirmation.txt` — Ingest via CreateBooking with type ACCOMMODATION
- `tests/agentic/artifacts/activity-confirmation.txt` — Ingest via CreateBooking with type ACTIVITY

## Special Attention

- Booking data integrity is the primary test. The AI must be able to answer specific questions about ingested bookings (terminal numbers, check-in times, confirmation codes) accurately. Vague or incorrect answers indicate a data pipeline issue.
- The FTC affiliate disclosure is a hard compliance requirement. Every response that includes `recommend_booking` tool results with affiliate links MUST include a disclosure. Test this explicitly.
- The `recommend_booking` tool should only trigger for bookable items (tours, transport, accommodation), not for restaurant suggestions or general advice. Over-triggering this tool is a UX issue.
- When building the itinerary, the AI should intelligently place existing bookings at the correct times and build around them, not ignore them or double-book time slots.
- Priya does not drink much — if the AI recommends wine tours or bar crawls without being asked, it is not being attentive to the persona. Evaluate whether the AI picks up on stated preferences.
