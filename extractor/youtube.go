package extractor

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/anilix/anilix/source"
)

const youtubeReferer = "https://www.youtube.com/"

type YoutubeExtractor struct {
	client *http.Client
}

func NewYoutubeExtractor() *YoutubeExtractor {
	return &YoutubeExtractor{
		client: &http.Client{},
	}
}

func (e *YoutubeExtractor) Name() string {
	return "youtube"
}

func (e *YoutubeExtractor) CanHandle(url string) bool {
	lower := strings.ToLower(url)
	return strings.Contains(lower, "youtube.com") ||
		strings.Contains(lower, "youtu.be") ||
		strings.Contains(lower, "yewtu.be") ||
		strings.Contains(lower, "invidious")
}

func (e *YoutubeExtractor) Extract(ctx context.Context, url, referer string) ([]*source.Stream, error) {
	if referer == "" {
		referer = youtubeReferer
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

	html, err := readBodyHTML(resp)
	if err != nil {
		return nil, err
	}

	// Try to find direct video URL (mp4)
	videoURL := e.extractVideoURL(html)
	if videoURL != "" {
		return []*source.Stream{{
			Provider: "youtube",
			Quality:  "auto",
			URL:      videoURL,
			Referer:  referer,
		}}, nil
	}

	// Try m3u8 (for yewtu.be or invidious instances)
	m3u8URL := e.extractM3U8URL(html)
	if m3u8URL != "" {
		return e.extractFromM3U8(ctx, m3u8URL, referer)
	}

	return nil, fmt.Errorf("no stream data found")
}

func (e *YoutubeExtractor) extractFromM3U8(ctx context.Context, m3u8URL, referer string) ([]*source.Stream, error) {
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
			Provider: "youtube",
			Quality:  v.Quality,
			URL:      v.URL,
			Referer:  referer,
		})
	}

	return streams, nil
}

func (e *YoutubeExtractor) extractVideoURL(html string) string {
	// Look for direct mp4 link
	re := regexp.MustCompile(`(?i)href\s*=\s*["']([^"']*\.mp4[^"']*)["']`)
	matches := re.FindStringSubmatch(html)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func (e *YoutubeExtractor) extractM3U8URL(html string) string {
	re := regexp.MustCompile(`(?i)(https?://[^\s"']+\.m3u8[^\s"']*)`)
	matches := re.FindStringSubmatch(html)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func (e *YoutubeExtractor) ExtractSubtitles(ctx context.Context, url, referer string) ([]string, error) {
	if referer == "" {
		referer = youtubeReferer
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
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

	html, err := readBodyHTML(resp)
	if err != nil {
		return nil, err
	}

	return extractSubtitlesFromHTML(html), nil
}