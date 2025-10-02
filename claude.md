# Immich MCP Server

## Overview
An MCP (Model Context Protocol) server for Immich photo management. This server provides AI assistants with tools to search, organize, and manage photos and videos in Immich through a standardized protocol interface.

## Technology Stack
- **Language**: Go 1.23+
- **MCP SDK**: `github.com/mark3labs/mcp-go`
- **Transport**: Streamable HTTP
- **API Client**: Custom Immich REST API client
- **Logging**: zerolog

## Core Features
- **Smart Search**: AI-powered photo search with 34+ filter parameters
- **Album Management**: Create, list, and manage photo albums
- **Asset Organization**: Tools for organizing photos by size, thumbnails, duration
- **Bulk Operations**: Process thousands of assets with pagination support
- **Metadata Management**: Read and update photo metadata and EXIF data

## Project Structure
```
immich-mcp/
├── cmd/mcp-immich/       # Main application entry point
├── pkg/
│   ├── server/           # MCP server with HTTP transport
│   ├── immich/           # Immich API client
│   ├── tools/            # MCP tool implementations
│   ├── config/           # Configuration management
│   └── auth/             # Authentication providers
├── test/                 # Test utilities and scripts
├── docs/                 # Additional documentation
├── config.yaml          # Server configuration
└── README.md           # Main documentation
```

## Configuration
The server uses `config.yaml` for configuration:

```yaml
# Server settings
listen_addr: ":8080"
transport_mode: "http"

# Immich connection
immich_url: "https://your-immich-instance.com"
immich_api_key: "your-api-key"

# Authentication (optional)
auth_mode: "none"  # Options: none, api_key, oauth, both

# Performance
cache_ttl: "5m"
rate_limit_per_second: 100
request_timeout: "30s"
```

## Running the Server

### Standard Mode
```bash
go build -o mcp-immich cmd/mcp-immich/main.go
./mcp-immich -config config.yaml
```

### With Custom Port
Update `config.yaml`:
```yaml
listen_addr: ":3033"
```

### Development Mode
```bash
go run cmd/mcp-immich/main.go -config config.yaml -log-level debug
```

## Available MCP Tools

### Search Tools
- `queryPhotos` - Basic photo search with filters
- `smartSearchAdvanced` - Comprehensive AI search with 34 parameters
- `movePhotosBySearch` - Search and organize photos into albums

### Album Management
- `listAlbums` - List all albums
- `createAlbum` - Create new album
- `deleteAlbumContents` - Remove assets from album

### Asset Organization
- `moveBrokenThumbnailsToAlbum` - Find assets with broken thumbnails
- `moveSmallImagesToAlbum` - Organize small images (≤400x400px)
- `moveLargeMoviesToAlbum` - Organize long videos (>20 min)
- `movePersonalVideosFromAlbum` - Separate personal videos from movies

### Asset Management
- `getAllAssets` - Retrieve all assets with pagination
- `getPhotoMetadata` - Get detailed asset metadata
- `updateAssetMetadata` - Update descriptions, ratings, favorites
- `analyzePhotos` - Analyze photo quality
- `exportPhotos` - Export photos with options

### Maintenance
- `repairAssets` - Repair broken assets and regenerate thumbnails

## Key Implementation Details

### HTTP Transport
The server uses the MCP StreamableHTTP transport (`pkg/server/server.go:56-74`):
- Handles HTTP POST requests with JSON payloads
- Supports streaming responses for large datasets
- Includes middleware for auth, rate limiting, and logging

### Immich Client
Custom client in `pkg/immich/client.go`:
- Handles all Immich API requests
- Supports pagination for large datasets
- Implements smart search with full parameter support
- Includes retry logic and error handling

### Tool Registry
Tools are registered in `pkg/tools/tools.go`:
- Each tool has input schemas defined
- Handlers process requests and call Immich client
- Support for dry-run mode on destructive operations

## Testing
```bash
# Run all tests
go test ./...

# Run specific test scripts
go run test/test_smart_search.go -query "sunset" -max 100
go run test/test_advanced_search.go -query "beach" -type IMAGE
```

## Security Warnings
⚠️ **EXPERIMENTAL AI-GENERATED CODE** - See README.md for full disclaimers
- Test on non-production Immich instances only
- Use dry-run mode before destructive operations
- This code has minimal production testing

## Performance Notes
- Handles millions of assets via pagination
- Automatic batching for bulk operations
- Configurable page sizes (default 1000)
- Memory-efficient streaming for exports
- In-memory caching for frequently accessed data

## License
AGPL-3.0 - All modifications must remain open source
