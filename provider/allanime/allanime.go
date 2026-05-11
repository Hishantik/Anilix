package allanime

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/anilix/anilix/source"
)

const (
	Name = "allanime"
	ID   = "allanime"
)

type Allanime struct{}

func (a *Allanime) Name() string { return Name }

func (a *Allanime) ID() string { return ID }

func (a *Allanime) Search(query string) ([]*source.Anime, error) {
	resp, err := searchAnime(query)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	var animes []*source.Anime
	for _, edge := range resp.Data.Shows.Edges {
		animes = append(animes, &source.Anime{
			Name:   edge.Name,
			URL:    fmt.Sprintf("https://allmanga.to/anime/%s", edge.ID),
			Cover:  edge.Thumbnail,
			Source: a,
		})
	}

	return animes, nil
}

func (a *Allanime) SeasonsOf(anime *source.Anime) ([]source.Season, error) {
	// Extract show ID from URL
	showID := extractShowID(anime.URL)
	if showID == "" {
		return nil, fmt.Errorf("invalid anime URL: %s", anime.URL)
	}

	resp, err := getShow(showID)
	if err != nil {
		return nil, fmt.Errorf("failed to get show: %w", err)
	}

	var seasons []source.Season
	seasonNum := 1

	// Parse availableEpisodesDetail for seasons
	for mode, episodes := range resp.Data.Show.AvailableEpisodesDetail {
		if len(episodes) > 0 {
			name := modeToSeasonName(mode)
			seasons = append(seasons, source.Season{
				Number: seasonNum,
				Name:   name,
			})
			seasonNum++
		}
	}

	// Default to single season if no details
	if len(seasons) == 0 {
		seasons = append(seasons, source.Season{Number: 1, Name: "Season 1"})
	}

	return seasons, nil
}

func (a *Allanime) EpisodesOf(anime *source.Anime, season int) ([]*source.Episode, error) {
	showID := extractShowID(anime.URL)
	if showID == "" {
		return nil, fmt.Errorf("invalid anime URL: %s", anime.URL)
	}

	resp, err := getShow(showID)
	if err != nil {
		return nil, fmt.Errorf("failed to get show: %w", err)
	}

	// Get episodes for the season
	mode := seasonToMode(season)
	var episodeNumbers []int

	if mode != "" {
		if episodes, ok := resp.Data.Show.AvailableEpisodesDetail[mode]; ok {
			for _, ep := range episodes {
				switch v := ep.(type) {
				case float64:
					episodeNumbers = append(episodeNumbers, int(v))
				case int:
					episodeNumbers = append(episodeNumbers, v)
				case string:
					if num, err := strconv.Atoi(v); err == nil {
						episodeNumbers = append(episodeNumbers, num)
					}
				}
			}
		}
	}

	// Fallback to availableEpisodes count
	if len(episodeNumbers) == 0 {
		count := resp.Data.Show.AvailableEpisodes["sub"]
		if count == 0 {
			count = resp.Data.Show.AvailableEpisodes["dub"]
		}
		if count == 0 {
			for i := 1; i <= 12; i++ {
				episodeNumbers = append(episodeNumbers, i)
			}
		} else {
			for i := 1; i <= count; i++ {
				episodeNumbers = append(episodeNumbers, i)
			}
		}
	}

	var episodes []*source.Episode
	for _, num := range episodeNumbers {
		episodes = append(episodes, &source.Episode{
			Number: float64(num),
			Title:  fmt.Sprintf("Episode %d", num),
			URL:    fmt.Sprintf("https://allmanga.to/anime/%s/episode/%d", showID, num),
			Season: season,
			Anime:  anime,
		})
	}

	return episodes, nil
}

func (a *Allanime) StreamsOf(episode *source.Episode) ([]*source.Stream, error) {
	showID := extractShowID(episode.Anime.URL)
	if showID == "" {
		return nil, fmt.Errorf("invalid episode URL")
	}

	episodeString := fmt.Sprintf("%d", int(episode.Number))

	resp, err := getEpisodeStreams(showID, episodeString)
	if err != nil {
		return nil, fmt.Errorf("failed to get streams: %w", err)
	}

	// Check for encrypted URLs
	for _, url := range resp.Data.Episode.SourceUrls {
		if strings.Contains(url.SourceUrl, "tobeparsed") {
			// Need to decode encrypted URL
			decoded, err := decodeTobeparsed(extractTobeparsed(url.SourceUrl))
			if err == nil {
				return extractStreamURLsFromDecoded(decoded, episode), nil
			}
		}
	}

	return extractStreamURLs(resp.Data.Episode.SourceUrls, episode), nil
}

func extractShowID(url string) string {
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

func modeToSeasonName(mode string) string {
	switch mode {
	case "sub":
		return "Season 1 (Sub)"
	case "dub":
		return "Season 1 (Dub)"
	case "raw":
		return "Season 1 (Raw)"
	default:
		return mode
	}
}

func seasonToMode(season int) string {
	switch season {
	case 1:
		return "sub"
	default:
		return "sub"
	}
}

func extractTobeparsed(url string) string {
	// Extract base64 data from tobeparsed URL
	parts := strings.Split(url, "tobeparsed/")
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}

func extractStreamURLs(urls []SourceUrl, episode *source.Episode) []*source.Stream {
	var streams []*source.Stream

	for _, url := range urls {
		cleanURL := cleanUrl(url.SourceUrl)
		if cleanURL == "" || strings.Contains(cleanURL, "tobeparsed") {
			continue
		}

		quality := extractQuality(cleanURL)
		streams = append(streams, &source.Stream{
			Quality:  quality,
			URL:      cleanURL,
			Provider: url.SourceName,
		})
	}

	return streams
}

func extractStreamURLsFromDecoded(urls []SourceUrl, episode *source.Episode) []*source.Stream {
	var streams []*source.Stream

	for _, url := range urls {
		cleanURL := cleanUrl(url.SourceUrl)
		if cleanURL == "" {
			continue
		}

		quality := extractQuality(cleanURL)
		streams = append(streams, &source.Stream{
			Quality:  quality,
			URL:      cleanURL,
			Provider: url.SourceName,
		})
	}

	return streams
}

func extractQuality(url string) string {
	if strings.Contains(url, "1080") {
		return "1080p"
	}
	if strings.Contains(url, "720") {
		return "720p"
	}
	if strings.Contains(url, "480") {
		return "480p"
	}
	if strings.Contains(url, "360") {
		return "360p"
	}
	return "best"
}