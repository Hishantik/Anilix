package extractor

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/anilix/anilix/curl"
	"github.com/anilix/anilix/source"
)

const sharepointReferer = "https://sharepoint.com/"

type SharepointExtractor struct{}

func NewSharepointExtractor() *SharepointExtractor {
	return &SharepointExtractor{}
}

func (e *SharepointExtractor) Name() string {
	return "sharepoint"
}

func (e *SharepointExtractor) CanHandle(url string) bool {
	lower := strings.ToLower(url)
	return strings.Contains(lower, "sharepoint") ||
		strings.Contains(lower, "s-mp4") ||
		strings.Contains(lower, "streamtape")
}

func (e *SharepointExtractor) Extract(ctx context.Context, url, referer string) ([]*source.Stream, error) {
	if referer == "" {
		referer = sharepointReferer
	}

	headers := map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		"Referer":    referer,
	}

	html, err := curl.Get(ctx, url, headers)
	if err != nil {
		return nil, fmt.Errorf("curl failed: %w", err)
	}

	// Try to find direct mp4 URL
	mp4URL := e.extractMP4URL(html)
	if mp4URL != "" {
		return []*source.Stream{{
			Provider: "sharepoint",
			Quality:  "auto",
			URL:      mp4URL,
			Referer:  referer,
		}}, nil
	}

	// Try to find m3u8 URL
	m3u8URL := e.extractM3U8URL(html)
	if m3u8URL != "" {
		return e.extractFromM3U8(ctx, m3u8URL, referer)
	}

	// Look for video player source URLs
	playerURL := e.extractPlayerSource(html)
	if playerURL != "" {
		return []*source.Stream{{
			Provider: "sharepoint",
			Quality:  "auto",
			URL:      playerURL,
			Referer:  referer,
		}}, nil
	}

	return nil, fmt.Errorf("no stream data found")
}

func (e *SharepointExtractor) extractFromM3U8(ctx context.Context, m3u8URL, referer string) ([]*source.Stream, error) {
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
			Provider: "sharepoint",
			Quality:  v.Quality,
			URL:      v.URL,
			Referer:  referer,
		})
	}

	return streams, nil
}

func (e *SharepointExtractor) extractMP4URL(html string) string {
	patterns := []string{
		`(?i)src\s*:\s*["']([^"']+\.mp4[^"']*)["']`,
		`(?i)file\s*:\s*["']([^"']+\.mp4[^"']*)["']`,
		`(?i)"url"\s*:\s*"([^"]+\.mp4[^"]*)"`,
		`(?i)video_src\s*=\s*["']([^"']+\.mp4[^"']*)["']`,
		`https?://[^\s"']+\.mp4[^\s"']*`,
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

func (e *SharepointExtractor) extractM3U8URL(html string) string {
	re := regexp.MustCompile(`(?i)(https?://[^\s"']+\.m3u8[^\s"']*)`)
	matches := re.FindStringSubmatch(html)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func (e *SharepointExtractor) extractPlayerSource(html string) string {
	patterns := []string{
		`(?i)<source[^>]+src\s*=\s*["']([^"']+)["']`,
		`(?i)player\.src\(\{[^}]*src\s*:\s*["']([^"']+)["']`,
		`(?i)jwplayer\([^)]*\)\.setup\(\{[^}]*file\s*:\s*["']([^"']+)["']`,
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

func (e *SharepointExtractor) ExtractSubtitles(ctx context.Context, url, referer string) ([]string, error) {
	return nil, nil
}
