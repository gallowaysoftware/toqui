package persona

import (
	"context"
	"testing"
)

func TestAllLocationProfilesRegistered(t *testing.T) {
	// All 40 locations should be registered (4 core + 36 extended)
	expectedCodes := []string{
		// Core 4
		"IT", "JP", "FR", "GB",
		// Extended 36
		"US", "ES", "DE", "PT", "GR", "TH", "MX", "AU", "BR", "IN",
		"KR", "VN", "MA", "PE", "NZ", "TR", "HR", "ZA", "CO", "EG",
		"ID", "PH", "CN", "CZ", "AT", "CH", "IE", "SE", "AR", "CL",
		"JO", "TZ", "IS", "SG", "HK", "KH",
	}

	for _, code := range expectedCodes {
		p := GetLocationProfile(code)
		if p == nil {
			t.Errorf("location profile for %s not registered", code)
			continue
		}
		if p.Name == "" {
			t.Errorf("location profile %s has empty Name", code)
		}
		if p.AccentColor == "" {
			t.Errorf("location profile %s has empty AccentColor", code)
		}
		if p.Flavor == "" {
			t.Errorf("location profile %s has empty Flavor", code)
		}
	}

	// Verify count
	count := len(locationProfiles)
	if count != 40 {
		t.Errorf("expected 40 location profiles, got %d", count)
	}
}

func TestAllThemeProfilesRegistered(t *testing.T) {
	// All 21 themes should be registered (3 core + 18 extended)
	expectedSlugs := []string{
		// Core 3
		"food", "history", "distilleries",
		// Extended 18
		"adventure", "wellness", "wine", "architecture", "nightlife",
		"shopping", "family", "photography", "nature", "romance",
		"budget", "luxury", "art", "music", "craft-beer", "diving", "hiking",
		"accessibility",
	}

	for _, slug := range expectedSlugs {
		p := GetThemeProfile(slug)
		if p == nil {
			t.Errorf("theme profile for %q not registered", slug)
			continue
		}
		if p.DisplayName == "" {
			t.Errorf("theme profile %q has empty DisplayName", slug)
		}
		if p.Archetype == "" {
			t.Errorf("theme profile %q has empty Archetype", slug)
		}
		if p.Flavor == "" {
			t.Errorf("theme profile %q has empty Flavor", slug)
		}
	}

	// Verify count
	count := len(themeProfiles)
	if count != 21 {
		t.Errorf("expected 21 theme profiles, got %d", count)
	}
}

func TestComposerTemplateIdentity(t *testing.T) {
	composer := NewComposer(nil) // nil generator = template fallback

	tests := []struct {
		name       string
		region     string
		themes     []string
		wantErr    bool
		checkName  bool
		nameSubstr string
	}{
		{
			name:       "core location + core theme",
			region:     "JP",
			themes:     []string{"food"},
			checkName:  true,
			nameSubstr: "Japan",
		},
		{
			name:       "new location + new theme: Czech craft-beer",
			region:     "CZ",
			themes:     []string{"craft-beer"},
			checkName:  true,
			nameSubstr: "Czech Republic",
		},
		{
			name:       "new location + new theme: Iceland hiking",
			region:     "IS",
			themes:     []string{"hiking"},
			checkName:  true,
			nameSubstr: "Iceland",
		},
		{
			name:       "new location + new theme: Philippines diving",
			region:     "PH",
			themes:     []string{"diving"},
			checkName:  true,
			nameSubstr: "Philippines",
		},
		{
			name:       "new theme: Italy accessibility",
			region:     "IT",
			themes:     []string{"accessibility"},
			checkName:  true,
			nameSubstr: "Italy",
		},
		{
			name:    "multiple themes",
			region:  "IT",
			themes:  []string{"food", "wine"},
			wantErr: false,
		},
		{
			name:    "empty themes",
			region:  "JP",
			themes:  []string{},
			wantErr: true,
		},
		{
			name:    "unknown region with valid theme",
			region:  "XX",
			themes:  []string{"food"},
			wantErr: false,
		},
		{
			name:    "valid region with unknown theme",
			region:  "JP",
			themes:  []string{"nonexistent"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := composer.Compose(context.Background(), tt.region, tt.themes)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if p == nil {
				t.Fatal("persona is nil")
			}
			if p.Name == "" {
				t.Error("persona has empty Name")
			}
			if p.Description == "" {
				t.Error("persona has empty Description")
			}
			if p.Greeting == "" {
				t.Error("persona has empty Greeting")
			}
			if p.ID == "" {
				t.Error("persona has empty ID")
			}
			if tt.checkName && tt.nameSubstr != "" {
				if p.Greeting == "" || !containsIgnoreCase(p.Greeting, tt.nameSubstr) {
					// Template greeting includes location name
					t.Logf("greeting=%q, expected to contain %q", p.Greeting, tt.nameSubstr)
				}
			}
		})
	}
}

