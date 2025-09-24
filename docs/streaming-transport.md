# MCP SDK Native Transport Implementation

## Overview

This document describes how the MCP Immich server uses the official MCP Go SDK's built-in transport capabilities without custom streaming implementations. The SDK handles all transport-level concerns including chunking, buffering, and protocol negotiations.

## Using MCP SDK Transports

The MCP Go SDK provides standard transports that handle all streaming automatically:

```go
// pkg/server/transport.go
package server

import (
    "net/http"
    "github.com/modelcontextprotocol/go-sdk/transport/stdio"
    "github.com/modelcontextprotocol/go-sdk/transport/http"
)

// HTTP transport - handles JSON-RPC over HTTP POST
func (s *MCPImmichServer) HandleHTTP(w http.ResponseWriter, r *http.Request) {
    transport := http.NewHTTPTransport(w, r)
    s.Server.ServeTransport(transport)
}

// Standard I/O transport - for CLI integration
func (s *MCPImmichServer) ServeStdio() error {
    transport := stdio.NewStdioTransport()
    return s.Server.ServeTransport(transport)
}
```

## Handling Large Results

The SDK automatically handles large responses. Tools simply return the complete result and the transport layer manages chunking:

```go
func (t *QueryPhotosTool) Execute(ctx context.Context, params json.RawMessage) (*protocol.ToolResult, error) {
    // Query with bucket pagination
    results, err := t.immich.QueryPhotosWithBuckets(ctx, input)
    if err != nil {
        return nil, err
    }

    // Return complete result - SDK handles transport-level concerns
    resultsJSON, _ := json.Marshal(results)

    return &protocol.ToolResult{
        Content: []protocol.Content{
            {
                Type: "text",
                Text: fmt.Sprintf("Found %d photos in %d buckets",
                    results.TotalCount, len(results.Buckets)),
            },
            {
                Type: "resource",
                Resource: &protocol.Resource{
                    URI:      "immich://photos/results",
                    MimeType: "application/json",
                    Text:     string(resultsJSON),
                },
            },
        },
    }, nil
}
```

## Server Configuration

The server simply initializes the MCP SDK and lets it handle all transport concerns:

```go
// pkg/server/mcp_server.go
func NewMCPServer(config *Config) (*MCPImmichServer, error) {
    // Initialize MCP server with SDK
    mcpServer := mcp.NewServer(
        mcp.WithName("mcp-immich"),
        mcp.WithVersion("1.0.0"),
        mcp.WithCapabilities(protocol.ServerCapabilities{
            Tools: &protocol.ToolsCapability{
                ListTools: true,
            },
        }),
    )

    server := &MCPImmichServer{
        Server: mcpServer,
        immich: NewImmichClient(config.ImmichURL, config.ImmichAPIKey),
        config: config,
    }

    // Register tools
    server.registerTools()

    return server, nil
}

// HTTP endpoint handlers
func (s *MCPImmichServer) SetupRoutes(r chi.Router) {
    // Single MCP endpoint - SDK handles everything
    r.Post("/mcp", s.HandleHTTP)

    // Health checks
    r.Get("/health", s.Health)
    r.Get("/ready", s.Ready)
}
```

## Transport Features Handled by SDK

The MCP SDK automatically provides:

1. **Protocol Negotiation** - Handles JSON-RPC 2.0 protocol
2. **Message Framing** - Manages message boundaries
3. **Buffering** - Internal buffering for efficiency
4. **Error Handling** - Standard error responses
5. **Connection Management** - Lifecycle management
6. **Content Encoding** - Automatic JSON marshaling

## Client Connection Examples

### HTTP Client
```bash
# Standard HTTP POST with JSON-RPC
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "tools/call",
    "params": {
      "name": "queryPhotos",
      "arguments": {"limit": 1000}
    },
    "id": 1
  }'
```

### Using MCP SDK Client
```go
import mcp "github.com/modelcontextprotocol/go-sdk/client"

client := mcp.NewClient(
    mcp.WithTransport(http.NewHTTPTransport("http://localhost:8080/mcp")),
)

result, err := client.CallTool(ctx, "queryPhotos", params)
```

## Benefits of Using SDK Transport

1. **Simplicity** - No custom streaming code to maintain
2. **Compatibility** - Guaranteed compatibility with MCP clients
3. **Reliability** - Battle-tested transport implementation
4. **Standards** - Follows MCP protocol specifications exactly
5. **Updates** - Automatic improvements with SDK updates