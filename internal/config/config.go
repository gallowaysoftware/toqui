package config

import (
	"fmt"
	"os"
)

type Config struct {
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
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:                  getEnv("PORT", "8090"),
		DatabaseURL:           getEnv("DATABASE_URL", "postgres://toqui:toqui@localhost:5432/toqui?sslmode=disable"),
		GoogleClientID:        os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret:    os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURI:     getEnv("GOOGLE_REDIRECT_URI", "http://localhost:8090/auth/google/callback"),
		JWTSecret:             getEnv("JWT_SECRET", "dev-secret-change-in-production"),
		AnthropicAPIKey:       os.Getenv("ANTHROPIC_API_KEY"),
		OpenAIAPIKey:          os.Getenv("OPENAI_API_KEY"),
		FirestoreProjectID:    getEnv("FIRESTORE_PROJECT_ID", "toqui-dev"),
		FirestoreEmulatorHost: os.Getenv("FIRESTORE_EMULATOR_HOST"),
		FrontendURL:           getEnv("FRONTEND_URL", "http://localhost:3000"),
	}

	if cfg.JWTSecret == "dev-secret-change-in-production" {
		fmt.Println("WARNING: Using default JWT secret. Set JWT_SECRET in production.")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
