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

type WebSearch struct {
	apiKey string
	cx     string
	client *http.Client
}

func NewWebSearch(apiKey, cx string) *WebSearch {
	return &WebSearch{
		apiKey: apiKey,
		cx:     cx,
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

// NewWebSearchStub returns a registered tool that responds with a graceful
// "feature unavailable" message. Used in environments where the Google Custom
// Search API key isn't configured so the AI gets a clear signal instead of
// the registry returning "unknown tool" — which Gemini interprets as a real
// failure and retries pointlessly (#194).
func NewWebSearchStub() *WebSearch {
	return &WebSearch{}
}

func (w *WebSearch) configured() bool {
	return w.apiKey != "" && w.cx != ""
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
	if !w.configured() {
		// IMPORTANT: this response MUST NOT contain an "error" field.
		// Gemini interprets any tool result with an "error" key as a genuine
		// failure and either retries the call or apologises to the user,
		// which cascades into follow-up tools (e.g. recommend_booking) never
		// being invoked (Run 4 R-16). A plain status/message payload tells
		// the AI "the call succeeded, there's just no web access" and lets
		// it gracefully fall back to parametric knowledge.
		return json.Marshal(map[string]any{
			"status":  "no_web_access",
			"results": []any{},
			"message": "Real-time web search is not configured in this environment. Proceed using your existing knowledge and tell the user you cannot verify time-sensitive details (current opening hours, prices, closures) without web access. Then continue answering their question with the information you have.",
		})
	}

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

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxToolResponseBytes))
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
