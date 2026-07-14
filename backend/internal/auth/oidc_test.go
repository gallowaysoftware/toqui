package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// mockOIDC stands up a minimal but spec-compliant OIDC IdP: discovery, JWKS,
// and a token endpoint that returns a signed ID token with configurable
// claims. Enough for go-oidc to run real discovery + signature verification.
type mockOIDC struct {
	server   *httptest.Server
	key      *rsa.PrivateKey
	clientID string

	// claims for the next issued id_token (mutable per test).
	email         string
	name          string
	sub           string
	emailVerified *bool
}

func newMockOIDC(t *testing.T) *mockOIDC {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("gen key: %v", err)
	}
	verifiedByDefault := true
	m := &mockOIDC{key: key, clientID: "toqui", email: "traveler@example.com", name: "Traveler", sub: "user-123", emailVerified: &verifiedByDefault}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		iss := m.server.URL
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                                iss,
			"authorization_endpoint":                iss + "/auth",
			"token_endpoint":                        iss + "/token",
			"jwks_uri":                              iss + "/keys",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("/keys", func(w http.ResponseWriter, _ *http.Request) {
		pub := m.key.Public().(*rsa.PublicKey)
		eb := make([]byte, 4)
		binary.BigEndian.PutUint32(eb, uint32(pub.E))
		// trim leading zero bytes of the exponent
		i := 0
		for i < len(eb)-1 && eb[i] == 0 {
			i++
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]any{{
				"kty": "RSA",
				"kid": "test-key",
				"use": "sig",
				"alg": "RS256",
				"n":   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
				"e":   base64.RawURLEncoding.EncodeToString(eb[i:]),
			}},
		})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		claims := jwt.MapClaims{
			"iss": m.server.URL,
			"aud": m.clientID,
			"sub": m.sub,
			"exp": time.Now().Add(time.Hour).Unix(),
			"iat": time.Now().Unix(),
		}
		if m.email != "" {
			claims["email"] = m.email
		}
		if m.name != "" {
			claims["name"] = m.name
		}
		if m.emailVerified != nil {
			claims["email_verified"] = *m.emailVerified
		}
		tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		tok.Header["kid"] = "test-key"
		signed, err := tok.SignedString(m.key)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "at",
			"token_type":   "Bearer",
			"id_token":     signed,
			"expires_in":   3600,
		})
	})

	m.server = httptest.NewServer(mux)
	t.Cleanup(m.server.Close)
	return m
}

func (m *mockOIDC) provider() *OIDCProvider {
	return NewOIDCProvider(m.server.URL, m.clientID, "secret", "", "Test IdP", false)
}

func (m *mockOIDC) providerAllowUnverified() *OIDCProvider {
	return NewOIDCProvider(m.server.URL, m.clientID, "secret", "", "Test IdP", true)
}

func TestOIDC_ExchangeAndVerify_Success(t *testing.T) {
	m := newMockOIDC(t)
	info, err := m.provider().ExchangeAndVerify(context.Background(), "any-code", "", "")
	if err != nil {
		t.Fatalf("ExchangeAndVerify: %v", err)
	}
	if info.Email != "traveler@example.com" || info.Name != "Traveler" || info.Subject != "user-123" {
		t.Errorf("claims mismatch: %+v", info)
	}
}

func TestOIDC_ExchangeAndVerify_EmailVerifiedTrue(t *testing.T) {
	m := newMockOIDC(t)
	verified := true
	m.emailVerified = &verified
	if _, err := m.provider().ExchangeAndVerify(context.Background(), "c", "", ""); err != nil {
		t.Errorf("email_verified=true should pass: %v", err)
	}
}

func TestOIDC_ExchangeAndVerify_EmailVerifiedFalseRejected(t *testing.T) {
	m := newMockOIDC(t)
	verified := false
	m.emailVerified = &verified
	if _, err := m.provider().ExchangeAndVerify(context.Background(), "c", "", ""); err == nil {
		t.Error("email_verified=false must be rejected")
	}
}

