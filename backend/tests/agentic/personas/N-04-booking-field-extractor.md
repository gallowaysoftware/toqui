# Persona: Structural — Booking Field Extraction Gauntlet

## Background

Structural test: ingest all 7 booking artifacts, then use ExtractBookingField RPC to ask a pointed question about each and verify correct extraction from the raw text. Tests the full booking pipeline end-to-end.

## Your Trip

A multi-destination trip covering the bookings across all artifacts. Create via CreateTrip RPC with title "Booking Gauntlet Trip", destination "Multiple Destinations." The trip is a container for bookings — no chat or itinerary needed.

## What to Test

### Phase 1: Ingest All 7 Booking Artifacts

Ingest each artifact via IngestBooking/CreateBooking RPC with the raw text content and correct booking type.

1. `tests/agentic/artifacts/flight-confirmation.txt` — type: BOOKING_TYPE_FLIGHT
2. `tests/agentic/artifacts/hotel-confirmation.txt` — type: BOOKING_TYPE_HOTEL
3. `tests/agentic/artifacts/activity-confirmation.txt` — type: BOOKING_TYPE_ACTIVITY
4. `tests/agentic/artifacts/car-rental-confirmation.txt` — type: BOOKING_TYPE_CAR_RENTAL
5. `tests/agentic/artifacts/hostel-booking.txt` — type: BOOKING_TYPE_HOTEL
6. `tests/agentic/artifacts/ryokan-booking.txt` — type: BOOKING_TYPE_HOTEL
7. `tests/agentic/artifacts/tour-booking.txt` — type: BOOKING_TYPE_TOUR

Record each booking_id. Verify all 7 succeed.

### Phase 2: List and Verify

8. ListBookings — verify 7 bookings, each with non-empty booking_id, correct type, and non-empty raw_text/summary.

### Phase 3: Extract Specific Fields

Call ExtractBookingField for each booking with a targeted question.

9. **Flight**: "What terminal and what time does my outbound flight depart?" — The answer must include a specific terminal number/letter and a specific departure time. If the answer is vague ("check your confirmation"), the extraction failed.

10. **Hotel**: "What is the cancellation policy?" — The answer must reference specific cancellation terms (deadline date, penalty amount, or free cancellation window). Generic advice to "check the hotel's policy" is a failure.

11. **Activity**: "What is the meeting point and what should I bring?" — The answer must include a specific meeting location and any required items mentioned in the confirmation (comfortable shoes, sunscreen, ID, etc.).

12. **Car Rental**: "What insurance is included and what's the fuel policy?" — The answer must mention the specific insurance type (CDW, liability, etc.) and the fuel return policy (full-to-full, prepaid, etc.).

13. **Hostel**: "What time is check-in at the Hanoi hostel?" — The answer must include a specific check-in time. If the artifact contains multiple hostels, the AI must correctly identify the Hanoi-specific information and not confuse it with other locations.

14. **Ryokan**: "What meals are included and what time is dinner?" — The answer must specify which meals are included (breakfast, dinner, or both) and a dinner time or window.

15. **Tour**: "How many food tastings are included and what dietary accommodations are available?" — The answer must include a number of tastings and mention dietary options (vegetarian, vegan, gluten-free, or a note about accommodations).

### Phase 4: Negative Test

16. Call ExtractBookingField on the flight booking with the question: "What is the hotel's wifi password?" — This is an irrelevant question. The AI should indicate that this information is not in the flight confirmation, not hallucinate an answer.

## Booking Artifacts

All 7 artifacts in `tests/agentic/artifacts/`:
- `flight-confirmation.txt` — Ingest as BOOKING_TYPE_FLIGHT
- `hotel-confirmation.txt` — Ingest as BOOKING_TYPE_HOTEL
- `activity-confirmation.txt` — Ingest as BOOKING_TYPE_ACTIVITY
- `car-rental-confirmation.txt` — Ingest as BOOKING_TYPE_CAR_RENTAL
- `hostel-booking.txt` — Ingest as BOOKING_TYPE_HOTEL
- `ryokan-booking.txt` — Ingest as BOOKING_TYPE_HOTEL
- `tour-booking.txt` — Ingest as BOOKING_TYPE_TOUR

## Special Attention

- Field extraction accuracy is the primary metric. Cross-reference every answer against the raw artifact. Specific answers contradicting the artifact are P0. Vague non-answers when the info IS present are P1.
- The negative test (Phase 4) detects hallucination. If the AI fabricates hotel info from a flight confirmation, flag as P0.
- Multi-hostel disambiguation: the hostel artifact has multiple locations. The AI must isolate Hanoi-specific info without confusing cities. Mixing locations is P1.
- Booking types in ListBookings must match submission. Type misclassification is P1.
- Record every extracted answer verbatim in the report alongside expected information for regression baseline.
