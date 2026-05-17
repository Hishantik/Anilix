# AGENTS.md - Anilix

Go anime streaming CLI. Entry: `main.go` -> `cmd.Execute()` -> TUI by default.

## Commands

```
go build      # Build the anilix binary
go install    # Build and install anilix to PATH
go test ./... # Run all tests
```

Run TUI:
```shell
./anilix
```

## Technology Stack

| Component | Technology |
|-----------|------------|
| Language | Go 1.26+ |
| CLI | spf13/cobra + spf13/viper |
| TUI | charmbracelet/bubbletea + bubbles + lipgloss |
| HTTP | curl (via exec) - primary method like ani-cli |
| Player | External (mpv, vlc, iina) |

## Directory Structure

```
anilix/
├── cmd/                   # CLI commands
│   ├── root.go           # Main entry (runs TUI by default)
│   └── version.go
├── provider/              # Source providers
│   ├── init.go           # Provider registration
│   ├── jikan/             # Jikan API (metadata/search)
│   │   ├── client.go     # REST client with curl
│   │   ├── linker.go     # MAL ID to AllAnime ID resolution
│   │   └── types.go      # API response types
│   └── allanime/         # AllAnime GraphQL API (streaming)
│       ├── client.go     # GraphQL client with curl
│       ├── decoder.go    # Hex decode & AES-256-CTR decryption
│       ├── types.go      # GraphQL response types
│       └── Allanime.go   # AllanimeProvider implementation
├── extractor/            # Stream extractors (registered in init.go)
├── source/               # Data models
├── player/               # Media player launcher
├── tui/                  # Bubble Tea TUI
├── config/               # Configuration (viper)
└── main.go               # Entry point
```

## How It Works (ani-cli approach)

```
User types query
        ↓
AllAnime Search (GraphQL via curl)
        ↓
Show results in left panel
        ↓
On selection → fetch Jikan metadata → show in right panel
        ↓
User selects anime → fetch episode list
        ↓
User selects episode → get provider URLs
        ↓
Try sources in priority order (wixmp → hianime → filemoon → ...)
        ↓
First successful stream → launch mpv
```

### Streaming Flow (like ani-cli)

The core streaming logic in `provider/allanime/allanime.go`:

1. Get episode sources from AllAnime API (via curl)
2. Decode hex-encoded provider names/URLs
3. Sort by provider priority (lower = higher priority)
4. For each source URL:
   - Try extractors until one succeeds
   - Return first valid streams found
5. Player tries streams in priority order until one plays

## Provider Priority

```go
priority := map[string]int{
    "wixmp":        1,  // Best quality, try first
    "hianime":      2,
    "filemoon":     3,
    "vidstreaming": 4,
    "mp4upload":    5,
    "streamsb":     6,
    "streamlare":   7,
    "default":      20, // Try last
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
| streamsb | streamsb.com, streamtape.com, ok.ru, vk.com | m3u8/mp4 |
| streamlare | streamlare.com | m3u8 |
| generic | Any URL (fallback) | Scans page for m3u8/mp4 |

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
}
```

## API Endpoints

### Jikan API (Metadata) - via curl
- `GET https://api.jikan.moe/v4/anime?q={query}` - Search
- `GET https://api.jikan.moe/v4/anime/{mal_id}` - Details

### AllAnime API (Streaming) - via curl
- `POST https://api.allanime.day/api` - GraphQL endpoint
- Search: `shows(search: {...})`
- Episodes: `show(_id: {...}).availableEpisodesDetail`
- Sources: `episode(showId: ..., episodeString: ...).sourceUrls`

## Important Notes

- **Always use curl** - not Go's http.Client. Like ani-cli, curl handles Cloudflare better.
- AllAnime API returns hex-encoded provider IDs - use `decoder.go` functions
- Rate limit Jikan to 3 requests/second
- TUI uses textinput.Blur() when in episodes mode so Enter key isn't captured
- Providers change frequently - add new extractors as needed
- sub/dub translation handled via AllAnime API parameter
