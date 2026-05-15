package source

type Anime struct {
	Name         string
	URL          string
	Cover        string
	Year         int
	Genres       []string
	Status       string
	MALID        int    // MyAnimeList ID for metadata linkage
	AllAnimeID   string // AllAnime show ID for episode/streams
	EpisodeCount int    // Total episode count
	Type         string // "TV", "Movie", "OVA", etc.
	Rating       string // "PG-13 - Teens 13 or older"
	Score        float64
	Rank         int
	Popularity   int
	Source       Source
}

func (a *Anime) String() string {
	return a.Name
}