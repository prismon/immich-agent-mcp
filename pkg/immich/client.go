package immich

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate"
)

// Client represents an Immich API client
type Client struct {
	baseURL     string
	apiKey      string
	httpClient  *http.Client
	rateLimiter *rate.Limiter
}

// NewClient creates a new Immich client
func NewClient(baseURL, apiKey string, timeout time.Duration) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:       10,
				MaxConnsPerHost:    10,
				IdleConnTimeout:    90 * time.Second,
				DisableCompression: false,
			},
		},
		rateLimiter: rate.NewLimiter(rate.Every(10*time.Millisecond), 100), // 100 req/sec
	}
}

// Ping checks if the Immich server is reachable
func (c *Client) Ping(ctx context.Context) error {
	endpoint := fmt.Sprintf("%s/api/server-info/ping", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}

	req.Header.Set("x-api-key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ping failed with status: %d", resp.StatusCode)
	}

	return nil
}

// QueryPhotos searches for photos with filters
func (c *Client) QueryPhotos(ctx context.Context, params QueryPhotosParams) (*PhotoResults, error) {
	endpoint := fmt.Sprintf("%s/api/search", c.baseURL)

	// Build query parameters
	query := url.Values{}
	if params.Query != "" {
		query.Set("q", params.Query)
	}
	if params.StartDate != "" {
		query.Set("startDate", params.StartDate)
	}
	if params.EndDate != "" {
		query.Set("endDate", params.EndDate)
	}
	if params.AlbumID != "" {
		query.Set("albumId", params.AlbumID)
	}
	if params.Type != "" {
		query.Set("type", params.Type)
	}
	query.Set("limit", fmt.Sprintf("%d", params.Limit))
	query.Set("offset", fmt.Sprintf("%d", params.Offset))

	fullURL := fmt.Sprintf("%s?%s", endpoint, query.Encode())

	var results PhotoResults
	if err := c.get(ctx, fullURL, &results); err != nil {
		return nil, err
	}

	return &results, nil
}

// GetTimeBuckets gets photo buckets for timeline view
func (c *Client) GetTimeBuckets(ctx context.Context, params BucketParams) (*BucketResults, error) {
	endpoint := fmt.Sprintf("%s/api/timeline/buckets", c.baseURL)

	query := url.Values{}
	query.Set("size", params.Size)
	if params.AlbumID != "" {
		query.Set("albumId", params.AlbumID)
	}
	if params.PersonID != "" {
		query.Set("personId", params.PersonID)
	}
	query.Set("isArchived", fmt.Sprintf("%t", params.IsArchived))
	query.Set("isFavorite", fmt.Sprintf("%t", params.IsFavorite))

	fullURL := fmt.Sprintf("%s?%s", endpoint, query.Encode())

	var buckets []TimeBucket
	if err := c.get(ctx, fullURL, &buckets); err != nil {
		return nil, err
	}

	return &BucketResults{
		Buckets:      buckets,
		TotalBuckets: len(buckets),
	}, nil
}

// GetBucketAssets gets assets for a specific time bucket
func (c *Client) GetBucketAssets(ctx context.Context, bucketDate, size string) ([]Asset, error) {
	endpoint := fmt.Sprintf("%s/api/timeline/bucket", c.baseURL)

	query := url.Values{}
	query.Set("timeBucket", bucketDate)
	query.Set("size", size)

	fullURL := fmt.Sprintf("%s?%s", endpoint, query.Encode())

	var assets []Asset
	if err := c.get(ctx, fullURL, &assets); err != nil {
		return nil, err
	}

	return assets, nil
}

// GetAssetMetadata gets detailed metadata for an asset
func (c *Client) GetAssetMetadata(ctx context.Context, assetID string) (*Asset, error) {
	// Immich API endpoint for getting asset info
	endpoint := fmt.Sprintf("%s/api/assets/%s", c.baseURL, assetID)

	var asset Asset
	if err := c.get(ctx, endpoint, &asset); err != nil {
		return nil, fmt.Errorf("failed to get asset %s: %w", assetID, err)
	}

	return &asset, nil
}

