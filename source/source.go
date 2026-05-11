package source

type Season struct {
	Number int
	Name   string  // e.g., "Season 1", "Season 2 - Cour 1"
}

// Source is the interface that all anime sources must implement.
type Source interface {
	Name() string
	Search(query string) ([]*Anime, error)
	SeasonsOf(anime *Anime) ([]Season, error)
	EpisodesOf(anime *Anime, season int) ([]*Episode, error)
	StreamsOf(episode *Episode) ([]*Stream, error)
	ID() string
}