package extractor

import (
	"context"

	"github.com/anilix/anilix/source"
)

// Extractor extracts streams from provider URLs
type Extractor interface {
	// Name returns the provider name
	Name() string

	// CanHandle returns true if this extractor can handle the URL
	CanHandle(url string) bool

	// Extract extracts streams from the provider URL
	Extract(ctx context.Context, url, referer string) ([]*source.Stream, error)

	// ExtractSubtitles extracts subtitle URLs from the provider page (optional)
	ExtractSubtitles(ctx context.Context, url, referer string) ([]string, error)
}

// NoSubtitlesExtractor wraps an extractor to return empty subtitles
type NoSubtitlesExtractor struct {
	Extractor
}

func (e *NoSubtitlesExtractor) ExtractSubtitles(ctx context.Context, url, referer string) ([]string, error) {
	return nil, nil
}

// Priority determines extraction order (lower = higher priority)
var ProviderPriority = map[string]int{
	"hianime":  1,
	"filemoon": 2,
	"wixmp":    3,
	"youtube":  4,
}

// Default extractors
var extractors []Extractor

// Register adds an extractor to the list
func Register(e Extractor) {
	extractors = append(extractors, e)
}

// All returns all registered extractors
func All() []Extractor {
	return extractors
}

// Resolve finds an extractor that can handle the URL
func Resolve(url string) Extractor {
	for _, e := range extractors {
		if e.CanHandle(url) {
			return e
		}
	}
	return nil
}