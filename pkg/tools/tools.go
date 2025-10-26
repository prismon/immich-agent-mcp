package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/patrickmn/go-cache"
	"github.com/yourusername/mcp-immich/pkg/immich"
)

// RegisterTools registers all tools with the MCP server
func RegisterTools(s *server.MCPServer, immichClient *immich.Client, cacheStore *cache.Cache) error {
	smartAlbumStore, err := NewSmartAlbumStore("")
	if err != nil {
		return err
	}

	// Query tools
	registerQueryPhotos(s, immichClient, cacheStore)
	registerQueryPhotosWithBuckets(s, immichClient, cacheStore)
	registerGetPhotoMetadata(s, immichClient, cacheStore)

	// Search tools
	registerSearchByFace(s, immichClient)
	registerSearchByLocation(s, immichClient)

	// Album tools
	registerListAlbums(s, immichClient, cacheStore)
	registerGetAllAlbums(s, immichClient, cacheStore)
	registerCreateAlbum(s, immichClient)
	registerMoveToAlbum(s, immichClient)
	registerDefineSmartAlbum(s, immichClient, smartAlbumStore)
	registerRefreshSmartAlbum(s, immichClient, smartAlbumStore)

	// Library tools
	registerListLibraries(s, immichClient, cacheStore)
	registerMoveToLibrary(s, immichClient)

	// Maintenance tools
	registerFindBrokenFiles(s, immichClient)
	registerRepairAssets(s, immichClient)
	registerMoveBrokenThumbnailsToAlbum(s, immichClient)
	registerMoveSmallImagesToAlbum(s, immichClient)
	registerMoveLargeMoviesToAlbum(s, immichClient)
	registerMovePersonalVideosFromAlbum(s, immichClient)
	registerMovePhotosBySearch(s, immichClient)
	registerSmartSearchAdvanced(s, immichClient)
	registerDeleteAlbumContents(s, immichClient)

	// Asset management tools
	registerUpdateAssetMetadata(s, immichClient)
	registerAnalyzePhotos(s, immichClient)
	registerExportPhotos(s, immichClient)
	registerGetAllAssets(s, immichClient, cacheStore)

	return nil
}

// queryPhotos tool
func registerQueryPhotos(s *server.MCPServer, immichClient *immich.Client, cacheStore *cache.Cache) {
	tool := mcp.Tool{
		Name:        "queryPhotos",
		Description: "Search and filter photos in Immich",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"query":     map[string]interface{}{"type": "string", "description": "Search query"},
				"startDate": map[string]interface{}{"type": "string", "format": "date-time"},
				"endDate":   map[string]interface{}{"type": "string", "format": "date-time"},
				"albumId":   map[string]interface{}{"type": "string"},
				"type":      map[string]interface{}{"type": "string", "enum": []string{"IMAGE", "VIDEO", "ALL"}},
				"limit":     map[string]interface{}{"type": "integer", "minimum": 1, "maximum": 1000, "default": 100},
			},
		},
	}

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var params struct {
			Query     string `json:"query"`
			StartDate string `json:"startDate"`
			EndDate   string `json:"endDate"`
			AlbumID   string `json:"albumId"`
			Type      string `json:"type"`
			Limit     int    `json:"limit"`
		}

		argBytes, ok := request.Params.Arguments.([]byte)
		if !ok {
			// Try to marshal if it's already a structured type
			argBytes, _ = json.Marshal(request.Params.Arguments)
		}
		if err := json.Unmarshal(argBytes, &params); err != nil {
			return nil, fmt.Errorf("invalid parameters: %w", err)
		}

		// Set defaults
		if params.Limit == 0 {
			params.Limit = 100
		}

		// Check cache
		cacheKey := fmt.Sprintf("%v", request.Params.Arguments)
		if cached, found := cacheStore.Get(cacheKey); found {
			return makeMCPResult(cached)
		}

		// Query Immich
		results, err := immichClient.QueryPhotos(ctx, immich.QueryPhotosParams{
			Query:     params.Query,
			StartDate: params.StartDate,
			EndDate:   params.EndDate,
			AlbumID:   params.AlbumID,
			Type:      params.Type,
			Limit:     params.Limit,
		})

		if err != nil {
			return nil, err
		}

		// Cache results
		cacheStore.Set(cacheKey, results, cache.DefaultExpiration)

		return makeMCPResult(map[string]interface{}{
			"success":    true,
			"totalCount": results.Total,
			"photos":     results.Photos,
		})
	}

	s.AddTool(tool, handler)
}

// queryPhotosWithBuckets tool
func registerQueryPhotosWithBuckets(s *server.MCPServer, immichClient *immich.Client, cacheStore *cache.Cache) {
	tool := mcp.Tool{
		Name:        "queryPhotosWithBuckets",
		Description: "Query photos using Immich's bucket-based pagination for timeline views",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"bucketSize": map[string]interface{}{"type": "string", "enum": []string{"day", "month", "year"}},
				"startDate":  map[string]interface{}{"type": "string", "format": "date-time"},
				"endDate":    map[string]interface{}{"type": "string", "format": "date-time"},
				"albumId":    map[string]interface{}{"type": "string"},
				"personId":   map[string]interface{}{"type": "string"},
				"isArchived": map[string]interface{}{"type": "boolean"},
				"isFavorite": map[string]interface{}{"type": "boolean"},
				"withAssets": map[string]interface{}{"type": "boolean"},
				"maxBuckets": map[string]interface{}{"type": "integer"},
			},
		},
	}

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var params struct {
			BucketSize string `json:"bucketSize"`
			AlbumID    string `json:"albumId"`
			PersonID   string `json:"personId"`
			IsArchived bool   `json:"isArchived"`
			IsFavorite bool   `json:"isFavorite"`
			WithAssets bool   `json:"withAssets"`
			MaxBuckets int    `json:"maxBuckets"`
		}

		argBytes, ok := request.Params.Arguments.([]byte)
		if !ok {
			// Try to marshal if it's already a structured type
			argBytes, _ = json.Marshal(request.Params.Arguments)
		}
		if err := json.Unmarshal(argBytes, &params); err != nil {
			return nil, fmt.Errorf("invalid parameters: %w", err)
		}

		// Get buckets
		results, err := immichClient.GetTimeBuckets(ctx, immich.BucketParams{
			Size:       params.BucketSize,
			AlbumID:    params.AlbumID,
			PersonID:   params.PersonID,
			IsArchived: params.IsArchived,
			IsFavorite: params.IsFavorite,
		})

		if err != nil {
			return nil, err
		}

		// Optionally load assets for each bucket
		if params.WithAssets && len(results.Buckets) > 0 {
			limit := len(results.Buckets)
			if params.MaxBuckets > 0 && params.MaxBuckets < limit {
				limit = params.MaxBuckets
			}

			for i := 0; i < limit; i++ {
				assets, err := immichClient.GetBucketAssets(ctx, results.Buckets[i].Date, params.BucketSize)
				if err == nil {
					// Store assets in a separate field (not AssetIDs which contains IDs)
					// This would need to extend the TimeBucket type
					_ = assets // For now, just fetch them
				}
			}
		}

		return makeMCPResult(map[string]interface{}{
			"success":      true,
			"buckets":      results.Buckets,
			"totalBuckets": results.TotalBuckets,
		})
	}

	s.AddTool(tool, handler)
}

// Additional tool implementations...
func registerGetPhotoMetadata(s *server.MCPServer, immichClient *immich.Client, cacheStore *cache.Cache) {
	tool := mcp.Tool{
		Name:        "getPhotoMetadata",
		Description: "Retrieve detailed metadata for a specific photo",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"photoId":       map[string]interface{}{"type": "string"},
				"includeExif":   map[string]interface{}{"type": "boolean", "default": true},
				"includeFaces":  map[string]interface{}{"type": "boolean", "default": true},
				"includeAlbums": map[string]interface{}{"type": "boolean", "default": true},
			},
			Required: []string{"photoId"},
		},
	}

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var params struct {
			PhotoID string `json:"photoId"`
		}

		argBytes, ok := request.Params.Arguments.([]byte)
		if !ok {
			// Try to marshal if it's already a structured type
			argBytes, _ = json.Marshal(request.Params.Arguments)
		}
		if err := json.Unmarshal(argBytes, &params); err != nil {
			return nil, fmt.Errorf("invalid parameters: %w", err)
		}

		asset, err := immichClient.GetAssetMetadata(ctx, params.PhotoID)
		if err != nil {
			return nil, err
		}

		return makeMCPResult(map[string]interface{}{
			"success": true,
			"photo":   asset,
		})
	}

	s.AddTool(tool, handler)
}

// Stub implementations for remaining tools
func registerSearchByFace(s *server.MCPServer, immichClient *immich.Client) {
	// Implementation similar to above
}

func registerSearchByLocation(s *server.MCPServer, immichClient *immich.Client) {
	// Implementation similar to above
}

func registerListAlbums(s *server.MCPServer, immichClient *immich.Client, cacheStore *cache.Cache) {
	tool := mcp.Tool{
		Name:        "listAlbums",
		Description: "List all albums (basic info only)",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"shared": map[string]interface{}{"type": "boolean", "default": false},
			},
		},
	}

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var params struct {
			Shared bool `json:"shared"`
		}

		argBytes, ok := request.Params.Arguments.([]byte)
		if !ok {
			argBytes, _ = json.Marshal(request.Params.Arguments)
		}
		if err := json.Unmarshal(argBytes, &params); err != nil {
			return nil, fmt.Errorf("invalid parameters: %w", err)
		}

		albums, err := immichClient.ListAlbums(ctx, params.Shared)
		if err != nil {
			return nil, err
		}

		return makeMCPResult(map[string]interface{}{
			"success": true,
			"albums":  albums,
			"count":   len(albums),
		})
	}

	s.AddTool(tool, handler)
}

