package extractor

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/anilix/anilix/source"
)

const vidstreamingReferer = "https://vidstreaming.io/"

type VidstreamingExtractor struct {
	client *http.Client
}

func NewVidstreamingExtractor() *VidstreamingExtractor {
	return &VidstreamingExtractor{
		client: &http.Client{},
	}
}

func (e *VidstreamingExtractor) Name() string {
	return "vidstreaming"
}

func (e *VidstreamingExtractor) CanHandle(url string) bool {
	lower := strings.ToLower(url)
	return strings.Contains(lower, "vidstreaming.io") ||
		strings.Contains(lower, "vidstream") ||
		strings.Contains(lower, "vid-mp4")
}

func (e *VidstreamingExtractor) Extract(ctx context.Context, url, referer string) ([]*source.Stream, error) {
	if referer == "" {
		referer = vidstreamingReferer
	}

	// Fix relative URL
	if strings.HasPrefix(url, "//") {
		url = "https:" + url
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", referer)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	html, err := readBody(resp)
	if err != nil {
		return nil, err
	}

	// Look for video sources in the page
	videoURL := e.extractVideoURL(html)
	if videoURL == "" {
		return nil, fmt.Errorf("no video URL found")
	}

	// If it's an m3u8, parse it
	if strings.Contains(videoURL, ".m3u8") {
		return e.extractFromM3U8(ctx, videoURL, url)
	}

	// Return as direct stream
	return []*source.Stream{
		{
			Provider: "vidstreaming",
			Quality:  "auto",
			URL:      videoURL,
			Referer:  url,
		},
	}, nil
}

func (e *VidstreamingExtractor) extractVideoURL(html string) string {
	// Look for sources in JSON format
	patterns := []string{
		`"file"\s*:\s*"([^"]+\.m3u8[^"]*)"`,
		`"source"\s*:\s*"([^"]+)"`,
		`"url"\s*:\s*"([^"]+\.(?:m3u8|mp4)[^"]*)"`,
		`file\s*:\s*['"]([^']+\.m3u8[^']*)['"]`,
		`sources\s*:\s*\[.*?"file"\s*:\s*"([^"]+)"`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(html)
		if len(matches) > 1 && matches[1] != "" {
			return matches[1]
		}
	}

	// Look for direct m3u8 URL
	m3u8Re := regexp.MustCompile(`(https?://[^\s"']+\.m3u8[^\s"']*)`)
	matches := m3u8Re.FindStringSubmatch(html)
	if len(matches) > 1 {
		return matches[1]
	}

	// Look for iframe
	iframeRe := regexp.MustCompile(`<iframe[^>]+src=["']([^"']+)["']`)
	matches = iframeRe.FindStringSubmatch(html)
	if len(matches) > 1 {
		return matches[1]
	}

	return ""
}

func (e *VidstreamingExtractor) extractFromM3U8(ctx context.Context, m3u8URL, referer string) ([]*source.Stream, error) {
	variants, err := ParseMasterPlaylist(m3u8URL, e.client)
	if err != nil {
		return nil, fmt.Errorf("failed to parse m3u8: %w", err)
	}

	if len(variants) == 0 {
		return nil, fmt.Errorf("no variants found")
	}

	streams := make([]*source.Stream, 0, len(variants))
	for _, v := range variants {
		streams = append(streams, &source.Stream{
			Provider: "vidstreaming",
			Quality:  v.Quality,
			URL:      v.URL,
			Referer:  referer,
		})
	}

	return streams, nil
}

func (e *VidstreamingExtractor) ExtractSubtitles(ctx context.Context, url, referer string) ([]string, error) {
	return nil, nil
}