# MCP Protocol Implementation

## Model Context Protocol Overview

The Model Context Protocol (MCP) is a standardized protocol for communication between AI assistants and external tools/services. This implementation uses the official Go MCP SDK (`github.com/modelcontextprotocol/go-sdk`) to provide a compliant server with support for multiple transport protocols.

## Protocol Specification

### JSON-RPC 2.0 Base

MCP is built on JSON-RPC 2.0 with the following structure:

```json
{
  "jsonrpc": "2.0",
  "method": "method_name",
  "params": {},
  "id": "unique_id"
}
```

### MCP Methods

#### 1. Initialize
Establishes connection and exchanges capabilities.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "method": "initialize",
  "params": {
    "protocolVersion": "2024-11-05",
    "capabilities": {
      "tools": {},
      "prompts": {}
    },
    "clientInfo": {
      "name": "example-client",
      "version": "1.0.0"
    }
  },
  "id": 1
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "result": {
    "protocolVersion": "2024-11-05",
    "capabilities": {
      "tools": {
        "listTools": true
      },
      "prompts": {
        "listPrompts": false
      },
      "resources": {
        "listResources": false
      }
    },
    "serverInfo": {
      "name": "mcp-immich",
      "version": "1.0.0"
    }
  },
  "id": 1
}
```

#### 2. List Tools
Returns available tools and their schemas.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "method": "tools/list",
  "params": {},
  "id": 2
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "result": {
    "tools": [
      {
        "name": "queryPhotos",
        "description": "Search and filter photos in Immich",
        "inputSchema": {
          "type": "object",
          "properties": {
            "query": {
              "type": "string",
              "description": "Search query"
            },
            "startDate": {
              "type": "string",
              "format": "date-time",
              "description": "Start date for filtering"
            },
            "endDate": {
              "type": "string",
              "format": "date-time",
              "description": "End date for filtering"
            },
            "albumId": {
              "type": "string",
              "description": "Filter by album ID"
            },
            "limit": {
              "type": "integer",
              "description": "Maximum number of results",
              "minimum": 1,
              "maximum": 1000
            }
          }
        }
      },
      {
        "name": "getPhotoMetadata",
        "description": "Retrieve detailed metadata for a specific photo",
        "inputSchema": {
          "type": "object",
          "properties": {
            "photoId": {
              "type": "string",
              "description": "The ID of the photo"
            }
          },
          "required": ["photoId"]
        }
      },
      {
        "name": "moveToAlbum",
        "description": "Move photos to a specified album",
        "inputSchema": {
          "type": "object",
          "properties": {
            "photoIds": {
              "type": "array",
              "items": {
                "type": "string"
              },
              "description": "Array of photo IDs to move"
            },
            "albumId": {
              "type": "string",
              "description": "Target album ID"
            }
          },
          "required": ["photoIds", "albumId"]
        }
      },
      {
        "name": "listAlbums",
        "description": "List all available albums",
        "inputSchema": {
          "type": "object",
          "properties": {
            "shared": {
              "type": "boolean",
              "description": "Include shared albums"
            }
          }
        }
      },
      {
        "name": "searchByFace",
        "description": "Search photos by detected faces",
        "inputSchema": {
          "type": "object",
          "properties": {
            "personId": {
              "type": "string",
              "description": "ID of the person/face to search"
            },
            "limit": {
              "type": "integer",
              "description": "Maximum number of results",
              "minimum": 1,
              "maximum": 1000
            }
          },
          "required": ["personId"]
        }
      },
      {
        "name": "searchByLocation",
        "description": "Search photos by GPS coordinates",
        "inputSchema": {
          "type": "object",
          "properties": {
            "latitude": {
              "type": "number",
              "description": "Latitude coordinate",
              "minimum": -90,
              "maximum": 90
            },
            "longitude": {
              "type": "number",
              "description": "Longitude coordinate",
              "minimum": -180,
              "maximum": 180
            },
            "radius": {
              "type": "number",
              "description": "Search radius in kilometers",
              "minimum": 0.1,
              "maximum": 100
            }
          },
          "required": ["latitude", "longitude"]
        }
      }
    ]
  },
  "id": 2
}
```

#### 3. Call Tool
Execute a specific tool with parameters.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "method": "tools/call",
  "params": {
    "name": "queryPhotos",
    "arguments": {
      "query": "beach sunset",
      "limit": 10
    }
  },
  "id": 3
}
```

**Response (Success):**
```json
{
  "jsonrpc": "2.0",
  "result": {
    "content": [
      {
        "type": "text",
        "text": "Found 10 photos matching 'beach sunset'"
      },
      {
        "type": "resource",
        "resource": {
          "uri": "immich://photos/batch",
          "mimeType": "application/json",
          "text": "[{\"id\":\"photo1\",\"filename\":\"sunset1.jpg\",\"date\":\"2024-01-15\"}]"
        }
      }
    ]
  },
  "id": 3
}
```

**Response (Error):**
```json
{
  "jsonrpc": "2.0",
  "error": {
    "code": -32602,
    "message": "Invalid params",
    "data": {
      "details": "Query string too long"
    }
  },
  "id": 3
}
```

## Streaming Implementation

### Server-Sent Events (SSE)

For large responses, use SSE to stream data progressively:

```http
HTTP/1.1 200 OK
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive

