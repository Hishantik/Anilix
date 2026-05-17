package extractor

func init() {
	// Register extractors for common providers
	// Priority order: wixmp → youtube → sharepoint → filemoon → hianime → vidguard → streamwish → mp4upload → generic
	Register(NewWixmpExtractor())
	Register(NewYoutubeExtractor())
	Register(NewSharepointExtractor())
	Register(NewFilemoonExtractor())
	Register(NewHianimeExtractor())
	Register(NewVidguardExtractor())
	Register(NewStreamwishExtractor())
	Register(NewMp4uploadExtractor())
	// Generic extractor as fallback - scans any page for m3u8/mp4 URLs
	Register(NewGenericExtractor())
}
