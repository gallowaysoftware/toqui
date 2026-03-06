package handlers

import (
	"github.com/gallowaysoftware/toqui-backend/internal/auth"
)

// newTestAuthService creates an auth.Service for unit tests with a known secret.
func newTestAuthService() *auth.Service {
	return auth.NewService("test-client-id", "test-client-secret", "http://localhost/callback", "test-jwt-secret")
}