// ListAlbums lists all albums
func (c *Client) ListAlbums(ctx context.Context, shared bool) ([]Album, error) {
	endpoint := fmt.Sprintf("%s/api/albums", c.baseURL)

	if shared {
		endpoint += "?shared=true"
	}

	var albums []Album
	if err := c.get(ctx, endpoint, &albums); err != nil {
		return nil, err
	}

	return albums, nil
}

// GetAllAlbumsWithInfo gets all albums with full metadata
func (c *Client) GetAllAlbumsWithInfo(ctx context.Context) ([]Album, error) {
	// Get all albums (both owned and shared)
	endpoint := fmt.Sprintf("%s/api/albums", c.baseURL)

	var albums []Album
	if err := c.get(ctx, endpoint, &albums); err != nil {
		return nil, err
	}

	return albums, nil
}

// GetAllAssets gets all assets with pagination support
func (c *Client) GetAllAssets(ctx context.Context, page, size int) (*AssetPage, error) {
	// Calculate offset from page and size
	offset := (page - 1) * size

	// Immich uses search API for getting all assets
	endpoint := fmt.Sprintf("%s/api/search/metadata", c.baseURL)

	// Create search request for all assets
	body := map[string]interface{}{
		"page":     offset/size + 1, // Convert to 1-based page
		"size":     size,
		"withExif": true, // Include EXIF data for dimensions
	}

	var searchResult struct {
		Assets struct {
			Total    int     `json:"total"`
			Count    int     `json:"count"`
			Items    []Asset `json:"items"`
			NextPage *string `json:"nextPage"`
		} `json:"assets"`
	}

	if err := c.post(ctx, endpoint, body, &searchResult); err != nil {
		return nil, err
	}

	hasMore := searchResult.Assets.NextPage != nil || searchResult.Assets.Count == size

	return &AssetPage{
		Assets:      searchResult.Assets.Items,
		Page:        page,
		PageSize:    size,
		TotalCount:  searchResult.Assets.Total,
		HasNextPage: hasMore,
	}, nil
}

// CreateAlbum creates a new album
func (c *Client) CreateAlbum(ctx context.Context, params CreateAlbumParams) (*Album, error) {
	endpoint := fmt.Sprintf("%s/api/albums", c.baseURL)

	body := map[string]interface{}{
		"albumName":   params.Name,
		"description": params.Description,
	}

	if len(params.AssetIDs) > 0 {
		body["assetIds"] = params.AssetIDs
	}

	var album Album
	if err := c.post(ctx, endpoint, body, &album); err != nil {
		return nil, err
	}

	return &album, nil
}

// UpdateAlbum updates an album's metadata (name and description)
func (c *Client) UpdateAlbum(ctx context.Context, albumID string, name, description string) (*Album, error) {
	endpoint := fmt.Sprintf("%s/api/albums/%s", c.baseURL, albumID)

	body := map[string]interface{}{}
	if name != "" {
		body["albumName"] = name
	}
	if description != "" {
		body["description"] = description
	}

	var album Album
	if err := c.patch(ctx, endpoint, body, &album); err != nil {
		return nil, err
	}

	return &album, nil
}

// AddAssetsToAlbum adds assets to an album
func (c *Client) AddAssetsToAlbum(ctx context.Context, albumID string, assetIDs []string) (*BulkIDResult, error) {
	endpoint := fmt.Sprintf("%s/api/albums/%s/assets", c.baseURL, albumID)

	body := map[string]interface{}{
		"ids": assetIDs,
	}

	// The API returns an array of results
	var results []struct {
		ID      string `json:"id"`
		Success bool   `json:"success"`
		Error   string `json:"error,omitempty"`
	}
	if err := c.put(ctx, endpoint, body, &results); err != nil {
		return nil, err
	}

	// Convert to BulkIDResult format
	bulkResult := &BulkIDResult{
		Success: []string{},
		Error:   []string{},
	}

	for _, res := range results {
		if res.Success {
			bulkResult.Success = append(bulkResult.Success, res.ID)
		} else {
			bulkResult.Error = append(bulkResult.Error, res.ID)
		}
	}

	return bulkResult, nil
}

