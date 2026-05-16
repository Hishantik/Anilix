# Jikan API Documentation

> Unofficial MyAnimeList API - Read-only REST API that scrapes MyAnimeList.net

**Base URL:** `https://api.jikan.moe/v4/`  
**Version:** 4.0.0  
**Auth:** None (GET requests only)  
**License:** MIT

---

## Rate Limiting

| Duration | Limit |
|----------|-------|
| Per Second | 3 requests |
| Per Minute | 60 requests |
| Daily | Unlimited |

> Note: It's still possible to get rate limited from MyAnimeList.net itself.

## Caching

- All requests are cached for **24 hours**
- Use `ETag` header for cache validation
- Response headers: `Expires`, `Last-Modified`, `X-Request-Fingerprint`

## Pagination

Most list endpoints support pagination:

```
GET /anime?page=1&limit=25
```

Common query parameters:
- `page` - Page number (default: 1)
- `limit` - Items per page (default: 25, max: 25)
- `sfw` - Safe-for-work filter (exclude NSFW results)

---

## Endpoints Overview

| Category | Endpoints |
|----------|-----------|
| [Anime](#anime) | search, details, characters, episodes, news, videos, relations, recommendations, reviews |
| [Manga](#manga) | search, details, characters, news, pictures, statistics, recommendations, reviews |
| [Characters](#characters) | details, anime, manga, voices, pictures |
| [People](#people) | details, anime, voices, manga, pictures |
| [Producers](#producers) | details, full, external |
| [Seasons](#seasons) | now, list, upcoming |
| [Top](#top) | anime, manga, people, characters, reviews |
| [Users](#users) | profile, statistics, favorites |
| [Watch](#watch) | episodes, promos |
| [Random](#random) | anime, manga, characters, people |
| [Other](#other) | genres, magazines, clubs, schedules, recommendations, reviews |

---

## Anime

### Search Anime

```
GET /anime
```

**Query Parameters:**
- `q` - Search query
- `type` - tv, movie, ova, special, ona, music
- `score` - Minimum score (0-10)
- `status` - airing, complete, upcoming
- `rating` - g, pg, pg13, r17, r, rx
- `genres` - Genre IDs (comma-separated)
- `order_by` - title, start_date, end_date, score, popularity, members, favorites
- `sort` - desc, asc
- `sfw` - true/false

**Example:**
```bash
curl "https://api.jikan.moe/v4/anime?q=naruto&sfw=true&limit=5"
```

**Response:**
```json
{
  "data": [
    {
      "mal_id": 20,
      "title": "Naruto",
      "type": "TV",
      "episodes": 220,
      "score": 7.97,
      "status": "Finished Airing"
    }
  ],
  "pagination": { "last_visible_page": 50, "has_next_page": true }
}
```

---

### Get Anime by ID

```
GET /anime/{id}
```

**Example:**
```bash
curl "https://api.jikan.moe/v4/anime/1"
```

**Response:**
```json
{
  "data": {
    "mal_id": 1,
    "title": "Cowboy Bebop",
    "title_english": "Cowboy Bebop",
    "type": "TV",
    "episodes": 26,
    "status": "Finished Airing",
    "rating": "R - 17+ (violence & profanity)",
    "score": 8.75,
    "synopsis": "Crime is timeless...",
    "genres": [{ "mal_id": 1, "name": "Action" }],
    "images": {
      "jpg": { "image_url": "https://...", "large_image_url": "https://..." }
    }
  }
}
```

---

### Get Full Anime Details

```
GET /anime/{id}/full
```

Returns complete anime with relations, external links, and streaming sources.

**Example:**
```bash
curl "https://api.jikan.moe/v4/anime/1/full"
```

---

### Get Anime Characters

```
GET /anime/{id}/characters
```

**Example:**
```bash
curl "https://api.jikan.moe/v4/anime/1/characters"
```

**Response:**
```json
{
  "data": [
    {
      "character": {
        "mal_id": 1,
        "name": "Spike Spiegel",
        "images": { "jpg": { "image_url": "https://..." } }
      },
      "role": "Main",
      "voice_actors": [{ "person": { "name": "Koichi Yamadera" } }]
    }
  ]
}
```

---

### Get Anime Episodes

```
GET /anime/{id}/episodes
```

**Query Parameters:**
- `page` - Page number

**Example:**
```bash
curl "https://api.jikan.moe/v4/anime/1/episodes"
```

---

### Get Single Episode

```
GET /anime/{id}/episodes/{episode}
```

**Example:**
```bash
curl "https://api.jikan.moe/v4/anime/1/episodes/1"
```

---

### Get Anime News

```
GET /anime/{id}/news
```

**Example:**
```bash
curl "https://api.jikan.moe/v4/anime/1/news"
```

---

### Get Anime Videos

```
GET /anime/{id}/videos
```

Returns promotional videos, music videos, and episode clips.

**Example:**
```bash
curl "https://api.jikan.moe/v4/anime/1/videos"
```

---

### Get Anime Pictures

```
GET /anime/{id}/pictures
```

**Example:**
```bash
curl "https://api.jikan.moe/v4/anime/1/pictures"
```

---

### Get Anime Statistics

```
GET /anime/{id}/statistics
```

**Example:**
```bash
curl "https://api.jikan.moe/v4/anime/1/statistics"
```

**Response:**
```json
{
  "data": {
    "watching": 50000,
    "completed": 200000,
    "on_hold": 10000,
    "dropped": 5000,
    "plan_to_watch": 30000,
    "total": 295000,
    "scores": { "10": 15000, "9": 30000, ... }
  }
}
```

---

### Get Anime Recommendations

```
GET /anime/{id}/recommendations
```

**Example:**
```bash
curl "https://api.jikan.moe/v4/anime/1/recommendations"
```

---

### Get Anime Reviews

```
GET /anime/{id}/reviews
```

**Query Parameters:**
- `page` - Page number
- `preliminary` - Include preliminary reviews

**Example:**
```bash
curl "https://api.jikan.moe/v4/anime/1/reviews"
```

---

### Get Anime Relations

```
GET /anime/{id}/relations
```

Returns related anime/manga (sequels, adaptations, etc.)

**Example:**
```bash
curl "https://api.jikan.moe/v4/anime/1/relations"
```

---

### Get Anime Themes

```
GET /anime/{id}/themes
```

Returns opening and ending themes.

**Example:**
```bash
curl "https://api.jikan.moe/v4/anime/1/themes"
```

---

### Get Anime External Links

```
GET /anime/{id}/external
```

**Example:**
```bash
curl "https://api.jikan.moe/v4/anime/1/external"
```

---

### Get Anime Streaming

```
GET /anime/{id}/streaming
```

Returns streaming availability (Crunchyroll, Netflix, etc.)

**Example:**
```bash
curl "https://api.jikan.moe/v4/anime/1/streaming"
```

---

## Manga

### Search Manga

```
GET /manga
```

**Query Parameters:**
- `q` - Search query
- `type` - manga, manhwa, manhua, lightnovel, novel
- `score` - Minimum score
- `status` - publishing, complete, upcoming
- `genres` - Genre IDs
- `order_by` - title, start_date, end_date, score, popularity, members, favorites
- `sort` - desc, asc

**Example:**
```bash
curl "https://api.jikan.moe/v4/manga?q=one%20piece&limit=5"
```

---

### Get Manga by ID

```
GET /manga/{id}
```

**Example:**
```bash
curl "https://api.jikan.moe/v4/manga/1"
```

---

### Get Full Manga Details

```
GET /manga/{id}/full
```

---

### Get Manga Characters

```
GET /manga/{id}/characters
```

---

### Get Manga News

```
GET /manga/{id}/news
```

---

### Get Manga Pictures

```
GET /manga/{id}/pictures
```

---

### Get Manga Statistics

```
GET /manga/{id}/statistics
```

---

### Get Manga More Info

```
GET /manga/{id}/moreinfo
```

---

### Get Manga Recommendations

```
GET /manga/{id}/recommendations
```

---

### Get Manga Reviews

```
GET /manga/{id}/reviews
```

---

## Characters

### Search Characters

```
GET /characters
```

**Query Parameters:**
- `q` - Search query
- `limit` - Number of results
- `page` - Page number

**Example:**
```bash
curl "https://api.jikan.moe/v4/characters?q=sakura&limit=5"
```

---

### Get Character by ID

```
GET /characters/{id}
```

**Example:**
```bash
curl "https://api.jikan.moe/v4/characters/6"
```

---

### Get Full Character Details

```
GET /characters/{id}/full
```

Includes biography, appearance, etc.

**Example:**
```bash
curl "https://api.jikan.moe/v4/characters/6/full"
```

---

### Get Character Anime

```
GET /characters/{id}/anime
```

Returns anime where the character appears.

**Example:**
```bash
curl "https://api.jikan.moe/v4/characters/6/anime"
```

---

### Get Character Manga

```
GET /characters/{id}/manga
```

---

### Get Character Voice Actors

```
GET /characters/{id}/voices
```

---

### Get Character Pictures

```
GET /characters/{id}/pictures
```

---

## People

### Search People

```
GET /people
```

**Example:**
```bash
curl "https://api.jikan.moe/v4/people?q=hayao&limit=5"
```

---

### Get Person by ID

```
GET /people/{id}
```

**Example:**
```bash
curl "https://api.jikan.moe/v4/people/1"
```

---

### Get Full Person Details

```
GET /people/{id}/full
```

---

### Get Person Anime

```
GET /people/{id}/anime
```

Anime the person worked on (director, voice actor, etc.)

---

### Get Person Voice Roles

```
GET /people/{id}/voices
```

---

### Get Person Manga

```
GET /people/{id}/manga
```

Manga the person authored or illustrated.

---

### Get Person Pictures

```
GET /people/{id}/pictures
```

---

## Producers

### Search Producers

```
GET /producers
```

**Query Parameters:**
- `q` - Search query
- `limit` - Results per page
- `page` - Page number

**Example:**
```bash
curl "https://api.jikan.moe/v4/producers?q=toei"
```

---

### Get Producer by ID

```
GET /ducers/{id}
```

---

### Get Full Producer Details

```
GET /producers/{id}/full
```

---

### Get Producer External Links

```
GET /producers/{id}/external
```

---

## Seasons

### Get Current Season

```
GET /seasons/now
```

**Query Parameters:**
- `filter` - Filter by type (tv, movie, ova, special, ona, music)
- `sfw` - Safe-for-work

**Example:**
```bash
curl "https://api.jikan.moe/v4/seasons/now"
```

**Response:**
```json
{
  "data": [
    {
      "mal_id": 51553,
      "title": "Witch Hat Atelier",
      "images": { "jpg": { "image_url": "https://..." } },
      "type": "TV"
    }
  ],
  "pagination": { "last_visible_page": 7, "has_next_page": true }
}
```

---

### Get Season by Year/Season

```
GET /seasons/{year}/{season}
```

Valid seasons: `winter`, `spring`, `summer`, `fall`

**Example:**
```bash
curl "https://api.jikan.moe/v4/seasons/2024/winter"
```

---

### Get All Seasons

```
GET /seasons
```

Returns list of all available seasons (year + season combinations).

**Example:**
```bash
curl "https://api.jikan.moe/v4/seasons"
```

---

### Get Upcoming Season

```
GET /seasons/upcoming
```

---

## Top

### Top Anime

```
GET /top/anime
```

**Query Parameters:**
- `filter` - tv, movie, ova, special, ona, music, bypopularity, airing, upcoming
- `page` - Page number
- `limit` - Results per page (1-25)
- `sfw` - Safe-for-work

**Example:**
```bash
curl "https://api.jikan.moe/v4/top/anime?limit=5"
```

**Response:**
```json
{
  "data": [
    {
      "mal_id": 52991,
      "title": "Sousou no Frieren",
      "rank": 1,
      "score": 8.75,
      "type": "TV"
    }
  ]
}
```

---

### Top Manga

```
GET /top/manga
```

**Query Parameters:**
- `filter` - manga, manhwa, manhua, lightnovel, novel, bypopularity, publishing
- `page`, `limit`

**Example:**
```bash
curl "https://api.jikan.moe/v4/top/manga?limit=5"
```

---

### Top People

```
GET /top/people
```

**Example:**
```bash
curl "https://api.jikan.moe/v4/top/people?limit=5"
```

---

### Top Characters

```
GET /top/characters
```

**Example:**
```bash
curl "https://api.jikan.moe/v4/top/characters?limit=5"
```

---

### Top Reviews

```
GET /top/reviews
```

---

## Users

### Search Users

```
GET /users
```

**Query Parameters:**
- `q` - Search query

**Example:**
```bash
curl "https://api.jikan.moe/v4/users?q=animelist"
```

---

### Get User by Username

```
GET /users/{username}
```

**Example:**
```bash
curl "https://api.jikan.moe/v4/users/nick"
```

---

### Get Full User Profile

```
GET /users/{username}/full
```

Includes about, stats, updates.

---

### Get User Statistics

```
GET /users/{username}/statistics
```

Anime/manga statistics for the user.

**Example:**
```bash
curl "https://api.jikan.moe/v4/users/nick/statistics"
```

---

### Get User Favorites

```
GET /users/{username}/favorites
```

---

### Get User Updates

```
GET /users/{username}/userupdates
```

Recent anime/manga list updates.

---

### Get User About

```
GET /users/{username}/about
```

---

### Get User by ID

```
GET /users/userbyid/{id}
```

Find user by their MyAnimeList ID.

---

## Watch

### Watch Episodes

```
GET /watch/episodes
```

**Query Parameters:**
- `page` - Page number

**Example:**
```bash
curl "https://api.jikan.moe/v4/watch/episodes"
```

---

### Watch Popular Episodes

```
GET /watch/episodes/popular
```

**Example:**
```bash
curl "https://api.jikan.moe/v4/watch/episodes/promos"
```

---

### Watch Promos

```
GET /watch/promos
```

---

### Watch Popular Promos

```
GET /watch/promos/popular
```

---

## Random

### Random Anime

```
GET /random/anime
```

**Query Parameters:**
- `sfw` - Safe-for-work

**Example:**
```bash
curl "https://api.jikan.moe/v4/random/anime"
```

---

### Random Manga

```
GET /random/manga
```

---

### Random Character

```
GET /random/characters
```

---

### Random Person

```
GET/random/people
```

---

### Random User

```
GET /random/users
```

---

## Other Endpoints

### Get Anime Genres

```
GET /genres/anime
```

**Example:**
```bash
curl "https://api.jikan.moe/v4/genres/anime"
```

---

### Get Manga Genres

```
GET /genres/manga
```

---

### Get Magazines

```
GET /magazines
```

---

### Get Clubs

```
GET /clubs
```

**Query Parameters:**
- `q` - Search query
- `page`, `limit`

---

### Get Club by ID

```
GET /clubs/{id}
```

---

### Get Club Members

```
GET /clubs/{id}/members
```

---

### Get Schedules

```
GET /chedules
```

**Query Parameters:**
- `filter` - monday, tuesday, etc.

**Example:**
```bash
curl "https://api.jikan.moe/v4/schedules?filter=monday"
```

---

### Get Anime Recommendations

```
GET /recommendations/anime
```

---

### Get Manga Recommendations

```
GET /recommendations/manga
```

---

### Get Anime Reviews

```
GET /reviews/anime
```

---

### Get Manga Reviews

```
GET /reviews/manga
```

---

## Error Responses

```json
{
  "status": 400,
  "type": "BadRequestException",
  "message": "Invalid parameter",
  "error": "...",
  "report_url": "https://github.com/..."
}
```

| Status | Meaning |
|--------|---------|
| 200 | Success |
| 304 | Not Modified (cache validation) |
| 400 | Bad Request |
| 404 | Not Found |
| 429 | Rate Limited |
| 500 | Internal Error |
| 503 | Service Unavailable |

---

## Cache Validation

To check if data has changed:

1. Get `ETag` from response headers
2. Send `If-None-Match: <etag>` header in next request
3. If unchanged: returns `304 Not Modified`

**Example:**
```bash
curl -H "If-None-Match: abc123" "https://api.jikan.moe/v4/anime/1"
```

---

## Quick Reference

```bash
# Search anime
curl "https://api.jikan.moe/v4/anime?q=naruto"

# Get anime details
curl "https://api.jikan.moe/v4/anime/1"

# Current season
curl "https://api.jikan.moe/v4/seasons/now"

# Top anime
curl "https://api.jikan.moe/v4/top/anime?limit=10"

# Search characters
curl "https://api.jikan.moe/v4/characters?q=goku"
```

---

## Useful for Anilix

| Use Case | Endpoint |
|----------|----------|
| Anime search | `GET /anime?q={query}` |
| Season anime | `GET /seasons/now` |
| Trending/Top | `GET /top/anime` |
| Character info | `GET /anime/{id}/characters` |
| Streaming links | `GET /anime/{id}/streaming` |
| Related anime | `GET /anime/{id}/relations` |
| Recommendations | `GET /anime/{id}/recommendations` |
| Ratings/Reviews | `GET /anime/{id}/reviews` |