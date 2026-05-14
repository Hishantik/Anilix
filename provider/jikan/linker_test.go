package jikan

import (
	"context"
	"testing"

	allanime "github.com/anilix/anilix/provider/allanime"
	"github.com/anilix/anilix/source"
)

type mockSource struct {
	episodes map[int][]*source.Episode
}

func (m *mockSource) Name() string { return "MockSource" }
func (m *mockSource) ID() string { return "mock" }
func (m *mockSource) Search(query string) ([]*source.Anime, error) { return nil, nil }
func (m *mockSource) SeasonsOf(anime *source.Anime) ([]source.Season, error) { return nil, nil }
func (m *mockSource) StreamsOf(episode *source.Episode) ([]*source.Stream, error) { return nil, nil }

func (m *mockSource) EpisodesOf(anime *source.Anime, season int) ([]*source.Episode, error) {
	if anime.AllAnimeID != "" {
		return m.episodes[anime.MALID], nil
	}
	return nil, nil
}

func TestAnimeLinker_GetEpisodes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	linker := NewAnimeLinker()

	// Use real AllAnime source for integration test
	allanimeSrc := allanime.NewAllanimeProvider()

	// Search for anime with known MAL ID (Boruto = 34566)
	anime := &source.Anime{
		Name:  "Boruto",
		MALID: 34566,
	}

	// This will try to resolve AllAnime ID and get episodes
	episodes, err := linker.GetEpisodes(ctx, anime, allanimeSrc)
	if err != nil {
		t.Fatalf("GetEpisodes failed: %v", err)
	}

	if len(episodes) == 0 {
		t.Error("expected episodes for Boruto")
	}

	t.Logf("Found %d episodes for %s", len(episodes), anime.Name)
}

func TestAnimeLinker_GetEpisodes_NoMALID(t *testing.T) {
	linker := NewAnimeLinker()

	mockSrc := &mockSource{}

	anime := &source.Anime{
		Name:  "Test Anime",
		MALID: 0,
	}

	_, err := linker.GetEpisodes(context.Background(), anime, mockSrc)
	if err == nil {
		t.Error("expected error when anime has no MAL ID")
	}
}