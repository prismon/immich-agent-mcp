package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/yourusername/mcp-immich/pkg/config"
	"github.com/yourusername/mcp-immich/pkg/immich"
	"github.com/yourusername/mcp-immich/pkg/livealbums"
)

// RegisterLiveAlbumTools registers all live album tools
func RegisterLiveAlbumTools(s *server.MCPServer, cfg *config.Config, immichClient *immich.Client) {
	registerCreateLiveAlbum(s, cfg, immichClient)
	registerListLiveAlbums(s, immichClient)
	registerUpdateLiveAlbum(s, immichClient)
	registerConvertToLiveAlbum(s, cfg, immichClient)
	registerDisableLiveAlbum(s, immichClient)
	registerGetLiveAlbumStatus(s, immichClient)
}

// registerCreateLiveAlbum creates a new live album with automatic updates
func registerCreateLiveAlbum(s *server.MCPServer, cfg *config.Config, immichClient *immich.Client) {
	tool := mcp.Tool{
		Name:        "createLiveAlbum",
		Description: "Create a live album that automatically updates based on search criteria. The album will periodically re-run the search and add new matching photos.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"albumName": map[string]interface{}{
					"type":        "string",
					"description": "Name of the live album to create",
				},
				"searchQuery": map[string]interface{}{
					"type":        "string",
					"description": "AI smart search query (e.g., 'beach', 'sunset', 'birthday party')",
				},
				"searchType": map[string]interface{}{
					"type":        "string",
					"description": "Type of search: 'smart' for AI search or 'advanced' for detailed filters",
					"enum":        []string{"smart", "advanced"},
					"default":     "smart",
				},
				"searchParams": map[string]interface{}{
					"type":        "object",
					"description": "Advanced search parameters (only used if searchType is 'advanced')",
				},
				"syncStrategy": map[string]interface{}{
					"type":        "string",
					"description": "Sync strategy: 'add-only' (only add new matches) or 'full-sync' (add new, remove non-matches)",
					"enum":        []string{"add-only", "full-sync"},
					"default":     "add-only",
				},
				"maxResults": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of assets to include in the album",
					"default":     5000,
					"minimum":     1,
					"maximum":     10000,
				},
				"enabled": map[string]interface{}{
					"type":        "boolean",
					"description": "Enable automatic updates for this album",
					"default":     true,
				},
			},
			Required: []string{"albumName", "searchQuery"},
		},
	}

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var params struct {
			AlbumName    string                 `json:"albumName"`
			SearchQuery  string                 `json:"searchQuery"`
			SearchType   string                 `json:"searchType"`
			SearchParams map[string]interface{} `json:"searchParams"`
			SyncStrategy string                 `json:"syncStrategy"`
			MaxResults   int                    `json:"maxResults"`
			Enabled      bool                   `json:"enabled"`
		}

		// Set defaults
		params.SearchType = "smart"
		params.SyncStrategy = cfg.LiveAlbumSyncStrategy
		params.MaxResults = cfg.LiveAlbumMaxResults
		params.Enabled = true

		argBytes, ok := request.Params.Arguments.([]byte)
		if !ok {
			argBytes, _ = json.Marshal(request.Params.Arguments)
		}
		if err := json.Unmarshal(argBytes, &params); err != nil {
			return nil, fmt.Errorf("invalid parameters: %w", err)
		}

		// Create metadata
		var metadata *livealbums.LiveAlbumMetadata
		if params.SearchType == "smart" {
			metadata = livealbums.NewSmartSearchMetadata(params.SearchQuery, params.SyncStrategy, params.MaxResults)
		} else {
			metadata = livealbums.NewAdvancedSearchMetadata(params.SearchParams, params.SyncStrategy, params.MaxResults)
		}
		metadata.Enabled = params.Enabled

		// Encode metadata to description
		description, err := metadata.EncodeToDescription()
		if err != nil {
			return nil, fmt.Errorf("failed to encode metadata: %w", err)
		}

		// Perform initial search
		var searchResults []immich.Asset
		if params.SearchType == "smart" {
			searchResults, err = immichClient.SmartSearch(ctx, params.SearchQuery, params.MaxResults)
			if err != nil {
				return nil, fmt.Errorf("smart search failed: %w", err)
			}
		} else {
			searchParams, err := convertToSmartSearchParams(params.SearchParams, params.MaxResults)
			if err != nil {
				return nil, fmt.Errorf("failed to convert search params: %w", err)
			}
			searchResults, err = immichClient.SmartSearchAdvanced(ctx, searchParams)
			if err != nil {
				return nil, fmt.Errorf("advanced search failed: %w", err)
			}
		}

		// Get asset IDs
		assetIDs := []string{}
		for _, asset := range searchResults {
			assetIDs = append(assetIDs, asset.ID)
		}

		// Create album with metadata in description
		album, err := immichClient.CreateAlbum(ctx, immich.CreateAlbumParams{
			Name:        params.AlbumName,
			Description: description,
			AssetIDs:    assetIDs,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create album: %w", err)
		}

		result := map[string]interface{}{
			"success":       true,
			"albumId":       album.ID,
			"albumName":     album.AlbumName,
			"searchType":    params.SearchType,
			"searchQuery":   params.SearchQuery,
			"syncStrategy":  params.SyncStrategy,
			"enabled":       params.Enabled,
			"initialAssets": len(assetIDs),
			"maxResults":    params.MaxResults,
			"message": fmt.Sprintf("Created live album '%s' with %d assets. Album will automatically update based on the search query.",
				album.AlbumName, len(assetIDs)),
		}

		return makeMCPResult(result)
	}

	s.AddTool(tool, handler)
}