event: message
data: {"jsonrpc":"2.0","method":"tools/call/progress","params":{"progress":10,"message":"Processing photos..."}}

event: message
data: {"jsonrpc":"2.0","method":"tools/call/progress","params":{"progress":50,"message":"Analyzing metadata..."}}

event: message
data: {"jsonrpc":"2.0","result":{"content":[{"type":"text","text":"Completed"}]},"id":3}

event: close
data:
```

### Chunked Transfer Encoding

For binary data or large payloads:

```http
HTTP/1.1 200 OK
Transfer-Encoding: chunked
Content-Type: application/json

1E\r\n
{"jsonrpc":"2.0","partial":true,
\r\n
2A\r\n
"result":{"content":[{"type":"text","text":
\r\n
15\r\n
"Processing complete"}
\r\n
8\r\n
]},"id":3}
\r\n
0\r\n
\r\n
```

## Transport Layer Implementation

The server supports multiple transport protocols using the MCP SDK transport interface:

### 1. Standard HTTP (POST /mcp)
Standard JSON-RPC 2.0 over HTTP for simple request-response patterns:

```go
// Client request
POST /mcp HTTP/1.1
Content-Type: application/json
X-API-Key: your-api-key  // Optional based on auth_mode

{"jsonrpc":"2.0","method":"tools/call","params":{"name":"queryPhotos","arguments":{"query":"sunset"}},"id":1}

// Server response
HTTP/1.1 200 OK
Content-Type: application/json

{"jsonrpc":"2.0","result":{"content":[...]},"id":1}
```

### 2. Server-Sent Events (GET /mcp/stream)
For streaming large datasets or real-time updates:

```go
// Client request
GET /mcp/stream HTTP/1.1
Accept: text/event-stream
X-API-Key: your-api-key  // Optional

// Server response with SSE
HTTP/1.1 200 OK
Content-Type: text/event-stream
Cache-Control: no-cache

event: message
data: {"jsonrpc":"2.0","method":"stream/start","params":{"totalItems":1000}}

event: progress
data: {"jsonrpc":"2.0","method":"stream/data","params":{"items":[...],"progress":25}}

event: complete
data: {"jsonrpc":"2.0","result":{"content":[...]},"id":1}
```

### 3. WebSocket (GET /mcp/ws)
For bidirectional real-time communication:

```go
// WebSocket upgrade
GET /mcp/ws HTTP/1.1
Upgrade: websocket
Connection: Upgrade
Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==
Sec-WebSocket-Version: 13

// Bidirectional messaging
Client -> Server: {"jsonrpc":"2.0","method":"tools/call","params":{...},"id":1}
Server -> Client: {"jsonrpc":"2.0","method":"progress","params":{"progress":50}}
Server -> Client: {"jsonrpc":"2.0","result":{...},"id":1}
```

## Error Codes

### Standard JSON-RPC Errors
- `-32700`: Parse error
- `-32600`: Invalid Request
- `-32601`: Method not found
- `-32602`: Invalid params
- `-32603`: Internal error

### MCP-Specific Errors
- `-32001`: Tool not found
- `-32002`: Tool execution failed
- `-32003`: Authentication required
- `-32004`: Rate limit exceeded
- `-32005`: Resource not found

## Session Management with MCP SDK

### Connection Lifecycle

The MCP SDK handles session management automatically. The server implementation follows this lifecycle:

```go
// 1. Server initialization
server := mcp.NewServer(
    mcp.WithName("mcp-immich"),
    mcp.WithVersion("1.0.0"),
    mcp.WithCapabilities(capabilities),
)

// 2. Tool registration
server.RegisterTool(toolDefinition, executeFunc)

// 3. Transport handling
server.ServeTransport(transport)
```

### Session Flow

1. **Client connects** via chosen transport (HTTP/SSE/WebSocket)
2. **Initialize handshake** exchanges capabilities
3. **Tool discovery** via `tools/list` method
4. **Tool execution** via `tools/call` method
5. **Streaming** if response exceeds threshold (>500 items)
6. **Graceful disconnect** or timeout

### State Management

Per-session state managed by the server:
- Transport type and connection details
- Authentication context (if configured)
- Active streaming channels
- Rate limiting counters
- Cache keys for session

## Request/Response Patterns

### Synchronous Pattern
```
Client                  Server
  |                        |
  |------- Request ------->|
  |                        | (Process)
  |<------ Response -------|
  |                        |
