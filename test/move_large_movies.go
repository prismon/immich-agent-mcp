//go:build ignore

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

	// First, do a dry run to count all large movies
	fmt.Println("=== DRY RUN - Counting all large movies (>20 minutes) ===")
	dryRunResult := callTool(ctx, mcpServer, "moveLargeMoviesToAlbum", map[string]interface{}{
		"albumName":   "Large Movies",
		"minDuration": 20,
		"dryRun":      true,
		"maxVideos":   0, // 0 means unlimited - count all
	})

	totalLarge := 0
	if result, ok := dryRunResult.(map[string]interface{}); ok {
		if found, ok := result["foundLargeMovies"].(float64); ok {
			totalLarge = int(found)
		}
		fmt.Printf("Found %d movies over 20 minutes\n", totalLarge)
		fmt.Printf("Total assets processed: %v\n", result["totalProcessed"])
		fmt.Printf("Last page: %v\n", result["lastPage"])

		// Show sample large movies
		if samples, ok := result["sampleLargeMovies"].([]interface{}); ok && len(samples) > 0 {
			fmt.Println("\nSample large movies to be moved:")
			for i, sample := range samples {
				if movie, ok := sample.(map[string]interface{}); ok {
					fmt.Printf("  %d. %s (duration: %s, ~%v minutes, ID: %s)\n",
						i+1, movie["name"], movie["duration"], movie["minutes"], movie["id"])
				}
			}
		}
	}

	if totalLarge == 0 {
		fmt.Println("No large movies found!")
		return
	}

	// Move ALL large movies
	fmt.Printf("\n=== ACTUAL RUN - Moving ALL %d large movies to 'Large Movies' album ===\n", totalLarge)
	fmt.Println("This may take a moment...")

	moveResult := callTool(ctx, mcpServer, "moveLargeMoviesToAlbum", map[string]interface{}{
		"albumName":   "Large Movies",
		"minDuration": 20,
		"dryRun":      false,
		"createAlbum": true,
		"maxVideos":   0, // 0 means unlimited - process all
		"startPage":   1,
	})

	if moveResult == nil {
		fmt.Println("ERROR: No result returned from tool")
		return
	}

	if result, ok := moveResult.(map[string]interface{}); ok {
		if result["success"] == true {
			fmt.Println("\n✅ Successfully completed!")
			fmt.Printf("Album: %v\n", result["albumName"])
			fmt.Printf("Album ID: %v\n", result["albumID"])
			fmt.Printf("Total large movies found: %v\n", result["foundLargeMovies"])
			fmt.Printf("Total assets processed: %v\n", result["totalProcessed"])
			fmt.Printf("Successfully moved: %v\n", result["movedCount"])
			fmt.Printf("Failed (likely duplicates): %v\n", result["failedCount"])

			// Calculate success rate
			if moved, ok := result["movedCount"].(float64); ok {
				if failed, ok := result["failedCount"].(float64); ok {
					if moved+failed > 0 {
						successRate := (moved / (moved + failed)) * 100
						fmt.Printf("\nSuccess rate: %.1f%%\n", successRate)
					}
				}
			}
		} else {
			fmt.Printf("Result: %v\n", result["message"])
		}
	} else {
		fmt.Printf("ERROR: Could not parse result: %v\n", moveResult)
	}

	// Verify final album state
	fmt.Println("\n=== VERIFICATION - Checking album contents ===")
	verifyAlbum(cfg, "Large Movies")
}

func verifyAlbum(cfg *config.Config, albumName string) {
	immichClient := immich.NewClient(cfg.ImmichURL, cfg.ImmichAPIKey, 30*time.Second)
	ctx := context.Background()

	albums, err := immichClient.ListAlbums(ctx, false)
	if err != nil {
		fmt.Printf("Error listing albums: %v\n", err)
		return
	}

	for _, album := range albums {
		if album.AlbumName == albumName {
			fmt.Printf("✅ Album '%s' now contains %d assets\n", album.AlbumName, album.AssetCount)
			return
		}
	}
	fmt.Printf("Album '%s' not found\n", albumName)
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
