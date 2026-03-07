package auth

import (
	"testing"

	"github.com/google/uuid"
)

func TestValidateRefreshToken_RejectsAccessToken(t *testing.T) {
	svc := NewService("client-id", "client-secret", "http://localhost/callback", "test-secret")
	userID := uuid.New()

	// Generate an access token (no "type" claim)
	accessToken, err := svc.GenerateAccessToken(userID)
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}

	// ValidateRefreshToken should reject access tokens
	_, err = svc.ValidateRefreshToken(accessToken)
	if err == nil {
		t.Fatal("expected error when validating access token as refresh token")
	}
	if got := err.Error(); got != "invalid token type: expected refresh token" {
		t.Errorf("unexpected error: %s", got)
	}
}

func TestValidateRefreshToken_AcceptsRefreshToken(t *testing.T) {
	svc := NewService("client-id", "client-secret", "http://localhost/callback", "test-secret")
	userID := uuid.New()

	refreshToken, err := svc.GenerateRefreshToken(userID)
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}

	got, err := svc.ValidateRefreshToken(refreshToken)
	if err != nil {
		t.Fatalf("ValidateRefreshToken: %v", err)
	}
	if got != userID {
		t.Errorf("got userID %s, want %s", got, userID)
	}
}

func TestValidateToken_AcceptsAccessToken(t *testing.T) {
	svc := NewService("client-id", "client-secret", "http://localhost/callback", "test-secret")
	userID := uuid.New()

	accessToken, err := svc.GenerateAccessToken(userID)
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}

	got, err := svc.ValidateToken(accessToken)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if got != userID {
		t.Errorf("got userID %s, want %s", got, userID)
	}
}
