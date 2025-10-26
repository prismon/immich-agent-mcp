package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/yourusername/mcp-immich/pkg/config"
	"golang.org/x/oauth2"
)

// Context keys for authentication
type contextKey int

const (
	contextKeyAPIKey contextKey = iota
	contextKeyOAuthToken
)

// Provider defines the authentication interface
type Provider interface {
	Authenticate(r *http.Request) (context.Context, error)
}

// NoOpProvider provides no authentication
type NoOpProvider struct{}

// NewNoOpProvider creates a new no-op auth provider
func NewNoOpProvider() Provider {
	return &NoOpProvider{}
}

// Authenticate always succeeds for no-op provider
func (p *NoOpProvider) Authenticate(r *http.Request) (context.Context, error) {
	return r.Context(), nil
}

// APIKeyProvider provides API key authentication
type APIKeyProvider struct {
	validKeys map[string]bool
}

// NewAPIKeyProvider creates a new API key provider
func NewAPIKeyProvider(keys []string) Provider {
	validKeys := make(map[string]bool)
	for _, key := range keys {
		validKeys[key] = true
	}
	return &APIKeyProvider{validKeys: validKeys}
}

// Authenticate validates API key from header or query param
func (p *APIKeyProvider) Authenticate(r *http.Request) (context.Context, error) {
	// Check header first
	apiKey := r.Header.Get("X-API-Key")
	if apiKey == "" {
		apiKey = r.Header.Get("x-api-key")
	}

	// Check query parameter as fallback
	if apiKey == "" {
		apiKey = r.URL.Query().Get("api_key")
	}

	if apiKey == "" {
		return nil, fmt.Errorf("no API key provided")
	}

	if !p.validKeys[apiKey] {
		return nil, fmt.Errorf("invalid API key")
	}

	// Add API key to context
	ctx := context.WithValue(r.Context(), contextKeyAPIKey, apiKey)
	return ctx, nil
}

// OAuthProvider provides OAuth 2.0 authentication
type OAuthProvider struct {
	config *oauth2.Config
}

// NewOAuthProvider creates a new OAuth provider
func NewOAuthProvider(cfg *config.OAuthConfig) (Provider, error) {
	if cfg == nil {
		return nil, fmt.Errorf("OAuth config is nil")
	}

	oauthConfig := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Scopes:       cfg.Scopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:  cfg.AuthURL,
			TokenURL: cfg.TokenURL,
		},
	}

	return &OAuthProvider{config: oauthConfig}, nil
}

// Authenticate validates OAuth bearer token
func (p *OAuthProvider) Authenticate(r *http.Request) (context.Context, error) {
	// Get bearer token from Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, fmt.Errorf("no authorization header")
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return nil, fmt.Errorf("invalid authorization header format")
	}

	token := parts[1]

	// In a real implementation, you would validate the token
	// For now, we'll just check it's not empty
	if token == "" {
		return nil, fmt.Errorf("empty bearer token")
	}

	// Add token to context
	ctx := context.WithValue(r.Context(), contextKeyOAuthToken, token)
	return ctx, nil
}

// MultiProvider tries multiple auth providers
type MultiProvider struct {
	providers []Provider
}

// NewMultiProvider creates a provider that tries multiple auth methods
func NewMultiProvider(providers ...Provider) Provider {
	return &MultiProvider{providers: providers}
}

// Authenticate tries each provider until one succeeds
func (p *MultiProvider) Authenticate(r *http.Request) (context.Context, error) {
	var lastErr error

	for _, provider := range p.providers {
		ctx, err := provider.Authenticate(r)
		if err == nil {
			return ctx, nil
		}
		lastErr = err
	}

	if lastErr != nil {
		return nil, lastErr
	}

	return nil, fmt.Errorf("no auth providers configured")
}