func registerGetAllAlbums(s *server.MCPServer, immichClient *immich.Client, cacheStore *cache.Cache) {
	tool := mcp.Tool{
		Name:        "getAllAlbums",
		Description: "Get all albums with complete metadata including asset counts, thumbnails, and sharing info",
		InputSchema: mcp.ToolInputSchema{
			Type:       "object",
			Properties: map[string]interface{}{},
		},
	}

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Use cache for this potentially expensive operation
		cacheKey := "getAllAlbums"
		if cached, found := cacheStore.Get(cacheKey); found {
			return makeMCPResult(cached)
		}

		albums, err := immichClient.GetAllAlbumsWithInfo(ctx)
		if err != nil {
			return nil, err
		}

		result := map[string]interface{}{
			"success":     true,
			"albums":      albums,
			"totalAlbums": len(albums),
		}

		// Cache for 1 minute
		cacheStore.Set(cacheKey, result, 1*time.Minute)

		return makeMCPResult(result)
	}

	s.AddTool(tool, handler)
}

func registerCreateAlbum(s *server.MCPServer, immichClient *immich.Client) {
	// Implementation similar to above
}

func registerMoveToAlbum(s *server.MCPServer, immichClient *immich.Client) {
	tool := mcp.Tool{
		Name:        "moveAssetsToAlbum",
		Description: "Move specified assets to an album",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"assetIds": map[string]interface{}{
					"type":        "array",
					"description": "List of asset IDs to move",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
				"albumName": map[string]interface{}{
					"type":        "string",
					"description": "Name of the album to move assets to",
				},
				"createAlbum": map[string]interface{}{
					"type":        "boolean",
					"description": "Create album if it doesn't exist",
					"default":     false,
				},
				"albumDescription": map[string]interface{}{
					"type":        "string",
					"description": "Description for the album if creating new",
					"default":     "",
				},
			},
			Required: []string{"assetIds", "albumName"},
		},
	}

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var params struct {
			AssetIds         []string `json:"assetIds"`
			AlbumName        string   `json:"albumName"`
			CreateAlbum      bool     `json:"createAlbum"`
			AlbumDescription string   `json:"albumDescription"`
		}

		argBytes, ok := request.Params.Arguments.([]byte)
		if !ok {
			argBytes, _ = json.Marshal(request.Params.Arguments)
		}
		if err := json.Unmarshal(argBytes, &params); err != nil {
			return nil, fmt.Errorf("invalid parameters: %w", err)
		}

		if len(params.AssetIds) == 0 {
			return makeMCPResult(map[string]interface{}{
				"success": false,
				"message": "No asset IDs provided",
			})
		}

		// Find existing album
		var albumID string
		var albumFound bool
		albums, err := immichClient.ListAlbums(ctx, false)
		if err != nil {
			return nil, fmt.Errorf("failed to list albums: %w", err)
		}

		for _, album := range albums {
			if album.AlbumName == params.AlbumName {
				albumID = album.ID
				albumFound = true
				break
			}
		}

		// Create album if needed
		if !albumFound {
			if !params.CreateAlbum {
				return nil, fmt.Errorf("album '%s' not found and createAlbum is false", params.AlbumName)
			}

			newAlbum, err := immichClient.CreateAlbum(ctx, immich.CreateAlbumParams{
				Name:        params.AlbumName,
				Description: params.AlbumDescription,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to create album: %w", err)
			}
			albumID = newAlbum.ID
		}

		// Add assets to album
		bulkResult, err := immichClient.AddAssetsToAlbum(ctx, albumID, params.AssetIds)
		if err != nil {
			return nil, fmt.Errorf("failed to add assets to album: %w", err)
		}

		result := map[string]interface{}{
			"success":      true,
			"albumID":      albumID,
			"albumName":    params.AlbumName,
			"albumCreated": !albumFound,
			"movedCount":   len(bulkResult.Success),
			"failedCount":  len(bulkResult.Error),
		}

		if len(bulkResult.Error) > 0 {
			result["failedAssets"] = bulkResult.Error
		}

		return makeMCPResult(result)
	}

	s.AddTool(tool, handler)
}

func registerDefineSmartAlbum(s *server.MCPServer, immichClient *immich.Client, store *SmartAlbumStore) {
	tool := mcp.Tool{
		Name:        "defineSmartAlbum",
		Description: "Create or update a smart album definition backed by a stored smart search query",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"smartAlbumId": map[string]interface{}{
					"type":        "string",
					"description": "Identifier of an existing smart album definition to update",
				},
				"smartAlbumName": map[string]interface{}{
					"type":        "string",
					"description": "Name for this smart album definition (used when referencing by name)",
				},
				"description": map[string]interface{}{
					"type":        "string",
					"description": "Optional description for the smart album definition",
				},
				"albumId": map[string]interface{}{
					"type":        "string",
					"description": "Existing Immich album ID that should receive assets",
				},
				"albumName": map[string]interface{}{
					"type":        "string",
					"description": "Album name to target or create if albumId is not provided",
				},
				"albumDescription": map[string]interface{}{
					"type":        "string",
					"description": "Description to apply if a new album is created",
				},
				"createAlbum": map[string]interface{}{
					"type":        "boolean",
					"description": "Create the album when it does not already exist",
					"default":     true,
				},
				"smartQuery": map[string]interface{}{
					"type":        "string",
					"description": "Smart search free-form query text",
				},
				"searchParams": map[string]interface{}{
					"type":        "object",
					"description": "Advanced smart search parameters mirroring Immich's /api/search/smart payload",
				},
				"maxResults": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of results to fetch when refreshing (1-5000)",
					"minimum":     1,
					"maximum":     5000,
					"default":     500,
				},
			},
		},
	}

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var params struct {
			SmartAlbumID     string                 `json:"smartAlbumId"`
			SmartAlbumName   string                 `json:"smartAlbumName"`
			Description      string                 `json:"description"`
			AlbumID          string                 `json:"albumId"`
			AlbumName        string                 `json:"albumName"`
			AlbumDescription string                 `json:"albumDescription"`
			CreateAlbum      bool                   `json:"createAlbum"`
			SmartQuery       string                 `json:"smartQuery"`
			SearchParams     map[string]interface{} `json:"searchParams"`
			MaxResults       int                    `json:"maxResults"`
		}

		params.CreateAlbum = true

		argBytes, ok := request.Params.Arguments.([]byte)
		if !ok {
			argBytes, _ = json.Marshal(request.Params.Arguments)
		}
		if err := json.Unmarshal(argBytes, &params); err != nil {
			return nil, fmt.Errorf("invalid parameters: %w", err)
		}

		var existing SmartAlbumDefinition
		var exists bool
		if params.SmartAlbumID != "" {
			existing, exists = store.GetByID(params.SmartAlbumID)
			if !exists {
				return nil, fmt.Errorf("smart album with id %s not found", params.SmartAlbumID)
			}
		} else {
			existing, exists = store.GetByName(params.SmartAlbumName)
		}

		if params.SmartAlbumID == "" && params.SmartAlbumName == "" {
			return nil, fmt.Errorf("either smartAlbumId or smartAlbumName must be provided")
		}

		smartAlbumName := params.SmartAlbumName
		if smartAlbumName == "" {
			smartAlbumName = existing.Name
		}
		if smartAlbumName == "" {
			return nil, fmt.Errorf("smartAlbumName is required")
		}

		var searchParams immich.SmartSearchParams
		providedParams := false
		if len(params.SearchParams) > 0 {
			payload, err := json.Marshal(params.SearchParams)
			if err != nil {
				return nil, fmt.Errorf("invalid searchParams: %w", err)
			}
			if err := json.Unmarshal(payload, &searchParams); err != nil {
				return nil, fmt.Errorf("invalid searchParams: %w", err)
			}
			providedParams = true
		}
		if exists && !providedParams {
			searchParams = existing.Query
		}

		if params.SmartQuery != "" {
			searchParams.Query = params.SmartQuery
		}

		maxResults := params.MaxResults
		if maxResults <= 0 {
			if exists && existing.MaxResults > 0 {
				maxResults = existing.MaxResults
			} else {
				maxResults = 500
			}
		}
		if maxResults > 5000 {
			maxResults = 5000
		}
		searchParams.Size = maxResults

		if !exists && searchParams.Query == "" && !providedParams {
			return nil, fmt.Errorf("either smartQuery or searchParams must be provided for new smart albums")
		}

		resolvedAlbumID := params.AlbumID
		resolvedAlbumName := ""
		resolvedAlbumDescription := ""
		albumCreated := false

		if resolvedAlbumID != "" {
			album, err := findAlbumByID(ctx, immichClient, resolvedAlbumID)
			if err != nil {
				return nil, err
			}
			if album == nil {
				return nil, fmt.Errorf("album with id %s not found", resolvedAlbumID)
			}
			resolvedAlbumName = album.AlbumName
			resolvedAlbumDescription = album.Description
		} else if params.AlbumName != "" {
			album, err := findAlbumByName(ctx, immichClient, params.AlbumName)
			if err != nil {
				return nil, err
			}
			if album == nil {
				if !params.CreateAlbum {
					return nil, fmt.Errorf("album '%s' not found and createAlbum is false", params.AlbumName)
				}

				createdAlbum, err := immichClient.CreateAlbum(ctx, immich.CreateAlbumParams{
					Name:        params.AlbumName,
					Description: params.AlbumDescription,
				})
				if err != nil {
					return nil, fmt.Errorf("failed to create album '%s': %w", params.AlbumName, err)
				}

				resolvedAlbumID = createdAlbum.ID
				resolvedAlbumName = createdAlbum.AlbumName
				resolvedAlbumDescription = createdAlbum.Description
				albumCreated = true
			} else {
				resolvedAlbumID = album.ID
				resolvedAlbumName = album.AlbumName
				resolvedAlbumDescription = album.Description
			}
		} else if exists {
			resolvedAlbumID = existing.AlbumID
			resolvedAlbumName = existing.AlbumName
			resolvedAlbumDescription = existing.AlbumDescription
		}

		if resolvedAlbumID == "" {
			return nil, fmt.Errorf("an albumId or albumName must be provided to link the smart album")
		}

		def := SmartAlbumDefinition{
			ID:               existing.ID,
			Name:             smartAlbumName,
			Description:      params.Description,
			AlbumID:          resolvedAlbumID,
			AlbumName:        resolvedAlbumName,
			AlbumDescription: resolvedAlbumDescription,
			Query:            searchParams,
			MaxResults:       maxResults,
			CreatedAt:        existing.CreatedAt,
			LastRunAt:        existing.LastRunAt,
			LastResultCount:  existing.LastResultCount,
			LastAddedCount:   existing.LastAddedCount,
			LastRunError:     existing.LastRunError,
		}

		if params.Description == "" && exists {
			def.Description = existing.Description
		}

		saved, err := store.Save(def)
		if err != nil {
			return nil, fmt.Errorf("failed to persist smart album: %w", err)
		}

		return makeMCPResult(map[string]interface{}{
			"success": true,
			"smartAlbum": map[string]interface{}{
				"id":          saved.ID,
				"name":        saved.Name,
				"description": saved.Description,
				"albumId":     saved.AlbumID,
				"albumName":   saved.AlbumName,
				"maxResults":  saved.MaxResults,
				"query":       saved.Query,
				"createdAt":   saved.CreatedAt,
				"updatedAt":   saved.UpdatedAt,
			},
			"albumCreated": albumCreated,
		})
	}

	s.AddTool(tool, handler)
}

