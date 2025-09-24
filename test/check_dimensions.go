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

	fmt.Println("=== Checking image dimensions distribution ===")

	// Size buckets
	buckets := map[string]int{
		"no_exif":    0,
		"0x0":        0,
		"≤100x100":   0,
		"≤200x200":   0,
		"≤400x400":   0,
		"≤800x800":   0,
		"≤1024x1024": 0,
		">1024x1024": 0,
	}

	totalImages := 0
	page := 1
	pageSize := 1000

	for page <= 10 { // Check first 10 pages
		assetPage, err := immichClient.GetAllAssets(ctx, page, pageSize)
		if err != nil {
			log.Printf("Failed to get page %d: %v", page, err)
			break
		}

		for _, asset := range assetPage.Assets {
			if asset.Type == "IMAGE" {
				totalImages++

				if asset.ExifInfo == nil {
					buckets["no_exif"]++
				} else {
					width := asset.ExifInfo.ExifImageWidth
					height := asset.ExifInfo.ExifImageHeight

					if width == 0 && height == 0 {
						buckets["0x0"]++
					} else if width > 0 && height > 0 {
						maxDim := width
						if height > maxDim {
							maxDim = height
						}

						switch {
						case width <= 100 && height <= 100:
							buckets["≤100x100"]++
						case width <= 200 && height <= 200:
							buckets["≤200x200"]++
						case width <= 400 && height <= 400:
							buckets["≤400x400"]++
						case width <= 800 && height <= 800:
							buckets["≤800x800"]++
						case width <= 1024 && height <= 1024:
							buckets["≤1024x1024"]++
						default:
							buckets[">1024x1024"]++
						}

						// Show examples of small images
						if width <= 400 && height <= 400 && buckets["≤400x400"] <= 5 {
							fmt.Printf("  Small image found: %s (%dx%d)\n",
								asset.OriginalFileName, width, height)
						}
					}
				}
			}
		}

		if !assetPage.HasNextPage || len(assetPage.Assets) == 0 {
			break
		}
		page++
	}

	fmt.Printf("\n=== Results from %d images ===\n", totalImages)
	fmt.Printf("No EXIF data: %d\n", buckets["no_exif"])
	fmt.Printf("0x0 dimensions: %d\n", buckets["0x0"])
	fmt.Printf("≤100x100: %d\n", buckets["≤100x100"])
	fmt.Printf("≤200x200: %d\n", buckets["≤200x200"])
	fmt.Printf("≤400x400: %d\n", buckets["≤400x400"])
	fmt.Printf("≤800x800: %d\n", buckets["≤800x800"])
	fmt.Printf("≤1024x1024: %d\n", buckets["≤1024x1024"])
	fmt.Printf(">1024x1024: %d\n", buckets[">1024x1024"])
}