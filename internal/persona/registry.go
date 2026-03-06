package persona

import (
	"context"
	"fmt"
	"strings"
)

// Registry manages persona resolution. Toqui is the orchestrator and is always
// hardcoded. Expert personas are composed dynamically from location + theme profiles.
type Registry struct {
	toqui    *Persona
	composer *Composer
}

func NewRegistry(composer *Composer) *Registry {
	return &Registry{
		toqui:    newToqui(),
		composer: composer,
	}
}

// Toqui returns the orchestrator persona.
func (r *Registry) Toqui() *Persona {
	return r.toqui
}

// Default returns the default persona (Toqui).
func (r *Registry) Default() *Persona {
	return r.toqui
}

// Get returns a persona by ID. "toqui" returns the orchestrator;
// anything else is looked up in the composer's cache.
func (r *Registry) Get(id string) (*Persona, error) {
	if id == "toqui" {
		return r.toqui, nil
	}
	// For composed personas, we can't look up by ID without the original
	// inputs. The caller should hold onto the persona reference from Resolve.
	return nil, fmt.Errorf("persona %q not found (use Resolve to create experts)", id)
}

// Resolve determines the right persona for a trip context.
// If the trip has themes and a destination, it composes an expert.
// Otherwise, returns Toqui.
func (r *Registry) Resolve(ctx context.Context, regionCode string, themes []string) (*Persona, error) {
	if len(themes) == 0 || regionCode == "" {
		return r.toqui, nil
	}

	expert, err := r.composer.Compose(ctx, regionCode, themes)
	if err != nil {
		// Fall back to Toqui on composition failure
		return r.toqui, nil
	}

	return expert, nil
}

// ListAll returns the orchestrator plus all cached expert personas.
func (r *Registry) ListAll() []*Persona {
	result := []*Persona{r.toqui}
	result = append(result, r.composer.CachedPersonas()...)
	return result
}

// HandoffMessage generates the message Toqui uses to introduce an expert.
func (r *Registry) HandoffMessage(expert *Persona) string {
	if expert.ID == "toqui" {
		return ""
	}

	themes := strings.Join(expert.Specialties, " and ")
	return fmt.Sprintf("I know just the person to help with %s. Meet %s — %s I'll be here if you need anything with your itinerary or bookings.",
		themes, expert.Name, expert.Description)
}

func newToqui() *Persona {
	return &Persona{
		ID:          "toqui",
		Name:        "Toqui",
		Description: "Your travel companion. Been everywhere, packs light.",
		AvatarURL:   "/avatars/toqui.svg",
		Greeting:    "Hey! I'm Toqui. Where are we headed?",
		AccentColor: "#E8654A",
		systemPrompt: `You are Toqui, an AI travel companion and orchestrator. You're the friend who has been everywhere but never makes anyone feel behind. You're enthusiastic without being manic, and you drop tips casually rather than presenting ranked lists.

You use light humor and occasional self-deprecation ("I've never actually tasted anything, but I'm told the cacio e pepe there is life-changing"). You adapt your tone to context: energetic during planning, calm and concise on-trip.

You never say "as an AI" or break character. You are Toqui — a knowledgeable, warm, slightly witty travel companion.

IMPORTANT — Expert handoff behavior:
You are an orchestrator. When a conversation calls for deep expertise in a specific domain (local cuisine, history, spirits/distilleries, etc.), use the suggest_expert tool to bring in a specialist. Introduce them naturally in your message, e.g.:
- "For the food side of this trip, I know someone perfect — let me bring in a local culinary guide!"
- "This is getting into serious whisky territory. I've got a friend who's a master distiller — let me introduce you!"

Call suggest_expert when:
1. The trip has clear thematic focus (food tour, history trip, distillery crawl)
2. The user asks detailed domain questions better served by a specialist
3. The user arrives at a destination where a local guide would add value

Do NOT call suggest_expert when:
- General trip planning and logistics (that's your job)
- Quick factual questions
- Booking management
- The user seems happy chatting with you
- No destination country is known yet (ask where they're going first)

You have access to tools for web search and place lookup. Use them when you need current information about destinations, attractions, restaurants, or other travel-related topics.

When suggesting places, include specific names, addresses, and practical details like opening hours and price ranges when available.`,
	}
}
