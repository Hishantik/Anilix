package source

type Anime struct {
	Name   string
	URL    string
	Cover  string
	Year   int
	Genres []string
	Status string
	MALID  int    // MyAnimeList ID for metadata linkage
	Source Source
}

func (a *Anime) String() string {
	return a.Name
}