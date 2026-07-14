//go:build integration

package integration

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/golang-jwt/jwt/v5"

	toquiv1 "github.com/gallowaysoftware/toqui/backend/gen/toqui/v1"
	"github.com/gallowaysoftware/toqui/backend/internal/auth"
	"github.com/gallowaysoftware/toqui/backend/internal/handlers"
)

// startMockOIDC stands up a minimal OIDC IdP (discovery + JWKS + token) that
// issues an ID token for the given email/name. Enough for go-oidc to run
// real discovery + signature verification.
func startMockOIDC(t *testing.T, email, name string) (issuer, clientID string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("gen key: %v", err)
	}
	clientID = "toqui"
	var srv *httptest.Server

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                                srv.URL,
			"authorization_endpoint":                srv.URL + "/auth",
			"token_endpoint":                        srv.URL + "/token",
			"jwks_uri":                              srv.URL + "/keys",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("/keys", func(w http.ResponseWriter, _ *http.Request) {
		pub := key.Public().(*rsa.PublicKey)
		eb := make([]byte, 4)
		binary.BigEndian.PutUint32(eb, uint32(pub.E))
		i := 0
		for i < len(eb)-1 && eb[i] == 0 {
			i++
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]any{{
				"kty": "RSA", "kid": "k", "use": "sig", "alg": "RS256",
				"n": base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
				"e": base64.RawURLEncoding.EncodeToString(eb[i:]),
			}},
		})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
			"iss": srv.URL, "aud": clientID, "sub": "sub-" + email,
			"email": email, "name": name,
			"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
		})
		tok.Header["kid"] = "k"
		signed, _ := tok.SignedString(key)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "at", "token_type": "Bearer", "id_token": signed, "expires_in": 3600,
		})
	})

	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL, clientID
}

// TestOIDCLogin_EndToEnd drives the OIDCLogin RPC against a mock IdP + real
// Postgres: it creates the user on first login and reuses the same account
// (same id) on the second — proving find-or-create by email.
func TestOIDCLogin_EndToEnd(t *testing.T) {
	env := NewTestEnv(t)
	env.CleanDB(t)
	ctx := context.Background()

	issuer, clientID := startMockOIDC(t, "traveler@example.com", "Traveler")
	provider := auth.NewOIDCProvider(issuer, clientID, "secret", "", "Authelia")
	authSvc := auth.NewService("", "", "", "test-jwt-secret-oidc-32chars-min!!")
	h := handlers.NewAuthHandler(authSvc, env.Pool, nil, nil, nil).WithOIDC(provider)

	// GetAuthProviders reports OIDC as enabled with the right metadata.
	providers, err := h.GetAuthProviders(ctx, connect.NewRequest(&toquiv1.GetAuthProvidersRequest{}))
	if err != nil {
		t.Fatalf("GetAuthProviders: %v", err)
	}
	if providers.Msg.Oidc == nil || !providers.Msg.Oidc.Enabled {
		t.Fatal("expected oidc.enabled=true")
	}
	if providers.Msg.Oidc.Name != "Authelia" || providers.Msg.Oidc.Issuer != issuer || providers.Msg.Oidc.ClientId != clientID {
		t.Errorf("oidc metadata wrong: %+v", providers.Msg.Oidc)
	}

	login := func() *toquiv1.OIDCLoginResponse {
		resp, err := h.OIDCLogin(ctx, connect.NewRequest(&toquiv1.OIDCLoginRequest{Code: "any"}))
		if err != nil {
			t.Fatalf("OIDCLogin: %v", err)
		}
		return resp.Msg
	}

	first := login()
	if first.AccessToken == "" || first.RefreshToken == "" {
		t.Fatal("login returned empty tokens")
	}
	if first.User.Email != "traveler@example.com" {
		t.Errorf("user email = %q", first.User.Email)
	}
	// The minted access token validates back to the same user.
	uid, err := authSvc.ValidateToken(first.AccessToken)
	if err != nil || uid.String() != first.User.Id {
		t.Errorf("access token doesn't validate to the user: %v / %s vs %s", err, uid, first.User.Id)
	}

	second := login()
	if second.User.Id != first.User.Id {
		t.Errorf("second OIDC login created a new account (%s) instead of reusing %s", second.User.Id, first.User.Id)
	}

	// Exactly one user row exists.
	var count int
	if err := env.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE email = $1", "traveler@example.com").Scan(&count); err != nil {
		t.Fatalf("count users: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 user row, got %d", count)
	}
}

// TestOIDCLogin_Disabled verifies the RPC is Unimplemented when OIDC isn't
// configured.
func TestOIDCLogin_Disabled(t *testing.T) {
	env := NewTestEnv(t)
	ctx := context.Background()
	authSvc := auth.NewService("", "", "", "test-jwt-secret-oidc-32chars-min!!")
	h := handlers.NewAuthHandler(authSvc, env.Pool, nil, nil, nil) // no WithOIDC

	_, err := h.OIDCLogin(ctx, connect.NewRequest(&toquiv1.OIDCLoginRequest{Code: "x"}))
	if connect.CodeOf(err) != connect.CodeUnimplemented {
		t.Errorf("got %v, want CodeUnimplemented", err)
	}
	providers, _ := h.GetAuthProviders(ctx, connect.NewRequest(&toquiv1.GetAuthProvidersRequest{}))
	if providers.Msg.Oidc != nil && providers.Msg.Oidc.Enabled {
		t.Error("oidc should report disabled")
	}
}

// TestOIDCLogin_DomainAllowlist blocks a login whose email domain isn't
// allowed.
func TestOIDCLogin_DomainAllowlist(t *testing.T) {
	env := NewTestEnv(t)
	env.CleanDB(t)
	ctx := context.Background()

	issuer, clientID := startMockOIDC(t, "outsider@evil.com", "Outsider")
	provider := auth.NewOIDCProvider(issuer, clientID, "secret", "", "Authelia")
	authSvc := auth.NewService("", "", "", "test-jwt-secret-oidc-32chars-min!!")
	h := handlers.NewAuthHandler(authSvc, env.Pool, nil, []string{"example.com"}, nil).WithOIDC(provider)

	_, err := h.OIDCLogin(ctx, connect.NewRequest(&toquiv1.OIDCLoginRequest{Code: "x"}))
	if connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("got %v, want CodePermissionDenied for a disallowed domain", err)
	}
}
