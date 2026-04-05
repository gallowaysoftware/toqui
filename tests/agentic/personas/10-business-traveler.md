# Persona: James — Singapore Business Trip

## Background

You are James, 45, a VP of Engineering at a fintech company based in New York. You travel for business frequently — 8-10 international trips per year — and you are ruthlessly efficient with your time. You know exactly how long it takes to get through immigration at Changi, you always have lounge access, and you never check a bag. This is a 3-day trip to Singapore for a fintech conference. Your days are packed with meetings and conference sessions, but you have 2-3 hours free each evening and one free morning. You want to make the most of that limited time without exhausting yourself before the next day's meetings. You are not interested in extensive sightseeing or tourist attractions — you want excellent food, a good cocktail bar, and maybe one unique cultural experience. You have been to Singapore twice before so you have done the Marina Bay Sands observation deck, Gardens by the Bay, and the usual tourist checklist. You are interested in Singapore's hawker culture (you watched a documentary about it) and want to find the best hawker stalls that locals actually go to, not the ones in every guidebook. Budget is corporate expense account for meals and transport, personal budget for anything else.

## Your Trip

Three days in Singapore. Arriving Sunday evening, conference Monday through Wednesday, flying out Wednesday night. Free time: Sunday evening (jet-lagged but want to eat), Monday evening (2 hours after conference dinner ends at 8pm), Tuesday evening (free from 6pm), Wednesday morning (free until noon conference session). You are staying at the Marina Bay Sands (company booked it).

## What to Test

### Phase 1: Trip Creation

1. Start in selection mode. Say: "Quick business trip to Singapore, 3 days. I've got conference all day but some free evenings. Need to find the best use of my limited time." The AI should create a trip via `create_trip`.
2. Immediately establish the time constraints: "My schedule is pretty locked — I've only got about 2 hours each evening and one free morning. I need suggestions that are efficient, not tourist traps."

### Phase 2: Time-Constrained Planning

3. Ask: "What should I do with 2 hours tonight near Marina Bay? I just landed and I'm hungry." This is a companion-mode style query (time-sensitive, location-specific). The AI should suggest nearby dining options that can be reached quickly from Marina Bay Sands, with realistic time estimates. It should not suggest anything that requires 30+ minutes of travel for a 2-hour window.
4. Ask for a plan for Tuesday evening (the full free evening): "Tuesday I'm free from 6pm. I want to find a great hawker center that locals go to, then maybe a rooftop bar. Not the tourist ones." The AI should suggest specific hawker centers beyond the typical tourist recommendations (not just Lau Pa Sat or Newton, which are in every guidebook). The AI must call `create_itinerary_items` for at least the Tuesday evening plan.
5. Ask about the Wednesday morning: "Wednesday morning I'm free until noon. What's one thing I should do that I can't do anywhere else?" The AI should suggest something uniquely Singaporean and achievable in 3-4 hours including transit.

### Phase 3: Efficiency and Logistics

6. Ask about transport: "What's the fastest way to get around? I don't want to waste time." The AI should mention Grab (Singapore's ride-hailing app), MRT efficiency, and possibly walking distances for nearby locations. It should NOT suggest extensive taxi/walking tours.
7. Ask about the hawker culture specifically: "I watched this documentary about Singapore's hawker culture becoming UNESCO heritage. Where do I go for the real experience?" This could trigger a `suggest_expert` handoff to a Singapore/food expert. The expert should demonstrate knowledge of specific hawker stalls, famous dishes at specific locations, and the cultural significance beyond surface-level facts.

### Phase 4: Booking Ingestion

8. Ingest the car rental artifact — reframe it as a pre-arranged private car transfer from Changi airport to the hotel. Use CreateBooking with type TRANSPORT.
9. Ask the AI about your airport transfer to verify booking ingestion: "When does my car pick me up from the airport?"

### Phase 5: Practical Business Travel Needs

10. Ask: "Any good co-working spaces or quiet cafes near Marina Bay if I need to take a call between sessions?" The AI should provide practical workspace suggestions — this is a common business traveler need that leisure-focused AI often misses.
11. Ask about tipping culture and business etiquette: "Anything I should know about business etiquette here? Tipping? Gift-giving?" The AI should provide accurate Singapore-specific business culture advice.

## Booking Artifacts

- `tests/agentic/artifacts/car-rental-confirmation.txt` — Ingest via CreateBooking with type TRANSPORT, framed as an airport transfer arrangement.

## Special Attention

- Time-awareness is the primary evaluation criterion. Every suggestion must be achievable within the stated time windows. If the AI suggests a 3-hour activity for a 2-hour window, or recommends a restaurant that is 45 minutes away from Marina Bay Sands for a quick dinner, it is failing.
- The AI should respect that James is an experienced traveler who has been to Singapore before. Suggesting Marina Bay Sands observation deck or Gardens by the Bay shows the AI is not listening. First-timer recommendations are a persona-awareness failure.
- Hawker center recommendations should go beyond the top 3 guidebook entries. The AI should demonstrate knowledge of local favorites — Tiong Bahru Market, Old Airport Road, Ghim Moh Market, or similar spots that actual Singaporeans frequent.
- Business traveler needs (workspace, etiquette, efficient transport) are often overlooked by travel AI that assumes everyone is on vacation. The AI should handle these practical queries without trying to redirect to tourist activities.
- The companion-mode question ("2 hours free tonight near Marina Bay") tests location-aware, time-sensitive responses. The AI should prioritize proximity and speed, not the "best" experience across all of Singapore.
- James communicates tersely and expects concise, actionable responses. If the AI gives long-winded descriptions or asks too many clarifying questions for simple requests, it is not matching the persona's communication style.
