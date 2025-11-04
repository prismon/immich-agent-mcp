package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/yourusername/mcp-immich/pkg/config"
	mcpserver "github.com/yourusername/mcp-immich/pkg/server"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = ""
)

func main() {
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	forceStdio := flag.Bool("stdio", false, "Force stdio transport mode")
	flag.Parse()

	zerolog.TimeFieldFormat = time.RFC3339

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal().Err(err).Str("config", *configPath).Msg("failed to load configuration")
	}

	// Configure logging according to config
	level, err := zerolog.ParseLevel(cfg.LogLevel)
	if err != nil {
		log.Warn().Str("level", cfg.LogLevel).Msg("invalid log level, defaulting to info")
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)
	if !cfg.LogJSON {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
	}

	transportMode := cfg.TransportMode
	if *forceStdio {
		transportMode = "stdio"
	}
	if transportMode == "" {
		transportMode = "http"
	}

	server, err := mcpserver.New(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialise MCP server")
	}

	log.Info().
		Str("version", version).
		Str("commit", commit).
		Str("built_at", date).
		Str("transport", transportMode).
		Msg("Starting MCP Immich server")

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := server.Start(ctx, transportMode); err != nil && err != context.Canceled {
		log.Fatal().Err(err).Msg("server terminated with error")
	}

	log.Info().Msg("Server exited gracefully")
}
