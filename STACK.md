# MCP Immich Server - Technology Stack

## Core Libraries

### MCP-go (Required)
```go
github.com/mark3labs/mcp-go v0.39.1
```
Robust Go implementation of the Model Context Protocol. Provides complete MCP protocol support with built-in transports, tool management, and testing utilities.
Documentation: https://mcp-go.dev/

### Go Version
```
Go 1.21 or later (minimum)
Go 1.22 recommended (latest stable)
```

## Production Dependencies

### HTTP Server
```go
net/http (standard library)
```
Go's standard HTTP package is sufficient. MCP-go handles routing internally.

### WebSocket Support
MCP-go includes WebSocket support built-in, no additional dependency needed.

### Configuration Management
```go
github.com/spf13/viper v1.19.0
```
Complete configuration solution supporting YAML, environment variables, and more.

### Structured Logging
```go
github.com/rs/zerolog v1.33.0
```
Zero-allocation JSON logger for high-performance logging.

### In-Memory Caching
```go
github.com/patrickmn/go-cache v2.1.0+incompatible
```
Simple, thread-safe in-memory cache with expiration.

### OAuth 2.0 (Optional)
```go
golang.org/x/oauth2 v0.22.0
```
Official OAuth 2.0 client implementation. Only needed if OAuth authentication is enabled.

### Rate Limiting
```go
golang.org/x/time v0.6.0
```
Provides rate limiting primitives.

### Metrics (Optional)
```go
github.com/prometheus/client_golang v1.20.4
```
Prometheus metrics client. Only needed if metrics endpoint is enabled.

### UUID Generation
```go
github.com/google/uuid v1.6.0
```
RFC 4122 compliant UUID generation.

### Redis Client (Optional - for distributed caching)
```go
github.com/redis/go-redis/v9 v9.6.1
```
Redis client for distributed caching. Only needed for multi-instance deployments.

## Complete go.mod

```go
module github.com/yourusername/mcp-immich

go 1.21

require (
    github.com/mark3labs/mcp-go v0.39.1
    github.com/spf13/viper v1.19.0
    github.com/rs/zerolog v1.33.0
    github.com/patrickmn/go-cache v2.1.0+incompatible
    golang.org/x/time v0.6.0
    github.com/google/uuid v1.6.0
)

// Testing dependencies
require (
    github.com/stretchr/testify v1.9.0
    github.com/golang/mock v1.6.0
)

// Optional dependencies - uncomment as needed
require (
    golang.org/x/oauth2 v0.22.0 // For OAuth authentication
    github.com/prometheus/client_golang v1.20.4 // For metrics
    github.com/redis/go-redis/v9 v9.6.1 // For distributed caching
)
```

## Development Dependencies

### Testing
```go
github.com/stretchr/testify v1.9.0
```
Testing toolkit with assertions and mocks.

### Linting
```bash
# Install golangci-lint
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.61.0
```

### Code Generation (if needed)
```go
github.com/golang/mock v1.6.0
```
Mock generation for testing.

## Docker Base Images

### Production Build
```dockerfile
# Build stage
FROM golang:1.22-alpine AS builder

# Runtime stage
FROM alpine:3.20
```

### Development
```dockerfile
FROM golang:1.22
```

## System Requirements

### Minimum Requirements
- **CPU**: 1 core
- **Memory**: 256MB RAM
- **Disk**: 50MB for binary
- **OS**: Linux, macOS, Windows (any OS supporting Go 1.21+)

### Recommended Production
- **CPU**: 2+ cores
- **Memory**: 512MB-1GB RAM
- **Disk**: 100MB for binary and logs
- **OS**: Linux (Alpine or Debian based)

## Immich API Compatibility

### Supported Immich Versions
- **Minimum**: v1.95.0
- **Recommended**: v1.106.0 or later
- **API Version**: v1

### Required Immich Features
- API key authentication
- Timeline/bucket API endpoints
- Library management endpoints
- Asset management endpoints
- Face recognition (optional)
- Machine learning features (optional)

## Network Requirements

### Ports
- **8080**: Default HTTP server port (configurable)
- **9090**: Metrics port (optional, if Prometheus enabled)

### Protocols
- **HTTP/1.1**: Minimum requirement
- **HTTP/2**: Automatically supported with TLS
- **TLS 1.2+**: Recommended for production

## Installation Commands

### Quick Start
```bash
# Clone repository
git clone https://github.com/yourusername/mcp-immich.git
cd mcp-immich

# Download dependencies
go mod download

# Build binary
go build -o mcp-immich cmd/mcp-immich/main.go

# Run server
./mcp-immich -config config.yaml
```

### Docker Build
```bash
# Build image
docker build -t mcp-immich:latest .

# Run container
docker run -d \
  -p 8080:8080 \
  -e MCP_IMMICH_URL=https://immich.example.com \
  -e MCP_IMMICH_API_KEY=your-key \
  mcp-immich:latest
```

### Production Build with Optimizations
```bash
# Build with optimizations
CGO_ENABLED=0 GOOS=linux go build \
  -ldflags="-w -s -X main.version=$(git describe --tags)" \
  -o mcp-immich \
  cmd/mcp-immich/main.go

# Strip debug symbols (reduces size by ~30%)
strip mcp-immich
```

## Version Management

### Semantic Versioning
Follow semantic versioning for releases:
- **Major**: Breaking API changes
- **Minor**: New features, backward compatible
- **Patch**: Bug fixes

### Git Tags
```bash
git tag -a v1.0.0 -m "Initial release"
git push origin v1.0.0
```

## Security Considerations

### Dependencies
- Run `go mod audit` regularly to check for vulnerabilities
- Use `dependabot` or similar for automated updates
- Pin major versions in production

### Build Security
```bash
# Scan for vulnerabilities
go install github.com/sonatype-nexus-community/nancy@latest
go list -json -deps ./... | nancy sleuth

# Static analysis
go install honnef.co/go/tools/cmd/staticcheck@latest
staticcheck ./...
```

## Performance Tuning

### Compiler Optimizations
```bash
# Profile-guided optimization (Go 1.21+)
go build -pgo=cpu.pprof -o mcp-immich cmd/mcp-immich/main.go
```

### Runtime Settings
```bash
# Set GOMAXPROCS for container environments
export GOMAXPROCS=2

# Tune GC for lower latency
export GOGC=100
export GOMEMLIMIT=512MiB
```

## Monitoring Stack (Optional)

### Prometheus
```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'mcp-immich'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: '/metrics'
```

### Grafana Dashboard
Import dashboard ID: `15789` (Go application metrics)

## CI/CD Pipeline

### GitHub Actions
```yaml
name: Build and Test
on: [push, pull_request]

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - run: go mod download
      - run: go test ./...
      - run: go build -v ./...
```

## License Compliance

All dependencies are compatible with MIT license:
- MCP SDK: MIT
- Chi: MIT
- Viper: MIT
- Zerolog: MIT
- go-cache: MIT
- All other dependencies: MIT or BSD-3-Clause