// registerListLiveAlbums lists all live albums
func registerListLiveAlbums(s *server.MCPServer, immichClient *immich.Client) {
	tool := mcp.Tool{
		Name:        "listLiveAlbums",
		Description: "List all live albums with their search criteria and sync settings",
		InputSchema: mcp.ToolInputSchema{
			Type:       "object",
			Properties: map[string]interface{}{},
		},
	}

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Get all albums
		albums, err := immichClient.GetAllAlbumsWithInfo(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get albums: %w", err)
		}

		// Filter and parse live albums
		liveAlbums := []map[string]interface{}{}
		for _, album := range albums {
			if livealbums.IsLive(album.Description) {
				metadata, err := livealbums.DecodeFromDescription(album.Description)
				if err != nil {
					continue
				}

				liveAlbum := map[string]interface{}{
					"albumId":      album.ID,
					"albumName":    album.AlbumName,
					"searchType":   metadata.SearchType,
					"searchQuery":  metadata.SearchQuery,
					"syncStrategy": metadata.SyncStrategy,
					"maxResults":   metadata.MaxResults,
					"enabled":      metadata.Enabled,
					"assetCount":   album.AssetCount,
					"lastUpdated":  metadata.LastUpdated.Format(time.RFC3339),
					"updateCount":  metadata.UpdateCount,
				}
				liveAlbums = append(liveAlbums, liveAlbum)
			}
		}

		result := map[string]interface{}{
			"success":    true,
			"totalCount": len(liveAlbums),
			"liveAlbums": liveAlbums,
		}

		return makeMCPResult(result)
	}

	s.AddTool(tool, handler)
}

// registerUpdateLiveAlbum manually triggers an update for a live album
func registerUpdateLiveAlbum(s *server.MCPServer, immichClient *immich.Client) {
	tool := mcp.Tool{
		Name:        "updateLiveAlbum",
		Description: "Manually trigger an update for a live album, re-running the search and syncing assets",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"albumId": map[string]interface{}{
					"type":        "string",
					"description": "ID of the live album to update",
				},
			},
			Required: []string{"albumId"},
		},
	}

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var params struct {
			AlbumID string `json:"albumId"`
		}

		argBytes, ok := request.Params.Arguments.([]byte)
		if !ok {
			argBytes, _ = json.Marshal(request.Params.Arguments)
		}
		if err := json.Unmarshal(argBytes, &params); err != nil {
			return nil, fmt.Errorf("invalid parameters: %w", err)
		}

		// Get all albums to find the target album
		albums, err := immichClient.GetAllAlbumsWithInfo(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get albums: %w", err)
		}

		var targetAlbum *immich.Album
		for _, album := range albums {
			if album.ID == params.AlbumID {
				targetAlbum = &album
				break
			}
		}

		if targetAlbum == nil {
			return nil, fmt.Errorf("album not found: %s", params.AlbumID)
		}

		if !livealbums.IsLive(targetAlbum.Description) {
			return nil, fmt.Errorf("album is not a live album: %s", targetAlbum.AlbumName)
		}

		// Update the album
		updater := livealbums.NewUpdater(immichClient)
		updateResult := updater.UpdateAlbum(ctx, *targetAlbum)

		if updateResult.Error != nil {
			return nil, fmt.Errorf("failed to update album: %w", updateResult.Error)
		}

		result := map[string]interface{}{
			"success":       true,
			"albumId":       updateResult.AlbumID,
			"albumName":     updateResult.AlbumName,
			"assetsAdded":   updateResult.AssetsAdded,
			"assetsRemoved": updateResult.AssetsRemoved,
			"totalAssets":   updateResult.TotalAssets,
			"updatedAt":     updateResult.UpdatedAt.Format(time.RFC3339),
			"message": fmt.Sprintf("Updated live album '%s': added %d, removed %d, total %d assets",
				updateResult.AlbumName, updateResult.AssetsAdded, updateResult.AssetsRemoved, updateResult.TotalAssets),
		}

		return makeMCPResult(result)
	}

	s.AddTool(tool, handler)
}

