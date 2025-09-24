# Immich API Integration Guide

## Overview

This document provides detailed specifications for integrating with the Immich API to implement MCP tools. Immich is a self-hosted photo and video management solution with a comprehensive REST API.

## API Configuration

### Base Configuration
```yaml
base_url: https://immich.example.com
api_version: v1
api_path: /api
full_endpoint: ${base_url}${api_path}
```

### Authentication
All Immich API requests require authentication via API key:

```http
GET /api/album
Host: immich.example.com
x-api-key: immich_api_key_here
Accept: application/json
```

## Core API Endpoints

### 1. Search API

#### Text Search
```http
GET /api/search
?q=sunset beach
&type=IMAGE
&isArchived=false
&isFavorite=false
&isMotion=false
&withPeople=true
&page=1
&size=100
```

**Response:**
```json
{
  "assets": {
    "total": 150,
    "count": 100,
    "items": [
      {
        "id": "asset-uuid-1",
        "deviceAssetId": "device-1",
        "ownerId": "user-uuid",
        "deviceId": "device-uuid",
        "type": "IMAGE",
        "originalPath": "/photos/IMG_001.jpg",
        "originalFileName": "IMG_001.jpg",
        "resized": true,
        "thumbhash": "base64hash",
        "fileCreatedAt": "2024-01-15T10:30:00Z",
        "fileModifiedAt": "2024-01-15T10:30:00Z",
        "updatedAt": "2024-01-15T10:35:00Z",
        "isFavorite": false,
        "isArchived": false,
        "duration": null,
        "exifInfo": {
          "make": "Apple",
          "model": "iPhone 14 Pro",
          "exifImageWidth": 4032,
          "exifImageHeight": 3024,
          "fileSizeInByte": 2500000,
          "orientation": "1",
          "dateTimeOriginal": "2024-01-15T10:30:00Z",
          "latitude": 37.7749,
          "longitude": -122.4194,
          "city": "San Francisco",
          "state": "California",
          "country": "United States",
          "iso": 100,
          "exposureTime": "1/500",
          "fNumber": 1.8,
          "lensModel": "iPhone 14 Pro back camera",
          "focalLength": 6.86
        },
        "smartInfo": {
          "tags": ["sunset", "beach", "ocean", "sky"],
          "objects": ["sun", "water", "sand", "clouds"],
          "faces": []
        }
      }
    ],
    "nextPage": "/api/search?page=2&size=100"
  },
  "albums": {
    "total": 5,
    "count": 5,
    "items": []
  }
}
```

#### Smart Search (ML-based)
```http
POST /api/search/smart
Content-Type: application/json

{
  "query": "golden retriever playing in park",
  "clip": true,
  "facets": {
    "people": [],
    "location": {
      "city": "San Francisco",
      "state": "California"
    },
    "camera": {
      "make": "Apple",
      "model": "iPhone 14 Pro"
    },
    "date": {
      "start": "2024-01-01",
      "end": "2024-12-31"
    }
  },
  "includeArchived": false,
  "take": 100,
  "skip": 0
}
```

### 2. Asset Management

#### Get Asset Details
```http
GET /api/asset/{id}
```

**Response:**
```json
{
  "id": "asset-uuid",
  "deviceAssetId": "device-1",
  "ownerId": "user-uuid",
  "type": "IMAGE",
  "originalPath": "/photos/IMG_001.jpg",
  "originalFileName": "IMG_001.jpg",
  "fileCreatedAt": "2024-01-15T10:30:00Z",
  "fileModifiedAt": "2024-01-15T10:30:00Z",
  "checksum": "sha1hash",
  "sidecarPath": null,
  "exifInfo": {...},
  "smartInfo": {...},
  "tags": [...],
  "people": [...],
  "albums": [...]
}
```

#### Update Asset Metadata
```http
PUT /api/asset/{id}
Content-Type: application/json

{
  "isFavorite": true,
  "isArchived": false,
  "description": "Beautiful sunset at the beach"
}
```

