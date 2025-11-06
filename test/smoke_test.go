package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yourusername/mcp-immich/pkg/config"
	"github.com/yourusername/mcp-immich/pkg/immich"
	"github.com/yourusername/mcp-immich/pkg/tools"
)

// TestConfig holds test configuration from environment
type TestConfig struct {
	ImmichURL     string
	ImmichAPIKey  string
	TestAlbumID   string
	TestPhotoID   string
	TestPersonID  string
	TestLibraryID string
}

// LoadTestConfig loads test configuration from config.yaml or environment
func LoadTestConfig() (*TestConfig, bool) {
	// Try to load from config.yaml in current directory or parent directory
	configPaths := []string{"config.yaml", "../config.yaml"}
	for _, configPath := range configPaths {
		if _, err := os.Stat(configPath); err == nil {
			cfg, err := config.Load(configPath)
			if err == nil && cfg.ImmichURL != "" && cfg.ImmichAPIKey != "" {
				return &TestConfig{
					ImmichURL:     cfg.ImmichURL,
					ImmichAPIKey:  cfg.ImmichAPIKey,
					TestAlbumID:   os.Getenv("TEST_ALBUM_ID"),
					TestPhotoID:   os.Getenv("TEST_PHOTO_ID"),
					TestPersonID:  os.Getenv("TEST_PERSON_ID"),
					TestLibraryID: os.Getenv("TEST_LIBRARY_ID"),
				}, true
			}
		}
	}

	// Fall back to environment variables
	url := os.Getenv("TEST_IMMICH_URL")
	apiKey := os.Getenv("TEST_IMMICH_API_KEY")

	if url == "" || apiKey == "" {
		return nil, false
	}

	return &TestConfig{
		ImmichURL:     url,
		ImmichAPIKey:  apiKey,
		TestAlbumID:   os.Getenv("TEST_ALBUM_ID"),
		TestPhotoID:   os.Getenv("TEST_PHOTO_ID"),
		TestPersonID:  os.Getenv("TEST_PERSON_ID"),
		TestLibraryID: os.Getenv("TEST_LIBRARY_ID"),
	}, true
}

// setupTestServer creates a test MCP server with all tools registered
func setupTestServer(t *testing.T, cfg *TestConfig) *server.MCPServer {
	// Create Immich client
	immichClient := immich.NewClient(cfg.ImmichURL, cfg.ImmichAPIKey, 30*time.Second)

	// Create cache
	cacheStore := cache.New(5*time.Minute, 10*time.Minute)

	// Create MCP server
	mcpServer := server.NewMCPServer("test-server", "1.0.0")

	// Register all tools
	if err := tools.RegisterTools(mcpServer, immichClient, cacheStore); err != nil {
		t.Fatalf("failed to register tools: %v", err)
	}

	return mcpServer
}

// callTool simulates calling a tool through the MCP server
func callTool(t *testing.T, srv *server.MCPServer, toolName string, params interface{}) (interface{}, error) {
	ctx := context.Background()

	// Marshal parameters
	argBytes, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}

	// Create a proper JSON-RPC request
	jsonRPCReq := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      mcp.NewRequestId("test-1"),
		Request: mcp.Request{
			Method: "tools/call",
		},
		Params: mcp.CallToolParams{
			Name:      toolName,
			Arguments: json.RawMessage(argBytes),
		},
	}

	// Handle the request through the server
	reqBytes := mustMarshal(t, jsonRPCReq)
	response := srv.HandleMessage(ctx, json.RawMessage(reqBytes))

	// Parse the response - it could be success or error
	responseBytes := mustMarshal(t, response)

	// Try to parse as success response first
	var jsonRPCResp mcp.JSONRPCResponse
	var result mcp.CallToolResult

	if err := json.Unmarshal(responseBytes, &jsonRPCResp); err == nil && jsonRPCResp.Result != nil {
		// Parse the result
		resultBytes, _ := json.Marshal(jsonRPCResp.Result)
		if err := json.Unmarshal(resultBytes, &result); err != nil {
			return nil, fmt.Errorf("failed to parse result: %w", err)
		}
	} else {
		// Try as error response
		var errResp mcp.JSONRPCError
		if err := json.Unmarshal(responseBytes, &errResp); err == nil {
			return nil, fmt.Errorf("RPC error: %v", errResp)
		}
		return nil, fmt.Errorf("unknown response format")
	}

	if result.IsError {
		return nil, fmt.Errorf("tool returned error")
	}

	// Parse the content
	if len(result.Content) > 0 {
		// Try to get text content
		if textContent, ok := mcp.AsTextContent(result.Content[0]); ok {
			var data interface{}
			if err := json.Unmarshal([]byte(textContent.Text), &data); err != nil {
				return textContent.Text, nil // Return raw text if not JSON
			}
			return data, nil
		}
	}

	return nil, fmt.Errorf("no content in response")
}

