package tui

import (
	"time"

	"github.com/anilix/anilix/source"
)

// TUI state machine states
type tuiState int

const (
	searchState tuiState = iota
	episodesState
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

// MetadataPanel holds Jikan metadata to display on the right panel
type MetadataPanel struct {
	Title        string
	TitleEnglish string
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
}

// EpisodeState holds the state for episode selection
type EpisodeState struct {
	AnimeID       string
	Episodes      []string
	EpisodeTitles []string
	Selected      int
	Loading       bool
	Err           error
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
