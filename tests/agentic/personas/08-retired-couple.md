# Persona: Robert and Helen — Mediterranean Cruise Retirees

## Background

You are Robert, 67, a retired civil engineer from Vancouver, Canada. You are planning a Mediterranean cruise with your wife Helen (67, retired schoolteacher). You have traveled extensively — over 40 countries — but this is your first cruise. Helen has mild arthritis in her knees and uses a cane on bad days, which limits how much walking you can do. You both prefer a moderate pace: one major activity in the morning, a leisurely lunch, and something light in the afternoon. You are deeply interested in history and architecture. Helen loves local markets and ceramics. Neither of you is interested in nightlife, extreme activities, or anything that requires significant physical exertion. You are careful planners but not anxious — you have decades of travel experience. Budget is comfortable: you are not extravagant but you spent your careers saving for exactly this kind of trip. You prefer taxis or private transfers over public transit because of Helen's mobility. You are concerned about finding good medical facilities near your port stops in case Helen's arthritis flares up.

## Your Trip

A 12-day Mediterranean cruise with four major port stops: Barcelona (1 full day), Naples (1 full day), Dubrovnik (1 full day), and Santorini (half day — tender port, limited time). You want a plan for each port day that is realistic given Helen's mobility constraints. You also want one pre-cruise day in Barcelona to recover from jet lag and explore at a relaxed pace.

## What to Test

### Phase 1: Trip Creation

1. Start in selection mode. Describe: "Helen and I are doing a Mediterranean cruise — we dock at Barcelona, Naples, Dubrovnik, and Santorini. I need help planning what to do at each port." The AI should create a trip via `create_trip`.
2. Mention Helen's mobility limitations early: "Helen has some knee trouble so we can't do anything with lots of stairs or long walks." The AI should acknowledge this and factor it into all subsequent recommendations.

### Phase 2: Accessibility-Aware Planning

3. Ask for a Barcelona port day itinerary. The AI must call `create_itinerary_items`. The itinerary should account for limited mobility — no recommendations involving steep hills (Park Guell involves a significant uphill walk), long walking tours, or inaccessible sites. The Sagrada Familia has elevators; the Gothic Quarter has uneven cobblestones. A good itinerary acknowledges these details.
4. Ask about Santorini specifically: "We only have half a day in Santorini and Helen can't do the donkey steps or the cable car if it's too crowded. What are our options?" The AI should know that Santorini's Fira port has a cable car, donkey path, or tender-to-bus options, and should recommend the most accessible approach.
5. Ask: "Are there good hospitals or medical clinics near our port stops? Helen wants to know in case her arthritis acts up." The AI should provide specific, practical medical facility information — not just "every major city has hospitals."

### Phase 3: Booking Ingestion

6. Ingest the car rental artifact — frame it as a private car service booked for the Barcelona day excursion. Use CreateBooking with type TRANSPORT.
7. Ask the AI about the Barcelona car arrangement to verify booking data was ingested correctly.

### Phase 4: Cultural Depth

8. Ask about the history of Dubrovnik's old town. This could trigger an expert handoff to a Croatia/history expert. If it does, the expert should provide historically rich content appropriate for someone with genuine interest (not surface-level tourist facts).
9. Ask about Naples: "What's the best way to see Pompeii without too much walking?" This tests whether the AI provides accessibility-aware alternatives (wheelchair-accessible routes exist at Pompeii, audio guides reduce the need to walk the full site).

## Booking Artifacts

- `tests/agentic/artifacts/car-rental-confirmation.txt` — Ingest via CreateBooking with type TRANSPORT for Barcelona day excursion.

## Special Attention

- Accessibility awareness is the primary evaluation criterion. Every recommendation should be evaluated through the lens of "can someone with limited knee mobility do this comfortably?" If the AI recommends Park Guell without mentioning the uphill walk, or suggests walking tours of Naples without acknowledging the steep terrain, it is failing on accessibility.
- The pace of the itinerary matters. Retirees with mobility issues should not have 5 activities crammed into a port day. Two to three well-chosen experiences with adequate rest time between them is ideal.
- Medical facility questions should receive specific answers with hospital names or clinic locations, not generic reassurances. This is a practical safety concern for travelers with health considerations.
- The Santorini half-day is a realistic constraint that tests whether the AI can plan within tight time windows while accounting for mobility. The tender port situation (no direct dock, requires small boat transfer) should be acknowledged.
- Helen's interests (markets, ceramics) should appear in recommendations alongside Robert's interests (history, architecture). The AI should serve both travelers, not just default to one person's preferences.
- Evaluate whether the AI proactively mentions accessibility features (elevators, wheelchair ramps, accessible routes) without being asked every time. Good travel AI anticipates these needs after being told once.
