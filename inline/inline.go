package inline

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/anilix/anilix/config"
	"github.com/anilix/anilix/extractor"
	"github.com/anilix/anilix/player"
	"github.com/anilix/anilix/provider"
	Allanime "github.com/anilix/anilix/provider/allanime"
	"github.com/anilix/anilix/provider/jikan"
	"github.com/anilix/anilix/source"
)

type Mode struct {
	reader  *bufio.Reader
	player  *player.Player
	quality string
}

func init() {
	// Register Jikan provider (search/metadata)
	provider.Register(&provider.Provider{
		ID:           "jikan",
		Name:         "Jikan",
		UsesHeadless: false,
		IsCustom:     false,
		CreateSource: func() (interface{}, error) {
			return jikan.NewJikanProvider(), nil
		},
	})

	// Register Allanime provider (streaming)
	provider.Register(&provider.Provider{
		ID:           "allanime",
		Name:         "AllAnime",
		UsesHeadless: false,
		IsCustom:     false,
		CreateSource: func() (interface{}, error) {
			return Allanime.NewAllanimeProvider(), nil
		},
	})
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

	fmt.Printf("\nSearching for: %s\n\n", query)

	// Get providers
	jikanProvider, ok := provider.Get("Jikan")
	if !ok {
		return fmt.Errorf("Jikan provider not available")
	}

	allanimeProvider, ok := provider.Get("AllAnime")
	if !ok {
		return fmt.Errorf("AllAnime provider not available")
	}

	// Create Jikan source
	jikanSrcIface, err := jikanProvider.CreateSource()
	if err != nil {
		return fmt.Errorf("failed to create Jikan source: %w", err)
	}
	jikanSrc, ok := jikanSrcIface.(source.Source)
	if !ok {
		return fmt.Errorf("invalid Jikan source")
	}

	allanimeSrcIface, err := allanimeProvider.CreateSource()
	if err != nil {
		return fmt.Errorf("failed to create AllAnime source: %w", err)
	}
	allanimeSrc, ok := allanimeSrcIface.(source.Source)
	if !ok {
		return fmt.Errorf("invalid AllAnime source")
	}

	// Search both APIs in parallel
	jikanChan := make(chan []*source.Anime, 1)
	allanimeChan := make(chan []*source.Anime, 1)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		results, err := jikanSrc.Search(query)
		if err == nil && results != nil {
			jikanChan <- results
		} else {
			jikanChan <- []*source.Anime{}
		}
	}()

	go func() {
		defer wg.Done()
		results, err := allanimeSrc.Search(query)
		if err == nil && results != nil {
			allanimeChan <- results
		} else {
			allanimeChan <- []*source.Anime{}
		}
	}()

	wg.Wait()
	close(jikanChan)
	close(allanimeChan)

	jikanResults := <-jikanChan
	allanimeResults := <-allanimeChan

	if len(jikanResults) == 0 && len(allanimeResults) == 0 {
		fmt.Println("No results found")
		return nil
	}

	// Combine results - match AllAnime by MAL ID
	combined := m.combineResults(jikanResults, allanimeResults)

	// Show simplified list (no MAL ID)
	m.showAnimeListSimple(combined)

	// Select anime
	choice, err := m.promptChoice(1, len(combined))
	if err != nil {
		return err
	}

	anime := combined[choice-1]

	// Show full Jikan metadata
	m.showAnimeMetadata(anime)

	// Check if we have AllAnime ID for streaming
	if anime.AllAnimeID == "" {
		fmt.Println("\nNote: No streaming available for this anime.")
		return nil
	}

	// Get episodes from AllAnime
	fmt.Println("\n--- Fetching episodes ---")

	seasons, err := allanimeSrc.SeasonsOf(anime)
	if err != nil {
		fmt.Printf("Failed to get seasons: %v\n", err)
		return nil
	}

	if len(seasons) == 0 {
		fmt.Println("No seasons found")
		return nil
	}

	fmt.Printf("\nSeasons found: %d\n", len(seasons))

	season := 1
	if len(seasons) > 1 {
		fmt.Println("\nSelect season:")
		m.showSeasons(seasons)
		seasonChoice, _ := m.promptChoice(1, len(seasons))
		season = seasonChoice
	}

	episodes, err := allanimeSrc.EpisodesOf(anime, season)
	if err != nil {
		fmt.Printf("Failed to get episodes: %v\n", err)
		return nil
	}

	fmt.Printf("\nTotal episodes: %d\n", len(episodes))

	showCount := 5
	if showCount > len(episodes) {
		showCount = len(episodes)
	}
	fmt.Printf("Showing first %d episodes:\n", showCount)
	for i := 0; i < showCount; i++ {
		fmt.Printf("  Episode %s\n", episodes[i].String())
	}

	if len(episodes) > showCount {
		fmt.Printf("  ... and %d more\n", len(episodes)-showCount)
	}

	// Ask to play
	fmt.Print("\nPlay episode? [y/n]: ")
	playChoice, _ := m.reader.ReadString('\n')
	if strings.TrimSpace(strings.ToLower(playChoice)) != "y" {
		return nil
	}

	// Select episode
	fmt.Println("\nSelect episode:")
	for i := 0; i < showCount && i < len(episodes); i++ {
		fmt.Printf("  %d. Episode %s\n", i+1, episodes[i].String())
	}

	epChoice, _ := m.promptChoice(1, showCount)
	episode := episodes[epChoice-1]
	episode.Anime = anime

	// Get streams
	fmt.Printf("\nFetching streams for Episode %s...\n", episode.String())

	streams, err := allanimeSrc.StreamsOf(episode)
	if err != nil {
		fmt.Printf("Failed to get streams: %v\n", err)
		return nil
	}

	fmt.Printf("Found %d stream sources\n", len(streams))

	if len(streams) == 0 {
		fmt.Println("No streams found")
		return nil
	}

	// Show available streams
	for i, s := range streams {
		urlPreview := s.URL
		if len(urlPreview) > 50 {
			urlPreview = urlPreview[:50] + "..."
		}
		fmt.Printf("  %d. %s (%s) - %s\n", i+1, s.Provider, s.Quality, urlPreview)
	}

	// Try to extract actual streams
	ctx := context.Background()
	for _, stream := range streams {
		fmt.Printf("\nTrying to extract %s stream...\n", stream.Provider)

		ext := extractor.Resolve(stream.URL)
		if ext == nil {
			fmt.Printf("  No extractor found for this URL type\n")
			continue
		}

		fmt.Printf("  Using extractor: %s\n", ext.Name())

		extractedStreams, err := ext.Extract(ctx, stream.URL, stream.Referer)
		if err != nil {
			fmt.Printf("  Extraction failed: %v\n", err)
			continue
		}

		fmt.Printf("  Found %d extracted streams:\n", len(extractedStreams))
		for j, es := range extractedStreams {
			urlPreview := es.URL
			if len(urlPreview) > 60 {
				urlPreview = urlPreview[:60] + "..."
			}
			fmt.Printf("    %d. %s - %s\n", j+1, es.Quality, urlPreview)
		}

		if len(extractedStreams) > 0 {
			playStream := extractedStreams[0]
			fmt.Printf("\n  Playing: %s (%s)\n", playStream.Quality, playStream.Provider)

			opts := player.Options{
				Referrer: playStream.Referer,
				Title:    fmt.Sprintf("%s - Episode %s", episode.Anime.Name, episode.String()),
			}
			if len(playStream.Subtitles) > 0 {
				for _, sub := range playStream.Subtitles {
					opts.Subtitles = append(opts.Subtitles, sub.URL)
				}
			}

			if err := m.player.Launch(playStream.URL, opts); err != nil {
				fmt.Printf("  Playback failed: %v\n", err)
				continue
			}
			fmt.Println("  Playback started!")
			return nil
		}
	}

	fmt.Println("\nNo playable stream found")
	return nil
}

