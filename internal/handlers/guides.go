package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
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
	Code        string `json:"code"`
	Name        string `json:"name"`
	AccentColor string `json:"accent_color"`
}

type destinationGuide struct {
	Code          string `json:"code"`
	Name          string `json:"name"`
	AccentColor   string `json:"accent_color"`
	Overview      string `json:"overview"`
	Highlights    string `json:"highlights"`
	FoodAndDine   string `json:"food_and_dining"`
	GettingAround string `json:"getting_around"`
	BestTime      string `json:"best_time_to_visit"`
	Tips          string `json:"practical_tips"`
	GeneratedAt   string `json:"generated_at"`
}

// HandleList handles GET /api/guides — returns available destinations.
func (h *GuideHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	profiles := persona.AllLocationProfiles()
	items := make([]destinationListItem, 0, len(profiles))
	for _, p := range profiles {
		items = append(items, destinationListItem{
			Code:        p.RegionCode,
			Name:        p.Name,
			AccentColor: p.AccentColor,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(items); err != nil {
		slog.Error("failed to encode guides list", "error", err)
	}
}

// HandleGuide handles GET /api/guides/{code} — generates or returns cached guide.
func (h *GuideHandler) HandleGuide(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract country code from path: /api/guides/IT
	code := strings.TrimPrefix(r.URL.Path, "/api/guides/")
	code = strings.ToUpper(strings.TrimSuffix(code, "/"))
	if code == "" {
		// No code — treat as list request
		h.HandleList(w, r)
		return
	}

	profile := persona.GetLocationProfile(code)
	if profile == nil {
		http.Error(w, "destination not found", http.StatusNotFound)
		return
	}

	// Check cache (24-hour TTL)
	h.mu.RLock()
	if cached, ok := h.cache[code]; ok && time.Now().Before(cached.expiresAt) {
		h.mu.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(cached.guide); err != nil {
			slog.Error("failed to encode cached guide", "error", err)
		}
		return
	}
	h.mu.RUnlock()

	// Generate guide
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	guide, err := h.generateGuide(ctx, profile)
	if err != nil {
		slog.Error("guide generation failed", "code", code, "error", err)
		http.Error(w, "guide generation failed", http.StatusInternalServerError)
		return
	}

	// Cache the result
	h.mu.Lock()
	h.cache[code] = &cachedGuide{
		guide:     guide,
		expiresAt: time.Now().Add(24 * time.Hour),
	}
	h.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(guide); err != nil {
		slog.Error("failed to encode guide", "error", err)
	}
}

func (h *GuideHandler) generateGuide(ctx context.Context, profile *persona.LocationProfile) (*destinationGuide, error) {
	system := "You are a travel guide writer. Write concise, helpful travel guide content. " +
		"Use 2-3 short paragraphs per section. Be specific with names of places, dishes, and tips. " +
		"Do not use markdown formatting — write plain text only. " +
		profile.Flavor

	sections := []struct {
		field  *string
		prompt string
	}{
		{nil, ""}, // placeholder for overview
	}

	guide := &destinationGuide{
		Code:        profile.RegionCode,
		Name:        profile.Name,
		AccentColor: profile.AccentColor,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}

	// Generate each section sequentially (fast model, small tokens)
	type sectionDef struct {
		target *string
		prompt string
	}
	defs := []sectionDef{
		{&guide.Overview, "Write a 2-paragraph overview of " + profile.Name + " as a travel destination. What makes it special and who should visit?"},
		{&guide.Highlights, "List the top 5-6 must-see attractions and experiences in " + profile.Name + ". Brief description for each."},
		{&guide.FoodAndDine, "Describe the food and dining scene in " + profile.Name + ". What dishes to try, where to eat, and any dining customs to know."},
		{&guide.GettingAround, "How to get around in " + profile.Name + " — transportation options, tips for travelers, and what to know about getting from the airport."},
		{&guide.BestTime, "When is the best time to visit " + profile.Name + "? Cover seasons, weather, peak vs off-peak, and any major events or festivals."},
		{&guide.Tips, "Give 5-6 practical tips for first-time visitors to " + profile.Name + ". Cover cultural etiquette, money, safety, and common mistakes to avoid."},
	}

	_ = sections // unused, using defs instead

	for _, def := range defs {
		text, err := h.chatFn(ctx, system, def.prompt)
		if err != nil {
			return nil, err
		}
		*def.target = strings.TrimSpace(text)
	}

	return guide, nil
}
