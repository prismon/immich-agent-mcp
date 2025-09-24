package immich

import "time"

// Asset represents an Immich asset
type Asset struct {
	ID               string    `json:"id"`
	DeviceAssetID    string    `json:"deviceAssetId"`
	OwnerID          string    `json:"ownerId"`
	DeviceID         string    `json:"deviceId"`
	LibraryID        string    `json:"libraryId,omitempty"`
	Type             string    `json:"type"` // IMAGE or VIDEO
	OriginalPath     string    `json:"originalPath"`
	OriginalFileName string    `json:"originalFileName"`
	Resized          bool      `json:"resized"`     // Has thumbnail
	Thumbhash        string    `json:"thumbhash,omitempty"`
	FileCreatedAt    time.Time `json:"fileCreatedAt"`
	FileModifiedAt   time.Time `json:"fileModifiedAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
	IsFavorite       bool      `json:"isFavorite"`
	IsArchived       bool      `json:"isArchived"`
	Duration         *string   `json:"duration,omitempty"`
	FileSize         int64     `json:"fileSizeInByte,omitempty"`
	Status           string    `json:"status,omitempty"`
	ExifInfo         *ExifInfo `json:"exifInfo,omitempty"`
	SmartInfo        *SmartInfo `json:"smartInfo,omitempty"`
}

// ExifInfo contains EXIF metadata
type ExifInfo struct {
	Make             string   `json:"make,omitempty"`
	Model            string   `json:"model,omitempty"`
	ExifImageWidth   int      `json:"exifImageWidth,omitempty"`
	ExifImageHeight  int      `json:"exifImageHeight,omitempty"`
	FileSizeInByte   int64    `json:"fileSizeInByte,omitempty"`
	Orientation      string   `json:"orientation,omitempty"`
	DateTimeOriginal string   `json:"dateTimeOriginal,omitempty"`
	Latitude         *float64 `json:"latitude,omitempty"`
	Longitude        *float64 `json:"longitude,omitempty"`
	City             string   `json:"city,omitempty"`
	State            string   `json:"state,omitempty"`
	Country          string   `json:"country,omitempty"`
	ISO              int      `json:"iso,omitempty"`
	ExposureTime     string   `json:"exposureTime,omitempty"`
	FNumber          float64  `json:"fNumber,omitempty"`
	LensModel        string   `json:"lensModel,omitempty"`
	FocalLength      float64  `json:"focalLength,omitempty"`
}

// SmartInfo contains AI-generated information
type SmartInfo struct {
	Tags    []string `json:"tags,omitempty"`
	Objects []string `json:"objects,omitempty"`
}

// Album represents an Immich album
type Album struct {
	ID                    string    `json:"id"`
	OwnerID               string    `json:"ownerId"`
	AlbumName             string    `json:"albumName"`
	Description           string    `json:"description,omitempty"`
	CreatedAt             time.Time `json:"createdAt"`
	UpdatedAt             time.Time `json:"updatedAt"`
	AlbumThumbnailAssetID string    `json:"albumThumbnailAssetId,omitempty"`
	Shared                bool      `json:"shared"`
	SharedUsers           []string  `json:"sharedUsers,omitempty"`
	HasSharedLink         bool      `json:"hasSharedLink"`
	AssetCount            int       `json:"assetCount"`
	Assets                []Asset   `json:"assets,omitempty"`
	Order                 string    `json:"order,omitempty"`
}

// Library represents an Immich library
type Library struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	Type              string    `json:"type"` // UPLOAD or EXTERNAL
	OwnerID           string    `json:"ownerId"`
	ImportPaths       []string  `json:"importPaths,omitempty"`
	ExclusionPatterns []string  `json:"exclusionPatterns,omitempty"`
	AssetCount        int       `json:"assetCount"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
	IsVisible         bool      `json:"isVisible"`
	IsWatched         bool      `json:"isWatched"`
}

