package Allanime

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/anilix/anilix/extractor"
	"github.com/anilix/anilix/source"
)

// AllanimeProvider implements source.Source for AllAnime GraphQL API
type AllanimeProvider struct {
	client      *AllanimeClient
	translation string // "sub" or "dub"
}

func NewAllanimeProvider() *AllanimeProvider {
	return &AllanimeProvider{
		client:      NewAllanimeClient(),
		translation: "sub",
	}
}

func (a *AllanimeProvider) Name() string {
	return "AllAnime"
}

func (a *AllanimeProvider) ID() string {
	return "allanime"
}

// Search implements source.Source - searches anime via AllAnime GraphQL
func (a *AllanimeProvider) Search(query string) ([]*source.Anime, error) {
	ctx := context.Background()

	edges, err := a.client.SearchShows(ctx, query, 10, 1, a.translation)
	if err != nil {
		return nil, fmt.Errorf("AllAnime search failed: %w", err)
	}

	animes := make([]*source.Anime, 0, len(edges))
	for _, show := range edges {
		anime := a.client.MapToAnime(&show)
		animes = append(animes, anime)
	}

	return animes, nil
}

// SeasonsOf returns seasons for the anime
func (a *AllanimeProvider) SeasonsOf(anime *source.Anime) ([]source.Season, error) {
	// AllAnime doesn't have explicit seasons, return single "Season 1"
	return []source.Season{{Number: 1, Name: "Season 1"}}, nil
}

// EpisodesOf returns episodes for a specific season
func (a *AllanimeProvider) EpisodesOf(anime *source.Anime, season int) ([]*source.Episode, error) {
	if anime.AllAnimeID == "" {
		return nil, fmt.Errorf("anime has no AllAnimeID")
	}

	ctx := context.Background()
	episodesMap, err := a.client.GetShowEpisodes(ctx, anime.AllAnimeID, a.translation)
	if err != nil {
		return nil, fmt.Errorf("failed to get episodes: %w", err)
	}

	// Get the list based on translation type (sub/dub)
	epList, ok := episodesMap[a.translation]
	if !ok {
		// Fallback to sub if dub not available
		epList, ok = episodesMap["sub"]
		if !ok {
			return nil, fmt.Errorf("no episodes found")
		}
	}

	episodes := make([]*source.Episode, 0, len(epList))
	for _, epNum := range epList {
		num, _ := strconv.ParseFloat(epNum, 64)
		ep := &source.Episode{
			Number:  num,
			URL:     fmt.Sprintf("https://allmanga.to/watch/%s/%s", anime.AllAnimeID, epNum),
			Anime:   anime,
			Season:  season,
		}
		episodes = append(episodes, ep)
	}

	return episodes, nil
}

// StreamsOf returns stream sources for an episode
// It resolves provider URLs using extractors to get actual playable stream URLs
func (a *AllanimeProvider) StreamsOf(episode *source.Episode) ([]*source.Stream, error) {
	if episode.Anime == nil || episode.Anime.AllAnimeID == "" {
		return nil, fmt.Errorf("episode has no anime or AllAnimeID")
	}

	ctx := context.Background()
	epNum := strconv.FormatFloat(episode.Number, 'f', -1, 64)
	sources, err := a.client.GetEpisodeSources(ctx, episode.Anime.AllAnimeID, epNum, a.translation)
	if err != nil {
		return nil, fmt.Errorf("failed to get episode sources: %w", err)
	}

	// Collect all extracted streams
	var allStreams []*source.Stream
	seen := make(map[string]bool)

	for _, src := range sources {
		providerName := src.SourceName

		// Check if provider name is hex-encoded (contains only hex chars)
		if isHexEncoded(providerName) {
			providerName = decodeHexProviderID(providerName)
		}

		providerName = getProviderName(providerName)

		// Try to extract using the matching extractor
		ext := extractor.Resolve(src.SourceUrl)
		if ext != nil {
			// Use extractor to get actual playable streams
			extracted, err := ext.Extract(ctx, src.SourceUrl, Referer)
			if err == nil && len(extracted) > 0 {
				for _, s := range extracted {
					// Deduplicate by URL
					if !seen[s.URL] {
						seen[s.URL] = true
						s.Provider = providerName
						allStreams = append(allStreams, s)
					}
				}
			}
		}

		// Fallback: if no extractor matched, use raw URL
		if ext == nil && src.SourceUrl != "" {
			stream := &source.Stream{
				Provider: providerName,
				URL:      src.SourceUrl,
				Referer:  Referer,
				Quality:  "auto",
			}
			if !seen[stream.URL] {
				seen[stream.URL] = true
				allStreams = append(allStreams, stream)
			}
		}
	}

	if len(allStreams) == 0 {
		return nil, fmt.Errorf("no streams found")
	}

	return allStreams, nil
}

// SetTranslation sets sub or dub preference
func (a *AllanimeProvider) SetTranslation(trans string) {
	if trans == "sub" || trans == "dub" {
		a.translation = trans
	}
}

// getProviderName normalizes provider names
func getProviderName(name string) string {
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "")
	return name
}

// TranslationType returns current translation setting
func (a *AllanimeProvider) TranslationType() string {
	return a.translation
}