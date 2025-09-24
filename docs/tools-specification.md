# MCP Tools Specification

## Overview

This document provides complete specifications for all MCP tools exposed by the Immich server, including input/output schemas, behavior, and implementation requirements.

## Tool Definitions

### 1. queryPhotos

**Purpose:** Search and filter photos in Immich based on various criteria.

**Input Schema:**
```typescript
interface QueryPhotosInput {
  // Text search query
  query?: string;

  // Date range filters
  startDate?: string; // ISO 8601 format
  endDate?: string;   // ISO 8601 format

  // Album filter
  albumId?: string;

  // Asset type filter
  type?: "IMAGE" | "VIDEO" | "ALL";

  // Status filters
  isFavorite?: boolean;
  isArchived?: boolean;

  // Pagination
  limit?: number;  // 1-1000, default: 100
  offset?: number; // default: 0

  // Sort options
  sortBy?: "date" | "name" | "size" | "modified";
  sortOrder?: "asc" | "desc";
}
```

**Output Schema:**
```typescript
interface QueryPhotosOutput {
  success: boolean;
  totalCount: number;
  returnedCount: number;
  photos: Photo[];
  nextOffset?: number;
  executionTime: number; // milliseconds
}

interface Photo {
  id: string;
  filename: string;
  originalPath: string;
  type: "IMAGE" | "VIDEO";
  mimeType: string;
  fileSize: number;
  createdAt: string;
  modifiedAt: string;

  // Optional metadata
  width?: number;
  height?: number;
  duration?: number; // for videos, in seconds

  // Location data
  location?: {
    latitude: number;
    longitude: number;
    city?: string;
    state?: string;
    country?: string;
  };

  // Camera data
  exif?: {
    make?: string;
    model?: string;
    iso?: number;
    exposureTime?: string;
    fNumber?: number;
    focalLength?: number;
    lens?: string;
  };

  // AI-generated tags
  tags?: string[];
  objects?: string[];

  // URLs
  thumbnailUrl: string;
  previewUrl: string;
  downloadUrl: string;
}
```

**Implementation Notes:**
- Combine multiple search criteria using AND logic
- Support partial text matching with fuzzy search
- Cache frequent queries for performance
- Stream results if count > 500

**Example Request:**
```json
{
  "query": "sunset beach",
  "startDate": "2024-01-01",
  "endDate": "2024-12-31",
  "type": "IMAGE",
  "isFavorite": true,
  "limit": 50,
  "sortBy": "date",
  "sortOrder": "desc"
}
```

### 2. getPhotoMetadata

**Purpose:** Retrieve detailed metadata for a specific photo or video.

**Input Schema:**
```typescript
interface GetPhotoMetadataInput {
  photoId: string;
  includeExif?: boolean;     // default: true
  includeFaces?: boolean;     // default: true
  includeAlbums?: boolean;    // default: true
  includeTags?: boolean;      // default: true
  includeHistory?: boolean;   // default: false
}
```

**Output Schema:**
```typescript
interface GetPhotoMetadataOutput {
  success: boolean;
  photo: DetailedPhoto;
}

interface DetailedPhoto extends Photo {
  // Extended EXIF data
  exifFull?: {
    // All EXIF tags
    [key: string]: any;
  };

  // Face recognition data
  faces?: Face[];

  // Album memberships
  albums?: Album[];

  // Edit history
  history?: HistoryEntry[];

  // File information
  checksum: string;
  originalName: string;
  sidecarFiles?: string[];

  // Sharing information
  shared: boolean;
  shareLinks?: ShareLink[];

  // AI analysis
  smartInfo?: {
    clipEmbedding?: number[];
    sceneCategories?: string[];
    aestheticScore?: number;
    qualityScore?: number;
  };
}

interface Face {
  id: string;
  personId?: string;
  personName?: string;
  boundingBox: {
    x: number;
    y: number;
    width: number;
    height: number;
  };
  confidence: number;
}

interface Album {
  id: string;
  name: string;
  description?: string;
  coverPhotoId?: string;
}

interface HistoryEntry {
  timestamp: string;
  action: string;
  userId: string;
  details?: any;
}
```

