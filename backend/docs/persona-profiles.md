# Persona Profiles

## Persona System Overview

The persona system gives each chat conversation a distinct expert voice tailored to the trip's destination and themes. Three components work together:

- **Registry** (`registry.go`) -- The entry point. Holds the hardcoded Toqui orchestrator persona and a reference to the Composer. `Resolve(ctx, regionCode, themes)` is the main method: if a region and themes are provided it delegates to the Composer; otherwise it returns Toqui. It also generates handoff messages when Toqui introduces an expert.

- **Composer** (`composer.go`) -- Builds expert personas dynamically by combining a `LocationProfile` with one or more `ThemeProfile`s. It generates a composite system prompt, resolves an identity (name, greeting, description) via AI or a template fallback, and caches the result so the same inputs always return the same persona.

- **Profiles** (`profiles.go`, `profiles_extended.go`) -- The raw data. Each `LocationProfile` carries cultural flavor text for a region; each `ThemeProfile` carries domain expertise flavor text. Profiles are stored in package-level maps and registered at init time.

### Toqui -- the default orchestrator

Toqui is always available at ID `"toqui"`. It is a generalist travel companion that handles trip planning, logistics, and bookings. When a conversation moves into deep domain territory (food, history, spirits, etc.), Toqui suggests handing off to a composed expert persona. The handoff is conversational, not automatic.

### How expert personas are composed

1. The caller provides a region code (e.g. `"IT"`) and a list of theme slugs (e.g. `["food", "history"]`).
2. The Composer looks up the matching `LocationProfile` and `ThemeProfile`(s).
3. A system prompt is built: base expert preamble + location flavor + theme expertise + common expert behavior rules.
4. An identity (name, greeting, description) is generated via the `IdentityGenerator` callback (AI) or the template fallback.
5. The result is cached and returned as a `*Persona`.

---

## Location Profiles

24 total (4 original + 20 extended). All registered in `profiles.go` and `profiles_extended.go`.

| #   | Region Code | Name           | Accent Color | Flavor Summary                                                                                                                    |
| --- | ----------- | -------------- | ------------ | --------------------------------------------------------------------------------------------------------------------------------- |
| 1   | `IT`        | Italy          | `#C7623A`    | Regional Italian culture (Roman, Neapolitan, Milanese, Florentine), anti-tourist-trap guidance, Italian phrases with translations |
| 2   | `JP`        | Japan          | `#D4436A`    | Japanese etiquette, traditional vs modern contrast, cultural context (bowing, chopsticks, unwritten rules)                        |
| 3   | `FR`        | France         | `#3D3B8E`    | French art de vivre, flanerie, refined but unpretentious taste, dry wit, regional differences (Paris, Provence, Lyon, Bordeaux)   |
| 4   | `GB`        | United Kingdom | `#2E4033`    | Distinct nations (Scotland, England, Wales, NI), pub culture, dry humor, countryside and coast                                    |
| 5   | `US`        | United States  | `#3C3B6E`    | Regional diversity (PNW, Deep South, New England, Southwest), national parks, road trip culture, local food scenes                |
| 6   | `ES`        | Spain          | `#C60B1E`    | Spanish rhythm (late dinners, siestas, fiestas), regional identity (Catalonia, Andalusia, Basque Country, Galicia), tapas culture |
| 7   | `DE`        | Germany        | `#FFCC00`    | Beer gardens, castle routes, regional identity (Bavaria, Berlin, Hamburg), surprising warmth beneath efficiency                   |
| 8   | `PT`        | Portugal       | `#006600`    | Saudade, fado, azulejo tiles, pasteis de nata, Lisbon vs Porto, port wine cellars, Douro Valley                                   |
| 9   | `GR`        | Greece         | `#0D5EAF`    | Mediterranean pace, island logistics, taverna culture, hospitality, hidden coves and mountain villages                            |
| 10  | `TH`        | Thailand       | `#F4C430`    | Temple etiquette, street food, island selection, Bangkok vs Chiang Mai, cultural context (wai, monarchy respect)                  |
| 11  | `MX`        | Mexico         | `#006847`    | Street tacos, cenotes, Day of the Dead, mezcal culture, regional moles, Mexico City vs Oaxaca vs Yucatan                          |
| 12  | `AU`        | Australia      | `#FF9900`    | Laid-back spirit, immense distances, Indigenous heritage, reef and outback, Aussie slang, Sydney vs Melbourne                     |
| 13  | `BR`        | Brazil         | `#009739`    | Samba, Carnival, churrasco, caipirinha, Amazon biodiversity, Rio vs Sao Paulo vs Salvador                                         |
| 14  | `IN`        | India          | `#FF9933`    | Kaleidoscopic diversity, spice markets, chai culture, state-by-state differences, navigating sensory richness                     |
| 15  | `KR`        | South Korea    | `#CD2E3A`    | K-food, hanok villages, jimjilbang, nightlife, Seoul neighborhoods (Hongdae, Bukchon), skincare culture                           |
| 16  | `VN`        | Vietnam        | `#DA251D`    | Morning pho ritual, motorbike chaos, Hoi An lanterns, Hanoi vs HCMC, Ha Long Bay, street food mastery                             |
| 17  | `MA`        | Morocco        | `#C1272D`    | Medinas, souks, tagine, hammam, mint tea hospitality, Marrakech vs Fez, Sahara desert, bazaar haggling                            |
| 18  | `PE`        | Peru           | `#D91023`    | Inca heritage, ceviche, pisco, altitude acclimatization, Lima food scene, Sacred Valley, Nazca Lines                              |
| 19  | `NZ`        | New Zealand    | `#00247D`    | Adventure spirit, Maori culture, extreme sports, wine regions, LOTR landscapes, North vs South Island                             |
| 20  | `TR`        | Turkey         | `#E30A17`    | East-meets-West, kebab mastery, Istanbul vs Cappadocia, Aegean coast, tea gardens, hammam tradition                               |
| 21  | `HR`        | Croatia        | `#171796`    | Adriatic coastline, island-hopping, Istrian truffles, Plitvice waterfalls, Game of Thrones locations, konoba restaurants          |
| 22  | `ZA`        | South Africa   | `#007749`    | Safari expertise (Big Five), wine routes, braai culture, Cape Town vs Johannesburg, Garden Route, complex history                 |
| 23  | `CO`        | Colombia       | `#FCD116`    | Transformation narrative, coffee culture, salsa, street art, Cartagena, Medellin, Cocora Valley                                   |
| 24  | `EG`        | Egypt          | `#C8102E`    | Ancient wonders, pyramid logistics, Nile cruises, Cairo chaos, Luxor, Red Sea diving, desert stargazing                           |

