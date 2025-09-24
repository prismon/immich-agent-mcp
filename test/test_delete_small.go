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

	// Check initial count
	fmt.Println("=== INITIAL STATUS ===")
	initialCount := getAlbumCount(cfg, "Bad Thumbnails")
	fmt.Printf("Album 'Bad Thumbnails' contains %d assets\n", initialCount)

	// Delete just 2 assets to trash (safer)
	fmt.Println("\n=== DELETING 2 ASSETS TO TRASH ===")
	deleteResult := callTool(ctx, mcpServer, "deleteAlbumContents", map[string]interface{}{
		"albumName":   "Bad Thumbnails",
		"dryRun":      false,
		"forceDelete": false, // Move to trash, not permanent delete
		"maxAssets":   2,     // Only delete 2 for testing
		"batchSize":   2,
	})

	if deleteResult != nil {
		if result, ok := deleteResult.(map[string]interface{}); ok {
			fmt.Printf("✅ Operation completed!\n")
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

	// Check final count
	fmt.Println("\n=== FINAL STATUS ===")
	finalCount := getAlbumCount(cfg, "Bad Thumbnails")
	fmt.Printf("Album 'Bad Thumbnails' now contains %d assets\n", finalCount)
	fmt.Printf("Difference: %d assets removed\n", initialCount-finalCount)
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