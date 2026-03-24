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

	// JWT
	JWTSecret string

	// AI providers
	AnthropicAPIKey string

	// Vertex AI (Gemini fallback) — uses ADC, no API key needed
	VertexAIProjectID string // GCP project for Vertex AI calls
	VertexAILocation  string // Region (default: us-central1)

	// Cost control
	DailyAITokenBudget int // Max total tokens per day (0 = unlimited)

	// Firestore
	FirestoreProjectID    string
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

	// Email ingestion
	SendGridWebhookKey string

	// Affiliate partners
	SkyscannerAffiliateID   string
	BookingComAffiliateID   string
	GetYourGuidePartnerID   string
	ViatorPartnerID         string
	DiscoverCarsAffiliateID string
	SafetyWingReferenceID   string

	// Capacity + usage limits
	MaxFreeUsers      int
	DailyMessageLimit int

	// LLM response caching
	LLMCacheEnabled bool
	LLMCacheTTL     time.Duration

	// Helcim payment processing
	HelcimAPIToken    string
	TripProPriceCents int // Default 1200 ($12.00 CAD)

	// Signup restrictions
	AllowedEmailDomains []string // Empty = allow all
	AllowedEmails       []string // Emails that bypass waitlist/capacity entirely
	AdminEmails         []string // Emails allowed to access /admin/* endpoints
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
		VertexAIProjectID:        os.Getenv("VERTEX_AI_PROJECT_ID"),
		VertexAILocation:         getEnv("VERTEX_AI_LOCATION", "us-central1"),
		DailyAITokenBudget:       getEnvInt("DAILY_AI_TOKEN_BUDGET", 0),
		FirestoreProjectID:       getEnv("FIRESTORE_PROJECT_ID", "toqui-dev"),
		FirestoreEmulatorHost:    os.Getenv("FIRESTORE_EMULATOR_HOST"),
		FrontendURL:              getEnv("FRONTEND_URL", "http://localhost:3000"),
		GoogleCustomSearchAPIKey: os.Getenv("GOOGLE_CUSTOM_SEARCH_API_KEY"),
		GoogleCustomSearchCX:     os.Getenv("GOOGLE_CUSTOM_SEARCH_CX"),
		GooglePlacesAPIKey:       os.Getenv("GOOGLE_PLACES_API_KEY"),
		SendGridWebhookKey:       os.Getenv("SENDGRID_WEBHOOK_KEY"),
		SkyscannerAffiliateID:    os.Getenv("SKYSCANNER_AFFILIATE_ID"),
		BookingComAffiliateID:    os.Getenv("BOOKINGCOM_AFFILIATE_ID"),
		GetYourGuidePartnerID:    os.Getenv("GETYOURGUIDE_PARTNER_ID"),
		ViatorPartnerID:          os.Getenv("VIATOR_PARTNER_ID"),
		DiscoverCarsAffiliateID:  os.Getenv("DISCOVERCARS_AFFILIATE_ID"),
		SafetyWingReferenceID:    os.Getenv("SAFETYWING_REFERENCE_ID"),
		MaxFreeUsers:             getEnvInt("MAX_FREE_USERS", 500),
		DailyMessageLimit:        getEnvInt("DAILY_MESSAGE_LIMIT", 30),
		LLMCacheEnabled:          getEnvBool("LLM_CACHE_ENABLED", true),
		LLMCacheTTL:              getEnvDuration("LLM_CACHE_TTL", time.Hour),
		AllowedEmailDomains:      parseCSVEnv("ALLOWED_EMAIL_DOMAINS"),
		AllowedEmails:            parseCSVEnv("ALLOWED_EMAILS"),
		AdminEmails:              parseCSVEnv("ADMIN_EMAILS"),
		HelcimAPIToken:           os.Getenv("HELCIM_API_TOKEN"),
		TripProPriceCents:        getEnvInt("TRIP_PRO_PRICE_CENTS", 1200),
		CORSAllowedOrigins:       parseCSVEnv("CORS_ALLOWED_ORIGINS"),
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

	if cfg.HelcimAPIToken == "" && cfg.TargetEnv != "local" {
		slog.Warn("HELCIM_API_TOKEN not set — payment processing will fail")
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
