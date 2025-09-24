# Immich MCP Server

A Model Context Protocol (MCP) server implementation for Immich, providing AI assistants with comprehensive photo and video management capabilities through a standardized interface.

## Features

- **Full MCP Protocol Support** using the Go SDK (`github.com/mark3labs/mcp-go`)
- **Streamable HTTP Transport** for efficient communication
- **Comprehensive Photo Management Tools**:
  - Smart AI-powered search with advanced filtering
  - Album management and organization
  - Asset cleanup and maintenance
  - Metadata operations
  - Bulk operations with pagination support

## Quick Start

### Prerequisites

- Go 1.23 or later
- Access to an Immich instance
- Immich API key

### Installation

```bash
# Clone the repository
git clone https://github.com/yourusername/mcp-immich.git
cd mcp-immich

# Build the server
go build -o mcp-immich cmd/mcp-immich/main.go

# Configure (create config.yaml)
cat > config.yaml <<EOF
immich_url: "https://your-immich-instance.com"
immich_api_key: "your-immich-api-key"
EOF

# Run the server
./mcp-immich
```

## Configuration

### Configuration File (config.yaml)

```yaml
# Immich connection settings
immich_url: "https://immich.example.com"
immich_api_key: "your-immich-api-key"

# Server settings
listen_addr: ":8080"
log_level: "info"  # debug, info, warn, error

# Cache settings
cache_ttl: "5m"
```

## Available Tools

### 1. Query and Search Tools

#### queryPhotos
Basic photo search with filters:
```json
{
  "query": "sunset beach",
  "startDate": "2024-01-01",
  "endDate": "2024-12-31",
  "albumId": "album-uuid",
  "type": "IMAGE",
  "limit": 100
}
```

#### smartSearchAdvanced
Comprehensive AI-powered search with all Immich API options:
```json
{
  "query": "beach sunset",
  "type": "IMAGE",
  "city": "San Francisco",
  "country": "United States",
  "state": "California",
  "make": "Canon",
  "model": "iPhone 14 Pro",
  "isFavorite": true,
  "isNotInAlbum": false,
  "takenAfter": "2024-01-01T00:00:00Z",
  "takenBefore": "2024-12-31T23:59:59Z",
  "rating": 4,
  "size": 1000,
  "withExif": true
}
```

Supports all 34 Immich search parameters including:
- **Location filters**: city, country, state
- **Camera metadata**: make, model, lensModel
- **Date ranges**: created, taken, updated, trashed dates
- **Asset properties**: type, visibility, favorites, ratings
- **Album/person filters**: albumIds, personIds, tagIds
- **Advanced options**: isMotion, isOffline, isNotInAlbum, withDeleted

#### movePhotosBySearch
Search for photos and move results to an album:
```json
{
  "query": "christmas",
  "albumName": "Christmas Photos",
  "maxResults": 500,
  "createAlbum": true,
  "dryRun": false
}
```

### 2. Album Management Tools

#### listAlbums
List all albums with metadata:
```json
{
  "shared": false
}
```

#### createAlbum
Create a new album:
```json
{
  "name": "Vacation 2024",
  "description": "Summer vacation photos"
}
```

#### deleteAlbumContents
Delete all assets from an album:
```json
{
  "albumName": "Temporary Album",
  "permanent": false,
  "dryRun": true,
  "maxAssets": 1000
}
```

### 3. Asset Organization Tools

#### moveBrokenThumbnailsToAlbum
Find and organize images with broken thumbnails:
```json
{
  "albumName": "Bad Thumbnails",
  "dryRun": false,
  "createAlbum": true,
  "maxImages": 0
}
```
Successfully processed 55,038 broken thumbnail images.

#### moveSmallImagesToAlbum
Organize small images (≤400x400 pixels):
```json
{
  "albumName": "Small Images",
  "dryRun": false,
  "createAlbum": true,
  "maxImages": 0
}
```
Successfully processed 4,699 small images.

#### moveLargeMoviesToAlbum
Organize large movies (>20 minutes):
```json
{
  "albumName": "Large Movies",
  "dryRun": false,
  "createAlbum": true,
  "maxMovies": 0
}
```
Successfully processed 871 large movies.

#### movePersonalVideosFromAlbum
Separate personal videos from movies:
```json
{
  "sourceAlbumName": "Large Movies",
  "targetAlbumName": "Personal Videos",
  "dryRun": false,
  "createAlbum": true
}
```

### 4. Asset Management Tools