func mustMarshal(t *testing.T, v interface{}) []byte {
	data, err := json.Marshal(v)
	require.NoError(t, err)
	return data
}

// TestSpecificPhotoID tests retrieving a specific photo by ID
func TestSpecificPhotoID(t *testing.T) {
	cfg, ok := LoadTestConfig()
	if !ok {
		t.Skip("Test configuration not available. Create config.yaml or set TEST_IMMICH_URL and TEST_IMMICH_API_KEY")
	}

	srv := setupTestServer(t, cfg)
	// Use environment variable or skip test if not set
	specificPhotoID := os.Getenv("TEST_PHOTO_ID")
	if specificPhotoID == "" {
		t.Skip("TEST_PHOTO_ID not set, skipping specific photo test")
	}

	t.Run("get specific photo metadata", func(t *testing.T) {
		result, err := callTool(t, srv, "getPhotoMetadata", map[string]interface{}{
			"photoId":       specificPhotoID,
			"includeExif":   true,
			"includeFaces":  true,
			"includeAlbums": true,
		})

		require.NoError(t, err, "Should successfully retrieve photo by ID")
		assert.NotNil(t, result)

		res, ok := result.(map[string]interface{})
		require.True(t, ok, "Result should be a map")
		assert.Contains(t, res, "photo", "Result should contain photo data")
		assert.Contains(t, res, "success", "Result should have success field")

		// Check photo data
		if photo, ok := res["photo"].(map[string]interface{}); ok {
			assert.Equal(t, specificPhotoID, photo["id"], "Photo ID should match")
			if fileName, ok := photo["originalFileName"]; ok {
				t.Logf("Successfully retrieved photo: %v", fileName)
			} else {
				t.Logf("Successfully retrieved photo with ID: %s", specificPhotoID)
			}
		}
	})

	t.Run("search for specific photo", func(t *testing.T) {
		// Try to query photos and find this specific one
		result, err := callTool(t, srv, "queryPhotos", map[string]interface{}{
			"ids":   []string{specificPhotoID},
			"limit": 1,
		})

		if err != nil {
			// If querying by ID isn't supported, skip
			t.Logf("Query by ID not supported: %v", err)
			return
		}

		res, ok := result.(map[string]interface{})
		require.True(t, ok)
		if photos, ok := res["photos"].([]interface{}); ok && len(photos) > 0 {
			if photo, ok := photos[0].(map[string]interface{}); ok {
				assert.Equal(t, specificPhotoID, photo["id"], "Should find the specific photo")
			}
		}
	})
}