// registerConvertToLiveAlbum converts an existing album to a live album
func registerConvertToLiveAlbum(s *server.MCPServer, cfg *config.Config, immichClient *immich.Client) {
	tool := mcp.Tool{
		Name:        "convertToLiveAlbum",
		Description: "Convert an existing album to a live album with automatic updates",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"albumId": map[string]interface{}{
					"type":        "string",
					"description": "ID of the album to convert",
				},
				"searchQuery": map[string]interface{}{
					"type":        "string",
					"description": "AI smart search query for future updates",
				},
				"searchType": map[string]interface{}{
					"type":        "string",
					"description": "Type of search: 'smart' or 'advanced'",
					"enum":        []string{"smart", "advanced"},
					"default":     "smart",
				},
				"searchParams": map[string]interface{}{
					"type":        "object",
					"description": "Advanced search parameters (only used if searchType is 'advanced')",
				},
				"syncStrategy": map[string]interface{}{
					"type":        "string",
					"description": "Sync strategy: 'add-only' or 'full-sync'",
					"enum":        []string{"add-only", "full-sync"},
					"default":     "add-only",
				},
				"maxResults": map[string]interface{}{
					"type":    "integer",
					"default": 5000,
				},
			},
			Required: []string{"albumId", "searchQuery"},
		},
	}

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var params struct {
			AlbumID      string                 `json:"albumId"`
			SearchQuery  string                 `json:"searchQuery"`
			SearchType   string                 `json:"searchType"`
			SearchParams map[string]interface{} `json:"searchParams"`
			SyncStrategy string                 `json:"syncStrategy"`
			MaxResults   int                    `json:"maxResults"`
		}

		// Set defaults
		params.SearchType = "smart"
		params.SyncStrategy = cfg.LiveAlbumSyncStrategy
		params.MaxResults = cfg.LiveAlbumMaxResults

		argBytes, ok := request.Params.Arguments.([]byte)
		if !ok {
			argBytes, _ = json.Marshal(request.Params.Arguments)
		}
		if err := json.Unmarshal(argBytes, &params); err != nil {
			return nil, fmt.Errorf("invalid parameters: %w", err)
		}

		// Get all albums to find the target album
		albums, err := immichClient.GetAllAlbumsWithInfo(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get albums: %w", err)
		}

		var targetAlbum *immich.Album
		for _, album := range albums {
			if album.ID == params.AlbumID {
				targetAlbum = &album
				break
			}
		}

		if targetAlbum == nil {
			return nil, fmt.Errorf("album not found: %s", params.AlbumID)
		}

		if livealbums.IsLive(targetAlbum.Description) {
			return nil, fmt.Errorf("album is already a live album: %s", targetAlbum.AlbumName)
		}

		// Create metadata
		var metadata *livealbums.LiveAlbumMetadata
		if params.SearchType == "smart" {
			metadata = livealbums.NewSmartSearchMetadata(params.SearchQuery, params.SyncStrategy, params.MaxResults)
		} else {
			metadata = livealbums.NewAdvancedSearchMetadata(params.SearchParams, params.SyncStrategy, params.MaxResults)
		}

		// Encode metadata to description
		description, err := metadata.EncodeToDescription()
		if err != nil {
			return nil, fmt.Errorf("failed to encode metadata: %w", err)
		}

		// Update album with new description
		updatedAlbum, err := immichClient.UpdateAlbum(ctx, params.AlbumID, "", description)
		if err != nil {
			return nil, fmt.Errorf("failed to update album: %w", err)
		}

		result := map[string]interface{}{
			"success":      true,
			"albumId":      updatedAlbum.ID,
			"albumName":    updatedAlbum.AlbumName,
			"searchType":   params.SearchType,
			"searchQuery":  params.SearchQuery,
			"syncStrategy": params.SyncStrategy,
			"maxResults":   params.MaxResults,
			"message": fmt.Sprintf("Converted album '%s' to a live album. It will now automatically update based on the search query.",
				updatedAlbum.AlbumName),
		}

		return makeMCPResult(result)
	}

	s.AddTool(tool, handler)
}

