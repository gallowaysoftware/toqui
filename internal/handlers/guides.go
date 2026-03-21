package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gallowaysoftware/toqui-backend/internal/persona"
)

// SimpleChatFn is a function that sends a prompt to an AI provider and returns the response.
type SimpleChatFn func(ctx context.Context, system, prompt string) (string, error)

// GuideHandler serves destination guide content generated from persona profiles.
type GuideHandler struct {
	chatFn SimpleChatFn

	mu    sync.RWMutex
	cache map[string]*cachedGuide
}

type cachedGuide struct {
	guide     *destinationGuide
	expiresAt time.Time
}

// NewGuideHandler creates a new GuideHandler.
func NewGuideHandler(chatFn SimpleChatFn) *GuideHandler {
	return &GuideHandler{
		chatFn: chatFn,
		cache:  make(map[string]*cachedGuide),
	}
}

type destinationListItem struct {
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	AccentColor string   `json:"accent_color"`
	Themes      []string `json:"themes,omitempty"`
}

type themeListItem struct {
	Slug        string `json:"slug"`
	DisplayName string `json:"display_name"`
}

type guideSection struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

type destinationGuide struct {
	Code        string         `json:"code"`
	Name        string         `json:"name"`
	AccentColor string         `json:"accent_color"`
	Theme       string         `json:"theme,omitempty"`
	ThemeName   string         `json:"theme_name,omitempty"`
	Sections    []guideSection `json:"sections"`
	GeneratedAt string         `json:"generated_at"`
}

// HandleList handles GET /api/guides — returns available destinations and themes.
func (h *GuideHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	profiles := persona.AllLocationProfiles()
	sort.Slice(profiles, func(i, j int) bool {
		return profiles[i].Name < profiles[j].Name
	})

	items := make([]destinationListItem, 0, len(profiles))
	for _, p := range profiles {
		items = append(items, destinationListItem{
			Code:        p.RegionCode,
			Name:        p.Name,
			AccentColor: p.AccentColor,
		})
	}

	themeProfiles := persona.AllThemeProfiles()
	sort.Slice(themeProfiles, func(i, j int) bool {
		return themeProfiles[i].DisplayName < themeProfiles[j].DisplayName
	})

	themes := make([]themeListItem, 0, len(themeProfiles))
	for _, t := range themeProfiles {
		themes = append(themes, themeListItem{
			Slug:        t.Slug,
			DisplayName: t.DisplayName,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"destinations": items,
		"themes":       themes,
	}); err != nil {
		slog.Error("failed to encode guides list", "error", err)
	}
}

// HandleGuide handles GET /api/guides/{code}[?theme=slug] — generates or returns cached guide.
func (h *GuideHandler) HandleGuide(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract country code from path: /api/guides/IT
	code := strings.TrimPrefix(r.URL.Path, "/api/guides/")
	code = strings.ToUpper(strings.TrimSuffix(code, "/"))
	if code == "" {
		h.HandleList(w, r)
		return
	}

	profile := persona.GetLocationProfile(code)
	if profile == nil {
		http.Error(w, "destination not found", http.StatusNotFound)
		return
	}

	// Optional theme filter
	themeSlug := strings.ToLower(r.URL.Query().Get("theme"))
	var themeProfile *persona.ThemeProfile
	if themeSlug != "" {
		themeProfile = persona.GetThemeProfile(themeSlug)
		if themeProfile == nil {
			http.Error(w, "theme not found", http.StatusNotFound)
			return
		}
	}

	// Cache key includes theme
	cacheKey := code
	if themeSlug != "" {
		cacheKey = code + ":" + themeSlug
	}

	// Check cache (24-hour TTL)
	h.mu.RLock()
	if cached, ok := h.cache[cacheKey]; ok && time.Now().Before(cached.expiresAt) {
		h.mu.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(cached.guide); err != nil {
			slog.Error("failed to encode cached guide", "error", err)
		}
		return
	}
	h.mu.RUnlock()

	// Generate guide
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	guide, err := h.generateGuide(ctx, profile, themeProfile)
	if err != nil {
		slog.Error("guide generation failed", "code", code, "theme", themeSlug, "error", err)
		http.Error(w, "guide generation failed", http.StatusInternalServerError)
		return
	}

	// Cache the result
	h.mu.Lock()
	h.cache[cacheKey] = &cachedGuide{
		guide:     guide,
		expiresAt: time.Now().Add(24 * time.Hour),
	}
	h.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(guide); err != nil {
		slog.Error("failed to encode guide", "error", err)
	}
}

func (h *GuideHandler) generateGuide(ctx context.Context, location *persona.LocationProfile, theme *persona.ThemeProfile) (*destinationGuide, error) {
	system := "You are a travel guide writer. Write concise, helpful travel guide content. " +
		"Use 2-3 short paragraphs per section. Be specific with names of places, dishes, and tips. " +
		"Do not use markdown formatting — write plain text only.\n\n" +
		location.Flavor

	guide := &destinationGuide{
		Code:        location.RegionCode,
		Name:        location.Name,
		AccentColor: location.AccentColor,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}

	name := location.Name

	// Build section prompts based on whether a theme is provided
	type sectionDef struct {
		title  string
		prompt string
	}

	var defs []sectionDef

	if theme != nil {
		// Theme-specific guide: inject theme expertise into system prompt
		system += "\n\n" + theme.Flavor
		guide.Theme = theme.Slug
		guide.ThemeName = theme.DisplayName

		themeName := theme.DisplayName
		defs = []sectionDef{
			{"Overview", "Write a 2-paragraph overview of " + name + " specifically through the lens of " + themeName + ". Why is " + name + " a great destination for " + themeName + " enthusiasts?"},
			{"Top Experiences", "List the top 5-6 " + themeName + " experiences, venues, or activities in " + name + ". Be specific with names and brief descriptions."},
			{"Insider Tips", "Give 5-6 insider tips for experiencing " + themeName + " in " + name + " like a local. Include practical advice, timing, etiquette, and hidden gems."},
			{"Where to Go", "Describe the best neighborhoods, districts, or regions in " + name + " for " + themeName + ". What makes each area unique?"},
			{"Best Time to Visit", "When is the best time to visit " + name + " for " + themeName + "? Cover seasonal events, festivals, or peak periods specific to " + themeName + "."},
			{"Budget Guide", "What should visitors expect to spend on " + themeName + " in " + name + "? Cover price ranges, where to splurge, and where to save."},
		}
	} else {
		// General destination guide
		defs = []sectionDef{
			{"Overview", "Write a 2-paragraph overview of " + name + " as a travel destination. What makes it special and who should visit?"},
			{"Top Highlights", "List the top 5-6 must-see attractions and experiences in " + name + ". Brief description for each."},
			{"Food & Dining", "Describe the food and dining scene in " + name + ". What dishes to try, where to eat, and any dining customs to know."},
			{"Getting Around", "How to get around in " + name + " — transportation options, tips for travelers, and what to know about getting from the airport."},
			{"Best Time to Visit", "When is the best time to visit " + name + "? Cover seasons, weather, peak vs off-peak, and any major events or festivals."},
			{"Practical Tips", "Give 5-6 practical tips for first-time visitors to " + name + ". Cover cultural etiquette, money, safety, and common mistakes to avoid."},
		}
	}

	guide.Sections = make([]guideSection, 0, len(defs))
	for _, def := range defs {
		text, err := h.chatFn(ctx, system, def.prompt)
		if err != nil {
			return nil, err
		}
		guide.Sections = append(guide.Sections, guideSection{
			Title:   def.title,
			Content: strings.TrimSpace(text),
		})
	}

	return guide, nil
}
