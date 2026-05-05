// genguides regenerates the curated set of destination guides from the
// persona system rather than the hand-authored prose currently shipped in
// internal/handlers/guides.go::staticGuides().
//
// This is PR 1 of the multi-PR feature for toqui-backend#30. PR 1 ships
// ONLY the tooling — no runtime path changes. The live GuidesHandler
// continues to read staticGuides() until PR 2 reviews the generated JSON
// and PR 3 flips the read path.
//
// Usage:
//
//	# Generate all guides into the gitignored output (default).
//	go run ./cmd/genguides
//
//	# Custom output paths (Make target uses these).
//	go run ./cmd/genguides \
//	  --output internal/handlers/guides_data.gen.json \
//	  --site-output ../toqui-site/src/data/guides.gen.ts
//
//	# Iterate on the prompt without writing files. Prints the first guide.
//	go run ./cmd/genguides --dry-run
//
//	# Compare generated content to staticGuides() side-by-side.
//	go run ./cmd/genguides --diff
//
// Requires either ANTHROPIC_API_KEY or GEMINI_API_KEY in the environment
// (same as the running server). The CLI will not run in CI — there are
// no API keys there. Run it on your dev machine when you want to refresh
// the artefact.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gallowaysoftware/toqui-backend/internal/ai"
	"github.com/gallowaysoftware/toqui-backend/internal/handlers"
	"github.com/gallowaysoftware/toqui-backend/internal/persona"
)

// guideSeed is one row in the curated set. Keep this list aligned with
// internal/handlers/guides.go::staticGuides() — the design proposal locked
// the curated subset at the existing 25 slugs. Expanding to all 989
// location×theme combinations is explicitly out of scope for v1.
type guideSeed struct {
	Slug        string
	RegionCode  string // ISO 3166-1 alpha-2 — passed to persona.Composer
	ThemeSlug   string // matches a persona.ThemeProfile slug
	Destination string // human-readable (city or country) for the guide payload
	Country     string // ISO country code surfaced in the guide payload
}

// curatedSeeds enumerates the 25 slugs the live API serves today. Order is
// preserved so generated output is deterministic.
var curatedSeeds = []guideSeed{
	{Slug: "tokyo-food", RegionCode: "JP", ThemeSlug: "food", Destination: "Tokyo", Country: "JP"},
	{Slug: "paris-culture", RegionCode: "FR", ThemeSlug: "history", Destination: "Paris", Country: "FR"},
	{Slug: "barcelona-nightlife", RegionCode: "ES", ThemeSlug: "nightlife", Destination: "Barcelona", Country: "ES"},
	{Slug: "rome-history", RegionCode: "IT", ThemeSlug: "history", Destination: "Rome", Country: "IT"},
	{Slug: "bangkok-street-food", RegionCode: "TH", ThemeSlug: "food", Destination: "Bangkok", Country: "TH"},
	{Slug: "london-history", RegionCode: "GB", ThemeSlug: "history", Destination: "London", Country: "GB"},
	{Slug: "marrakech-food", RegionCode: "MA", ThemeSlug: "food", Destination: "Marrakech", Country: "MA"},
	{Slug: "lisbon-food", RegionCode: "PT", ThemeSlug: "food", Destination: "Lisbon", Country: "PT"},
	{Slug: "mexico-city-food", RegionCode: "MX", ThemeSlug: "food", Destination: "Mexico City", Country: "MX"},
	{Slug: "istanbul-culture", RegionCode: "TR", ThemeSlug: "history", Destination: "Istanbul", Country: "TR"},
	{Slug: "bali-adventure", RegionCode: "ID", ThemeSlug: "adventure", Destination: "Bali", Country: "ID"},
	{Slug: "seoul-food", RegionCode: "KR", ThemeSlug: "food", Destination: "Seoul", Country: "KR"},
	{Slug: "scotland-distilleries", RegionCode: "GB", ThemeSlug: "distilleries", Destination: "Scotland", Country: "GB"},
	{Slug: "vietnam-food", RegionCode: "VN", ThemeSlug: "food", Destination: "Vietnam", Country: "VN"},
	{Slug: "iceland-nature", RegionCode: "IS", ThemeSlug: "nature", Destination: "Iceland", Country: "IS"},
	{Slug: "greece-romance", RegionCode: "GR", ThemeSlug: "romance", Destination: "Greece", Country: "GR"},
	{Slug: "new-york-food", RegionCode: "US", ThemeSlug: "food", Destination: "New York", Country: "US"},
	{Slug: "peru-adventure", RegionCode: "PE", ThemeSlug: "adventure", Destination: "Peru", Country: "PE"},
	{Slug: "amsterdam-art", RegionCode: "NL", ThemeSlug: "art", Destination: "Amsterdam", Country: "NL"},
	{Slug: "india-food", RegionCode: "IN", ThemeSlug: "food", Destination: "India", Country: "IN"},
	{Slug: "argentina-wine", RegionCode: "AR", ThemeSlug: "wine", Destination: "Mendoza", Country: "AR"},
	{Slug: "new-zealand-adventure", RegionCode: "NZ", ThemeSlug: "adventure", Destination: "New Zealand", Country: "NZ"},
	{Slug: "prague-history", RegionCode: "CZ", ThemeSlug: "history", Destination: "Prague", Country: "CZ"},
	{Slug: "japan-adventure", RegionCode: "JP", ThemeSlug: "adventure", Destination: "Japan", Country: "JP"},
	{Slug: "sydney-adventure", RegionCode: "AU", ThemeSlug: "adventure", Destination: "Sydney", Country: "AU"},
}

