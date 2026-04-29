package apple

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// fakeAppleEnv spins up a httptest.Server that mimics Apple's /auth/token
// and /auth/keys endpoints, plus an RSA keypair we use to sign fake ID
// tokens. Everything is deterministic so tests are repeatable.
type fakeAppleEnv struct {
	server *httptest.Server
	rsaKey *rsa.PrivateKey
	kid    string

	tokenHits atomic.Int32
	jwksHits  atomic.Int32

	// Override what /auth/token returns (signed with rsaKey by default).
	tokenIDToken func() string

	// Override the JWKS payload (defaults to one entry, kid -> rsaKey).
	jwksOverride []byte
}

func newFakeAppleEnv(t *testing.T) *fakeAppleEnv {
	t.Helper()
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	env := &fakeAppleEnv{
		rsaKey: rsaKey,
		kid:    "test-key-1",
	}
	env.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/token":
			env.tokenHits.Add(1)
			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			body, _ := io.ReadAll(r.Body)
			form, _ := url.ParseQuery(string(body))
			if form.Get("grant_type") != "authorization_code" {
				http.Error(w, "bad grant type", http.StatusBadRequest)
				return
			}
			if form.Get("client_id") == "" || form.Get("client_secret") == "" || form.Get("code") == "" {
				http.Error(w, "missing field", http.StatusBadRequest)
				return
			}
			id := ""
			if env.tokenIDToken != nil {
				id = env.tokenIDToken()
			} else {
				id = env.signIDToken(t, defaultClaims(form.Get("client_id")))
			}
			resp := map[string]any{
				"access_token":  "atok",
				"token_type":    "Bearer",
				"expires_in":    3600,
				"refresh_token": "rtok",
				"id_token":      id,
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		case "/auth/keys":
			env.jwksHits.Add(1)
			if env.jwksOverride != nil {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(env.jwksOverride)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(env.jwksJSON())
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(env.server.Close)
	return env
}

func (e *fakeAppleEnv) jwksJSON() []byte {
	pub := e.rsaKey.PublicKey
	n := base64.RawURLEncoding.EncodeToString(pub.N.Bytes())
	eBytes := big.NewInt(int64(pub.E)).Bytes()
	eEnc := base64.RawURLEncoding.EncodeToString(eBytes)
	out := map[string]any{
		"keys": []any{
			map[string]any{
				"kid": e.kid,
				"kty": "RSA",
				"use": "sig",
				"alg": "RS256",
				"n":   n,
				"e":   eEnc,
			},
		},
	}
	b, _ := json.Marshal(out)
	return b
}

// signIDToken issues a JWT with the given claims, signed by env.rsaKey.
func (e *fakeAppleEnv) signIDToken(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = e.kid
	s, err := tok.SignedString(e.rsaKey)
	if err != nil {
		t.Fatalf("sign id token: %v", err)
	}
	return s
}

func defaultClaims(aud string) jwt.MapClaims {
	now := time.Now()
	return jwt.MapClaims{
		"iss":            "https://appleid.apple.com",
		"aud":            aud,
		"sub":            "001234.abc.5678",
		"iat":            now.Unix(),
		"exp":            now.Add(time.Hour).Unix(),
		"email":          "user@privaterelay.appleid.com",
		"email_verified": true,
	}
}

// generateP8 produces a PEM-encoded ECDSA P-256 key, mimicking Apple's .p8 export.
func generateP8(t *testing.T) []byte {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate ec key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("marshal pkcs8: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
}

// newTestClient builds an apple.Client wired against the fake env.
func newTestClient(t *testing.T, env *fakeAppleEnv) *Client {
	t.Helper()
	cfg := Config{
		TeamID:     "TEAMID1234",
		ServicesID: "com.toqui.app.signin",
		KeyID:      "KEYID12345",
		PrivateKey: generateP8(t),
		TokenURL:   env.server.URL + "/auth/token",
		JWKSURL:    env.server.URL + "/auth/keys",
		Issuer:     "https://appleid.apple.com",
	}
	c, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

// ---- Tests ------------------------------------------------------------

func TestNewClient_RejectsMissingFields(t *testing.T) {
	_, err := NewClient(Config{ServicesID: "x", KeyID: "y", PrivateKey: generateP8(t)})
	if err == nil {
		t.Fatal("expected error for missing TeamID")
	}
	_, err = NewClient(Config{TeamID: "x", KeyID: "y", PrivateKey: generateP8(t)})
	if err == nil {
		t.Fatal("expected error for missing ServicesID")
	}
	_, err = NewClient(Config{TeamID: "x", ServicesID: "y", PrivateKey: generateP8(t)})
	if err == nil {
		t.Fatal("expected error for missing KeyID")
	}
	_, err = NewClient(Config{TeamID: "x", ServicesID: "y", KeyID: "z"})
	if err == nil {
		t.Fatal("expected error for missing PrivateKey")
	}
}

func TestNewClient_RejectsInvalidKey(t *testing.T) {
	_, err := NewClient(Config{
		TeamID:     "x",
		ServicesID: "y",
		KeyID:      "z",
		PrivateKey: []byte("not a pem"),
	})
	if err == nil {
		t.Fatal("expected error for bogus key")
	}
}

func TestIsConfigured(t *testing.T) {
	pem := generateP8(t)
	if !IsConfigured("a", "b", "c", pem) {
		t.Error("expected configured")
	}
	if IsConfigured("", "b", "c", pem) {
		t.Error("blank team should be unconfigured")
	}
	if IsConfigured("a", "", "c", pem) {
		t.Error("blank services should be unconfigured")
	}
	if IsConfigured("a", "b", "", pem) {
		t.Error("blank kid should be unconfigured")
	}
	if IsConfigured("a", "b", "c", nil) {
		t.Error("nil key should be unconfigured")
	}
}

func TestGenerateClientSecret_HasCorrectClaims(t *testing.T) {
	env := newFakeAppleEnv(t)
	c := newTestClient(t, env)
	secret, err := c.GenerateClientSecret()
	if err != nil {
		t.Fatalf("generate client secret: %v", err)
	}
	// Parse without verification to inspect the claims.
	parser := jwt.NewParser()
	tok, _, err := parser.ParseUnverified(secret, jwt.MapClaims{})
	if err != nil {
		t.Fatalf("parse secret: %v", err)
	}
	if alg, _ := tok.Header["alg"].(string); alg != "ES256" {
		t.Errorf("alg = %q, want ES256", alg)
	}
	if kid, _ := tok.Header["kid"].(string); kid != "KEYID12345" {
		t.Errorf("kid = %q, want KEYID12345", kid)
	}
	mc, _ := tok.Claims.(jwt.MapClaims)
	if iss, _ := mc["iss"].(string); iss != "TEAMID1234" {
		t.Errorf("iss = %q, want TEAMID1234", iss)
	}
	if sub, _ := mc["sub"].(string); sub != "com.toqui.app.signin" {
		t.Errorf("sub = %q, want com.toqui.app.signin", sub)
	}
	if aud, _ := mc["aud"].(string); aud != "https://appleid.apple.com" {
		t.Errorf("aud = %q, want https://appleid.apple.com", aud)
	}
	iat, ok1 := mc["iat"].(float64)
	exp, ok2 := mc["exp"].(float64)
	if !ok1 || !ok2 {
		t.Fatalf("missing iat/exp")
	}
	delta := exp - iat
	if delta < 60 || delta > 6*60 {
		t.Errorf("client secret lifetime = %ds, want ~5m", int(delta))
	}
}

func TestExchangeCode_HappyPath(t *testing.T) {
	env := newFakeAppleEnv(t)
	c := newTestClient(t, env)
	ctx := context.Background()
	resp, err := c.ExchangeCode(ctx, "abc123", "https://app.example/cb")
	if err != nil {
		t.Fatalf("exchange: %v", err)
	}
	if resp.IDToken == "" {
		t.Fatal("missing id token")
	}
	if env.tokenHits.Load() != 1 {
		t.Errorf("token hits = %d, want 1", env.tokenHits.Load())
	}
}

func TestExchangeCode_RejectsEmptyCode(t *testing.T) {
	env := newFakeAppleEnv(t)
	c := newTestClient(t, env)
	if _, err := c.ExchangeCode(context.Background(), "", ""); err == nil {
		t.Fatal("expected error for empty code")
	}
}

func TestExchangeCode_AppleError(t *testing.T) {
	env := newFakeAppleEnv(t)
	// Make the fake server return a 400 with Apple-ish error body.
	env.server.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"code expired"}`))
	}))
	defer srv.Close()
	cfg := Config{
		TeamID:     "T",
		ServicesID: "S",
		KeyID:      "K",
		PrivateKey: generateP8(t),
		TokenURL:   srv.URL,
		JWKSURL:    srv.URL,
	}
	c, err := NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.ExchangeCode(context.Background(), "code", "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "invalid_grant") {
		t.Errorf("expected invalid_grant in error, got %v", err)
	}
}

func TestVerifyIDToken_HappyPath(t *testing.T) {
	env := newFakeAppleEnv(t)
	c := newTestClient(t, env)

	idTok := env.signIDToken(t, defaultClaims("com.toqui.app.signin"))
	claims, err := c.VerifyIDToken(context.Background(), idTok)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.Subject != "001234.abc.5678" {
		t.Errorf("sub = %q", claims.Subject)
	}
	if claims.Email != "user@privaterelay.appleid.com" {
		t.Errorf("email = %q", claims.Email)
	}
	if !claims.EmailVerified {
		t.Error("expected email_verified=true")
	}
}

func TestVerifyIDToken_BadSignature(t *testing.T) {
	env := newFakeAppleEnv(t)
	c := newTestClient(t, env)

	// Sign with a DIFFERENT key — JWKS won't have it.
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, defaultClaims("com.toqui.app.signin"))
	tok.Header["kid"] = env.kid // claim the right kid; signature will fail
	s, err := tok.SignedString(other)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.VerifyIDToken(context.Background(), s); err == nil {
		t.Fatal("expected signature error")
	}
}

func TestVerifyIDToken_Expired(t *testing.T) {
	env := newFakeAppleEnv(t)
	c := newTestClient(t, env)
	claims := defaultClaims("com.toqui.app.signin")
	// Push exp into the past, well beyond the 5-minute skew window.
	claims["exp"] = time.Now().Add(-1 * time.Hour).Unix()
	idTok := env.signIDToken(t, claims)
	if _, err := c.VerifyIDToken(context.Background(), idTok); err == nil {
		t.Fatal("expected expired error")
	}
}

func TestVerifyIDToken_WrongIssuer(t *testing.T) {
	env := newFakeAppleEnv(t)
	c := newTestClient(t, env)
	claims := defaultClaims("com.toqui.app.signin")
	claims["iss"] = "https://evil.example.com"
	idTok := env.signIDToken(t, claims)
	if _, err := c.VerifyIDToken(context.Background(), idTok); err == nil || !strings.Contains(err.Error(), "issuer") {
		t.Fatalf("expected issuer error, got %v", err)
	}
}

func TestVerifyIDToken_WrongAudience(t *testing.T) {
	env := newFakeAppleEnv(t)
	c := newTestClient(t, env)
	idTok := env.signIDToken(t, defaultClaims("com.someone.else"))
	if _, err := c.VerifyIDToken(context.Background(), idTok); err == nil || !strings.Contains(err.Error(), "audience") {
		t.Fatalf("expected audience error, got %v", err)
	}
}

func TestVerifyIDToken_FutureIAT(t *testing.T) {
	env := newFakeAppleEnv(t)
	c := newTestClient(t, env)
	claims := defaultClaims("com.toqui.app.signin")
	// Push iat 30m into the future — well outside the 5m skew window.
	claims["iat"] = time.Now().Add(30 * time.Minute).Unix()
	idTok := env.signIDToken(t, claims)
	if _, err := c.VerifyIDToken(context.Background(), idTok); err == nil || !strings.Contains(err.Error(), "future") {
		t.Fatalf("expected future iat error, got %v", err)
	}
}

func TestVerifyIDToken_ToleratesSmallSkew(t *testing.T) {
	env := newFakeAppleEnv(t)
	c := newTestClient(t, env)
	claims := defaultClaims("com.toqui.app.signin")
	// Push iat 1 minute into the future — inside the 5m skew window.
	claims["iat"] = time.Now().Add(1 * time.Minute).Unix()
	idTok := env.signIDToken(t, claims)
	if _, err := c.VerifyIDToken(context.Background(), idTok); err != nil {
		t.Fatalf("expected small skew to be accepted, got %v", err)
	}
}

func TestVerifyIDToken_RejectsHS256(t *testing.T) {
	env := newFakeAppleEnv(t)
	c := newTestClient(t, env)
	// Sign with HS256 — must be rejected (algorithm confusion attack).
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, defaultClaims("com.toqui.app.signin"))
	tok.Header["kid"] = env.kid
	s, err := tok.SignedString([]byte("symkey"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.VerifyIDToken(context.Background(), s); err == nil {
		t.Fatal("expected method rejection")
	}
}

func TestVerifyIDToken_MissingKidHeader(t *testing.T) {
	env := newFakeAppleEnv(t)
	c := newTestClient(t, env)
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, defaultClaims("com.toqui.app.signin"))
	// no kid header
	s, err := tok.SignedString(env.rsaKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.VerifyIDToken(context.Background(), s); err == nil {
		t.Fatal("expected missing kid error")
	}
}

func TestJWKSCache_DoesNotRefetchWithinTTL(t *testing.T) {
	env := newFakeAppleEnv(t)
	c := newTestClient(t, env)
	tok1 := env.signIDToken(t, defaultClaims("com.toqui.app.signin"))
	tok2 := env.signIDToken(t, defaultClaims("com.toqui.app.signin"))
	if _, err := c.VerifyIDToken(context.Background(), tok1); err != nil {
		t.Fatalf("first verify: %v", err)
	}
	if _, err := c.VerifyIDToken(context.Background(), tok2); err != nil {
		t.Fatalf("second verify: %v", err)
	}
	if got := env.jwksHits.Load(); got != 1 {
		t.Errorf("jwks hits = %d, want 1 (cache should serve second call)", got)
	}
}

func TestJWKSCache_RefetchesWhenStale(t *testing.T) {
	env := newFakeAppleEnv(t)
	c := newTestClient(t, env)
	idTok := env.signIDToken(t, defaultClaims("com.toqui.app.signin"))
	if _, err := c.VerifyIDToken(context.Background(), idTok); err != nil {
		t.Fatalf("first verify: %v", err)
	}
	// Force the cache to look stale by rewinding jwksAt by >1h.
	c.mu.Lock()
	c.jwksAt = time.Now().Add(-2 * time.Hour)
	c.mu.Unlock()
	idTok2 := env.signIDToken(t, defaultClaims("com.toqui.app.signin"))
	if _, err := c.VerifyIDToken(context.Background(), idTok2); err != nil {
		t.Fatalf("second verify: %v", err)
	}
	if got := env.jwksHits.Load(); got != 2 {
		t.Errorf("jwks hits = %d, want 2 (cache should have been refetched)", got)
	}
}

func TestJWKSCache_RefetchesOnUnknownKid(t *testing.T) {
	env := newFakeAppleEnv(t)
	c := newTestClient(t, env)
	idTok := env.signIDToken(t, defaultClaims("com.toqui.app.signin"))
	if _, err := c.VerifyIDToken(context.Background(), idTok); err != nil {
		t.Fatalf("first verify: %v", err)
	}
	// Rotate the key on the fake server.
	newKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	env.rsaKey = newKey
	env.kid = "test-key-2"
	idTok2 := env.signIDToken(t, defaultClaims("com.toqui.app.signin"))
	if _, err := c.VerifyIDToken(context.Background(), idTok2); err != nil {
		t.Fatalf("second verify: %v", err)
	}
	if got := env.jwksHits.Load(); got != 2 {
		t.Errorf("jwks hits = %d, want 2 (cache miss on new kid should refetch)", got)
	}
}

func TestExchangeCode_FullFlow_VerifiesIDToken(t *testing.T) {
	env := newFakeAppleEnv(t)
	c := newTestClient(t, env)
	ctx := context.Background()
	resp, err := c.ExchangeCode(ctx, "code123", "https://app.example/cb")
	if err != nil {
		t.Fatalf("exchange: %v", err)
	}
	claims, err := c.VerifyIDToken(ctx, resp.IDToken)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.Subject == "" {
		t.Fatal("expected sub")
	}
}

// Ensure the JWKS parse path skips non-RSA keys without exploding.
func TestJWKS_IgnoresNonRSA(t *testing.T) {
	env := newFakeAppleEnv(t)
	env.jwksOverride = []byte(fmt.Sprintf(`{"keys":[
		{"kid":"ec1","kty":"EC","crv":"P-256","x":"...","y":"..."},
		%s
	]}`, singleKeyJSON(t, env)))
	c := newTestClient(t, env)
	idTok := env.signIDToken(t, defaultClaims("com.toqui.app.signin"))
	if _, err := c.VerifyIDToken(context.Background(), idTok); err != nil {
		t.Fatalf("verify with mixed JWKS: %v", err)
	}
}

// generateSEC1 produces a PEM-encoded ECDSA P-256 key in legacy SEC1 form.
// We accept this format too — it's what `openssl ecparam` spits out by default.
func generateSEC1(t *testing.T) []byte {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate ec key: %v", err)
	}
	der, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		t.Fatalf("marshal sec1: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})
}

func TestGenerateClientSecret_AcceptsSEC1Key(t *testing.T) {
	env := newFakeAppleEnv(t)
	cfg := Config{
		TeamID:     "T",
		ServicesID: "S",
		KeyID:      "K",
		PrivateKey: generateSEC1(t),
		TokenURL:   env.server.URL + "/auth/token",
		JWKSURL:    env.server.URL + "/auth/keys",
	}
	c, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if _, err := c.GenerateClientSecret(); err != nil {
		t.Fatalf("client secret: %v", err)
	}
}

func TestParseECPrivateKey_RejectsRSAKey(t *testing.T) {
	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	der, _ := x509.MarshalPKCS8PrivateKey(rsaKey)
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	if _, err := parseECPrivateKey(pemBytes); err == nil {
		t.Fatal("expected error for RSA key")
	}
}

func TestVerifyIDToken_EmptyToken(t *testing.T) {
	env := newFakeAppleEnv(t)
	c := newTestClient(t, env)
	if _, err := c.VerifyIDToken(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty token")
	}
}

func TestVerifyIDToken_AudAsArray(t *testing.T) {
	env := newFakeAppleEnv(t)
	c := newTestClient(t, env)
	claims := defaultClaims("ignored-string")
	claims["aud"] = []string{"com.toqui.app.signin"}
	idTok := env.signIDToken(t, claims)
	if _, err := c.VerifyIDToken(context.Background(), idTok); err != nil {
		t.Fatalf("array aud: %v", err)
	}
}

func TestVerifyIDToken_MissingSub(t *testing.T) {
	env := newFakeAppleEnv(t)
	c := newTestClient(t, env)
	claims := defaultClaims("com.toqui.app.signin")
	delete(claims, "sub")
	idTok := env.signIDToken(t, claims)
	if _, err := c.VerifyIDToken(context.Background(), idTok); err == nil {
		t.Fatal("expected missing sub error")
	}
}

func TestExchangeCode_MissingIDToken(t *testing.T) {
	env := newFakeAppleEnv(t)
	// Override the token endpoint to drop id_token.
	env.tokenIDToken = func() string { return "" }
	// Replace the fake server's handler? Simpler: spin up a new server.
	env.server.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"a","token_type":"Bearer","expires_in":3600}`))
	}))
	defer srv.Close()
	cfg := Config{
		TeamID:     "T",
		ServicesID: "S",
		KeyID:      "K",
		PrivateKey: generateP8(t),
		TokenURL:   srv.URL,
		JWKSURL:    srv.URL,
	}
	c, _ := NewClient(cfg)
	if _, err := c.ExchangeCode(context.Background(), "code", ""); err == nil {
		t.Fatal("expected missing id_token error")
	}
}

