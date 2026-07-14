package auth

import (
	"context"
	"fmt"
	"sync"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// OIDCUserInfo is the identity extracted from a verified ID token.
type OIDCUserInfo struct {
	Subject string // the `sub` claim (stable per user per provider)
	Email   string
	Name    string
}

// OIDCProvider is a generic OpenID Connect login provider — Authelia,
// Authentik, Keycloak, or any spec-compliant issuer. It sits alongside the
// Google path (which stays as-is) and is enabled purely by config.
//
// Discovery is lazy: the OIDC well-known document is fetched on first use,
// not at construction, so toqui starts cleanly even when the IdP boots
// alongside it (the common self-host / fleet case). A failed discovery is
// retried on the next request rather than cached.
type OIDCProvider struct {
	issuer       string
	clientID     string
	clientSecret string
	redirectURI  string
	name         string // display name for the sign-in button
	scopes       []string

	mu       sync.Mutex
	provider *oidc.Provider
	verifier *oidc.IDTokenVerifier
	oauth    *oauth2.Config
}

// NewOIDCProvider builds a provider from config. It performs no network I/O;
// discovery happens on first ExchangeAndVerify. name defaults to "SSO".
func NewOIDCProvider(issuer, clientID, clientSecret, redirectURI, name string) *OIDCProvider {
	if name == "" {
		name = "SSO"
	}
	return &OIDCProvider{
		issuer:       issuer,
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURI:  redirectURI,
		name:         name,
		scopes:       []string{oidc.ScopeOpenID, "email", "profile"},
	}
}

func (p *OIDCProvider) Name() string     { return p.name }
func (p *OIDCProvider) Issuer() string   { return p.issuer }
func (p *OIDCProvider) ClientID() string { return p.clientID }

// ensure lazily runs OIDC discovery and builds the verifier + oauth2 config.
// Safe for concurrent callers; a prior failure is retried.
func (p *OIDCProvider) ensure(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.provider != nil {
		return nil
	}
	provider, err := oidc.NewProvider(ctx, p.issuer)
	if err != nil {
		return fmt.Errorf("oidc discovery for %s: %w", p.issuer, err)
	}
	p.provider = provider
	p.verifier = provider.Verifier(&oidc.Config{ClientID: p.clientID})
	p.oauth = &oauth2.Config{
		ClientID:     p.clientID,
		ClientSecret: p.clientSecret,
		RedirectURL:  p.redirectURI,
		Endpoint:     provider.Endpoint(),
		Scopes:       p.scopes,
	}
	return nil
}

// ExchangeAndVerify exchanges an authorization code for tokens, verifies the
// ID token's signature + issuer + audience, and returns the user identity.
// redirectURI (the one the client used) is validated against the allowlist
// and overridden into the exchange, and codeVerifier carries PKCE — same
// handling as the Google path.
func (p *OIDCProvider) ExchangeAndVerify(ctx context.Context, code, redirectURI, codeVerifier string) (*OIDCUserInfo, error) {
	if err := p.ensure(ctx); err != nil {
		return nil, err
	}

	var opts []oauth2.AuthCodeOption
	if redirectURI != "" {
		if !AllowedRedirectURIs[redirectURI] && redirectURI != p.redirectURI {
			return nil, fmt.Errorf("redirect_uri not allowed: %s", redirectURI)
		}
		opts = append(opts, oauth2.SetAuthURLParam("redirect_uri", redirectURI))
	}
	if codeVerifier != "" {
		opts = append(opts, oauth2.SetAuthURLParam("code_verifier", codeVerifier))
	}

	token, err := p.oauth.Exchange(ctx, code, opts...)
	if err != nil {
		return nil, fmt.Errorf("oidc code exchange: %w", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return nil, fmt.Errorf("oidc token response has no id_token")
	}
	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("verify id_token: %w", err)
	}

	var claims struct {
		Email         string `json:"email"`
		EmailVerified *bool  `json:"email_verified"`
		Name          string `json:"name"`
		Subject       string `json:"sub"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("decode id_token claims: %w", err)
	}
	if claims.Email == "" {
		return nil, fmt.Errorf("id_token has no email claim (grant the email scope on the OIDC client)")
	}
	// Reject only an *explicit* email_verified:false. Many self-host IdPs
	// (Authelia) omit the claim entirely; the operator vouches for those
	// accounts by provisioning them, so absence is treated as trusted.
	if claims.EmailVerified != nil && !*claims.EmailVerified {
		return nil, fmt.Errorf("email is not verified at the identity provider")
	}

	return &OIDCUserInfo{Subject: claims.Subject, Email: claims.Email, Name: claims.Name}, nil
}
