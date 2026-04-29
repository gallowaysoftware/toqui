// Package apple implements Sign in with Apple's server-side flow:
//
//  1. ExchangeCode — POST authorization_code to https://appleid.apple.com/auth/token
//     with a dynamically-signed client_secret JWT (ECDSA ES256, .p8 key).
//  2. VerifyIDToken — fetch the public JWKS from https://appleid.apple.com/auth/keys,
//     cache it for an hour, and verify the RS256 signature + iss/aud/exp/iat claims.
//
// The package has no global state — credentials and HTTP client are injected
// at construction. All operations are safe for concurrent use.
package apple

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Apple endpoints. Exposed as variables so tests can point them at a httptest.Server.
var (
	defaultTokenURL = "https://appleid.apple.com/auth/token"
	defaultJWKSURL  = "https://appleid.apple.com/auth/keys"
	defaultIssuer   = "https://appleid.apple.com"
)

// jwksCacheTTL controls how often the JWKS endpoint is refetched. Apple
// rotates its keys infrequently; an hour is the sweet spot used by Apple's
// own sample code.
const jwksCacheTTL = time.Hour

// allowedIATSkew is the maximum window in which a token's `iat` may sit in
// the future before being rejected (clock-skew allowance). Five minutes
// matches the spec's recommended drift tolerance.
const allowedIATSkew = 5 * time.Minute

// allowedExpSkew gives the same five-minute grace period to `exp` so we
// don't reject tokens issued by a slightly-fast Apple server. The
// underlying jwt library already validates exp; we just relax it.
const allowedExpSkew = 5 * time.Minute

// TokenResponse is Apple's response from POST /auth/token.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token"`
}

// IDTokenClaims captures the subset of Apple ID token claims we care about.
// `email` is only present on the FIRST login per Apple's spec — we MUST persist
// it then. Subsequent logins return an empty email.
type IDTokenClaims struct {
	Subject        string `json:"sub"`
	Email          string `json:"email,omitempty"`
	EmailVerified  bool   `json:"email_verified,omitempty"`
	IsPrivateEmail bool   `json:"is_private_email,omitempty"`
	Issuer         string `json:"iss"`
	Audience       string `json:"aud"`
	IssuedAt       int64  `json:"iat"`
	ExpiresAt      int64  `json:"exp"`
}

// Config holds the static credentials needed to talk to Apple. Every field
// is required; a zero Config is invalid.
type Config struct {
	TeamID     string // Apple Developer team ID (10-char alphanumeric).
	ServicesID string // Services ID (NOT the iOS bundle ID). Used as the OAuth client_id.
	KeyID      string // Apple Sign-In key ID (10-char alphanumeric).
	PrivateKey []byte // PEM-encoded ECDSA P-256 (.p8) private key.
	HTTPClient *http.Client
	TokenURL   string           // optional override for tests
	JWKSURL    string           // optional override for tests
	Issuer     string           // optional override for tests
	Now        func() time.Time // optional override for tests
}

// Client talks to Apple's identity service.
type Client struct {
	cfg      Config
	httpC    *http.Client
	tokenURL string
	jwksURL  string
	issuer   string
	now      func() time.Time

	mu     sync.Mutex
	jwks   map[string]*rsa.PublicKey
	jwksAt time.Time
}