```

### Asynchronous Pattern with Progress
```
Client                  Server
  |                        |
  |------- Request ------->|
  |                        | (Start processing)
  |<---- Progress 10% -----|
  |<---- Progress 50% -----|
  |<---- Progress 90% -----|
  |<------ Response -------|
  |                        |
```

### Streaming Pattern
```
Client                  Server
  |                        |
  |------- Request ------->|
  |                        | (Begin stream)
  |<------ Chunk 1 --------|
  |<------ Chunk 2 --------|
  |<------ Chunk 3 --------|
  |<------ Chunk N --------|
  |<-------- End ----------|
  |                        |
```

## Content Types

### Text Content
```json
{
  "type": "text",
  "text": "Human-readable response text"
}
```

### Resource Content
```json
{
  "type": "resource",
  "resource": {
    "uri": "immich://photo/abc123",
    "mimeType": "image/jpeg",
    "text": "Base64 encoded thumbnail or metadata"
  }
}
```

### Image Content
```json
{
  "type": "image",
  "data": "base64_encoded_image_data",
  "mimeType": "image/jpeg"
}
```

## Rate Limiting

### Headers
```http
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1699564800
```

### Error Response
```json
{
  "jsonrpc": "2.0",
  "error": {
    "code": -32004,
    "message": "Rate limit exceeded",
    "data": {
      "retryAfter": 60,
      "limit": 100,
      "window": "1m"
    }
  },
  "id": 123
}
```

## Security Configuration

### Authentication Modes

Configurable via `auth_mode` setting:

#### 1. No Authentication (`auth_mode: "none"`)
```yaml
# Development or trusted environment
auth_mode: "none"
```

#### 2. API Key Authentication (`auth_mode: "api_key"`)
```yaml
auth_mode: "api_key"
api_keys:
  - "key-1234-abcd"
  - "key-5678-efgh"
```

Clients authenticate via:
```http
X-API-Key: key-1234-abcd
# OR
GET /mcp?api_key=key-1234-abcd
```

#### 3. OAuth 2.0 (`auth_mode: "oauth"`)
```yaml
auth_mode: "oauth"
oauth:
  client_id: "your-client-id"
  client_secret: "your-secret"
  auth_url: "https://auth.provider.com/authorize"
  token_url: "https://auth.provider.com/token"
  scopes: ["photos.read", "photos.write"]
```

Clients authenticate via:
```http
Authorization: Bearer <oauth_token>
```

#### 4. Combined (`auth_mode: "both"`)
Accepts either API key or OAuth token.

### Security Features

- **Immich API Key**: Stored server-side only, never exposed to clients
- **Input Validation**: JSON schema validation on all tool inputs
- **Rate Limiting**: Configurable per-client limits
- **TLS Support**: Recommended for production (terminate at load balancer)
- **CORS**: Configurable for browser-based clients

## Implementation Details

### Go SDK Integration

The server uses the official MCP Go SDK for protocol compliance:

```go
import (
    mcp "github.com/modelcontextprotocol/go-sdk"
    "github.com/modelcontextprotocol/go-sdk/protocol"
    "github.com/modelcontextprotocol/go-sdk/transport"
)
```

### Tool Interface

All tools implement the standard interface:

```go
type MCPTool interface {
    Definition() protocol.Tool
    Execute(ctx context.Context, params json.RawMessage) (*protocol.ToolResult, error)
}
```

### Streaming Threshold

Automatic streaming for large results:
- Threshold: >500 items
- Chunking: 100 items per SSE event
- Backpressure: Automatic flow control

### Configuration

All protocol behavior configurable via YAML or environment variables:

```yaml
# Protocol settings
enable_streaming: true
stream_threshold: 500
chunk_size: 100

# Timeouts
request_timeout: "30s"
stream_timeout: "5m"
idle_timeout: "2m"

# Rate limiting
max_requests_per_second: 100
burst_size: 200
```

## Client Libraries

### Compatible MCP Clients

- **TypeScript/JavaScript**: `@modelcontextprotocol/client`
- **Python**: `mcp-client`
- **Go**: Use the same SDK as client
- **Any JSON-RPC 2.0 client**: With proper message formatting

### Example Client Usage (Go)

```go
// Using the MCP SDK as a client
client := mcp.NewClient(
    mcp.WithTransport(transport.NewHTTPTransport(url)),
)

// Initialize connection
err := client.Initialize(ctx)

// List available tools
tools, err := client.ListTools(ctx)

// Call a tool
result, err := client.CallTool(ctx, "queryPhotos", params)
```