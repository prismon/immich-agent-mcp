# MCP Immich Server Architecture

## Overview

The MCP Immich Server is a Model Context Protocol (MCP) server implementation built with the official Go MCP SDK (`github.com/modelcontextprotocol/go-sdk`). It provides AI assistants with the ability to interact with Immich instances, exposing photo and video management capabilities through standardized MCP tools with support for HTTP streaming (SSE, chunked encoding, and WebSocket).

## Core Architecture Components

### 1. MCP Server Layer
Built on the official MCP Go SDK, the server provides:
- **Tool Registration**: Exposes Immich operations as MCP tools using SDK patterns
- **Request/Response Handling**: Processes JSON-RPC 2.0 messages via SDK transport layer
- **Streaming Support**: Multiple transport options (SSE, chunked HTTP, WebSocket)
- **Flexible Authentication**: Optional OAuth2 and API key authentication
- **Connection Management**: Persistent connections with configurable pooling

### 2. Communication Protocol

#### Transport Options
- **Standard HTTP**: JSON-RPC 2.0 over HTTP POST for simple requests
- **Server-Sent Events (SSE)**: Real-time streaming for large datasets at `/mcp/stream`
- **WebSocket**: Bidirectional communication at `/mcp/ws`
- **Chunked Transfer**: HTTP/1.1 chunked encoding for progressive responses

#### Features
- **Backpressure Management**: Built-in flow control for streaming
- **Connection Pooling**: Configurable connection reuse to Immich API
- **HTTP/2 Support**: Automatic with compatible clients

#### Message Format
```json
{
  "jsonrpc": "2.0",
  "method": "tools/call",
  "params": {
    "name": "queryPhotos",
    "arguments": {
      "query": "sunset",
      "limit": 50
    }
  },
  "id": "request-123"
}
```

### 3. Service Lifecycle Management

The server uses a structured lifecycle management approach:

```
┌──────────────┐
│   Starting   │
└──────┬───────┘
       │
       ▼
┌──────────────┐
│ Initializing │ ──► Load Configuration
└──────┬───────┘     Validate API Keys
       │             Initialize HTTP Server
       ▼
┌──────────────┐
│   Running    │ ──► Accept Connections
└──────┬───────┘     Process Requests
       │             Stream Responses
       ▼
┌──────────────┐
│  Stopping    │ ──► Graceful Shutdown
└──────┬───────┘     Close Connections
       │             Cleanup Resources
       ▼
┌──────────────┐
│   Stopped    │
└──────────────┘
```

### 4. Request Processing Pipeline

```
Client Request
      │
      ▼
┌─────────────────┐
│  HTTP Handler   │──► Parse HTTP Request
└────────┬────────┘    Validate Headers
         │
         ▼
┌─────────────────┐
│  MCP Processor  │──► Decode MCP Message
└────────┬────────┘    Validate Tool Call
         │
         ▼
┌─────────────────┐
│ Tool Dispatcher │──► Route to Tool Handler
└────────┬────────┘    Apply Rate Limiting
         │
         ▼
┌─────────────────┐
│ Immich Client   │──► Execute API Call
└────────┬────────┘    Handle Pagination
         │
         ▼
┌─────────────────┐
│ Response Stream │──► Format Response
└────────┬────────┘    Stream to Client
         │
         ▼
    Client Response
```

## Component Responsibilities

### MCPServer (pkg/server)
- **Primary Role**: Core server using MCP Go SDK
- **Responsibilities**:
  - Initialize MCP SDK server with capabilities
  - Register and manage tool implementations
  - Handle multiple transport protocols
  - Coordinate authentication providers
  - Manage server lifecycle and graceful shutdown
- **Key Dependencies**:
  - `github.com/modelcontextprotocol/go-sdk`
  - `github.com/go-chi/chi/v5` for HTTP routing
  - `github.com/gorilla/websocket` for WebSocket support

### ImmichClient (pkg/immich)
- **Primary Role**: Immich API wrapper
- **Responsibilities**:
  - Authenticate requests with Immich API key
  - Execute HTTP requests to Immich endpoints
  - Handle pagination for large result sets
  - Implement retry logic with exponential backoff
  - Transform Immich responses to MCP protocol format
  - Rate limiting (100 req/sec default)
  - Response caching with configurable TTL

### Transport Handlers (pkg/transport)
- **Primary Role**: Multiple streaming transport implementations
- **Components**:
  - `HTTPSSETransport`: Server-Sent Events streaming
  - `HTTPChunkedTransport`: Chunked transfer encoding
  - `WebSocketTransport`: Bidirectional WebSocket communication
- **Responsibilities**:
  - Implement MCP SDK transport interface
  - Handle message serialization/deserialization
  - Manage backpressure and flow control
  - Provide progress updates for long-running operations

### Tool System (pkg/tools)
- **Primary Role**: MCP tool implementations
- **Tool Interface**:
  - `Definition()`: Returns MCP protocol tool definition
  - `Execute()`: Handles tool invocation with parameters
- **Available Tools**:
  - `QueryPhotosTool`: Search and filter photos
  - `GetPhotoMetadataTool`: Retrieve detailed metadata
  - `MoveToAlbumTool`: Organize photos into albums
  - `ListAlbumsTool`: List available albums
  - `SearchByFaceTool`: Face recognition search
  - `SearchByLocationTool`: GPS-based search
- **Features**:
  - Automatic streaming for large result sets (>500 items)
  - Input validation via JSON schema
  - Error handling with MCP error codes

## Error Handling Strategy

### Error Categories

1. **Client Errors (4xx)**
   - Invalid MCP message format
   - Missing or invalid tool parameters
   - Authentication failures
   - Rate limit exceeded