**Implementation Notes:**
- Cache metadata for recently accessed photos
- Lazy load expensive data (faces, history)
- Include computed fields (aesthetic score)

### 3. moveToAlbum

**Purpose:** Move one or more photos to a specified album.

**Input Schema:**
```typescript
interface MoveToAlbumInput {
  photoIds: string[];
  albumId: string;
  removeFromOtherAlbums?: boolean; // default: false
  createAlbumIfNotExists?: boolean; // default: false
  albumName?: string; // Required if createAlbumIfNotExists is true
}
```

**Output Schema:**
```typescript
interface MoveToAlbumOutput {
  success: boolean;
  movedCount: number;
  failedCount: number;
  failures?: MoveFailure[];
  albumId: string;
  albumName: string;
}

interface MoveFailure {
  photoId: string;
  reason: string;
  errorCode: string;
}
```

**Implementation Notes:**
- Batch operations for performance
- Validate all photos exist before moving
- Handle permission checks
- Support atomic operations (all or nothing)

### 4. listAlbums

**Purpose:** List all available albums with optional filtering.

**Input Schema:**
```typescript
interface ListAlbumsInput {
  // Filter options
  shared?: boolean;
  ownerId?: string;
  search?: string;

  // Include options
  includeEmpty?: boolean;    // default: true
  includeCounts?: boolean;   // default: true
  includeThumbnails?: boolean; // default: true

  // Pagination
  limit?: number;  // 1-1000
  offset?: number;

  // Sorting
  sortBy?: "name" | "created" | "modified" | "assetCount";
  sortOrder?: "asc" | "desc";
}
```

**Output Schema:**
```typescript
interface ListAlbumsOutput {
  success: boolean;
  totalCount: number;
  albums: AlbumInfo[];
}

interface AlbumInfo {
  id: string;
  name: string;
  description?: string;
  createdAt: string;
  updatedAt: string;

  // Ownership
  ownerId: string;
  ownerName: string;

  // Sharing
  shared: boolean;
  sharedWith?: string[];
  shareLink?: string;

  // Statistics
  assetCount: number;
  videoCount: number;
  photoCount: number;
  totalSize: number;

  // Visual
  thumbnailUrl?: string;
  coverPhotoId?: string;

  // Metadata
  startDate?: string; // Earliest photo date
  endDate?: string;   // Latest photo date
  locations?: string[]; // Unique locations
}
```

**Implementation Notes:**
- Cache album list with TTL
- Compute statistics asynchronously
- Support incremental loading

### 5. searchByFace

**Purpose:** Search for photos containing a specific person's face.

**Input Schema:**
```typescript
interface SearchByFaceInput {
  personId?: string;
  personName?: string; // Alternative to personId

  // Additional filters
  startDate?: string;
  endDate?: string;
  albumId?: string;

  // Face detection options
  minConfidence?: number; // 0-1, default: 0.7
  includeHidden?: boolean;
  includeSimilar?: boolean; // Find similar faces

  // Pagination
  limit?: number;
  offset?: number;
}
```

**Output Schema:**
```typescript
interface SearchByFaceOutput {
  success: boolean;
  personId: string;
  personName?: string;
  totalCount: number;
  photos: FacePhoto[];
}

interface FacePhoto extends Photo {
  faceData: {
    boundingBox: BoundingBox;
    confidence: number;
    landmarks?: FaceLandmarks;
    embedding?: number[];
  };

  otherPeople?: {
    id: string;
    name?: string;
    confidence: number;
  }[];
}

interface FaceLandmarks {
  leftEye: Point;
  rightEye: Point;
  nose: Point;
  mouth: Point;
}

interface Point {
  x: number;
  y: number;
}
```

**Implementation Notes:**
- Use face embedding for similarity search
- Support multiple faces per photo
- Cache person search results

### 6. searchByLocation

**Purpose:** Search for photos taken near specific GPS coordinates.

**Input Schema:**
```typescript
interface SearchByLocationInput {
  // Location parameters
  latitude: number;     // -90 to 90
  longitude: number;    // -180 to 180
  radius: number;       // kilometers, 0.1-100

  // Alternative location search
  city?: string;
  state?: string;
  country?: string;
  address?: string;

  // Filters
  startDate?: string;
  endDate?: string;
  type?: "IMAGE" | "VIDEO" | "ALL";

  // Options
  includeNoLocation?: boolean; // Include photos without GPS
  clusterResults?: boolean;    // Group nearby photos

  // Pagination
  limit?: number;
  offset?: number;
}
```

