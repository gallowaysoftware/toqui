# Persona: Structural — Unicode & International Character Handling

## Background

This test verifies the backend handles non-ASCII content correctly throughout the stack. A global travel app must handle CJK characters, Arabic, Cyrillic, accented Latin, emoji, and mixed scripts without corruption, truncation, or errors.

## What to Test

### Phase 1: Trip with Unicode Title/Description
1. **CreateTrip** with title "東京桜旅行 🌸" (Tokyo Cherry Blossom Trip) and description "Voyage à travers le Japon — découvrir les temples, la cuisine, et les cerisiers en fleurs."
2. **GetTrip** — verify title and description round-trip exactly, including emoji and accented characters.
3. **ListTrips** — verify the trip appears with correct title in the list.
4. **Search** — call SearchTripsByUser with query "東京". Verify the trip is found via full-text search with CJK characters.

### Phase 2: Itinerary with Mixed Scripts
5. **UpdateItinerary** — create items with these titles:
   - Day 1: "浅草寺 (Sensō-ji Temple)" — CJK + Latin
   - Day 2: "Café près de la Tour Eiffel" — accented Latin
   - Day 3: "Большой театр (Bolshoi Theatre)" — Cyrillic + Latin
   - Day 4: "مسجد الحسن الثاني (Hassan II Mosque)" — Arabic + Latin
   - Day 5: "🎌 Festival Day — 祭り" — emoji + CJK
6. **GetItinerary** — verify ALL titles round-trip exactly.
7. **Search itinerary** — GET /api/search/itinerary?q=浅草寺. Verify CJK search works.

### Phase 3: Bookings with International Content
8. **IngestBooking** — ingest a booking with raw text containing accented characters: "Hôtel & Résidence — Châtelet-Les Halles, €149/nuit"
9. **GetBooking** — verify the parsed title/provider preserves accents.
10. **ExtractBookingField** — ask about the hotel, verify response handles accents.

### Phase 4: Chat with Unicode
11. **SendMessage** — send "東京で美味しいラーメン屋を教えてください" (Tell me a good ramen shop in Tokyo). Verify AI responds (content doesn't matter, just that it doesn't error).
12. **GetChatHistory** — verify the Japanese message is stored and retrieved correctly.

### Phase 5: User Data with Unicode
13. **Preferences** — PUT /api/preferences with {dietary: "végétarien", budget: "modéré"}. GET and verify accents preserved.
14. **Trip sharing** — share the unicode-titled trip. GET /shared/{token} — verify the title renders correctly in the shared view.
15. **iCal export** — export and verify the VCALENDAR contains the unicode titles without mojibake.

## Pass Criteria

All 15 phases must pass. Any character corruption, truncation, or encoding error is a P0 for a global app.