2. **Server Errors (5xx)**
   - Immich API unavailable
   - Internal processing errors
   - Resource exhaustion
   - Configuration errors

### Error Response Format
```json
{
  "jsonrpc": "2.0",
  "error": {
    "code": -32602,
    "message": "Invalid params",
    "data": {
      "field": "albumId",
      "reason": "Album not found"
    }
  },
  "id": "request-123"
}
```

## Concurrency Model

### Request Processing
- **Async/Await**: Non-blocking I/O operations
- **Connection Pool**: Limited concurrent connections to Immich
- **Request Queue**: FIFO processing with priority support
- **Timeout Management**: Configurable timeouts for all operations

### Resource Limits
```
MAX_CONCURRENT_REQUESTS: 100
CONNECTION_POOL_SIZE: 10
REQUEST_TIMEOUT: 30s
STREAM_BUFFER_SIZE: 64KB
MAX_RESPONSE_SIZE: 100MB
```

## Security Considerations

### Authentication Modes
Configurable via `auth_mode` setting:
- **none**: No authentication required (development/trusted environments)
- **api_key**: Simple API key authentication via header or query parameter
- **oauth**: OAuth2 flow with configurable provider (optional)
- **both**: Accept either API key or OAuth token

### Security Features
- Immich API key stored securely and never exposed
- Optional TLS/SSL for all communications
- Input validation and sanitization
- Rate limiting per client
- Request size limits

### Data Protection
- TLS/SSL for all HTTP communications
- Input validation and sanitization
- Rate limiting per client
- Request size limits

### Audit Logging
- Request/response logging (sanitized)
- Error tracking and monitoring
- Performance metrics collection
- Security event logging

## Performance Optimizations

### Caching Strategy
- **Response Cache**: LRU cache for frequently accessed data
- **Connection Reuse**: Persistent HTTP connections
- **Metadata Cache**: Album and face information caching
- **TTL Management**: Configurable cache expiration

### Streaming Optimizations
- **Chunked Responses**: Break large responses into manageable chunks
- **Progressive Loading**: Send partial results as available
- **Compression**: Gzip/Brotli for response compression
- **Binary Protocol**: Optional protobuf for efficiency

## Deployment Architecture

### Standalone Deployment
```
┌─────────────┐     HTTP/SSE/WS    ┌──────────────┐     HTTPS      ┌─────────────┐
│ MCP Client  │ ◄─────────────────► │ MCP Server   │ ◄────────────► │   Immich    │
└─────────────┘                     └──────────────┘                └─────────────┘
                                           │
                                    ┌──────┴──────┐
                                    │  Config     │
                                    │  (YAML/ENV) │
                                    └─────────────┘
```

### Containerized Deployment
```
┌──────────────────────────────────────────────┐
│                Docker Host                   │
├──────────────────────────────────────────────┤
│  ┌─────────────┐                             │
│  │ MCP Server  │  Environment Variables:     │
│  │  Container  │  - MCP_IMMICH_URL          │
│  │             │  - MCP_IMMICH_API_KEY      │
│  │ Port: 8080  │  - MCP_AUTH_MODE           │
│  └──────┬──────┘  - MCP_ENABLE_STREAMING    │
│         │                                    │
│         ▼                                    │
│  ┌──────────────┐                           │
│  │    Immich    │                           │
│  │   Container  │                           │
│  └──────────────┘                           │
└──────────────────────────────────────────────┘
```

### Production Deployment with Kubernetes
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mcp-immich
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: mcp-server
        image: mcp-immich:latest
        ports:
        - containerPort: 8080
        env:
        - name: MCP_IMMICH_URL
          valueFrom:
            secretKeyRef:
              name: immich-config
              key: url
        - name: MCP_IMMICH_API_KEY
          valueFrom:
            secretKeyRef:
              name: immich-config
              key: api-key
        - name: MCP_AUTH_MODE
          value: "api_key"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
```

## Monitoring and Observability

### Metrics
- Request rate and latency per tool
- Error rate by category and tool
- Active connection count by transport type
- Stream performance metrics
- Cache hit/miss rates
- Immich API response times

### Health Endpoints
- `/health` - Basic liveness check
- `/ready` - Readiness probe including Immich connectivity
- `/metrics` - Prometheus-compatible metrics (when enabled)

### Logging
Structured logging with `zerolog`:
- **ERROR**: Critical failures requiring attention
- **WARN**: Recoverable issues and deprecations
- **INFO**: Normal operation events
- **DEBUG**: Detailed diagnostic information
- **TRACE**: Full request/response payloads (disabled by default)

## Project Structure

```
mcp-immich/
├── cmd/
│   └── mcp-immich/
│       └── main.go           # Application entry point
├── pkg/
│   ├── server/
│   │   ├── mcp_server.go     # Core MCP server implementation
│   │   ├── handlers.go       # HTTP/WebSocket handlers
│   │   └── config.go         # Configuration structures
│   ├── immich/
│   │   ├── client.go         # Immich API client
│   │   └── models.go         # Immich data models
│   ├── transport/
│   │   ├── http_sse.go       # SSE transport implementation
│   │   ├── http_chunked.go   # Chunked HTTP transport
│   │   └── websocket.go      # WebSocket transport
│   ├── tools/
│   │   ├── base.go           # Tool interface definition
│   │   ├── query_photos.go   # Photo search tool
│   │   └── [other tools].go  # Additional tool implementations
│   ├── auth/
│   │   ├── provider.go       # Authentication interfaces
│   │   ├── apikey.go         # API key authentication
│   │   └── oauth.go          # OAuth2 implementation
│   └── cache/
│       └── manager.go        # Caching layer
├── config.yaml              # Default configuration
├── Dockerfile               # Container build definition
├── go.mod                   # Go module dependencies
└── go.sum                   # Dependency checksums
```