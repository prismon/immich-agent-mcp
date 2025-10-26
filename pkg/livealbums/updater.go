package livealbums

import (
	"context"
	"fmt"
	"time"

	"github.com/yourusername/mcp-immich/pkg/immich"
	"github.com/rs/zerolog/log"
)

// UpdateResult contains the result of an album update
type UpdateResult struct {
	AlbumID       string
	AlbumName     string
	AssetsAdded   int
	AssetsRemoved int
	TotalAssets   int
	UpdatedAt     time.Time
	Error         error
}

// Updater handles live album updates
type Updater struct {
	client *immich.Client
}

// NewUpdater creates a new live album updater
func NewUpdater(client *immich.Client) *Updater {
	return &Updater{
		client: client,
	}
}

// UpdateAlbum updates a single live album
func (u *Updater) UpdateAlbum(ctx context.Context, album immich.Album) UpdateResult {
	result := UpdateResult{
		AlbumID:   album.ID,
		AlbumName: album.AlbumName,
		UpdatedAt: time.Now(),
	}

	// Parse metadata from description
	metadata, err := DecodeFromDescription(album.Description)
	if err != nil {
		result.Error = fmt.Errorf("failed to parse metadata: %w", err)
		return result
	}

	// Validate metadata
	if err := metadata.Validate(); err != nil {
		result.Error = fmt.Errorf("invalid metadata: %w", err)
		return result
	}

	// Check if updates are enabled
	if !metadata.Enabled {
		log.Debug().
			Str("album_id", album.ID).
			Str("album_name", album.AlbumName).
			Msg("Live album updates disabled, skipping")
		return result
	}

	log.Info().
		Str("album_id", album.ID).
		Str("album_name", album.AlbumName).
		Str("search_type", metadata.SearchType).
		Str("sync_strategy", metadata.SyncStrategy).
		Msg("Updating live album")

	// Get current album assets
	currentAssets, err := u.client.GetAlbumAssets(ctx, album.ID)
	if err != nil {
		result.Error = fmt.Errorf("failed to get current album assets: %w", err)
		return result
	}

	currentAssetIDs := make(map[string]bool)
	for _, asset := range currentAssets {
		currentAssetIDs[asset.ID] = true
	}

	// Run search to get new assets
	var newAssets []immich.Asset
	switch metadata.SearchType {
	case "smart":
		newAssets, err = u.client.SmartSearch(ctx, metadata.SearchQuery, metadata.MaxResults)
		if err != nil {
			result.Error = fmt.Errorf("failed to run smart search: %w", err)
			return result
		}
	case "advanced":
		// Convert metadata.SearchParams to SmartSearchParams
		params, err := convertToSmartSearchParams(metadata.SearchParams, metadata.MaxResults)
		if err != nil {
			result.Error = fmt.Errorf("failed to convert search params: %w", err)
			return result
		}
		newAssets, err = u.client.SmartSearchAdvanced(ctx, params)
		if err != nil {
			result.Error = fmt.Errorf("failed to run advanced search: %w", err)
			return result
		}
	default:
		result.Error = fmt.Errorf("unknown search type: %s", metadata.SearchType)
		return result
	}

	// Build set of new asset IDs
	newAssetIDs := make(map[string]bool)
	newAssetIDsList := []string{}
	for _, asset := range newAssets {
		if !newAssetIDs[asset.ID] {
			newAssetIDs[asset.ID] = true
			newAssetIDsList = append(newAssetIDsList, asset.ID)
		}
	}

	// Determine assets to add (in new but not in current)
	assetsToAdd := []string{}
	for assetID := range newAssetIDs {
		if !currentAssetIDs[assetID] {
			assetsToAdd = append(assetsToAdd, assetID)
		}
	}

	// Add new assets
	if len(assetsToAdd) > 0 {
		log.Info().
			Str("album_id", album.ID).
			Int("count", len(assetsToAdd)).
			Msg("Adding assets to album")

		_, err := u.client.AddAssetsToAlbum(ctx, album.ID, assetsToAdd)
		if err != nil {
			result.Error = fmt.Errorf("failed to add assets: %w", err)
			return result
		}
		result.AssetsAdded = len(assetsToAdd)
	}

	// For full-sync, remove assets that are no longer in search results
	if metadata.SyncStrategy == "full-sync" {
		assetsToRemove := []string{}
		for assetID := range currentAssetIDs {
			if !newAssetIDs[assetID] {
				assetsToRemove = append(assetsToRemove, assetID)
			}
		}

		if len(assetsToRemove) > 0 {
			log.Info().
				Str("album_id", album.ID).
				Int("count", len(assetsToRemove)).
				Msg("Removing assets from album (full-sync mode)")

			_, err := u.client.RemoveAssetsFromAlbum(ctx, album.ID, assetsToRemove)
			if err != nil {
				result.Error = fmt.Errorf("failed to remove assets: %w", err)
				return result
			}
			result.AssetsRemoved = len(assetsToRemove)
		}
	}

	// Update metadata
	metadata.UpdateTimestamp()
	metadata.LastAssetIDs = newAssetIDsList

	// Save updated metadata
	newDescription, err := metadata.EncodeToDescription()
	if err != nil {
		result.Error = fmt.Errorf("failed to encode metadata: %w", err)
		return result
	}

	_, err = u.client.UpdateAlbum(ctx, album.ID, "", newDescription)
	if err != nil {
		result.Error = fmt.Errorf("failed to update album metadata: %w", err)
		return result
	}

	result.TotalAssets = len(newAssetIDs)

	log.Info().
		Str("album_id", album.ID).
		Str("album_name", album.AlbumName).
		Int("added", result.AssetsAdded).
		Int("removed", result.AssetsRemoved).
		Int("total", result.TotalAssets).
		Msg("Live album update completed")

	return result
}

// UpdateAllLiveAlbums updates all live albums
func (u *Updater) UpdateAllLiveAlbums(ctx context.Context) ([]UpdateResult, error) {
	// Get all albums
	albums, err := u.client.GetAllAlbumsWithInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get albums: %w", err)
	}

	results := []UpdateResult{}

	// Find and update live albums
	for _, album := range albums {
		if IsLive(album.Description) {
			result := u.UpdateAlbum(ctx, album)
			results = append(results, result)
		}
	}

	log.Info().
		Int("total_albums", len(albums)).
		Int("live_albums_updated", len(results)).
		Msg("Completed live album update cycle")

	return results, nil
}

// convertToSmartSearchParams converts map to SmartSearchParams
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
			// Handle both int and float64 (JSON numbers)
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
