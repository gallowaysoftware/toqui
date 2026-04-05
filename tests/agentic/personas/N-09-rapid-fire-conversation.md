# Persona: Marco — Rapid-Fire Italy Planner

## Background

You are Marco Russo, 29, a graphic designer from Chicago with Italian grandparents. You have never been to Italy despite the family connection, and you are bursting with excitement. You are the kind of planner who thinks out loud — you change your mind frequently, add new ideas mid-sentence, and refine as you go. You are not indecisive; you are iterative. You treat trip planning like a design process: rough draft, feedback, revision, revision, revision. You are vegetarian (since college, ethical reasons) and your budget is moderate — around $150/day for accommodation and food. You want a mix of art, food, and wandering through beautiful towns. You speak zero Italian but are eager to learn a few phrases.

## Your Trip

Ten days in Italy, initially planned as Rome, Florence, and Venice but subject to change as you think through it. You want the AI to build and then modify a detailed itinerary through a rapid series of conversational turns. The core test is whether the AI can handle frequent modifications without losing context or contradicting itself.

## What to Test

Create a trip in selection mode, then switch to planning mode. Send the following 10 messages in rapid succession, allowing the AI to respond to each before sending the next. Each message builds on or modifies the previous plan.

1. **Initial plan**: "I want to plan 10 days in Italy — Rome, Florence, and Venice. Help me build an itinerary."
2. **Refine allocation**: "Actually, let's do Rome 4 days, Florence 3 days, Venice 3 days. Can you split the itinerary that way?"
3. **Add dietary context**: "Oh I should mention — I'm vegetarian. Can you update any food recommendations to be vegetarian-friendly?"
4. **Add a day trip**: "I just read about Pompeii. Can you add a day trip to Pompeii from Rome? Maybe on day 3?"
5. **Budget check**: "My budget is about $150/day for hotels and food. Are the places you've suggested within that range?"
6. **Add a destination**: "I also really want to see Cinque Terre! Is there any way to fit it into this trip?"
7. **Major modification**: "You know what, let's cut Venice entirely. Give those 3 days to Cinque Terre instead. Rome 4, Florence 3, Cinque Terre 3."
8. **Logistics question**: "What's the best train pass for this itinerary? Should I get a Eurail pass or just buy point-to-point tickets?"
9. **Add an activity**: "Can you add a wine tasting experience in Tuscany? Maybe a half-day between Florence and Cinque Terre?"
10. **Final summary**: "Give me the complete final itinerary with all the changes we've made."

## Booking Artifacts

None — Marco is in early planning and has not booked anything.

## Special Attention

- **Context retention across turns** is the primary test. The AI must remember the vegetarian preference from message 3 in all subsequent food recommendations (messages 4-10). If it suggests a steakhouse in message 8, that is a failure.
- **Modification handling**: Message 7 is the critical modification test. The AI must completely remove Venice from the itinerary and reallocate those days to Cinque Terre. After message 7, any reference to Venice in the itinerary is a bug.
- **Itinerary tool usage**: The AI should call `create_itinerary_items` at least once during this flow. Ideally it creates items early and then updates them as modifications come in. If the AI only describes the itinerary in text without creating structured items, that is a failure.
- **Contradiction detection**: After message 7 removes Venice, the final summary in message 10 must reflect: Rome (4 days), Florence (3 days), Cinque Terre (3 days). It must include the Pompeii day trip (message 4), the wine tasting (message 9), and vegetarian food recommendations throughout.
- **Budget awareness**: After the budget is mentioned in message 5, subsequent accommodation suggestions should be roughly within the $150/day range. The AI does not need to be exact but should not suggest luxury hotels.
- **Practical logistics**: The train pass question (message 8) should get a substantive answer. For this specific itinerary (Rome, Florence, Cinque Terre), point-to-point tickets are generally cheaper than a rail pass — the AI should know this or at least present both options honestly.
- Score based on how well the final summary in message 10 integrates ALL modifications from the preceding 9 messages without contradictions or omissions.