// generatedGuide is the JSON-serialised output. It mirrors handlers.Guide
// minus runtime-only fields (CTAText/CTAURL are derived at serve time
// from the appURL config in PR 3).
type generatedGuide struct {
	Slug             string `json:"slug"`
	Title            string `json:"title"`
	PersonaName      string `json:"persona_name"`
	PersonaSpecialty string `json:"persona_specialty"`
	Destination      string `json:"destination"`
	Country          string `json:"country"`
	Theme            string `json:"theme"`
	Excerpt          string `json:"excerpt"`
	Content          string `json:"content"`
}

func main() {
	var (
		outputPath     = flag.String("output", "internal/handlers/guides_data.gen.json", "Path to write generated JSON guide data.")
		siteOutputPath = flag.String("site-output", "../toqui-site/src/data/guides.gen.ts", "Path to write generated TypeScript guide data for toqui-site. Empty disables.")
		dryRun         = flag.Bool("dry-run", false, "Generate the first guide only and print it. No files written.")
		diff           = flag.Bool("diff", false, "Compare each generated guide to the existing staticGuides() entry and print a side-by-side report.")
		timeout        = flag.Duration("timeout", 10*time.Minute, "Total time budget for generation.")
	)
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	provider, err := buildProvider()
	if err != nil {
		fatal("AI provider init failed: %v", err)
	}

	composer := persona.NewComposer(newAIIdentityGenerator(provider))

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	seeds := curatedSeeds
	if *dryRun {
		seeds = curatedSeeds[:1]
		slog.Info("dry-run mode — generating first guide only")
	}

	existing := buildExistingIndex()

	results := make([]generatedGuide, 0, len(seeds))
	for i, seed := range seeds {
		slog.Info("generating guide", "slug", seed.Slug, "i", i+1, "of", len(seeds))
		g, err := generate(ctx, provider, composer, seed)
		if err != nil {
			fatal("generate %s: %v", seed.Slug, err)
		}
		results = append(results, g)

		if *diff {
			printDiff(seed.Slug, existing[seed.Slug], g)
		}
	}

	if *dryRun {
		// Pretty-print the single guide so the operator can eyeball it
		// before iterating on the prompt.
		out, _ := json.MarshalIndent(results[0], "", "  ")
		fmt.Println(string(out))
		return
	}

	if err := writeJSON(*outputPath, results); err != nil {
		fatal("write JSON: %v", err)
	}
	slog.Info("wrote backend guides JSON", "path", *outputPath, "count", len(results))

	if *siteOutputPath != "" {
		if err := writeSiteTS(*siteOutputPath, results); err != nil {
			fatal("write site TS: %v", err)
		}
		slog.Info("wrote toqui-site guides TS", "path", *siteOutputPath, "count", len(results))
	}
}

// buildProvider constructs an AI provider from environment credentials,
// matching the precedence used by cmd/server (Gemini Developer API → Vertex
// AI → Claude). The CLI does not load the full server config because it
// only needs AI credentials.
func buildProvider() (ai.Provider, error) {
	geminiKey := os.Getenv("GEMINI_API_KEY")
	vertexProject := os.Getenv("VERTEX_AI_PROJECT_ID")
	vertexLocation := os.Getenv("VERTEX_AI_LOCATION")
	if vertexLocation == "" {
		vertexLocation = "us-central1"
	}
	anthropicKey := os.Getenv("ANTHROPIC_API_KEY")

	var gemini ai.Provider
	if geminiKey != "" || vertexProject != "" {
		gp, err := ai.NewGeminiProvider(geminiKey, vertexProject, vertexLocation)
		if err != nil {
			slog.Warn("gemini provider init failed; falling back to claude", "error", err)
		} else {
			gemini = gp
		}
	}

	var claude ai.Provider
	if anthropicKey != "" {
		claude = ai.NewClaudeProvider(anthropicKey)
	}

	switch {
	case gemini != nil && claude != nil:
		return ai.NewFallbackProvider(gemini, claude), nil
	case gemini != nil:
		return gemini, nil
	case claude != nil:
		return claude, nil
	default:
		return nil, fmt.Errorf("set GEMINI_API_KEY, VERTEX_AI_PROJECT_ID (with ADC), or ANTHROPIC_API_KEY")
	}
}