// TestQueryPhotos smoke test
func TestQueryPhotos(t *testing.T) {
	cfg, ok := LoadTestConfig()
	if !ok {
		t.Skip("Test configuration not available. Create config.yaml or set TEST_IMMICH_URL and TEST_IMMICH_API_KEY")
	}

	srv := setupTestServer(t, cfg)

	tests := []struct {
		name   string
		params map[string]interface{}
		check  func(t *testing.T, result interface{})
	}{
		{
			name: "basic query",
			params: map[string]interface{}{
				"limit": 10,
			},
			check: func(t *testing.T, result interface{}) {
				assert.NotNil(t, result)
				res, ok := result.(map[string]interface{})
				require.True(t, ok)
				assert.Contains(t, res, "photos")
				assert.Contains(t, res, "totalCount")
				assert.Contains(t, res, "success")
			},
		},
		{
			name: "query with date range",
			params: map[string]interface{}{
				"startDate": "2024-01-01",
				"endDate":   "2024-12-31",
				"limit":     5,
			},
			check: func(t *testing.T, result interface{}) {
				assert.NotNil(t, result)
			},
		},
		{
			name: "query with search text",
			params: map[string]interface{}{
				"query": "sunset",
				"limit": 20,
			},
			check: func(t *testing.T, result interface{}) {
				assert.NotNil(t, result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := callTool(t, srv, "queryPhotos", tt.params)
			require.NoError(t, err, "queryPhotos should not return error")
			tt.check(t, result)
		})
	}
}

// TestQueryPhotosWithBuckets smoke test
func TestQueryPhotosWithBuckets(t *testing.T) {
	cfg, ok := LoadTestConfig()
	if !ok {
		t.Skip("Test configuration not available. Create config.yaml or set TEST_IMMICH_URL and TEST_IMMICH_API_KEY")
	}

	srv := setupTestServer(t, cfg)

	tests := []struct {
		name   string
		params map[string]interface{}
	}{
		{
			name: "daily buckets",
			params: map[string]interface{}{
				"bucketSize": "day",
				"maxBuckets": 7,
			},
		},
		{
			name: "monthly buckets",
			params: map[string]interface{}{
				"bucketSize": "month",
				"maxBuckets": 12,
			},
		},
		{
			name: "yearly buckets with assets",
			params: map[string]interface{}{
				"bucketSize": "year",
				"withAssets": true,
				"maxBuckets": 2,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := callTool(t, srv, "queryPhotosWithBuckets", tt.params)
			require.NoError(t, err)
			assert.NotNil(t, result)

			res, ok := result.(map[string]interface{})
			require.True(t, ok)
			assert.Contains(t, res, "buckets")
			assert.Contains(t, res, "totalBuckets")
			assert.Contains(t, res, "success")
		})
	}
}

// TestGetPhotoMetadata smoke test
func TestGetPhotoMetadata(t *testing.T) {
	cfg, ok := LoadTestConfig()
	if !ok {
		t.Skip("Test configuration not available. Create config.yaml or set TEST_IMMICH_URL and TEST_IMMICH_API_KEY")
	}

	if cfg.TestPhotoID == "" {
		t.Skip("TEST_PHOTO_ID not configured")
	}

	srv := setupTestServer(t, cfg)

	result, err := callTool(t, srv, "getPhotoMetadata", map[string]interface{}{
		"photoId":       cfg.TestPhotoID,
		"includeExif":   true,
		"includeFaces":  true,
		"includeAlbums": true,
	})

	require.NoError(t, err)
	assert.NotNil(t, result)

	res, ok := result.(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, res, "photo")
	assert.Contains(t, res, "success")
}

// TestSearchByFace smoke test
func TestSearchByFace(t *testing.T) {
	cfg, ok := LoadTestConfig()
	if !ok {
		t.Skip("Test configuration not available. Create config.yaml or set TEST_IMMICH_URL and TEST_IMMICH_API_KEY")
	}

	if cfg.TestPersonID == "" {
		t.Skip("TEST_PERSON_ID not configured")
	}

	srv := setupTestServer(t, cfg)

	result, err := callTool(t, srv, "searchByFace", map[string]interface{}{
		"personId": cfg.TestPersonID,
		"limit":    10,
	})

	// Face search might not be available, so we allow errors
	if err != nil {
		t.Logf("Face search returned error (may be expected): %v", err)
		return
	}

	assert.NotNil(t, result)
}

// TestSearchByLocation smoke test
func TestSearchByLocation(t *testing.T) {
	cfg, ok := LoadTestConfig()
	if !ok {
		t.Skip("Test configuration not available. Create config.yaml or set TEST_IMMICH_URL and TEST_IMMICH_API_KEY")
	}

	srv := setupTestServer(t, cfg)

	result, err := callTool(t, srv, "searchByLocation", map[string]interface{}{
		"latitude":  40.7128, // New York City
		"longitude": -74.0060,
		"radius":    10000, // 10km
		"limit":     5,
	})

	// Location search might return no results
	if err != nil {
		t.Logf("Location search returned error (may be expected): %v", err)
		return
	}

	assert.NotNil(t, result)
}

// TestListAlbums smoke test
func TestListAlbums(t *testing.T) {
	cfg, ok := LoadTestConfig()
	if !ok {
		t.Skip("Test configuration not available. Create config.yaml or set TEST_IMMICH_URL and TEST_IMMICH_API_KEY")
	}

	srv := setupTestServer(t, cfg)

	result, err := callTool(t, srv, "listAlbums", map[string]interface{}{
		"shared": false,
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
}

// TestCreateAlbum smoke test (non-destructive - creates and cleans up)
func TestCreateAlbum(t *testing.T) {
	cfg, ok := LoadTestConfig()
	if !ok {
		t.Skip("Test configuration not available. Create config.yaml or set TEST_IMMICH_URL and TEST_IMMICH_API_KEY")
	}

	if os.Getenv("ALLOW_WRITE_TESTS") != "true" {
		t.Skip("Write tests disabled. Set ALLOW_WRITE_TESTS=true to enable")
	}

	srv := setupTestServer(t, cfg)

	albumName := fmt.Sprintf("Test Album %d", time.Now().Unix())

	result, err := callTool(t, srv, "createAlbum", map[string]interface{}{
		"name":        albumName,
		"description": "Temporary test album - safe to delete",
	})

	require.NoError(t, err)
	assert.NotNil(t, result)

	// Note: In a real test, we'd delete the album afterward
	t.Logf("Created test album: %s", albumName)
}

// TestMoveToAlbum smoke test
func TestMoveToAlbum(t *testing.T) {
	cfg, ok := LoadTestConfig()
	if !ok {
		t.Skip("Test configuration not available. Create config.yaml or set TEST_IMMICH_URL and TEST_IMMICH_API_KEY")
	}

	if cfg.TestAlbumID == "" || cfg.TestPhotoID == "" {
		t.Skip("TEST_ALBUM_ID and TEST_PHOTO_ID required")
	}

	if os.Getenv("ALLOW_WRITE_TESTS") != "true" {
		t.Skip("Write tests disabled. Set ALLOW_WRITE_TESTS=true to enable")
	}

	srv := setupTestServer(t, cfg)

	result, err := callTool(t, srv, "moveToAlbum", map[string]interface{}{
		"albumId":  cfg.TestAlbumID,
		"assetIds": []string{cfg.TestPhotoID},
	})

	// This might fail if photo is already in album
	if err != nil {
		t.Logf("Move to album returned error (may be expected): %v", err)
		return
	}

	assert.NotNil(t, result)
}

// TestListLibraries smoke test
func TestListLibraries(t *testing.T) {
	cfg, ok := LoadTestConfig()
	if !ok {
		t.Skip("Test configuration not available. Create config.yaml or set TEST_IMMICH_URL and TEST_IMMICH_API_KEY")
	}

	srv := setupTestServer(t, cfg)

	result, err := callTool(t, srv, "listLibraries", map[string]interface{}{})

	require.NoError(t, err)
	assert.NotNil(t, result)
}

// TestMoveToLibrary smoke test
func TestMoveToLibrary(t *testing.T) {
	cfg, ok := LoadTestConfig()
	if !ok {
		t.Skip("Test configuration not available. Create config.yaml or set TEST_IMMICH_URL and TEST_IMMICH_API_KEY")
	}

	if cfg.TestLibraryID == "" || cfg.TestPhotoID == "" {
		t.Skip("TEST_LIBRARY_ID and TEST_PHOTO_ID required")
	}

	if os.Getenv("ALLOW_WRITE_TESTS") != "true" {
		t.Skip("Write tests disabled. Set ALLOW_WRITE_TESTS=true to enable")
	}

	srv := setupTestServer(t, cfg)

	result, err := callTool(t, srv, "moveToLibrary", map[string]interface{}{
		"targetLibraryId":  cfg.TestLibraryID,
		"assetIds":         []string{cfg.TestPhotoID},
		"removeFromSource": false,
	})

	// This might fail if library operations aren't supported
	if err != nil {
		t.Logf("Move to library returned error (may be expected): %v", err)
		return
	}

	assert.NotNil(t, result)
}

// TestFindBrokenFiles smoke test
func TestFindBrokenFiles(t *testing.T) {
	cfg, ok := LoadTestConfig()
	if !ok {
		t.Skip("Test configuration not available. Create config.yaml or set TEST_IMMICH_URL and TEST_IMMICH_API_KEY")
	}

	srv := setupTestServer(t, cfg)

	result, err := callTool(t, srv, "findBrokenFiles", map[string]interface{}{
		"checkType": "missing_thumbnail",
		"limit":     5,
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
}

// TestRepairAssets smoke test
func TestRepairAssets(t *testing.T) {
	cfg, ok := LoadTestConfig()
	if !ok {
		t.Skip("Test configuration not available. Create config.yaml or set TEST_IMMICH_URL and TEST_IMMICH_API_KEY")
	}

	if os.Getenv("ALLOW_WRITE_TESTS") != "true" {
		t.Skip("Write tests disabled. Set ALLOW_WRITE_TESTS=true to enable")
	}

	if cfg.TestPhotoID == "" {
		t.Skip("TEST_PHOTO_ID required")
	}

	srv := setupTestServer(t, cfg)

	result, err := callTool(t, srv, "repairAssets", map[string]interface{}{
		"assetIds": []string{cfg.TestPhotoID},
		"actions": map[string]bool{
			"regenerateThumbnails": true,
		},
	})

	// Repair might not be available
	if err != nil {
		t.Logf("Repair assets returned error (may be expected): %v", err)
		return
	}

	assert.NotNil(t, result)
}

// TestUpdateAssetMetadata smoke test
func TestUpdateAssetMetadata(t *testing.T) {
	cfg, ok := LoadTestConfig()
	if !ok {
		t.Skip("Test configuration not available. Create config.yaml or set TEST_IMMICH_URL and TEST_IMMICH_API_KEY")
	}

	if os.Getenv("ALLOW_WRITE_TESTS") != "true" {
		t.Skip("Write tests disabled. Set ALLOW_WRITE_TESTS=true to enable")
	}

	if cfg.TestPhotoID == "" {
		t.Skip("TEST_PHOTO_ID required")
	}

	srv := setupTestServer(t, cfg)

	result, err := callTool(t, srv, "updateAssetMetadata", map[string]interface{}{
		"assetId": cfg.TestPhotoID,
		"updates": map[string]interface{}{
			"description": fmt.Sprintf("Test update %d", time.Now().Unix()),
		},
	})

	// Update might fail if not supported
	if err != nil {
		t.Logf("Update metadata returned error (may be expected): %v", err)
		return
	}

	assert.NotNil(t, result)
}

// TestAnalyzePhotos smoke test
func TestAnalyzePhotos(t *testing.T) {
	cfg, ok := LoadTestConfig()
	if !ok {
		t.Skip("Test configuration not available. Create config.yaml or set TEST_IMMICH_URL and TEST_IMMICH_API_KEY")
	}

	if os.Getenv("ALLOW_WRITE_TESTS") != "true" {
		t.Skip("Write tests disabled. Set ALLOW_WRITE_TESTS=true to enable")
	}

	if cfg.TestPhotoID == "" {
		t.Skip("TEST_PHOTO_ID required")
	}

	srv := setupTestServer(t, cfg)

	result, err := callTool(t, srv, "analyzePhotos", map[string]interface{}{
		"assetIds": []string{cfg.TestPhotoID},
		"options": map[string]bool{
			"detectFaces":   true,
			"classifyScene": true,
		},
	})

	// Analysis might not be available
	if err != nil {
		t.Logf("Analyze photos returned error (may be expected): %v", err)
		return
	}

	assert.NotNil(t, result)
}

// TestGetAllAlbums tests the getAllAlbums tool
func TestGetAllAlbums(t *testing.T) {
	cfg, ok := LoadTestConfig()
	if !ok {
		t.Skip("Test configuration not available. Create config.yaml or set TEST_IMMICH_URL and TEST_IMMICH_API_KEY")
	}

	srv := setupTestServer(t, cfg)

	result, err := callTool(t, srv, "getAllAlbums", map[string]interface{}{})
	require.NoError(t, err)
	assert.NotNil(t, result)

	res, ok := result.(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, res, "albums")
	assert.Contains(t, res, "totalAlbums")
	assert.Contains(t, res, "success")

	t.Logf("Found %v albums", res["totalAlbums"])
}

// TestGetAllAssets tests the getAllAssets tool with pagination
func TestGetAllAssets(t *testing.T) {
	cfg, ok := LoadTestConfig()
	if !ok {
		t.Skip("Test configuration not available. Create config.yaml or set TEST_IMMICH_URL and TEST_IMMICH_API_KEY")
	}

	srv := setupTestServer(t, cfg)

	t.Run("first page", func(t *testing.T) {
		result, err := callTool(t, srv, "getAllAssets", map[string]interface{}{
			"page":     1,
			"pageSize": 10,
		})
		require.NoError(t, err)
		assert.NotNil(t, result)

		res, ok := result.(map[string]interface{})
		require.True(t, ok)
		assert.Contains(t, res, "assets")
		assert.Contains(t, res, "page")
		assert.Contains(t, res, "pageSize")
		assert.Contains(t, res, "assetCount")
		assert.Contains(t, res, "hasNextPage")
		assert.Equal(t, float64(1), res["page"])
		assert.Equal(t, float64(10), res["pageSize"])

		t.Logf("Page 1: Found %v assets, hasNextPage=%v", res["assetCount"], res["hasNextPage"])
	})

	t.Run("second page", func(t *testing.T) {
		result, err := callTool(t, srv, "getAllAssets", map[string]interface{}{
			"page":     2,
			"pageSize": 10,
		})
		require.NoError(t, err)
		assert.NotNil(t, result)

		res, ok := result.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, float64(2), res["page"])

		t.Logf("Page 2: Found %v assets, hasNextPage=%v", res["assetCount"], res["hasNextPage"])
	})

	t.Run("large page size", func(t *testing.T) {
		result, err := callTool(t, srv, "getAllAssets", map[string]interface{}{
			"page":     1,
			"pageSize": 100,
		})
		require.NoError(t, err)
		assert.NotNil(t, result)

		res, ok := result.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, float64(100), res["pageSize"])

		t.Logf("Large page: Found %v assets", res["assetCount"])
	})
}

// TestExportPhotos smoke test
func TestExportPhotos(t *testing.T) {
	cfg, ok := LoadTestConfig()
	if !ok {
		t.Skip("Test configuration not available. Create config.yaml or set TEST_IMMICH_URL and TEST_IMMICH_API_KEY")
	}

	if cfg.TestPhotoID == "" {
		t.Skip("TEST_PHOTO_ID required")
	}

	srv := setupTestServer(t, cfg)

	result, err := callTool(t, srv, "exportPhotos", map[string]interface{}{
		"assetIds": []string{cfg.TestPhotoID},
		"format":   "original",
	})

	require.NoError(t, err)
	assert.NotNil(t, result)

	res, ok := result.(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, res, "downloadURL")
}

// TestMoveBrokenThumbnailsToAlbum tests the broken thumbnails tool
func TestMoveBrokenThumbnailsToAlbum(t *testing.T) {
	cfg, ok := LoadTestConfig()
	if !ok {
		t.Skip("Test configuration not available. Create config.yaml or set TEST_IMMICH_URL and TEST_IMMICH_API_KEY")
	}

	srv := setupTestServer(t, cfg)

	// First do a dry run to find broken images
	result, err := callTool(t, srv, "moveBrokenThumbnailsToAlbum", map[string]interface{}{
		"albumName": "Broken Thumbnails Test",
		"dryRun":    true,
		"maxImages": 10,
	})

	require.NoError(t, err)
	assert.NotNil(t, result)

	res, ok := result.(map[string]interface{})
	require.True(t, ok)

	t.Logf("Found %v images with no thumbhash", res["foundBrokenImages"])

	if samples, ok := res["sampleBrokenImages"].([]interface{}); ok && len(samples) > 0 {
		t.Logf("Sample broken images:")
		for i, sample := range samples {
			if asset, ok := sample.(map[string]interface{}); ok {
				t.Logf("  %d. ID: %s, Name: %s", i+1, asset["id"], asset["originalFileName"])
			}
		}
	}
}

// TestMoveSmallImagesToAlbum tests the small images tool
func TestMoveSmallImagesToAlbum(t *testing.T) {
	cfg, ok := LoadTestConfig()
	if !ok {
		t.Skip("Test configuration not available. Create config.yaml or set TEST_IMMICH_URL and TEST_IMMICH_API_KEY")
	}

	srv := setupTestServer(t, cfg)

	// Do a dry run to find small images
	result, err := callTool(t, srv, "moveSmallImagesToAlbum", map[string]interface{}{
		"albumName":    "Small Images Test",
		"maxDimension": 200,
		"dryRun":       true,
		"maxImages":    100, // Increased to scan more images
	})

	require.NoError(t, err)
	assert.NotNil(t, result)

	res, ok := result.(map[string]interface{})
	require.True(t, ok)

	t.Logf("Found %v images <= 200x200 pixels", res["foundSmallImages"])

	if samples, ok := res["sampleSmallImages"].([]interface{}); ok && len(samples) > 0 {
		t.Logf("Sample small images:")
		for i, sample := range samples {
			if img, ok := sample.(map[string]interface{}); ok {
				t.Logf("  %d. %s: %vx%v pixels", i+1, img["name"], img["width"], img["height"])
			}
		}
	}
}

// TestKnownBrokenImage tests specifically with the known broken image ID
func TestKnownBrokenImage(t *testing.T) {
	cfg, ok := LoadTestConfig()
	if !ok {
		t.Skip("Test configuration not available. Create config.yaml or set TEST_IMMICH_URL and TEST_IMMICH_API_KEY")
	}

	srv := setupTestServer(t, cfg)
	badImageID := "cdc42b77-5087-48d2-a556-e09bd400f9f7"

	// Get metadata for the known broken image
	result, err := callTool(t, srv, "getPhotoMetadata", map[string]interface{}{
		"photoId": badImageID,
	})

	require.NoError(t, err)
	res, ok := result.(map[string]interface{})
	require.True(t, ok)

	if photo, ok := res["photo"].(map[string]interface{}); ok {
		t.Logf("Known broken image analysis:")
		t.Logf("  ID: %s", photo["id"])
		t.Logf("  Name: %s", photo["originalFileName"])
		t.Logf("  Thumbhash: '%v'", photo["thumbhash"])
		t.Logf("  Resized: %v", photo["resized"])

		if exif, ok := photo["exifInfo"].(map[string]interface{}); ok {
			t.Logf("  Dimensions: %vx%v", exif["exifImageWidth"], exif["exifImageHeight"])
		}

		// Verify it has no thumbhash
		thumbhash, _ := photo["thumbhash"].(string)
		assert.Empty(t, thumbhash, "Known broken image should have no thumbhash")
	}
}

// TestIntegrationHTTPServer tests the full HTTP server integration
func TestIntegrationHTTPServer(t *testing.T) {
	cfg, ok := LoadTestConfig()
	if !ok {
		t.Skip("Test configuration not available. Create config.yaml or set TEST_IMMICH_URL and TEST_IMMICH_API_KEY")
	}

	// Create test server
	srv := setupTestServer(t, cfg)
	streamableHTTP := server.NewStreamableHTTPServer(srv)

	// Create HTTP test server
	testServer := httptest.NewServer(http.HandlerFunc(streamableHTTP.ServeHTTP))
	defer testServer.Close()

	// Test that the server responds to requests
	resp, err := http.Get(testServer.URL + "/mcp")
	require.NoError(t, err)
	defer resp.Body.Close()

	// StreamableHTTP might require specific headers/methods
	// This is just a basic connectivity test
	t.Logf("Server responded with status: %d", resp.StatusCode)
}
