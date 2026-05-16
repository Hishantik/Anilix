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
// Like ani-cli: fetch+extract per provider in priority order, stop at first success
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

	fmt.Printf("DEBUG: AllanimeProvider.StreamsOf - Got %d sources\n", len(sources))

	// Decode and prepare sources with priority
	preparedSources := make([]preparedSource, 0, len(sources))
	for _, src := range sources {
		providerName := src.SourceName
		streamUrl := src.SourceUrl

		// Decode hex-encoded provider names
		if isHexEncoded(providerName) {
			providerName = decodeHexProviderID(providerName)
		}

		// Decode hex-encoded URLs (skip clock URLs)
		if isHexEncoded(streamUrl) && !strings.Contains(streamUrl, "/clock") {
			streamUrl = decodeHexProviderID(streamUrl)
		}

		providerName = getProviderName(providerName)
		priority := getProviderPriority(providerName)

		preparedSources = append(preparedSources, preparedSource{
			provider:    providerName,
			url:         streamUrl,
			priority:    priority,
		})
	}

	// Sort by priority (lower = higher priority)
	sortByPriority(preparedSources)

	// Try each provider in priority order (like ani-cli)
	seen := make(map[string]bool)
	for _, ps := range preparedSources {
		// Skip clock URLs that need special handling
		if strings.Contains(ps.url, "/clock") {
			fmt.Printf("DEBUG: Skipping clock URL: %s\n", ps.url[:min(30, len(ps.url))])
			continue
		}

		// Skip embed URLs (these can't be played directly)
		if isEmbedURL(ps.url) {
			fmt.Printf("DEBUG: Skipping embed URL: %s\n", ps.url[:min(30, len(ps.url))])
			continue
		}

		fmt.Printf("DEBUG: Trying provider: %s (priority %d), url: %s\n", ps.provider, ps.priority, ps.url[:min(50, len(ps.url))])

		// Find extractor for this URL
		var extracted []*source.Stream
		for _, ext := range extractor.All() {
			if ext.CanHandle(ps.url) {
				fmt.Printf("DEBUG: Using extractor: %s for URL: %s\n", ext.Name(), ps.url[:min(60, len(ps.url))])
				streams, err := ext.Extract(ctx, ps.url, Referer)
				if err == nil && len(streams) > 0 {
					extracted = streams
					fmt.Printf("DEBUG: Extracted %d streams from %s\n", len(streams), ext.Name())
					break
				}
				fmt.Printf("DEBUG: Extractor %s failed: %v\n", ext.Name(), err)
			}
		}

		// If we got streams, return them (like ani-cli stops at first working provider)
		if len(extracted) > 0 {
			result := make([]*source.Stream, 0, len(extracted))
			for _, s := range extracted {
				// Fix relative URLs
				url := s.URL
				if strings.HasPrefix(url, "//") {
					url = "https:" + url
				}

				if !seen[url] {
					seen[url] = true
					s.Provider = ps.provider
					s.URL = url
					s.Referer = ps.url
					result = append(result, s)
					fmt.Printf("DEBUG:   -> stream: provider=%s, url=%s\n", s.Provider, url[:min(80, len(url))])
				}
			}
			if len(result) > 0 {
				fmt.Printf("DEBUG: Returning %d streams from provider: %s\n", len(result), ps.provider)
				return result, nil
			}
		}

		// Fallback: if no extractor found or extraction failed, try raw URL if it's a direct video URL
		// Don't use embed URLs as fallback - they can't be played directly
		url := ps.url
		if strings.HasPrefix(url, "//") {
			url = "https:" + url
		}
		isEmbed := isEmbedURL(url) ||
			strings.HasSuffix(url, "/embed") ||
			strings.Contains(url, "/embed-") ||
			strings.Contains(url, "/e/") ||
			strings.Contains(url, "/videoembed")
		if !seen[url] && url != "" && !isEmbed {
			seen[url] = true
			fmt.Printf("DEBUG: Using raw URL as fallback: %s\n", url[:min(80, len(url))])
			return []*source.Stream{{
				Provider: ps.provider,
				URL:      url,
				Referer:  Referer,
				Quality:  "auto",
			}}, nil
		} else if isEmbed {
			fmt.Printf("DEBUG: Skipping embed URL as fallback: %s\n", url[:min(40, len(url))])
		}
	}

	return nil, fmt.Errorf("no streams found")
}

// preparedSource holds a decoded source with priority
type preparedSource struct {
	provider string
	url      string
	priority int
}

// getProviderPriority returns priority for a provider (lower = higher priority)
func getProviderPriority(provider string) int {
	priority := map[string]int{
		"wixmp":        1,
		"hianime":      2,
		"filemoon":     3,
		"vidstreaming": 4,
		"mp4upload":    5,
		"anihdplay":    6,
		"streamsb":     7,
		"streamlare":   8,
		"ok":           9,  // ok.ru fallback
		"fm-hls":       10, // filemoon hls
		"default":      20,
	}
	if p, ok := priority[provider]; ok {
		return p
	}
	return priority["default"]
}

// sortByPriority sorts sources by priority (ani-cli order: wixmp > hianime > filemoon)
func sortByPriority(sources []preparedSource) {
	for i := 0; i < len(sources)-1; i++ {
		for j := i + 1; j < len(sources); j++ {
			if sources[i].priority > sources[j].priority {
				sources[i], sources[j] = sources[j], sources[i]
			}
		}
	}
}

// isEmbedURL checks if URL is an embed page that can't be played directly
func isEmbedURL(url string) bool {
	// Skip common embed domains
	return strings.Contains(url, "ok.ru/videoembed") ||
		strings.Contains(url, "vk.com/video_ext") ||
		strings.Contains(url, "/videoembed") ||
		strings.HasSuffix(url, "/embed") ||
		strings.Contains(url, "/embed/") ||
		strings.Contains(url, "/e/") ||
		strings.Contains(url, "allanime.uns.bio") ||
		strings.Contains(url, "allanime.bio") ||
		strings.Contains(url, "#") || // Fragment identifiers indicate player pages
		strings.HasSuffix(url, "/player")
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