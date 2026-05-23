package config

import (
	"bufio"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// loadEnvFile loads environment variables from env/.env.{env}.
// Variables already set in the process environment are not overwritten.
// Returns nil if the file does not exist (file is optional).
func loadEnvFile(env string) error {
	path := filepath.Join("env", ".env."+env)
	return parseEnvFile(path)
}

// parseEnvFile reads a .env file and sets environment variables.
// Handles comments (#), blank lines, and quoted values (single/double).
// Existing environment variables are never overwritten.
func parseEnvFile(path string) error {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		slog.Debug("env file not found, using defaults", "path", path)
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()

	loaded := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		// Strip surrounding quotes
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}

		// Don't overwrite existing env vars
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
			loaded++
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	slog.Info("loaded env file", "path", path, "vars", loaded)
	return nil
}
