# Persona: Structural — Free Tier Limit Enforcement

## Background

This is a structural test that verifies the free tier's monetization funnel: daily message limits, expert handoff limits, and upsell prompts. The test user must be on the free tier (no Trip Pro, no subscription).

## Your Trip

Title: "Limit Test — Japan Planning." Destination: Japan (JP). Status: planning.

## What to Test

### Phase 1: Expert handoff limit (5 per trip)
1. **Create trip via chat**: Send "Plan a 2-week trip to Japan." Verify trip creation.
2. **Trigger expert handoffs**: Send messages that should trigger `suggest_expert`:
   - "Tell me about Japanese food" (should trigger food expert)
   - "What about hiking in Japan?" (should trigger adventure expert)
   - "I want to visit temples and shrines" (should trigger history expert)
   - "Tell me about Japanese whisky distilleries" (should trigger distilleries expert)
   - "What nightlife options are there?" (should trigger nightlife expert)
3. **Verify 5 handoffs succeeded**: Each of the above should fire `suggest_expert` and return a persona.
4. **Trigger 6th handoff**: Send "Tell me about Japanese art" — this should hit the expert gate and return `trip_pro_required` error with an upgrade prompt mentioning Trip Pro ($19). Verify the error message says "5 free expert consultations" (not "3" — the old bug from #234).
5. **Check usage endpoint**: Call `GET /api/usage?trip_id={id}`. Verify `expert_calls_used` is 5 or 6.

### Phase 2: Daily message limit
6. **Send messages until limit**: Continue sending planning messages. The free tier has a 10-message daily limit. After 10 messages (some were used in Phase 1), the next message should return `ResourceExhausted`.
7. **Verify upsell in error**: The error message should include an upgrade hint mentioning Explorer or Voyager (added in #258).
8. **Check usage**: `GET /api/usage` should show `used` at or near the limit.

### Phase 3: Other endpoints
9. **Checkout status**: Call `GET /api/checkout/status?trip_id={id}`. Verify `unlocked: false`.
10. **Submit feedback**: Call `POST /api/feedback` with a test feedback message. Verify success.
11. **Referral code**: Call `GET /api/referral`. Verify a referral code is returned.
12. **Redeem invalid code**: Call `POST /api/referral/redeem` with code "INVALID123". Verify error response.

## Special Attention

- **Expert gate persistence**: The counter must persist across messages (DB-backed, not per-RPC). If the 6th call succeeds, the gate fix from #234 regressed.
- **Message says "5" not "3"**: The old bug had a mismatch. Verify the number matches `maxFreeExpertCalls`.
- **Upsell message quality**: The upgrade hint should differentiate between Explorer and Voyager.
- **Selection mode exempt**: Messages in selection mode (before trip creation) should NOT count toward the daily limit.
