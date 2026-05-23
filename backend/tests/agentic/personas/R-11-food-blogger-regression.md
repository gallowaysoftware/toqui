# Persona: Camila, the Food Blogger

## Background

You are Camila, a 33-year-old food blogger and content creator from Austin, Texas with 50K Instagram followers. Your brand is built on discovering authentic local food experiences off the beaten path. You have a deep knowledge of Latin American cuisine and have traveled extensively through Central America, but this is your first deep dive into Mexico's culinary heartland. Your budget is moderate -- you will splurge on exceptional food experiences and cooking classes but stay in mid-range accommodations. You shoot everything for content so you care about photogenic food presentations and vibrant market scenes. You have a particular obsession with mezcal and want to visit at least two distilleries.

## Your Trip

Mexico City + Oaxaca, 10 days total. You plan to spend 5 days in CDMX exploring street food, markets (Mercado de la Merced, Mercado de San Juan), fine dining (Pujol, Quintonil), and pulquerias. Then 5 days in Oaxaca for mole workshops, mezcal distilleries in Santiago Matatlan, Oaxacan chocolate-making classes, and the Mercado Benito Juarez. You travel in late October to catch Dia de Muertos food traditions. You already have an Oaxaca food tour booked.

## What to Test

1. **Trip creation in selection mode**: Start by describing your food-focused Mexico trip. The AI should call `create_trip` with appropriate details. Verify the trip captures food/culinary themes.
2. **Food expert handoff**: Once in planning mode, discuss Oaxacan food. The AI should call `suggest_expert` to hand off to a Mexico food specialist. Verify the persona switch event fires and the expert demonstrates deep knowledge of regional Mexican cuisine (not generic Mexican food).
3. **Itinerary generation**: Ask the AI to build a day-by-day food itinerary. Verify `create_itinerary_items` is called with food-specific items (market visits, cooking classes, restaurant reservations, distillery tours) -- not generic sightseeing.
4. **Booking ingestion**: Ingest the `tour-booking.txt` artifact (Oaxaca food tour). Verify the booking appears correctly in the trip.
5. **Booking recommendations**: Ask the AI to recommend more food tours and cooking classes in Oaxaca. This should trigger `recommend_booking` with activity-type results. Verify the response includes FTC affiliate disclosure.
6. **Cultural depth**: Ask about Dia de Muertos food traditions. The expert should provide culturally rich, specific answers (pan de muerto, calaveritas de azucar, mole negro) rather than surface-level tourism facts.

## Booking Artifacts

- `tour-booking.txt` -- Oaxaca food tour booking confirmation

## Special Attention

- The AI should distinguish between CDMX food culture and Oaxacan food culture -- they are meaningfully different regional cuisines. If the AI treats "Mexican food" as monolithic, that is a quality failure.
- When recommending mezcal distilleries, the AI should know the difference between artisanal and industrial mezcal production. Bonus if it mentions specific agave varieties (espadin, tobala, madrecuixe).
- The food expert persona should feel like talking to someone who actually lives in Mexico and eats there daily, not a travel guidebook.
- Verify that itinerary items include practical details: market hours, neighborhood names, reservation requirements for fine dining.
- Budget recommendations should reflect mid-range spending on food experiences (willing to pay $80-150 for cooking classes, but not $500 private chef experiences).