func registerRefreshSmartAlbum(s *server.MCPServer, immichClient *immich.Client, store *SmartAlbumStore) {
	tool := mcp.Tool{
		Name:        "refreshSmartAlbum",
		Description: "Run a stored smart search definition and sync new results into its destination album",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"smartAlbumId": map[string]interface{}{
					"type":        "string",
					"description": "Identifier of the smart album definition to refresh",
				},
				"smartAlbumName": map[string]interface{}{
					"type":        "string",
					"description": "Name of the smart album definition to refresh when id is not provided",
				},
				"dryRun": map[string]interface{}{
					"type":        "boolean",
					"description": "When true, compute differences without modifying the album",
					"default":     false,
				},
				"maxResults": map[string]interface{}{
					"type":        "integer",
					"description": "Override the stored maximum number of search matches (1-5000)",
					"minimum":     1,
					"maximum":     5000,
				},
				"previewLimit": map[string]interface{}{
					"type":        "integer",
					"description": "Limit for the number of asset IDs returned as a preview",
					"minimum":     1,
					"maximum":     200,
					"default":     25,
				},
			},
		},
	}

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var params struct {
			SmartAlbumID   string `json:"smartAlbumId"`
			SmartAlbumName string `json:"smartAlbumName"`
			DryRun         bool   `json:"dryRun"`
			MaxResults     int    `json:"maxResults"`
			PreviewLimit   int    `json:"previewLimit"`
		}

		params.PreviewLimit = 25

		argBytes, ok := request.Params.Arguments.([]byte)
		if !ok {
			argBytes, _ = json.Marshal(request.Params.Arguments)
		}
		if err := json.Unmarshal(argBytes, &params); err != nil {
			return nil, fmt.Errorf("invalid parameters: %w", err)
		}

		if params.SmartAlbumID == "" && params.SmartAlbumName == "" {
			return nil, fmt.Errorf("either smartAlbumId or smartAlbumName must be provided")
		}

		if params.PreviewLimit < 0 {
			params.PreviewLimit = 0
		} else if params.PreviewLimit > 200 {
			params.PreviewLimit = 200
		}

		def, err := resolveSmartAlbumDefinition(store, params.SmartAlbumID, params.SmartAlbumName)
		if err != nil {
			return nil, err
		}

		effectiveParams := def.Query
		if params.MaxResults > 0 {
			if params.MaxResults > 5000 {
				params.MaxResults = 5000
			}
			effectiveParams.Size = params.MaxResults
		} else if effectiveParams.Size == 0 {
			if def.MaxResults > 0 {
				effectiveParams.Size = def.MaxResults
			} else {
				effectiveParams.Size = 500
			}
		}

		now := time.Now().UTC()

		assets, searchErr := immichClient.SmartSearchAdvanced(ctx, effectiveParams)
		if searchErr != nil {
			def.LastRunAt = &now
			def.LastRunError = searchErr.Error()
			def.LastAddedCount = 0
			def.LastResultCount = 0
			if _, saveErr := store.Save(def); saveErr != nil {
				return nil, fmt.Errorf("smart search failed: %v (additionally failed to persist state: %w)", searchErr, saveErr)
			}
			return nil, searchErr
		}

		existingAssets, err := immichClient.GetAlbumAssets(ctx, def.AlbumID)
		if err != nil {
			def.LastRunAt = &now
			def.LastRunError = err.Error()
			def.LastAddedCount = 0
			def.LastResultCount = 0
			if _, saveErr := store.Save(def); saveErr != nil {
				return nil, fmt.Errorf("failed to read existing album assets: %v (additionally failed to persist state: %w)", err, saveErr)
			}
			return nil, fmt.Errorf("failed to read existing album assets: %w", err)
		}

		existingMap := make(map[string]struct{}, len(existingAssets))
		for _, asset := range existingAssets {
			existingMap[asset.ID] = struct{}{}
		}

		newIDs := make([]string, 0)
		skipped := 0
		for _, asset := range assets {
			if _, found := existingMap[asset.ID]; found {
				skipped++
				continue
			}
			newIDs = append(newIDs, asset.ID)
		}

		result := map[string]interface{}{
			"success":         true,
			"smartAlbumId":    def.ID,
			"smartAlbumName":  def.Name,
			"albumId":         def.AlbumID,
			"albumName":       def.AlbumName,
			"dryRun":          params.DryRun,
			"totalMatches":    len(assets),
			"alreadyInAlbum":  skipped,
			"potentialAdds":   len(newIDs),
			"previewAssetIds": []string{},
		}

		previewIDs := newIDs
		if params.PreviewLimit > 0 && len(previewIDs) > params.PreviewLimit {
			previewIDs = previewIDs[:params.PreviewLimit]
		}
		if len(previewIDs) > 0 {
			result["previewAssetIds"] = previewIDs
		}

		def.LastRunAt = &now
		def.LastResultCount = len(assets)
		def.LastRunError = ""

		if params.DryRun || len(newIDs) == 0 {
			def.LastAddedCount = 0
			if _, err := store.Save(def); err != nil {
				return nil, fmt.Errorf("failed to persist smart album refresh metadata: %w", err)
			}
			if params.DryRun {
				result["note"] = "dry run - no assets added"
			}
			return makeMCPResult(result)
		}

		bulkResult, err := immichClient.AddAssetsToAlbum(ctx, def.AlbumID, newIDs)
		if err != nil {
			def.LastAddedCount = 0
			def.LastRunError = err.Error()
			if _, saveErr := store.Save(def); saveErr != nil {
				return nil, fmt.Errorf("failed to add assets: %v (additionally failed to persist state: %w)", err, saveErr)
			}
			return nil, fmt.Errorf("failed to add assets to album: %w", err)
		}

		addedIDs := bulkResult.Success
		failedIDs := bulkResult.Error

		sort.Strings(addedIDs)
		sort.Strings(failedIDs)

		def.LastAddedCount = len(addedIDs)
		if len(failedIDs) > 0 {
			def.LastRunError = fmt.Sprintf("%d assets failed to add", len(failedIDs))
		}

		savedDef, err := store.Save(def)
		if err != nil {
			return nil, fmt.Errorf("failed to persist smart album refresh results: %w", err)
		}

		result["addedCount"] = len(addedIDs)
		result["addedAssetIds"] = addedIDs
		if len(failedIDs) > 0 {
			result["failedAssetIds"] = failedIDs
		}
		result["smartAlbum"] = map[string]interface{}{
			"id":              savedDef.ID,
			"name":            savedDef.Name,
			"lastRunAt":       savedDef.LastRunAt,
			"lastResultCount": savedDef.LastResultCount,
			"lastAddedCount":  savedDef.LastAddedCount,
			"lastRunError":    savedDef.LastRunError,
		}

		return makeMCPResult(result)
	}

	s.AddTool(tool, handler)
}

func resolveSmartAlbumDefinition(store *SmartAlbumStore, id, name string) (SmartAlbumDefinition, error) {
	if id != "" {
		if def, ok := store.GetByID(id); ok {
			return def, nil
		}
		return SmartAlbumDefinition{}, fmt.Errorf("smart album with id %s not found", id)
	}

	if def, ok := store.GetByName(name); ok {
		return def, nil
	}
	return SmartAlbumDefinition{}, fmt.Errorf("smart album named '%s' not found", name)
}

func findAlbumByID(ctx context.Context, client *immich.Client, albumID string) (*immich.Album, error) {
	albums, err := client.GetAllAlbumsWithInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list albums: %w", err)
	}
	for _, album := range albums {
		if album.ID == albumID {
			return &album, nil
		}
	}
	return nil, nil
}

func findAlbumByName(ctx context.Context, client *immich.Client, name string) (*immich.Album, error) {
	albums, err := client.GetAllAlbumsWithInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list albums: %w", err)
	}
	for _, album := range albums {
		if strings.EqualFold(album.AlbumName, name) {
			return &album, nil
		}
	}
	return nil, nil
}

func registerListLibraries(s *server.MCPServer, immichClient *immich.Client, cacheStore *cache.Cache) {
	// Implementation similar to above
}

func registerMoveToLibrary(s *server.MCPServer, immichClient *immich.Client) {
	// Implementation similar to above
}

