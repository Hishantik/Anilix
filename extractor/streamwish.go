package extractor

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/anilix/anilix/curl"
	"github.com/anilix/anilix/source"
)

const streamwishReferer = "https://streamwish.to/"

type StreamwishExtractor struct{}

func NewStreamwishExtractor() *StreamwishExtractor {
	return &StreamwishExtractor{}
}

func (e *StreamwishExtractor) Name() string {
	return "streamwish"
}

func (e *StreamwishExtractor) CanHandle(url string) bool {
	lower := strings.ToLower(url)
	return strings.Contains(lower, "streamwish") ||
		strings.Contains(lower, "bysekoze.com") ||
		strings.Contains(lower, "listeamed.net") ||
		strings.Contains(lower, "strmup.cc")
}

func (e *StreamwishExtractor) Extract(ctx context.Context, url, referer string) ([]*source.Stream, error) {
	if referer == "" {
		referer = streamwishReferer
	}

	headers := map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		"Referer":    referer,
	}

	html, err := curl.Get(ctx, url, headers)
	if err != nil {
		return nil, fmt.Errorf("curl failed: %w", err)
	}

	// Try to find m3u8 URL
	m3u8URL := e.extractM3U8URL(html)
	if m3u8URL != "" {
		return e.extractFromM3U8(ctx, m3u8URL, referer)
	}

	// Try to find mp4 URL
	mp4URL := e.extractMP4URL(html)
	if mp4URL != "" {
		return []*source.Stream{{
			Provider: "streamwish",
			Quality:  "auto",
			URL:      mp4URL,
			Referer:  referer,
		}}, nil
	}

	return nil, fmt.Errorf("no stream data found")
}

func (e *StreamwishExtractor) extractFromM3U8(ctx context.Context, m3u8URL, referer string) ([]*source.Stream, error) {
	variants, err := ParseMasterPlaylistCurl(ctx, m3u8URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse m3u8: %w", err)
	}

	if len(variants) == 0 {
		return nil, fmt.Errorf("no variants found")
	}

	streams := make([]*source.Stream, 0, len(variants))
	for _, v := range variants {
		streams = append(streams, &source.Stream{
			Provider: "streamwish",
			Quality:  v.Quality,
			URL:      v.URL,
			Referer:  referer,
		})
	}

	return streams, nil
}

func (e *StreamwishExtractor) extractM3U8URL(html string) string {
	patterns := []string{
		`(?i)(https?://[^\s"']+\.m3u8[^\s"']*)`,
		`(?i)file\s*:\s*["']([^"']+\.m3u8[^"']*)["']`,
		`(?i)src\s*:\s*["']([^"']+\.m3u8[^"']*)["']`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(html)
		if len(matches) > 1 {
			return matches[1]
		}
	}

	return ""
}

func (e *StreamwishExtractor) extractMP4URL(html string) string {
	re := regexp.MustCompile(`(?i)(https?://[^\s"']+\.mp4[^\s"']*)`)
	matches := re.FindStringSubmatch(html)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func (e *StreamwishExtractor) ExtractSubtitles(ctx context.Context, url, referer string) ([]string, error) {
	return nil, nil
}
