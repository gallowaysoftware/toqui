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
	DailyAITokenBudget int // Max total tokens per day (0 = unlimited)

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
	DailyMessageLimit int // Deprecated: kept for backward compat; use tier-specific limits

	// Tier-specific daily message limits (0 = unlimited)
	DailyMessageLimitFree int
	DailyMessageLimitPro  int

	// AI provider priority: "gemini" (default) or "claude"
	AIProvider string

	// LLM response caching
	LLMCacheEnabled bool
	LLMCacheTTL     time.Duration

	// Stripe payment processing (Trip Pro + subscriptions)
	StripeSecretKey                string
	StripeWebhookSecret            string
	StripeTripProProductID         string // Stripe Product ID for Trip Pro one-time purchase
	TripProPriceCents              int    // Default 1900 ($19.00 CAD)
	StripeExplorerMonthlyProductID string
	StripeExplorerAnnualProductID  string
	StripeVoyagerMonthlyProductID  string
	StripeVoyagerAnnualProductID   string

	// Email (Resend) for transactional emails
	ResendAPIKey string
	EmailFrom    string

	// Analytics (PostHog)
	PostHogAPIKey string // Empty = analytics disabled

	// Signup restrictions
	AllowedEmailDomains []string // Empty = allow all
	AllowedEmails       []string // Emails that bypass waitlist/capacity entirely
	AdminEmails         []string // Emails allowed to access /admin/* endpoints

	// Referral
	ReferralMaxRewards int // Max referral trip unlocks a referrer can earn (default: 10)

	// GDPR export storage
	GCSExportBucket string // GCS bucket for GDPR data exports (empty = local filesystem fallback)
	ExportLocalDir  string // Local directory for exports when GCS is not configured

	// Staging overrides
	StagingProAll bool // When true, all trips are treated as unlocked (staging only)
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
		TargetEnv:                      env,
		Port:                           getEnv("PORT", "8090"),
		DatabaseURL:                    getEnv("DATABASE_URL", "postgres://toqui:toqui@localhost:5432/toqui?sslmode=disable"),
		GoogleClientID:                 os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret:             os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURI:              getEnv("GOOGLE_REDIRECT_URI", "http://localhost:8090/auth/google/callback"),
		FacebookClientID:               os.Getenv("FACEBOOK_CLIENT_ID"),
		FacebookClientSecret:           os.Getenv("FACEBOOK_CLIENT_SECRET"),
		FacebookRedirectURI:            getEnv("FACEBOOK_REDIRECT_URI", "http://localhost:8090/auth/facebook/callback"),
		JWTSecret:                      getEnv("JWT_SECRET", "dev-secret-change-in-production"),
		AnthropicAPIKey:                os.Getenv("ANTHROPIC_API_KEY"),
		GeminiAPIKey:                   os.Getenv("GEMINI_API_KEY"),
		VertexAIProjectID:              os.Getenv("VERTEX_AI_PROJECT_ID"),
		VertexAILocation:               getEnv("VERTEX_AI_LOCATION", "us-central1"),
		DailyAITokenBudget:             getEnvInt("DAILY_AI_TOKEN_BUDGET", 0),
		FirestoreProjectID:             getEnv("FIRESTORE_PROJECT_ID", "toqui-dev"),
		FirestoreDatabaseID:            getEnv("FIRESTORE_DATABASE_ID", ""),
		FirestoreEmulatorHost:          os.Getenv("FIRESTORE_EMULATOR_HOST"),
		FrontendURL:                    getEnv("FRONTEND_URL", "http://localhost:3000"),
		GoogleCustomSearchAPIKey:       os.Getenv("GOOGLE_CUSTOM_SEARCH_API_KEY"),
		GoogleCustomSearchCX:           os.Getenv("GOOGLE_CUSTOM_SEARCH_CX"),
		GooglePlacesAPIKey:             os.Getenv("GOOGLE_PLACES_API_KEY"),
		SendGridWebhookKey:             os.Getenv("SENDGRID_WEBHOOK_KEY"),
		SkyscannerAffiliateID:          os.Getenv("SKYSCANNER_AFFILIATE_ID"),
		BookingComAffiliateID:          os.Getenv("BOOKINGCOM_AFFILIATE_ID"),
		GetYourGuidePartnerID:          os.Getenv("GETYOURGUIDE_PARTNER_ID"),
		ViatorPartnerID:                os.Getenv("VIATOR_PARTNER_ID"),
		DiscoverCarsAffiliateID:        os.Getenv("DISCOVERCARS_AFFILIATE_ID"),
		SafetyWingReferenceID:          os.Getenv("SAFETYWING_REFERENCE_ID"),
		MaxFreeUsers:                   getEnvInt("MAX_FREE_USERS", 500),
		DailyMessageLimit:              getEnvInt("DAILY_MESSAGE_LIMIT", 30),
		DailyMessageLimitFree:          getEnvInt("DAILY_MESSAGE_LIMIT_FREE", 10),
		DailyMessageLimitPro:           getEnvInt("DAILY_MESSAGE_LIMIT_PRO", 50),
		AIProvider:                     getEnv("AI_PROVIDER", "gemini"),
		LLMCacheEnabled:                getEnvBool("LLM_CACHE_ENABLED", true),
		LLMCacheTTL:                    getEnvDuration("LLM_CACHE_TTL", time.Hour),
		PostHogAPIKey:                  os.Getenv("POSTHOG_API_KEY"),
		AllowedEmailDomains:            parseCSVEnv("ALLOWED_EMAIL_DOMAINS"),
		AllowedEmails:                  parseCSVEnv("ALLOWED_EMAILS"),
		AdminEmails:                    parseCSVEnv("ADMIN_EMAILS"),
		ReferralMaxRewards:             getEnvInt("REFERRAL_MAX_REWARDS", 10),
		StagingProAll:                  getEnvBool("STAGING_PRO_ALL", false),
		ResendAPIKey:                   os.Getenv("RESEND_API_KEY"),
		EmailFrom:                      getEnv("EMAIL_FROM", "Toqui <hello@toqui.travel>"),
		StripeSecretKey:                os.Getenv("STRIPE_SECRET_KEY"),
		StripeWebhookSecret:            os.Getenv("STRIPE_WEBHOOK_SECRET"),
		StripeTripProProductID:         os.Getenv("STRIPE_TRIP_PRO_PRODUCT_ID"),
		TripProPriceCents:              getEnvInt("TRIP_PRO_PRICE_CENTS", 1900),
		StripeExplorerMonthlyProductID: os.Getenv("STRIPE_EXPLORER_MONTHLY_PRODUCT"),
		StripeExplorerAnnualProductID:  os.Getenv("STRIPE_EXPLORER_ANNUAL_PRODUCT"),
		StripeVoyagerMonthlyProductID:  os.Getenv("STRIPE_VOYAGER_MONTHLY_PRODUCT"),
		StripeVoyagerAnnualProductID:   os.Getenv("STRIPE_VOYAGER_ANNUAL_PRODUCT"),
		CORSAllowedOrigins:             parseCSVEnv("CORS_ALLOWED_ORIGINS"),
		GCSExportBucket:                getEnv("GCS_EXPORT_BUCKET", ""),
		ExportLocalDir:                 getEnv("EXPORT_LOCAL_DIR", "/tmp/toqui-exports"),
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
