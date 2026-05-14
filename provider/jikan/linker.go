package jikan

import (
	"context"
	"fmt"

	"github.com/anilix/anilix/cache"
	"github.com/anilix/anilix/source"
)

type AnimeLinker struct {
	cache *cache.MALIDCache
}

func NewAnimeLinker() *AnimeLinker {
	c, _ := cache.NewMALIDCache() // ignore error - cache is optional
	return &AnimeLinker{cache: c}
}

// ResolveAllAnimeID resolves MAL ID to AllAnime ID using cache and AllAnime search
func (l *AnimeLinker) ResolveAllAnimeID(ctx context.Context, anime *source.Anime, AllanimeSrc source.Source) (string, error) {
	if anime == nil {
		return "", fmt.Errorf("anime is nil")
	}

	if anime.MALID == 0 {
		return "", fmt.Errorf("anime has no MAL ID")
	}

	// 1. Check cache first
	if l.cache != nil {
		if cachedID, err := l.cache.Get(anime.MALID); err == nil && cachedID != "" {
			return cachedID, nil
		}
	}

	// 2. Search AllAnime with same name query
	results, err := AllanimeSrc.Search(anime.Name)
	if err != nil {
		return "", fmt.Errorf("AllAnime search failed: %w", err)
	}

	// 3. Match by MAL ID
	for _, r := range results {
		if r.MALID == anime.MALID {
			// 4. Cache the result
			if l.cache != nil {
				l.cache.Set(anime.MALID, r.AllAnimeID)
			}
			return r.AllAnimeID, nil
		}
	}

	return "", fmt.Errorf("no matching AllAnime show found for MAL ID %d", anime.MALID)
}

// GetEpisodes resolves AllAnimeID if needed, then fetches episodes
func (l *AnimeLinker) GetEpisodes(ctx context.Context, anime *source.Anime, AllanimeSrc source.Source) ([]*source.Episode, error) {
	if anime == nil {
		return nil, fmt.Errorf("anime is nil")
	}

	if anime.MALID == 0 {
		return nil, fmt.Errorf("anime has no MAL ID - cannot map to streaming source")
	}

	// Resolve AllAnimeID if not already set
	if anime.AllAnimeID == "" {
		allanimeID, err := l.ResolveAllAnimeID(ctx, anime, AllanimeSrc)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve AllAnime ID: %w", err)
		}
		anime.AllAnimeID = allanimeID
	}

	seasons, err := AllanimeSrc.SeasonsOf(anime)
	if err != nil {
		return nil, fmt.Errorf("failed to get seasons: %w", err)
	}

	if len(seasons) == 0 {
		return AllanimeSrc.EpisodesOf(anime, 0)
	}

	return AllanimeSrc.EpisodesOf(anime, seasons[0].Number)
}

func (l *AnimeLinker) GetStreams(episode *source.Episode, src source.Source) ([]*source.Stream, error) {
	if episode == nil {
		return nil, fmt.Errorf("episode is nil")
	}

	return src.StreamsOf(episode)
}