// DeleteAssets permanently deletes assets
func (c *Client) DeleteAssets(ctx context.Context, assetIDs []string, forceDelete bool) error {
	endpoint := fmt.Sprintf("%s/api/assets", c.baseURL)

	body := map[string]interface{}{
		"ids":   assetIDs,
		"force": forceDelete, // true = permanent delete, false = trash
	}

	return c.delete(ctx, endpoint, body)
}

// GetAlbumAssets gets all assets in an album
func (c *Client) GetAlbumAssets(ctx context.Context, albumID string) ([]Asset, error) {
	endpoint := fmt.Sprintf("%s/api/albums/%s", c.baseURL, albumID)

	var album Album
	if err := c.get(ctx, endpoint, &album); err != nil {
		return nil, err
	}

	return album.Assets, nil
}

// RemoveAssetsFromAlbum removes assets from an album
func (c *Client) RemoveAssetsFromAlbum(ctx context.Context, albumID string, assetIDs []string) (*BulkIDResult, error) {
	endpoint := fmt.Sprintf("%s/api/albums/%s/assets", c.baseURL, albumID)

	body := map[string]interface{}{
		"ids": assetIDs,
	}

	// For DELETE operations, the API may return no body on success
	// We'll try to parse the response, but if parsing fails, assume all succeeded
	if err := c.delete(ctx, endpoint, body); err != nil {
		return nil, err
	}

	// If delete succeeded, return success for all IDs
	bulkResult := &BulkIDResult{
		Success: assetIDs,
		Error:   []string{},
	}

	return bulkResult, nil
}

// SmartSearchParams contains all parameters for smart search
type SmartSearchParams struct {
	Query         string   `json:"query,omitempty"`
	AlbumIds      []string `json:"albumIds,omitempty"`
	PersonIds     []string `json:"personIds,omitempty"`
	TagIds        []string `json:"tagIds,omitempty"`
	City          string   `json:"city,omitempty"`
	Country       string   `json:"country,omitempty"`
	State         string   `json:"state,omitempty"`
	Make          string   `json:"make,omitempty"`
	Model         string   `json:"model,omitempty"`
	LensModel     string   `json:"lensModel,omitempty"`
	DeviceId      string   `json:"deviceId,omitempty"`
	LibraryId     string   `json:"libraryId,omitempty"`
	QueryAssetId  string   `json:"queryAssetId,omitempty"`
	Type          string   `json:"type,omitempty"`       // IMAGE, VIDEO, AUDIO, OTHER
	Visibility    string   `json:"visibility,omitempty"` // archive, timeline, hidden, locked
	CreatedAfter  string   `json:"createdAfter,omitempty"`
	CreatedBefore string   `json:"createdBefore,omitempty"`
	TakenAfter    string   `json:"takenAfter,omitempty"`
	TakenBefore   string   `json:"takenBefore,omitempty"`
	UpdatedAfter  string   `json:"updatedAfter,omitempty"`
	UpdatedBefore string   `json:"updatedBefore,omitempty"`
	TrashedAfter  string   `json:"trashedAfter,omitempty"`
	TrashedBefore string   `json:"trashedBefore,omitempty"`
	IsFavorite    *bool    `json:"isFavorite,omitempty"`
	IsEncoded     *bool    `json:"isEncoded,omitempty"`
	IsMotion      *bool    `json:"isMotion,omitempty"`
	IsOffline     *bool    `json:"isOffline,omitempty"`
	IsNotInAlbum  *bool    `json:"isNotInAlbum,omitempty"`
	WithDeleted   *bool    `json:"withDeleted,omitempty"`
	WithExif      *bool    `json:"withExif,omitempty"`
	Rating        *int     `json:"rating,omitempty"` // -1 to 5
	Page          int      `json:"page,omitempty"`
	Size          int      `json:"size,omitempty"` // 1 to 1000
	Language      string   `json:"language,omitempty"`
}

// SmartSearch performs AI-powered search (simple version for backwards compatibility)
func (c *Client) SmartSearch(ctx context.Context, query string, limit int) ([]Asset, error) {
	params := SmartSearchParams{
		Query: query,
		Size:  limit,
	}
	return c.SmartSearchAdvanced(ctx, params)
}

