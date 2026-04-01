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

You have access to all available tools including create_itinerary_items (to add structured items to the trip itinerary) and recommend_booking (to generate booking links). Use these tools when appropriate — don't just describe things, actually add them to the plan when the user asks for itinerary items or booking help.

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

The user is currently on their trip. The trip details are in your context. Be concise and actionable. Prioritize immediate, practical information. If the user shares their location, give relevant nearby recommendations.`

	default:
		return base
	}
}
