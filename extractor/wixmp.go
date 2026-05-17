package extractor

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/anilix/anilix/curl"
	"github.com/anilix/anilix/source"
)

const wixmpReferer = "https://wixmp.com/"

type WixmpExtractor struct{}

func NewWixmpExtractor() *WixmpExtractor {
	return &WixmpExtractor{}
}

func (e *WixmpExtractor) Name() string {
	return "wixmp"
}

func (e *WixmpExtractor) CanHandle(url string) bool {
	lower := strings.ToLower(url)
	return strings.Contains(lower, "wixmp") ||
		strings.Contains(lower, "wixms") ||
		strings.Contains(lower, "vidguard") ||
		strings.Contains(lower, "repackager.wixmp.com")
}

func (e *WixmpExtractor) Extract(ctx context.Context, url, referer string) ([]*source.Stream, error) {
	if referer == "" {
		referer = wixmpReferer
	}

	// Handle repackager.wixmp.com URLs with quality variants (like ani-cli)
	if strings.Contains(url, "repackager.wixmp.com") {
		return e.extractWixmpVariants(ctx, url, referer)
	}

	headers := map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		"Referer":    referer,
		"Origin":     "https://wixmp.com",
	}

	html, err := curl.Get(ctx, url, headers)
	if err != nil {
		return nil, fmt.Errorf("curl failed: %w", err)
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

// extractWixmpVariants handles repackager.wixmp.com URLs with multiple quality variants
// URL format: https://repackager.wixmp.com/720,1080,480/path/video.mp4.urlset/...
// Like ani-cli: extracts quality list from URL path, generates separate URL per quality
func (e *WixmpExtractor) extractWixmpVariants(ctx context.Context, url, referer string) ([]*source.Stream, error) {
	// Extract quality list from URL path (e.g., "720,1080,480")
	qualities := e.extractWixmpQualities(url)
	if len(qualities) == 0 {
		// Fallback: treat as single quality
		return e.extractSingleStream(ctx, url, referer)
	}

	// Generate a stream for each quality variant
	var streams []*source.Stream
	for _, q := range qualities {
		// Replace quality segment in URL to get specific quality URL
		qualityURL := e.buildWixmpQualityURL(url, q)
		if qualityURL == "" {
			continue
		}

		streams = append(streams, &source.Stream{
			Provider: "wixmp",
			Quality:  fmt.Sprintf("%dp", q),
			URL:      qualityURL,
			Referer:  referer,
		})
	}

	// Sort by quality descending (best first)
	sort.Slice(streams, func(i, j int) bool {
		qi, _ := strconv.Atoi(strings.TrimSuffix(streams[i].Quality, "p"))
		qj, _ := strconv.Atoi(strings.TrimSuffix(streams[j].Quality, "p"))
		return qi > qj
	})

	return streams, nil
}

// extractWixmpQualities extracts quality numbers from repackager.wixmp.com URL
// URL format: .../720,1080,480/... -> [720, 1080, 480]
func (e *WixmpExtractor) extractWixmpQualities(url string) []int {
	// Match pattern: /digits,digits,digits/ in the URL path
	re := regexp.MustCompile(`/(\d+(?:,\d+)*)/`)
	matches := re.FindStringSubmatch(url)
	if len(matches) < 2 {
		return nil
	}

	parts := strings.Split(matches[1], ",")
	var qualities []int
	for _, p := range parts {
		if q, err := strconv.Atoi(p); err == nil && q > 0 {
			qualities = append(qualities, q)
		}
	}

	return qualities
}

// buildWixmpQualityURL constructs a URL for a specific quality
// Replaces the quality segment (e.g., "720,1080,480") with just the target quality
func (e *WixmpExtractor) buildWixmpQualityURL(url string, quality int) string {
	re := regexp.MustCompile(`/(\d+(?:,\d+)*)/`)
	return re.ReplaceAllStringFunc(url, func(match string) string {
		return fmt.Sprintf("/%d/", quality)
	})
}

// extractSingleStream fetches the URL and tries to extract a single stream
func (e *WixmpExtractor) extractSingleStream(ctx context.Context, url, referer string) ([]*source.Stream, error) {
	headers := map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		"Referer":    referer,
		"Origin":     "https://wixmp.com",
	}

	html, err := curl.Get(ctx, url, headers)
	if err != nil {
		return nil, fmt.Errorf("curl failed: %w", err)
	}

	m3u8URL := e.extractM3U8URL(html)
	if m3u8URL != "" {
		return e.extractFromM3U8(ctx, m3u8URL, referer)
	}

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

	headers := map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		"Referer":    referer,
	}

	html, err := curl.Get(ctx, url, headers)
	if err != nil {
		return nil, err
	}

	return extractSubtitlesFromHTML(html), nil
}
