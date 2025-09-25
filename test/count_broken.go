//go:build ignore

package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/yourusername/mcp-immich/pkg/config"
	"github.com/yourusername/mcp-immich/pkg/immich"
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

	// Create client
	immichClient := immich.NewClient(cfg.ImmichURL, cfg.ImmichAPIKey, 30*time.Second)
	ctx := context.Background()

	fmt.Println("=== Counting broken thumbnail images with proper pagination ===")

	totalBroken := 0
	page := 1
	pageSize := 1000
	maxPages := 1000 // Safety limit

	for page <= maxPages {
		assetPage, err := immichClient.GetAllAssets(ctx, page, pageSize)
		if err != nil {
			log.Fatalf("Failed to get page %d: %v", page, err)
		}

		brokenInPage := 0
		for _, asset := range assetPage.Assets {
			if asset.Type == "IMAGE" && asset.Thumbhash == "" {
				totalBroken++
				brokenInPage++
			}
		}

		fmt.Printf("Page %d: Found %d broken images (Total so far: %d)\n", page, brokenInPage, totalBroken)
		fmt.Printf("  Page info: %d assets, HasNextPage=%v, TotalCount=%d\n",
			len(assetPage.Assets), assetPage.HasNextPage, assetPage.TotalCount)

		// Check if we've processed all pages
		if !assetPage.HasNextPage || len(assetPage.Assets) == 0 {
			fmt.Printf("\nReached end of results at page %d\n", page)
			break
		}

		page++

		// Check if we might be hitting a hard limit
		if page*pageSize >= 50000 {
			fmt.Printf("\nWARNING: Approaching 50k limit at page %d\n", page)
		}
	}

	fmt.Printf("\n=== FINAL COUNT ===\n")
	fmt.Printf("Total broken images found: %d\n", totalBroken)
	fmt.Printf("Pages processed: %d\n", page)
}