// NewClient builds a Client from a Config. It returns an error if the
// configuration is incomplete — in particular, if any required field is
// empty. Callers that want a "soft" Apple integration (gated on env-var
// presence) should check IsConfigured beforehand.
func NewClient(cfg Config) (*Client, error) {
	if cfg.TeamID == "" {
		return nil, errors.New("apple: team_id required")
	}
	if cfg.ServicesID == "" {
		return nil, errors.New("apple: services_id required")
	}
	if cfg.KeyID == "" {
		return nil, errors.New("apple: key_id required")
	}
	if len(cfg.PrivateKey) == 0 {
		return nil, errors.New("apple: private_key required")
	}
	// Validate the key actually parses up front so we fail at startup,
	// not on the first login attempt.
	if _, err := parseECPrivateKey(cfg.PrivateKey); err != nil {
		return nil, fmt.Errorf("apple: parse private key: %w", err)
	}
	httpC := cfg.HTTPClient
	if httpC == nil {
		httpC = &http.Client{Timeout: 10 * time.Second}
	}
	tokenURL := cfg.TokenURL
	if tokenURL == "" {
		tokenURL = defaultTokenURL
	}
	jwksURL := cfg.JWKSURL
	if jwksURL == "" {
		jwksURL = defaultJWKSURL
	}
	issuer := cfg.Issuer
	if issuer == "" {
		issuer = defaultIssuer
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &Client{
		cfg:      cfg,
		httpC:    httpC,
		tokenURL: tokenURL,
		jwksURL:  jwksURL,
		issuer:   issuer,
		now:      now,
	}, nil
}

// IsConfigured reports whether the four required Apple credentials are
// present in the given values. Callers (e.g. main.go) use this to decide
// whether to wire the Apple client at all — when any of the four are empty,
// the AppleLogin RPC stays in "unimplemented" mode.
func IsConfigured(teamID, servicesID, keyID string, privateKey []byte) bool {
	return teamID != "" && servicesID != "" && keyID != "" && len(privateKey) > 0
}

// ExchangeCode swaps an authorization code for a token bundle. The redirect
// URI is the one the frontend sent during the initial authorization request
// — Apple verifies it matches.
func (c *Client) ExchangeCode(ctx context.Context, code, redirectURI string) (*TokenResponse, error) {
	if code == "" {
		return nil, errors.New("apple: authorization code required")
	}
	clientSecret, err := c.GenerateClientSecret()
	if err != nil {
		return nil, fmt.Errorf("apple: generate client secret: %w", err)
	}
	form := url.Values{
		"client_id":     {c.cfg.ServicesID},
		"client_secret": {clientSecret},
		"code":          {code},
		"grant_type":    {"authorization_code"},
	}
	if redirectURI != "" {
		form.Set("redirect_uri", redirectURI)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("apple: build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpC.Do(req)
	if err != nil {
		return nil, fmt.Errorf("apple: token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("apple: read token response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		// Apple returns {"error":"invalid_grant"} etc. — surface that.
		var errBody struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		_ = json.Unmarshal(body, &errBody)
		if errBody.Error != "" {
			return nil, fmt.Errorf("apple: token exchange failed: %s (%s)", errBody.Error, errBody.ErrorDescription)
		}
		return nil, fmt.Errorf("apple: token exchange failed: status %d", resp.StatusCode)
	}

	var tr TokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return nil, fmt.Errorf("apple: decode token response: %w", err)
	}
	if tr.IDToken == "" {
		return nil, errors.New("apple: token response missing id_token")
	}
	return &tr, nil
}

// VerifyIDToken validates Apple's ID token: signature against the JWKS,
// issuer, audience, expiry, and issued-at (clock skew tolerated).
func (c *Client) VerifyIDToken(ctx context.Context, idToken string) (*IDTokenClaims, error) {
	if idToken == "" {
		return nil, errors.New("apple: id token required")
	}

	parsed, err := jwt.Parse(idToken, func(t *jwt.Token) (interface{}, error) {
		// Apple signs with RS256.
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("apple: unexpected signing method: %v", t.Header["alg"])
		}
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("apple: token missing kid header")
		}
		key, err := c.lookupKey(ctx, kid)
		if err != nil {
			return nil, err
		}
		return key, nil
	},
		jwt.WithValidMethods([]string{"RS256"}),
		// Allow small clock skew on exp. We re-check exp explicitly below.
		jwt.WithLeeway(allowedExpSkew),
		// Pin time to the injected clock so tests are deterministic.
		jwt.WithTimeFunc(c.now),
	)

	if err != nil {
		return nil, fmt.Errorf("apple: verify id token: %w", err)
	}
	if !parsed.Valid {
		return nil, errors.New("apple: id token invalid")
	}

	mc, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("apple: claims wrong shape")
	}

	claims, err := c.normalizeClaims(mc)
	if err != nil {
		return nil, err
	}

	if claims.Issuer != c.issuer {
		return nil, fmt.Errorf("apple: wrong issuer: %q (want %q)", claims.Issuer, c.issuer)
	}
	if claims.Audience != c.cfg.ServicesID {
		return nil, fmt.Errorf("apple: wrong audience: %q (want %q)", claims.Audience, c.cfg.ServicesID)
	}
	now := c.now()
	if claims.ExpiresAt > 0 && now.After(time.Unix(claims.ExpiresAt, 0).Add(allowedExpSkew)) {
		return nil, errors.New("apple: id token expired")
	}
	if claims.IssuedAt > 0 {
		iat := time.Unix(claims.IssuedAt, 0)
		if iat.After(now.Add(allowedIATSkew)) {
			return nil, errors.New("apple: id token iat is in the future")
		}
	}
	if claims.Subject == "" {
		return nil, errors.New("apple: id token missing sub")
	}
	return claims, nil
}

