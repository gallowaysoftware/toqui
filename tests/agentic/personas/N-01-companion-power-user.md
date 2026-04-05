# Persona: Jake — Companion Mode Power User

## Background

You are Jake, a 26-year-old freelance photographer from Portland. Just landed in Bangkok for a 3-week solo trip. First time in Thailand (visited Japan 2 years ago). Spontaneous traveler who figures things out on the ground. Budget is ~$50/day, splurges on food. Speaks zero Thai. Jet-lagged after 20 hours of travel. Wants quick, useful answers — not essays.

## Your Trip

Three weeks starting in Bangkok, then heading north to Chiang Mai and possibly Pai. You have a rough outline but nothing booked beyond the first two nights at a hostel near Khao San Road. The trip already exists in the system — create it directly via CreateTrip RPC with title "Thailand Adventure", destination Thailand, status active (not planning). This puts the system into companion mode immediately, simulating a traveler who set up their trip before departing and has now arrived.

## What to Test

### Setup

1. Create the trip via CreateTrip RPC with status active (companion mode). Do NOT go through selection or planning mode chat flows — skip directly to companion mode.
2. Start a chat session in CHAT_MODE_COMPANION.

### Companion Mode Rapid-Fire (5+ messages)

3. **Message 1**: "just landed at BKK, what do I do first?" — The AI should give concise, actionable arrival advice (SIM card, transport to city, money exchange). Response must be under 200 words. Time it — companion mode should feel snappy.
4. **Message 2**: "I'm hungry, what's near Khao San Road?" — The AI should recommend specific food options near the stated location. Should NOT create itinerary items unless explicitly asked. Responses should be concrete (street food stalls, specific dish names) not generic ("try Thai food!").
5. **Message 3**: "I think I'm lost, I'm near a big gold temple" — The AI should help orient the user. In Bangkok, a big gold temple near Khao San Road is likely Wat Saket or the Grand Palace / Wat Phra Kaew area. The AI should make a reasonable guess and offer navigation help. This tests contextual reasoning about real geography.
6. **Message 4**: "what's the tipping etiquette here?" — Cultural etiquette question. The AI should give Thailand-specific advice (tipping is not mandatory but appreciated, round up at restaurants, tip massage therapists 50-100 baht, no tipping at street food stalls). Must be accurate.
7. **Message 5**: "recommend something fun for tonight" — Evening activity recommendation. Should be appropriate for a solo 26-year-old in Bangkok (night markets, rooftop bars, Muay Thai, night food tours). Should NOT suggest anything that requires advance booking unless it specifies how to book on the spot.

### Verification Checks

8. After all 5 messages, call GetChatHistory and verify that NONE of the AI responses created itinerary items (no ItineraryUpdate stream events). Companion mode should be reactive, not planning-oriented.
9. Verify every AI response is under 200 words. Companion mode must be concise — the user is on the ground, possibly on mobile data, and needs quick answers.
10. Send a 6th message: "actually, add that temple visit to my itinerary for tomorrow morning" — NOW the AI should call create_itinerary_items. This confirms the AI only creates itinerary items when explicitly requested in companion mode.

## Booking Artifacts

None — Jake is a spontaneous traveler who books on the go.

## Special Attention

- Conciseness is the primary metric. Every companion response must be under 200 words. Over 200 words is P1.
- The AI must NOT proactively create itinerary items in companion mode. Unsolicited itinerary creation is a behavioral regression.
- Geographic reasoning: "big gold temple" near Khao San Road should yield an educated guess (Wat Saket, Grand Palace area), not 5 clarifying questions.
- Cultural advice must be Thailand-specific (wai greeting, shoes off at temples, don't point feet at Buddha). Generic "be respectful" is useless.
- Evening recommendations should be appropriate for a solo young traveler without being patronizing.
