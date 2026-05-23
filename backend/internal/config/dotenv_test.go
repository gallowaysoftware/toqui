package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseEnvFile(t *testing.T) {
	// Create a temp .env file
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env.test")

	content := `# This is a comment
PORT=9090
DATABASE_URL="postgres://user:pass@host/db"
SINGLE_QUOTED='hello world'
EMPTY_VALUE=
SPACES_AROUND = value_with_spaces
# Another comment

NO_EQUALS_LINE
ANTHROPIC_API_KEY=sk-test-key-123
`

	if err := os.WriteFile(envFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Clear any existing vars that might interfere
	envVars := []string{"PORT", "DATABASE_URL", "SINGLE_QUOTED", "EMPTY_VALUE",
		"SPACES_AROUND", "ANTHROPIC_API_KEY"}
	for _, key := range envVars {
		t.Setenv(key, "")
		os.Unsetenv(key)
	}

	// Parse the file
	if err := parseEnvFile(envFile); err != nil {
		t.Fatalf("parseEnvFile: %v", err)
	}

	tests := []struct {
		key      string
		expected string
	}{
		{"PORT", "9090"},
		{"DATABASE_URL", "postgres://user:pass@host/db"},
		{"SINGLE_QUOTED", "hello world"},
		{"EMPTY_VALUE", ""},
		{"SPACES_AROUND", "value_with_spaces"},
		{"ANTHROPIC_API_KEY", "sk-test-key-123"},
	}

	for _, tt := range tests {
		got := os.Getenv(tt.key)
		if got != tt.expected {
			t.Errorf("%s: got %q, want %q", tt.key, got, tt.expected)
		}
	}
}

func TestParseEnvFile_NoOverwrite(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env.test")

	content := `PORT=9090
API_KEY=from-file
`
	if err := os.WriteFile(envFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Pre-set PORT in the environment — should NOT be overwritten
	t.Setenv("PORT", "8080")
	// API_KEY is not set — should be set from file
	os.Unsetenv("API_KEY")

	if err := parseEnvFile(envFile); err != nil {
		t.Fatalf("parseEnvFile: %v", err)
	}

	if got := os.Getenv("PORT"); got != "8080" {
		t.Errorf("PORT should not be overwritten: got %q, want %q", got, "8080")
	}

	if got := os.Getenv("API_KEY"); got != "from-file" {
		t.Errorf("API_KEY should be set from file: got %q, want %q", got, "from-file")
	}
}

func TestParseEnvFile_MissingFile(t *testing.T) {
	// Missing file should not error — it's optional
	err := parseEnvFile("/nonexistent/.env.nope")
	if err != nil {
		t.Errorf("missing file should not error: %v", err)
	}
}

func TestLoadEnvFile(t *testing.T) {
	// loadEnvFile looks for env/.env.{name} relative to cwd.
	// Since we can't easily control cwd in tests, just verify the function exists
	// and handles missing files gracefully.
	err := loadEnvFile("nonexistent")
	if err != nil {
		t.Errorf("loadEnvFile with missing file should not error: %v", err)
	}
}