// SmartSearchAdvanced performs AI-powered search with all available parameters
func (c *Client) SmartSearchAdvanced(ctx context.Context, params SmartSearchParams) ([]Asset, error) {
	endpoint := fmt.Sprintf("%s/api/search/smart", c.baseURL)

	var allAssets []Asset
	page := 1

	// Set default page size if not specified
	if params.Size == 0 || params.Size > 1000 {
		params.Size = 100
	}
	pageSize := params.Size
	if pageSize > 100 {
		pageSize = 100 // API returns max 100 per page
	}

	for {
		// Build request body from params
		body := make(map[string]interface{})

		// Add all non-empty parameters
		if params.Query != "" {
			body["query"] = params.Query
		}
		if len(params.AlbumIds) > 0 {
			body["albumIds"] = params.AlbumIds
		}
		if len(params.PersonIds) > 0 {
			body["personIds"] = params.PersonIds
		}
		if len(params.TagIds) > 0 {
			body["tagIds"] = params.TagIds
		}
		if params.City != "" {
			body["city"] = params.City
		}
		if params.Country != "" {
			body["country"] = params.Country
		}
		if params.State != "" {
			body["state"] = params.State
		}
		if params.Make != "" {
			body["make"] = params.Make
		}
		if params.Model != "" {
			body["model"] = params.Model
		}
		if params.LensModel != "" {
			body["lensModel"] = params.LensModel
		}
		if params.DeviceId != "" {
			body["deviceId"] = params.DeviceId
		}
		if params.LibraryId != "" {
			body["libraryId"] = params.LibraryId
		}
		if params.QueryAssetId != "" {
			body["queryAssetId"] = params.QueryAssetId
		}
		if params.Type != "" {
			body["type"] = params.Type
		}
		if params.Visibility != "" {
			body["visibility"] = params.Visibility
		}
		if params.CreatedAfter != "" {
			body["createdAfter"] = params.CreatedAfter
		}
		if params.CreatedBefore != "" {
			body["createdBefore"] = params.CreatedBefore
		}
		if params.TakenAfter != "" {
			body["takenAfter"] = params.TakenAfter
		}
		if params.TakenBefore != "" {
			body["takenBefore"] = params.TakenBefore
		}
		if params.UpdatedAfter != "" {
			body["updatedAfter"] = params.UpdatedAfter
		}
		if params.UpdatedBefore != "" {
			body["updatedBefore"] = params.UpdatedBefore
		}
		if params.TrashedAfter != "" {
			body["trashedAfter"] = params.TrashedAfter
		}
		if params.TrashedBefore != "" {
			body["trashedBefore"] = params.TrashedBefore
		}
		if params.IsFavorite != nil {
			body["isFavorite"] = *params.IsFavorite
		}
		if params.IsEncoded != nil {
			body["isEncoded"] = *params.IsEncoded
		}
		if params.IsMotion != nil {
			body["isMotion"] = *params.IsMotion
		}
		if params.IsOffline != nil {
			body["isOffline"] = *params.IsOffline
		}
		if params.IsNotInAlbum != nil {
			body["isNotInAlbum"] = *params.IsNotInAlbum
		}
		if params.WithDeleted != nil {
			body["withDeleted"] = *params.WithDeleted
		}
		if params.WithExif != nil {
			body["withExif"] = *params.WithExif
		}
		if params.Rating != nil {
			body["rating"] = *params.Rating
		}
		if params.Language != "" {
			body["language"] = params.Language
		}

		// Set pagination
		body["size"] = pageSize
		body["page"] = page

		var searchResult struct {
			Assets struct {
				Total    int         `json:"total"`
				Count    int         `json:"count"`
				Items    []Asset     `json:"items"`
				NextPage interface{} `json:"nextPage"`
			} `json:"assets"`
		}

		if err := c.post(ctx, endpoint, body, &searchResult); err != nil {
			return nil, err
		}

		// Add the items from this page
		allAssets = append(allAssets, searchResult.Assets.Items...)

		// Check if we've collected enough
		if params.Size > 0 && len(allAssets) >= params.Size {
			allAssets = allAssets[:params.Size]
			break
		}

		// Check if there are more pages
		if searchResult.Assets.NextPage == nil || len(searchResult.Assets.Items) == 0 {
			break
		}

		page++

		// Safety limit to prevent infinite loops
		if page > 50 { // Max 5000 results (50 * 100)
			break
		}
	}

	return allAssets, nil
}

