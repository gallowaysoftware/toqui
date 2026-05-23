# Agent Feedback: Carol (Returning User)

## Persona

Experienced user with existing trips, testing select_trip, multi-trip switching, and creating new trips alongside old ones

## Journey Summary

Carol tested the returning-user flow with 2 existing trips (Morocco active, Greece planning). The `select_trip` tool worked perfectly for vague references — "my Greece trip" was correctly matched. Planning mode on the Greek Islands trip gave excellent, detailed responses with real place names. However, switching back to Morocco via selection mode **failed** because an `UpdateTrip` bug had wiped the trip's title and description, making it invisible to the AI. Companion mode in Marrakech worked well. Creating a new trip (New Zealand) via selection mode worked perfectly. Trip count verified at 3.

## Step-by-Step Results

### 1. Selection Mode — Vague Trip Reference ("Greece trip")

- **Input**: "hey, what's happening with my Greece trip?"
- **AI Response**: No conversational text — went straight to tool call
- **Tool Calls**: `select_trip` with `trip_id: "2fd1009a-69ef-4a14-8a07-bed97db84b6e"` ✅ correct ID
- **Trip Selected**: ✅ Yes — "Greek Islands Hopping"
- **Verdict**: ✅ Perfect matching. AI correctly identified "Greece trip" → "Greek Islands Hopping" from the trip list. The `TripSelected` event was emitted with full trip data.
- **Note**: The AI didn't say anything conversational before calling the tool — just selected silently. Minor UX issue — user might prefer "Let me pull up your Greek Islands trip!" before the navigation.

### 2. Planning Mode — Santorini Spots

