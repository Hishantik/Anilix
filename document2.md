# ani-cli API & Endpoint Architecture Documentation

## Overview

This document explains all major APIs, services, provider endpoints, and external resources used by the ani-cli project.

It also explains:

- How ani-cli communicates with AllAnime
- GraphQL usage
- Provider extraction flow
- Streaming architecture
- m3u8 handling
- Subtitle extraction
- Provider resolution
- External dependencies
- Runtime network flow

---

# Core Architecture

ani-cli is NOT just a simple API client.

It is:

- a GraphQL client
- a provider resolver
- a stream extraction engine
- an HLS parser
- a playback launcher

The main API only provides provider metadata.

The real complexity comes from:
- provider extraction
- decryption
- stream resolution
- HLS parsing

---

# Main Runtime Flow

```text
User Search
    ↓
AllAnime GraphQL API
    ↓
Provider References
    ↓
Provider Embed URLs
    ↓
Provider Extraction
    ↓
m3u8/mp4 Stream URLs
    ↓
mpv/VLC Playback
```

---

# Main APIs Used

| Purpose | Endpoint |
|---|---|
| Main GraphQL API | https://api.allanime.day/api |
| AnimeSchedule API | https://animeschedule.net/api/v3/anime |
| GitHub Raw Update | https://raw.githubusercontent.com/pystardust/ani-cli/master/ani-cli |

---

# Main Base URLs

| Purpose | URL |
|---|---|
| AllAnime Referer | https://allmanga.to |
| AllAnime Base | allanime.day |
| AllAnime API Base | https://api.allanime.day |
| AnimeSchedule | https://animeschedule.net |

---

# 1. AllAnime GraphQL API

Main endpoint:

```text
https://api.allanime.day/api
```

Used for:
- anime search
- episode lists
- provider references
- source metadata

This is the primary backend used by ani-cli.

---

# 2. Anime Search Endpoint

ani-cli performs anime searching using GraphQL.

Example GraphQL operation:

```graphql
query {
  shows(...)
}
```

Purpose:
- search anime titles
- retrieve anime IDs
- retrieve episode counts

Typical request:

```http
POST https://api.allanime.day/api
```

---

# Example Search Request

```json
{
  "variables": {
    "search": {
      "query": "Naruto"
    },
    "limit": 10,
    "page": 1,
    "translationType": "sub",
    "countryOrigin": "ALL"
  },
  "query": "query($search: SearchInput){ shows(search:$search){ edges{ _id name }}}"
}
```

---

# Example Search Response

```json
{
  "data": {
    "shows": {
      "edges": [
        {
          "_id": "abc123",
          "name": "Naruto"
        }
      ]
    }
  }
}
```

---

# 3. Episode List Endpoint

ani-cli fetches episodes using:

```graphql
query {
  show(_id: ...)
}
```

Purpose:
- retrieve episode lists
- determine available episodes
- support sub/dub modes

Typical request:

```http
POST https://api.allanime.day/api
```

---

# Example Episode Query

```graphql
query($showId: String!) {
  show(_id: $showId) {
    availableEpisodesDetail
  }
}
```

---

# 4. Episode Source Endpoint

This endpoint retrieves provider references for a specific episode.

GraphQL operation:

```graphql
query {
  episode(...)
}
```

Purpose:
- retrieve provider source URLs
- retrieve encrypted payloads
- retrieve provider metadata

---

# Example Episode Source Query

```graphql
query(
  $showId: String!,
  $translationType: VaildTranslationTypeEnumType!,
  $episodeString: String!
) {
  episode(
    showId: $showId
    translationType: $translationType
    episodeString: $episodeString
  ) {
    sourceUrls
  }
}
```

---

# Persisted GraphQL Queries

ani-cli sometimes uses persisted GraphQL query mode.

Example:

```text
https://api.allanime.day/api?variables=...&extensions=...
```

Purpose:
- reduce payload size
- optimize requests
- mimic frontend behavior

---

# 5. Provider Extraction System

The AllAnime API does NOT directly return clean streams.

Instead it returns:
- provider references
- encrypted blobs
- obfuscated URLs

ani-cli then extracts actual streams from providers.

---