// TimeBucket represents a time-based grouping of assets
type TimeBucket struct {
	Date     string `json:"timeBucket"`
	Count    int    `json:"count"`
	AssetIDs []string `json:"assetIds,omitempty"`
}

// PhotoResults represents search results
type PhotoResults struct {
	Total  int     `json:"total"`
	Count  int     `json:"count"`
	Photos []Asset `json:"items"`
}

// AssetPage represents a paginated page of assets
type AssetPage struct {
	Assets      []Asset `json:"assets"`
	Page        int     `json:"page"`
	PageSize    int     `json:"pageSize"`
	TotalCount  int     `json:"totalCount"`
	HasNextPage bool    `json:"hasNextPage"`
}

// BucketResults represents bucket query results
type BucketResults struct {
	Buckets      []TimeBucket `json:"buckets"`
	TotalBuckets int          `json:"totalBuckets"`
	TotalAssets  int          `json:"totalAssets,omitempty"`
}

// BrokenAsset represents an asset with issues
type BrokenAsset struct {
	ID              string `json:"id"`
	FileName        string `json:"fileName"`
	FilePath        string `json:"filePath"`
	LibraryID       string `json:"libraryId"`
	FileSize        int64  `json:"fileSize"`
	HasThumbnail    bool   `json:"hasThumbnail"`
	Status          string `json:"status,omitempty"`
	ProcessingError string `json:"processingError,omitempty"`
	IssueType       string `json:"issueType"`
	SuggestedFix    string `json:"suggestedFix"`
}

// BulkIDResult represents results from bulk operations
type BulkIDResult struct {
	Success []string `json:"success"`
	Error   []string `json:"error"`
}

// Request parameter types

// QueryPhotosParams parameters for photo queries
type QueryPhotosParams struct {
	Query       string
	StartDate   string
	EndDate     string
	AlbumID     string
	Type        string // IMAGE, VIDEO, ALL
	IsFavorite  bool
	IsArchived  bool
	Limit       int
	Offset      int
}

// BucketParams parameters for bucket queries
type BucketParams struct {
	Size       string // day, month, year
	AlbumID    string
	PersonID   string
	IsArchived bool
	IsFavorite bool
}

// CreateAlbumParams parameters for album creation
type CreateAlbumParams struct {
	Name        string
	Description string
	AssetIDs    []string
}

// FaceSearchParams parameters for face search
type FaceSearchParams struct {
	PersonID      string
	MinConfidence float64
	Limit         int
}

// LocationSearchParams parameters for location search
type LocationSearchParams struct {
	Latitude  float64
	Longitude float64
	Radius    float64 // kilometers
	Limit     int
}

// MoveToLibraryParams parameters for library moves
type MoveToLibraryParams struct {
	AssetIDs          []string
	TargetLibraryID   string
	RemoveFromSource  bool
	SkipDuplicates    bool
}

// MoveToLibraryResult result from library move
type MoveToLibraryResult struct {
	Success bool
	Moved   int
	Skipped int
	Failed  int
}

// AnalyzeOptions options for analysis
type AnalyzeOptions struct {
	DetectObjects bool
	DetectFaces   bool
	GenerateTags  bool
	AssessQuality bool
}

// AnalyzeResult result from analysis
type AnalyzeResult struct {
	Success   bool
	JobID     string
	Processed int
}

// RepairActions actions for repair
type RepairActions struct {
	RegenerateThumbnails bool
	RegeneratePreviews   bool
	ReextractMetadata    bool
	FixPermissions       bool
}

// RepairResult result from repair
type RepairResult struct {
	Success bool
	JobID   string
	Summary struct {
		Total      int
		Queued     int
		InProgress int
		Completed  int
		Failed     int
	}
}

// ExportResult result from export
type ExportResult struct {
	Success     bool
	ExportID    string
	DownloadURL string
	ExpiresAt   string
	TotalSize   int64
	FileCount   int
	Format      string
}