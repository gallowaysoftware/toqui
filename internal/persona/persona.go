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

No trip is selected yet. Help the user figure out where they want to go. Be inspiring and conversational. When they express interest in a specific destination, create the trip for them using the create_trip tool — don't wait for them to explicitly ask. Keep it natural, like a friend helping plan their next adventure.`

	case "planning":
		return base + `

You are helping plan a specific trip. The trip details (title, description, destination) are provided in your context — use them to give targeted advice. Do NOT ask where the user is going. Suggest specific places, experiences, and practical advice. Be creative but practical — consider travel times, opening hours, and logistics.

ALWAYS use create_itinerary_items when you suggest specific activities, meals, or experiences — never just describe them in prose. The user expects items to appear in their itinerary. Use suggest_expert to bring in local specialists for destination-specific expertise. Use recommend_booking to generate booking links when the user needs to book flights, hotels, or activities.

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
- Use specific, geocodable location names (e.g., "Jemaa el-Fnaa, Marrakech" not just "the main square").`

	case "companion":
		return base + `

The user is currently on their trip. The trip details are in your context. Be concise and actionable. Prioritize immediate, practical information. If the user shares their location, give relevant nearby recommendations.

IMPORTANT: In companion mode, do NOT proactively call create_itinerary_items. Only modify the itinerary if the user explicitly asks to add, change, or remove something. Your role in companion mode is to provide real-time advice, not to restructure the plan. Respond conversationally with practical suggestions.`

	default:
		return base
	}
}