func registerFindBrokenFiles(s *server.MCPServer, immichClient *immich.Client) {
	// Implementation similar to above
}

func registerRepairAssets(s *server.MCPServer, immichClient *immich.Client) {
	// Implementation similar to above
}

func registerUpdateAssetMetadata(s *server.MCPServer, immichClient *immich.Client) {
	// Implementation similar to above
}

func registerAnalyzePhotos(s *server.MCPServer, immichClient *immich.Client) {
	// Implementation similar to above
}

func registerExportPhotos(s *server.MCPServer, immichClient *immich.Client) {
	// Implementation similar to above
}

func registerGetAllAssets(s *server.MCPServer, immichClient *immich.Client, cacheStore *cache.Cache) {
	tool := mcp.Tool{
		Name:        "getAllAssets",
		Description: "Get all assets with pagination support. Walk through all images in the library, page by page.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"page": map[string]interface{}{
					"type":        "integer",
					"description": "Page number (1-based)",
					"minimum":     1,
					"default":     1,
				},
				"pageSize": map[string]interface{}{
					"type":        "integer",
					"description": "Number of assets per page",
					"minimum":     1,
					"maximum":     1000,
					"default":     50,
				},
			},
		},
	}

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var params struct {
			Page     int `json:"page"`
			PageSize int `json:"pageSize"`
		}

		// Set defaults
		params.Page = 1
		params.PageSize = 50

		argBytes, ok := request.Params.Arguments.([]byte)
		if !ok {
			argBytes, _ = json.Marshal(request.Params.Arguments)
		}
		if err := json.Unmarshal(argBytes, &params); err != nil {
			return nil, fmt.Errorf("invalid parameters: %w", err)
		}

		// Validate parameters
		if params.Page < 1 {
			params.Page = 1
		}
		if params.PageSize < 1 {
			params.PageSize = 50
		}
		if params.PageSize > 1000 {
			params.PageSize = 1000
		}

		// Check cache for this specific page
		cacheKey := fmt.Sprintf("getAllAssets:page:%d:size:%d", params.Page, params.PageSize)
		if cached, found := cacheStore.Get(cacheKey); found {
			return makeMCPResult(cached)
		}

		assetPage, err := immichClient.GetAllAssets(ctx, params.Page, params.PageSize)
		if err != nil {
			return nil, err
		}

		result := map[string]interface{}{
			"success":     true,
			"assets":      assetPage.Assets,
			"page":        assetPage.Page,
			"pageSize":    assetPage.PageSize,
			"assetCount":  len(assetPage.Assets),
			"hasNextPage": assetPage.HasNextPage,
			"totalCount":  assetPage.TotalCount,
		}

		// Cache for 30 seconds (shorter than albums since data changes more frequently)
		cacheStore.Set(cacheKey, result, 30*time.Second)

		return makeMCPResult(result)
	}

	s.AddTool(tool, handler)
}

// registerMoveBrokenThumbnailsToAlbum registers the tool for moving images with no thumbhash
func registerMoveBrokenThumbnailsToAlbum(s *server.MCPServer, immichClient *immich.Client) {
	tool := mcp.Tool{
		Name:        "moveBrokenThumbnailsToAlbum",
		Description: "Find all images with no thumbhash (broken thumbnails) and move them to a specified album",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"albumName": map[string]interface{}{
					"type":        "string",
					"description": "Name of the album to move broken images to",
				},
				"createAlbum": map[string]interface{}{
					"type":        "boolean",
					"description": "Create album if it doesn't exist",
					"default":     true,
				},
				"dryRun": map[string]interface{}{
					"type":        "boolean",
					"description": "Just find broken images without moving them",
					"default":     false,
				},
				"maxImages": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of images to process (0 for unlimited)",
					"default":     1000,
				},
				"startPage": map[string]interface{}{
					"type":        "integer",
					"description": "Starting page number for pagination",
					"default":     1,
				},
			},
			Required: []string{"albumName"},
		},
	}

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var params struct {
			AlbumName   string `json:"albumName"`
			CreateAlbum bool   `json:"createAlbum"`
			DryRun      bool   `json:"dryRun"`
			MaxImages   int    `json:"maxImages"`
			StartPage   int    `json:"startPage"`
		}

		// Set defaults
		params.CreateAlbum = true
		params.MaxImages = 1000
		params.StartPage = 1

		argBytes, ok := request.Params.Arguments.([]byte)
		if !ok {
			argBytes, _ = json.Marshal(request.Params.Arguments)
		}
		if err := json.Unmarshal(argBytes, &params); err != nil {
			return nil, fmt.Errorf("invalid parameters: %w", err)
		}

		// Find images with no thumbhash
		brokenImages := []immich.Asset{}
		page := params.StartPage
		pageSize := 1000 // Increased for efficiency
		totalProcessed := 0

		for params.MaxImages == 0 || len(brokenImages) < params.MaxImages {
			// Check for context cancellation
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("operation cancelled: %w", ctx.Err())
			default:
			}

			assetPage, err := immichClient.GetAllAssets(ctx, page, pageSize)
			if err != nil {
				return nil, fmt.Errorf("failed to get assets page %d: %w", page, err)
			}

			totalProcessed += len(assetPage.Assets)

			for _, asset := range assetPage.Assets {
				// Simple check: IMAGE type with no thumbhash
				if asset.Type == "IMAGE" && asset.Thumbhash == "" {
					brokenImages = append(brokenImages, asset)
					if params.MaxImages > 0 && len(brokenImages) >= params.MaxImages {
						break
					}
				}
			}

			if !assetPage.HasNextPage {
				break
			}
			page++
		}

		result := map[string]interface{}{
			"foundBrokenImages": len(brokenImages),
			"totalProcessed":    totalProcessed,
			"lastPage":          page,
		}

		// Include first few broken images in dry run for inspection
		if params.DryRun {
			sampleSize := 5
			if len(brokenImages) < sampleSize {
				sampleSize = len(brokenImages)
			}
			result["sampleBrokenImages"] = brokenImages[:sampleSize]
			result["dryRun"] = true
			result["message"] = fmt.Sprintf("Dry run: found %d images with no thumbhash", len(brokenImages))
			return makeMCPResult(result)
		}

		if len(brokenImages) == 0 {
			result["message"] = "No broken thumbnail images found"
			result["success"] = true
			return makeMCPResult(result)
		}

		// Find or create album
		var albumID string
		var albumFound bool
		albums, err := immichClient.ListAlbums(ctx, false)
		if err != nil {
			return nil, fmt.Errorf("failed to list albums: %w", err)
		}

		for _, album := range albums {
			if album.AlbumName == params.AlbumName {
				albumID = album.ID
				albumFound = true
				break
			}
		}

		if !albumFound {
			if !params.CreateAlbum {
				return nil, fmt.Errorf("album '%s' not found and createAlbum is false", params.AlbumName)
			}

			newAlbum, err := immichClient.CreateAlbum(ctx, immich.CreateAlbumParams{
				Name:        params.AlbumName,
				Description: "Album for images with broken thumbnails (no thumbhash)",
			})
			if err != nil {
				return nil, fmt.Errorf("failed to create album: %w", err)
			}
			albumID = newAlbum.ID
			result["albumCreated"] = true
		} else {
			result["albumCreated"] = false
		}

		// Move images to album
		assetIDs := make([]string, len(brokenImages))
		for i, img := range brokenImages {
			assetIDs[i] = img.ID
		}

		bulkResult, err := immichClient.AddAssetsToAlbum(ctx, albumID, assetIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to add assets to album: %w", err)
		}

		result["movedCount"] = len(bulkResult.Success)
		result["failedCount"] = len(bulkResult.Error)
		result["albumID"] = albumID
		result["albumName"] = params.AlbumName
		result["success"] = true

		return makeMCPResult(result)
	}

	s.AddTool(tool, handler)
}

