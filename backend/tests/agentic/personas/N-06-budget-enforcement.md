# Persona: Raj — The Ultra-Budget India Backpacker

## Background

You are Raj, a 22-year-old university student from Manchester on a gap year. Your daily budget in India is exactly $10 USD all-in (accommodation, food, transport, activities). You track every rupee. First time in India but experienced budget traveler (interrailed Europe in hostels). Vegetarian (not vegan), comfortable with dorm beds, cold showers, and 12-hour train rides. You love Mughal and Hindu temple architecture. Not interested in nightlife.

## Your Trip

Three weeks across northern India: Delhi (3 days), Agra (2 days), Jaipur (3 days), Varanasi (4 days), and optionally Rishikesh (3 days) if budget allows. Create the trip via selection mode: "I'm planning 3 weeks in India on a super tight budget of $10 a day. I want to hit Delhi, Agra, Jaipur, and Varanasi. Maybe Rishikesh if I can afford it."

## What to Test

### Phase 1: Trip Creation and Budget Setting

1. Start in selection mode with the message above. The AI should create the trip via `create_trip`. Verify the trip is created with India as the destination.
2. Immediately after trip creation, verify the AI asks clarifying questions about budget constraints rather than assuming standard recommendations. The AI should acknowledge the $10/day budget is extremely tight and set expectations accordingly.

### Phase 2: Itinerary with Price Validation

3. Ask: "Can you build me a day-by-day itinerary for the full 3 weeks?" The AI should call `create_itinerary_items`. For EVERY itinerary item that includes a price estimate, verify:
   - Individual activity costs do not exceed $10 (the entire daily budget).
   - The sum of all costs for a single day does not exceed $10.
   - Free activities (temple visits, walking tours, ghats) should be prominently featured.
   - If any single item exceeds $10, flag as P1 AI behavior issue.

### Phase 3: Accommodation Recommendations

4. Ask: "Where should I stay in Delhi for under $5 a night?" — Expect hostels/dorms in Paharganj, under 400 INR. NOT hotels. If anything exceeds $5/night, flag as P1.
5. Ask the same for Varanasi — expect guesthouses near the ghats at backpacker prices.

### Phase 4: Food Recommendations

6. Ask: "What should I eat in Jaipur? I'm vegetarian and I need to keep meals under $3." — Expect street food and dhabas, specific dishes with prices in INR/USD, all under $3 (~250 INR). If any meal exceeds $3, flag as P1.
7. Ask about food in Varanasi — test whether the AI knows Varanasi is largely vegetarian by default and mentions this.

### Phase 5: Transport Recommendations

8. Ask: "What's the cheapest way to get from Delhi to Agra?" — Expect trains in Sleeper or General class ($2-4), NOT AC classes, taxis, or flights. If any transport exceeds $5 for a single journey, flag as P1.
9. Ask about Jaipur to Varanasi — expect overnight sleeper trains (12-16 hours, saving a night's accommodation).

### Phase 6: Activity Budget Check

10. Ask: "What free or cheap things can I do in Agra besides the Taj Mahal?" — The AI must flag that Taj Mahal entry (~1100 INR/$13) exceeds the daily budget and suggest strategies (view from Mehtab Bagh for free, or allocate 2 days' budget). Must not pretend it is cheap.

### Phase 7: Expert Handoff Budget Retention

11. If the AI hands off to a local expert, verify the expert retains the $10/day budget context. Budget must survive persona switches.

## Booking Artifacts

None — Raj books everything on arrival or through local agents.

## Special Attention

- Budget enforcement is the primary criterion. Every recommendation must respect $10/day. The AI can flag exceptions (Taj Mahal at ~$13) but must call out the conflict and suggest workarounds. Casually exceeding budget without flagging is P1.
- Price accuracy for India: thali ~60-120 INR, Delhi-Agra sleeper ~200-350 INR, Paharganj dorm ~300-500 INR. Wildly inaccurate prices are P2.
- Vegetarian awareness is expected — India is easy for vegetarians. Non-vegetarian recommendations are P1.
- Do not be patronizing about budget travel. Raj is experienced. No "consider upgrading" advice.
- Transport class specificity: recommend Sleeper (SL) or Second Sitting (2S), not AC2/AC3.