// normalizeClaims pulls the fields we care about from MapClaims, coping
// with Apple's mixed types (email_verified can be string or bool; aud can
// be string or []string per RFC 7519).
func (c *Client) normalizeClaims(mc jwt.MapClaims) (*IDTokenClaims, error) {
	out := &IDTokenClaims{}
	if v, ok := mc["sub"].(string); ok {
		out.Subject = v
	}
	if v, ok := mc["email"].(string); ok {
		out.Email = v
	}
	if v, ok := mc["iss"].(string); ok {
		out.Issuer = v
	}
	switch a := mc["aud"].(type) {
	case string:
		out.Audience = a
	case []interface{}:
		// Apple uses single-aud, but be defensive.
		if len(a) > 0 {
			if s, ok := a[0].(string); ok {
				out.Audience = s
			}
		}
	}
	switch v := mc["iat"].(type) {
	case float64:
		out.IssuedAt = int64(v)
	case json.Number:
		n, _ := v.Int64()
		out.IssuedAt = n
	}
	switch v := mc["exp"].(type) {
	case float64:
		out.ExpiresAt = int64(v)
	case json.Number:
		n, _ := v.Int64()
		out.ExpiresAt = n
	}
	switch v := mc["email_verified"].(type) {
	case bool:
		out.EmailVerified = v
	case string:
		out.EmailVerified = v == "true"
	}
	switch v := mc["is_private_email"].(type) {
	case bool:
		out.IsPrivateEmail = v
	case string:
		out.IsPrivateEmail = v == "true"
	}
	return out, nil
}

// lookupKey returns the RSA public key for a given kid, refreshing the
// JWKS cache if it's expired or doesn't contain the requested kid.
func (c *Client) lookupKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	c.mu.Lock()
	cached, hit := c.jwks[kid]
	fresh := !c.jwksAt.IsZero() && c.now().Sub(c.jwksAt) < jwksCacheTTL
	c.mu.Unlock()

	if hit && fresh {
		return cached, nil
	}

	// Either cache miss or stale — refetch.
	if err := c.refreshJWKS(ctx); err != nil {
		return nil, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	key, ok := c.jwks[kid]
	if !ok {
		return nil, fmt.Errorf("apple: kid %q not in JWKS", kid)
	}
	return key, nil
}

// refreshJWKS fetches https://appleid.apple.com/auth/keys, parses the JWK
// set, and replaces the in-memory cache.
func (c *Client) refreshJWKS(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.jwksURL, nil)
	if err != nil {
		return fmt.Errorf("apple: build jwks request: %w", err)
	}
	resp, err := c.httpC.Do(req)
	if err != nil {
		return fmt.Errorf("apple: jwks fetch: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("apple: read jwks: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("apple: jwks http %d", resp.StatusCode)
	}
	var raw struct {
		Keys []struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			Use string `json:"use"`
			Alg string `json:"alg"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return fmt.Errorf("apple: parse jwks: %w", err)
	}
	keys := make(map[string]*rsa.PublicKey, len(raw.Keys))
	for _, k := range raw.Keys {
		if k.Kty != "RSA" {
			continue
		}
		nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
		if err != nil {
			continue
		}
		eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
		if err != nil {
			continue
		}
		// Pad e if shorter than 4 bytes.
		var eInt int
		for _, b := range eBytes {
			eInt = eInt<<8 | int(b)
		}
		keys[k.Kid] = &rsa.PublicKey{
			N: new(big.Int).SetBytes(nBytes),
			E: eInt,
		}
	}
	if len(keys) == 0 {
		return errors.New("apple: jwks contained no usable RSA keys")
	}
	c.mu.Lock()
	c.jwks = keys
	c.jwksAt = c.now()
	c.mu.Unlock()
	return nil
}