// registerMoveSmallImagesToAlbum registers the tool for moving small images
func registerMoveSmallImagesToAlbum(s *server.MCPServer, immichClient *immich.Client) {
	tool := mcp.Tool{
		Name:        "moveSmallImagesToAlbum",
		Description: "Find all images 400x400 pixels or smaller and move them to a 'Small Images' album",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"albumName": map[string]interface{}{
					"type":        "string",
					"description": "Name of the album for small images",
					"default":     "Small Images",
				},
				"maxDimension": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum width or height in pixels to be considered small",
					"default":     400,
				},
				"createAlbum": map[string]interface{}{
					"type":        "boolean",
					"description": "Create album if it doesn't exist",
					"default":     true,
				},
				"dryRun": map[string]interface{}{
					"type":        "boolean",
					"description": "Just find small images without moving them",
					"default":     false,
				},
				"maxImages": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of images to process",
					"default":     1000,
				},
			},
		},
	}

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var params struct {
			AlbumName    string `json:"albumName"`
			MaxDimension int    `json:"maxDimension"`
			CreateAlbum  bool   `json:"createAlbum"`
			DryRun       bool   `json:"dryRun"`
			MaxImages    int    `json:"maxImages"`
			StartPage    int    `json:"startPage"`
		}

		// Set defaults
		params.AlbumName = "Small Images"
		params.MaxDimension = 400
		params.CreateAlbum = true
		params.MaxImages = 1000
		params.StartPage = 1

		argBytes, ok := request.Params.Arguments.([]byte)
		if !ok {
			argBytes, _ = json.Marshal(request.Params.Arguments)
		}
		if err := json.Unmarshal(argBytes, &params); err != nil {
			return nil, fmt.Errorf("invalid parameters: %w", err)
		}

		// Find small images
		smallImages := []immich.Asset{}
		page := params.StartPage
		pageSize := 1000 // Increased for efficiency
		totalProcessed := 0

		for params.MaxImages == 0 || len(smallImages) < params.MaxImages {
			// Check for context cancellation
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("operation cancelled: %w", ctx.Err())
			default:
			}

			assetPage, err := immichClient.GetAllAssets(ctx, page, pageSize)
			if err != nil {
				return nil, fmt.Errorf("failed to get assets page %d: %w", page, err)
			}

			totalProcessed += len(assetPage.Assets)

			for _, asset := range assetPage.Assets {
				// Check if image is small
				if asset.Type == "IMAGE" && asset.ExifInfo != nil {
					width := asset.ExifInfo.ExifImageWidth
					height := asset.ExifInfo.ExifImageHeight

					// Check if both dimensions are <= maxDimension (and > 0)
					if width > 0 && height > 0 && width <= params.MaxDimension && height <= params.MaxDimension {
						smallImages = append(smallImages, asset)
						if params.MaxImages > 0 && len(smallImages) >= params.MaxImages {
							break
						}
					}
				}
			}

			if !assetPage.HasNextPage {
				break
			}
			page++
		}

		result := map[string]interface{}{
			"foundSmallImages": len(smallImages),
			"maxDimension":     params.MaxDimension,
			"totalProcessed":   totalProcessed,
			"lastPage":         page,
		}

		// Include sample in dry run
		if params.DryRun {
			sampleSize := 5
			if len(smallImages) < sampleSize {
				sampleSize = len(smallImages)
			}

			sampleData := []map[string]interface{}{}
			for i := 0; i < sampleSize; i++ {
				img := smallImages[i]
				sampleData = append(sampleData, map[string]interface{}{
					"id":     img.ID,
					"name":   img.OriginalFileName,
					"width":  img.ExifInfo.ExifImageWidth,
					"height": img.ExifInfo.ExifImageHeight,
				})
			}

			result["sampleSmallImages"] = sampleData
			result["dryRun"] = true
			result["message"] = fmt.Sprintf("Dry run: found %d images <= %dx%d pixels", len(smallImages), params.MaxDimension, params.MaxDimension)
			return makeMCPResult(result)
		}

		if len(smallImages) == 0 {
			result["message"] = fmt.Sprintf("No images smaller than %dx%d found", params.MaxDimension, params.MaxDimension)
			result["success"] = true
			return makeMCPResult(result)
		}

		// Find or create album
		var albumID string
		var albumFound bool
		albums, err := immichClient.ListAlbums(ctx, false)
		if err != nil {
			return nil, fmt.Errorf("failed to list albums: %w", err)
		}

		for _, album := range albums {
			if album.AlbumName == params.AlbumName {
				albumID = album.ID
				albumFound = true
				break
			}
		}

		if !albumFound {
			if !params.CreateAlbum {
				return nil, fmt.Errorf("album '%s' not found and createAlbum is false", params.AlbumName)
			}

			newAlbum, err := immichClient.CreateAlbum(ctx, immich.CreateAlbumParams{
				Name:        params.AlbumName,
				Description: fmt.Sprintf("Album for small images (%dx%d or smaller)", params.MaxDimension, params.MaxDimension),
			})
			if err != nil {
				return nil, fmt.Errorf("failed to create album: %w", err)
			}
			albumID = newAlbum.ID
			result["albumCreated"] = true
		} else {
			result["albumCreated"] = false
		}

		// Move images to album
		assetIDs := make([]string, len(smallImages))
		for i, img := range smallImages {
			assetIDs[i] = img.ID
		}

		bulkResult, err := immichClient.AddAssetsToAlbum(ctx, albumID, assetIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to add assets to album: %w", err)
		}

		result["movedCount"] = len(bulkResult.Success)
		result["failedCount"] = len(bulkResult.Error)
		result["albumID"] = albumID
		result["albumName"] = params.AlbumName
		result["success"] = true

		return makeMCPResult(result)
	}

	s.AddTool(tool, handler)
}

// registerMoveLargeMoviesToAlbum registers the tool for moving large movies
func registerMoveLargeMoviesToAlbum(s *server.MCPServer, immichClient *immich.Client) {
	tool := mcp.Tool{
		Name:        "moveLargeMoviesToAlbum",
		Description: "Find all movies over 20 minutes and move them to a 'Large Movies' album",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"albumName": map[string]interface{}{
					"type":        "string",
					"description": "Name of the album for large movies",
					"default":     "Large Movies",
				},
				"minDuration": map[string]interface{}{
					"type":        "integer",
					"description": "Minimum duration in minutes to be considered large",
					"default":     20,
				},
				"createAlbum": map[string]interface{}{
					"type":        "boolean",
					"description": "Create album if it doesn't exist",
					"default":     true,
				},
				"dryRun": map[string]interface{}{
					"type":        "boolean",
					"description": "Just find large movies without moving them",
					"default":     false,
				},
				"maxVideos": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of videos to process (0 for unlimited)",
					"default":     1000,
				},
				"startPage": map[string]interface{}{
					"type":        "integer",
					"description": "Starting page number for pagination",
					"default":     1,
				},
			},
		},
	}

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var params struct {
			AlbumName   string `json:"albumName"`
			MinDuration int    `json:"minDuration"`
			CreateAlbum bool   `json:"createAlbum"`
			DryRun      bool   `json:"dryRun"`
			MaxVideos   int    `json:"maxVideos"`
			StartPage   int    `json:"startPage"`
		}

		// Set defaults
		params.AlbumName = "Large Movies"
		params.MinDuration = 20
		params.CreateAlbum = true
		params.MaxVideos = 1000
		params.StartPage = 1

		argBytes, ok := request.Params.Arguments.([]byte)
		if !ok {
			argBytes, _ = json.Marshal(request.Params.Arguments)
		}
		if err := json.Unmarshal(argBytes, &params); err != nil {
			return nil, fmt.Errorf("invalid parameters: %w", err)
		}

		// Convert minimum duration to seconds
		minDurationSec := params.MinDuration * 60

		// Find large movies
		largeMovies := []immich.Asset{}
		page := params.StartPage
		pageSize := 1000
		totalProcessed := 0

		for params.MaxVideos == 0 || len(largeMovies) < params.MaxVideos {
			// Check for context cancellation
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("operation cancelled: %w", ctx.Err())
			default:
			}

			assetPage, err := immichClient.GetAllAssets(ctx, page, pageSize)
			if err != nil {
				return nil, fmt.Errorf("failed to get assets page %d: %w", page, err)
			}

			totalProcessed += len(assetPage.Assets)

			for _, asset := range assetPage.Assets {
				// Check if it's a video with duration
				if asset.Type == "VIDEO" && asset.Duration != nil {
					// Parse duration string (format: "H:MM:SS.mmmmm")
					durationSec := parseDuration(*asset.Duration)
					if durationSec >= minDurationSec {
						largeMovies = append(largeMovies, asset)
						if params.MaxVideos > 0 && len(largeMovies) >= params.MaxVideos {
							break
						}
					}
				}
			}

			if !assetPage.HasNextPage {
				break
			}
			page++
		}

		result := map[string]interface{}{
			"foundLargeMovies": len(largeMovies),
			"minDuration":      params.MinDuration,
			"totalProcessed":   totalProcessed,
			"lastPage":         page,
		}

		// Include sample in dry run
		if params.DryRun {
			sampleSize := 5
			if len(largeMovies) < sampleSize {
				sampleSize = len(largeMovies)
			}

			sampleData := []map[string]interface{}{}
			for i := 0; i < sampleSize; i++ {
				movie := largeMovies[i]
				durationMin := 0
				if movie.Duration != nil {
					durationMin = parseDuration(*movie.Duration) / 60
				}
				sampleData = append(sampleData, map[string]interface{}{
					"id":       movie.ID,
					"name":     movie.OriginalFileName,
					"duration": *movie.Duration,
					"minutes":  durationMin,
				})
			}

			result["sampleLargeMovies"] = sampleData
			result["dryRun"] = true
			result["message"] = fmt.Sprintf("Dry run: found %d movies over %d minutes", len(largeMovies), params.MinDuration)
			result["success"] = true
			return makeMCPResult(result)
		}

		if len(largeMovies) == 0 {
			result["message"] = fmt.Sprintf("No movies over %d minutes found", params.MinDuration)
			result["success"] = true
			return makeMCPResult(result)
		}

		// Find or create album
		var albumID string
		var albumFound bool
		albums, err := immichClient.ListAlbums(ctx, false)
		if err != nil {
			return nil, fmt.Errorf("failed to list albums: %w", err)
		}

		for _, album := range albums {
			if album.AlbumName == params.AlbumName {
				albumID = album.ID
				albumFound = true
				break
			}
		}

		if !albumFound {
			if !params.CreateAlbum {
				return nil, fmt.Errorf("album '%s' not found and createAlbum is false", params.AlbumName)
			}

			newAlbum, err := immichClient.CreateAlbum(ctx, immich.CreateAlbumParams{
				Name:        params.AlbumName,
				Description: fmt.Sprintf("Movies over %d minutes", params.MinDuration),
			})
			if err != nil {
				return nil, fmt.Errorf("failed to create album: %w", err)
			}
			albumID = newAlbum.ID
			result["albumCreated"] = true
		} else {
			result["albumCreated"] = false
		}

		// Move movies to album
		movieIDs := make([]string, len(largeMovies))
		for i, movie := range largeMovies {
			movieIDs[i] = movie.ID
		}

		bulkResult, err := immichClient.AddAssetsToAlbum(ctx, albumID, movieIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to add movies to album: %w", err)
		}

		result["movedCount"] = len(bulkResult.Success)
		result["failedCount"] = len(bulkResult.Error)
		result["albumID"] = albumID
		result["albumName"] = params.AlbumName
		result["success"] = true

		return makeMCPResult(result)
	}

	s.AddTool(tool, handler)
}