// combineResults merges Jikan results with AllAnime results by matching MAL ID
func (m *Mode) combineResults(jikanResults, allanimeResults []*source.Anime) []*source.Anime {
	if len(jikanResults) == 0 {
		return allanimeResults
	}

	// Create MAL ID -> AllAnime lookup
	malIDToAnime := make(map[string]*source.Anime)
	for _, aa := range allanimeResults {
		if aa.MALID > 0 {
			malIDToAnime[fmt.Sprintf("%d", aa.MALID)] = aa
		}
	}

	// For each Jikan result, try to find matching AllAnime by MAL ID
	for _, jikanAnime := range jikanResults {
		if jikanAnime.MALID > 0 {
			if aa, ok := malIDToAnime[fmt.Sprintf("%d", jikanAnime.MALID)]; ok {
				jikanAnime.AllAnimeID = aa.AllAnimeID
			}
		}
	}

	return jikanResults
}

// showAnimeListSimple shows anime list without MAL ID
func (m *Mode) showAnimeListSimple(animes []*source.Anime) {
	fmt.Println("Search results:")
	for i, a := range animes {
		year := ""
		if a.Year > 0 {
			year = fmt.Sprintf(" (%d)", a.Year)
		}
		typeStr := ""
		if a.Type != "" {
			typeStr = fmt.Sprintf(", %s", a.Type)
		}
		fmt.Printf("%d. %s%s%s\n", i+1, a.Name, year, typeStr)
	}
}

// showAnimeMetadata shows full Jikan metadata after selection
func (m *Mode) showAnimeMetadata(anime *source.Anime) {
	fmt.Printf("\nSelected: %s\n", anime.Name)

	if anime.Type != "" {
		fmt.Printf("  Type: %s\n", anime.Type)
	}
	if anime.EpisodeCount > 0 {
		fmt.Printf("  Episodes: %d\n", anime.EpisodeCount)
	}
	if anime.Status != "" {
		fmt.Printf("  Status: %s\n", anime.Status)
	}
	if anime.Rating != "" {
		fmt.Printf("  Rating: %s\n", anime.Rating)
	}
	if anime.Score > 0 {
		fmt.Printf("  Score: %.2f\n", anime.Score)
	}
	if anime.Rank > 0 {
		fmt.Printf("  Rank: #%d\n", anime.Rank)
	}
	if anime.Popularity > 0 {
		fmt.Printf("  Popularity: #%d\n", anime.Popularity)
	}
	if anime.Year > 0 {
		fmt.Printf("  Year: %d\n", anime.Year)
	}
	if len(anime.Genres) > 0 {
		fmt.Printf("  Genres: %s\n", strings.Join(anime.Genres, ", "))
	}
}

func (m *Mode) showSeasons(seasons []source.Season) {
	fmt.Println("\nSeasons:")
	for i, s := range seasons {
		fmt.Printf("%d. %s\n", i+1, s.Name)
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