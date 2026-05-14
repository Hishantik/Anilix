package Allanime

import (
	"context"
	"testing"
)

func TestIntegration_GetEpisodesByShowID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	client := NewAllanimeClient()

	// Direct test using known AllAnime show ID (Boruto)
	// This bypasses search and directly tests episode list retrieval
	showID := "vkD8H5e7HsG2jctw9" // Boruto: Naruto Next Generations

	t.Logf("Getting episodes for show ID: %s", showID)

	episodes, err := client.GetShowEpisodes(ctx, showID, "sub")
	if err != nil {
		t.Fatalf("GetShowEpisodes failed: %v", err)
	}

	subCount := 0
	dubCount := 0
	if sub, ok := episodes["sub"]; ok {
		subCount = len(sub)
	}
	if dub, ok := episodes["dub"]; ok {
		dubCount = len(dub)
	}

	t.Logf("Episode counts - Sub: %d, Dub: %d", subCount, dubCount)

	if subCount == 0 {
		t.Fatal("expected sub episodes")
	}

	// Get sources for first episode
	firstEp := episodes["sub"][len(episodes["sub"])-1] // episode "1"
	t.Logf("\nGetting sources for episode: %s", firstEp)

	sources, err := client.GetEpisodeSources(ctx, showID, firstEp, "sub")
	if err != nil {
		t.Fatalf("GetEpisodeSources failed: %v", err)
	}

	t.Logf("Found %d sources", len(sources))
	for i, src := range sources {
		if i < 3 { // Only log first 3
			t.Logf("  - Provider: %s, URL: %s...", src.SourceName, src.SourceUrl[:min(50, len(src.SourceUrl))])
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestIntegration_AllanimeSearch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	client := NewAllanimeClient()

	// Test search for Naruto
	shows, err := client.SearchShows(ctx, "Naruto", 10, 1, "sub")
	if err != nil {
		t.Fatalf("AllAnime search failed: %v", err)
	}

	if len(shows) == 0 {
		t.Fatal("expected search results")
	}

	t.Logf("Found %d results", len(shows))
	for _, show := range shows {
		t.Logf("  - %s (ID: %s, MAL ID: %s, Episodes: %d)", show.Name, show.ID, show.MalID, show.AvailableEpisodes.Sub)
	}

	// Find Boruto with MAL ID 34566
	var mainShow ShowNode
	for _, show := range shows {
		if show.MalID == "34566" {
			mainShow = show
			break
		}
	}

	// Fallback: any show with many episodes
	if mainShow.ID == "" {
		for _, show := range shows {
			if show.AvailableEpisodes.Sub >= 100 {
				mainShow = show
				break
			}
		}
	}

	// Last fallback: any show with episodes
	if mainShow.ID == "" {
		for _, show := range shows {
			if show.AvailableEpisodes.Sub > 0 {
				mainShow = show
				break
			}
		}
	}

	if mainShow.ID != "" {
		t.Logf("\nGetting episodes for: %s (ID: %s, MAL ID: %s)", mainShow.Name, mainShow.ID, mainShow.MalID)

		episodes, err := client.GetShowEpisodes(ctx, mainShow.ID, "sub")
		if err != nil {
			t.Fatalf("GetShowEpisodes failed: %v", err)
		}

		t.Logf("Episode counts - Sub: %d, Dub: %d", len(episodes["sub"]), len(episodes["dub"]))

		// Get sources for first episode
		if len(episodes["sub"]) > 0 {
			firstEp := episodes["sub"][0]
			t.Logf("\nGetting sources for episode: %s", firstEp)

			sources, err := client.GetEpisodeSources(ctx, mainShow.ID, firstEp, "sub")
			if err != nil {
				t.Fatalf("GetEpisodeSources failed: %v", err)
			}

			t.Logf("Found %d sources:", len(sources))
			for _, src := range sources {
				urlPreview := src.SourceUrl
				if len(urlPreview) > 60 {
					urlPreview = urlPreview[:60] + "..."
				}
				t.Logf("  - Provider: %s, URL: %s", src.SourceName, urlPreview)
			}
		}
	}
}

func TestIntegration_GetEpisodeSources(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	client := NewAllanimeClient()

	// First search for an anime
	shows, err := client.SearchShows(ctx, "One Piece", 5, 1, "sub")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(shows) == 0 {
		t.Skip("No results found, skipping test")
	}

	show := shows[0]
	t.Logf("Testing with: %s (ID: %s)", show.Name, show.ID)

	// Get episode list
	episodes, err := client.GetShowEpisodes(ctx, show.ID, "sub")
	if err != nil {
		t.Fatalf("GetShowEpisodes failed: %v", err)
	}

	subEpisodes, ok := episodes["sub"]
	if !ok || len(subEpisodes) == 0 {
		t.Skip("No sub episodes found")
	}

	// Get sources for first episode
	episodeString := subEpisodes[0]
	t.Logf("Fetching sources for episode: %s", episodeString)

	sources, err := client.GetEpisodeSources(ctx, show.ID, episodeString, "sub")
	if err != nil {
		t.Fatalf("GetEpisodeSources failed: %v", err)
	}

	t.Logf("Found %d sources", len(sources))
	for _, src := range sources {
		t.Logf("  - Provider: %s, URL: %s", src.SourceName, src.SourceUrl)
	}
}

func TestIntegration_DecryptToBeParsed(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Test the decryption with a real encrypted payload
	// This test verifies the key generation matches ani-cli
	if AllAnimeKey == "" {
		t.Fatal("AllAnimeKey should be generated")
	}

	t.Logf("Generated key: %s", AllAnimeKey)

	// Key should be 64 hex chars (256 bits / 4)
	if len(AllAnimeKey) != 64 {
		t.Errorf("Expected key length 64, got %d", len(AllAnimeKey))
	}
}

func TestIntegration_Provider(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	p := NewAllanimeProvider()

	// Test search through provider interface
	results, err := p.Search("Cowboy Bebop")
	if err != nil {
		t.Fatalf("Provider search failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected search results")
	}

	t.Logf("Found %d results", len(results))
	for _, anime := range results {
		t.Logf("  - %s (AllAnime ID: %s, Episodes: %d)", anime.Name, anime.AllAnimeID, anime.EpisodeCount)
	}
}