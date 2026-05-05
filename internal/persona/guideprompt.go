package persona

import (
	"fmt"
	"strings"
)

// GuidePromptOptions configures BuildGuidePrompt. Zero values are valid.
type GuidePromptOptions struct {
	// PersonaName is the composed expert's first name (e.g. "Hana").
	// When empty, the prompt instructs the model to write in a generic
	// expert voice rather than referencing a specific character.
	PersonaName string
	// PersonaSpecialty is the one-line "Expert in ..." subtitle from the
	// composed identity. Used to keep voice continuity with the chat persona.
	PersonaSpecialty string
}

// BuildGuidePrompt returns the user prompt sent to the AI when generating
// a destination guide for a (location, theme) pair.
//
// PR 1 of the genguides feature ships this builder but does NOT change any
// runtime path — the live GuidesHandler still serves the hand-authored
// staticGuides() content. The CLI in cmd/genguides is the only caller for
// now; PR 2 reviews the generated artefact and PR 3 flips the runtime path.
//
// Hard rules baked into the prompt (DO NOT soften without re-litigating
// the design proposal — see toqui-backend#30):
//   - No specific restaurants, hotels, attractions, or businesses by name.
//   - No claims about visa requirements, health risks, or safety conditions.
//   - Only neighborhoods, districts, and categories of experience.
//
// The output shape is stabilised so a downstream parser (PR 2) can read it
// deterministically: a YAML-ish front matter with title + excerpt followed
// by four named markdown sections.
func BuildGuidePrompt(loc *LocationProfile, theme *ThemeProfile, opts GuidePromptOptions) string {
	var b strings.Builder

	locationName := "the destination"
	regionCode := ""
	if loc != nil {
		locationName = loc.Name
		regionCode = loc.RegionCode
	}

	themeName := "travel"
	if theme != nil {
		themeName = theme.DisplayName
	}

	b.WriteString("You are writing a destination guide for travelers planning a trip.\n\n")

	if opts.PersonaName != "" {
		fmt.Fprintf(&b, "Voice: write as %s", opts.PersonaName)
		if opts.PersonaSpecialty != "" {
			fmt.Fprintf(&b, " — %s", opts.PersonaSpecialty)
		}
		b.WriteString(". Use the first person sparingly; the guide is informational, not a chat message.\n\n")
	} else {
		b.WriteString("Voice: confident, warm, expert. Informational tone — not a chat message.\n\n")
	}

	fmt.Fprintf(&b, "Destination: %s", locationName)
	if regionCode != "" {
		fmt.Fprintf(&b, " (%s)", regionCode)
	}
	b.WriteString("\n")
	fmt.Fprintf(&b, "Theme: %s\n\n", themeName)

	// Hard rules — DO NOT REORDER OR SOFTEN.
	// internal/persona/guideprompt_test.go pins the exact sentences below as
	// regression guards. Any edit that drops or weakens these lines must
	// update the test in the same PR (and re-litigate the design proposal).
	b.WriteString("HARD RULES (these are non-negotiable):\n")
	b.WriteString("- Do NOT mention specific restaurants, hotels, attractions, or businesses by name.\n")
	b.WriteString("- Do NOT make claims about visa requirements, health risks, or safety conditions.\n")
	b.WriteString("- Use only neighborhoods, districts, and categories of experience.\n")
	b.WriteString("- Do NOT invent statistics, dates, or quotations. If you are uncertain, omit the detail.\n")
	b.WriteString("- Avoid superlatives that imply ranking (\"the best\", \"the world's greatest\") — describe character instead.\n\n")

	b.WriteString("Output format — return EXACTLY this structure with no preamble or trailer:\n\n")
	b.WriteString("---\n")
	b.WriteString("title: <a roughly 60-character title that names the destination and theme>\n")
	b.WriteString("excerpt: <a roughly 200-character single-sentence hook for the guide>\n")
	b.WriteString("---\n\n")
	b.WriteString("## Why visit\n\n")
	b.WriteString("<2-3 short paragraphs on the character of the destination through the lens of the theme. Neighborhoods and categories only.>\n\n")
	b.WriteString("## What to do\n\n")
	b.WriteString("<2-4 short paragraphs grouped by neighborhood or district. Describe types of experiences (markets, rooftop bars, coastal walks) — never name a specific business.>\n\n")
	b.WriteString("## Best time to visit\n\n")
	b.WriteString("<1-2 short paragraphs on seasons, weather patterns, and travel rhythms. No claims about safety or health.>\n\n")
	b.WriteString("## Practical notes\n\n")
	b.WriteString("<1-2 short paragraphs on getting around, neighborhood orientation, and general etiquette. Do not give visa, health, or safety advice — point readers to official sources for those.>\n\n")
	b.WriteString("Total length: approximately 1500 tokens. Plain markdown only — no tables, no images, no links.\n")

	return b.String()
}
