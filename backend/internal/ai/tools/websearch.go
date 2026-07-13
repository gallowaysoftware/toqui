package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gallowaysoftware/toqui/backend/internal/ai"
)

// searchResult is the normalized shape returned to the AI regardless of the
// underlying search backend.
type searchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// searchBackend performs a web search and returns normalized results.
// Implementations: Google Custom Search and SearXNG. A nil backend means
// web search is not configured (the tool returns a graceful no_web_access).
type searchBackend interface {
	search(ctx context.Context, query string) ([]searchResult, error)
}

// WebSearch is the web_search chat tool. It's always registered (even when
// unconfigured) so the AI gets a clear "no web access" signal instead of an
// "unknown tool" error — which Gemini treats as a real failure and retries
// pointlessly (#194).
type WebSearch struct {
	backend searchBackend
}

// NewWebSearch builds the Google Custom Search backed tool.
func NewWebSearch(apiKey, cx string) *WebSearch {
	return &WebSearch{backend: &googleSearch{
		apiKey: apiKey,
		cx:     cx,
		client: &http.Client{Timeout: 15 * time.Second},
	}}
}

// NewWebSearchSearXNG builds the SearXNG backed tool. baseURL points at a
// SearXNG instance with the JSON output format enabled (`search.formats:
// [json]` in its settings.yml). Self-hosters typically run one alongside
// Toqui or point at an existing instance.
func NewWebSearchSearXNG(baseURL string) *WebSearch {
	return &WebSearch{backend: &searxngSearch{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 15 * time.Second},
	}}
}

// NewWebSearchStub returns a registered tool with no backend — it responds
// with a graceful "feature unavailable" message so the AI falls back to
// parametric knowledge instead of erroring.
func NewWebSearchStub() *WebSearch {
	return &WebSearch{}
}

func (w *WebSearch) configured() bool { return w.backend != nil }

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
		// which cascades into follow-up tools (e.g. create_itinerary_items) never
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

	results, err := w.backend.search(ctx, input.Query)
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{"results": results})
}

// googleSearch queries the Google Custom Search JSON API.
type googleSearch struct {
	apiKey string
	cx     string
	client *http.Client
}

func (g *googleSearch) search(ctx context.Context, query string) ([]searchResult, error) {
	u := fmt.Sprintf("https://www.googleapis.com/customsearch/v1?key=%s&cx=%s&q=%s&num=5",
		url.QueryEscape(g.apiKey), url.QueryEscape(g.cx), url.QueryEscape(query))

	body, err := httpGetLimited(ctx, g.client, u, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Items []struct {
			Title   string `json:"title"`
			Link    string `json:"link"`
			Snippet string `json:"snippet"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse google results: %w", err)
	}
	results := make([]searchResult, 0, len(result.Items))
	for _, item := range result.Items {
		results = append(results, searchResult{Title: item.Title, URL: item.Link, Snippet: item.Snippet})
	}
	return results, nil
}

// searxngSearch queries a SearXNG instance's JSON API. SearXNG must have the
// JSON format enabled; the snippet lives in the result's "content" field.
type searxngSearch struct {
	baseURL string
	client  *http.Client
}

func (s *searxngSearch) search(ctx context.Context, query string) ([]searchResult, error) {
	u := fmt.Sprintf("%s/search?q=%s&format=json", s.baseURL, url.QueryEscape(query))

	// SearXNG rejects the default Go user agent from some instances; send a
	// plain one.
	body, err := httpGetLimited(ctx, s.client, u, map[string]string{"User-Agent": "Toqui/1.0"})
	if err != nil {
		return nil, err
	}

	var result struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse searxng results: %w", err)
	}

	const maxResults = 5
	results := make([]searchResult, 0, maxResults)
	for _, item := range result.Results {
		if len(results) >= maxResults {
			break
		}
		results = append(results, searchResult{Title: item.Title, URL: item.URL, Snippet: item.Content})
	}
	return results, nil
}

// httpGetLimited performs a GET and returns the (size-bounded) body.
func httpGetLimited(ctx context.Context, client *http.Client, u string, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute search: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("search backend returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxToolResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	return body, nil
}
