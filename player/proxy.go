package player

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// StartProxy starts a local HTTP proxy that fetches from remoteURL and serves
// the content over localhost. This is used on Android where players launched
// via am start from PRoot can't directly fetch remote URLs.
// Returns the local URL to pass to the player, a stop function, and any error.
func StartProxy(remoteURL, referrer string) (localURL string, stop func(), err error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, fmt.Errorf("proxy listen failed: %w", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port
	localURL = fmt.Sprintf("http://127.0.0.1:%d/video", port)

	// Keep-alive HTTP client with connection pooling
	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 5,
			IdleConnTimeout:     5 * time.Minute,
		},
		Timeout: 30 * time.Minute,
	}

	// Playlist cache: brief TTL to reduce redundant fetches
	type plEntry struct {
		data      []byte
		expiresAt time.Time
	}
	var (
		plCacheMu sync.Mutex
		plCache   = map[string]plEntry{}
	)

	isPlaylist := func(ct, u string) bool {
		lower := strings.ToLower(u)
		return strings.Contains(ct, "mpegurl") ||
			strings.HasSuffix(lower, ".m3u8") ||
			strings.HasSuffix(lower, ".m3u")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/video", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Minute)
		defer cancel()

		// ?u= param lets m3u8 segment/playlist URLs route through us
		targetURL := remoteURL
		if u := r.URL.Query().Get("u"); u != "" {
			targetURL = u
		}

		// Check playlist cache (2s TTL)
		plCacheMu.Lock()
		if e, ok := plCache[targetURL]; ok && time.Now().Before(e.expiresAt) {
			plCacheMu.Unlock()
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(e.data)))
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Write(e.data)
			return
		}
		delete(plCache, targetURL) // expired
		plCacheMu.Unlock()

		req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
		if referrer != "" {
			req.Header.Set("Referer", referrer)
		}

		// Forward range header for seeking (segments only)
		if rangeH := r.Header.Get("Range"); rangeH != "" {
			req.Header.Set("Range", rangeH)
		}

		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, "fetch failed", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		ct := resp.Header.Get("Content-Type")

		// Fix generic Content-Type so mpv knows it's video
		if ct == "" || ct == "application/octet-stream" || ct == "binary/octet-stream" {
			if sniffed := sniffVideoType(targetURL); sniffed != "" {
				ct = sniffed
			}
		}

		// Playlist: read fully, rewrite URLs, cache 2s — must handle
		// before WriteHeader because rewriting changes Content-Length.
		if isPlaylist(ct, targetURL) && resp.StatusCode == 200 {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return
			}
			var targetBase *url.URL
			if parsed, err := url.Parse(targetURL); err == nil {
				targetBase = parsed
			}
			rewritten := rewritePlaylist(string(body), localURL, targetBase)
			data := []byte(rewritten)

			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.WriteHeader(resp.StatusCode)

			plCacheMu.Lock()
			plCache[targetURL] = plEntry{data: data, expiresAt: time.Now().Add(2 * time.Second)}
			plCacheMu.Unlock()

			w.Write(data)
			return
		}

		// Segments: forward headers and stream
		for _, key := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges"} {
			if key == "Content-Type" {
				w.Header().Set(key, ct)
				continue
			}
			if v := resp.Header.Get(key); v != "" {
				w.Header().Set(key, v)
			}
		}
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(resp.StatusCode)

		bw := bufio.NewWriterSize(w, 256*1024)
		io.Copy(bw, resp.Body)
		bw.Flush()
	})

	srv := &http.Server{Handler: mux}

	go func() {
		srv.Serve(listener)
	}()

	stop = func() {
		srv.Shutdown(context.Background())
	}

	return localURL, stop, nil
}

// rewritePlaylist rewrites relative/absolute URLs in m3u8 playlists to route through the proxy.
// It handles both bare URI lines and URI= attributes in HLS tags (e.g. #EXT-X-MAP:URI="...").
func rewritePlaylist(content, proxyURL string, baseURL *url.URL) string {
	proxyPrefix := proxyURL + "?u="

	resolveRaw := func(raw string) string {
		ref, err := url.Parse(raw)
		if err != nil {
			return raw
		}
		var resolved string
		if ref.IsAbs() {
			resolved = ref.String()
		} else if baseURL != nil {
			resolved = baseURL.ResolveReference(ref).String()
		} else {
			return raw
		}
		return proxyPrefix + url.QueryEscape(resolved)
	}

	var buf strings.Builder
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			buf.WriteString(line)
			buf.WriteByte('\n')
			continue
		}

		// HLS tags with URI= attributes (#EXT-X-MAP, #EXT-X-MEDIA, #EXT-X-I-FRAME-STREAM-INF, etc.)
		if strings.HasPrefix(trimmed, "#") && strings.Contains(trimmed, "URI=") {
			buf.WriteString(rewriteTagURI(trimmed, resolveRaw))
			buf.WriteByte('\n')
			continue
		}

		// Comment/metadata tags — pass through
		if strings.HasPrefix(trimmed, "#") {
			buf.WriteString(line)
			buf.WriteByte('\n')
			continue
		}

		// Bare URI line (segment or playlist reference)
		buf.WriteString(resolveRaw(trimmed))
		buf.WriteByte('\n')
	}
	return buf.String()
}

// rewriteTagURI rewrites URI="value" attributes inside HLS tags.
func rewriteTagURI(tag string, resolve func(string) string) string {
	// Find URI="..." (quoted)
	if idx := strings.Index(tag, `URI="`); idx >= 0 {
		start := idx + len(`URI="`)
		end := strings.Index(tag[start:], `"`)
		if end < 0 {
			return tag
		}
		end += start
		uri := tag[start:end]
		return tag[:start] + resolve(uri) + tag[end:]
	}
	// Find URI=value (unquoted, rare but valid)
	if idx := strings.Index(tag, "URI="); idx >= 0 {
		start := idx + len("URI=")
		end := strings.IndexAny(tag[start:], ", \t")
		if end < 0 {
			end = len(tag)
		} else {
			end += start
		}
		uri := tag[start:end]
		return tag[:start] + resolve(uri) + tag[end:]
	}
	return tag
}

// sniffVideoType returns a video Content-Type based on the URL extension.
func sniffVideoType(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "video/mp4"
	}
	path := strings.ToLower(u.Path)
	switch {
	case strings.HasSuffix(path, ".mp4"):
		return "video/mp4"
	case strings.HasSuffix(path, ".webm"):
		return "video/webm"
	case strings.HasSuffix(path, ".mkv"):
		return "video/x-matroska"
	case strings.HasSuffix(path, ".avi"):
		return "video/x-msvideo"
	case strings.HasSuffix(path, ".m3u8"):
		return "application/vnd.apple.mpegurl"
	case strings.HasSuffix(path, ".ts"):
		return "video/mp2t"
	case strings.HasSuffix(path, ".fmp4"):
		return "video/mp4"
	default:
		return "video/mp4"
	}
}
