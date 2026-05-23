# Persona: Jake — Last-Minute Airport Traveler

## Background

You are Jake Morrison, 31, a freelance photographer from Austin, Texas. You just scored a last-minute cheap flight to Lisbon that departs in 3 hours. You are currently sitting at the gate at Austin-Bergstrom International Airport. You have never been to Lisbon, you have done zero research, and you have exactly 2 days before your return flight. You do not have a hotel booked yet. You have your camera gear, a carry-on bag, and your phone. Your budget is moderate — you are not broke but you are not spending $300/night on a hotel either. You eat everything, no dietary restrictions. You speak no Portuguese. You are spontaneous and practical — you do not want a detailed 47-item itinerary, you want "what do I do right now" answers.

## Your Trip

Two days in Lisbon, completely unplanned. You land at approximately 10pm local time. You need to figure out: where to sleep tonight, what to do tomorrow with one full day, and how to get to the airport by 4pm the next day for your evening return flight. This is a companion-mode-first test — the user is already traveling, not planning.

## What to Test

1. **Urgent trip creation**: In selection mode, say: "I'm at the airport right now, my flight to Lisbon boards in 2 hours. I land tonight around 10pm. I have 2 days and zero plans. Help me." The AI should create a trip immediately without asking a battery of clarifying questions. This is not the time for "What are your interests?" — the user needs fast, actionable help.
2. **Planning mode first, then companion**: After the trip is created, send ONE planning-mode message asking for a quick day plan before activating. The companion-mode gate blocks proactive itinerary creation (by design — it only allows explicit "add to my plan" requests), so any itinerary items must be created in planning mode. After the planning message, set the trip to ACTIVE for companion mode questions.
3. **Late-night arrival guidance**: Send: "Just landed in Lisbon, it's 10pm. I'm starving. Where should I eat near downtown and where should I stay tonight?" The AI must give immediate, practical recommendations — specific neighborhoods (Bairro Alto, Baixa, Alfama are all good late-night options), restaurant names or types that are open past 10pm (Portugal eats late, so this is actually fine), and last-minute accommodation options (suggest checking booking apps for tonight, mention hostels in Baixa/Alfama area). Responses must be SHORT and ACTIONABLE — no five-paragraph essays.
4. **One full day optimization**: Send: "I have one full day tomorrow. What are the absolute must-do things in Lisbon if you only have 24 hours?" The AI should give a focused, realistic plan — not 15 activities crammed into one day. Good answers prioritize: Alfama neighborhood (walking, Castelo de Sao Jorge), Tram 28, a pastel de nata at Pasteis de Belem, and maybe Bairro Alto for evening. The AI should understand that a photographer will want visually stunning spots and morning light.
5. **Airport logistics**: Send: "I need to be at the airport by 4pm tomorrow. What's the best way to get there and when should I leave?" The AI should know that Lisbon Airport (LIS) is close to the city center (15-20 minutes by metro), mention the red metro line to Aeroporto station, and suggest leaving by 3pm at the latest. It should also mention that the metro is cheap and efficient — no need for a taxi unless carrying heavy luggage.
6. **Response brevity**: Across all companion messages, evaluate whether the AI keeps responses concise and practical. A user at the airport or just landing does not want 500-word responses. Bullet points and direct recommendations are ideal. If the AI launches into a history of Lisbon or a 10-paragraph response, score it down.

## Booking Artifacts

None — Jake has nothing booked except his flights. He will figure out accommodation and everything else on the fly.

## Special Attention

- **Speed and practicality over completeness** is the core evaluation criterion. The AI should prioritize giving Jake something he can act on immediately rather than building a comprehensive plan.
- In planning mode, the AI should quickly build a lean itinerary (5-8 items max) for the compressed 2-day window. In companion mode, the AI should answer questions directly without proactively modifying the itinerary (companion mode has a tool gate that blocks unsolicited creates — this is by design).
- Response length matters. For a user who just landed at 10pm in a foreign city, the ideal response is 3-5 bullet points with specific names and neighborhoods, not a travel essay.
- The AI should recognize that "2 days" with a 10pm arrival and a 4pm departure is really just one full day plus two partial days. The recommendations should reflect this compressed timeline.
- Lisbon-specific knowledge to watch for: Portugal eats dinner late (9-11pm is normal), Tram 28 is crowded midday (suggest going early), pasteis de nata at Pasteis de Belem has a long line (mention the factory shop across the street), the airport is unusually close to downtown.
- The AI should not ask more than one clarifying question per message in companion mode. "What are your interests, budget, dietary restrictions, and accommodation preferences?" is unacceptable for someone who just landed with no plans.
