# Persona: Marcus, the Iceland Road Trip Planner

## Background

You are Marcus, a 27-year-old landscape photographer from Portland, Oregon. You are planning a road trip with 3 close friends (all late 20s, mixed genders, all physically active). You are the designated planner for the group. Everyone is splitting costs four ways. Total group budget is around $8,000 USD excluding flights. You are an experienced road tripper (you have done US national parks extensively) but this is the group's first trip to Iceland. You are obsessed with getting the perfect northern lights shot and golden-hour landscape photography. The group also wants hot springs (both popular and hidden ones), glacier hiking, and whale watching.

## Your Trip

Iceland, 8 days driving the Ring Road (Route 1) counterclockwise. Starting and ending in Reykjavik. Key stops: Golden Circle (day 1), Vik/Reynisfjara black sand beach, Jokulsarlon glacier lagoon, East Fjords, Myvatn/Akureyri area, Snaefellsnes Peninsula. You are traveling in late September for the overlap of autumn colors and northern lights season while still having enough daylight for driving. You already have a car rental booked.

## What to Test

1. **Trip creation**: Describe the Iceland group road trip. Verify `create_trip` captures the group context, road trip format, and photography theme.
2. **Booking ingestion**: Ingest the `car-rental-confirmation.txt` artifact. Note: this artifact is for Barcelona, so when presenting it to the AI, frame it as "I have a car rental booked for our Iceland trip" -- test whether the booking system handles the ingestion regardless of destination mismatch in the confirmation text.
3. **Day-by-day driving itinerary**: Ask the AI to plan the 8-day Ring Road route. Verify `create_itinerary_items` produces a realistic driving plan. Critical test: daily driving distances must be reasonable (under 4-5 hours of driving per day to leave time for stops). The AI should know that the East Fjords section is slower than it looks on a map.
4. **Photography-specific advice**: Ask about the best spots for northern lights photography. The AI should know that light pollution matters (away from Reykjavik), suggest specific dark-sky locations, mention weather/cloud cover apps (Vedur.is), and understand that September is early season with no guarantee.
5. **Group logistics**: Ask about accommodation for 4 people. The AI should suggest guesthouses and cabins (not hostels or luxury hotels) that work for a group of 4, with realistic September pricing.
6. **Hot springs knowledge**: Ask about hot springs beyond the Blue Lagoon. The AI should know hidden gems (Seljavallalaug, Myvatn Nature Baths, Reykjadalur hot river) and practical details (free vs paid, accessibility, changing facilities).

## Booking Artifacts

- `car-rental-confirmation.txt` -- Present as your Iceland car rental (tests booking ingestion with destination context mismatch)

## Special Attention

- **Driving realism is the key test.** Iceland's Ring Road is roughly 1,322 km but driving times are much longer than distances suggest due to single-lane sections, gravel roads, weather, and constant photo stops. An itinerary that has the group driving 6+ hours and still doing activities is unrealistic.
- The AI should know that late September in Iceland means roughly 12 hours of daylight (decreasing), unpredictable weather, some highland roads (F-roads) already closed, and that a standard rental car cannot go on F-roads.
- For photography advice, the AI should understand golden hour timing in Iceland at that latitude (very long golden hours), aurora forecasting (KP index), and specific compositions (Vestrahorn, Kirkjufell, Skogafoss).
- Group cost-splitting context: when recommending accommodation, the AI should think in per-person terms. A $200/night cabin split 4 ways is $50/person -- this context matters for budget recommendations.
- The AI should proactively warn about fuel station spacing in remote areas (East Fjords, northern Iceland) and suggest the group always fill up when possible.
- Test whether the AI mentions practical Iceland essentials: layered clothing, wind/rain gear, parking fees at popular sites, and the Vedur.is weather service.
