//go:build ignore

package main

import (
	"context"
	"encoding/json"
	"flag"
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
	var query string
	var albumName string
	var maxResults int
	var dryRun bool

	flag.StringVar(&query, "query", "sunset", "Smart search query")
	flag.StringVar(&albumName, "album", "Sunset Photos", "Album name to create/use")
	flag.IntVar(&maxResults, "max", 100, "Maximum results to move")
	flag.BoolVar(&dryRun, "dryrun", true, "Dry run mode")
	flag.Parse()

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

	fmt.Printf("Testing smart search with query: '%s'\n", query)
	fmt.Printf("Album: %s\n", albumName)
	fmt.Printf("Max results: %d\n", maxResults)
	fmt.Printf("Dry run: %v\n\n", dryRun)

	result := callTool(ctx, mcpServer, "movePhotosBySearch", map[string]interface{}{
		"query":      query,
		"albumName":  albumName,
		"maxResults": maxResults,
		"dryRun":     dryRun,
	})

	if result == nil {
		fmt.Println("ERROR: No result returned from tool")
		return
	}

	if res, ok := result.(map[string]interface{}); ok {
		if res["success"] == true {
			fmt.Println("\n✅ Successfully completed!")
			fmt.Printf("Album: %v\n", res["albumName"])
			fmt.Printf("Search query: %v\n", res["query"])

			if res["dryRun"] == true {
				fmt.Printf("Total found: %v\n", res["foundAssets"])
				fmt.Println("(DRY RUN - no actual changes made)")

				// Show sample results if available
				if samples, ok := res["sampleResults"].([]interface{}); ok && len(samples) > 0 {
					fmt.Println("\nSample results:")
					for i, sample := range samples {
						if i >= 5 {
							break
						} // Show first 5
						if s, ok := sample.(map[string]interface{}); ok {
							fmt.Printf("  - %s (%s)\n", s["fileName"], s["type"])
						}
					}
				}
			} else {
				fmt.Printf("Assets moved: %v\n", res["movedCount"])
				fmt.Printf("Failed: %v\n", res["failedCount"])
				if res["albumCreated"] == true {
					fmt.Println("Album was newly created")
				}
			}
		} else {
			fmt.Printf("Error: %v\n", res["message"])
		}
	} else {
		fmt.Printf("Result: %v\n", result)
	}
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
