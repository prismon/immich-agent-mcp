# Immich MCP Agent Architecture

## Overview
The Immich MCP Agent exposes Immich photo management capabilities as Model Context Protocol (MCP) tools.
The project is organised as a set of focused Go packages that wrap the Immich HTTP API, orchestrate
transport concerns such as authentication, caching and rate limiting, and register a catalogue of
tools that can be invoked by MCP compatible clients.

```
+-------------+       +-----------------+       +-----------------------+
| MCP Client  | <-->  | Server Package  | <-->  | Immich Client Package |
+-------------+       |  (HTTP + MCP)   |       |   (REST wrapper)      |
        ^             +-----------------+       +-----------------------+
        |                     ^                          |
        |                     |                          v
        |             +---------------+         +-----------------+
        |             | Auth Package  |         | Cache + Rate    |
        |             | (pluggable)   |         | limiting helpers|
        |             +---------------+         +-----------------+
        |                     ^                          |
        |                     |                          v
        |             +-----------------+        +-----------------------+
        +-----------> | Tools Package   | ----> | Immich HTTP Endpoints  |
                      | (tool registry) |        +-----------------------+
                      +-----------------+
```

## Package responsibilities

### `pkg/config`
* Loads configuration from YAML files or environment variables using **Viper**.
* Applies defaults for network timeouts, cache behaviour, rate limiting and metrics settings.
* Performs structural validation for the Immich connection and authentication mode so that the server
  starts with a coherent configuration.

### `pkg/auth`
* Defines the `Provider` interface used by the HTTP server middleware.
* Implements `NoOp`, `APIKey`, `OAuth` and `MultiProvider` strategies, allowing deployments to
  combine authentication schemes without changing the server code.

### `pkg/immich`
* Wraps the Immich REST API with a strongly typed client that supports pagination, smart search,
  album management, maintenance operations and export helpers.
* Adds resiliency features such as request timeouts, exponential rate limiting and consistent error
  wrapping so higher layers receive actionable feedback.
* Provides utility types shared by the tool implementations.

### `pkg/tools`
* Registers the full catalogue of MCP tools with the MCP server instance.
* Contains the business logic that maps tool calls to Immich client calls, applies caching where
  beneficial and normalises results for the MCP protocol.

### `pkg/server`
* Bootstraps the Immich client, authentication provider, in-memory cache and rate limiter.
* Hosts an HTTP server that exposes `/mcp`, `/health` and `/ready` endpoints. Requests flow through
  logging, rate limiting and authentication middleware before reaching the MCP handler.
* Encapsulates transport-specific concerns so that tool implementations remain focused on the
  Immich domain.

## Request lifecycle

1. **Inbound request** – The MCP client sends an HTTP request to `/mcp`. The logging middleware
   records the request metadata.
2. **Rate limiting** – The rate limiter enforces the configured per-second and burst thresholds to
   protect the Immich backend.
3. **Authentication** – The selected auth provider validates API keys or bearer tokens and enriches
   the request context.
4. **Tool dispatch** – The MCP server routes the call to the registered handler inside `pkg/tools`.
5. **Immich interaction** – Tool handlers call the Immich client, which performs authenticated HTTP
   requests, applies request timeouts and surfaces errors with context.
6. **Response composition** – Handlers transform Immich responses into MCP content structures and
   optionally cache the result for repeated calls.

## Operational concerns

* **Health and readiness** – `/health` returns a static response, while `/ready` performs a live
  `Ping` against the Immich API to verify connectivity before reporting readiness.
* **Caching** – A shared in-memory cache reduces repeated API calls for frequently requested data
  such as album and search results. Cache lifetime is configurable.
* **Rate limiting** – Both the Immich client and the HTTP server enforce rate limiting to shield the
  upstream service from bursts and to provide back-pressure to callers.
* **Extensibility** – New tools can be added by extending `pkg/tools` and registering them inside
  `RegisterTools`. Shared configuration and client facilities avoid duplication across handlers.

## Testing strategy

* Package-level tests in `pkg/server` validate middleware, configuration fallbacks and lifecycle
  behaviour without requiring a running Immich instance.
* Integration-style smoke tests in the `test` package exercise the registered tools when Immich
  credentials are supplied via configuration or environment variables. These tests automatically
  skip when the external dependencies are unavailable, keeping the default `go test ./...` run fast.