---

## Theme Profiles

15 total (3 original + 12 extended). All registered in `profiles.go` and `profiles_extended.go`.

| #   | Slug           | Display Name              | Archetype                | Domain Summary                                                                                      |
| --- | -------------- | ------------------------- | ------------------------ | --------------------------------------------------------------------------------------------------- |
| 1   | `food`         | Food & Cuisine            | chef                     | Restaurants, markets, street food, fine dining, regional specialties, authenticity opinions         |
| 2   | `history`      | History & Culture         | professor                | Historical sites, building stories, connecting past to present, making history feel like gossip     |
| 3   | `distilleries` | Distilleries & Spirits    | master distiller         | Whisky, wine, beer, gin tours and tastings, cask types, terroir, craft knowledge                    |
| 4   | `adventure`    | Adventure & Outdoors      | expedition leader        | Hiking, diving, climbing, kayaking, paragliding, gear and safety, fitness-matched challenges        |
| 5   | `wellness`     | Wellness & Retreats       | wellness curator         | Spas, retreats, yoga, meditation, thermal baths, holistic practices, local healing traditions       |
| 6   | `wine`         | Wine & Vineyards          | sommelier                | Vineyards, terroir, tasting rooms, pairings, production methods, walk-in vs reservation knowledge   |
| 7   | `architecture` | Architecture & Design     | architectural historian  | Building styles (Romanesque through contemporary), urban planning, architects, best vantage points  |
| 8   | `nightlife`    | Nightlife & Entertainment | nightlife curator        | Clubs, rooftop bars, live music, speakeasies, neighborhood-by-neighborhood knowledge, door policies |
| 9   | `shopping`     | Shopping & Markets        | style curator            | Local crafts, luxury districts, flea markets, artisan workshops, bargaining etiquette               |
| 10  | `family`       | Family Travel             | family travel specialist | Kid-friendly activities, stroller accessibility, educational experiences, restaurant welcomeness    |
| 11  | `photography`  | Photography & Visual      | visual storyteller       | Golden hour spots, hidden viewpoints, photo etiquette, cultural sensitivity around photography      |
| 12  | `nature`       | Nature & Wildlife         | naturalist               | Wildlife spotting, national parks, eco-tourism, seasonal migrations, ethical operators              |
| 13  | `romance`      | Romance & Couples         | romance curator          | Intimate dining, scenic walks, sunset viewpoints, hotel views, couple-tailored experiences          |
| 14  | `budget`       | Budget Travel             | savvy traveler           | Free activities, cheap eats, hostel culture, transport hacks, smart splurge decisions               |
| 15  | `luxury`       | Luxury Travel             | luxury concierge         | Michelin dining, five-star hotels, private tours, VIP access, discerning value judgment             |