// registerDisableLiveAlbum disables automatic updates for a live album
func registerDisableLiveAlbum(s *server.MCPServer, immichClient *immich.Client) {
	tool := mcp.Tool{
		Name:        "disableLiveAlbum",
		Description: "Disable or enable automatic updates for a live album",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"albumId": map[string]interface{}{
					"type":        "string",
					"description": "ID of the live album",
				},
				"enabled": map[string]interface{}{
					"type":        "boolean",
					"description": "Enable (true) or disable (false) automatic updates",
				},
			},
			Required: []string{"albumId", "enabled"},
		},
	}

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var params struct {
			AlbumID string `json:"albumId"`
			Enabled bool   `json:"enabled"`
		}

		argBytes, ok := request.Params.Arguments.([]byte)
		if !ok {
			argBytes, _ = json.Marshal(request.Params.Arguments)
		}
		if err := json.Unmarshal(argBytes, &params); err != nil {
			return nil, fmt.Errorf("invalid parameters: %w", err)
		}

		// Get all albums to find the target album
		albums, err := immichClient.GetAllAlbumsWithInfo(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get albums: %w", err)
		}

		var targetAlbum *immich.Album
		for _, album := range albums {
			if album.ID == params.AlbumID {
				targetAlbum = &album
				break
			}
		}

		if targetAlbum == nil {
			return nil, fmt.Errorf("album not found: %s", params.AlbumID)
		}

		if !livealbums.IsLive(targetAlbum.Description) {
			return nil, fmt.Errorf("album is not a live album: %s", targetAlbum.AlbumName)
		}

		// Parse and update metadata
		metadata, err := livealbums.DecodeFromDescription(targetAlbum.Description)
		if err != nil {
			return nil, fmt.Errorf("failed to parse metadata: %w", err)
		}

		metadata.Enabled = params.Enabled

		// Encode metadata
		description, err := metadata.EncodeToDescription()
		if err != nil {
			return nil, fmt.Errorf("failed to encode metadata: %w", err)
		}

		// Update album
		_, err = immichClient.UpdateAlbum(ctx, params.AlbumID, "", description)
		if err != nil {
			return nil, fmt.Errorf("failed to update album: %w", err)
		}

		status := "disabled"
		if params.Enabled {
			status = "enabled"
		}

		result := map[string]interface{}{
			"success":   true,
			"albumId":   targetAlbum.ID,
			"albumName": targetAlbum.AlbumName,
			"enabled":   params.Enabled,
			"message": fmt.Sprintf("Automatic updates %s for live album '%s'",
				status, targetAlbum.AlbumName),
		}

		return makeMCPResult(result)
	}

	s.AddTool(tool, handler)
}