func TestRefreshJWKS_FailsOnHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	cfg := Config{
		TeamID:     "T",
		ServicesID: "S",
		KeyID:      "K",
		PrivateKey: generateP8(t),
		TokenURL:   srv.URL,
		JWKSURL:    srv.URL,
	}
	c, _ := NewClient(cfg)
	// Build a token signed with some key; we just want lookupKey to attempt
	// a JWKS fetch against the failing endpoint.
	otherKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, defaultClaims("S"))
	tok.Header["kid"] = "anykid"
	s, _ := tok.SignedString(otherKey)
	if _, err := c.VerifyIDToken(context.Background(), s); err == nil {
		t.Fatal("expected error when JWKS fetch fails")
	}
}

func TestRefreshJWKS_FailsOnEmptyKeys(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"keys":[]}`))
	}))
	defer srv.Close()
	cfg := Config{
		TeamID:     "T",
		ServicesID: "S",
		KeyID:      "K",
		PrivateKey: generateP8(t),
		TokenURL:   srv.URL,
		JWKSURL:    srv.URL,
	}
	c, _ := NewClient(cfg)
	otherKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, defaultClaims("S"))
	tok.Header["kid"] = "anykid"
	s, _ := tok.SignedString(otherKey)
	if _, err := c.VerifyIDToken(context.Background(), s); err == nil {
		t.Fatal("expected error when JWKS is empty")
	}
}

func singleKeyJSON(t *testing.T, env *fakeAppleEnv) string {
	t.Helper()
	pub := env.rsaKey.PublicKey
	n := base64.RawURLEncoding.EncodeToString(pub.N.Bytes())
	eBytes := big.NewInt(int64(pub.E)).Bytes()
	eEnc := base64.RawURLEncoding.EncodeToString(eBytes)
	return fmt.Sprintf(`{"kid":%q,"kty":"RSA","use":"sig","alg":"RS256","n":%q,"e":%q}`, env.kid, n, eEnc)
}
