# AllAnime API Query Documentation

## Overview

This document explains how to query anime data from the AllAnime GraphQL API.

It covers:

- GraphQL endpoint usage
- Anime searching
- Query variables
- Sub vs Dub modes
- Episode retrieval
- Stream source retrieval
- Example requests
- Example responses
- Go integration examples
- ani-cli query flow

---

# Main GraphQL Endpoint

AllAnime uses a GraphQL API.

Main endpoint:

https://api.allanime.day/api

All anime searching, episode retrieval, and provider resolution begins from this endpoint.

---

# Main Query Flow

Anime Search
      ↓
Retrieve Anime ID
      ↓
Fetch Episode List
      ↓
Fetch Episode Sources
      ↓
Provider Extraction
      ↓
m3u8/mp4 Streams

---

# Search Anime Query

Anime searching is performed using the `shows` GraphQL query.

---

# Example Search Request

Search for Naruto:

{
  "variables": {
    "search": {
      "allowAdult": false,
      "allowUnknown": false,
      "query": "Naruto"
    },
    "limit": 10,
    "page": 1,
    "translationType": "sub",
    "countryOrigin": "ALL"
  },
  "query": "query($search: SearchInput $limit: Int $page: Int $translationType: VaildTranslationTypeEnumType $countryOrigin: VaildCountryOriginEnumType){ shows(search:$search limit:$limit page:$page translationType:$translationType countryOrigin:$countryOrigin){ edges{ _id name availableEpisodes }}}"
}

---

# Search Request Using curl

curl -X POST "https://api.allanime.day/api" \
  -H "Content-Type: application/json" \
  -d '{...}'

---

# Example Search Response

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

---

# Important Search Fields

- `_id` = internal anime ID
- `name` = anime title
- `availableEpisodes` = episode counts

The `_id` is required for:
- episode retrieval
- provider extraction
- stream extraction

---

# Query Parameters Explained

- `query` = anime title
- `translationType` = sub or dub
- `limit` = result count
- `page` = pagination
- `countryOrigin` = ALL
- `allowAdult` = adult filter
- `allowUnknown` = unknown filter

---

# Sub vs Dub Mode

Sub:

"translationType": "sub"

Dub:

"translationType": "dub"

---

# Episode List Query

query($showId: String!) {
  show(_id: $showId) {
    availableEpisodesDetail
  }
}

---

# Example Episode List Request

{
  "variables": {
    "showId": "abc123"
  },
  "query": "query($showId:String!){ show(_id:$showId){ availableEpisodesDetail }}"
}

---

# Episode Source Query

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

---

# Important Architecture Insight

The GraphQL API is only the FIRST layer.

The difficult part happens AFTER the API response:

- provider extraction
- AES decryption
- m3u8 parsing
- subtitle extraction
- referrer handling

---

# ani-cli Internal Flow

Search Anime
      ↓
Get Anime ID
      ↓
Fetch Episode List
      ↓
Fetch Provider Sources
      ↓
Resolve Providers
      ↓
Extract Streams
      ↓
Launch mpv

---

# Important Takeaway

The AllAnime GraphQL API primarily provides:

- anime metadata
- provider references
- episode metadata

It does NOT directly provide:
- final playback URLs
- clean stream URLs

You must additionally implement:
- provider extractors
- AES decryption
- m3u8 handling
- stream normalization