// registerMovePersonalVideosFromAlbum registers tool to separate personal videos from movies
func registerMovePersonalVideosFromAlbum(s *server.MCPServer, immichClient *immich.Client) {
	tool := mcp.Tool{
		Name:        "movePersonalVideosFromAlbum",
		Description: "Move personal videos from an album (like Large Movies) to a Personal Videos album",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"sourceAlbum": map[string]interface{}{
					"type":        "string",
					"description": "Source album to move videos from",
					"default":     "Large Movies",
				},
				"targetAlbum": map[string]interface{}{
					"type":        "string",
					"description": "Target album for personal videos",
					"default":     "Personal Videos",
				},
				"patterns": map[string]interface{}{
					"type":        "array",
					"description": "Filename patterns to identify personal videos",
					"items":       map[string]interface{}{"type": "string"},
					"default":     []string{"^\\d{8}_", "^IMG_", "^VID_", "^MOV_", "^DSC", "^DSCN", "^GOPR", "^DJI_"},
				},
				"createAlbum": map[string]interface{}{
					"type":        "boolean",
					"description": "Create target album if it doesn't exist",
					"default":     true,
				},
				"dryRun": map[string]interface{}{
					"type":        "boolean",
					"description": "Just identify personal videos without moving them",
					"default":     false,
				},
				"removeFromSource": map[string]interface{}{
					"type":        "boolean",
					"description": "Remove videos from source album after moving",
					"default":     true,
				},
			},
			Required: []string{},
		},
	}

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var params struct {
			SourceAlbum      string   `json:"sourceAlbum"`
			TargetAlbum      string   `json:"targetAlbum"`
			Patterns         []string `json:"patterns"`
			CreateAlbum      bool     `json:"createAlbum"`
			DryRun           bool     `json:"dryRun"`
			RemoveFromSource bool     `json:"removeFromSource"`
		}

		// Set defaults
		params.SourceAlbum = "Large Movies"
		params.TargetAlbum = "Personal Videos"
		params.Patterns = []string{
			"^\\d{8}_",              // Date format: 20160525_
			"^\\d{4}-\\d{2}-\\d{2}", // Date format: 2024-01-15
			"^IMG_",                 // iPhone/camera format
			"^VID_",                 // Video format
			"^MOV_",                 // Movie format
			"^DSC",                  // Digital camera
			"^DSCN",                 // Nikon
			"^GOPR",                 // GoPro
			"^DJI_",                 // DJI drone
			"^PXL_",                 // Pixel phone
			"^FILE",                 // Generic file
			"\\.MOV$",               // MOV extension (personal videos)
			"\\.mov$",               // mov extension
		}
		params.CreateAlbum = true
		params.RemoveFromSource = true

		argBytes, ok := request.Params.Arguments.([]byte)
		if !ok {
			argBytes, _ = json.Marshal(request.Params.Arguments)
		}
		if err := json.Unmarshal(argBytes, &params); err != nil {
			return nil, fmt.Errorf("invalid parameters: %w", err)
		}

		// Find source album
		var sourceAlbumID string
		albums, err := immichClient.ListAlbums(ctx, false)
		if err != nil {
			return nil, fmt.Errorf("failed to list albums: %w", err)
		}

		for _, album := range albums {
			if album.AlbumName == params.SourceAlbum {
				sourceAlbumID = album.ID
				break
			}
		}

		if sourceAlbumID == "" {
			return nil, fmt.Errorf("source album '%s' not found", params.SourceAlbum)
		}

		// Get assets from source album
		sourceAssets, err := immichClient.GetAlbumAssets(ctx, sourceAlbumID)
		if err != nil {
			return nil, fmt.Errorf("failed to get assets from source album: %w", err)
		}

		// Identify personal videos
		personalVideos := []immich.Asset{}
		for _, asset := range sourceAssets {
			if asset.Type == "VIDEO" {
				// Check if filename matches any personal video pattern
				for _, pattern := range params.Patterns {
					matched, _ := regexp.MatchString(pattern, asset.OriginalFileName)
					if matched {
						personalVideos = append(personalVideos, asset)
						break
					}
				}
			}
		}

		result := map[string]interface{}{
			"sourceAlbum":         params.SourceAlbum,
			"targetAlbum":         params.TargetAlbum,
			"totalVideosInSource": len(sourceAssets),
			"personalVideosFound": len(personalVideos),
		}

		// Include sample in dry run
		if params.DryRun {
			sampleSize := 10
			if len(personalVideos) < sampleSize {
				sampleSize = len(personalVideos)
			}

			sampleData := []map[string]interface{}{}
			for i := 0; i < sampleSize; i++ {
				video := personalVideos[i]
				durationStr := ""
				if video.Duration != nil {
					durationStr = *video.Duration
				}
				sampleData = append(sampleData, map[string]interface{}{
					"id":       video.ID,
					"name":     video.OriginalFileName,
					"duration": durationStr,
				})
			}

			result["samplePersonalVideos"] = sampleData
			result["dryRun"] = true
			result["message"] = fmt.Sprintf("Dry run: found %d personal videos to move", len(personalVideos))
			result["success"] = true
			return makeMCPResult(result)
		}

		if len(personalVideos) == 0 {
			result["message"] = "No personal videos found in source album"
			result["success"] = true
			return makeMCPResult(result)
		}

		// Find or create target album
		var targetAlbumID string
		var targetAlbumFound bool

		for _, album := range albums {
			if album.AlbumName == params.TargetAlbum {
				targetAlbumID = album.ID
				targetAlbumFound = true
				break
			}
		}

		if !targetAlbumFound {
			if !params.CreateAlbum {
				return nil, fmt.Errorf("target album '%s' not found and createAlbum is false", params.TargetAlbum)
			}

			newAlbum, err := immichClient.CreateAlbum(ctx, immich.CreateAlbumParams{
				Name:        params.TargetAlbum,
				Description: "Personal videos from phones, cameras, and other devices",
			})
			if err != nil {
				return nil, fmt.Errorf("failed to create target album: %w", err)
			}
			targetAlbumID = newAlbum.ID
			result["targetAlbumCreated"] = true
		} else {
			result["targetAlbumCreated"] = false
		}

		// Move videos to target album
		videoIDs := make([]string, len(personalVideos))
		for i, video := range personalVideos {
			videoIDs[i] = video.ID
		}

		bulkResult, err := immichClient.AddAssetsToAlbum(ctx, targetAlbumID, videoIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to add videos to target album: %w", err)
		}

		result["movedCount"] = len(bulkResult.Success)
		result["failedCount"] = len(bulkResult.Error)

		// Remove from source album if requested
		if params.RemoveFromSource && len(bulkResult.Success) > 0 {
			removeResult, err := immichClient.RemoveAssetsFromAlbum(ctx, sourceAlbumID, bulkResult.Success)
			if err != nil {
				result["removeError"] = fmt.Sprintf("failed to remove from source: %v", err)
			} else {
				result["removedFromSource"] = len(removeResult.Success)
			}
		}

		result["targetAlbumID"] = targetAlbumID
		result["success"] = true
		result["message"] = fmt.Sprintf("Moved %d personal videos from %s to %s",
			len(bulkResult.Success), params.SourceAlbum, params.TargetAlbum)

		return makeMCPResult(result)
	}

	s.AddTool(tool, handler)
}

