package livealbums

import (
	"encoding/json"
	"fmt"
	"time"
)

// LiveAlbumMetadata stores search criteria and sync settings for live albums
type LiveAlbumMetadata struct {
	IsLiveAlbum   bool                   `json:"liveAlbum"`
	SearchType    string                 `json:"searchType"`    // "smart" or "advanced"
	SearchQuery   string                 `json:"searchQuery"`   // For smart search
	SearchParams  map[string]interface{} `json:"searchParams"`  // For advanced search
	SyncStrategy  string                 `json:"syncStrategy"`  // "add-only" or "full-sync"
	MaxResults    int                    `json:"maxResults"`    // Max results per update
	LastUpdated   time.Time              `json:"lastUpdated"`   // Last update timestamp
	Enabled       bool                   `json:"enabled"`       // Enable/disable auto-updates
	UpdateCount   int                    `json:"updateCount"`   // Number of updates performed
	LastAssetIDs  []string               `json:"lastAssetIds"`  // Asset IDs from last update (for full-sync)
}

// EncodeToDescription converts metadata to JSON string for album description
func (m *LiveAlbumMetadata) EncodeToDescription() (string, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return "", fmt.Errorf("failed to marshal metadata: %w", err)
	}
	return string(data), nil
}

// DecodeFromDescription parses metadata from album description JSON
func DecodeFromDescription(description string) (*LiveAlbumMetadata, error) {
	var metadata LiveAlbumMetadata
	if err := json.Unmarshal([]byte(description), &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}
	return &metadata, nil
}

// IsLive checks if the description contains live album metadata
func IsLive(description string) bool {
	metadata, err := DecodeFromDescription(description)
	if err != nil {
		return false
	}
	return metadata.IsLiveAlbum
}

// NewSmartSearchMetadata creates metadata for a smart search-based live album
func NewSmartSearchMetadata(query string, syncStrategy string, maxResults int) *LiveAlbumMetadata {
	return &LiveAlbumMetadata{
		IsLiveAlbum:  true,
		SearchType:   "smart",
		SearchQuery:  query,
		SearchParams: nil,
		SyncStrategy: syncStrategy,
		MaxResults:   maxResults,
		LastUpdated:  time.Now(),
		Enabled:      true,
		UpdateCount:  0,
		LastAssetIDs: []string{},
	}
}

// NewAdvancedSearchMetadata creates metadata for an advanced search-based live album
func NewAdvancedSearchMetadata(params map[string]interface{}, syncStrategy string, maxResults int) *LiveAlbumMetadata {
	// Extract query from params if available
	query := ""
	if q, ok := params["query"].(string); ok {
		query = q
	}

	return &LiveAlbumMetadata{
		IsLiveAlbum:  true,
		SearchType:   "advanced",
		SearchQuery:  query,
		SearchParams: params,
		SyncStrategy: syncStrategy,
		MaxResults:   maxResults,
		LastUpdated:  time.Now(),
		Enabled:      true,
		UpdateCount:  0,
		LastAssetIDs: []string{},
	}
}

// Validate validates the metadata
func (m *LiveAlbumMetadata) Validate() error {
	if !m.IsLiveAlbum {
		return fmt.Errorf("not a live album")
	}

	if m.SearchType != "smart" && m.SearchType != "advanced" {
		return fmt.Errorf("invalid search type: %s (must be 'smart' or 'advanced')", m.SearchType)
	}

	if m.SearchType == "smart" && m.SearchQuery == "" {
		return fmt.Errorf("search query is required for smart search")
	}

	if m.SearchType == "advanced" && m.SearchParams == nil {
		return fmt.Errorf("search params are required for advanced search")
	}

	if m.SyncStrategy != "add-only" && m.SyncStrategy != "full-sync" {
		return fmt.Errorf("invalid sync strategy: %s (must be 'add-only' or 'full-sync')", m.SyncStrategy)
	}

	if m.MaxResults <= 0 {
		return fmt.Errorf("max results must be greater than 0")
	}

	return nil
}

// UpdateTimestamp updates the last updated timestamp and increments the update count
func (m *LiveAlbumMetadata) UpdateTimestamp() {
	m.LastUpdated = time.Now()
	m.UpdateCount++
}
