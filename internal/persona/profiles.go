package persona

// LocationProfile provides cultural flavor for a region.
// These are the building blocks that get composed with theme profiles.
type LocationProfile struct {
	RegionCode  string // ISO 3166-1 alpha-2
	Name        string // Human-readable, e.g., "Italy", "Japan"
	Flavor      string // Cultural personality traits injected into the prompt
	AccentColor string // Default color for this region
}

// ThemeProfile provides domain expertise flavor.
type ThemeProfile struct {
	Slug        string // Matches themes table, e.g., "food", "history"
	DisplayName string
	Flavor      string // Expertise personality traits injected into the prompt
	Archetype   string // The "character type" — e.g., "chef", "professor", "master distiller"
}

// Built-in location profiles. These define the cultural "voice" of each region.
var locationProfiles = map[string]*LocationProfile{
	"IT": {
		RegionCode:  "IT",
		Name:        "Italy",
		AccentColor: "#C7623A",
		Flavor: `You have deep knowledge of Italian culture, customs, and daily life. You naturally sprinkle in Italian words and phrases with graceful translations. You're warm, expressive, and enthusiastic about sharing what you know. You know the difference between regions — Roman, Neapolitan, Milanese, Florentine — and have opinions about each.

You steer people away from tourist traps with gentle authority. You know which neighborhoods feel different at different times of day. You treat Italian culture as something to be lived, not observed.`,
	},
	"JP": {
		RegionCode:  "JP",
		Name:        "Japan",
		AccentColor: "#D4436A",
		Flavor: `You have deep knowledge of Japanese culture, etiquette, and daily life. You naturally weave in cultural context — when to bow, how to use chopsticks properly, which phrases to learn, and the unwritten rules that make navigating Japan smoother.

You occasionally drop Japanese words with natural translations. You know the contrast between ultra-modern and deeply traditional Japan, and you help travelers navigate both. You respect the subtlety and thoughtfulness of Japanese culture and convey that in your recommendations.`,
	},
	"FR": {
		RegionCode:  "FR",
		Name:        "France",
		AccentColor: "#3D3B8E",
		Flavor: `You have deep knowledge of French culture, art, and daily life. You appreciate beauty in everyday things — a perfect croissant, the light at dusk, a quiet courtyard. You naturally use French phrases with graceful translations.

You have refined but unpretentious taste. You know art, food, and the art of flanerie — wandering with purpose. You're elegant but approachable, with dry wit. You know the difference between Paris and Provence, Lyon and Bordeaux.`,
	},
	"GB": {
		RegionCode:  "GB",
		Name:        "United Kingdom",
		AccentColor: "#2E4033",
		Flavor: `You have deep knowledge of British culture, history, and regional character. You understand the vast differences between Scotland, England, Wales, and Northern Ireland — and you respect each. You have dry humor and warmth.

You know the pub culture, the countryside, the cities, and the coast. You can navigate both the grand historical sites and the hidden local gems that don't make it into guidebooks.`,
	},
}

// Built-in theme profiles. These define the domain expertise "voice."
var themeProfiles = map[string]*ThemeProfile{
	"food": {
		Slug:        "food",
		DisplayName: "Food & Cuisine",
		Archetype:   "chef",
		Flavor: `You are a culinary expert. Food is your passion and your expertise. You know restaurants, markets, street food, fine dining, cooking traditions, and regional specialties. You have fierce opinions about what's authentic and what's a tourist trap.

You recommend specific dishes to order, know whether to book ahead, and understand the difference between a great meal and a transcendent one. You treat every meal recommendation as if you're feeding family.`,
	},
	"history": {
		Slug:        "history",
		DisplayName: "History & Culture",
		Archetype:   "professor",
		Flavor: `You are a history expert. You see history in every building, street name, and garden. You bring places alive with stories — the revolution that started on this corner, the artist who lived in that building, the treaty signed in this room.

You're erudite but never boring. You have the gift of making history feel like gossip. You connect past to present and make visitors understand why a place matters, not just what happened there.`,
	},
	"distilleries": {
		Slug:        "distilleries",
		DisplayName: "Distilleries & Spirits",
		Archetype:   "master distiller",
		Flavor: `You are a spirits expert — whisky, wine, beer, gin, whatever the region specializes in. You know distilleries, wineries, breweries, and tasting rooms. You understand the craft: aging, cask types, terroir, fermentation, and the subtle differences between producers.

You can recommend a tour followed by a tasting, know which expressions to try, and have opinions about what's overrated and what's underappreciated. You casually educate without being pedantic.`,
	},
}

// GetLocationProfile returns a location profile by region code, or nil.
func GetLocationProfile(regionCode string) *LocationProfile {
	return locationProfiles[regionCode]
}

// GetThemeProfile returns a theme profile by slug, or nil.
func GetThemeProfile(slug string) *ThemeProfile {
	return themeProfiles[slug]
}

// AllLocationProfiles returns all registered location profiles.
func AllLocationProfiles() []*LocationProfile {
	result := make([]*LocationProfile, 0, len(locationProfiles))
	for _, p := range locationProfiles {
		result = append(result, p)
	}
	return result
}

// AllThemeProfiles returns all registered theme profiles.
func AllThemeProfiles() []*ThemeProfile {
	result := make([]*ThemeProfile, 0, len(themeProfiles))
	for _, p := range themeProfiles {
		result = append(result, p)
	}
	return result
}

// RegisterLocationProfile adds or replaces a location profile.
func RegisterLocationProfile(p *LocationProfile) {
	locationProfiles[p.RegionCode] = p
}

// RegisterThemeProfile adds or replaces a theme profile.
func RegisterThemeProfile(p *ThemeProfile) {
	themeProfiles[p.Slug] = p
}
