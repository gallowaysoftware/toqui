package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type contextKey string

const userIDKey contextKey = "user_id"

// JWT issuer and audience constants for claim validation.
const (
	jwtIssuer   = "toqui"
	jwtAudience = "toqui-api"
)

type GoogleUserInfo struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	AvatarURL string `json:"picture"`
}

type Service struct {
	oauthConfig *oauth2.Config
	jwtSecret   []byte
}

func NewService(clientID, clientSecret, redirectURI, jwtSecret string) *Service {
	return &Service{
		oauthConfig: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURI,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     google.Endpoint,
		},
		jwtSecret: []byte(jwtSecret),
	}
}

// GeneratePKCE creates a PKCE code verifier and its S256 challenge.
func GeneratePKCE() (verifier, challenge string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generate PKCE verifier: %w", err)
	}
	verifier = base64.RawURLEncoding.EncodeToString(buf)
	h := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(h[:])
	return verifier, challenge, nil
}

// AuthCodeURL returns the Google OAuth authorization URL with PKCE S256 challenge.
func (s *Service) AuthCodeURL(state, codeChallenge string) string {
	return s.oauthConfig.AuthCodeURL(state,
		oauth2.SetAuthURLParam("code_challenge", codeChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)
}

// AllowedRedirectURIs is the set of redirect URIs that clients may use.
// The server's configured redirect URI is always allowed.
var AllowedRedirectURIs = map[string]bool{}

// ExchangeCodeOpts holds optional parameters for ExchangeCode.
type ExchangeCodeOpts struct {
	RedirectURI  string // Override redirect URI (validated against allowlist)
	CodeVerifier string // PKCE code verifier for S256 challenge verification
}

// ExchangeCode exchanges a Google authorization code for user info.
// Accepts optional ExchangeCodeOpts for redirect URI override and PKCE verification.
func (s *Service) ExchangeCode(ctx context.Context, code string, optsList ...ExchangeCodeOpts) (*GoogleUserInfo, error) {
	var authOpts []oauth2.AuthCodeOption
	if len(optsList) > 0 {
		o := optsList[0]
		if o.RedirectURI != "" {
			if !AllowedRedirectURIs[o.RedirectURI] && o.RedirectURI != s.oauthConfig.RedirectURL {
				return nil, fmt.Errorf("redirect_uri not allowed: %s", o.RedirectURI)
			}
			authOpts = append(authOpts, oauth2.SetAuthURLParam("redirect_uri", o.RedirectURI))
		}
		if o.CodeVerifier != "" {
			authOpts = append(authOpts, oauth2.SetAuthURLParam("code_verifier", o.CodeVerifier))
		}
	}
	token, err := s.oauthConfig.Exchange(ctx, code, authOpts...)
	if err != nil {
		return nil, fmt.Errorf("exchange code: %w", err)
	}

	client := s.oauthConfig.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, fmt.Errorf("get user info: %w", err)
	}
	defer resp.Body.Close()

	// Limit read to 1 MB — Google's userinfo response is small JSON.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read user info: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("user info request failed: %s", body)
	}

	var info GoogleUserInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("unmarshal user info: %w", err)
	}

	return &info, nil
}

func (s *Service) GenerateAccessToken(userID uuid.UUID) (string, error) {
	claims := jwt.MapClaims{
		"sub": userID.String(),
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
		"iss": jwtIssuer,
		"aud": jwtAudience,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

// RefreshTokenResult contains the signed token string along with metadata
// needed for server-side tracking (JTI, family, expiry).
type RefreshTokenResult struct {
	Token     string
	JTI       string
	Family    uuid.UUID
	ExpiresAt time.Time
}

// GenerateRefreshToken creates a new refresh token with a unique JTI.
// If family is uuid.Nil, a new token family is started (initial login).
// Otherwise, the token belongs to the given family (rotation).
func (s *Service) GenerateRefreshToken(userID uuid.UUID, family uuid.UUID) (*RefreshTokenResult, error) {
	jti := uuid.New().String()
	if family == uuid.Nil {
		family = uuid.New()
	}
	expiresAt := time.Now().Add(30 * 24 * time.Hour)

	claims := jwt.MapClaims{
		"sub":    userID.String(),
		"exp":    expiresAt.Unix(),
		"iat":    time.Now().Unix(),
		"iss":    jwtIssuer,
		"aud":    jwtAudience,
		"type":   "refresh",
		"jti":    jti,
		"family": family.String(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return nil, err
	}
	return &RefreshTokenResult{
		Token:     signed,
		JTI:       jti,
		Family:    family,
		ExpiresAt: expiresAt,
	}, nil
}

func (s *Service) ValidateToken(tokenString string) (uuid.UUID, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.jwtSecret, nil
	}, jwt.WithIssuer(jwtIssuer), jwt.WithAudience(jwtAudience))
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return uuid.Nil, fmt.Errorf("invalid token")
	}

	// Reject refresh tokens — they must not be used as access tokens.
	if tokenType, _ := claims["type"].(string); tokenType == "refresh" {
		return uuid.Nil, fmt.Errorf("refresh token cannot be used as access token")
	}

	sub, ok := claims["sub"].(string)
	if !ok {
		return uuid.Nil, fmt.Errorf("missing sub claim")
	}

	return uuid.Parse(sub)
}

// RefreshTokenClaims contains the validated claims from a refresh token.
type RefreshTokenClaims struct {
	UserID uuid.UUID
	JTI    string
	Family uuid.UUID
}

func (s *Service) ValidateRefreshToken(tokenString string) (*RefreshTokenClaims, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.jwtSecret, nil
	}, jwt.WithIssuer(jwtIssuer), jwt.WithAudience(jwtAudience))
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	tokenType, _ := claims["type"].(string)
	if tokenType != "refresh" {
		return nil, fmt.Errorf("invalid token type: expected refresh token")
	}

	sub, ok := claims["sub"].(string)
	if !ok {
		return nil, fmt.Errorf("missing sub claim")
	}

	userID, err := uuid.Parse(sub)
	if err != nil {
		return nil, fmt.Errorf("invalid sub claim: %w", err)
	}

	result := &RefreshTokenClaims{UserID: userID}

	// Extract JTI and family if present (tokens issued before rotation
	// won't have these — they're still valid but won't be tracked).
	if jti, _ := claims["jti"].(string); jti != "" {
		result.JTI = jti
	}
	if familyStr, _ := claims["family"].(string); familyStr != "" {
		if f, err := uuid.Parse(familyStr); err == nil {
			result.Family = f
		}
	}

	return result, nil
}

func ContextWithUserID(ctx context.Context, userID uuid.UUID) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(userIDKey).(uuid.UUID)
	return id, ok
}
