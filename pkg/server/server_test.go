package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yourusername/mcp-immich/pkg/config"
)

func TestNewServer(t *testing.T) {
	cfg := &config.Config{
		ImmichURL:    "http://localhost:2283",
		ImmichAPIKey: "test-key",
		AuthMode:     "none",
		CacheTTL:     5 * time.Minute,
		RateLimitPerSecond: 100,
		RateLimitBurst:     200,
	}

	srv, err := New(cfg)
	require.NoError(t, err)
	assert.NotNil(t, srv)
	assert.NotNil(t, srv.mcpServer)
	assert.NotNil(t, srv.immich)
	assert.NotNil(t, srv.cache)
	assert.NotNil(t, srv.rateLimiter)
	assert.NotNil(t, srv.authProvider)
}

func TestServerHealthCheck(t *testing.T) {
	cfg := &config.Config{
		ImmichURL:    "http://localhost:2283",
		ImmichAPIKey: "test-key",
		AuthMode:     "none",
		CacheTTL:     5 * time.Minute,
		RateLimitPerSecond: 100,
		RateLimitBurst:     200,
	}

	srv, err := New(cfg)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	srv.handleHealth(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "healthy")
}

func TestServerAuthModes(t *testing.T) {
	tests := []struct {
		name     string
		authMode string
		apiKeys  []string
		wantErr  bool
	}{
		{
			name:     "no auth",
			authMode: "none",
			wantErr:  false,
		},
		{
			name:     "api key auth with keys",
			authMode: "api_key",
			apiKeys:  []string{"key1", "key2"},
			wantErr:  false,
		},
		{
			name:     "api key auth without keys",
			authMode: "api_key",
			apiKeys:  []string{},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				ImmichURL:    "http://localhost:2283",
				ImmichAPIKey: "test-key",
				AuthMode:     tt.authMode,
				APIKeys:      tt.apiKeys,
				CacheTTL:     5 * time.Minute,
				RateLimitPerSecond: 100,
				RateLimitBurst:     200,
			}

			// Validate config
			err := cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	cfg := &config.Config{
		ImmichURL:    "http://localhost:2283",
		ImmichAPIKey: "test-key",
		AuthMode:     "none",
		CacheTTL:     5 * time.Minute,
		RateLimitPerSecond: 1, // Very low for testing
		RateLimitBurst:     1,
	}

	srv, err := New(cfg)
	require.NoError(t, err)

	handler := srv.rateLimitMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request should succeed
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)

	// Second immediate request should be rate limited
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusTooManyRequests, w2.Code)

	// Wait for rate limiter to reset
	time.Sleep(1100 * time.Millisecond)

	// Third request should succeed
	req3 := httptest.NewRequest(http.MethodGet, "/test", nil)
	w3 := httptest.NewRecorder()
	handler.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusOK, w3.Code)
}

func TestStartStopServer(t *testing.T) {
	cfg := &config.Config{
		ListenAddr:   ":0", // Random port
		ImmichURL:    "http://localhost:2283",
		ImmichAPIKey: "test-key",
		AuthMode:     "none",
		CacheTTL:     5 * time.Minute,
		RateLimitPerSecond: 100,
		RateLimitBurst:     200,
		RequestTimeout: 30 * time.Second,
	}

	srv, err := New(cfg)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())

	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- srv.Start(ctx)
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Stop server
	cancel()

	// Wait for server to stop
	select {
	case err := <-errChan:
		assert.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Server did not stop in time")
	}
}