#### getAllAssets
Retrieve all assets with pagination:
```json
{
  "page": 1,
  "perPage": 100,
  "withExif": true
}
```

#### getPhotoMetadata
Get detailed metadata for a specific photo:
```json
{
  "photoId": "asset-uuid",
  "includeExif": true,
  "includeFaces": true
}
```

#### updateAssetMetadata
Update asset metadata:
```json
{
  "assetId": "asset-uuid",
  "description": "Sunset at the beach",
  "rating": 5,
  "isFavorite": true
}
```

#### analyzePhotos
Analyze photos for quality issues:
```json
{
  "checkBlurry": true,
  "checkDuplicates": true,
  "checkCorrupted": true
}
```

#### exportPhotos
Export photos with options:
```json
{
  "albumId": "album-uuid",
  "format": "original",
  "includeMetadata": true
}
```

### 5. Maintenance Tools

#### repairAssets
Repair broken or incomplete assets:
```json
{
  "regenerateThumbnails": true,
  "fixMetadata": true,
  "verifyChecksums": true
}
```

## Pagination Support

All tools that handle large datasets support pagination to efficiently process millions of assets:

- Automatic pagination for results > 100 items
- Configurable page sizes
- Smart batching for bulk operations
- Memory-efficient streaming for large exports

Example handling 50,000+ assets:
```json
{
  "maxImages": 0,  // 0 means unlimited
  "startPage": 1   // Processes all pages automatically
}
```

## Client Integration

### Using with Claude Desktop

Add to your Claude Desktop configuration:

```json
{
  "mcpServers": {
    "immich": {
      "command": "path/to/mcp-immich",
      "args": ["-config", "path/to/config.yaml"]
    }
  }
}
```

### Programmatic Usage

```go
import (
    "github.com/yourusername/mcp-immich/pkg/immich"
)

client := immich.NewClient(baseURL, apiKey, timeout)

// Simple search
results, err := client.SmartSearch(ctx, "sunset", 100)

// Advanced search
params := immich.SmartSearchParams{
    Query: "beach",
    Type: "IMAGE",
    IsFavorite: &trueBool,
    Size: 500,
}
results, err := client.SmartSearchAdvanced(ctx, params)
```

## Development

### Project Structure

```
immich-mcp/
├── cmd/mcp-immich/       # Application entry point
├── pkg/
│   ├── server/           # MCP server implementation
│   ├── immich/           # Immich API client
│   ├── tools/            # MCP tool implementations
│   └── config/           # Configuration handling
├── test/                 # Test files and examples
├── config.yaml          # Configuration file
└── go.mod              # Go dependencies
```

### Running Tests

```bash
# Run all tests
go test ./...

# Test specific functionality
go run test/test_smart_search.go -query "sunset" -max 100
go run test/test_advanced_search.go -query "nature" -type IMAGE -size 200
```

### Building from Source

```bash
# Install dependencies
go mod download

# Build binary
go build -o mcp-immich cmd/mcp-immich/main.go

# Run with config
./mcp-immich -config config.yaml
```

## Performance

- **Efficient Pagination**: Handles datasets with millions of assets
- **Batch Processing**: Processes assets in configurable chunks
- **Caching**: Built-in caching for frequently accessed data
- **Concurrent Operations**: Parallel processing where applicable

## Troubleshooting

### Common Issues

1. **API returns only 100 results**
   - The tool automatically handles pagination for larger requests
   - Use `size` parameter up to 5000 for bulk operations

2. **Connection to Immich fails**
   - Verify `immich_url` is accessible
   - Check `immich_api_key` is valid
   - Ensure network connectivity

3. **Slow performance with large datasets**
   - Use pagination parameters
   - Enable caching in config
   - Consider using dry run first

### Debug Logging

Enable debug logging:
```yaml
log_level: "debug"
```

## Limitations

- Maximum 5000 results per search (API limitation)
- Some filters require AI-powered search query to be set
- Bulk operations may take time for large datasets

## License

MIT License - See LICENSE file for details

## Contributing

Contributions are welcome! Please:
1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Submit a pull request

## Support

- [GitHub Issues](https://github.com/yourusername/mcp-immich/issues)
- [Immich Documentation](https://immich.app/docs)
- [MCP Specification](https://github.com/modelcontextprotocol)

## Acknowledgments

- [Immich](https://github.com/immich-app/immich) - Self-hosted photo and video backup solution
- [Model Context Protocol](https://github.com/modelcontextprotocol) - MCP specification
- [mark3labs/mcp-go](https://github.com/mark3labs/mcp-go) - Go SDK for MCP