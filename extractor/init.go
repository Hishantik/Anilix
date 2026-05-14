package extractor

func init() {
	// Register extractors in priority order (lower index = higher priority)
	Register(NewHianimeExtractor())
	Register(NewFilemoonExtractor())
	Register(NewWixmpExtractor())
	Register(NewYoutubeExtractor())
}