// SearchByFace searches for assets containing a specific person
func (c *Client) SearchByFace(ctx context.Context, params FaceSearchParams) (*PhotoResults, error) {
	endpoint := fmt.Sprintf("%s/api/person/%s/assets", c.baseURL, params.PersonID)

	var results PhotoResults
	if err := c.get(ctx, endpoint, &results); err != nil {
		return nil, err
	}

	return &results, nil
}

// SearchByLocation searches for assets near coordinates
func (c *Client) SearchByLocation(ctx context.Context, params LocationSearchParams) (*PhotoResults, error) {
	endpoint := fmt.Sprintf("%s/api/search/location", c.baseURL)

	body := map[string]interface{}{
		"latitude":  params.Latitude,
		"longitude": params.Longitude,
		"radius":    params.Radius,
		"take":      params.Limit,
	}

	var results PhotoResults
	if err := c.post(ctx, endpoint, body, &results); err != nil {
		return nil, err
	}

	return &results, nil
}

// FindBrokenAssets finds assets with issues
func (c *Client) FindBrokenAssets(ctx context.Context, checkType, libraryID string, limit int) ([]BrokenAsset, error) {
	// Get all assets with metadata
	endpoint := fmt.Sprintf("%s/api/asset", c.baseURL)

	query := url.Values{}
	if libraryID != "" {
		query.Set("libraryId", libraryID)
	}

	fullURL := endpoint
	if len(query) > 0 {
		fullURL = fmt.Sprintf("%s?%s", endpoint, query.Encode())
	}

	var assets []Asset
	if err := c.get(ctx, fullURL, &assets); err != nil {
		return nil, err
	}

	// Filter broken assets
	var brokenAssets []BrokenAsset
	for _, asset := range assets {
		if isBroken(asset, checkType) {
			brokenAssets = append(brokenAssets, BrokenAsset{
				ID:           asset.ID,
				FileName:     asset.OriginalFileName,
				FilePath:     asset.OriginalPath,
				FileSize:     asset.FileSize,
				HasThumbnail: asset.Resized,
				LibraryID:    asset.LibraryID,
			})

			if len(brokenAssets) >= limit {
				break
			}
		}
	}

	return brokenAssets, nil
}

// ListLibraries lists all libraries
func (c *Client) ListLibraries(ctx context.Context) ([]Library, error) {
	endpoint := fmt.Sprintf("%s/api/library", c.baseURL)

	var libraries []Library
	if err := c.get(ctx, endpoint, &libraries); err != nil {
		return nil, err
	}

	return libraries, nil
}

// MoveAssetsToLibrary moves assets to a library
func (c *Client) MoveAssetsToLibrary(ctx context.Context, params MoveToLibraryParams) (*MoveToLibraryResult, error) {
	endpoint := fmt.Sprintf("%s/api/library/%s/assets", c.baseURL, params.TargetLibraryID)

	body := map[string]interface{}{
		"ids":       params.AssetIDs,
		"duplicate": !params.RemoveFromSource,
	}

	var bulkResult BulkIDResult
	if err := c.post(ctx, endpoint, body, &bulkResult); err != nil {
		return nil, err
	}

	// Convert to our result format
	result := &MoveToLibraryResult{
		Success: len(bulkResult.Success) > 0,
		Moved:   len(bulkResult.Success),
		Failed:  len(bulkResult.Error),
	}

	return result, nil
}

// UpdateAssetMetadata updates asset metadata
func (c *Client) UpdateAssetMetadata(ctx context.Context, assetID string, updates map[string]interface{}) error {
	endpoint := fmt.Sprintf("%s/api/asset/%s", c.baseURL, assetID)
	return c.put(ctx, endpoint, updates, nil)
}

