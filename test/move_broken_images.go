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
	// Load config - try parent directory
	cfg, err := config.Load("../config.yaml")
	if err != nil {
		// Try current directory
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

	// First, do a dry run to see what we'll move
	fmt.Println("=== DRY RUN - Finding broken thumbnail images ===")
	dryRunResult := callTool(ctx, mcpServer, "moveBrokenThumbnailsToAlbum", map[string]interface{}{
		"albumName": "Bad Thumbnails",
		"dryRun":    true,
		"maxImages": 100,
	})

	if result, ok := dryRunResult.(map[string]interface{}); ok {
		fmt.Printf("Found %v images with no thumbhash\n", result["foundBrokenImages"])

		if samples, ok := result["sampleBrokenImages"].([]interface{}); ok && len(samples) > 0 {
			fmt.Println("\nSample broken images to be moved:")
			for i, sample := range samples {
				if asset, ok := sample.(map[string]interface{}); ok {
					fmt.Printf("  %d. ID: %s, Name: %s\n", i+1, asset["id"], asset["originalFileName"])
				}
			}
		}
	}

	// Now actually move them - let's try with 10 images, skipping ones that might be duplicates
	fmt.Println("\n=== ACTUAL RUN - Moving images to 'Bad Thumbnails' album ===")
	fmt.Println("Attempting to move 10 new images...")
	moveResult := callTool(ctx, mcpServer, "moveBrokenThumbnailsToAlbum", map[string]interface{}{
		"albumName":   "Bad Thumbnails",
		"dryRun":      false,
		"createAlbum": false, // Album already exists
		"maxImages":   50, // Try more to get some that aren't duplicates
	})

	if moveResult == nil {
		fmt.Println("ERROR: No result returned from tool")
		return
	}

	if result, ok := moveResult.(map[string]interface{}); ok {
		fmt.Printf("Raw result: %+v\n", result)

		if result["success"] == true {
			fmt.Println("\n✅ Successfully moved images!")
			fmt.Printf("Album: %v\n", result["albumName"])
			fmt.Printf("Album ID: %v\n", result["albumID"])
			fmt.Printf("Images moved: %v\n", result["movedCount"])
			fmt.Printf("Failed to move: %v\n", result["failedCount"])
			if result["albumCreated"] == true {
				fmt.Println("Note: Album was created")
			}
		} else {
			fmt.Printf("Result: %v\n", result["message"])
		}
	} else {
		fmt.Printf("ERROR: Could not parse result: %v\n", moveResult)
	}
}

func callTool(ctx context.Context, srv *server.MCPServer, toolName string, params interface{}) interface{} {
	// Marshal parameters
	argBytes, err := json.Marshal(params)
	if err != nil {
		fmt.Printf("Error marshaling params: %v\n", err)
		return nil
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
	reqBytes, err := json.Marshal(jsonRPCReq)
	if err != nil {
		fmt.Printf("Error marshaling request: %v\n", err)
		return nil
	}

	response := srv.HandleMessage(ctx, json.RawMessage(reqBytes))

	// Parse the response
	responseBytes, err := json.Marshal(response)
	if err != nil {
		fmt.Printf("Error marshaling response: %v\n", err)
		return nil
	}

	fmt.Printf("Raw response: %s\n\n", string(responseBytes))

	// First check if it's an error response
	var jsonRPCError mcp.JSONRPCError
	if err := json.Unmarshal(responseBytes, &jsonRPCError); err == nil && jsonRPCError.Error.Code != 0 {
		fmt.Printf("JSON-RPC error: %s (code: %d)\n", jsonRPCError.Error.Message, jsonRPCError.Error.Code)
		return nil
	}

	// Otherwise parse as success response
	var jsonRPCResp mcp.JSONRPCResponse
	var result mcp.CallToolResult

	if err := json.Unmarshal(responseBytes, &jsonRPCResp); err != nil {
		fmt.Printf("Error unmarshaling JSON-RPC response: %v\n", err)
		return nil
	}

	if jsonRPCResp.Result == nil {
		fmt.Printf("No result in JSON-RPC response\n")
		return nil
	}

	resultBytes, err := json.Marshal(jsonRPCResp.Result)
	if err != nil {
		fmt.Printf("Error marshaling result: %v\n", err)
		return nil
	}

	if err := json.Unmarshal(resultBytes, &result); err != nil {
		fmt.Printf("Error unmarshaling tool result: %v\n", err)
		return nil
	}

	if result.IsError {
		fmt.Printf("Tool returned error\n")
		return nil
	}

	if len(result.Content) == 0 {
		fmt.Printf("No content in tool result\n")
		return nil
	}

	if textContent, ok := mcp.AsTextContent(result.Content[0]); ok {
		var data interface{}
		if err := json.Unmarshal([]byte(textContent.Text), &data); err != nil {
			fmt.Printf("Error unmarshaling text content: %v\n", err)
			return nil
		}
		return data
	}

	fmt.Printf("Content is not text\n")
	return nil
}