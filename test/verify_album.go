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

	// List albums to find "Bad Thumbnails"
	albums, err := immichClient.ListAlbums(ctx, true) // with assets
	if err != nil {
		log.Fatal("failed to list albums: ", err)
	}

	for _, album := range albums {
		if album.AlbumName == "Bad Thumbnails" {
			fmt.Printf("Album: %s\n", album.AlbumName)
			fmt.Printf("Album ID: %s\n", album.ID)
			fmt.Printf("Description: %s\n", album.Description)
			fmt.Printf("Asset Count: %d\n", album.AssetCount)
			fmt.Printf("Created: %s\n", album.CreatedAt)
			fmt.Printf("Updated: %s\n", album.UpdatedAt)

			fmt.Printf("\n✅ Successfully verified! Album contains %d assets\n", album.AssetCount)
			return
		}
	}

	fmt.Println("Album 'Bad Thumbnails' not found")
}