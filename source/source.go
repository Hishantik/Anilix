package source

// Source is the interface that all anime sources must implement.
type Source interface {
	Name() string
	Search(query string) ([]*Anime, error)
	EpisodesOf(anime *Anime) ([]*Episode, error)
	StreamsOf(episode *Episode) ([]*Stream, error)
	ID() string
}