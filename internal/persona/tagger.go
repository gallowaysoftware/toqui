package persona

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ThemeTagger uses AI to analyze trip context and assign theme tags.
type ThemeTagger struct {
	// chat is a function that sends a prompt to an AI and returns the response text.
	// This avoids a direct dependency on the AI provider package.
	chat func(ctx context.Context, system, prompt string) (string, error)
}

func NewThemeTagger(chatFn func(ctx context.Context, system, prompt string) (string, error)) *ThemeTagger {
	return &ThemeTagger{chat: chatFn}
}

// TagResult holds the AI's theme analysis of a trip.
type TagResult struct {
	Themes          []TagScore `json:"themes"`
	DestinationCode string     `json:"destination_code"`
}

type TagScore struct {
	Slug       string  `json:"slug"`
	Confidence float64 `json:"confidence"`
}

// AnalyzeTrip determines themes for a trip based on its description,
// title, and optionally recent chat messages.
func (t *ThemeTagger) AnalyzeTrip(ctx context.Context, title, description string, recentMessages []string) (*TagResult, error) {
	// Get current available themes
	var themeList []string
	for slug, tp := range themeProfiles {
		themeList = append(themeList, fmt.Sprintf("- %s: %s", slug, tp.DisplayName))
	}

	var msgContext string
	if len(recentMessages) > 0 {
		// Take last 5 messages max
		msgs := recentMessages
		if len(msgs) > 5 {
			msgs = msgs[len(msgs)-5:]
		}
		msgContext = fmt.Sprintf("\n\nRecent chat messages about this trip:\n%s", strings.Join(msgs, "\n"))
	}

	prompt := fmt.Sprintf(`Analyze this trip and determine its themes and destination country.

Trip title: %s
Trip description: %s%s

Available themes:
%s

Respond with JSON only:
{
  "themes": [{"slug": "theme_slug", "confidence": 0.0-1.0}],
  "destination_code": "XX"
}

Rules:
- Only include themes with confidence > 0.3
- destination_code is ISO 3166-1 alpha-2 (e.g., "IT", "JP", "FR", "GB")
- If you can't determine the destination, use ""
- A trip can have multiple themes
- Rank by confidence (most relevant first)`,
		title, description, msgContext, strings.Join(themeList, "\n"))

	response, err := t.chat(ctx, "You are a trip analyzer. Classify trips by theme and destination. Respond with JSON only.", prompt)
	if err != nil {
		return nil, fmt.Errorf("theme analysis: %w", err)
	}

	return parseTagResult(response)
}

func parseTagResult(raw string) (*TagResult, error) {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("no JSON found in response")
	}

	var result TagResult
	if err := json.Unmarshal([]byte(raw[start:end+1]), &result); err != nil {
		return nil, fmt.Errorf("parse tag result: %w", err)
	}

	return &result, nil
}