# Providers Used by ani-cli

| Provider | Stream Type |
|---|---|
| Filemoon | m3u8 |
| Hianime | m3u8 |
| Wixmp | mp4/m3u8 |
| SharePoint | mp4 |
| YouTube mirrors | mp4 |

These providers are dynamically resolved.

---

# Provider Flow

```text
AllAnime API
      ↓
Provider URLs
      ↓
Provider Extractors
      ↓
m3u8/mp4 URLs
      ↓
Playback
```

---

# 6. Provider Embed URLs

ani-cli dynamically requests provider endpoints returned by AllAnime.

Example pattern:

```text
https://allanime.day/<provider_path>
```

These responses often contain:
- encrypted payloads
- HLS manifests
- subtitles
- mirror URLs

---

# 7. Filemoon Extraction

Filemoon responses are encrypted.

ani-cli:
- downloads payload
- base64 decodes
- AES decrypts
- extracts stream URLs

This provider requires special extraction logic.

---

# 8. HLS / m3u8 Endpoints

Many providers return:

```text
master.m3u8
```

These are HLS playlists.

They contain:
- multiple quality streams
- audio tracks
- subtitles

Typical qualities:
- 360p
- 720p
- 1080p

---

# m3u8 Parsing Flow

```text
Provider Response
      ↓
master.m3u8
      ↓
Variant Streams
      ↓
Quality Selection
      ↓
Final Stream URL
```

---

# 9. Subtitle Extraction

Subtitle URLs are extracted dynamically.

Typically:
- VTT subtitles
- language metadata
- subtitle URLs

ani-cli passes subtitles directly to mpv.

---

# 10. AniSkip Integration

ani-cli integrates with AniSkip for intro skipping.

Purpose:
- skip anime intros
- skip recaps
- skip endings

AniSkip uses:
- MAL IDs
- timestamp metadata

This is separate from streaming.

---

# 11. AnimeSchedule API

Used for:
- next episode countdowns
- airing schedules
- release tracking

Endpoint:

```text
https://animeschedule.net/api/v3/anime
```

ani-cli also scrapes anime pages:

```text
https://animeschedule.net/anime/<route>
```

---

# 12. GitHub Raw Update Endpoint

ani-cli supports self-updating.

Endpoint:

```text
https://raw.githubusercontent.com/pystardust/ani-cli/master/ani-cli
```

Purpose:
- download latest script
- compare versions
- patch local installation

---

# Runtime Network Flow

```text
User Search
    ↓
api.allanime.day/api
    ↓
Anime Metadata
    ↓
Provider References
    ↓
Provider URLs
    ↓
Provider Extraction
    ↓
m3u8/mp4 URLs
    ↓
mpv/VLC
```

---

# Real Architecture Insight

ani-cli primarily depends on ONE central API:

```text
https://api.allanime.day/api
```

BUT actual streaming depends on MANY external providers and CDNs.

So in reality the architecture is:

```text
1 metadata/provider-index API
+
many streaming provider endpoints
```

---

# Why This Matters

When building your own project:

You must understand that:

- the GraphQL API is only the entry point
- providers are the actual stream sources
- extraction is provider-specific
- providers frequently change

---

# Suggested Architecture for Your Project

```text
Metadata Layer
    ↓
Jikan API

Provider Index Layer
    ↓
AllAnime GraphQL

Provider Layer
    ├── Filemoon
    ├── Hianime
    ├── Wixmp
    └── SharePoint

Extraction Layer
    ↓
AES + m3u8 Parsing

Playback Layer
    ↓
mpv/VLC
```

---

# Recommended Design Principles

## Keep Providers Modular

Do NOT hardcode provider logic everywhere.

---

## Normalize Streams

Convert ALL providers into one internal stream model.

---

## Cache Aggressively

Cache:
- anime IDs
- episode lists
- extracted streams

---

## Expect Breakage

Providers change often.

Build isolated extractors.

---

# Final Takeaway

The real complexity of ani-cli is NOT the API itself.

The real complexity is:

- provider extraction
- stream resolution
- AES decryption
- HLS parsing
- subtitle extraction
- provider instability

The AllAnime GraphQL API is only the first layer of the streaming pipeline.
