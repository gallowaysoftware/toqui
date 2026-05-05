package persona

import (
	"strings"
	"testing"
)

// These tests pin the hard-rule sentences that the design proposal for
// toqui-backend#30 nailed down. A change that softens any of them should
// fail loudly so the author has to re-litigate the rule in the PR.
func TestBuildGuidePrompt_HardRulesPresent(t *testing.T) {
	loc := &LocationProfile{RegionCode: "JP", Name: "Japan"}
	theme := &ThemeProfile{Slug: "food", DisplayName: "Food & Cuisine"}

	out := BuildGuidePrompt(loc, theme, GuidePromptOptions{})

	mustContain := []string{
		"Do NOT mention specific restaurants, hotels, attractions, or businesses by name.",
		"Do NOT make claims about visa requirements, health risks, or safety conditions.",
		"Use only neighborhoods, districts, and categories of experience.",
	}
	for _, s := range mustContain {
		if !strings.Contains(out, s) {
			t.Errorf("guide prompt missing required hard-rule sentence:\n  %q", s)
		}
	}
}

func TestBuildGuidePrompt_OutputShape(t *testing.T) {
	loc := &LocationProfile{RegionCode: "FR", Name: "France"}
	theme := &ThemeProfile{Slug: "history", DisplayName: "History & Culture"}

	out := BuildGuidePrompt(loc, theme, GuidePromptOptions{
		PersonaName:      "Marie",
		PersonaSpecialty: "Expert in French art and culture.",
	})

	for _, section := range []string{"## Why visit", "## What to do", "## Best time to visit", "## Practical notes"} {
		if !strings.Contains(out, section) {
			t.Errorf("guide prompt missing required section header %q", section)
		}
	}
	if !strings.Contains(out, "title:") || !strings.Contains(out, "excerpt:") {
		t.Errorf("guide prompt missing front-matter title/excerpt fields")
	}
	if !strings.Contains(out, "Marie") {
		t.Errorf("guide prompt should reference persona name when supplied")
	}
	if !strings.Contains(out, "France") {
		t.Errorf("guide prompt should reference destination name")
	}
}

func TestBuildGuidePrompt_NilSafe(t *testing.T) {
	// The CLI may invoke this with a missing location profile if a slug is
	// added that we don't have a profile for. Make sure we don't panic.
	out := BuildGuidePrompt(nil, nil, GuidePromptOptions{})
	if !strings.Contains(out, "destination") {
		t.Errorf("nil-safe prompt should still mention destination")
	}
}
