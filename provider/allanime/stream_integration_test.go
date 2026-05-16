package Allanime

import (
	"context"
	"testing"

	"github.com/anilix/anilix/source"
)

func TestIntegration_FullStreamExtraction(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	client := NewAllanimeClient()

	// Search for a show
	shows, err := client.SearchShows(ctx, "One Piece", 3, 1, "sub")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(shows) == 0 {
		t.Fatal("no results found")
	}

	show := shows[0]
	t.Logf("Testing with: %s (ID: %s)", show.Name, show.ID)

	// Get episodes
	episodes, err := client.GetShowEpisodes(ctx, show.ID, "sub")
	if err != nil {
		t.Fatalf("GetShowEpisodes failed: %v", err)
	}

	subEpisodes, ok := episodes["sub"]
	if !ok || len(subEpisodes) == 0 {
		t.Skip("no sub episodes")
	}

	// Get sources for first episode
	firstEp := subEpisodes[0]
	t.Logf("Getting sources for episode: %s", firstEp)

	sources, err := client.GetEpisodeSources(ctx, show.ID, firstEp, "sub")
	if err != nil {
		t.Fatalf("GetEpisodeSources failed: %v", err)
	}

	t.Logf("Found %d sources", len(sources))

	if len(sources) == 0 {
		t.Fatal("no sources found")
	}

	// Create a minimal anime and episode for StreamsOf
	anime := &source.Anime{
		AllAnimeID: show.ID,
		Name:       show.Name,
	}

	episode := &source.Episode{
		Number: 1,
		Anime:  anime,
	}

	// Test the provider's StreamsOf method which does extraction
	provider := NewAllanimeProvider()
	streams, err := provider.StreamsOf(episode)
	if err != nil {
		t.Fatalf("StreamsOf failed: %v", err)
	}

	t.Logf("Extracted %d streams", len(streams))
	for i, s := range streams {
		t.Logf("  %d. Provider: %s, Quality: %s", i+1, s.Provider, s.Quality)
		// Show truncated URL
		url := s.URL
		if len(url) > 50 {
			url = url[:50] + "..."
		}
		t.Logf("      URL: %s", url)
	}

	if len(streams) == 0 {
		t.Error("expected at least one stream")
	}
}