// registerDeleteAlbumContents registers the tool for deleting all assets from an album
func registerDeleteAlbumContents(s *server.MCPServer, immichClient *immich.Client) {
	tool := mcp.Tool{
		Name:        "deleteAlbumContents",
		Description: "Delete all assets from an album and remove them from the timeline",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"albumName": map[string]interface{}{
					"type":        "string",
					"description": "Name of the album to delete contents from",
				},
				"albumId": map[string]interface{}{
					"type":        "string",
					"description": "ID of the album (if known, otherwise will search by name)",
				},
				"forceDelete": map[string]interface{}{
					"type":        "boolean",
					"description": "Permanently delete (true) or move to trash (false)",
					"default":     false,
				},
				"dryRun": map[string]interface{}{
					"type":        "boolean",
					"description": "Just count assets without deleting them",
					"default":     false,
				},
				"batchSize": map[string]interface{}{
					"type":        "integer",
					"description": "Number of assets to delete in each batch",
					"default":     100,
				},
				"maxAssets": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of assets to delete (0 for all)",
					"default":     0,
				},
			},
			Required: []string{},
		},
	}

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var params struct {
			AlbumName   string `json:"albumName"`
			AlbumID     string `json:"albumId"`
			ForceDelete bool   `json:"forceDelete"`
			DryRun      bool   `json:"dryRun"`
			BatchSize   int    `json:"batchSize"`
			MaxAssets   int    `json:"maxAssets"`
		}

		// Set defaults
		params.BatchSize = 100

		argBytes, ok := request.Params.Arguments.([]byte)
		if !ok {
			argBytes, _ = json.Marshal(request.Params.Arguments)
		}
		if err := json.Unmarshal(argBytes, &params); err != nil {
			return nil, fmt.Errorf("invalid parameters: %w", err)
		}

		// Find album if not provided by ID
		var albumID string
		var albumName string

		if params.AlbumID != "" {
			albumID = params.AlbumID
			albumName = params.AlbumName // May be empty
		} else if params.AlbumName != "" {
			// Search for album by name
			albums, err := immichClient.ListAlbums(ctx, false)
			if err != nil {
				return nil, fmt.Errorf("failed to list albums: %w", err)
			}

			for _, album := range albums {
				if album.AlbumName == params.AlbumName {
					albumID = album.ID
					albumName = album.AlbumName
					break
				}
			}

			if albumID == "" {
				return nil, fmt.Errorf("album '%s' not found", params.AlbumName)
			}
		} else {
			return nil, fmt.Errorf("either albumName or albumId must be provided")
		}

		// Get all assets in the album
		assets, err := immichClient.GetAlbumAssets(ctx, albumID)
		if err != nil {
			return nil, fmt.Errorf("failed to get album assets: %w", err)
		}

		if len(assets) == 0 {
			return makeMCPResult(map[string]interface{}{
				"success":    true,
				"albumID":    albumID,
				"albumName":  albumName,
				"assetCount": 0,
				"message":    "Album is empty, nothing to delete",
			})
		}

		// Apply maxAssets limit if specified
		assetsToDelete := assets
		if params.MaxAssets > 0 && len(assets) > params.MaxAssets {
			assetsToDelete = assets[:params.MaxAssets]
		}

		result := map[string]interface{}{
			"albumID":        albumID,
			"albumName":      albumName,
			"totalAssets":    len(assets),
			"assetsToDelete": len(assetsToDelete),
		}

		if params.DryRun {
			// Just return count and sample
			sampleSize := 5
			if len(assetsToDelete) < sampleSize {
				sampleSize = len(assetsToDelete)
			}

			sampleData := []map[string]interface{}{}
			for i := 0; i < sampleSize; i++ {
				asset := assetsToDelete[i]
				sampleData = append(sampleData, map[string]interface{}{
					"id":       asset.ID,
					"fileName": asset.OriginalFileName,
					"type":     asset.Type,
				})
			}

			result["sampleAssets"] = sampleData
			result["dryRun"] = true
			result["message"] = fmt.Sprintf("Dry run: would delete %d assets from album", len(assetsToDelete))
			result["success"] = true
			return makeMCPResult(result)
		}

		// Delete assets in batches
		deleted := 0
		failed := 0
		var deleteErrors []string

		for i := 0; i < len(assetsToDelete); i += params.BatchSize {
			// Check for context cancellation
			select {
			case <-ctx.Done():
				result["deleted"] = deleted
				result["failed"] = failed + (len(assetsToDelete) - i)
				result["success"] = false
				result["message"] = "Operation cancelled"
				return makeMCPResult(result)
			default:
			}

			end := i + params.BatchSize
			if end > len(assetsToDelete) {
				end = len(assetsToDelete)
			}

			batch := assetsToDelete[i:end]
			batchIDs := make([]string, len(batch))
			for j, asset := range batch {
				batchIDs[j] = asset.ID
			}

			err := immichClient.DeleteAssets(ctx, batchIDs, params.ForceDelete)
			if err != nil {
				failed += len(batch)
				deleteErrors = append(deleteErrors, fmt.Sprintf("batch %d-%d: %v", i, end, err))
			} else {
				deleted += len(batch)
			}
		}

		result["deleted"] = deleted
		result["failed"] = failed
		result["forceDelete"] = params.ForceDelete
		result["success"] = failed == 0

		if failed > 0 {
			result["errors"] = deleteErrors
			result["message"] = fmt.Sprintf("Deleted %d assets, %d failed", deleted, failed)
		} else {
			if params.ForceDelete {
				result["message"] = fmt.Sprintf("Permanently deleted %d assets from album", deleted)
			} else {
				result["message"] = fmt.Sprintf("Moved %d assets to trash from album", deleted)
			}
		}

		return makeMCPResult(result)
	}

	s.AddTool(tool, handler)
}

// registerMovePhotosBySearch registers tool to move assets found by smart search to an album
func registerMovePhotosBySearch(s *server.MCPServer, immichClient *immich.Client) {
	tool := mcp.Tool{
		Name:        "movePhotosBySearch",
		Description: "Search for photos using AI smart search and move results to a new album",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Search query (e.g., 'beach', 'sunset', 'birthday party')",
				},
				"albumName": map[string]interface{}{
					"type":        "string",
					"description": "Name of the album to create/add photos to",
				},
				"maxResults": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of search results to include",
					"default":     100,
				},
				"createAlbum": map[string]interface{}{
					"type":        "boolean",
					"description": "Create album if it doesn't exist",
					"default":     true,
				},
				"dryRun": map[string]interface{}{
					"type":        "boolean",
					"description": "Just show search results without creating album",
					"default":     false,
				},
			},
			Required: []string{"query", "albumName"},
		},
	}

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var params struct {
			Query       string `json:"query"`
			AlbumName   string `json:"albumName"`
			MaxResults  int    `json:"maxResults"`
			CreateAlbum bool   `json:"createAlbum"`
			DryRun      bool   `json:"dryRun"`
		}

		// Set defaults
		params.MaxResults = 100
		params.CreateAlbum = true

		argBytes, ok := request.Params.Arguments.([]byte)
		if !ok {
			argBytes, _ = json.Marshal(request.Params.Arguments)
		}
		if err := json.Unmarshal(argBytes, &params); err != nil {
			return nil, fmt.Errorf("invalid parameters: %w", err)
		}

		// Perform smart search
		searchResults, err := immichClient.SmartSearch(ctx, params.Query, params.MaxResults)
		if err != nil {
			return nil, fmt.Errorf("smart search failed: %w", err)
		}

		result := map[string]interface{}{
			"query":       params.Query,
			"albumName":   params.AlbumName,
			"foundAssets": len(searchResults),
			"maxResults":  params.MaxResults,
		}

		if len(searchResults) == 0 {
			result["message"] = fmt.Sprintf("No assets found for query: %s", params.Query)
			result["success"] = true
			return makeMCPResult(result)
		}

		// In dry run, show sample results
		if params.DryRun {
			sampleSize := 10
			if len(searchResults) < sampleSize {
				sampleSize = len(searchResults)
			}

			sampleData := []map[string]interface{}{}
			for i := 0; i < sampleSize; i++ {
				asset := searchResults[i]
				sampleData = append(sampleData, map[string]interface{}{
					"id":       asset.ID,
					"fileName": asset.OriginalFileName,
					"type":     asset.Type,
					"date":     asset.FileCreatedAt,
				})
			}

			result["sampleResults"] = sampleData
			result["dryRun"] = true
			result["message"] = fmt.Sprintf("Dry run: found %d assets for '%s'", len(searchResults), params.Query)
			result["success"] = true
			return makeMCPResult(result)
		}

		// Find or create album
		var albumID string
		var albumFound bool
		albums, err := immichClient.ListAlbums(ctx, false)
		if err != nil {
			return nil, fmt.Errorf("failed to list albums: %w", err)
		}

		for _, album := range albums {
			if album.AlbumName == params.AlbumName {
				albumID = album.ID
				albumFound = true
				break
			}
		}

		if !albumFound {
			if !params.CreateAlbum {
				return nil, fmt.Errorf("album '%s' not found and createAlbum is false", params.AlbumName)
			}

			newAlbum, err := immichClient.CreateAlbum(ctx, immich.CreateAlbumParams{
				Name:        params.AlbumName,
				Description: fmt.Sprintf("Photos from search: %s", params.Query),
			})
			if err != nil {
				return nil, fmt.Errorf("failed to create album: %w", err)
			}
			albumID = newAlbum.ID
			result["albumCreated"] = true
		} else {
			result["albumCreated"] = false
		}

		// Add assets to album
		assetIDs := make([]string, len(searchResults))
		for i, asset := range searchResults {
			assetIDs[i] = asset.ID
		}

		bulkResult, err := immichClient.AddAssetsToAlbum(ctx, albumID, assetIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to add assets to album: %w", err)
		}

		result["albumID"] = albumID
		result["movedCount"] = len(bulkResult.Success)
		result["failedCount"] = len(bulkResult.Error)
		result["success"] = true
		result["message"] = fmt.Sprintf("Added %d assets from search '%s' to album '%s'",
			len(bulkResult.Success), params.Query, params.AlbumName)

		return makeMCPResult(result)
	}

	s.AddTool(tool, handler)
}

