# Jikan API Integration Design

## Overview

Integrate Jikan API (MyAnimeList) for anime search and metadata. Streaming links come from separate pluggable sources.

## Architecture

```
JikanProvider (search) → Anime (with MAL ID) → AnimeLinker → StreamingSource (episodes/streams)
```

### Components

1. **JikanProvider** — implements source.Source, handles search/metadata via Jikan API
2. **AnimeLinker** — maps Anime (with MAL ID) to appropriate streaming source
3. **JikanClient** — HTTP client for Jikan API (rate limiting, retry)

### Data Flow

1. `JikanProvider.Search(query)` → Jikan `/anime` endpoint → `[]*Anime` with MAL ID, cover, year, genres, status
2. User selects anime from results
3. `AnimeLinker.GetEpisodes(anime, streamingSource)` → calls streaming source with MAL ID
4. `StreamingSource.StreamsOf(episode)` → actual streaming links

## Jikan API Integration

### Base URL
`https://api.jikan.moe/v4`

### Endpoints

- **Search**: `GET /anime?q={query}&limit=20`
- **Anime Details**: `GET /anime/{mal_id}` (for richer metadata if needed)

### Response Mapping

| Jikan Field | Anime Field |
|-------------|-------------|
| `mal_id` | `MALID` |
| `title_english` or `title` | `Name` |
| `images.jpg.large_image_url` | `Cover` |
| `year` | `Year` |
| `genres[].name` | `Genres` |
| `status` | `Status` |
| `url` | `URL` |

## Rate Limiting

- Jikan allows 3 requests per second
- Implement token bucket or request queue
- Handle 429 responses with exponential backoff

## Error Handling

- API errors: wrap with context ("Jikan search failed: {error}")
- No streaming source found: return clear error ("No streaming source available for this anime")
- Delegate errors: propagate from streaming source

## Configuration

- Jikan API base URL (configurable, default: https://api.jikan.moe/v4)
- AnimeLinker receives streaming source at runtime (pluggable)

## Testing

- Mock Jikan API responses for search tests
- Mock streaming source for linker tests
- Test rate limiting behavior

## Out of Scope (v1)

- Caching Jikan responses
- User preferences for preferred streaming source per anime
- Automatic streaming source discovery