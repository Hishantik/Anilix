package tui

import (
	"github.com/hishantik/anilix/provider/jikan"
	"github.com/hishantik/anilix/source"
)

type SearchResultsMsg struct {
	Results []*source.Anime
}

type SearchErrorMsg struct {
	Err error
}

type MetadataFetchTriggered struct {
	Index int
}

type metadataDebounceMsg struct {
	Index int
}

type episodeMetadataDebounceMsg struct {
	Index int
}

// MetadataLoadedMsg is sent when a single API source returns.
type MetadataLoadedMsg struct {
	Metadata *MetadataPanel
	Index    int
}

// MetadataBatchLoadedMsg carries all metadata for a search result set.
type MetadataBatchLoadedMsg struct {
	MetadataMap map[int]*MetadataPanel
}

type EpisodesLoadedMsg struct {
	Episodes      []string
	EpisodeTitles []string
	Error         error
}

type EpisodeMetadataFetchTriggered struct {
	Index int
}

type EpisodeMetadataLoadedMsg struct {
	Metadata   *EpisodeMetadataPanel
	Index      int
	CacheKey   string         `json:"-"`
	RawEpisode *jikan.Episode `json:"-"`
}

type PlayStreamMsg struct{}

type TUIErrorMsg struct {
	Err error
}

type progressTickMsg struct{}