// newAIIdentityGenerator mirrors the helper in cmd/server/main.go but kept
// local to avoid importing the server package (which pulls in the full
// service graph). PR 2/3 may consolidate these once the runtime path also
// reads from the JSON.
func newAIIdentityGenerator(provider ai.Provider) persona.IdentityGenerator {
	return func(ctx context.Context, req *persona.IdentityRequest) (*persona.IdentityResult, error) {
		prompt := persona.IdentityGeneratorPrompt(req)
		aiReq := &ai.ChatRequest{
			SystemPrompt: "You are a creative writer who generates character identities for AI travel guides. Respond with JSON only.",
			Messages:     []ai.Message{{Role: "user", Content: prompt}},
			MaxTokens:    256,
			Temperature:  0.8,
			ModelTier:    ai.ModelTierFast,
		}
		text, err := streamToString(ctx, provider, aiReq)
		if err != nil {
			return nil, err
		}
		return persona.ParseIdentityResult(text)
	}
}

// generate drives the full pipeline for one seed: compose the persona to
// pull a stable name+specialty, build the guide prompt with the safety
// rules, run the AI, and parse the structured output.
func generate(ctx context.Context, provider ai.Provider, composer *persona.Composer, seed guideSeed) (generatedGuide, error) {
	composed, err := composer.Compose(ctx, seed.RegionCode, []string{seed.ThemeSlug})
	if err != nil {
		return generatedGuide{}, fmt.Errorf("compose persona: %w", err)
	}

	loc := persona.GetLocationProfile(seed.RegionCode)
	theme := persona.GetThemeProfile(seed.ThemeSlug)
	prompt := persona.BuildGuidePrompt(loc, theme, persona.GuidePromptOptions{
		PersonaName:      composed.Name,
		PersonaSpecialty: composed.Description,
	})

	aiReq := &ai.ChatRequest{
		SystemPrompt: "You write factual, neighborhood-level destination guides. You follow output format instructions exactly and never invent specific business names.",
		Messages:     []ai.Message{{Role: "user", Content: prompt}},
		MaxTokens:    2048,
		Temperature:  0.6,
		ModelTier:    ai.ModelTierSmart,
	}
	body, err := streamToString(ctx, provider, aiReq)
	if err != nil {
		return generatedGuide{}, fmt.Errorf("AI call: %w", err)
	}

	title, excerpt, content, err := parseGuideOutput(body)
	if err != nil {
		return generatedGuide{}, fmt.Errorf("parse guide output: %w\n--- raw ---\n%s", err, body)
	}

	return generatedGuide{
		Slug:             seed.Slug,
		Title:            title,
		PersonaName:      composed.Name,
		PersonaSpecialty: composed.Description,
		Destination:      seed.Destination,
		Country:          seed.Country,
		Theme:            seed.ThemeSlug,
		Excerpt:          excerpt,
		Content:          content,
	}, nil
}

// streamToString collects an AI streaming response into a single string.
func streamToString(ctx context.Context, provider ai.Provider, req *ai.ChatRequest) (string, error) {
	ch, err := provider.ChatStream(ctx, req)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for ev := range ch {
		switch ev.Type {
		case ai.EventTextDelta:
			b.WriteString(ev.Text)
		case ai.EventError:
			return "", ev.Error
		}
	}
	return b.String(), nil
}

