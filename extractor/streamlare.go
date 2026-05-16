package extractor

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/anilix/anilix/source"
)

type StreamlareExtractor struct {
	client *http.Client
}

func NewStreamlareExtractor() *StreamlareExtractor {
	return &StreamlareExtractor{
		client: &http.Client{},
	}
}

func (e *StreamlareExtractor) Name() string {
	return "streamlare"
}

func (e *StreamlareExtractor) CanHandle(url string) bool {
	lower := strings.ToLower(url)
	return strings.Contains(lower, "streamlare.com") ||
		strings.Contains(lower, "sl-mp4")
}

func (e *StreamlareExtractor) Extract(ctx context.Context, url, referer string) ([]*source.Stream, error) {
	if referer == "" {
		referer = "https://streamlare.com/"
	}

	// If it's a clock URL, we can't handle it
	if strings.Contains(url, "/clock") {
		return nil, fmt.Errorf("clock URLs not supported")
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

	// Look for video URL in the page
	videoURL := e.extractVideoURL(html)
	if videoURL != "" {
		return []*source.Stream{
			{
				Provider: "streamlare",
				Quality:  "auto",
				URL:      videoURL,
				Referer:  url,
			},
		}, nil
	}

	return nil, fmt.Errorf("no video URL found")
}

func (e *StreamlareExtractor) extractVideoURL(html string) string {
	// Look for video URL in various patterns
	patterns := []string{
		`video\s*:\s*["']([^"']+)["']`,
		`file\s*:\s*["']([^"']+\.mp4[^"']*)["']`,
		`source\s*:\s*["']([^"']+)["']`,
		`(https?://[^\s"']+\.mp4[^\s"']*)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(html)
		if len(matches) > 1 {
			url := matches[1]
			if strings.HasPrefix(url, "//") {
				url = "https:" + url
			}
			if strings.Contains(url, "http") {
				return url
			}
		}
	}

	return ""
}

func (e *StreamlareExtractor) ExtractSubtitles(ctx context.Context, url, referer string) ([]string, error) {
	return nil, nil
}