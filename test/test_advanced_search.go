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
	var assetType string
	var city string
	var country string
	var isFavorite bool
	var isNotInAlbum bool
	var size int
	var takenAfter string
	var takenBefore string

	flag.StringVar(&query, "query", "", "Smart search query")
	flag.StringVar(&assetType, "type", "", "Asset type: IMAGE, VIDEO, AUDIO, OTHER")
	flag.StringVar(&city, "city", "", "Filter by city")
	flag.StringVar(&country, "country", "", "Filter by country")
	flag.BoolVar(&isFavorite, "favorite", false, "Filter favorites only")
	flag.BoolVar(&isNotInAlbum, "notinalbum", false, "Filter assets not in albums")
	flag.IntVar(&size, "size", 100, "Maximum results")
	flag.StringVar(&takenAfter, "after", "", "Photos taken after date (YYYY-MM-DD)")
	flag.StringVar(&takenBefore, "before", "", "Photos taken before date (YYYY-MM-DD)")
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
	tools.RegisterTools(mcpServer, immichClient, cacheStore)

	ctx := context.Background()

	fmt.Println("=== Advanced Smart Search Test ===")
	fmt.Printf("Query: %s\n", query)
	if assetType != "" {
		fmt.Printf("Type: %s\n", assetType)
	}
	if city != "" {
		fmt.Printf("City: %s\n", city)
	}
	if country != "" {
		fmt.Printf("Country: %s\n", country)
	}
	if isFavorite {
		fmt.Println("Favorites only: true")
	}
	if isNotInAlbum {
		fmt.Println("Not in albums: true")
	}
	if takenAfter != "" || takenBefore != "" {
		fmt.Printf("Date range: %s to %s\n", takenAfter, takenBefore)
	}
	fmt.Printf("Max results: %d\n\n", size)

	// Build parameters
	params := map[string]interface{}{
		"size": size,
	}

	if query != "" {
		params["query"] = query
	}
	if assetType != "" {
		params["type"] = assetType
	}
	if city != "" {
		params["city"] = city
	}
	if country != "" {
		params["country"] = country
	}
	if isFavorite {
		params["isFavorite"] = true
	}
	if isNotInAlbum {
		params["isNotInAlbum"] = true
	}
	if takenAfter != "" {
		params["takenAfter"] = takenAfter + "T00:00:00Z"
	}
	if takenBefore != "" {
		params["takenBefore"] = takenBefore + "T23:59:59Z"
	}

	result := callTool(ctx, mcpServer, "smartSearchAdvanced", params)

	if result == nil {
		fmt.Println("ERROR: No result returned from tool")
		return
	}

	if res, ok := result.(map[string]interface{}); ok {
		fmt.Printf("Found: %v assets\n", res["foundCount"])

		if filters, ok := res["activeFilters"].([]interface{}); ok && len(filters) > 0 {
			fmt.Println("\nActive filters:")
			for _, filter := range filters {
				fmt.Printf("  - %v\n", filter)
			}
		}

		// Show sample results if available
		if samples, ok := res["sampleResults"].([]interface{}); ok && len(samples) > 0 {
			fmt.Println("\nSample results:")
			for i, sample := range samples {
				if i >= 5 { break } // Show first 5
				if s, ok := sample.(map[string]interface{}); ok {
					fmt.Printf("  %d. %s (%s)", i+1, s["fileName"], s["type"])
					if location, ok := s["location"]; ok && location != "" {
						fmt.Printf(" - %s", location)
					}
					if camera, ok := s["camera"]; ok && camera != "" {
						fmt.Printf(" [%s]", camera)
					}
					fmt.Println()
				}
			}
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