#### Batch Operations
```http
POST /api/asset/bulk
Content-Type: application/json

{
  "assetIds": ["id1", "id2", "id3"],
  "operation": "archive" | "unarchive" | "favorite" | "unfavorite" | "delete"
}
```

### 3. Album Management

#### List Albums
```http
GET /api/album
?shared=true
&assetId={assetId}
```

**Response:**
```json
[
  {
    "id": "album-uuid",
    "ownerId": "user-uuid",
    "albumName": "Summer Vacation 2024",
    "description": "Photos from our trip",
    "createdAt": "2024-01-01T00:00:00Z",
    "updatedAt": "2024-01-15T10:00:00Z",
    "albumThumbnailAssetId": "asset-uuid",
    "shared": false,
    "sharedUsers": [],
    "hasSharedLink": false,
    "assetCount": 250,
    "assets": [],
    "order": "desc"
  }
]
```

#### Create Album
```http
POST /api/album
Content-Type: application/json

{
  "albumName": "New Album",
  "description": "Album description",
  "assetIds": ["asset1", "asset2"]
}
```

#### Add Assets to Album
```http
PUT /api/album/{albumId}/assets
Content-Type: application/json

{
  "assetIds": ["asset1", "asset2", "asset3"]
}
```

#### Remove Assets from Album
```http
DELETE /api/album/{albumId}/assets
Content-Type: application/json

{
  "assetIds": ["asset1", "asset2"]
}
```

### 4. Face Recognition

#### List People
```http
GET /api/person
?withHidden=false
```

**Response:**
```json
{
  "total": 10,
  "visible": 8,
  "people": [
    {
      "id": "person-uuid",
      "name": "John Doe",
      "thumbnailPath": "/api/person/{id}/thumbnail",
      "assetCount": 150,
      "isHidden": false,
      "birthDate": "1990-01-01"
    }
  ]
}
```

#### Get Person's Assets
```http
GET /api/person/{personId}/assets
?page=1
&size=100
```

#### Merge People
```http
POST /api/person/{id}/merge
Content-Type: application/json

{
  "ids": ["person-id-2", "person-id-3"]
}
```

### 5. Location-based Search

#### Search by Coordinates
```http
POST /api/search/location
Content-Type: application/json

{
  "latitude": 37.7749,
  "longitude": -122.4194,
  "radius": 10,
  "unit": "km",
  "includeArchived": false,
  "take": 100,
  "skip": 0
}
```

#### Get Location Suggestions
```http
GET /api/search/cities
```

**Response:**
```json
[
  {
    "city": "San Francisco",
    "state": "California",
    "country": "United States",
    "assetCount": 500
  }
]
```

### 6. Timeline API

#### Get Timeline
```http
GET /api/timeline
?size=bucket | detail
&timeBucket=day | month | year
&startDate=2024-01-01
&endDate=2024-12-31
&userId={userId}
&albumId={albumId}
&includeArchived=false
&isFavorite=false
```

**Response:**
```json
{
  "buckets": [
    {
      "timeBucket": "2024-01-15",
      "assetCount": 25,
      "assets": [...]
    }
  ]
}
```

## Pagination Strategies

### Cursor-based Pagination
```http
GET /api/asset?cursor={lastAssetId}&limit=100
```

### Offset-based Pagination
```http
GET /api/asset?page=2&size=100
```

### Handling Large Result Sets
```javascript
async function* paginateResults(endpoint, params) {
  let page = 1;
  let hasMore = true;

  while (hasMore) {
    const response = await fetch(`${endpoint}?page=${page}&size=100`, params);
    const data = await response.json();

    yield data.items;

    hasMore = data.items.length === 100;
    page++;
  }
}
```

## File Operations

### Download Original
```http
GET /api/asset/download/{id}
```

### Get Thumbnail
```http
GET /api/asset/thumbnail/{id}
?format=JPEG | WEBP
&size=thumbnail | preview
```

