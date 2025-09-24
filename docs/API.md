# Immich MCP Server API Documentation

## Overview

The Immich MCP Server provides a Model Context Protocol interface to Immich's photo management capabilities. This document details all available tools, their parameters, and usage examples.

## Table of Contents

1. [Search Tools](#search-tools)
2. [Album Management](#album-management)
3. [Asset Organization](#asset-organization)
4. [Asset Management](#asset-management)
5. [Maintenance Tools](#maintenance-tools)

## Search Tools

### queryPhotos

Basic photo search with simple filters.

**Parameters:**
- `query` (string, optional): Search query text
- `startDate` (string, optional): Start date in ISO format
- `endDate` (string, optional): End date in ISO format
- `albumId` (string, optional): Filter by album ID
- `type` (string, optional): Asset type (IMAGE, VIDEO, ALL)
- `limit` (integer, default: 100): Maximum results to return

**Example:**
```json
{
  "query": "beach sunset",
  "startDate": "2024-01-01",
  "endDate": "2024-12-31",
  "type": "IMAGE",
  "limit": 50
}
```

### smartSearchAdvanced

Comprehensive AI-powered search with all Immich API options.

**Parameters:**
- `query` (string): AI-powered semantic search query
- `albumIds` (string[]): Filter by album IDs
- `personIds` (string[]): Filter by person IDs
- `tagIds` (string[]): Filter by tag IDs
- `city` (string): Filter by city name
- `country` (string): Filter by country name
- `state` (string): Filter by state/province
- `make` (string): Camera manufacturer (e.g., "Canon", "Apple")
- `model` (string): Camera model (e.g., "iPhone 14 Pro")
- `lensModel` (string): Lens model
- `deviceId` (string): Specific device ID
- `libraryId` (string): Library ID
- `queryAssetId` (string): Find similar assets to this ID
- `type` (string): Asset type (IMAGE, VIDEO, AUDIO, OTHER)
- `visibility` (string): Visibility status (archive, timeline, hidden, locked)
- `createdAfter` (string): Assets created after date (ISO 8601)
- `createdBefore` (string): Assets created before date (ISO 8601)
- `takenAfter` (string): Photos taken after date (ISO 8601)
- `takenBefore` (string): Photos taken before date (ISO 8601)
- `updatedAfter` (string): Assets updated after date (ISO 8601)
- `updatedBefore` (string): Assets updated before date (ISO 8601)
- `trashedAfter` (string): Assets trashed after date (ISO 8601)
- `trashedBefore` (string): Assets trashed before date (ISO 8601)
- `isFavorite` (boolean): Filter favorites only
- `isEncoded` (boolean): Filter by encoding status
- `isMotion` (boolean): Filter motion photos/videos
- `isOffline` (boolean): Filter offline assets
- `isNotInAlbum` (boolean): Assets not in any album
- `withDeleted` (boolean): Include deleted assets
- `withExif` (boolean): Include EXIF data in results
- `rating` (integer, -1 to 5): Filter by rating
- `size` (integer, 1-5000, default: 100): Maximum results
- `language` (string): Language for query processing

**Example:**
```json
{
  "query": "mountains",
  "type": "IMAGE",
  "country": "Switzerland",
  "isFavorite": true,
  "takenAfter": "2023-01-01T00:00:00Z",
  "takenBefore": "2023-12-31T23:59:59Z",
  "size": 500,
  "withExif": true
}
```

### movePhotosBySearch

Search for photos and automatically move them to an album.

**Parameters:**
- `query` (string, required): Search query
- `albumName` (string, required): Target album name
- `maxResults` (integer, default: 100): Maximum photos to move
- `createAlbum` (boolean, default: true): Create album if doesn't exist
- `dryRun` (boolean, default: false): Preview without making changes

**Example:**
```json
{
  "query": "birthday party",
  "albumName": "Birthday Parties",
  "maxResults": 200,
  "createAlbum": true,
  "dryRun": false
}
```

## Album Management

### listAlbums

List all albums with metadata.

**Parameters:**
- `shared` (boolean, optional): Filter shared albums only

**Response:**
- Array of albums with:
  - `id`: Album ID
  - `albumName`: Album name
  - `description`: Album description
  - `assetCount`: Number of assets
  - `createdAt`: Creation date
  - `updatedAt`: Last update date

**Example:**
```json
{
  "shared": false
}
```

### createAlbum

Create a new album.

**Parameters:**
- `name` (string, required): Album name
- `description` (string, optional): Album description

**Example:**
```json
{
  "name": "Summer Vacation 2024",
  "description": "Photos from our trip to Hawaii"
}
```

### deleteAlbumContents

Delete all assets from an album.

**Parameters:**
- `albumName` (string, required): Album name
- `permanent` (boolean, default: false): Permanently delete (vs trash)
- `dryRun` (boolean, default: true): Preview without deleting
- `maxAssets` (integer, default: 1000): Maximum assets to delete

**Example:**
```json
{
  "albumName": "Temporary Photos",
  "permanent": false,
  "dryRun": true,
  "maxAssets": 500
}
```

## Asset Organization

### moveBrokenThumbnailsToAlbum

Find and organize images with missing or broken thumbnails.

**Parameters:**
- `albumName` (string, default: "Bad Thumbnails"): Target album name
- `dryRun` (boolean, default: false): Preview mode
- `createAlbum` (boolean, default: true): Create album if needed
- `maxImages` (integer, default: 0): Maximum images (0 = unlimited)
- `startPage` (integer, default: 1): Starting page for pagination

**Detection Criteria:**
- Missing thumbhash
- Image type assets only

**Example:**
```json
{
  "albumName": "Broken Thumbnails",
  "dryRun": false,
  "maxImages": 0
}
```

### moveSmallImagesToAlbum

Organize small images (≤400x400 pixels).

**Parameters:**
- `albumName` (string, default: "Small Images"): Target album name
- `dryRun` (boolean, default: false): Preview mode
- `createAlbum` (boolean, default: true): Create album if needed
- `maxImages` (integer, default: 0): Maximum images (0 = unlimited)
- `startPage` (integer, default: 1): Starting page for pagination

**Detection Criteria:**
- Width ≤ 400 pixels AND height ≤ 400 pixels
- Image type assets only

**Example:**
```json
{
  "albumName": "Small Images",
  "dryRun": false,
  "maxImages": 1000
}
```

### moveLargeMoviesToAlbum

Organize large movie files (>20 minutes).

**Parameters:**
- `albumName` (string, default: "Large Movies"): Target album name
- `dryRun` (boolean, default: false): Preview mode
- `createAlbum` (boolean, default: true): Create album if needed
- `maxMovies` (integer, default: 0): Maximum movies (0 = unlimited)
- `startPage` (integer, default: 1): Starting page for pagination

**Detection Criteria:**
- Video duration > 20 minutes
- Video type assets only

**Example:**
```json
{
  "albumName": "Feature Films",
  "dryRun": false,
  "maxMovies": 100
}
```

### movePersonalVideosFromAlbum

Separate personal videos from movie collections.

**Parameters:**
- `sourceAlbumName` (string, required): Source album to check
- `targetAlbumName` (string, default: "Personal Videos"): Target album
- `dryRun` (boolean, default: false): Preview mode
- `createAlbum` (boolean, default: true): Create target album if needed

**Detection Criteria:**
- Videos containing "IMG_", "VID_", "MOV_" prefixes
- Common smartphone video patterns
- Personal camera naming conventions

**Example:**
```json
{
  "sourceAlbumName": "Large Movies",
  "targetAlbumName": "Home Videos",
  "dryRun": false
}
```

## Asset Management

### getAllAssets

Retrieve all assets with pagination support.

**Parameters:**
- `page` (integer, default: 1): Page number
- `perPage` (integer, default: 100): Items per page
- `withExif` (boolean, default: false): Include EXIF metadata

**Response:**
- `assets`: Array of asset objects
- `total`: Total number of assets
- `page`: Current page
- `perPage`: Items per page
- `hasMore`: Boolean indicating more pages available

**Example:**
```json
{
  "page": 1,
  "perPage": 250,
  "withExif": true
}
```

### getPhotoMetadata

Get detailed metadata for a specific photo.

**Parameters:**
- `photoId` (string, required): Asset ID
- `includeExif` (boolean, default: true): Include EXIF data
- `includeFaces` (boolean, default: false): Include face detection data

**Response:**
- Complete asset metadata including:
  - Basic info (filename, type, size)
  - EXIF data (camera, settings, location)
  - Face detection results
  - Album associations

**Example:**
```json
{
  "photoId": "abc-123-def-456",
  "includeExif": true,
  "includeFaces": true
}
```

### updateAssetMetadata

Update asset metadata.

**Parameters:**
- `assetId` (string, required): Asset ID
- `description` (string, optional): New description
- `rating` (integer, optional): Rating (1-5)
- `isFavorite` (boolean, optional): Favorite status
- `isArchived` (boolean, optional): Archive status

**Example:**
```json
{
  "assetId": "abc-123-def-456",
  "description": "Beautiful sunset at the beach",
  "rating": 5,
  "isFavorite": true
}
```

### analyzePhotos

Analyze photos for quality issues.

**Parameters:**
- `checkBlurry` (boolean, default: true): Check for blurry images
- `checkDuplicates` (boolean, default: true): Find duplicates
- `checkCorrupted` (boolean, default: true): Check for corrupted files
- `albumId` (string, optional): Limit to specific album

**Response:**
- `blurryPhotos`: Array of potentially blurry images
- `duplicates`: Groups of duplicate images
- `corrupted`: List of corrupted files
- `summary`: Analysis summary statistics

**Example:**
```json
{
  "checkBlurry": true,
  "checkDuplicates": true,
  "checkCorrupted": false
}
```

### exportPhotos

Export photos with various options.

**Parameters:**
- `albumId` (string, optional): Export specific album
- `assetIds` (string[], optional): Export specific assets
- `format` (string, default: "original"): Export format
- `includeMetadata` (boolean, default: true): Include metadata
- `includeRaw` (boolean, default: false): Include RAW files

**Example:**
```json
{
  "albumId": "vacation-2024",
  "format": "original",
  "includeMetadata": true,
  "includeRaw": false
}
```

## Maintenance Tools

### repairAssets

Repair broken or incomplete assets.

**Parameters:**
- `regenerateThumbnails` (boolean, default: true): Regenerate thumbnails
- `fixMetadata` (boolean, default: true): Repair metadata
- `verifyChecksums` (boolean, default: true): Verify file integrity
- `dryRun` (boolean, default: false): Preview repairs without applying

**Response:**
- `thumbnailsRegenerated`: Count of regenerated thumbnails
- `metadataFixed`: Count of metadata repairs
- `checksumsVerified`: Count of verified files
- `errors`: List of unrecoverable errors

**Example:**
```json
{
  "regenerateThumbnails": true,
  "fixMetadata": true,
  "verifyChecksums": false,
  "dryRun": true
}
```

## Pagination

All tools that return large datasets support pagination:

### Automatic Pagination
Tools automatically paginate when retrieving more than 100 items:
- Smart search: Up to 5000 results
- Asset organization: Unlimited with `maxImages: 0`
- Album operations: Handles millions of assets

### Manual Pagination
For fine control, use pagination parameters:
- `page`: Page number (1-based)
- `perPage`: Items per page
- `startPage`: Starting page for bulk operations

## Error Handling

All tools return standardized error responses:

```json
{
  "error": {
    "code": "INVALID_PARAMETER",
    "message": "Description of the error",
    "details": {
      "field": "fieldName",
      "reason": "Detailed reason"
    }
  }
}
```

Common error codes:
- `INVALID_PARAMETER`: Invalid input parameters
- `NOT_FOUND`: Resource not found
- `PERMISSION_DENIED`: Insufficient permissions
- `RATE_LIMITED`: Too many requests
- `SERVER_ERROR`: Internal server error

## Rate Limiting

The API implements rate limiting to protect the Immich server:
- Default: 100 requests per minute
- Bulk operations: 10 requests per minute
- Configurable via server settings

## Best Practices

1. **Use Dry Run First**: Test operations with `dryRun: true`
2. **Batch Operations**: Use bulk tools for large datasets
3. **Handle Pagination**: Process results in pages for large datasets
4. **Check Capabilities**: Verify tool availability before use
5. **Monitor Progress**: Use response counts to track operations
6. **Error Recovery**: Implement retry logic for transient errors

## Examples

### Organize Photos by Year

```json
// Find photos from 2023
{
  "tool": "smartSearchAdvanced",
  "params": {
    "query": "photos",
    "takenAfter": "2023-01-01T00:00:00Z",
    "takenBefore": "2023-12-31T23:59:59Z",
    "size": 5000
  }
}

// Move to album
{
  "tool": "movePhotosBySearch",
  "params": {
    "query": "photos",
    "albumName": "2023 Photos",
    "maxResults": 5000
  }
}
```

### Clean Up Library

```json
// Find broken thumbnails
{
  "tool": "moveBrokenThumbnailsToAlbum",
  "params": {
    "albumName": "Needs Repair",
    "maxImages": 0
  }
}

// Repair assets
{
  "tool": "repairAssets",
  "params": {
    "regenerateThumbnails": true,
    "fixMetadata": true
  }
}
```

### Export Favorites

```json
// Find favorites
{
  "tool": "smartSearchAdvanced",
  "params": {
    "query": "*",
    "isFavorite": true,
    "size": 1000
  }
}

// Export them
{
  "tool": "exportPhotos",
  "params": {
    "assetIds": ["<from search results>"],
    "includeMetadata": true
  }
}
```