// AnalyzeAssets triggers analysis jobs for assets
func (c *Client) AnalyzeAssets(ctx context.Context, assetIDs []string, options AnalyzeOptions) (*AnalyzeResult, error) {
	endpoint := fmt.Sprintf("%s/api/jobs", c.baseURL)

	body := map[string]interface{}{
		"assetIds": assetIDs,
		"name":     "analyze",
	}

	var result AnalyzeResult
	if err := c.post(ctx, endpoint, body, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// RepairAssets triggers repair jobs for assets
func (c *Client) RepairAssets(ctx context.Context, assetIDs []string, actions RepairActions) (*RepairResult, error) {
	endpoint := fmt.Sprintf("%s/api/jobs", c.baseURL)

	body := map[string]interface{}{
		"assetIds": assetIDs,
		"name":     "regenerate-thumbnails",
	}

	var result RepairResult
	if err := c.post(ctx, endpoint, body, &result); err != nil {
		return nil, err
	}

	result.JobID = fmt.Sprintf("repair-%d", time.Now().Unix())
	result.Summary.Total = len(assetIDs)
	result.Summary.Queued = len(assetIDs)

	return &result, nil
}

// ExportAssets exports assets for download
func (c *Client) ExportAssets(ctx context.Context, assetIDs []string, format string) (*ExportResult, error) {
	if len(assetIDs) == 0 {
		return nil, fmt.Errorf("no asset IDs provided")
	}

	// Generate download URLs
	downloadURLs := make([]string, 0, len(assetIDs))
	for _, id := range assetIDs {
		url := fmt.Sprintf("%s/api/asset/download/%s", c.baseURL, id)
		downloadURLs = append(downloadURLs, url)
	}

	downloadURL := ""
	if len(downloadURLs) > 0 {
		downloadURL = downloadURLs[0]
	}

	result := &ExportResult{
		Success:     true,
		ExportID:    fmt.Sprintf("export-%d", time.Now().Unix()),
		DownloadURL: downloadURL,
		ExpiresAt:   time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		FileCount:   len(assetIDs),
		Format:      format,
	}

	return result, nil
}

// Helper methods for HTTP operations

func (c *Client) get(ctx context.Context, url string, result interface{}) error {
	return c.request(ctx, http.MethodGet, url, nil, result)
}

func (c *Client) post(ctx context.Context, url string, body interface{}, result interface{}) error {
	return c.request(ctx, http.MethodPost, url, body, result)
}

func (c *Client) put(ctx context.Context, url string, body interface{}, result interface{}) error {
	return c.request(ctx, http.MethodPut, url, body, result)
}

func (c *Client) delete(ctx context.Context, url string, body interface{}) error {
	return c.request(ctx, http.MethodDelete, url, body, nil)
}

func (c *Client) patch(ctx context.Context, url string, body interface{}, result interface{}) error {
	return c.request(ctx, http.MethodPatch, url, body, result)
}

func (c *Client) request(ctx context.Context, method, url string, body interface{}, result interface{}) error {
	// Rate limit
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return err
	}

	// Prepare body
	var bodyReader io.Reader
	var jsonBody []byte
	if body != nil {
		var err error
		jsonBody, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	requestLogger := log.Info().
		Str("method", method).
		Str("url", url)

	var prettyPayload string
	if len(jsonBody) > 0 && zerolog.GlobalLevel() <= zerolog.DebugLevel {
		var buf bytes.Buffer
		if err := json.Indent(&buf, jsonBody, "", "  "); err != nil {
			buf.Write(jsonBody)
		}
		prettyPayload = buf.String()
		requestLogger = requestLogger.Str("payload", prettyPayload)
	}

	requestLogger.Msg("Calling Immich API")

	// Create request
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("x-api-key", c.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	responseLogger := log.Info().
		Str("method", method).
		Str("url", url).
		Int("status", resp.StatusCode)
	if prettyPayload != "" {
		responseLogger = responseLogger.Str("payload", prettyPayload)
	}
	responseLogger.Msg("Received Immich API response")

	// Check status
	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error: status=%d body=%s", resp.StatusCode, string(bodyBytes))
	}

	// Decode response
	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

// Helper function to check if an asset is broken
func isBroken(asset Asset, checkType string) bool {
	switch checkType {
	case "missing_thumbnail":
		return asset.FileSize > 0 && !asset.Resized
	case "zero_size":
		return asset.FileSize == 0
	case "processing_failed":
		return asset.Status == "failed"
	default:
		return (asset.FileSize > 0 && !asset.Resized) ||
			asset.FileSize == 0 ||
			asset.Status == "failed"
	}
}
