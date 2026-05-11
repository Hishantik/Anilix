package source

type Stream struct {
	Quality   string
	URL       string
	Provider  string
	Subtitles []Subtitle
}

type Subtitle struct {
	Language string
	URL      string
}