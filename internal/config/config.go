package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all configuration for the Toqui backend.
// Values are loaded in three layers:
//  1. Parse env/.env.{TARGET_ENV} file (no-overwrite of existing env vars)
//  2. Read os.Getenv() with defaults
//  3. Resolve gcsm:// prefixes via GCP Secret Manager
type Config struct {
	// TargetEnv is the environment name: "local", "staging", or "prod".
	TargetEnv string

	Port        string
	DatabaseURL string

	// Google OAuth
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURI  string

	// Facebook/Meta OAuth (covers Facebook + Instagram login)
	FacebookClientID     string
	FacebookClientSecret string
	FacebookRedirectURI  string

	// Apple Sign-In
	// All four fields are required to enable Apple Sign-In; when any is
	// empty, the AppleLogin RPC returns Unimplemented (deliberate — the
	// backend ships before Apple Developer enrollment completes).
	//
	// Note: AppleServicesID is the Services ID configured in the Apple
	// Developer portal, NOT the iOS app bundle ID. ApplePrivateKey holds
	// the PEM-encoded contents of the .p8 key (supports gcsm:// resolution).
	AppleTeamID     string
	AppleServicesID string
	AppleKeyID      string
	ApplePrivateKey string

	// JWT
	JWTSecret string

	// AI providers
	AnthropicAPIKey string

	// Gemini — supports two backends (both use Gemini 3 models):
	// 1. Developer API (generativelanguage.googleapis.com) — uses API key
	// 2. Vertex AI (aiplatform.googleapis.com, global endpoint) — uses ADC
	// When GeminiAPIKey is set, the Developer API is used (preferred).
	// When only VertexAIProjectID is set, Vertex AI is used as fallback.
	GeminiAPIKey      string // Gemini Developer API key (from Secret Manager)
	VertexAIProjectID string // GCP project for Vertex AI calls (fallback)
	VertexAILocation  string // Overridden to "global" for Gemini 3 (config default: us-central1)

	// Cost control
	DailyAITokenBudget int // Max total tokens per day (0 = unlimited) — legacy in-memory guard

	// AI daily cost budget — DB-backed hard limit on total AI spend per day.
	// Expressed in cents (e.g. 50000 = $500/day). 0 = unlimited.
	AIDailyBudgetCents int

	// Firestore
	FirestoreProjectID    string
	FirestoreDatabaseID   string
	FirestoreEmulatorHost string

	// Frontend URL for CORS (primary origin)
	FrontendURL string

	// CORSAllowedOrigins is the full list of allowed CORS origins.
	// If empty, defaults to FrontendURL only.
	CORSAllowedOrigins []string

	// Google APIs (tools)
	GoogleCustomSearchAPIKey string
	GoogleCustomSearchCX     string
	GooglePlacesAPIKey       string

	// AI provider priority: "gemini" (default) or "claude"
	AIProvider string

	// LLM response caching
	LLMCacheEnabled bool
	LLMCacheTTL     time.Duration

	// Email (Resend) for transactional emails
	ResendAPIKey string
	EmailFrom    string

	// Signup restrictions
	AllowedEmailDomains []string // Empty = allow all

	// GDPR export storage
	GCSExportBucket string // GCS bucket for GDPR data exports (empty = local filesystem fallback)
	ExportLocalDir  string // Local directory for exports when GCS is not configured
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
		TargetEnv:                 env,
		Port:                      getEnv("PORT", "8090"),
		DatabaseURL:               getEnv("DATABASE_URL", "postgres://toqui:toqui@localhost:5432/toqui?sslmode=disable"),
		GoogleClientID:            os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret:        os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURI:         getEnv("GOOGLE_REDIRECT_URI", "http://localhost:8090/auth/google/callback"),
		FacebookClientID:          os.Getenv("FACEBOOK_CLIENT_ID"),
		FacebookClientSecret:      os.Getenv("FACEBOOK_CLIENT_SECRET"),
		FacebookRedirectURI:       getEnv("FACEBOOK_REDIRECT_URI", "http://localhost:8090/auth/facebook/callback"),
		AppleTeamID:               os.Getenv("APPLE_TEAM_ID"),
		AppleServicesID:           os.Getenv("APPLE_SERVICES_ID"),
		AppleKeyID:                os.Getenv("APPLE_KEY_ID"),
		ApplePrivateKey:           os.Getenv("APPLE_PRIVATE_KEY"),
		JWTSecret:                 getEnv("JWT_SECRET", "dev-secret-change-in-production"),
		AnthropicAPIKey:           os.Getenv("ANTHROPIC_API_KEY"),
		GeminiAPIKey:              os.Getenv("GEMINI_API_KEY"),
		VertexAIProjectID:         os.Getenv("VERTEX_AI_PROJECT_ID"),
		VertexAILocation:          getEnv("VERTEX_AI_LOCATION", "us-central1"),
		DailyAITokenBudget:        getEnvInt("DAILY_AI_TOKEN_BUDGET", 0),
		AIDailyBudgetCents:        getEnvInt("AI_DAILY_BUDGET_CENTS", 0),
		FirestoreProjectID:        getEnv("FIRESTORE_PROJECT_ID", "toqui-dev"),
		FirestoreDatabaseID:       getEnv("FIRESTORE_DATABASE_ID", ""),
		FirestoreEmulatorHost:     os.Getenv("FIRESTORE_EMULATOR_HOST"),
		FrontendURL:               getEnv("FRONTEND_URL", "http://localhost:3000"),
		GoogleCustomSearchAPIKey:  os.Getenv("GOOGLE_CUSTOM_SEARCH_API_KEY"),
		GoogleCustomSearchCX:      os.Getenv("GOOGLE_CUSTOM_SEARCH_CX"),
		GooglePlacesAPIKey:        os.Getenv("GOOGLE_PLACES_API_KEY"),
		AIProvider:                getEnv("AI_PROVIDER", "gemini"),
		LLMCacheEnabled:           getEnvBool("LLM_CACHE_ENABLED", true),
		LLMCacheTTL:               getEnvDuration("LLM_CACHE_TTL", time.Hour),
		AllowedEmailDomains:       parseCSVEnv("ALLOWED_EMAIL_DOMAINS"),
		ResendAPIKey:              os.Getenv("RESEND_API_KEY"),
		EmailFrom:                 getEnv("EMAIL_FROM", "Toqui <hello@toqui.travel>"),
		CORSAllowedOrigins:        parseCSVEnv("CORS_ALLOWED_ORIGINS"),
		GCSExportBucket:           getEnv("GCS_EXPORT_BUCKET", ""),
		ExportLocalDir:            getEnv("EXPORT_LOCAL_DIR", "/tmp/toqui-exports"),
	}

	// Layer 3: resolve gcsm:// references
	if err := resolveSecrets(cfg); err != nil {
		return nil, fmt.Errorf("resolve secrets: %w", err)
	}

	if cfg.JWTSecret == "dev-secret-change-in-production" {
		if cfg.TargetEnv != "local" {
			return nil, fmt.Errorf("JWT_SECRET must be set in %s environment (default dev secret is not allowed)", cfg.TargetEnv)
		}
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

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		slog.Warn("invalid integer env var, using default", "key", key, "value", v, "default", fallback)
		return fallback
	}
	return n
}

func getEnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		slog.Warn("invalid boolean env var, using default", "key", key, "value", v, "default", fallback)
		return fallback
	}
	return b
}

func parseCSVEnv(key string) []string {
	v := os.Getenv(key)
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		slog.Warn("invalid duration env var, using default", "key", key, "value", v, "default", fallback)
		return fallback
	}
	return d
}
