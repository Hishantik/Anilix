# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Anilix is a Go-based anime streaming CLI inspired by ani-cli. Features:
- **TUI Mode**: Interactive search with split-view (left: anime list, right: metadata)
- **Inline Mode**: Script-friendly CLI mode
- Uses AllAnime for streaming and Jikan (MyAnimeList) for metadata

## Build Commands

```shell
go build      # Build the anilix binary
go install    # Build and install anilix to PATH
go test ./... # Run all tests
```

Run a single test:
```shell
go test ./... -run TestName
```

Run TUI:
```shell
./anilix tui
```

## Technology Stack

| Component | Technology |
|-----------|------------|
| Language | Go 1.26+ |
| CLI | spf13/cobra + spf13/viper |
| TUI | charmbracelet/bubbletea + bubbles + lipgloss |
| HTTP | net/http + curl (via exec) |
| Player | External (mpv, vlc, iina) |

## Directory Structure

```
anilix/
├── cmd/                   # CLI commands
│   ├── root.go           # Main entry, has "tui" and default (inline) commands
│   └── root_test.go
├── provider/              # Source providers
│   ├── init.go           # Provider registration
│   ├── jikan/             # Jikan API (metadata/search)
│   │   ├── client.go      # REST client with rate limiting & caching
│   │   └── types.go      # API response types
│   └── allanime/         # AllAnime GraphQL API (streaming)
│       ├── client.go     # GraphQL client with curl fallback
│       ├── decoder.go    # Hex decode & AES-256-CTR decryption
│       ├── types.go      # GraphQL response types
│       └── allanime.go   # AllanimeProvider implementation
├── extractor/            # Stream extractors (registered in init.go)
│   ├── init.go           # Registers all extractors
│   ├── extractor.go     # Extractor interface
│   ├── aes.go            # AES decryption utilities
│   ├── m3u8.go           # m3u8 playlist parsing
│   ├── wixmp.go          # Wixmp extractor
│   ├── hianime.go        # Hianime extractor
│   ├── filemoon.go       # Filemoon extractor
│   ├── anihdplay.go      # Anihdplay extractor
│   ├── vidstreaming.go   # Vidstreaming extractor
│   └── mp4upload.go      # Mp4upload extractor
├── source/               # Data models
│   ├── anime.go
│   ├── episode.go
│   └── stream.go
├── player/               # Media player launcher
│   ├── play.go           # Player interface (mpv, vlc, iina)
│   ├── detect.go         # Auto-detect available players
│   └── syncplay.go
├── tui/                  # Bubble Tea TUI
│   ├── search.go         # Main search model with split-view
│   └── models.go         # State models (SearchState, EpisodeState)
├── inline/               # Inline CLI mode
│   └── inline.go         # Script-friendly interface
├── config/               # Configuration (viper)
└── main.go               # Entry point
```

## How It Works

### TUI Flow (like ani-cli)

```
User types query
        ↓
AllAnime Search (via GraphQL)
        ↓
Show results in left panel
        ↓
On selection → fetch Jikan metadata → show in right panel
        ↓
User selects anime → fetch episode list
        ↓
User selects episode → get all provider URLs
        ↓
Try each provider in priority: wixmp → hianime → filemoon → others
        ↓
First successful stream → launch mpv
```

### AllanimeProvider.StreamsOf()

The core streaming logic in `provider/allanime/allanime.go`:

```go
func (a *AllanimeProvider) StreamsOf(episode) ([]*Stream, error) {
    sources := a.client.GetEpisodeSources()  // Get raw URLs from API

    for _, src := range sources {
        // Decode hex-encoded provider names if needed
        if isHexEncoded(src.SourceName) {
            src.SourceName = decodeHexProviderID(src.SourceName)
        }

        // Find matching extractor
        ext := extractor.Resolve(src.SourceUrl)
        if ext != nil {
            // Extract actual stream URLs (m3u8/mp4)
            extracted := ext.Extract(ctx, src.SourceUrl, Referer)
            allStreams = append(allStreams, extracted...)
        }

        // Fallback: add raw URL if no extractor found
        if ext == nil && src.SourceUrl != "" {
            allStreams = append(allStreams, &Stream{URL: src.SourceUrl, ...})
        }
    }
    return allStreams
}
```

### TUI Priority System

The TUI sorts streams by priority (like ani-cli):

```go
priority := map[string]int{
    "wixmp":        1,  // Best quality, try first
    "hianime":      2,  // Good fallback
    "filemoon":     3,
    "vidstreaming": 4,
    "mp4upload":    5,
    "streamsb":     6,
    "default":      7,  // Try last
}
```

Then tries each until one works:

```go
for _, s := range sorted {
    if p.Launch(url, opts) == nil {  // Success!
        return s
    }
    // Failed, try next...
}
```

## Provider Extraction

AllAnime returns provider **references**, not direct streams. Extractors fetch these pages and extract actual video URLs:

| Extractor | Handles | Returns |
|-----------|---------|---------|
| wixmp | wixmp.com | m3u8 |
| hianime | hianime.com | m3u8 |
| filemoon | filemoon.sx | m3u8 |
| anihdplay | anihdplay.com | m3u8/mp4 |
| vidstreaming | vidstreaming.io | m3u8 |
| mp4upload | mp4upload.com | mp4 |

## Data Models

```go
type Anime struct {
    MALID       int    // MyAnimeList ID (for Jikan metadata)
    AllAnimeID  string // AllAnime show ID (for streaming)
    Name        string
    Cover       string
    Year        int
    Type        string
    Status      string
    Episodes    int
    Score       float64
    Rank        int
    Genres      []string
}

type Episode struct {
    Number  float64
    Title   string
    URL     string
    Anime   *Anime
}

type Stream struct {
    Provider  string
    Quality   string
    URL       string
    Referer   string
    Subtitles []string
}
```

## API Endpoints

### Jikan API (Metadata)
- `GET https://api.jikan.moe/v4/anime?q={query}` - Search
- `GET https://api.jikan.moe/v4/anime/{mal_id}` - Details
- `GET https://api.jikan.moe/v4/anime/{mal_id}/episodes` - Episode list

### AllAnime API (Streaming)
- `POST https://api.allanime.day/api` - GraphQL endpoint
- Search: `shows(search: {...})`
- Episodes: `show(_id: {...}).availableEpisodesDetail`
- Sources: `episode(showId: ..., episodeString: ...).sourceUrls`

## Caching Strategy

| Data | TTL |
|------|-----|
| Jikan anime details | 1 hour |
| Episode list | 1 day |
| Stream extraction | 30 minutes |

## Important Notes

- AllAnime API returns hex-encoded provider IDs - use `decoder.go` functions
- Rate limit Jikan to 3 requests/second
- Use curl fallback in `client.go` if Go HTTP gets blocked by Cloudflare
- TUI uses textinput.Blur() when in episodes mode so Enter key isn't captured
- Providers change frequently - add new extractors as needed