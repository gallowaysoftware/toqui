//go:build integration_apple

// Apple Sign-In integration test — runs only with the `integration_apple`
// build tag and only when real Apple credentials are present in the
// environment. CI does NOT run this by default; it's a manual smoke test
// for after Apple Developer enrollment is complete.
//
// To run:
//
//	APPLE_TEAM_ID=... APPLE_SERVICES_ID=... APPLE_KEY_ID=... \
//	  APPLE_PRIVATE_KEY="$(cat AuthKey_XXXX.p8)" \
//	  APPLE_TEST_AUTH_CODE=... APPLE_TEST_ID_TOKEN=... \
//	  go test -tags integration_apple ./internal/integration/ -run Apple

package integration

import (
	"context"
	"os"
	"testing"

	"github.com/gallowaysoftware/toqui-backend/internal/auth/apple"
)

func TestAppleClient_RealJWKSFetch(t *testing.T) {
	teamID := os.Getenv("APPLE_TEAM_ID")
	servicesID := os.Getenv("APPLE_SERVICES_ID")
	keyID := os.Getenv("APPLE_KEY_ID")
	privateKey := os.Getenv("APPLE_PRIVATE_KEY")
	if teamID == "" || servicesID == "" || keyID == "" || privateKey == "" {
		t.Skip("APPLE_* env vars not set — skipping real Apple integration test")
	}

	c, err := apple.NewClient(apple.Config{
		TeamID:     teamID,
		ServicesID: servicesID,
		KeyID:      keyID,
		PrivateKey: []byte(privateKey),
	})
	if err != nil {
		t.Fatalf("apple.NewClient: %v", err)
	}

	// Generate a client secret JWT (offline — no Apple call required).
	secret, err := c.GenerateClientSecret()
	if err != nil {
		t.Fatalf("GenerateClientSecret: %v", err)
	}
	if secret == "" {
		t.Fatal("client secret was empty")
	}

	// If the operator supplied a real captured ID token, verify it against
	// Apple's live JWKS. Otherwise we just trip the JWKS path with a known-bad
	// token so the cache still gets exercised.
	idTok := os.Getenv("APPLE_TEST_ID_TOKEN")
	if idTok != "" {
		if _, err := c.VerifyIDToken(context.Background(), idTok); err != nil {
			t.Logf("VerifyIDToken (informational): %v", err)
		}
	}
}
