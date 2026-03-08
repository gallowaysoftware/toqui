package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/gallowaysoftware/toqui-backend/internal/ai"
)

type WebSearch struct {
	apiKey string
	cx     string
	client *http.Client
}

func NewWebSearch(apiKey, cx string) *WebSearch {
	return &WebSearch{
		apiKey: apiKey,
		cx:     cx,
		client: http.DefaultClient,
	}
}

func (w *WebSearch) Definition() ai.ToolDefinition {
	return ai.ToolDefinition{
		Name:        "web_search",
		Description: "Search the web for current information about travel destinations, attractions, restaurants, events, and other travel-related topics.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"query": {
					"type": "string",
					"description": "The search query"
				}
			},
			"required": ["query"]
		}`),
	}
}

func (w *WebSearch) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	var input struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return nil, fmt.Errorf("unmarshal args: %w", err)
	}

	u := fmt.Sprintf("https://www.googleapis.com/customsearch/v1?key=%s&cx=%s&q=%s&num=5",
		w.apiKey, w.cx, url.QueryEscape(input.Query))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute search: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var result struct {
		Items []struct {
			Title   string `json:"title"`
			Link    string `json:"link"`
			Snippet string `json:"snippet"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return json.Marshal(map[string]string{"error": "failed to parse search results"})
	}

	type searchResult struct {
		Title   string `json:"title"`
		URL     string `json:"url"`
		Snippet string `json:"snippet"`
	}
	results := make([]searchResult, 0, len(result.Items))
	for _, item := range result.Items {
		results = append(results, searchResult{
			Title:   item.Title,
			URL:     item.Link,
			Snippet: item.Snippet,
		})
	}

	return json.Marshal(map[string]any{"results": results})
}
