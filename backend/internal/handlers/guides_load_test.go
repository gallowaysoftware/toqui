package handlers

import (
	"strings"
	"testing"
)

// PR 3 of toqui-backend#30 flips the guides runtime path from the
// hand-written staticGuides() to the embedded guides_data.gen.json
// (produced by make genguides). These tests pin the load behaviour
// + the runtime invariants the marketing site and the bundle handler
// depend on.

func TestNewGuidesHandler_LoadsFromEmbeddedJSON(t *testing.T) {
	// In the prod binary `generatedGuidesJSON` is non-empty (the
	// committed JSON is embedded at build). Verify NewGuidesHandler
	// loads from it: count > 0, every guide has the embed-derived
	// fields plus the runtime-derived CTAText/CTAURL.
	if strings.TrimSpace(generatedGuidesJSON) == "" {
		t.Skip("generatedGuidesJSON is empty in this environment — fallback path is exercised by TestLoadGuides_FallsBackToStaticOnEmptyEmbed")
	}

	const appURL = "https://app.toqui.test"
	h := NewGuidesHandler(appURL)
	if len(h.guides) == 0 {
		t.Fatal("expected non-empty guides slice from embedded JSON")
	}

	// Every guide must have the runtime-derived CTA fields. Without
	// these the marketing site's "Plan your trip with X" CTA cards
	// render with empty links.
	for _, g := range h.guides {
		if g.CTAURL != appURL {
			t.Errorf("guide %s: expected CTAURL=%s, got %q", g.Slug, appURL, g.CTAURL)
		}
		if g.CTAText == "" {
			t.Errorf("guide %s: CTAText must be non-empty (deriveCTAText degrades gracefully)", g.Slug)
		}
	}

	// Every slug must be unique — bySlug map relies on this.
	seen := make(map[string]bool, len(h.guides))
	for _, g := range h.guides {
		if seen[g.Slug] {
			t.Errorf("duplicate slug in embedded set: %s", g.Slug)
		}
		seen[g.Slug] = true
	}
}

func TestLoadGuides_FallsBackToStaticOnEmptyEmbed(t *testing.T) {
	// Drive the fallback branch directly. Save and restore the embed
	// var so other tests in this package aren't affected.
	original := generatedGuidesJSON
	generatedGuidesJSON = ""
	defer func() { generatedGuidesJSON = original }()

	guides, source := loadGuides("https://app.toqui.test")

	if source != "static" {
		t.Errorf("expected source=static for empty embed, got %q", source)
	}
	if len(guides) == 0 {
		t.Error("staticGuides() fallback returned 0 guides — local dev is broken")
	}
}

func TestLoadGuides_FallsBackOnMalformedEmbed(t *testing.T) {
	// A corrupt embed must NOT silently serve. The handler logs an
	// error AND falls back to static, so the boot is still useful but
	// an operator notices.
	original := generatedGuidesJSON
	generatedGuidesJSON = "{this is not valid json"
	defer func() { generatedGuidesJSON = original }()

	guides, source := loadGuides("https://app.toqui.test")

	if source != "static" {
		t.Errorf("expected source=static for malformed embed, got %q", source)
	}
	if len(guides) == 0 {
		t.Error("malformed-embed fallback returned 0 guides")
	}
}

func TestLoadGuides_FallsBackOnEmptyArray(t *testing.T) {
	// Edge case: well-formed JSON but the array is empty. Treat as
	// "no generated content" and fall back rather than serving an
	// empty list — `len(raw) > 0` guard.
	original := generatedGuidesJSON
	generatedGuidesJSON = "[]"
	defer func() { generatedGuidesJSON = original }()

	guides, source := loadGuides("https://app.toqui.test")

	if source != "static" {
		t.Errorf("expected source=static for empty-array embed, got %q", source)
	}
	if len(guides) == 0 {
		t.Error("expected static fallback to return guides")
	}
}

// --- deriveCTAText ---

func TestDeriveCTAText_HappyPath(t *testing.T) {
	got := deriveCTAText("Tokyo", "Hana")
	want := "Plan your Tokyo trip with Hana"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDeriveCTAText_EmptyDestinationDegrades(t *testing.T) {
	got := deriveCTAText("", "Hana")
	want := "Plan your trip with Hana"
	if got != want {
		t.Errorf("empty destination should still produce a usable CTA: got %q, want %q", got, want)
	}
}

func TestDeriveCTAText_EmptyPersonaDegrades(t *testing.T) {
	got := deriveCTAText("Tokyo", "")
	want := "Plan your Tokyo trip"
	if got != want {
		t.Errorf("empty persona should still produce a usable CTA: got %q, want %q", got, want)
	}
}

func TestDeriveCTAText_BothEmptyDegrades(t *testing.T) {
	// Defensive: if a future generator pass produces a guide with
	// neither field populated (shouldn't happen but the prompt could
	// regress), the CTA must still be a valid sentence — the
	// marketing site renders it as button text.
	got := deriveCTAText("", "")
	if got == "" {
		t.Error("CTA must never be empty — marketing site renders it as button text")
	}
	if !strings.Contains(got, "Plan your trip") {
		t.Errorf("expected fallback to contain 'Plan your trip', got %q", got)
	}
}
