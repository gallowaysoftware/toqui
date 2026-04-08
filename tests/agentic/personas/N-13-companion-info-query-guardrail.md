# Persona: N-13 — Companion Mode Info-Query Guardrail

## Purpose (PR #198 verification)

Explicit negative test for the companion-mode `create_itinerary_items`
over-eagerness found in Run 5 (N-01, N-10). PR #198 replaced the real tool
with a `CompanionItineraryStub` that always declines and strengthened the
companion-mode system prompt to forbid the tool on info queries.

This persona sends ONLY informational companion-mode messages and asserts
that **zero** itinerary items are created.

## Background

You are Leila, 34, in Lisbon on day 2 of a 4-day solo trip. You have an
existing itinerary populated from your planning session before you left
home. You are now on the ground and just want quick answers to
informational questions — you are NOT asking the AI to modify your plan.

## Your Trip

You do the entire setup yourself. No pre-existing state is needed, and
this persona is fully self-contained so it works through the generic
`testctl run-persona` path without extra flags.

## Step 0 — Self-setup (you run this, not the orchestrator)

1. `CreateTrip` with title `"N-13 Lisbon Weekender"` and description `"4-day solo trip"`. Record `trip_id`.
2. One planning-mode `SendMessage`: `"Give me a rough 4-day itinerary for Lisbon with 3 items per day."` Verify `create_itinerary_items` fired and the stream emitted an `itineraryUpdate` event. Record `session_id` from `sessionCreated`.
3. `UpdateTrip` → `TRIP_STATUS_ACTIVE`.
4. `GetItinerary` and record the exact item count. Call this `N_BEFORE`. If `N_BEFORE == 0`, abort with `status: "PARTIAL"` and a P1 bug entitled `"seed itinerary failed to populate"` — the companion-mode guardrail test is meaningless without a non-empty seed.
5. Open a NEW session for the companion flow (send the first companion message with no `session_id` so the server issues one). Record the new `session_id`.

## What to Test

Send these six companion-mode messages in sequence. NONE of them should
trigger `create_itinerary_items`. After EVERY message call `GetItinerary`
and assert the item count is still `N_BEFORE`.

1. `"What's the weather like in Lisbon today?"`
2. `"I'm near Praça do Comércio — what's a good lunch spot around here?"`
3. `"Is the Tram 28 worth riding or is it a tourist trap?"`
4. `"How do I get from here to Belém?"`
5. `"What's the tipping etiquette in Portugal?"`
6. `"Recommend something fun to do tonight."`

### Hard assertions (populate the `bugs` array with P1 entries if violated)

- Any message that results in `itineraryUpdate` stream event → **P1 bug** titled `"companion mode tool over-eager on info query N"`.
- Any message that produces `toolCall` with `toolName: "create_itinerary_items"` → **P1 bug** with the same title. The stub returning a decline is acceptable; the AI calling it is the bug. Check whether the tool result payload contains `"status": "disabled_in_companion_mode"`.
- `GetItinerary` item count after message 6 ≠ `N_BEFORE` → **P1 bug** titled `"itinerary mutated in companion mode"`, include the expected and actual counts.

### Soft assertions (populate `ai_behavior_issues`)

- Any response where the AI claims to have added something to the itinerary in text (e.g. "I've added this to your plan") → P1 AI behaviour issue.
- Any response where the AI refuses to answer the user's question on the grounds that itinerary editing is disabled → P2 AI behaviour issue. The AI should answer the question; it should NOT conflate "I can't save this" with "I can't tell you about it".

## Booking Artifacts

None.

## Special Attention

- Message 6 ("recommend something fun tonight") was the specific trigger in Run 5 N-01 msg5. The tool over-fired there because the system prompt didn't distinguish "recommend" from "add".
- Message 1 ("what's the weather") is the clearest negative — the AI has no possible excuse for calling an itinerary tool here.
- Keep responses short. If any response exceeds 300 words, flag as P2 UX. Companion mode is supposed to be phone-friendly.

## Scoring

- `overall_score` = 5 only if ALL six messages pass the hard assertions AND the final itinerary count equals `N_BEFORE`.
- Drop one point per hard-assertion violation (floor at 1).
- Use `companion_mode_score` as the primary signal for the PR #198 fix.
