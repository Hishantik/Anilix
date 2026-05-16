package extractor

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/anilix/anilix/source"
)

type StreamsbExtractor struct {
	client *http.Client
}

func NewStreamsbExtractor() *StreamsbExtractor {
	return &StreamsbExtractor{
		client: &http.Client{},
	}
}

func (e *StreamsbExtractor) Name() string {
	return "streamsb"
}

func (e *StreamsbExtractor) CanHandle(url string) bool {
	lower := strings.ToLower(url)
	return strings.Contains(lower, "streamsb.com") ||
		strings.Contains(lower, "streamtape.com") ||
		strings.Contains(lower, "ok.ru/videoembed") ||
		strings.Contains(lower, "vk.com/video_ext")
}

func (e *StreamsbExtractor) Extract(ctx context.Context, url, referer string) ([]*source.Stream, error) {
	// Handle ok.ru videoembed URLs
	if strings.Contains(url, "ok.ru/videoembed") {
		return e.extractOkRu(ctx, url, referer)
	}

	// Handle streamsb URLs
	if strings.Contains(url, "streamsb.com") || strings.Contains(url, "streamtape.com") {
		return e.extractStreamsb(ctx, url, referer)
	}

	// Handle vk.com URLs
	if strings.Contains(url, "vk.com/video_ext") {
		return e.extractVk(ctx, url, referer)
	}

	return nil, fmt.Errorf("unsupported URL: %s", url)
}

func (e *StreamsbExtractor) extractOkRu(ctx context.Context, url, referer string) ([]*source.Stream, error) {
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

	// Try to find the video metadata in the page
	// Look for player params that contain the video URL
	videoURL := e.extractOkRuVideoURL(html)
	if videoURL != "" {
		return []*source.Stream{
			{
				Provider: "streamsb",
				Quality:  "auto",
				URL:      videoURL,
				Referer:  url,
			},
		}, nil
	}

	return nil, fmt.Errorf("no video URL found")
}

func (e *StreamsbExtractor) extractOkRuVideoURL(html string) string {
	// Look for flashvars or player params
	patterns := []string{
		`flashvars\s*=\s*["']([^"']+)["']`,
		`data-flashvars=["']([^"']+)["']`,
		`url\s*:\s*["']([^"']+\.m3u8[^"']*)["']`,
		`src\s*:\s*["']([^"']+\.m3u8[^"']*)["']`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(html)
		if len(matches) > 1 {
			// Could be a URL directly or need decoding
			candidate := matches[1]
			if strings.Contains(candidate, "http") {
				return candidate
			}
		}
	}

	// Look for m3u8 in the HTML directly
	m3u8Pattern := `(https?://[^\s"']+\.m3u8[^\s"']*)`
	re := regexp.MustCompile(m3u8Pattern)
	matches := re.FindStringSubmatch(html)
	if len(matches) > 1 {
		return matches[1]
	}

	return ""
}

func (e *StreamsbExtractor) extractStreamsb(ctx context.Context, url, referer string) ([]*source.Stream, error) {
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

	// Look for the video URL in streamsb pages
	videoURL := e.extractStreamsbVideoURL(html, url)
	if videoURL != "" {
		// If it's an m3u8, parse it
		if strings.Contains(videoURL, ".m3u8") {
			return e.extractFromM3U8(ctx, videoURL, url)
		}
		return []*source.Stream{
			{
				Provider: "streamsb",
				Quality:  "auto",
				URL:      videoURL,
				Referer:  url,
			},
		}, nil
	}

	return nil, fmt.Errorf("no video URL found")
}

func (e *StreamsbExtractor) extractStreamsbVideoURL(html, baseURL string) string {
	// Look for various player configurations
	patterns := []string{
		`sources\s*:\s*\[[^\]]*file\s*:\s*["']([^"']+)["']`,
		`file\s*:\s*["']([^"']+\.m3u8[^"']*)["']`,
		`video\s*:\s*["']([^"']+)["']`,
		`src\s*:\s*["']([^"']+\.m3u8[^"']*)["']`,
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

	// Look for direct m3u8 URLs
	m3u8Pattern := `(https?://[^\s"']+\.m3u8[^\s"']*)`
	re := regexp.MustCompile(m3u8Pattern)
	matches := re.FindStringSubmatch(html)
	if len(matches) > 1 {
		return matches[1]
	}

	return ""
}

func (e *StreamsbExtractor) extractVk(ctx context.Context, url, referer string) ([]*source.Stream, error) {
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

	// Look for video URL in VK page
	videoURL := e.extractVkVideoURL(html)
	if videoURL != "" {
		return []*source.Stream{
			{
				Provider: "streamsb",
				Quality:  "auto",
				URL:      videoURL,
				Referer:  url,
			},
		}, nil
	}

	return nil, fmt.Errorf("no video URL found")
}

func (e *StreamsbExtractor) extractVkVideoURL(html string) string {
	// Look for mp4 URLs
	mp4Pattern := `(https?://[^\s"']+\.mp4[^\s"']*)`
	re := regexp.MustCompile(mp4Pattern)
	matches := re.FindStringSubmatch(html)
	if len(matches) > 1 {
		return matches[1]
	}

	// Look for m3u8 URLs
	m3u8Pattern := `(https?://[^\s"']+\.m3u8[^\s"']*)`
	re = regexp.MustCompile(m3u8Pattern)
	matches = re.FindStringSubmatch(html)
	if len(matches) > 1 {
		return matches[1]
	}

	return ""
}

func (e *StreamsbExtractor) extractFromM3U8(ctx context.Context, m3u8URL, referer string) ([]*source.Stream, error) {
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
			Provider: "streamsb",
			Quality:  v.Quality,
			URL:      v.URL,
			Referer:  referer,
		})
	}

	return streams, nil
}

func (e *StreamsbExtractor) ExtractSubtitles(ctx context.Context, url, referer string) ([]string, error) {
	return nil, nil
}