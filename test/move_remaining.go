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

	fmt.Println("=== Moving ALL remaining broken thumbnail images ===")
	fmt.Println("This will process all pages to ensure we get everything...")

	// Move ALL broken images (0 = unlimited)
	moveResult := callTool(ctx, mcpServer, "moveBrokenThumbnailsToAlbum", map[string]interface{}{
		"albumName":   "Bad Thumbnails",
		"dryRun":      false,
		"createAlbum": false, // Album already exists
		"maxImages":   0, // 0 means unlimited - process all
		"startPage":   1, // Start from beginning
	})

	if moveResult == nil {
		fmt.Println("ERROR: No result returned from tool")
		return
	}

	if result, ok := moveResult.(map[string]interface{}); ok {
		if result["success"] == true {
			fmt.Println("\n✅ Successfully completed!")
			fmt.Printf("Album: %v\n", result["albumName"])
			fmt.Printf("Total images found with broken thumbnails: %v\n", result["foundBrokenImages"])
			fmt.Printf("Total assets processed: %v\n", result["totalProcessed"])
			fmt.Printf("Last page processed: %v\n", result["lastPage"])
			fmt.Printf("Successfully moved: %v\n", result["movedCount"])
			fmt.Printf("Failed (likely duplicates): %v\n", result["failedCount"])

			// Calculate new vs duplicate rate
			if moved, ok := result["movedCount"].(float64); ok {
				if failed, ok := result["failedCount"].(float64); ok {
					newImages := moved
					duplicates := failed
					fmt.Printf("\nNew images added: %.0f\n", newImages)
					fmt.Printf("Duplicates skipped: %.0f\n", duplicates)
				}
			}
		} else {
			fmt.Printf("Result: %v\n", result["message"])
		}
	} else {
		fmt.Printf("ERROR: Could not parse result: %v\n", moveResult)
	}

	// Verify final album state
	fmt.Println("\n=== FINAL VERIFICATION ===")
	verifyAlbum(cfg)
}

func verifyAlbum(cfg *config.Config) {
	immichClient := immich.NewClient(cfg.ImmichURL, cfg.ImmichAPIKey, 30*time.Second)
	ctx := context.Background()

	albums, err := immichClient.ListAlbums(ctx, false)
	if err != nil {
		fmt.Printf("Error listing albums: %v\n", err)
		return
	}

	for _, album := range albums {
		if album.AlbumName == "Bad Thumbnails" {
			fmt.Printf("✅ Album '%s' now contains %d total assets\n", album.AlbumName, album.AssetCount)
			return
		}
	}
	fmt.Println("Album 'Bad Thumbnails' not found")
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