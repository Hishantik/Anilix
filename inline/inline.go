package inline

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/anilix/anilix/config"
	"github.com/anilix/anilix/player"
	"github.com/anilix/anilix/provider"
	"github.com/anilix/anilix/source"
)

type Mode struct {
	reader     *bufio.Reader
	player     *player.Player
	quality    string
	currentSrc source.Source
}

func New() *Mode {
	playerName := config.GetString("player")
	if playerName == "" {
		playerName = "mpv"
	}

	quality := config.GetString("quality")
	if quality == "" {
		quality = "best"
	}

	return &Mode{
		reader:  bufio.NewReader(os.Stdin),
		player:  player.FromString(playerName),
		quality: quality,
	}
}

func (m *Mode) Run(query string) error {
	if query == "" {
		fmt.Print("Search anime: ")
		query, _ = m.reader.ReadString('\n')
		query = strings.TrimSpace(query)
		if query == "" {
			return fmt.Errorf("empty query")
		}
	}

	// Get available providers
	providers := provider.All()
	if len(providers) == 0 {
		return fmt.Errorf("no providers available")
	}

	// Use first provider or configured one
	srcName := config.GetString("source")
	var src source.Source
	if srcName != "" {
		if p, ok := provider.Get(srcName); ok {
			s, err := p.CreateSource()
			if err != nil {
				return fmt.Errorf("failed to create source: %w", err)
			}
			src = s
			m.currentSrc = src
		} else {
			return fmt.Errorf("source not found: %s", srcName)
		}
	} else {
		s, err := providers[0].CreateSource()
		if err != nil {
			return fmt.Errorf("failed to create source: %w", err)
		}
		src = s
		m.currentSrc = src
	}

	// Search
	fmt.Printf("\nSearching for: %s\n\n", query)
	results, err := src.Search(query)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		fmt.Println("No results found")
		return nil
	}

	// Show results
	m.showAnimeList(results)

	// Select anime
	choice, err := m.promptChoice(1, len(results))
	if err != nil {
		return err
	}

	anime := results[choice-1]
	fmt.Printf("\nSelected: %s\n", anime.Name)

	// Get seasons
	seasons, err := src.SeasonsOf(anime)
	if err != nil {
		return fmt.Errorf("failed to get seasons: %w", err)
	}

	if len(seasons) == 0 {
		return fmt.Errorf("no seasons available")
	}

	// Single season - skip selection
	var selectedSeason int
	if len(seasons) == 1 {
		selectedSeason = seasons[0].Number
		fmt.Printf("Season: %s\n", seasons[0].Name)
	} else {
		// Show seasons
		m.showSeasons(seasons)

		// Select season
		seasonChoice, err := m.promptChoice(1, len(seasons))
		if err != nil {
			return err
		}
		selectedSeason = seasons[seasonChoice-1].Number
	}

	// Get episodes
	episodes, err := src.EpisodesOf(anime, selectedSeason)
	if err != nil {
		return fmt.Errorf("failed to get episodes: %w", err)
	}

	if len(episodes) == 0 {
		return fmt.Errorf("no episodes available")
	}

	// Show episodes
	m.showEpisodes(episodes)

	// Select episode
	epChoice, err := m.promptChoice(1, len(episodes))
	if err != nil {
		return err
	}

	episode := episodes[epChoice-1]
	fmt.Printf("\nSelected: Episode %s - %s\n", episode.String(), episode.Title)

	// Get streams
	streams, err := src.StreamsOf(episode)
	if err != nil {
		return fmt.Errorf("failed to get streams: %w", err)
	}

	if len(streams) == 0 {
		return fmt.Errorf("no streams available")
	}

	// Show quality options
	m.showStreams(streams)

	// Select quality
	var stream *source.Stream
	if m.quality == "best" || m.quality == "" {
		stream = streams[0] // First is best (1080p)
	} else {
		for _, s := range streams {
			if s.Quality == m.quality {
				stream = s
				break
			}
		}
		if stream == nil {
			stream = streams[0]
		}
	}

	fmt.Printf("\nPlaying: %s (%s)\n", episode.Title, stream.Quality)

	// Launch player
	title := fmt.Sprintf("%s - Episode %s", anime.Name, episode.String())
	return m.player.Launch(stream.URL, player.Options{
		Title:    title,
		Referrer: "https://allmanga.to",
	})
}

func (m *Mode) showAnimeList(animes []*source.Anime) {
	fmt.Println("Search results:")
	for i, a := range animes {
		fmt.Printf("%d. %s (%d) [%s]\n", i+1, a.Name, a.Year, a.Status)
	}
}

func (m *Mode) showSeasons(seasons []source.Season) {
	fmt.Println("\nSeasons:")
	for i, s := range seasons {
		fmt.Printf("%d. %s\n", i+1, s.Name)
	}
}

func (m *Mode) showEpisodes(episodes []*source.Episode) {
	fmt.Println("\nEpisodes:")
	for i, e := range episodes {
		title := e.Title
		if len(title) > 50 {
			title = title[:47] + "..."
		}
		fmt.Printf("%d. Episode %s - %s\n", i+1, e.String(), title)
	}
}

func (m *Mode) showStreams(streams []*source.Stream) {
	fmt.Println("\nQuality options:")
	for i, s := range streams {
		fmt.Printf("%d. %s\n", i+1, s.Quality)
	}
}

func (m *Mode) promptChoice(min, max int) (int, error) {
	for {
		fmt.Printf("Select [%d-%d]: ", min, max)
		input, _ := m.reader.ReadString('\n')
		input = strings.TrimSpace(input)

		choice, err := strconv.Atoi(input)
		if err != nil {
			fmt.Println("Invalid input. Enter a number.")
			continue
		}

		if choice < min || choice > max {
			fmt.Printf("Enter a number between %d and %d\n", min, max)
			continue
		}

		return choice, nil
	}
}