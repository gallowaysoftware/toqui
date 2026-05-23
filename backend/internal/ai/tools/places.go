package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	var input struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return nil, fmt.Errorf("unmarshal args: %w", err)
	}

	// Use Google Places Text Search API
	url := fmt.Sprintf("https://maps.googleapis.com/maps/api/place/textsearch/json?query=%s&key=%s",
		url.QueryEscape(input.Query), p.apiKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute search: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxToolResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
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
		return json.Marshal(map[string]string{"error": "failed to parse place results"})
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

	return json.Marshal(map[string]any{"places": places})
}
