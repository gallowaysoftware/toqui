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

	result, err := svc.GenerateRefreshToken(userID, uuid.Nil)
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}

	claims, err := svc.ValidateRefreshToken(result.Token)
	if err != nil {
		t.Fatalf("ValidateRefreshToken: %v", err)
	}
	if claims.UserID != userID {
		t.Errorf("got userID %s, want %s", claims.UserID, userID)
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

func TestRefreshToken_ContainsJTIAndFamily(t *testing.T) {
	svc := NewService("client-id", "client-secret", "http://localhost/callback", "test-secret")
	userID := uuid.New()

	result, err := svc.GenerateRefreshToken(userID, uuid.Nil)
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}

	if result.JTI == "" {
		t.Error("expected non-empty JTI")
	}
	if result.Family == uuid.Nil {
		t.Error("expected non-nil family UUID")
	}

	claims, err := svc.ValidateRefreshToken(result.Token)
	if err != nil {
		t.Fatalf("ValidateRefreshToken: %v", err)
	}
	if claims.JTI != result.JTI {
		t.Errorf("got JTI %s, want %s", claims.JTI, result.JTI)
	}
	if claims.Family != result.Family {
		t.Errorf("got family %s, want %s", claims.Family, result.Family)
	}
}

func TestRefreshToken_RotationPreservesFamily(t *testing.T) {
	svc := NewService("client-id", "client-secret", "http://localhost/callback", "test-secret")
	userID := uuid.New()
	family := uuid.New()

	result, err := svc.GenerateRefreshToken(userID, family)
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}

	if result.Family != family {
		t.Errorf("expected family %s, got %s", family, result.Family)
	}

	claims, err := svc.ValidateRefreshToken(result.Token)
	if err != nil {
		t.Fatalf("ValidateRefreshToken: %v", err)
	}
	if claims.Family != family {
		t.Errorf("got family %s, want %s", claims.Family, family)
	}
}

func TestRefreshToken_UniqueJTIs(t *testing.T) {
	svc := NewService("client-id", "client-secret", "http://localhost/callback", "test-secret")
	userID := uuid.New()
	family := uuid.New()

	r1, _ := svc.GenerateRefreshToken(userID, family)
	r2, _ := svc.GenerateRefreshToken(userID, family)

	if r1.JTI == r2.JTI {
		t.Error("expected different JTIs for different tokens")
	}
}
