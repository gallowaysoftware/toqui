//go:build integration

package integration

import (
	"testing"

	"github.com/google/uuid"
	"github.com/gallowaysoftware/toqui-backend/internal/auth"
)

func TestAuthTokenFlow(t *testing.T) {
	svc := auth.NewService("test-client-id", "test-secret", "http://localhost/callback", "test-jwt-secret-32chars!!")

	userID := uuid.New()

	// Generate access token
	accessToken, err := svc.GenerateAccessToken(userID)
	if err != nil {
		t.Fatalf("generate access token: %v", err)
	}
	if accessToken == "" {
		t.Fatal("access token is empty")
	}

	// Validate access token
	gotID, err := svc.ValidateToken(accessToken)
	if err != nil {
		t.Fatalf("validate access token: %v", err)
	}
	if gotID != userID {
		t.Errorf("validated ID = %v, want %v", gotID, userID)
	}

	// Generate refresh token. Pass uuid.Nil as family to start a new
	// token family (initial login, not a rotation).
	refreshToken, err := svc.GenerateRefreshToken(userID, uuid.Nil)
	if err != nil {
		t.Fatalf("generate refresh token: %v", err)
	}

	// Validate refresh token. Returns claims (UserID, JTI, Family) —
	// we only assert UserID here, server-side JTI tracking is covered
	// by unit tests in internal/auth.
	claims, err := svc.ValidateRefreshToken(refreshToken.Token)
	if err != nil {
		t.Fatalf("validate refresh token: %v", err)
	}
	if claims.UserID != userID {
		t.Errorf("refresh validated ID = %v, want %v", claims.UserID, userID)
	}

	// Invalid token should fail
	_, err = svc.ValidateToken("invalid-token")
	if err == nil {
		t.Error("expected error for invalid token, got nil")
	}
}
