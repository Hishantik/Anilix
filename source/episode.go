package source

import "strconv"

type Episode struct {
	Number float64
	Title  string
	URL    string
	Anime  *Anime
}

func (e *Episode) String() string {
	if e.Title != "" {
		return e.Title
	}
	if e.Number == float64(int(e.Number)) {
		return strconv.Itoa(int(e.Number))
	}
	return strconv.FormatFloat(e.Number, 'f', 1, 64)
}