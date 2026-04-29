package apple

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// clientSecretLifetime caps how long a generated client-secret JWT is valid.
// Apple permits up to 6 months, but rotating frequently is the safer default
// — if our key ever leaks, the blast radius is bounded by this window.
const clientSecretLifetime = 5 * time.Minute

// GenerateClientSecret produces a fresh JWT signed with the Apple .p8 key.
// Apple uses this as the OAuth2 client_secret on the /auth/token POST.
//
// Claims (per Apple Developer docs):
//
//	iss = team_id
//	iat = now
//	exp = now + clientSecretLifetime
//	aud = https://appleid.apple.com (or test override)
//	sub = services_id
//
// The header carries kid = key_id and alg = ES256.
func (c *Client) GenerateClientSecret() (string, error) {
	key, err := parseECPrivateKey(c.cfg.PrivateKey)
	if err != nil {
		return "", err
	}
	now := c.now()
	claims := jwt.MapClaims{
		"iss": c.cfg.TeamID,
		"iat": now.Unix(),
		"exp": now.Add(clientSecretLifetime).Unix(),
		"aud": c.issuer,
		"sub": c.cfg.ServicesID,
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	tok.Header["kid"] = c.cfg.KeyID
	signed, err := tok.SignedString(key)
	if err != nil {
		return "", fmt.Errorf("apple: sign client secret: %w", err)
	}
	return signed, nil
}

// parseECPrivateKey extracts an ECDSA private key from a PEM-encoded .p8
// blob. Apple's keys are issued as PKCS#8 (the .p8 extension), but we
// accept the legacy SEC1 form as well so anyone hand-rolling a test key
// with `openssl ecparam` doesn't trip.
func parseECPrivateKey(pemBytes []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("apple: no PEM block found in private key")
	}
	// PKCS#8 (Apple's .p8 export)
	if k, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		ec, ok := k.(*ecdsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("apple: PKCS#8 key not ECDSA: %T", k)
		}
		return ec, nil
	}
	// SEC1 fallback
	if k, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		return k, nil
	}
	return nil, errors.New("apple: failed to parse private key (tried PKCS#8 and SEC1)")
}
