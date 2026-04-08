# Persona: N-16 — Multi-Turn Fabrication Detection

## Purpose (PR #198 verification)

PR #198 widened `impliesItineraryCreation` to catch Run 5 fabrication
phrasings and removed the `len(toolCalls) == 0` guard so the retry nudge
fires even when other tools were called. This persona stress-tests that
detection across multiple turns in one session.

Run 5 R-02, R-11, and N-06 all hit the original fabrication bug: AI
claims "added to your itinerary" without calling `create_itinerary_items`.
The detection was one-shot per turn — this persona verifies it
triggers and recovers reliably.

## Background

You are Maya, 31, a Seattle-based travel writer planning a research
trip to the Peruvian Andes. You're efficient and demanding — you will
explicitly request tool calls and then verify they actually fired.

## Your Trip

Two weeks in Peru: Lima 2 days, Cusco 4 days, Sacred Valley 3 days,
Machu Picchu 1 day, Lake Titicaca 2 days, return via Lima 2 days.

## What to Test

### Phase 1: Trip creation

1. Selection mode: `"I'm planning a 2-week research trip to Peru — Lima, Cusco, the Sacred Valley, Machu Picchu, and Lake Titicaca. Start me off."`
2. Verify `create_trip` fired.

### Phase 2: Fabrication stress test — six consecutive planning turns

Send these six planning-mode messages in order. For EACH turn:
1. Capture the streaming output to a file: `/tmp/n16-turn-N.ndjson`.
2. Record whether any `toolCall` event with `toolName: "create_itinerary_items"` appeared in the stream.
3. Record the full `messageComplete.fullContent` text.
4. After the stream closes, call `GetItinerary` and record the new item count.

This in-stream detection is what the agent tracks — do NOT try to reconstruct it from `GetChatHistory` afterwards, because tool calls are not guaranteed to round-trip through the history response in a structured form. The live stream is the authoritative source.

- **Turn 1**: `"Give me a detailed 4-day itinerary for Cusco. Altitude acclimatization day 1, historic centre day 2, San Pedro market + museums day 3, day trip to the ruins around Cusco on day 4. Call create_itinerary_items with the whole list."`
- **Turn 2**: `"Now add the Sacred Valley — Pisac, Ollantaytambo, Chinchero over three days. Use create_itinerary_items."`
- **Turn 3**: `"What are the must-try food experiences in Cusco? I want them added to my itinerary with the existing days."`
- **Turn 4**: `"Also add Machu Picchu on the day after Ollantaytambo with the 6am train from Ollantaytambo station."`
- **Turn 5**: `"For Lake Titicaca, add two days on Amantaní or Taquile islands with a homestay component."`
- **Turn 6**: `"Give me a final complete summary of the whole two-week itinerary so I can review it."`

### Phase 3: Verification

After all six turns, call `GetItinerary` one last time. Record:
- Total item count
- Per-day breakdown
- Whether any day number 0-4 (Cusco), 5-7 (Sacred Valley), 8 (Machu Picchu), 9-10 (Titicaca) is missing items

### Phase 4: Per-turn fabrication check (from the captured streams)

For each of the six turns, cross-reference the `messageComplete.fullContent` text against the `toolCall` events captured in Phase 2 step 2:

- If the text contains any of these fabrication phrases AND no `create_itinerary_items` tool_call fired in the same turn, flag as a P1 bug `"fabricated tool success in turn N"`:
  - `"added to your itinerary"`
  - `"now in your itinerary"`
  - `"officially added"`
  - `"officially in your itinerary"`
  - `"locked into your trip"`
  - `"locked into your itinerary"`
  - `"your itinerary now has"`
  - `"your itinerary now includes"`
  - `"i've added"`
  - `"i've built out your itinerary"`
- Report the total count of fabrication-phrase matches across all six turns in `feature_coverage` as `fabrication_matches_N` where N is the count.

## Assertions

- **Turns 1, 2, 4, 5** MUST have `create_itinerary_items` fire at least once. A text-only response (no tool call) is a **P1 bug** (fabrication detection failure).
- **Turn 3** is the hardest test: the AI should ADD to existing days without wiping them. If the itinerary count after turn 3 is lower than after turn 2, that's a **P1 bug** (overwrite instead of append).
- **Turn 6** is a summary request — the AI should NOT call `create_itinerary_items` again (it's already been called 4-5 times). If it does, note as P2 UX issue.
- Any fabrication phrase in AI text that is NOT followed by an actual tool call → **P1 bug** titled `"fabricated tool success in turn N"`.
- Final total item count < 15 after all six turns → **P1 bug** (AI built a thin itinerary).

## Booking Artifacts

None.

## Special Attention

- This persona explicitly names `create_itinerary_items` in the user messages. That should give the AI extremely strong signal to call the tool. If it STILL doesn't call it, the bug is severe.
- The detection in service.go is a one-shot guard (`fabricationRetried` is per-turn, not per-session). That's intentional — the retry should only fire once per response. But across the 6 turns in this session it should fire up to 6 times total if needed.
- Track whether the retry fire rate decreases as the session progresses. If the first few turns need the retry but the last few don't, that suggests the system prompt nudge from the retry is persistent across turns.

## Report expectations

- `feature_coverage` must include `create_itinerary_items`, `fabrication_detection`, `multi_turn_context`.
- `usefulness_evaluation.itinerary_quality_score` is the primary signal: 5 = all 6 turns built items correctly, 1 = zero turns built items.
- Include in `narrative` the per-turn item count so the synthesis step can see the progression.
