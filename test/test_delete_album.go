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
	if err := tools.RegisterTools(mcpServer, immichClient, cacheStore); err != nil {
		log.Fatal("failed to register tools: ", err)
	}

	ctx := context.Background()

	// SAFETY: First check what we would delete with a dry run
	fmt.Println("=== DRY RUN - Checking 'Bad Thumbnails' album contents ===")
	dryRunResult := callTool(ctx, mcpServer, "deleteAlbumContents", map[string]interface{}{
		"albumName": "Bad Thumbnails",
		"dryRun":    true,
		"maxAssets": 10, // Just check first 10 for safety
	})

	if dryRunResult == nil {
		fmt.Println("ERROR: No result returned from dry run")
		return
	}

	if result, ok := dryRunResult.(map[string]interface{}); ok {
		fmt.Printf("Album ID: %v\n", result["albumID"])
		fmt.Printf("Album Name: %v\n", result["albumName"])
		fmt.Printf("Total assets in album: %v\n", result["totalAssets"])
		fmt.Printf("Would delete: %v assets\n", result["assetsToDelete"])

		if samples, ok := result["sampleAssets"].([]interface{}); ok && len(samples) > 0 {
			fmt.Println("\nSample assets that would be deleted:")
			for i, sample := range samples {
				if asset, ok := sample.(map[string]interface{}); ok {
					fmt.Printf("  %d. %s (ID: %s, Type: %s)\n",
						i+1, asset["fileName"], asset["id"], asset["type"])
				}
			}
		}
	}

	// Prompt for confirmation
	fmt.Println("\n⚠️  WARNING: This will DELETE assets from the album!")
	fmt.Println("To actually delete, uncomment the deletion code below and run again.")

	// UNCOMMENT BELOW TO ACTUALLY DELETE (BE VERY CAREFUL!)
	/*
		fmt.Println("\n=== ACTUAL DELETION - Moving 10 assets to trash ===")
		deleteResult := callTool(ctx, mcpServer, "deleteAlbumContents", map[string]interface{}{
			"albumName":   "Bad Thumbnails",
			"dryRun":      false,
			"forceDelete": false, // false = move to trash (safer)
			"maxAssets":   10,    // Only delete 10 for testing
			"batchSize":   5,     // Delete in batches of 5
		})

		if deleteResult != nil {
			if result, ok := deleteResult.(map[string]interface{}); ok {
				fmt.Printf("\n✅ Operation completed!\n")
				fmt.Printf("Deleted: %v assets\n", result["deleted"])
				fmt.Printf("Failed: %v assets\n", result["failed"])
				fmt.Printf("Message: %v\n", result["message"])

				if errors, ok := result["errors"].([]interface{}); ok && len(errors) > 0 {
					fmt.Println("\nErrors encountered:")
					for _, err := range errors {
						fmt.Printf("  - %v\n", err)
					}
				}
			}
		}
	*/

	// Check album status after
	fmt.Println("\n=== VERIFICATION - Current album status ===")
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
			fmt.Printf("Album '%s' currently contains %d assets\n", album.AlbumName, album.AssetCount)
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
