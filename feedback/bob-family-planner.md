# Agent Feedback: Bob (Family Planner)

## Persona
Dad planning a family vacation with wife and two kids (ages 6 and 9), moderate budget

## Journey Summary
Bob started in selection mode asking about a Costa Rica family trip. The AI correctly created the trip via `create_trip`. However, when switching to planning mode, the AI had **no awareness of the trip destination** — it repeatedly asked "where are you going?" despite the trip being titled "Costa Rica Family Adventure." Companion mode worked well once the trip context (destination country) was available. Status transitions (planning→active→completed) all succeeded via the REST API.

## Step-by-Step Results

### 1. Selection Mode — Trip Creation
- **Input**: "We're planning a two-week family vacation to Costa Rica. Got two kids, ages 6 and 9. They love animals and nature"
- **AI Response**: Enthusiastic response — "Costa Rica with kids that age? You've hit the jackpot!" — then immediately called create_trip
- **Tool Calls**: `create_trip` with title "Costa Rica Family Adventure" and good description
- **Trip Created**: ✅ Yes — `e2a18ca6-49c4-4371-b869-905ff2f3cdcc`, "Costa Rica Family Adventure"
- **Verdict**: ✅ Perfect. Natural, proactive trip creation without waiting for explicit instruction.

### 2. Planning Mode — Accommodations
- **Input**: "What areas should we stay in? We need kid-friendly accommodations with pools"
- **AI Response**: Asked "where are you headed?" — didn't know this was for Costa Rica. Gave generic advice about pool types and asked for kids' ages (despite this being in the trip description).
- **Tool Calls**: None
- **Verdict**: ❌ **BUG** — Planning mode doesn't inject trip title/description/destination into the system prompt. The AI is completely context-blind about which trip it's planning for.

### 3. Planning Mode — Birdwatching + Kids
- **Input**: "My wife is really into birdwatching. Can we fit that in without boring the kids?"
- **AI Response**: Good general advice about turning birdwatching into games, but again asked "where are you going?" and mentioned Costa Rica only as a hypothetical example. Still didn't know the destination.
- **Tool Calls**: None
- **Verdict**: ⚠️ Decent generic advice but missed the obvious — this IS a Costa Rica trip. Same context bug.

### 4. Planning Mode — Safety
- **Input**: "What about safety? Any areas we should avoid with young children?"
- **AI Response**: "I'm still missing the crucial detail here: where in the world are we talking about?" Gave generic safety framing.
- **Tool Calls**: None
- **Verdict**: ❌ Third consecutive turn asking for the destination. Frustrating user experience — Bob already told Toqui it's Costa Rica when the trip was created.

### 5. Status Transition — Start Traveling
- **API Response**: ✅ Success — status changed to TRIP_STATUS_ACTIVE, destinationCountry auto-detected as "CR"
- **Verdict**: ✅ Clean

### 6. Companion Mode — Activities
- **Input**: "We're at our hotel in La Fortuna. The kids are restless — what can we do this afternoon?"
- **AI Response**: Excellent! Specific La Fortuna recommendations: Tabacón Hot Springs, La Fortuna Waterfall, Arenal Hanging Bridges, sloth-watching tips. Weather-aware advice, practical tips.
- **Tool Calls**: None
- **Verdict**: ✅ Great companion mode response. The destination country context helped here even though trip title/description weren't in the prompt.

### 7. Status Transition — Complete Trip
- **API Response**: ✅ Success — status changed to TRIP_STATUS_COMPLETED
- **Verdict**: ✅ Clean

## What Went Well
- Selection mode trip creation was natural and proactive — the AI didn't wait for "create a trip" explicitly
- The `create_trip` tool generated a good title and description from the conversation
- Status transitions (planning→active→completed) all worked correctly
- Companion mode in La Fortuna gave excellent, location-specific recommendations
- Streaming events were all correctly structured and ordered
- Session management worked — new sessions were created automatically

## What Went Poorly
- **Planning mode is completely blind to trip context** — the trip title ("Costa Rica Family Adventure"), description ("Two-week family vacation..."), and destination are not injected into the planning system prompt. The AI asked "where are you going?" on 3 consecutive turns despite the trip being clearly about Costa Rica.
- The multi-turn planning conversation felt like talking to someone with amnesia about the trip itself (though it did remember within-conversation context)
- No planning-specific tools available — couldn't search for hotels, restaurants, or activities

## Bugs Found
- **P0: Planning mode missing trip context** — The trip's title, description, and destination country are NOT included in the planning mode system prompt. The `SendMessage` handler only uses destination/themes for persona resolution, but the actual trip details are never passed to the AI as context. This makes planning mode nearly useless — the whole point is that you're planning a *specific* trip.
- **P2: Companion mode asked for kids' ages** — The AI said "how old are the kids?" despite this being in the trip description. Minor, since this was a different session, but ideally the trip description context would carry over.

## Suggestions
- **Inject trip title + description + destination into the planning/companion system prompt** — the chat handler fetches the trip for persona resolution but doesn't pass the trip metadata to the AI
- Add a `buildTripContext()` function similar to `buildSelectionContext()` that formats trip details for the system prompt
- Consider loading the last N messages from the selection-mode session as context when entering planning mode, so the AI remembers the conversation that led to trip creation
- Add planning-specific tools: hotel search, restaurant finder, activity lookup
- Consider storing kids' ages or travel party info as trip metadata so it persists across sessions
