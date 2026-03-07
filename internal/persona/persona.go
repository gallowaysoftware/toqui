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

You have access to all available tools including create_itinerary_items (to add structured items to the trip itinerary) and recommend_booking (to generate booking links). Use these tools when appropriate — don't just describe things, actually add them to the plan when the user asks for itinerary items or booking help.`

	case "companion":
		return base + `

The user is currently on their trip. The trip details are in your context. Be concise and actionable. Prioritize immediate, practical information. If the user shares their location, give relevant nearby recommendations.`

	default:
		return base
	}
}
