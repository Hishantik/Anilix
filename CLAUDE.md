# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Anilix is a Go-based anime streaming/downloading CLI inspired by ani-cli and mangal architecture. Currently in early development - the repository is initialized but no code has been written yet.

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

## Technology Stack

| Component | Technology |
|-----------|------------|
| Language | Go 1.18+ |
| CLI | spf13/cobra + spf13/viper |
| TUI | charmbracelet/bubbletea + bubbles + lipgloss |
| HTTP | gocolly/colly + curl |
| Parsing | PuerkitoBio/goquery |
| Anilist API | REST/GraphQL client |
| Player | External (mpv, vlc, iina) |

## Architecture

### Source Interface

```go
type Source interface {
    Name() string
    Search(query string) ([]*Anime, error)
    EpisodesOf(anime *Anime) ([]*Episode, error)
    StreamsOf(episode *Episode) ([]*Stream, error)
    ID() string
}
```

### Data Models

- `Anime` - Title, URL, Cover, Year, Genres, Status, **MALID** (MyAnimeList ID for metadata linkage)
- `Episode` - Number, Title, Season, URL, associated Anime
- `Stream` - Quality, URL, Provider, Subtitle URLs

### Jikan Integration (Search & Metadata)

Jikan API (MyAnimeList) provides anime search and metadata. Streaming links come from separate sources.

**Architecture:**
```
JikanProvider (search) → Anime (with MAL ID) → AnimeLinker → StreamingSource (episodes/streams)
```

- **JikanProvider** — handles search/metadata via Jikan API, returns Anime with MAL ID
- **AnimeLinker** — maps Anime (with MAL ID) to appropriate streaming source's episodes
- **StreamingSource** — existing sources (allmanga, etc.) for episodes and streams

**Data Flow:**
1. `JikanProvider.Search(query)` → Jikan API → `[]*Anime` (with MAL ID, cover, year, genres)
2. User selects anime → AnimeLinker maps to streaming source via MAL ID
3. `StreamingSource.EpisodesOf(anime)` / `StreamsOf(episode)` → actual streaming links

### Provider Pattern

- `provider.Provider` wraps source implementations with metadata (ID, Name, UsesHeadless, IsCustom)
- Built-in providers use Go scrapers
- Custom providers via Lua scripts (future)

### UI Modes

1. **TUI** - Interactive terminal UI using charmbracelet/bubbletea (primary)
2. **Inline** - Script-friendly CLI mode
3. **Mini** - Simple command prompts (ani-cli style)

## Key Dependencies

```go
github.com/charmbracelet/bubbletea v0.23.1
github.com/charmbracelet/bubbles v0.14.0
github.com/charmbracelet/lipgloss v0.6.0
github.com/spf13/cobra v1.6.1
github.com/spf13/viper v1.14.0
github.com/gocolly/colly/v2 v2.1.0
github.com/PuerkitoBio/goquery v1.8.0
github.com/yuin/gopher-lua v1.0.0  // Optional for Lua scrapers
```

## Planned Directory Structure

```
anilix/
├── cmd/               # CLI commands (root, config, inline, tui, sources, history)
├── provider/          # Source providers
│   ├── provider.go    # Provider struct
│   ├── init.go        # Registration
│   ├── allmanga/      # allmanga.to source
│   ├── jikan/         # Jikan API (search/metadata only)
│   │   ├── client.go  # Jikan API client
│   │   ├── types.go   # API response types
│   │   └── linker.go  # AnimeLinker for MAL ID mapping
│   ├── generic/       # Generic scraper
│   └── custom/        # Lua scrapers (optional v2)
├── source/            # Data models (source.go, anime.go, episode.go, stream.go)
├── tui/               # Bubble tea TUI (tui.go, bubble.go, handlers.go, keymap.go, view.go)
├── integration/       # Anilist API integration
├── downloader/        # Parallel downloads
├── history/           # Watch history
├── player/            # Media player launcher
└── config/            # Configuration
```