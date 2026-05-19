# Anilix

A terminal-based anime streaming application built in Go. Search, browse, and stream anime directly from your terminal with an interactive TUI.

## Features

- **Search**: Find anime by title with metadata (cover art, year, genres, status)
- **Browse**: Navigate seasons and episodes with an interactive interface
- **Stream**: Extract and play video streams in your preferred media player
- **TUI**: Full terminal user interface powered by Bubbletea
- **Caching**: Persistent ID mapping cache for fast lookups
- **Configuration**: TOML-based config at `~/.anilix/anilix.toml`

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                      User Interface Layer                        │
│                  (TUI / CLI / Interactive)                       │
└──────────────────────────────┬──────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Source Interface (Abstraction)                │
│                                                                 │
│   Search(query) → []*Anime                                      │
│   SeasonsOf(anime) → []Season                                   │
│   EpisodesOf(anime, season) → []*Episode                        │
│   StreamsOf(episode) → []*Stream                                │
└──────────────────────────────┬──────────────────────────────────┘
                               │
          ┌────────────────────┴────────────────────┐
          ▼                                         ▼
┌──────────────────────┐                 ┌──────────────────────────┐
│   Metadata Provider  │                 │   Content Provider       │
│   (Search & Info)    │                 │   (Episodes & Streams)   │
│                      │                 │                          │
│   - REST API client  │                 │   - GraphQL client       │
│   - Returns: ID,     │                 │   - Returns: Episodes,   │
│     cover, year,     │                 │     stream sources       │
│     genres, status   │                 │                          │
└──────────────────────┘                 └───────────┬──────────────┘
                                                    │
                                                    ▼
                                         ┌──────────────────────────┐
                                         │   ID Resolver            │
                                         │   (Cross-Provider Link)  │
                                         │                          │
                                         │   - Maps metadata IDs    │
                                         │     to content IDs       │
                                         │   - SQLite-backed cache  │
                                         │   - Permanent TTL        │
                                         └───────────┬──────────────┘
                                                     │
                                                     ▼
                                         ┌──────────────────────────┐
                                         │   Stream Extractors      │
                                         │                          │
                                         │   - Embed page parsing   │
                                         │   - URL decryption       │
                                         │   - m3u8/mp4 resolution  │
                                         │   - Priority-based       │
                                         │     parallel extraction  │
                                         └───────────┬──────────────┘
                                                     │
                                                     ▼
                                         ┌──────────────────────────┐
                                         │   Player Launcher        │
                                         │                          │
                                         │   - mpv / VLC / iina     │
                                         │   - Referer injection    │
                                         │   - Android support      │
                                         └──────────────────────────┘
```

## Data Flow

```
User searches anime title
        │
        ▼
Metadata Provider → []*Anime (ID, cover, year, genres)
        │
        ▼
User selects anime
        │
        ▼
ID Resolver:
  1. Check SQLite cache (metadata ID → content ID)
  2. If not cached: query content provider
  3. Match and cache the mapping
        │
        ▼
Content Provider → []*Episode
        │
        ▼
User selects episode
        │
        ▼
Content Provider → []*Stream (provider URLs)
        │
        ▼
Stream Extractors:
  1. Resolve extractor for each URL
  2. Extract actual m3u8/mp4 stream URLs
  3. Return playable streams with referers
        │
        ▼
Player.Launch(stream.URL, stream.Referer)
```

## Technologies & Libraries

### Core

| Technology | Purpose |
|------------|---------|
| **Go 1.25** | Language and runtime |
| **SQLite** | Persistent ID mapping cache |

### TUI Framework

| Library | Purpose |
|---------|---------|
| [Bubbletea](https://github.com/charmbracelet/bubbletea) | Terminal UI framework (Elm architecture) |
| [Bubbles](https://github.com/charmbracelet/bubbles) | TUI components (text input, viewport, spinner) |
| [Lipgloss](https://github.com/charmbracelet/lipgloss) | Terminal styling and layout |

### CLI & Configuration

| Library | Purpose |
|---------|---------|
| [Cobra](https://github.com/spf13/cobra) | CLI command framework |
| [Viper](https://github.com/spf13/viper) | Configuration management (TOML) |

### Data & Networking

| Library | Purpose |
|---------|---------|
| `go-sqlite3` | SQLite driver for Go |
| `encoding/json` | JSON serialization |
| `crypto/aes` | AES-256-CTR stream decryption |
| `exec.Command("curl")` | HTTP requests via curl binary |

### Key Patterns

- **Provider/Extractor registration**: `init()` + `Register()` pattern for pluggable providers and extractors
- **Extractor priority**: Configurable extraction order with parallel execution and timeout
- **HTTP via curl**: All HTTP done through shell-out to `curl` binary (PRoot compatibility)
- **SQLite cache**: Permanent TTL for ID mappings (anime IDs don't change)
- **TUI state machine**: Bubbletea's Elm architecture with distinct search and episode states

## Project Structure

```
anilix/
├── cmd/             # Cobra CLI commands (root, tui, sources, version)
├── config/          # Viper-based configuration
├── curl/            # HTTP client wrapper (curl binary)
├── extractor/       # Stream extractors with priority ordering
├── player/          # Media player detection and launcher
├── provider/
│   ├── allanime/    # Content provider (episodes, streams)
│   └── jikan/       # Metadata provider (search, info) + ID resolver
├── source/          # Data models and Source interface
└── tui/             # Bubbletea TUI (search, episode selection)
```

## Build & Run

```bash
# Build (PRoot-compatible tmpfs workaround)
mkdir -p /tmp/anilix-build && cp -r . /tmp/anilix-build/ && cd /tmp/anilix-build && go build -o /storage/self/primary/github/Anilix/anilix .

# Run
cp ./anilix /tmp/anilix && chmod +x /tmp/anilix && /tmp/anilix tui
```

## Testing

```bash
# Run all tests
go test ./...

# Run specific package tests
go test ./source -v
go test ./provider/jikan -v
go test ./provider/allanime -v
go test ./extractor -v
go test ./player -v

# Run integration tests (require network)
go test ./provider/allanime -v -run TestIntegration -short=false
go test ./provider/jikan -v -run TestIntegration -short=false
go test ./extractor -v -run TestIntegration -short=false
```

## Configuration

Config file: `~/.anilix/anilix.toml`

```toml
[player]
default = "mpv"  # mpv, vlc, iina

[extractor]
timeout = 5  # seconds per extractor
```

## License

See [LICENSE](LICENSE) for details.