// registerSmartSearchAdvanced registers the comprehensive smart search tool with all API options
func registerSmartSearchAdvanced(s *server.MCPServer, immichClient *immich.Client) {
	tool := mcp.Tool{
		Name:        "smartSearchAdvanced",
		Description: "Advanced smart search with all available filters and options",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "AI-powered search query (e.g., 'beach sunset', 'cats playing')",
				},
				"albumIds": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Filter by specific album IDs",
				},
				"personIds": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Filter by specific person IDs",
				},
				"tagIds": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Filter by specific tag IDs",
				},
				"city": map[string]interface{}{
					"type":        "string",
					"description": "Filter by city name",
				},
				"country": map[string]interface{}{
					"type":        "string",
					"description": "Filter by country name",
				},
				"state": map[string]interface{}{
					"type":        "string",
					"description": "Filter by state/province name",
				},
				"make": map[string]interface{}{
					"type":        "string",
					"description": "Filter by camera make (e.g., 'Canon', 'Sony')",
				},
				"model": map[string]interface{}{
					"type":        "string",
					"description": "Filter by camera model (e.g., 'iPhone 14 Pro')",
				},
				"lensModel": map[string]interface{}{
					"type":        "string",
					"description": "Filter by lens model",
				},
				"deviceId": map[string]interface{}{
					"type":        "string",
					"description": "Filter by specific device ID",
				},
				"libraryId": map[string]interface{}{
					"type":        "string",
					"description": "Filter by library ID",
				},
				"queryAssetId": map[string]interface{}{
					"type":        "string",
					"description": "Find similar assets to this asset ID",
				},
				"type": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"IMAGE", "VIDEO", "AUDIO", "OTHER"},
					"description": "Filter by asset type",
				},
				"visibility": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"archive", "timeline", "hidden", "locked"},
					"description": "Filter by visibility status",
				},
				"createdAfter": map[string]interface{}{
					"type":        "string",
					"format":      "date-time",
					"description": "Assets created after this date (ISO 8601)",
				},
				"createdBefore": map[string]interface{}{
					"type":        "string",
					"format":      "date-time",
					"description": "Assets created before this date (ISO 8601)",
				},
				"takenAfter": map[string]interface{}{
					"type":        "string",
					"format":      "date-time",
					"description": "Photos taken after this date (ISO 8601)",
				},
				"takenBefore": map[string]interface{}{
					"type":        "string",
					"format":      "date-time",
					"description": "Photos taken before this date (ISO 8601)",
				},
				"updatedAfter": map[string]interface{}{
					"type":        "string",
					"format":      "date-time",
					"description": "Assets updated after this date (ISO 8601)",
				},
				"updatedBefore": map[string]interface{}{
					"type":        "string",
					"format":      "date-time",
					"description": "Assets updated before this date (ISO 8601)",
				},
				"trashedAfter": map[string]interface{}{
					"type":        "string",
					"format":      "date-time",
					"description": "Assets trashed after this date (ISO 8601)",
				},
				"trashedBefore": map[string]interface{}{
					"type":        "string",
					"format":      "date-time",
					"description": "Assets trashed before this date (ISO 8601)",
				},
				"isFavorite": map[string]interface{}{
					"type":        "boolean",
					"description": "Filter by favorite status",
				},
				"isEncoded": map[string]interface{}{
					"type":        "boolean",
					"description": "Filter by encoding status",
				},
				"isMotion": map[string]interface{}{
					"type":        "boolean",
					"description": "Filter for motion photos/videos",
				},
				"isOffline": map[string]interface{}{
					"type":        "boolean",
					"description": "Filter for offline assets",
				},
				"isNotInAlbum": map[string]interface{}{
					"type":        "boolean",
					"description": "Filter for assets not in any album",
				},
				"withDeleted": map[string]interface{}{
					"type":        "boolean",
					"description": "Include deleted assets",
				},
				"withExif": map[string]interface{}{
					"type":        "boolean",
					"description": "Include EXIF data in results",
				},
				"rating": map[string]interface{}{
					"type":        "integer",
					"minimum":     -1,
					"maximum":     5,
					"description": "Filter by rating (-1 to 5)",
				},
				"size": map[string]interface{}{
					"type":        "integer",
					"minimum":     1,
					"maximum":     5000,
					"default":     100,
					"description": "Maximum number of results (supports pagination)",
				},
				"language": map[string]interface{}{
					"type":        "string",
					"description": "Language for search query processing",
				},
			},
		},
	}

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var params struct {
			Query         string   `json:"query"`
			AlbumIds      []string `json:"albumIds"`
			PersonIds     []string `json:"personIds"`
			TagIds        []string `json:"tagIds"`
			City          string   `json:"city"`
			Country       string   `json:"country"`
			State         string   `json:"state"`
			Make          string   `json:"make"`
			Model         string   `json:"model"`
			LensModel     string   `json:"lensModel"`
			DeviceId      string   `json:"deviceId"`
			LibraryId     string   `json:"libraryId"`
			QueryAssetId  string   `json:"queryAssetId"`
			Type          string   `json:"type"`
			Visibility    string   `json:"visibility"`
			CreatedAfter  string   `json:"createdAfter"`
			CreatedBefore string   `json:"createdBefore"`
			TakenAfter    string   `json:"takenAfter"`
			TakenBefore   string   `json:"takenBefore"`
			UpdatedAfter  string   `json:"updatedAfter"`
			UpdatedBefore string   `json:"updatedBefore"`
			TrashedAfter  string   `json:"trashedAfter"`
			TrashedBefore string   `json:"trashedBefore"`
			IsFavorite    *bool    `json:"isFavorite"`
			IsEncoded     *bool    `json:"isEncoded"`
			IsMotion      *bool    `json:"isMotion"`
			IsOffline     *bool    `json:"isOffline"`
			IsNotInAlbum  *bool    `json:"isNotInAlbum"`
			WithDeleted   *bool    `json:"withDeleted"`
			WithExif      *bool    `json:"withExif"`
			Rating        *int     `json:"rating"`
			Size          int      `json:"size"`
			Language      string   `json:"language"`
		}

		// Set default size
		params.Size = 100

		argBytes, ok := request.Params.Arguments.([]byte)
		if !ok {
			argBytes, _ = json.Marshal(request.Params.Arguments)
		}
		if err := json.Unmarshal(argBytes, &params); err != nil {
			return nil, fmt.Errorf("invalid parameters: %w", err)
		}

		// Convert to immich.SmartSearchParams
		searchParams := immich.SmartSearchParams{
			Query:         params.Query,
			AlbumIds:      params.AlbumIds,
			PersonIds:     params.PersonIds,
			TagIds:        params.TagIds,
			City:          params.City,
			Country:       params.Country,
			State:         params.State,
			Make:          params.Make,
			Model:         params.Model,
			LensModel:     params.LensModel,
			DeviceId:      params.DeviceId,
			LibraryId:     params.LibraryId,
			QueryAssetId:  params.QueryAssetId,
			Type:          params.Type,
			Visibility:    params.Visibility,
			CreatedAfter:  params.CreatedAfter,
			CreatedBefore: params.CreatedBefore,
			TakenAfter:    params.TakenAfter,
			TakenBefore:   params.TakenBefore,
			UpdatedAfter:  params.UpdatedAfter,
			UpdatedBefore: params.UpdatedBefore,
			TrashedAfter:  params.TrashedAfter,
			TrashedBefore: params.TrashedBefore,
			IsFavorite:    params.IsFavorite,
			IsEncoded:     params.IsEncoded,
			IsMotion:      params.IsMotion,
			IsOffline:     params.IsOffline,
			IsNotInAlbum:  params.IsNotInAlbum,
			WithDeleted:   params.WithDeleted,
			WithExif:      params.WithExif,
			Rating:        params.Rating,
			Size:          params.Size,
			Language:      params.Language,
		}

		// Perform the search
		searchResults, err := immichClient.SmartSearchAdvanced(ctx, searchParams)
		if err != nil {
			return nil, fmt.Errorf("smart search failed: %w", err)
		}

		// Build active filters list for clarity
		var activeFilters []string
		if params.Query != "" {
			activeFilters = append(activeFilters, fmt.Sprintf("query='%s'", params.Query))
		}
		if params.Type != "" {
			activeFilters = append(activeFilters, fmt.Sprintf("type=%s", params.Type))
		}
		if params.IsFavorite != nil && *params.IsFavorite {
			activeFilters = append(activeFilters, "favorites only")
		}
		if params.IsNotInAlbum != nil && *params.IsNotInAlbum {
			activeFilters = append(activeFilters, "not in albums")
		}
		if params.City != "" {
			activeFilters = append(activeFilters, fmt.Sprintf("city=%s", params.City))
		}
		if params.Country != "" {
			activeFilters = append(activeFilters, fmt.Sprintf("country=%s", params.Country))
		}
		if params.TakenAfter != "" || params.TakenBefore != "" {
			activeFilters = append(activeFilters, "date range filter")
		}

		result := map[string]interface{}{
			"foundCount":    len(searchResults),
			"activeFilters": activeFilters,
			"requestedSize": params.Size,
		}

		// Include sample results
		sampleSize := 10
		if len(searchResults) < sampleSize {
			sampleSize = len(searchResults)
		}

		sampleData := []map[string]interface{}{}
		for i := 0; i < sampleSize; i++ {
			asset := searchResults[i]
			assetInfo := map[string]interface{}{
				"id":       asset.ID,
				"fileName": asset.OriginalFileName,
				"type":     asset.Type,
				"date":     asset.FileCreatedAt,
			}

			// Add location info if available
			if asset.ExifInfo != nil {
				if asset.ExifInfo.City != "" || asset.ExifInfo.Country != "" {
					location := ""
					if asset.ExifInfo.City != "" {
						location = asset.ExifInfo.City
						if asset.ExifInfo.State != "" {
							location += ", " + asset.ExifInfo.State
						}
						if asset.ExifInfo.Country != "" {
							location += ", " + asset.ExifInfo.Country
						}
					} else if asset.ExifInfo.Country != "" {
						location = asset.ExifInfo.Country
					}
					assetInfo["location"] = location
				}

				// Add camera info if available
				if asset.ExifInfo.Make != "" || asset.ExifInfo.Model != "" {
					camera := ""
					if asset.ExifInfo.Make != "" {
						camera = asset.ExifInfo.Make
					}
					if asset.ExifInfo.Model != "" {
						if camera != "" {
							camera += " "
						}
						camera += asset.ExifInfo.Model
					}
					assetInfo["camera"] = camera
				}
			}

			sampleData = append(sampleData, assetInfo)
		}
		result["sampleResults"] = sampleData

		// Add asset IDs for further processing
		assetIds := make([]string, len(searchResults))
		for i, asset := range searchResults {
			assetIds[i] = asset.ID
		}
		result["assetIds"] = assetIds

		return makeMCPResult(result)
	}

	s.AddTool(tool, handler)
}

// Helper function to parse duration string (format: "H:MM:SS.mmmmm" or "MM:SS.mmmmm")
func parseDuration(duration string) int {
	// Remove milliseconds if present
	parts := strings.Split(duration, ".")
	timeStr := parts[0]

	// Split by colon
	timeParts := strings.Split(timeStr, ":")
	seconds := 0

	switch len(timeParts) {
	case 3: // H:MM:SS
		hours, _ := strconv.Atoi(timeParts[0])
		minutes, _ := strconv.Atoi(timeParts[1])
		secs, _ := strconv.Atoi(timeParts[2])
		seconds = hours*3600 + minutes*60 + secs
	case 2: // MM:SS
		minutes, _ := strconv.Atoi(timeParts[0])
		secs, _ := strconv.Atoi(timeParts[1])
		seconds = minutes*60 + secs
	case 1: // SS
		seconds, _ = strconv.Atoi(timeParts[0])
	}

	return seconds
}

// Helper function to create MCP result
func makeMCPResult(data interface{}) (*mcp.CallToolResult, error) {
	content, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	return mcp.NewToolResultText(string(content)), nil
}
