# Repository Guidelines

## Project Structure & Module Organization
The Go source lives in `pkg/` with focused subpackages: `auth` handles Immich token flows, `config` loads YAML, `immich` wraps the HTTP client, `tools` registers MCP tool definitions, and `server` wires transports and middleware. Scenario docs and protocol notes sit in `docs/`. Long-running smoke and integration harnesses reside in `test/`, while sample configuration lives in `config.yaml.example`.

## Build, Test, and Development Commands
- `make build` compiles the Streamable HTTP server targeting `cmd/mcp-immich`.
- `make run` launches the service using `config.yaml`; use `make run-stdio` for stdio transport.
- `make test` runs package tests with `-race`; append `test-short`, `test-coverage`, or `check-coverage` as needed.
- `make lint`, `make fmt`, and `make vet` enforce formatting and static checks; `make ci` bundles the full pipeline.
- Docker users can `make docker-build` to emit a container that mirrors the CI release image.

## Coding Style & Naming Conventions
Go files must stay `gofmt`-clean; run `make fmt` before opening a PR. Prefer cohesive packages inside `pkg/` and exported identifiers that align with MCP tool names (e.g. `SmartSearchAdvanced`). Tests use `*_test.go`; helper binaries in `test/` may keep `package main` when they are stand-alone scripts. Keep configuration defaults in YAML with lower_snake_case keys.

## Testing Guidelines
Unit tests belong alongside the code in `pkg/**`. Use `go test ./pkg/...` locally, and gate changes with `make test-coverage`; CI expects ≥80% coverage via `make check-coverage`. Smoke and integration suites under `test/` require an Immich sandbox and environment variables described in `test/smoke_test.go`; run them with `make test-smoke` or `make test-integration`. Record any manual validation in `test/coverage.md`.

## Commit & Pull Request Guidelines
Write imperative, present-tense commit subjects under 72 characters (`Add logging details`, `Fix bugs and update dependencies`). Squash fixups before pushing. Pull requests should link related issues, summarise risk, and call out configuration changes or new MCP tools. Include screenshots or example commands when altering tool behaviour, and confirm relevant Makefile targets succeed locally.

## Configuration & Security Notes
Copy `config.yaml.example` and never commit real Immich keys. Validate new configuration fields in `pkg/config` and document flags in `docs/`. Treat the server as experimental: guard destructive tools with logging, and prefer dry-run flags by default.
