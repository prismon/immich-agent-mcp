package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/patrickmn/go-cache"
	"github.com/yourusername/mcp-immich/pkg/config"
	"github.com/yourusername/mcp-immich/pkg/immich"
	"github.com/yourusername/mcp-immich/pkg/tools"
)

func main() {
	// Load config
	cfg, err := config.Load("../config.yaml")
	if err != nil {
		cfg, err = config.Load("config.yaml")
		if err != nil {
			log.Fatal("failed to read config: ", err)
		}
	}

	// Setup MCP server and tools
	immichClient := immich.NewClient(cfg.ImmichURL, cfg.ImmichAPIKey, 30*time.Second)
	cacheStore := cache.New(5*time.Minute, 10*time.Minute)
	mcpServer := server.NewMCPServer("test-server", "1.0.0")
	tools.RegisterTools(mcpServer, immichClient, cacheStore)

	ctx := context.Background()

	// Check current state of Large Movies album
	fmt.Println("=== CHECKING CURRENT STATE ===")
	initialCount := getAlbumCount(cfg, "Large Movies")
	fmt.Printf("Large Movies album currently contains %d assets\n", initialCount)

	// First, do a dry run to identify personal videos
	fmt.Println("\n=== DRY RUN - Identifying personal videos in Large Movies album ===")
	dryRunResult := callTool(ctx, mcpServer, "movePersonalVideosFromAlbum", map[string]interface{}{
		"sourceAlbum": "Large Movies",
		"targetAlbum": "Personal Videos",
		"dryRun":      true,
	})

	personalCount := 0
	if result, ok := dryRunResult.(map[string]interface{}); ok {
		if found, ok := result["personalVideosFound"].(float64); ok {
			personalCount = int(found)
		}
		fmt.Printf("Total videos in Large Movies: %v\n", result["totalVideosInSource"])
		fmt.Printf("Personal videos identified: %d\n", personalCount)

		// Show sample personal videos
		if samples, ok := result["samplePersonalVideos"].([]interface{}); ok && len(samples) > 0 {
			fmt.Println("\nSample personal videos to be moved:")
			for i, sample := range samples {
				if video, ok := sample.(map[string]interface{}); ok {
					fmt.Printf("  %d. %s (duration: %s, ID: %s)\n",
						i+1, video["name"], video["duration"], video["id"])
				}
			}
		}
	}

	if personalCount == 0 {
		fmt.Println("No personal videos found in Large Movies album!")
		return
	}

	// Move personal videos to Personal Videos album
	fmt.Printf("\n=== ACTUAL RUN - Moving %d personal videos to 'Personal Videos' album ===\n", personalCount)
	fmt.Println("This will also remove them from the Large Movies album...")

	moveResult := callTool(ctx, mcpServer, "movePersonalVideosFromAlbum", map[string]interface{}{
		"sourceAlbum":      "Large Movies",
		"targetAlbum":      "Personal Videos",
		"dryRun":           false,
		"createAlbum":      true,
		"removeFromSource": true,
	})

	if moveResult == nil {
		fmt.Println("ERROR: No result returned from tool")
		return
	}

	if result, ok := moveResult.(map[string]interface{}); ok {
		if result["success"] == true {
			fmt.Println("\n✅ Successfully completed!")
			fmt.Printf("Source album: %v\n", result["sourceAlbum"])
			fmt.Printf("Target album: %v\n", result["targetAlbum"])
			fmt.Printf("Personal videos found: %v\n", result["personalVideosFound"])
			fmt.Printf("Successfully moved: %v\n", result["movedCount"])
			fmt.Printf("Failed to move: %v\n", result["failedCount"])

			if removedCount, ok := result["removedFromSource"]; ok {
				fmt.Printf("Removed from source album: %v\n", removedCount)
			}

			if removeError, ok := result["removeError"]; ok {
				fmt.Printf("⚠️  Error removing from source: %v\n", removeError)
			}

			fmt.Printf("\nMessage: %v\n", result["message"])
		} else {
			fmt.Printf("Result: %v\n", result["message"])
		}
	} else {
		fmt.Printf("ERROR: Could not parse result: %v\n", moveResult)
	}

	// Verify final state
	fmt.Println("\n=== VERIFICATION - Final album states ===")
	finalLargeMovies := getAlbumCount(cfg, "Large Movies")
	personalVideos := getAlbumCount(cfg, "Personal Videos")

	fmt.Printf("Large Movies album now contains: %d assets (was %d)\n", finalLargeMovies, initialCount)
	fmt.Printf("Personal Videos album now contains: %d assets\n", personalVideos)
	fmt.Printf("Net change in Large Movies: %d assets removed\n", initialCount - finalLargeMovies)
}

func getAlbumCount(cfg *config.Config, albumName string) int {
	immichClient := immich.NewClient(cfg.ImmichURL, cfg.ImmichAPIKey, 30*time.Second)
	ctx := context.Background()

	albums, err := immichClient.ListAlbums(ctx, false)
	if err != nil {
		fmt.Printf("Error listing albums: %v\n", err)
		return -1
	}

	for _, album := range albums {
		if album.AlbumName == albumName {
			return album.AssetCount
		}
	}
	return 0
}

func callTool(ctx context.Context, srv *server.MCPServer, toolName string, params interface{}) interface{} {
	argBytes, err := json.Marshal(params)
	if err != nil {
		fmt.Printf("Error marshaling params: %v\n", err)
		return nil
	}

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

	reqBytes, err := json.Marshal(jsonRPCReq)
	if err != nil {
		fmt.Printf("Error marshaling request: %v\n", err)
		return nil
	}

	response := srv.HandleMessage(ctx, json.RawMessage(reqBytes))
	responseBytes, _ := json.Marshal(response)

	// Check for error response
	var jsonRPCError mcp.JSONRPCError
	if err := json.Unmarshal(responseBytes, &jsonRPCError); err == nil && jsonRPCError.Error.Code != 0 {
		fmt.Printf("Error: %s\n", jsonRPCError.Error.Message)
		return nil
	}

	// Parse success response
	var jsonRPCResp mcp.JSONRPCResponse
	if err := json.Unmarshal(responseBytes, &jsonRPCResp); err == nil && jsonRPCResp.Result != nil {
		resultBytes, _ := json.Marshal(jsonRPCResp.Result)
		var result mcp.CallToolResult
		if err := json.Unmarshal(resultBytes, &result); err == nil {
			if !result.IsError && len(result.Content) > 0 {
				if textContent, ok := mcp.AsTextContent(result.Content[0]); ok {
					var data interface{}
					if err := json.Unmarshal([]byte(textContent.Text), &data); err == nil {
						return data
					}
				}
			}
		}
	}

	return nil
}