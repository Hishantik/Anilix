package tui

import (
	"time"

	"github.com/hishantik/anilix/source"
)

// TUI state machine states
type tuiState int

const (
	searchState tuiState = iota
	episodesState
	confirmQuitState
)

// SelectionResult holds the selected anime and episode
type SelectionResult struct {
	Anime   *source.Anime
	Episode string
}

// SearchState holds the state for the anime search TUI
type SearchState struct {
	Query           string
	Results         []*source.Anime
	Selected        int
	Metadata        *MetadataPanel
	Loading         bool
	MetadataLoading bool
	Err             error
	TranslationType string
}

// MetadataPanel holds merged metadata to display on the right panel
type MetadataPanel struct {
	Title        string
	TitleEnglish string
	TitleNative  string
	Cover        string
	Year         int
	Type         string
	Status       string
	Episodes     int
	Score        float64
	Rank         int
	Popularity   int
	Genres       []string
	Synopsis     string
	Source       string // "Jikan", "AniList", or "Jikan + AniList"
}

// EpisodeState holds the state for episode selection
type EpisodeState struct {
	AnimeID           string
	Episodes          []string
	EpisodeTitles     []string
	Selected          int
	Loading           bool
	Err               error
	EpisodeMetadata   *EpisodeMetadataPanel
	MetadataLoading   bool
	Playing           bool
}

// EpisodeMetadataPanel holds metadata for a single episode
type EpisodeMetadataPanel struct {
	Title         string
	TitleJapanese string
	Aired         string
	Score         float64
	Filler        bool
	Recap         bool
	Synopsis      string
	Duration      int
}

// EpisodeMetadataFetchTriggered is sent when metadata fetch is triggered
type EpisodeMetadataFetchTriggered struct {
	Index int
}

// EpisodeMetadataLoadedMsg is sent when episode metadata is loaded
type EpisodeMetadataLoadedMsg struct {
	Metadata *EpisodeMetadataPanel
	Index    int
}

// NewSearchState creates a new search state
func NewSearchState() *SearchState {
	return &SearchState{
		Query:           "",
		Results:         nil,
		Selected:        0,
		Metadata:        nil,
		Loading:         false,
		Err:             nil,
		TranslationType: "sub",
	}
}

// NewEpisodeState creates a new episode state
func NewEpisodeState() *EpisodeState {
	return &EpisodeState{
		Episodes: nil,
		Selected: 0,
		Loading:  false,
	}
}

// debounceSearch triggers a search after a delay
func debounceSearch(delay time.Duration, fn func()) {
	time.Sleep(delay)
	fn()
}