// parseGuideOutput pulls the front-matter title/excerpt and the body from
// the AI response. The prompt asks for a YAML-ish front matter delimited
// by `---` lines; everything after the closing delimiter is the body.
func parseGuideOutput(raw string) (title, excerpt, content string, err error) {
	s := strings.TrimSpace(raw)
	// Strip an optional ```markdown / ``` fence.
	if strings.HasPrefix(s, "```") {
		if nl := strings.Index(s, "\n"); nl != -1 {
			s = s[nl+1:]
		}
		if idx := strings.LastIndex(s, "```"); idx != -1 {
			s = s[:idx]
		}
		s = strings.TrimSpace(s)
	}
	if !strings.HasPrefix(s, "---") {
		return "", "", "", fmt.Errorf("output did not start with --- front matter")
	}
	rest := strings.TrimPrefix(s, "---")
	rest = strings.TrimLeft(rest, "\n")
	closeIdx := strings.Index(rest, "\n---")
	if closeIdx == -1 {
		return "", "", "", fmt.Errorf("front matter not closed by ---")
	}
	frontMatter := rest[:closeIdx]
	content = strings.TrimSpace(rest[closeIdx+len("\n---"):])

	for _, line := range strings.Split(frontMatter, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "title:"):
			title = strings.TrimSpace(strings.TrimPrefix(line, "title:"))
		case strings.HasPrefix(line, "excerpt:"):
			excerpt = strings.TrimSpace(strings.TrimPrefix(line, "excerpt:"))
		}
	}
	if title == "" {
		return "", "", "", fmt.Errorf("front matter missing title")
	}
	if excerpt == "" {
		return "", "", "", fmt.Errorf("front matter missing excerpt")
	}
	if content == "" {
		return "", "", "", fmt.Errorf("body content empty")
	}
	// Strip surrounding quotes if the model wrapped values.
	title = strings.Trim(title, `"'`)
	excerpt = strings.Trim(excerpt, `"'`)
	return title, excerpt, content, nil
}

// writeJSON serialises results to a stable, human-reviewable JSON file.
// Indented output minimises diff churn when PR 2 commits the artefact.
func writeJSON(path string, guides []generatedGuide) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(guides, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

// writeSiteTS serialises results to the toqui-site TS module shape. The
// existing guides.ts uses single-line entries; we use json.Marshal per
// field so escaping is correct and write a deterministic format.
func writeSiteTS(path string, guides []generatedGuide) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("// AUTO-GENERATED by toqui-backend cmd/genguides. Do not edit by hand.\n")
	b.WriteString("// Run `make genguides` in toqui-backend to regenerate.\n\n")
	b.WriteString("export interface Guide {\n")
	b.WriteString("  slug: string;\n")
	b.WriteString("  title: string;\n")
	b.WriteString("  personaName: string;\n")
	b.WriteString("  personaSpecialty: string;\n")
	b.WriteString("  destination: string;\n")
	b.WriteString("  country: string;\n")
	b.WriteString("  theme: string;\n")
	b.WriteString("  excerpt: string;\n")
	b.WriteString("  content: string;\n")
	b.WriteString("}\n\n")
	b.WriteString("export const guides: Guide[] = [\n")
	for _, g := range guides {
		b.WriteString("  {\n")
		writeTSField(&b, "slug", g.Slug)
		writeTSField(&b, "title", g.Title)
		writeTSField(&b, "personaName", g.PersonaName)
		writeTSField(&b, "personaSpecialty", g.PersonaSpecialty)
		writeTSField(&b, "destination", g.Destination)
		writeTSField(&b, "country", g.Country)
		writeTSField(&b, "theme", g.Theme)
		writeTSField(&b, "excerpt", g.Excerpt)
		writeTSField(&b, "content", g.Content)
		b.WriteString("  },\n")
	}
	b.WriteString("];\n")
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func writeTSField(b *strings.Builder, key, value string) {
	encoded, _ := json.Marshal(value)
	fmt.Fprintf(b, "    %s: %s,\n", key, encoded)
}

// buildExistingIndex pulls handlers.staticGuides for diffing. The CLI
// passes an empty appURL because we don't compare CTA fields.
func buildExistingIndex() map[string]handlers.Guide {
	h := handlers.NewGuidesHandler("")
	out := make(map[string]handlers.Guide)
	for _, g := range h.Guides() {
		out[g.Slug] = g
	}
	return out
}

func printDiff(slug string, old handlers.Guide, gen generatedGuide) {
	fmt.Printf("\n=== %s ===\n", slug)
	fmt.Printf("[ existing ] title: %s\n", old.Title)
	fmt.Printf("[ generated] title: %s\n", gen.Title)
	fmt.Printf("[ existing ] excerpt: %s\n", truncate(old.Excerpt, 120))
	fmt.Printf("[ generated] excerpt: %s\n", truncate(gen.Excerpt, 120))
	fmt.Printf("[ existing ] content: %d chars\n", len(old.Content))
	fmt.Printf("[ generated] content: %d chars\n", len(gen.Content))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "genguides: "+format+"\n", args...)
	os.Exit(1)
}