**Output Schema:**
```typescript
interface SearchByLocationOutput {
  success: boolean;
  centerPoint: {
    latitude: number;
    longitude: number;
    address?: string;
  };
  searchRadius: number;
  totalCount: number;
  photos: LocationPhoto[];
  clusters?: LocationCluster[];
}

interface LocationPhoto extends Photo {
  distance: number; // kilometers from center
  bearing: number;  // degrees from north
  accuracy?: number; // GPS accuracy in meters
  altitude?: number;
  speed?: number;
}

interface LocationCluster {
  id: string;
  centerLatitude: number;
  centerLongitude: number;
  photoCount: number;
  thumbnailUrl: string;
  address?: string;
}
```

**Implementation Notes:**
- Use spatial indexing for performance
- Support reverse geocoding
- Cache geocoding results

### 7. createAlbum

**Purpose:** Create a new album with optional initial photos.

**Input Schema:**
```typescript
interface CreateAlbumInput {
  name: string;
  description?: string;

  // Initial content
  photoIds?: string[];

  // Sharing options
  shared?: boolean;
  sharedWithUserIds?: string[];

  // Album settings
  order?: "asc" | "desc";
  coverPhotoId?: string;
}
```

**Output Schema:**
```typescript
interface CreateAlbumOutput {
  success: boolean;
  album: AlbumInfo;
  addedPhotos: number;
  shareLink?: string;
}
```

### 8. updateAssetMetadata

**Purpose:** Update metadata for one or more assets.

**Input Schema:**
```typescript
interface UpdateAssetMetadataInput {
  assetIds: string[];
  updates: {
    description?: string;
    isFavorite?: boolean;
    isArchived?: boolean;
    tags?: string[];
    dateTimeOriginal?: string;
    location?: {
      latitude: number;
      longitude: number;
    };
  };

  // Options
  preserveExisting?: boolean; // Merge vs replace
  validateExif?: boolean;
}
```

**Output Schema:**
```typescript
interface UpdateAssetMetadataOutput {
  success: boolean;
  updatedCount: number;
  failedCount: number;
  failures?: UpdateFailure[];
}
```

### 9. analyzePhotos

**Purpose:** Perform AI analysis on photos for object detection, scene recognition, etc.

**Input Schema:**
```typescript
interface AnalyzePhotosInput {
  photoIds: string[];

  // Analysis types
  detectObjects?: boolean;
  detectFaces?: boolean;
  detectText?: boolean;
  generateTags?: boolean;
  assessQuality?: boolean;
  extractColors?: boolean;

  // Options
  forceReanalysis?: boolean;
  language?: string; // For text detection
}
```

**Output Schema:**
```typescript
interface AnalyzePhotosOutput {
  success: boolean;
  analyses: PhotoAnalysis[];
}

interface PhotoAnalysis {
  photoId: string;

  objects?: DetectedObject[];
  faces?: DetectedFace[];
  text?: ExtractedText[];
  tags?: GeneratedTag[];
  quality?: QualityAssessment;
  colors?: ColorPalette;

  processingTime: number;
}

interface DetectedObject {
  label: string;
  confidence: number;
  boundingBox?: BoundingBox;
}

interface QualityAssessment {
  overall: number;     // 0-100
  sharpness: number;
  exposure: number;
  composition: number;
  aesthetics: number;
}
```

### 10. exportPhotos

**Purpose:** Export photos in various formats for download or backup.

**Input Schema:**
```typescript
interface ExportPhotosInput {
  photoIds: string[];

  // Export options
  format: "original" | "jpeg" | "webp" | "zip";
  quality?: number; // 1-100 for lossy formats
  maxDimension?: number; // Resize to fit

  // Metadata options
  includeExif?: boolean;
  includeSidecar?: boolean;
  stripLocation?: boolean;

  // Organization
  folderStructure?: "flat" | "date" | "album";
  filenamePattern?: string; // e.g., "{date}_{original}"
}
```