// registerGetLiveAlbumStatus gets the status and metadata of a live album
func registerGetLiveAlbumStatus(s *server.MCPServer, immichClient *immich.Client) {
	tool := mcp.Tool{
		Name:        "getLiveAlbumStatus",
		Description: "Get detailed status and metadata for a live album",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"albumId": map[string]interface{}{
					"type":        "string",
					"description": "ID of the live album",
				},
			},
			Required: []string{"albumId"},
		},
	}

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var params struct {
			AlbumID string `json:"albumId"`
		}

		argBytes, ok := request.Params.Arguments.([]byte)
		if !ok {
			argBytes, _ = json.Marshal(request.Params.Arguments)
		}
		if err := json.Unmarshal(argBytes, &params); err != nil {
			return nil, fmt.Errorf("invalid parameters: %w", err)
		}

		// Get all albums to find the target album
		albums, err := immichClient.GetAllAlbumsWithInfo(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get albums: %w", err)
		}

		var targetAlbum *immich.Album
		for _, album := range albums {
			if album.ID == params.AlbumID {
				targetAlbum = &album
				break
			}
		}

		if targetAlbum == nil {
			return nil, fmt.Errorf("album not found: %s", params.AlbumID)
		}

		if !livealbums.IsLive(targetAlbum.Description) {
			return nil, fmt.Errorf("album is not a live album: %s", targetAlbum.AlbumName)
		}

		// Parse metadata
		metadata, err := livealbums.DecodeFromDescription(targetAlbum.Description)
		if err != nil {
			return nil, fmt.Errorf("failed to parse metadata: %w", err)
		}

		result := map[string]interface{}{
			"success":      true,
			"albumId":      targetAlbum.ID,
			"albumName":    targetAlbum.AlbumName,
			"searchType":   metadata.SearchType,
			"searchQuery":  metadata.SearchQuery,
			"syncStrategy": metadata.SyncStrategy,
			"maxResults":   metadata.MaxResults,
			"enabled":      metadata.Enabled,
			"assetCount":   targetAlbum.AssetCount,
			"lastUpdated":  metadata.LastUpdated.Format(time.RFC3339),
			"updateCount":  metadata.UpdateCount,
			"createdAt":    targetAlbum.CreatedAt.Format(time.RFC3339),
			"updatedAt":    targetAlbum.UpdatedAt.Format(time.RFC3339),
		}

		if metadata.SearchType == "advanced" && metadata.SearchParams != nil {
			result["searchParams"] = metadata.SearchParams
		}

		return makeMCPResult(result)
	}

	s.AddTool(tool, handler)
}

// Helper function to convert search params (same as in updater.go)
func convertToSmartSearchParams(params map[string]interface{}, maxResults int) (immich.SmartSearchParams, error) {
	searchParams := immich.SmartSearchParams{
		Size: maxResults,
	}

	// Helper function to safely convert values
	getString := func(key string) string {
		if v, ok := params[key]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
		return ""
	}

	getStringSlice := func(key string) []string {
		if v, ok := params[key]; ok {
			if slice, ok := v.([]interface{}); ok {
				result := []string{}
				for _, item := range slice {
					if s, ok := item.(string); ok {
						result = append(result, s)
					}
				}
				return result
			}
		}
		return nil
	}

	getBoolPtr := func(key string) *bool {
		if v, ok := params[key]; ok {
			if b, ok := v.(bool); ok {
				return &b
			}
		}
		return nil
	}

	getIntPtr := func(key string) *int {
		if v, ok := params[key]; ok {
			switch val := v.(type) {
			case int:
				return &val
			case float64:
				intVal := int(val)
				return &intVal
			}
		}
		return nil
	}

	// Populate search params
	searchParams.Query = getString("query")
	searchParams.QueryAssetId = getString("queryAssetId")
	searchParams.AlbumIds = getStringSlice("albumIds")
	searchParams.PersonIds = getStringSlice("personIds")
	searchParams.TagIds = getStringSlice("tagIds")
	searchParams.City = getString("city")
	searchParams.Country = getString("country")
	searchParams.State = getString("state")
	searchParams.Make = getString("make")
	searchParams.Model = getString("model")
	searchParams.LensModel = getString("lensModel")
	searchParams.DeviceId = getString("deviceId")
	searchParams.LibraryId = getString("libraryId")
	searchParams.Type = getString("type")
	searchParams.Visibility = getString("visibility")
	searchParams.CreatedAfter = getString("createdAfter")
	searchParams.CreatedBefore = getString("createdBefore")
	searchParams.TakenAfter = getString("takenAfter")
	searchParams.TakenBefore = getString("takenBefore")
	searchParams.UpdatedAfter = getString("updatedAfter")
	searchParams.UpdatedBefore = getString("updatedBefore")
	searchParams.TrashedAfter = getString("trashedAfter")
	searchParams.TrashedBefore = getString("trashedBefore")
	searchParams.IsFavorite = getBoolPtr("isFavorite")
	searchParams.IsEncoded = getBoolPtr("isEncoded")
	searchParams.IsMotion = getBoolPtr("isMotion")
	searchParams.IsOffline = getBoolPtr("isOffline")
	searchParams.IsNotInAlbum = getBoolPtr("isNotInAlbum")
	searchParams.WithDeleted = getBoolPtr("withDeleted")
	searchParams.WithExif = getBoolPtr("withExif")
	searchParams.Rating = getIntPtr("rating")
	searchParams.Language = getString("language")

	return searchParams, nil
}
