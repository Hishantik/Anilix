# Complete Go TUI Anime Streaming Architecture & ani-cli Query System Documentation

This document combines:
- AllAnime GraphQL integration
- ani-cli query implementation
- provider architecture
- AES decryption
- m3u8 extraction
- Cloudflare-safe practices
- Go architecture recommendations
- multi-provider handling
- stream extraction flow

## Main GraphQL Endpoint

https://api.allanime.day/api

## Main Runtime Flow

User Search
↓
Jikan Search
↓
Anime Selection
↓
AllAnime GraphQL
↓
Provider References
↓
Provider Resolution
↓
Provider Extraction
↓
m3u8/mp4 Streams
↓
mpv/VLC Playback

## ani-cli Query Architecture

GraphQL Query String
↓
Variables JSON
↓
curl POST Request
↓
Raw JSON Response
↓
sed/grep extraction

## Main Query Types

1. Anime Search Query
2. Episode List Query
3. Episode Source Query
4. Persisted GraphQL Queries

## Search Query Example

query(
  $search: SearchInput
  $limit: Int
  $page: Int
  $translationType: VaildTranslationTypeEnumType
  $countryOrigin: VaildCountryOriginEnumType
) {
  shows(
    search: $search
    limit: $limit
    page: $page
    translationType: $translationType
    countryOrigin: $countryOrigin
  ) {
    edges {
      _id
      name
      availableEpisodes
    }
  }
}

## Query Variables

- query
- translationType
- limit
- page
- countryOrigin
- allowAdult
- allowUnknown

## Sub vs Dub

sub = subtitled anime

dub = dubbed anime

## Episode Query Example

query($showId: String!) {
  show(_id: $showId) {
    availableEpisodesDetail
  }
}

## Episode Source Query

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

## Provider Architecture

Provider Resolver
↓
Filemoon
Hianime
Wixmp
YouTube
↓
Stream Extraction
↓
Normalized Streams

## Recommended Provider Interface

```go
type Provider interface {
    Name() string
    CanHandle(url string) bool
    Extract(url string) ([]Stream, error)
}
```

## AES Extraction Pipeline

Provider Response
↓
Base64 Decode
↓
Extract IV
↓
AES-CTR Decrypt
↓
Recover Provider JSON
↓
Extract Stream URLs

## AES Decryption in Go

```go
block, err := aes.NewCipher(key)
stream := cipher.NewCTR(block, iv)
stream.XORKeyStream(dst, ciphertext)
```

## m3u8 Flow

Provider Response
↓
master.m3u8
↓
Variant Streams
↓
Quality Selection
↓
Final Stream URL

## Referrer Handling

Many providers require:

Referer: https://allmanga.to

## mpv Integration

```go
exec.Command(
    "mpv",
    "--referrer="+stream.Referer,
    stream.URL,
).Start()
```

## Cloudflare-safe Practices

- Use browser headers
- Reuse HTTP client
- Enable cookie jar
- Add rate limiting
- Cache aggressively

## Recommended Go Libraries

- bubbletea
- lipgloss
- grafov/m3u8
- go-mpvjsonipc
- chromedp
- rod

## Final Takeaway

The real complexity is NOT the API itself.

The real complexity is:
- provider extraction
- AES decryption
- m3u8 parsing
- subtitle extraction
- provider instability