---

## How Profiles Are Used

### Composite key

Every unique combination of region + themes produces a deterministic persona ID:

1. Region code is uppercased.
2. Theme slugs are sorted alphabetically.
3. The string `REGIONCODE:slug1,slug2,...` is SHA-256 hashed.
4. The ID is `expert-` followed by the first 8 bytes of the hash in hex (e.g. `expert-a1b2c3d4e5f6a7b8`).

This means the same inputs always produce the same cache key.

### Caching

The Composer holds an in-memory `map[string]*Persona` behind a `sync.RWMutex`. Once a persona is composed for a given key, subsequent calls with the same region + themes return the cached instance immediately. `CachedPersonas()` exposes all cached experts, and `ListAll()` on the Registry returns Toqui plus all cached experts.

### Identity generation

Two paths:

1. **AI generation** (preferred) -- If an `IdentityGenerator` function is provided to `NewComposer`, it is called with the location name, region code, theme display names, and archetypes. The AI returns a JSON object with `name`, `description`, and `greeting` fields. `IdentityGeneratorPrompt()` provides the prompt template. `ParseIdentityResult()` handles extracting JSON from potentially markdown-wrapped AI responses.

2. **Template fallback** -- If no generator is set (or if AI generation fails upstream), `templateIdentity` produces a formulaic identity: name like "Your Italy chef", description like "A local chef and professor specializing in Italy", greeting like "Hello! I'm your Food & Cuisine guide for Italy."

### System prompt composition

The composed system prompt is built in `buildComposedPrompt`:

1. Base preamble: `"You are an expert local guide."`
2. Location: `"You are based in {Name}."` followed by the full location flavor text.
3. Theme expertise: each theme's flavor text, under a "Your primary expertise" or "Your areas of expertise" heading.
4. Common expert behavior: stay in character, use tools for current info, provide specific names/addresses/details.

---

## Adding New Profiles

### Adding a location profile

1. Create a `LocationProfile` struct with:
   - `RegionCode` -- ISO 3166-1 alpha-2 code (e.g. `"CA"`)
   - `Name` -- Human-readable country/region name
   - `AccentColor` -- Hex color for UI theming
   - `Flavor` -- Multi-paragraph cultural personality text injected into the system prompt

2. Call `RegisterLocationProfile(&LocationProfile{...})` inside the `init()` function in `profiles_extended.go` (or `profiles.go` for core profiles).

Example:

```go
RegisterLocationProfile(&LocationProfile{
    RegionCode:  "CA",
    Name:        "Canada",
    AccentColor: "#FF0000",
    Flavor:      `You have deep knowledge of Canadian culture...`,
})
```

### Adding a theme profile

1. Create a `ThemeProfile` struct with:
   - `Slug` -- Lowercase identifier matching the `themes` table slug (e.g. `"music"`)
   - `DisplayName` -- Human-readable name (e.g. `"Music & Concerts"`)
   - `Archetype` -- The character type the expert embodies (e.g. `"music journalist"`)
   - `Flavor` -- Multi-paragraph domain expertise text injected into the system prompt

2. Call `RegisterThemeProfile(&ThemeProfile{...})` inside the `init()` function in `profiles_extended.go`.

3. Add a matching row to the `themes` database table (either via a new migration or by adding to the seed data in migration `000002`).

Example:

```go
RegisterThemeProfile(&ThemeProfile{
    Slug:        "music",
    DisplayName: "Music & Concerts",
    Archetype:   "music journalist",
    Flavor:      `You are a music expert...`,
})
```

Note: The profile maps are package-level and not concurrency-safe for writes after init. All registrations should happen in `init()` functions.
