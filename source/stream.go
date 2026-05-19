package source

type Stream struct {
	Quality       string
	URL           string
	Provider      string
	Referer       string // Required for some providers
	Subtitles     []Subtitle
	NeedsReferrer bool   // True if stream requires Referer header to play
}

type Subtitle struct {
	Language string
	URL      string
}