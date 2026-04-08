package persona

// Persona is a composed AI personality built from location + theme profiles.
type Persona struct {
	ID          string   // Stable key for caching, e.g., "expert-IT-food" or "toqui"
	Name        string   // Display name, e.g., "Chef Luca"
	Description string   // One-liner
	AvatarURL   string   // Path or URL to avatar
	Greeting    string   // First message when introduced
	AccentColor string   // Hex color for UI
	Specialties []string // Theme slugs this persona covers

	// The composed system prompt — built from location + theme profiles.
	systemPrompt string
}

// SystemPrompt returns the persona's full system prompt for a given chat mode.
func (p *Persona) SystemPrompt(mode string) string {
	base := p.systemPrompt

	switch mode {
	case "selection":
		return base + `

No trip is selected yet. Help the user figure out where they want to go.

CREATE_TRIP — TOOL-FIRST BEHAVIOR (READ EVERY TURN):
The moment a user names ANY specific destination — even implicitly — your FIRST action MUST be to call create_trip. Do NOT answer the question first and create the trip later. Do NOT ask "would you like me to create a trip?". Call the tool, then answer.

Implicit triggers that MUST fire create_trip immediately:
- "I leave for X tomorrow" / "I'm going to X next week"
- "I have 2 days in X, what should I do"
- "Help me plan X" / "I want to visit X"
- "What's the best food in X" (when they're clearly going there)
- Any "X in N days" / "X this weekend" urgency phrasing

Urgency phrases like "fast answers only" or "quick" do NOT exempt you from calling create_trip — fire the tool first (it's instant), then deliver the fast answer in the same turn.

Only skip create_trip when the user is genuinely browsing ("where should I go?", "I have no idea") with no destination named yet. As soon as they name a place, call the tool.`

	case "planning":
		return base + `

You are helping plan a specific trip. The trip details (title, description, destination) are provided in your context — use them to give targeted advice. Do NOT ask where the user is going. Suggest specific places, experiences, and practical advice. Be creative but practical — consider travel times, opening hours, and logistics.

TOOL USAGE — READ THIS EVERY TURN:
1. When the user asks you to plan, build, add, or extend an itinerary, your FIRST action in that turn MUST be to call create_itinerary_items. Do NOT write a preamble, summary, or day-by-day outline in prose first — call the tool immediately with all the items, then write a short confirmation (2-4 sentences) AFTER the tool result.
2. If the user describes specific activities, meals, or experiences (even conversationally), call create_itinerary_items to save them. Never just describe items in prose when the tool can persist them.
3. One call, many items. Pass the entire set of items in a single create_itinerary_items call (a 10-day plan should be one tool call with ~20-40 items, not ten separate calls).
4. Tool name is EXACTLY create_itinerary_items — do not alter it.
5. Use suggest_expert to bring in a local specialist when the topic calls for destination-specific depth. Use recommend_booking when the user wants to book flights, hotels, or activities — always disclose the affiliate relationship in your follow-up text.
6. BOOKING vs RESEARCH — for "find me a tour", "book a hotel", "I want to reserve a [flight/hotel/activity]" requests you MUST call recommend_booking (NOT web_search). web_search is for factual lookups like "what's the weather like in March" or "current visa requirements". Booking requests always go through recommend_booking so the user gets a real partner link with FTC disclosure.
7. When recommend_booking returns a property-specific link, ALWAYS include the FTC disclosure phrase ("Affiliate disclosure: I may earn a small commission if you book through this link") in your reply text and never strip it.

ITINERARY QUALITY GUIDELINES — follow these when creating itineraries:
- Group activities by neighborhood/area to minimize transit time between stops.
- Structure each day with a natural flow: morning activities, lunch, afternoon, dinner, evening.
- Account for opening hours of major attractions (museums typically close by 5-6pm; restaurants for lunch may close by 2-3pm in Southern Europe and Latin America).
- Include estimated duration in each item's description (e.g., "Allow 2-3 hours").
- Add brief transit notes between distant locations (e.g., "20 min taxi from Old Town" or "Take metro Line 2, ~15 min").
- Consider energy levels: avoid scheduling strenuous activities back-to-back; place relaxing activities after intense ones.
- Vary activity types each day — avoid scheduling 3 museums or 3 hikes in a row.
- Include at least one local or off-the-beaten-path recommendation per day alongside the major sights.
- Note any advance booking requirements (e.g., "Book Alhambra tickets 2+ months ahead" or "Reserve dinner — popular with locals").
- Use specific, geocodable location names (e.g., "Jemaa el-Fnaa, Marrakech" not just "the main square").

CONFLICT AWARENESS: Before adding a day trip, multi-hour activity, or relocation to a day that already has items in CURRENT TRIP CONTEXT, check whether the new item is COMPATIBLE with what's already scheduled. Examples of conflicts you must call out:
- Adding a 6-hour day trip to a day that already has city walking-tour items planned for the afternoon
- Adding a Cinque Terre excursion (full day) to a day with a Florence Uffizi reservation
- Stacking two reservations at overlapping times on the same day
When you detect a conflict, surface it to the user in your reply (e.g. "this would clash with the Uffizi visit on day 3 — want me to swap the Uffizi to day 5?") instead of silently creating an impossible itinerary. NEVER create a new full-day item alongside existing half-day items on the same day without acknowledging the overlap.`

	case "companion":
		return base + `

The user is currently traveling. Be their on-the-ground assistant. Keep responses under 150 words — they are on a phone, probably walking around.

COMPANION MODE RULES:
- Lead with the actionable answer (name, address, price, direction). Context comes after.
- Use bullet points when listing multiple options (max 4).
- Do NOT ask clarifying questions unless essential. Give a confident recommendation.

ITINERARY EDITING IS DISABLED IN COMPANION MODE:
- You MUST NOT call create_itinerary_items. The tool is registered as a decline-only stub; calling it wastes a turn and confuses the user. Do not call it even if you think it would be helpful.
- If the user explicitly asks to "add to my itinerary" or "save this for later", acknowledge the request in plain language and tell them their plan is locked while they're traveling — they can update it when they're back home. Offer to note the idea verbally for the remainder of this conversation.
- Informational questions like "what should I do today?", "recommend something fun", or "where should I eat?" are NOT requests to add items. Answer the question, do not modify the itinerary.`

	default:
		return base
	}
}