### Get Video Stream
```http
GET /api/asset/video/{id}/stream
Range: bytes=0-1024
```

## Error Handling

### Error Response Format
```json
{
  "error": "Bad Request",
  "message": "Invalid search parameters",
  "statusCode": 400,
  "details": {
    "field": "startDate",
    "reason": "Invalid date format"
  }
}
```

### Common Error Codes
- `400`: Bad Request - Invalid parameters
- `401`: Unauthorized - Invalid or missing API key
- `403`: Forbidden - Insufficient permissions
- `404`: Not Found - Resource doesn't exist
- `409`: Conflict - Resource already exists
- `413`: Payload Too Large - Request body too large
- `429`: Too Many Requests - Rate limit exceeded
- `500`: Internal Server Error
- `503`: Service Unavailable

## Rate Limiting

### Headers
```http
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 995
X-RateLimit-Reset: 1699564800
Retry-After: 60
```

### Strategies
1. **Exponential Backoff**
   ```javascript
   let delay = 1000; // Start with 1 second
   for (let i = 0; i < maxRetries; i++) {
     try {
       return await makeRequest();
     } catch (error) {
       if (error.status === 429) {
         await sleep(delay);
         delay *= 2; // Double the delay
       } else {
         throw error;
       }
     }
   }
   ```

2. **Request Queuing**
   - Implement request queue with rate limiting
   - Maximum concurrent requests: 10
   - Requests per second: 100

## WebSocket Events

### Connection
```javascript
ws://immich.example.com/api/ws
Authorization: Bearer {api_key}
```

### Event Types
```json
{
  "type": "upload.progress",
  "data": {
    "assetId": "asset-uuid",
    "progress": 75,
    "total": 100
  }
}
```

### Available Events
- `upload.start`
- `upload.progress`
- `upload.complete`
- `upload.error`
- `asset.delete`
- `asset.update`
- `album.update`
- `person.merge`

## Batch Processing

### Batch Upload
```http
POST /api/asset/upload
Content-Type: multipart/form-data

------WebKitFormBoundary
Content-Disposition: form-data; name="assetData"; filename="photo1.jpg"
Content-Type: image/jpeg

[Binary data]
------WebKitFormBoundary
Content-Disposition: form-data; name="deviceAssetId"

device-asset-id-1
------WebKitFormBoundary
Content-Disposition: form-data; name="deviceId"

device-uuid
------WebKitFormBoundary
Content-Disposition: form-data; name="fileCreatedAt"

2024-01-15T10:30:00Z
------WebKitFormBoundary--
```

### Batch Delete
```http
DELETE /api/asset
Content-Type: application/json

{
  "ids": ["asset1", "asset2", "asset3"],
  "force": false
}
```

## Performance Optimization

### Field Selection
```http
GET /api/asset?fields=id,originalFileName,exifInfo.dateTimeOriginal
```

### Response Compression
```http
GET /api/search
Accept-Encoding: gzip, deflate, br
```

### Connection Pooling
- Keep-Alive timeout: 120 seconds
- Maximum connections: 10
- Connection TTL: 600 seconds

### Caching Headers
```http
Cache-Control: private, max-age=3600
ETag: "asset-version-hash"
Last-Modified: Thu, 15 Jan 2024 10:30:00 GMT
```

## Migration and Sync

### Get Server Info
```http
GET /api/server-info
```

**Response:**
```json
{
  "version": "v1.95.0",
  "versionMajor": 1,
  "versionMinor": 95,
  "versionPatch": 0,
  "features": {
    "clipEncode": true,
    "facialRecognition": true,
    "map": true,
    "reverseGeocoding": true,
    "search": true,
    "trash": true,
    "oauth": false,
    "passwordLogin": true
  }
}
```

### Get Storage Info
```http
GET /api/server-info/storage
```

**Response:**
```json
{
  "diskAvailable": 50000000000,
  "diskSize": 100000000000,
  "diskUse": 50000000000,
  "diskUsagePercentage": 50
}
```