**Output Schema:**
```typescript
interface ExportPhotosOutput {
  success: boolean;
  exportId: string;
  downloadUrl: string;
  expiresAt: string;
  totalSize: number;
  fileCount: number;
  format: string;
}
```

### 11. queryPhotosWithBuckets

**Purpose:** Query photos using Immich's bucket-based pagination for efficient browsing of large collections.

**Input Schema:**
```typescript
interface QueryPhotosWithBucketsInput {
  // Bucket configuration
  bucketSize: "day" | "month" | "year";
  startDate?: string; // ISO 8601
  endDate?: string;   // ISO 8601

  // Filters
  albumId?: string;
  personId?: string;
  isArchived?: boolean;
  isFavorite?: boolean;
  type?: "IMAGE" | "VIDEO" | "ALL";

  // Options
  withAssets?: boolean;    // Include asset details in buckets
  maxBuckets?: number;     // Limit number of buckets returned
}
```

**Output Schema:**
```typescript
interface QueryPhotosWithBucketsOutput {
  success: boolean;
  buckets: PhotoBucket[];
  totalCount: number;
  totalBuckets: number;
}

interface PhotoBucket {
  date: string;        // Bucket date (e.g., "2024-01-15")
  count: number;       // Number of assets in bucket
  assetIds: string[];  // Asset IDs in this bucket
  assets?: Photo[];    // Full asset details if requested
  startDate: string;   // Actual start of bucket period
  endDate: string;     // Actual end of bucket period
}
```

**Implementation Notes:**
- Use Immich's timeline bucket API for efficient pagination
- Buckets group photos by time period
- Supports lazy loading of asset details
- Ideal for timeline views and date-based browsing

### 12. findBrokenFiles

**Purpose:** Identify files with processing issues or corruption.

**Input Schema:**
```typescript
interface FindBrokenFilesInput {
  // Check type
  checkType: "missing_thumbnail" | "zero_size" | "processing_failed" | "all";

  // Scope
  libraryId?: string;     // Check specific library
  assetType?: "IMAGE" | "VIDEO" | "ALL";

  // Options
  includeDetails?: boolean; // Include extended error information
  limit?: number;          // Max results (1-1000, default: 100)
}
```

**Output Schema:**
```typescript
interface FindBrokenFilesOutput {
  success: boolean;
  brokenFiles: BrokenFile[];
  totalFound: number;
  scanStats: {
    totalScanned: number;
    scanDuration: number; // milliseconds
    librariesChecked: string[];
  };
}

interface BrokenFile {
  id: string;
  fileName: string;
  filePath: string;
  libraryId: string;
  fileSize: number;

  // Issue details
  issueType: "missing_thumbnail" | "zero_size" | "processing_failed";
  hasThumbnail: boolean;
  hasPreview: boolean;
  processingStatus: string;
  processingError?: string;

  // Metadata
  uploadedAt: string;
  modifiedAt: string;
  ownerId: string;

  // Suggested actions
  suggestedFix: "reprocess" | "reupload" | "delete" | "manual_review";
}
```

**Implementation Notes:**
- Query assets with extended metadata
- Check for missing thumbnails despite non-zero file size
- Identify zero-size files that should have content
- Detect assets stuck in processing
- Provide actionable fix suggestions

### 13. moveToLibrary

**Purpose:** Move assets between Immich libraries.

**Input Schema:**
```typescript
interface MoveToLibraryInput {
  assetIds: string[];
  targetLibraryId: string;

  // Options
  removeFromSource?: boolean;  // Remove from source library (default: true)
  skipDuplicates?: boolean;    // Skip if already in target (default: true)
  preserveAlbums?: boolean;    // Keep album associations (default: false)
}
```

**Output Schema:**
```typescript
interface MoveToLibraryOutput {
  success: boolean;
  moved: number;
  skipped: number;
  failed: number;

  details: {
    movedAssets: string[];
    skippedAssets: {
      assetId: string;
      reason: "duplicate" | "permission_denied" | "not_found";
    }[];
    failedAssets: {
      assetId: string;
      error: string;
    }[];
  };

  targetLibrary: {
    id: string;
    name: string;
    assetCount: number;
  };
}
```

