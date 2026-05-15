package extractor

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/anilix/anilix/source"
)

const anihdplayReferer = "https://anihdplay.com/"

type AnihdplayExtractor struct {
	client *http.Client
}

func NewAnihdplayExtractor() *AnihdplayExtractor {
	return &AnihdplayExtractor{
		client: &http.Client{},
	}
}

func (e *AnihdplayExtractor) Name() string {
	return "anihdplay"
}

func (e *AnihdplayExtractor) CanHandle(url string) bool {
	lower := strings.ToLower(url)
	return strings.Contains(lower, "anihdplay.com")
}

func (e *AnihdplayExtractor) Extract(ctx context.Context, url, referer string) ([]*source.Stream, error) {
	if referer == "" {
		referer = anihdplayReferer
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

	// Try to find the video URL in the page
	// Look for iframe src, video src, or direct mp4 links
	videoURL := e.extractVideoURL(html)
	if videoURL == "" {
		return nil, fmt.Errorf("no video URL found")
	}

	// Check if it's a direct mp4 or needs further extraction
	if strings.HasSuffix(videoURL, ".mp4") {
		return []*source.Stream{
			{
				Provider: "anihdplay",
				Quality:  "auto",
				URL:      videoURL,
				Referer:  url,
			},
		}, nil
	}

	// If it's another URL, try to resolve it
	return e.extractFromURL(ctx, videoURL, url)
}

func (e *AnihdplayExtractor) extractVideoURL(html string) string {
	// Look for iframe with src
	iframePatterns := []string{
		`iframe[^>]+src=["']([^"']+)["']`,
		`<iframe[^>]*>\s*<iframe[^>]+src=["']([^"']+)["']`,
	}

	for _, pattern := range iframePatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(html)
		if len(matches) > 1 {
			return matches[1]
		}
	}

	// Look for video tag
	videoPatterns := []string{
		`<video[^>]+src=["']([^"']+)["']`,
		`video\s*:\s*["']([^"']+)["']`,
		`file\s*:\s*["']([^"']+\.mp4[^"']*)["']`,
	}

	for _, pattern := range videoPatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(html)
		if len(matches) > 1 {
			return matches[1]
		}
	}

	// Look for direct mp4 URL
	mp4Pattern := `(https?://[^\s"']+\.mp4[^\s"']*)`
	re := regexp.MustCompile(mp4Pattern)
	matches := re.FindStringSubmatch(html)
	if len(matches) > 1 {
		return matches[1]
	}

	// Look for source tag
	sourcePatterns := []string{
		`<source[^>]+src=["']([^"']+)["']`,
	}

	for _, pattern := range sourcePatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(html)
		if len(matches) > 1 {
			return matches[1]
		}
	}

	// Look for data-src attributes
	dataSrcPattern := `data-src=["']([^"']+)["']`
	re = regexp.MustCompile(dataSrcPattern)
	matches = re.FindStringSubmatch(html)
	if len(matches) > 1 {
		return matches[1]
	}

	return ""
}

func (e *AnihdplayExtractor) extractFromURL(ctx context.Context, videoURL, referer string) ([]*source.Stream, error) {
	// If it's an m3u8, parse it
	if strings.Contains(videoURL, ".m3u8") {
		return e.extractFromM3U8(ctx, videoURL, referer)
	}

	// If it's another page, fetch and try again
	req, err := http.NewRequestWithContext(ctx, "GET", videoURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", referer)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	html, err := readBody(resp)
	if err != nil {
		return nil, err
	}

	// Try to find video URL in the new page
	videoURL = e.extractVideoURL(html)
	if videoURL == "" {
		return nil, fmt.Errorf("no video URL found in nested page")
	}

	// Recursively handle
	return e.extractFromURL(ctx, videoURL, videoURL)
}

func (e *AnihdplayExtractor) extractFromM3U8(ctx context.Context, m3u8URL, referer string) ([]*source.Stream, error) {
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
			Provider: "anihdplay",
			Quality:  v.Quality,
			URL:      v.URL,
			Referer:  referer,
		})
	}

	return streams, nil
}

func (e *AnihdplayExtractor) ExtractSubtitles(ctx context.Context, url, referer string) ([]string, error) {
	return nil, nil
}