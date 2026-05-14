package extractor

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

// M3U8Variant represents a quality variant in an m3u8 playlist
type M3U8Variant struct {
	URL       string
	Quality   string
	Bandwidth int
}

// ParseMasterPlaylist parses a master m3u8 and returns quality variants
func ParseMasterPlaylist(url string, client *http.Client) ([]M3U8Variant, error) {
	if client == nil {
		client = &http.Client{}
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch m3u8: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("non-200 status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return ParseM3U8(string(body), url), nil
}

// ParseM3U8 parses m3u8 content and extracts variants
func ParseM3U8(content, baseURL string) []M3U8Variant {
	variants := make([]M3U8Variant, 0)

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "#EXT-X-STREAM-INF:") {
			if i+1 < len(lines) {
				uri := strings.TrimSpace(lines[i+1])
				variant := parseStreamInf(line, uri, baseURL)
				if variant.URL != "" {
					variants = append(variants, variant)
				}
			}
		}
	}

	return variants
}

func parseStreamInf(line, uri, baseURL string) M3U8Variant {
	variant := M3U8Variant{}

	resRe := regexp.MustCompile(`RESOLUTION=(\d+)x(\d+)`)
	matches := resRe.FindStringSubmatch(line)
	if len(matches) == 3 {
		_, height := matches[1], matches[2]
		variant.Quality = fmt.Sprintf("%sp", height)
	}

	bwRe := regexp.MustCompile(`BANDWIDTH=(\d+)`)
	bwMatches := bwRe.FindStringSubmatch(line)
	if len(bwMatches) == 2 {
		fmt.Sscanf(bwMatches[1], "%d", &variant.Bandwidth)
	}

	variant.URL = resolveURL(baseURL, uri)

	return variant
}

func resolveURL(baseURL, relative string) string {
	if strings.HasPrefix(relative, "http") {
		return relative
	}

	parts := strings.Split(baseURL, "/")
	if len(parts) < 3 {
		return relative
	}

	basePath := strings.Join(parts[:len(parts)-1], "/")

	if strings.HasPrefix(relative, "/") {
		return parts[0] + "//" + parts[2] + relative
	}

	return basePath + "/" + relative
}

func GetBestVariant(variants []M3U8Variant) *M3U8Variant {
	if len(variants) == 0 {
		return nil
	}

	best := &variants[0]
	for _, v := range variants[1:] {
		if v.Bandwidth > best.Bandwidth {
			best = &v
		}
	}

	return best
}

func SelectVariant(variants []M3U8Variant, preference string) *M3U8Variant {
	if len(variants) == 0 {
		return nil
	}

	preference = strings.ToLower(preference)

	for _, v := range variants {
		if strings.ToLower(v.Quality) == preference {
			return &v
		}
	}

	if preference == "auto" || preference == "" {
		return GetBestVariant(variants)
	}

	best := GetBestVariant(variants)
	return best
}

func ExtractSubtitles(ctx context.Context, url string, client *http.Client) ([]string, error) {
	if client == nil {
		client = &http.Client{}
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("non-200 status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return extractSubtitlesFromHTML(string(body)), nil
}

var subtitlePatterns = []*regexp.Regexp{
	regexp.MustCompile(`"subtitles"\s*:\s*\[\s*\{[^}]*"src"\s*:\s*"([^"]+)"`),
	regexp.MustCompile(`"tracks"\s*:\s*\[\s*\{[^}]*"src"\s*:\s*"([^"]+)"`),
	regexp.MustCompile(`"captions"\s*:\s*\[\s*\{[^}]*"src"\s*:\s*"([^"]+)"`),
	regexp.MustCompile(`"src"\s*:\s*"([^"]+\.vtt[^"]*)"`),
	regexp.MustCompile(`"src"\s*:\s*"([^"]+\.srt[^"]*)"`),
	regexp.MustCompile(`<track[^>]*src\s*=\s*["']([^"']+\.vtt[^"']*)["']`),
}

func extractSubtitlesFromHTML(html string) []string {
	seen := make(map[string]bool)
	var subtitles []string

	for _, re := range subtitlePatterns {
		matches := re.FindAllStringSubmatch(html, -1)
		for _, match := range matches {
			if len(match) > 1 && match[1] != "" && !seen[match[1]] {
				seen[match[1]] = true
				subtitles = append(subtitles, match[1])
			}
		}
	}

	return subtitles
}
