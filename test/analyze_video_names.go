//go:build ignore

package main

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"
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

	// Get Large Movies album
	albums, err := immichClient.ListAlbums(ctx, false)
	if err != nil {
		log.Fatal("Failed to list albums:", err)
	}

	var largeMoviesID string
	for _, album := range albums {
		if album.AlbumName == "Large Movies" {
			largeMoviesID = album.ID
			break
		}
	}

	if largeMoviesID == "" {
		fmt.Println("Large Movies album not found")
		return
	}

	// Get all assets in the album
	assets, err := immichClient.GetAlbumAssets(ctx, largeMoviesID)
	if err != nil {
		log.Fatal("Failed to get album assets:", err)
	}

	fmt.Printf("=== Analyzing %d videos in Large Movies album ===\n\n", len(assets))

	// Categorize videos
	personalPatterns := []string{
		`^\d{8}_`,            // Date format: 20160525_
		`^\d{4}-\d{2}-\d{2}`, // Date format: 2024-01-15
		`^IMG_`,              // iPhone/camera format
		`^VID_`,              // Video format
		`^MOV_`,              // Movie format
		`^DSC`,               // Digital camera
		`^DSCN`,              // Nikon
		`^GOPR`,              // GoPro
		`^DJI_`,              // DJI drone
		`^PXL_`,              // Pixel phone
		`^FILE`,              // Generic file
		`\.MOV$`,             // MOV extension (personal videos)
		`\.mov$`,             // mov extension
		`^MVI_`,              // Canon video
		`^100`,               // Some cameras
		`^P\d{7}`,            // Some phone formats
	}

	moviePatterns := []string{
		`\.(mkv|avi|mp4)$`,
		`\d{4}\.`,        // Year in filename
		`S\d{2}E\d{2}`,   // TV series format
		`[Ss]\d+[Ee]\d+`, // Alternative series format
		`BluRay`,
		`1080p`,
		`720p`,
		`x264`,
		`x265`,
		`HDTV`,
		`WEB-DL`,
		`WebRip`,
	}

	personal := []string{}
	movies := []string{}
	unknown := []string{}

	for _, asset := range assets {
		if asset.Type != "VIDEO" {
			continue
		}

		name := asset.OriginalFileName
		isPersonal := false
		isMovie := false

		// Check personal patterns
		for _, pattern := range personalPatterns {
			matched, _ := regexp.MatchString(pattern, name)
			if matched {
				isPersonal = true
				break
			}
		}

		// Check movie patterns
		if !isPersonal {
			for _, pattern := range moviePatterns {
				matched, _ := regexp.MatchString(pattern, name)
				if matched {
					isMovie = true
					break
				}
			}
		}

		if isPersonal {
			personal = append(personal, name)
		} else if isMovie {
			movies = append(movies, name)
		} else {
			unknown = append(unknown, name)
		}
	}

	// Sort for better readability
	sort.Strings(personal)
	sort.Strings(movies)
	sort.Strings(unknown)

	// Display results
	fmt.Printf("PERSONAL VIDEOS (%d):\n", len(personal))
	for i, name := range personal {
		fmt.Printf("  %d. %s\n", i+1, name)
		if i >= 9 && len(personal) > 10 {
			fmt.Printf("  ... and %d more\n", len(personal)-10)
			break
		}
	}

	fmt.Printf("\nMOVIES/TV SHOWS (%d):\n", len(movies))
	for i, name := range movies {
		fmt.Printf("  %d. %s\n", i+1, name)
		if i >= 9 && len(movies) > 10 {
			fmt.Printf("  ... and %d more\n", len(movies)-10)
			break
		}
	}

	fmt.Printf("\nUNCATEGORIZED (%d):\n", len(unknown))
	for i, name := range unknown {
		fmt.Printf("  %d. %s\n", i+1, name)
		if i >= 19 && len(unknown) > 20 {
			fmt.Printf("  ... and %d more\n", len(unknown)-20)
			break
		}
	}

	// Show statistics
	fmt.Printf("\n=== SUMMARY ===\n")
	fmt.Printf("Total videos: %d\n", len(assets))
	fmt.Printf("Personal videos: %d (%.1f%%)\n", len(personal), float64(len(personal))*100/float64(len(assets)))
	fmt.Printf("Movies/TV shows: %d (%.1f%%)\n", len(movies), float64(len(movies))*100/float64(len(assets)))
	fmt.Printf("Uncategorized: %d (%.1f%%)\n", len(unknown), float64(len(unknown))*100/float64(len(assets)))

	// Suggest patterns for uncategorized
	if len(unknown) > 0 {
		fmt.Println("\nAnalyzing uncategorized filenames for patterns...")
		extensions := make(map[string]int)
		prefixes := make(map[string]int)

		for _, name := range unknown {
			// Get extension
			parts := strings.Split(name, ".")
			if len(parts) > 1 {
				ext := "." + parts[len(parts)-1]
				extensions[ext]++
			}

			// Get prefix (first 3 chars)
			if len(name) >= 3 {
				prefix := name[:3]
				prefixes[prefix]++
			}
		}

		fmt.Println("Common extensions:")
		for ext, count := range extensions {
			if count > 1 {
				fmt.Printf("  %s: %d files\n", ext, count)
			}
		}

		fmt.Println("Common prefixes:")
		for prefix, count := range prefixes {
			if count > 2 {
				fmt.Printf("  %s*: %d files\n", prefix, count)
			}
		}
	}
}
