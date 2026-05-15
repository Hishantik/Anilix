package source

type Stream struct {
	Quality   string
	URL       string
	Provider  string
	Referer   string // Required for some providers
	Subtitles []Subtitle
}

type Subtitle struct {
	Language string
	URL      string
}