func TestOIDC_ExchangeAndVerify_NoEmailRejected(t *testing.T) {
	m := newMockOIDC(t)
	m.email = ""
	if _, err := m.provider().ExchangeAndVerify(context.Background(), "c", "", ""); err == nil {
		t.Error("missing email claim must be rejected")
	}
}

func TestOIDC_ExchangeAndVerify_WrongAudienceRejected(t *testing.T) {
	m := newMockOIDC(t)
	// Provider expects a different client id than the token's aud.
	p := NewOIDCProvider(m.server.URL, "someone-else", "secret", "", "Test IdP", false)
	if _, err := p.ExchangeAndVerify(context.Background(), "c", "", ""); err == nil {
		t.Error("id_token with a mismatched audience must fail verification")
	}
}

func TestOIDC_RedirectURINotAllowed(t *testing.T) {
	m := newMockOIDC(t)
	// A redirect the allowlist doesn't contain and that isn't the configured
	// one must be rejected before any network call.
	if _, err := m.provider().ExchangeAndVerify(context.Background(), "c", "https://evil.example.com/cb", ""); err == nil {
		t.Error("disallowed redirect_uri must be rejected")
	}
}

func TestOIDC_ExchangeAndVerify_EmailVerifiedOmittedRejectedByDefault(t *testing.T) {
	m := newMockOIDC(t)
	m.emailVerified = nil // IdP omits the claim entirely
	if _, err := m.provider().ExchangeAndVerify(context.Background(), "c", "", ""); err == nil {
		t.Error("an omitted email_verified claim must be rejected by default (takeover guard)")
	}
}

func TestOIDC_ExchangeAndVerify_EmailVerifiedOmittedAllowedWithOptOut(t *testing.T) {
	m := newMockOIDC(t)
	m.emailVerified = nil
	if _, err := m.providerAllowUnverified().ExchangeAndVerify(context.Background(), "c", "", ""); err != nil {
		t.Errorf("OIDC_ALLOW_UNVERIFIED_EMAIL should accept an omitted claim: %v", err)
	}
}

func TestOIDC_ExchangeAndVerify_EmailVerifiedFalseRejectedEvenWithOptOut(t *testing.T) {
	m := newMockOIDC(t)
	verified := false
	m.emailVerified = &verified
	// The opt-out relaxes an *absent* claim, not an explicit denial.
	if _, err := m.providerAllowUnverified().ExchangeAndVerify(context.Background(), "c", "", ""); err == nil {
		t.Error("an explicit email_verified:false must be rejected even with the opt-out")
	}
}

func TestOIDC_ExchangeAndVerify_AllowedRedirectURI(t *testing.T) {
	m := newMockOIDC(t)
	const redirect = "https://app.example.com/auth/callback"
	AllowedRedirectURIs[redirect] = true
	defer delete(AllowedRedirectURIs, redirect)

	if _, err := m.provider().ExchangeAndVerify(context.Background(), "c", redirect, ""); err != nil {
		t.Errorf("an allowlisted redirect_uri should be accepted: %v", err)
	}
}

func TestOIDC_LazyDiscoveryRetries(t *testing.T) {
	// Discovery is not cached on failure: a provider whose IdP is initially
	// down fails ensure(), then succeeds once the IdP serves discovery.
	var ready atomic.Bool
	var srv *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		if !ready.Load() {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                                srv.URL,
			"authorization_endpoint":                srv.URL + "/auth",
			"token_endpoint":                        srv.URL + "/token",
			"jwks_uri":                              srv.URL + "/keys",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	p := NewOIDCProvider(srv.URL, "toqui", "secret", "", "", false)
	if p.Name() != "SSO" {
		t.Errorf("default name = %q, want SSO", p.Name())
	}
	if err := p.ensure(context.Background()); err == nil {
		t.Fatal("expected discovery to fail while the IdP is down")
	}
	ready.Store(true)
	if err := p.ensure(context.Background()); err != nil {
		t.Errorf("discovery should succeed once the IdP is up (not cached on failure): %v", err)
	}
}
