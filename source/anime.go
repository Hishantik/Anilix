package source

type Anime struct {
	Name   string
	URL    string
	Cover  string
	Year   int
	Genres []string
	Status string
	Source Source
}

func (a *Anime) String() string {
	return a.Name
}