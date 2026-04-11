package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
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

func TestValidateToken_RejectsTokenMissingIssuer(t *testing.T) {
	svc := NewService("client-id", "client-secret", "http://localhost/callback", "test-secret")
	userID := uuid.New()

	// Craft a token without iss claim
	claims := jwt.MapClaims{
		"sub": userID.String(),
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
		"aud": jwtAudience,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(svc.jwtSecret)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	_, err = svc.ValidateToken(signed)
	if err == nil {
		t.Fatal("expected error when validating token without iss claim")
	}
}

func TestValidateToken_RejectsTokenMissingAudience(t *testing.T) {
	svc := NewService("client-id", "client-secret", "http://localhost/callback", "test-secret")
	userID := uuid.New()

	// Craft a token without aud claim
	claims := jwt.MapClaims{
		"sub": userID.String(),
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
		"iss": jwtIssuer,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(svc.jwtSecret)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	_, err = svc.ValidateToken(signed)
	if err == nil {
		t.Fatal("expected error when validating token without aud claim")
	}
}

func TestValidateToken_RejectsTokenWrongIssuer(t *testing.T) {
	svc := NewService("client-id", "client-secret", "http://localhost/callback", "test-secret")
	userID := uuid.New()

	claims := jwt.MapClaims{
		"sub": userID.String(),
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
		"iss": "wrong-issuer",
		"aud": jwtAudience,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(svc.jwtSecret)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	_, err = svc.ValidateToken(signed)
	if err == nil {
		t.Fatal("expected error when validating token with wrong issuer")
	}
}

func TestValidateToken_RejectsTokenWrongAudience(t *testing.T) {
	svc := NewService("client-id", "client-secret", "http://localhost/callback", "test-secret")
	userID := uuid.New()

	claims := jwt.MapClaims{
		"sub": userID.String(),
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
		"iss": jwtIssuer,
		"aud": "wrong-audience",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(svc.jwtSecret)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	_, err = svc.ValidateToken(signed)
	if err == nil {
		t.Fatal("expected error when validating token with wrong audience")
	}
}

func TestValidateRefreshToken_RejectsTokenMissingIssuer(t *testing.T) {
	svc := NewService("client-id", "client-secret", "http://localhost/callback", "test-secret")
	userID := uuid.New()

	claims := jwt.MapClaims{
		"sub":  userID.String(),
		"exp":  time.Now().Add(30 * 24 * time.Hour).Unix(),
		"iat":  time.Now().Unix(),
		"aud":  jwtAudience,
		"type": "refresh",
		"jti":  uuid.New().String(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(svc.jwtSecret)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	_, err = svc.ValidateRefreshToken(signed)
	if err == nil {
		t.Fatal("expected error when validating refresh token without iss claim")
	}
}

func TestValidateRefreshToken_RejectsTokenWrongAudience(t *testing.T) {
	svc := NewService("client-id", "client-secret", "http://localhost/callback", "test-secret")
	userID := uuid.New()

	claims := jwt.MapClaims{
		"sub":  userID.String(),
		"exp":  time.Now().Add(30 * 24 * time.Hour).Unix(),
		"iat":  time.Now().Unix(),
		"iss":  jwtIssuer,
		"aud":  "wrong-audience",
		"type": "refresh",
		"jti":  uuid.New().String(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(svc.jwtSecret)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	_, err = svc.ValidateRefreshToken(signed)
	if err == nil {
		t.Fatal("expected error when validating refresh token with wrong audience")
	}
}
