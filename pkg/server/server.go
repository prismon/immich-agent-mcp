package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/mark3labs/mcp-go/server"
	"github.com/patrickmn/go-cache"
	"github.com/rs/zerolog/log"
	"github.com/yourusername/mcp-immich/pkg/auth"
	"github.com/yourusername/mcp-immich/pkg/config"
	"github.com/yourusername/mcp-immich/pkg/immich"
	"github.com/yourusername/mcp-immich/pkg/tools"
	"golang.org/x/time/rate"
)

// Server represents the MCP Immich server
type Server struct {
	config         *config.Config
	mcpServer      *server.MCPServer
	streamableHTTP *server.StreamableHTTPServer
	immich         *immich.Client
	cache          *cache.Cache
	rateLimiter    *rate.Limiter
	authProvider   auth.Provider
}

// New creates a new MCP Immich server
func New(cfg *config.Config) (*Server, error) {
	if cfg.CacheTTL <= 0 {
		cfg.CacheTTL = 5 * time.Minute
	}
	if cfg.RateLimitPerSecond <= 0 {
		cfg.RateLimitPerSecond = 100
	}
	if cfg.RateLimitBurst <= 0 {
		cfg.RateLimitBurst = 200
	}
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = 30 * time.Second
	}
	if cfg.ImmichTimeout <= 0 {
		cfg.ImmichTimeout = 30 * time.Second
	}

	// Create Immich client
	immichClient := immich.NewClient(cfg.ImmichURL, cfg.ImmichAPIKey, cfg.ImmichTimeout)

	// Create cache
	cacheStore := cache.New(cfg.CacheTTL, cfg.CacheTTL*2)

	// Create rate limiter
	rateLimiter := rate.NewLimiter(rate.Limit(cfg.RateLimitPerSecond), cfg.RateLimitBurst)

	// Create auth provider
	authProvider, err := createAuthProvider(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth provider: %w", err)
	}

	// Create MCP server
	mcpServer := server.NewMCPServer(
		"mcp-immich",
		"1.0.0",
	)

	// Register all tools
	tools.RegisterTools(mcpServer, immichClient, cacheStore)

	// Create StreamableHTTP server
	streamableHTTP := server.NewStreamableHTTPServer(mcpServer)

	s := &Server{
		config:         cfg,
		mcpServer:      mcpServer,
		streamableHTTP: streamableHTTP,
		immich:         immichClient,
		cache:          cacheStore,
		rateLimiter:    rateLimiter,
		authProvider:   authProvider,
	}

	return s, nil
}

// Start starts the server with StreamableHTTP transport
func (s *Server) Start(ctx context.Context) error {
	return s.startHTTP(ctx)
}

// startHTTP starts the server with StreamableHTTP transport
func (s *Server) startHTTP(ctx context.Context) error {
	mux := http.NewServeMux()

	// MCP StreamableHTTP endpoint
	mux.HandleFunc("/mcp", s.streamableHTTP.ServeHTTP)

	// Health check
	mux.HandleFunc("/health", s.handleHealth)

	// Ready check
	mux.HandleFunc("/ready", s.handleReady)

	// Apply middleware
	handler := s.authMiddleware(
		s.rateLimitMiddleware(
			s.loggingMiddleware(mux),
		),
	)

	httpServer := &http.Server{
		Addr:         s.config.ListenAddr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: s.config.RequestTimeout,
		IdleTimeout:  60 * time.Second,
	}

	log.Info().Str("addr", s.config.ListenAddr).Msg("Starting StreamableHTTP server")

	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Wait for context or error
	select {
	case <-ctx.Done():
		log.Info().Msg("Shutting down HTTP server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	case err := <-errChan:
		return err
	}
}

// handleHealth handles health check requests
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"healthy"}`))
}

// handleReady handles readiness check requests
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	// Check Immich connectivity
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := s.immich.Ping(ctx); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"status":"not_ready","reason":"immich_unavailable"}`))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ready"}`))
}

// createAuthProvider creates the appropriate auth provider based on config
func createAuthProvider(cfg *config.Config) (auth.Provider, error) {
	switch cfg.AuthMode {
	case "none":
		return auth.NewNoOpProvider(), nil
	case "api_key":
		return auth.NewAPIKeyProvider(cfg.APIKeys), nil
	case "oauth":
		if cfg.OAuth == nil {
			return nil, fmt.Errorf("oauth config required for oauth auth mode")
		}
		return auth.NewOAuthProvider(cfg.OAuth)
	case "both":
		providers := []auth.Provider{}
		if len(cfg.APIKeys) > 0 {
			providers = append(providers, auth.NewAPIKeyProvider(cfg.APIKeys))
		}
		if cfg.OAuth != nil {
			oauthProvider, err := auth.NewOAuthProvider(cfg.OAuth)
			if err != nil {
				return nil, err
			}
			providers = append(providers, oauthProvider)
		}
		return auth.NewMultiProvider(providers...), nil
	default:
		return nil, fmt.Errorf("invalid auth mode: %s", cfg.AuthMode)
	}
}
