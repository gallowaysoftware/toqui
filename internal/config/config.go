package config

import (
	"fmt"
	"log/slog"
	"os"
)

// Config holds all configuration for the Toqui backend.
// Values are loaded in three layers:
//  1. Parse env/.env.{TARGET_ENV} file (no-overwrite of existing env vars)
//  2. Read os.Getenv() with defaults
//  3. Resolve gcsm:// prefixes via GCP Secret Manager (staging/prod only)
type Config struct {
	// TargetEnv is the environment name: "local", "staging", or "prod".
	TargetEnv string

	Port        string
	DatabaseURL string

	// Google OAuth
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURI  string

	// JWT
	JWTSecret string

	// AI providers
	AnthropicAPIKey string
	OpenAIAPIKey    string

	// Firestore
	FirestoreProjectID    string
	FirestoreEmulatorHost string

	// Frontend URL for CORS
	FrontendURL string

	// Google APIs (tools)
	GoogleCustomSearchAPIKey string
	GoogleCustomSearchCX     string
	GooglePlacesAPIKey       string
}

// Load builds a Config using the three-layer loading strategy:
// env file → os.Getenv with defaults → GCP Secret Manager resolution.
func Load() (*Config, error) {
	env := getEnv("TARGET_ENV", "local")

	// Layer 1: load env file (does not overwrite existing env vars)
	if err := loadEnvFile(env); err != nil {
		return nil, fmt.Errorf("load env file for %s: %w", env, err)
	}

	// Layer 2: read env vars with defaults
	cfg := &Config{
		TargetEnv:                env,
		Port:                     getEnv("PORT", "8090"),
		DatabaseURL:              getEnv("DATABASE_URL", "postgres://toqui:toqui@localhost:5432/toqui?sslmode=disable"),
		GoogleClientID:           os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret:       os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURI:        getEnv("GOOGLE_REDIRECT_URI", "http://localhost:8090/auth/google/callback"),
		JWTSecret:                getEnv("JWT_SECRET", "dev-secret-change-in-production"),
		AnthropicAPIKey:          os.Getenv("ANTHROPIC_API_KEY"),
		OpenAIAPIKey:             os.Getenv("OPENAI_API_KEY"),
		FirestoreProjectID:       getEnv("FIRESTORE_PROJECT_ID", "toqui-dev"),
		FirestoreEmulatorHost:    os.Getenv("FIRESTORE_EMULATOR_HOST"),
		FrontendURL:              getEnv("FRONTEND_URL", "http://localhost:3000"),
		GoogleCustomSearchAPIKey: os.Getenv("GOOGLE_CUSTOM_SEARCH_API_KEY"),
		GoogleCustomSearchCX:     os.Getenv("GOOGLE_CUSTOM_SEARCH_CX"),
		GooglePlacesAPIKey:       os.Getenv("GOOGLE_PLACES_API_KEY"),
	}

	// Layer 3: resolve gcsm:// references (only for non-local environments)
	if env != "local" {
		if err := resolveSecrets(cfg); err != nil {
			return nil, fmt.Errorf("resolve secrets: %w", err)
		}
	}

	if cfg.TargetEnv == "local" && cfg.JWTSecret == "dev-secret-change-in-production" {
		slog.Warn("using default JWT secret — set JWT_SECRET for non-local environments")
	}

	slog.Info("config loaded", "env", env, "port", cfg.Port)
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