**Implementation Notes:**
- Batch operations for efficiency
- Check permissions before moving
- Handle duplicates gracefully
- Maintain referential integrity

### 14. listLibraries

**Purpose:** List all available Immich libraries.

**Input Schema:**
```typescript
interface ListLibrariesInput {
  // Filter options
  type?: "UPLOAD" | "EXTERNAL" | "ALL";
  ownerId?: string;

  // Include options
  includeStats?: boolean;     // Include asset counts and size
  includePermissions?: boolean; // Include user permissions
}
```

**Output Schema:**
```typescript
interface ListLibrariesOutput {
  success: boolean;
  libraries: Library[];
  totalCount: number;
}

interface Library {
  id: string;
  name: string;
  type: "UPLOAD" | "EXTERNAL";
  ownerId: string;
  ownerName: string;

  // Paths (for external libraries)
  importPaths?: string[];
  exclusionPatterns?: string[];

  // Statistics
  stats?: {
    assetCount: number;
    totalSize: number;
    videoCount: number;
    imageCount: number;
    lastModified: string;
  };

  // Permissions
  permissions?: {
    canRead: boolean;
    canWrite: boolean;
    canDelete: boolean;
    canShare: boolean;
  };

  // Settings
  isWatched: boolean;  // Auto-import for external libraries
  isVisible: boolean;
  createdAt: string;
  updatedAt: string;
}
```

**Implementation Notes:**
- Enumerate all libraries accessible to user
- Include library type and configuration
- Calculate statistics on demand
- Check user permissions per library

### 15. repairAssets

**Purpose:** Attempt to repair broken or problematic assets.

**Input Schema:**
```typescript
interface RepairAssetsInput {
  assetIds: string[];

  // Repair actions
  actions: {
    regenerateThumbnails?: boolean;
    regeneratePreviews?: boolean;
    reextractMetadata?: boolean;
    rerunMachineLearning?: boolean;
    fixPermissions?: boolean;
  };

  // Options
  force?: boolean;  // Force regeneration even if exists
  priority?: "low" | "normal" | "high";
}
```

**Output Schema:**
```typescript
interface RepairAssetsOutput {
  success: boolean;
  jobId: string;

  summary: {
    total: number;
    queued: number;
    inProgress: number;
    completed: number;
    failed: number;
  };

  results?: {
    assetId: string;
    status: "queued" | "processing" | "completed" | "failed";
    actions: string[];
    error?: string;
  }[];

  estimatedTime?: number; // seconds
}
```

**Implementation Notes:**
- Queue repair jobs for background processing
- Support multiple repair actions per asset
- Provide progress tracking via job ID
- Handle large batches efficiently

## Transport and Streaming

The MCP SDK handles all transport-level concerns including streaming of large datasets. Tools return complete results and the SDK manages chunking and delivery based on the transport protocol (HTTP, stdio, etc).

## Error Handling

### Standard Error Response
```typescript
interface ToolError {
  success: false;
  error: {
    code: string;
    message: string;
    details?: any;
    retryable: boolean;
    retryAfter?: number; // seconds
  };
}
```

### Error Codes
- `INVALID_INPUT`: Invalid input parameters
- `NOT_FOUND`: Resource not found
- `PERMISSION_DENIED`: Insufficient permissions
- `RATE_LIMITED`: Rate limit exceeded
- `QUOTA_EXCEEDED`: Storage or API quota exceeded
- `IMMICH_ERROR`: Upstream Immich API error
- `TIMEOUT`: Operation timed out
- `INTERNAL_ERROR`: Unexpected server error

## Performance Requirements

### Response Time Targets
- Simple queries (getPhotoMetadata): < 100ms
- List operations (listAlbums): < 500ms
- Search operations (queryPhotos): < 2s for 1000 results
- Batch operations (moveToAlbum): < 5s for 100 items
- Analysis operations (analyzePhotos): < 30s per photo

### Concurrency Limits
- Maximum concurrent tool calls: 10
- Maximum items per batch operation: 1000
- Maximum search results: 10000
- Stream chunk size: 100 items

### Caching Strategy
- Cache TTL for metadata: 5 minutes
- Cache TTL for search results: 1 minute
- Cache TTL for album lists: 10 minutes
- LRU cache size: 1000 items