- **Input**: "I want to spend most of my time in Santorini. What are the must-see spots?"
- **AI Response**: Extremely detailed, well-organized response covering Oia, Fira, Pyrgos, Megalochori, Red Beach, Vlychada Beach, Akrotiri Archaeological Site, wine recommendations (Estate Argyros, Sigalas), practical tips (ATV rental, dinner reservations, Lucky's Souvlakis).
- **Tool Calls**: None
- **Verdict**: ✅ Excellent. Rich, opinionated, practical advice. The response felt like talking to a friend who's been there multiple times.

### 3. Planning Mode — Mykonos Nightlife

- **Input**: "What about nightlife in Mykonos? I heard it's wild"
- **AI Response**: Comprehensive nightlife guide — Paradise Beach clubs, Super Paradise, Little Venice bars, Cavo Paradiso, practical rhythm guide (beach clubs → sunset → dinner → clubs), budget warning (€100+/night).
- **Tool Calls**: None
- **Verdict**: ✅ Great multi-turn continuity. Maintained the Greek Islands context perfectly. Response was fun, detailed, and practical.

### 4. Selection Mode — Switch to Morocco

- **Input**: "actually, take me to the Morocco one instead"
- **AI Response**: "I don't see a Morocco trip in your current list — you have a Greek Islands trip and one other trip that doesn't have a title showing." Offered to create a new Morocco trip instead.
- **Tool Calls**: None (no `select_trip` called)
- **Trip Selected**: ❌ No — AI couldn't find Morocco
- **Verdict**: ❌ **BUG** — The Morocco trip's title and description were wiped to empty strings by the `UpdateTrip` call (when we set status to ACTIVE). The `buildSelectionContext` function lists trips as "- {title} (id: {uuid}, status: {status})" — with an empty title, the AI sees a blank entry and can't match "Morocco" to anything.
- **Root Cause**: `UpdateTrip` SQL unconditionally sets ALL fields. Sending `{"id":"...","status":"TRIP_STATUS_ACTIVE"}` sets title="" and description="" because protobuf zero values are indistinguishable from "not set."

### 5. Companion Mode — Lost in Marrakech

- **Input**: "I'm lost in the Marrakech medina and I can't find my riad. Help!"
- **AI Response**: Practical emergency advice — look for Koutoubia Mosque minaret, ask for Jemaa el-Fnaa, take a photo of current location, asked for riad name. Cultural awareness (French/English speaking locals).
- **Tool Calls**: None
- **Verdict**: ✅ Excellent companion mode response. Calm, practical, actionable. Exactly what you'd want when lost. Didn't panic, gave step-by-step guidance.

### 6. Selection Mode — Create New Zealand Trip

- **Input**: "You know what, I also want to plan something for New Zealand"
- **AI Response**: Enthusiastic response about NZ landscapes, asked about North/South Island preferences, then proactively called create_trip.
- **Tool Calls**: `create_trip` with title "New Zealand Adventure", description "Exploring the stunning landscapes and adventures of New Zealand"
- **Trip Created**: ✅ Yes — `449015bf-2da8-4a65-b132-18aa15d063d1`, "New Zealand Adventure"
- **Verdict**: ✅ Natural flow — conversational engagement before creating the trip.

### 7. Trip List Verification

- **Total trips**: 3 ✅
- **Expected**: 3 (Morocco, Greece, New Zealand)
- **Actual**: Morocco (no title, active, MA), Greek Islands (planning, GR), New Zealand (planning, no country yet)
- **Verdict**: ⚠️ Count is correct, but Morocco trip is damaged (missing title/description) and New Zealand hasn't been tagged with a destination country yet.

## What Went Well

- `select_trip` tool correctly matched "my Greece trip" → "Greek Islands Hopping" from a vague natural language reference
- `TripSelected` event was properly emitted and contained full trip data
- Planning mode gave rich, detailed, opinionated responses for Santorini and Mykonos — felt like a real travel expert
- Multi-turn planning maintained excellent conversational continuity within a session
- Companion mode "lost in Marrakech" gave calm, practical, culturally-aware emergency guidance
- `create_trip` for New Zealand was natural — AI engaged conversationally before creating
- Session management worked correctly across all mode switches
- Trip count verification passed

## What Went Poorly

- **Morocco trip data wiped by UpdateTrip** — setting status to ACTIVE zeroed out title and description, making the trip invisible to `select_trip` matching
- When the AI couldn't find Morocco, it didn't try fuzzy matching on destination_country (MA) — it only looked at titles
- **Step 1 had no conversational response** — the AI called `select_trip` silently without saying anything to the user first. The frontend would just navigate with no explanation.
- New Zealand trip wasn't auto-tagged with a destination country in the ListTrips response (may be async/delayed)

## Bugs Found

- **P0: UpdateTrip wipes unset fields** — `UpdateTrip` SQL sets ALL columns unconditionally. Calling `UpdateTrip(id, status=ACTIVE)` with empty title/description proto fields zeroes out the stored title and description. This is a data-loss bug. Fix: use partial updates (only SET fields that are explicitly provided), or use field masks.
- **P1: buildSelectionContext doesn't show destination country** — The trip list shown to the AI only includes title, ID, status, and description. If the title is empty (due to the UpdateTrip bug), the AI can't identify the trip even though `destination_country: "MA"` is available. The selection context should include the destination country as a fallback identifier.
- **P2: Silent select_trip** — When the AI calls `select_trip` on step 1, it emits no text to the user. The `TripSelected` event fires and the frontend navigates, but there's no "Let me pull up your Greek Islands trip!" message. The system prompt should instruct the AI to acknowledge the selection before calling the tool.

## Suggestions

- **Fix UpdateTrip to use partial updates** — Only update columns whose values are explicitly set (non-zero). Use proto field masks or check for empty strings before including in the SQL SET clause.
- **Add destination_country to selection context** — `buildSelectionContext` should include `destination: Morocco (MA)` alongside the title so the AI can match trips even with missing titles.
- **Instruct AI to acknowledge before tool calls** — The selection system prompt should say "Always tell the user which trip you're selecting before calling select_trip" to avoid silent navigation.
- **Add trip metadata to planning/companion system prompt** — Same bug as Bob found: the AI in planning mode doesn't know the trip title, description, or destination. It should be injected.
- **Consider field masks for UpdateTrip** — Add `google.protobuf.FieldMask update_mask` to the proto so clients explicitly declare which fields they're updating.
