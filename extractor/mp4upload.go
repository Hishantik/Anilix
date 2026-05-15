package extractor

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/anilix/anilix/source"
)

type Mp4uploadExtractor struct {
	client *http.Client
}

func NewMp4uploadExtractor() *Mp4uploadExtractor {
	return &Mp4uploadExtractor{
		client: &http.Client{},
	}
}

func (e *Mp4uploadExtractor) Name() string {
	return "mp4upload"
}

func (e *Mp4uploadExtractor) CanHandle(url string) bool {
	lower := strings.ToLower(url)
	return strings.Contains(lower, "mp4upload.com")
}

func (e *Mp4uploadExtractor) Extract(ctx context.Context, url, referer string) ([]*source.Stream, error) {
	if referer == "" {
		referer = "https://mp4upload.com/"
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

	// Look for video URL in the embed page
	videoURL := e.extractVideoURL(html)
	if videoURL == "" {
		return nil, fmt.Errorf("no video URL found")
	}

	return []*source.Stream{
		{
			Provider: "mp4upload",
			Quality:  "auto",
			URL:      videoURL,
			Referer:  url,
		},
	}, nil
}

func (e *Mp4uploadExtractor) extractVideoURL(html string) string {
	// Look for video URL in script
	patterns := []string{
		`video\.src\s*=\s*["']([^"']+)["']`,
		`video\s*:\s*\{.*?src\s*:\s*["']([^"']+)["']`,
		`file\s*:\s*["']([^"']+\.mp4[^"']*)["']`,
		`(https?://[^\s"']+\.mp4[^\s"']*)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(html)
		if len(matches) > 1 && matches[1] != "" {
			url := matches[1]
			if strings.HasPrefix(url, "//") {
				url = "https:" + url
			}
			return url
		}
	}

	return ""
}

func (e *Mp4uploadExtractor) ExtractSubtitles(ctx context.Context, url, referer string) ([]string, error) {
	return nil, nil
}