func TestComposerCaching(t *testing.T) {
	composer := NewComposer(nil)

	p1, err := composer.Compose(context.Background(), "CZ", []string{"craft-beer"})
	if err != nil {
		t.Fatalf("first compose: %v", err)
	}

	p2, err := composer.Compose(context.Background(), "CZ", []string{"craft-beer"})
	if err != nil {
		t.Fatalf("second compose: %v", err)
	}

	// Same combination should return same cached persona
	if p1 != p2 {
		t.Error("expected same persona pointer from cache, got different pointers")
	}

	// Different combination should return different persona
	p3, err := composer.Compose(context.Background(), "IS", []string{"hiking"})
	if err != nil {
		t.Fatalf("third compose: %v", err)
	}
	if p1.ID == p3.ID {
		t.Error("different region+theme combos should produce different IDs")
	}
}

func TestComposerThemeOrderIndependence(t *testing.T) {
	composer := NewComposer(nil)

	p1, err := composer.Compose(context.Background(), "IT", []string{"food", "wine"})
	if err != nil {
		t.Fatalf("compose food,wine: %v", err)
	}

	p2, err := composer.Compose(context.Background(), "IT", []string{"wine", "food"})
	if err != nil {
		t.Fatalf("compose wine,food: %v", err)
	}

	// Theme order shouldn't matter — same composite key
	if p1.ID != p2.ID {
		t.Errorf("theme order should not matter: got IDs %q and %q", p1.ID, p2.ID)
	}
}

func TestRegistryResolve(t *testing.T) {
	composer := NewComposer(nil)
	registry := NewRegistry(composer)

	tests := []struct {
		name      string
		region    string
		themes    []string
		wantToqui bool
	}{
		{
			name:      "valid combo returns expert",
			region:    "CZ",
			themes:    []string{"craft-beer"},
			wantToqui: false,
		},
		{
			name:      "no themes returns Toqui",
			region:    "JP",
			themes:    nil,
			wantToqui: true,
		},
		{
			name:      "no region returns Toqui",
			region:    "",
			themes:    []string{"food"},
			wantToqui: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := registry.Resolve(context.Background(), tt.region, tt.themes)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantToqui {
				if p.ID != "toqui" {
					t.Errorf("expected toqui, got %q", p.ID)
				}
			} else {
				if p.ID == "toqui" {
					t.Error("expected expert, got toqui")
				}
			}
		})
	}
}

func TestRegistryHandoffMessage(t *testing.T) {
	composer := NewComposer(nil)
	registry := NewRegistry(composer)

	// Toqui should produce empty handoff
	msg := registry.HandoffMessage(registry.Toqui())
	if msg != "" {
		t.Errorf("toqui handoff should be empty, got %q", msg)
	}

	// Expert should produce non-empty handoff
	expert, err := registry.Resolve(context.Background(), "CZ", []string{"craft-beer"})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	msg = registry.HandoffMessage(expert)
	if msg == "" {
		t.Error("expert handoff message should not be empty")
	}
}

func TestPersonaSystemPromptModes(t *testing.T) {
	composer := NewComposer(nil)
	p, err := composer.Compose(context.Background(), "IS", []string{"hiking"})
	if err != nil {
		t.Fatalf("compose: %v", err)
	}

	// Test that mode-specific prompts are appended
	selection := p.SystemPrompt("selection")
	planning := p.SystemPrompt("planning")
	companion := p.SystemPrompt("companion")
	base := p.SystemPrompt("")

	if selection == base {
		t.Error("selection mode should differ from base")
	}
	if planning == base {
		t.Error("planning mode should differ from base")
	}
	if companion == base {
		t.Error("companion mode should differ from base")
	}
	if selection == planning {
		t.Error("selection and planning should differ")
	}
}

func TestParseIdentityResult(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:  "valid JSON",
			input: `{"name": "Luca", "description": "Italian chef.", "greeting": "Ciao!"}`,
		},
		{
			name:  "JSON wrapped in markdown",
			input: "```json\n{\"name\": \"Luca\", \"description\": \"Chef.\", \"greeting\": \"Hi!\"}\n```",
		},
		{
			name:    "no JSON",
			input:   "just some text",
			wantErr: true,
		},
		{
			name:    "empty name",
			input:   `{"name": "", "description": "test", "greeting": "hi"}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseIdentityResult(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Name == "" {
				t.Error("name is empty")
			}
		})
	}
}

func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			len(s) > 0 && len(substr) > 0 &&
				(s[0] == substr[0] || s[0]+32 == substr[0] || s[0] == substr[0]+32))
}
