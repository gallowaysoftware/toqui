package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/gallowaysoftware/toqui/backend/internal/ai"
)

// maxToolResponseBytes is the maximum response body size for external tool API
// calls. Prevents OOM if a remote API returns an unexpectedly large response.
const maxToolResponseBytes = 2 << 20 // 2 MB

type PlaceLookup struct {
	apiKey string
	client *http.Client
}

func NewPlaceLookup(apiKey string) *PlaceLookup {
	return &PlaceLookup{
		apiKey: apiKey,
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

// NewPlaceLookupStub returns a registered place_lookup tool with no API key —
// it responds with a graceful "unavailable" message rather than erroring, so
// the AI (which is told about place_lookup in the system prompt) falls back to
// its own knowledge. Same rationale as NewWebSearchStub (#194): an unknown
// tool would make Gemini retry pointlessly. Used when GOOGLE_PLACES_API_KEY
// isn't set — the common self-host case.
func NewPlaceLookupStub() *PlaceLookup {
	return &PlaceLookup{}
}

func (p *PlaceLookup) configured() bool { return p.apiKey != "" }

func (p *PlaceLookup) Definition() ai.ToolDefinition {
	return ai.ToolDefinition{
		Name:        "place_lookup",
		Description: "Look up details about a specific place including address, ratings, opening hours, and photos. Use this when you need specific information about a restaurant, hotel, attraction, or other point of interest.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"query": {
					"type": "string",
					"description": "The name and location of the place to look up, e.g. 'Eiffel Tower Paris'"
				}
			},
			"required": ["query"]
		}`),
	}
}

func (p *PlaceLookup) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	// Like web_search, no failure path returns a Go error or an "error"-keyed
	// payload — that would make the chat loop feed {"error": ...} to Gemini,
	// which treats it as a real failure and abandons follow-up tools (#194).
	// "Not configured" and "backend failed" both degrade to no_place_data.
	if !p.configured() {
		return placeUnavailable("Place lookup is not configured in this environment.")
	}

	var input struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		slog.Warn("place_lookup: bad arguments, returning no_place_data", "error", err)
		return placeUnavailable("Place lookup could not run (bad query).")
	}

	places, err := p.lookup(ctx, input.Query)
	if err != nil {
		slog.Warn("place_lookup: backend failed, returning no_place_data", "error", err)
		return placeUnavailable("Place lookup is temporarily unavailable.")
	}
	return json.Marshal(map[string]any{"places": places})
}

type placeResult struct {
	Name      string   `json:"name"`
	Address   string   `json:"address"`
	Rating    float64  `json:"rating"`
	Reviews   int      `json:"review_count"`
	Latitude  float64  `json:"latitude"`
	Longitude float64  `json:"longitude"`
	Types     []string `json:"types"`
}

func (p *PlaceLookup) lookup(ctx context.Context, query string) ([]placeResult, error) {
	u := fmt.Sprintf("https://maps.googleapis.com/maps/api/place/textsearch/json?query=%s&key=%s",
		url.QueryEscape(query), url.QueryEscape(p.apiKey))

	body, err := httpGetLimited(ctx, p.client, u, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Results []struct {
			Name             string  `json:"name"`
			FormattedAddress string  `json:"formatted_address"`
			Rating           float64 `json:"rating"`
			UserRatingsTotal int     `json:"user_ratings_total"`
			Geometry         struct {
				Location struct {
					Lat float64 `json:"lat"`
					Lng float64 `json:"lng"`
				} `json:"location"`
			} `json:"geometry"`
			Types []string `json:"types"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse place results: %w", err)
	}

	places := make([]placeResult, 0, len(result.Results))
	for _, r := range result.Results {
		places = append(places, placeResult{
			Name:      r.Name,
			Address:   r.FormattedAddress,
			Rating:    r.Rating,
			Reviews:   r.UserRatingsTotal,
			Latitude:  r.Geometry.Location.Lat,
			Longitude: r.Geometry.Location.Lng,
			Types:     r.Types,
		})
	}
	return places, nil
}

func placeUnavailable(reason string) (json.RawMessage, error) {
	return json.Marshal(map[string]any{
		"status":  "no_place_data",
		"places":  []any{},
		"message": reason + " Answer from your existing knowledge and tell the user you cannot verify live details (current hours, ratings, exact address) without it.",
	})
}
