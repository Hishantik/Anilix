package extractor

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/anilix/anilix/source"
)

const wixmpReferer = "https://wixmp.com/"

type WixmpExtractor struct {
	client *http.Client
}

func NewWixmpExtractor() *WixmpExtractor {
	return &WixmpExtractor{
		client: &http.Client{},
	}
}

func (e *WixmpExtractor) Name() string {
	return "wixmp"
}

func (e *WixmpExtractor) CanHandle(url string) bool {
	lower := strings.ToLower(url)
	return strings.Contains(lower, "wixmp") ||
		strings.Contains(lower, "wixms") ||
		strings.Contains(lower, "vidguard")
}

func (e *WixmpExtractor) Extract(ctx context.Context, url, referer string) ([]*source.Stream, error) {
	if referer == "" {
		referer = wixmpReferer
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", referer)
	req.Header.Set("Origin", "https://wixmp.com")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	html, err := readBodyHTML(resp)
	if err != nil {
		return nil, err
	}

	// Try to find m3u8 or mp4 URL
	m3u8URL := e.extractM3U8URL(html)
	if m3u8URL != "" {
		return e.extractFromM3U8(ctx, m3u8URL, referer)
	}

	// Try direct MP4 URL
	mp4URL := e.extractMP4URL(html)
	if mp4URL != "" {
		return []*source.Stream{{
			Provider: "wixmp",
			Quality:  "auto",
			URL:      mp4URL,
			Referer:  referer,
		}}, nil
	}

	return nil, fmt.Errorf("no stream data found")
}

func (e *WixmpExtractor) extractFromM3U8(ctx context.Context, m3u8URL, referer string) ([]*source.Stream, error) {
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
			Provider: "wixmp",
			Quality:  v.Quality,
			URL:      v.URL,
			Referer:  referer,
		})
	}

	return streams, nil
}

func (e *WixmpExtractor) extractM3U8URL(html string) string {
	re := regexp.MustCompile(`(?i)(https?://[^\s"']+\.m3u8[^\s"']*)`)
	matches := re.FindStringSubmatch(html)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func (e *WixmpExtractor) extractMP4URL(html string) string {
	re := regexp.MustCompile(`(?i)source\s*:\s*["']([^"']+\.mp4[^"']*)["']`)
	matches := re.FindStringSubmatch(html)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func (e *WixmpExtractor) ExtractSubtitles(ctx context.Context, url, referer string) ([]string, error) {
	if referer == "" {
		referer = wixmpReferer
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