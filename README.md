# Anilix

A Go-based anime streaming/downloading CLI inspired by [ani-cli](https://github.com/pystardust/ani-cli) and [mangal](https://github.com/oxytocl/mangal).

## Current Status

**Working**: Anime search, episode listing, and stream extraction are functional via CLI.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        User Interface                           │
│                    (Inline CLI / TUI / Mini)                    │
└──────────────────────────────┬──────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│                     Source Interface                            │
│  Search(query) → []*Anime                                        │
│  EpisodesOf(anime, season) → []*Episode                         │
│  StreamsOf(episode) → []*Stream                                │
└──────────────────────────────┬──────────────────────────────────┘
                               │
          ┌────────────────────┴────────────────────┐
          ▼                                         ▼
┌──────────────────────┐                 ┌──────────────────────────┐
│   Jikan Provider     │                 │   AllAnime Provider      │
│   (Metadata/Search)  │                 │   (Episodes/Streams)     │
│                      │                 │                          │
│ - MyAnimeList API    │                 │ - GraphQL API           │
│ - Returns: MAL ID,   │                 │ - Returns: AllAnime ID, │
│   Cover, Year,       │                 │   Episodes, Providers   │
│   Genres, Status     │                 │                          │
└──────────────────────┘                 └───────────┬──────────────┘
                                                    │
                                                    ▼
                                         ┌──────────────────────────┐
                                         │   AnimeLinker            │
                                         │   (Jikan → AllAnime)     │
                                         │                          │
                                         │ - Resolves MAL ID →      │
                                         │   AllAnime ID            │
                                         │ - Uses SQLite cache      │
                                         │   for ID mappings        │
                                         └───────────┬──────────────┘
                                                     │
                                                     ▼
                                         ┌──────────────────────────┐
                                         │   Extractors            │
                                         │                          │
                                         │ Hianime / Filemoon /     │
                                         │ Wixmp / YouTube          │
                                         │                          │
                                         │ - Fetch embed pages     │
                                         │ - Extract m3u8/mp4 URLs │
                                         │ - AES decryption        │
                                         │ - m3u8 parsing          │
                                         └──────────────────────────┘
                                                     │
                                                     ▼
                                         ┌──────────────────────────┐
                                         │   Player (mpv/VLC/iina) │
                                         └──────────────────────────┘
```

## Data Flow

```
User searches "Naruto"
        │
        ▼
Jikan API → []*Anime (MAL ID: 20, Cover, Year, Genres)
        │
        ▼
User selects anime
        │
        ▼
AnimeLinker.ResolveAllAnimeID():
  1. Check SQLite cache (MAL ID → AllAnime ID)
  2. If not cached: Search AllAnime with same query
  3. Match by MAL ID → Get AllAnime ID
  4. Cache the result
        │
        ▼
AllAnime EpisodesOf() → []*Episode (293 episodes)
        │
        ▼
User selects episode
        │
        ▼
AllAnime StreamsOf() → []*Stream:
  1. Get provider URLs from GraphQL API
  2. For each URL, resolve matching extractor
  3. Extract actual m3u8/mp4 URLs
  4. Return playable streams
        │
        ▼
Player.Launch(stream.URL, stream.Referer)
```

## Key Components

### Providers

| Provider | Purpose | API |
|----------|---------|-----|
| Jikan | Search & Metadata | REST (api.jikan.moe/v4) |
| AllAnime | Episodes & Streams | GraphQL (api.allanime.day) |

### Extractors

| Extractor | Stream Type |
|-----------|-------------|
| Hianime | m3u8 (encrypted) |
| Filemoon | m3u8 (AES-256-CTR) |
| Wixmp | mp4/m3u8 |
| YouTube | mp4 |

### Cache

- **SQLite** at `~/.anilix/malid_cache.db`
- Table: `malid_allanime_map` (mal_id → Allanime_id)
- Permanent TTL (anime IDs don't change)

## Usage

```go
// Search via Jikan
jikanProv := jikan.NewJikanProvider()
animes, _ := jikanProv.Search("Naruto")

// Resolve AllAnime ID and get episodes
linker := jikan.NewAnimeLinker()
episodes, _ := linker.GetEpisodes(ctx, animes[0], AllanimeProv)

// Get streams
streams, _ := AllanimeProv.StreamsOf(episodes[0])

// Play
player := player.Mpv
player.Launch(streams[0].URL, player.Options{Referrer: streams[0].Referer})
```

## Project Structure

```
anilix/
├── cache/           # SQLite caching (MAL ID → AllAnime ID)
├── extractor/       # Stream extractors (Hianime, Filemoon, etc.)
│   ├── aes.go       # AES-256-CTR decryption
│   └── m3u8.go      # m3u8 playlist parsing
├── player/          # Media player launcher (mpv, VLC, iina)
├── provider/
│   ├── allanime/    # AllAnime GraphQL provider
│   └── jikan/       # Jikan API provider + AnimeLinker
└── source/          # Data models (Anime, Episode, Stream)
```

## Build & Test

```bash
go build      # Build anilix binary
go test ./... # Run all tests

# Run specific integration tests
go test ./provider/allanime -v -run TestIntegration -short=false
go test ./provider/jikan -v -run TestIntegration -short=false
go test ./extractor -v -run TestIntegration -short=false
```

## Test Results

```
✓ Jikan search returns anime with MAL ID
✓ AllAnime resolves ID via AnimeLinker (cached)
✓ AllAnime returns 293 episodes for Boruto
✓ AllAnime returns 6 provider sources
✓ Extractors resolve and return 9 playable streams
✓ Streams contain m3u8/mp4 URLs with referers
```

## Dependencies

- `github.com/mattn/go-sqlite3` - SQLite cache
- Standard library: net/http, encoding/json, crypto/aes, etc.

## Notes

- Jikan provides metadata only (search, cover, genres, MAL ID)
- AllAnime provides actual streaming sources
- AnimeLinker bridges the two via MAL ID matching
- Stream URLs require proper referer headers for playback