# Go TUI Anime Streaming Client Architecture Guide

## Overview

This document explains how to build a scalable anime streaming TUI application in Go using:

- Jikan API for metadata
- AllAnime GraphQL for provider indexing
- Modular provider extractors for streams
- mpv/VLC for playback

It also explains:

- AES decryption
- Base64 decoding
- m3u8 extraction
- Referrer handling
- Cloudflare-safe practices
- Multi-provider architecture
- Caching
- Parallel extraction
- Long-term maintainability

---

# Core Philosophy

Treat:

- Jikan as your stable anime database
- AllAnime as a provider index
- Providers as independent extraction modules

This separation keeps your application maintainable.

---

# High-Level Architecture

```text
                ┌────────────┐
                │    TUI     │
                └─────┬──────┘
                      │
             ┌────────┴────────┐
             │ Service Layer   │
             └───────┬─────────┘
                     │
        ┌────────────┴────────────┐
        │                         │
┌───────▼────────┐      ┌────────▼────────┐
│ Jikan Client   │      │ AllAnime Client │
└───────┬────────┘      └────────┬────────┘
        │                        │
        │              ┌────────▼────────┐
        │              │ Provider Resolver│
        │              └────────┬────────┘
        │                       │
        │         ┌─────────────┼─────────────┐
        │         │             │             │
        ▼         ▼             ▼             ▼
   Filemoon    Hianime       Wixmp       YouTube
        │         │             │             │
        └─────────┴──────┬──────┴─────────────┘
                         │
                 ┌───────▼────────┐
                 │ Stream Extractor│
                 └───────┬────────┘
                         │
                 ┌───────▼────────┐
                 │ m3u8 + AES Layer│
                 └───────┬────────┘
                         │
                  ┌──────▼──────┐
                  │ mpv / VLC   │
                  └─────────────┘
```

---

# What Each API Should Handle

## Jikan API

Use Jikan for:

- Search
- Posters
- Metadata
- Ratings
- MAL IDs
- Genres
- Recommendations
- Synopses

Example:

```http
GET https://api.jikan.moe/v4/anime?q=naruto
```

Jikan should be your canonical anime database.

---

## AllAnime API

Use AllAnime only for:

- Episode lists
- Provider references
- Stream source metadata

Do NOT use it as your primary metadata database.

---

# Recommended Workflow

```text
User searches anime
        ↓
Jikan Search
        ↓
User selects anime
        ↓
Resolve AllAnime ID
        ↓
Fetch episode list
        ↓
Fetch provider references
        ↓
Resolve matching providers
        ↓
Extract streams
        ↓
Parse m3u8
        ↓
Launch mpv
```

---

# Recommended Project Structure

```text
cmd/

internal/

    api/
        jikan.go
        allanime.go

    providers/
        provider.go
        filemoon.go
        hianime.go
        wixmp.go
        youtube.go

    extractor/
        decrypt.go
        m3u8.go

    player/
        mpv.go

    tui/
        search.go
        episodes.go
        player.go

    cache/
        sqlite.go
```

---

# Clean API Design

## NEVER expose GraphQL directly

Bad:

```go
map[string]interface{}
```

Good:

```go
func SearchAnime(title string) ([]Anime, error)

func GetEpisodes(id string) ([]Episode, error)

func GetStreams(id string, ep string) ([]Stream, error)
```

Your TUI should never know GraphQL exists internally.

---

# Suggested Models

```go
type Anime struct {
    MALID int
    AllAnimeID string
    Title string
}

type Episode struct {
    Number string
}

type Stream struct {
    Provider string
    Quality string
    URL string
    Referer string
    Subtitle string
}
```

---

# Multi-Provider Architecture

This is the MOST important architectural decision.

Do NOT hardcode provider logic across the app.

Instead build isolated provider modules.

---

# Provider Interface

```go
type Provider interface {
    Name() string

    CanHandle(url string) bool

    Extract(url string) ([]Stream, error)
}
```

This keeps providers modular.

---

# Example Provider

```go
type FilemoonProvider struct{}

func (p FilemoonProvider) Name() string {
    return "filemoon"
}

func (p FilemoonProvider) CanHandle(url string) bool {
    return strings.Contains(url, "filemoon")
}

func (p FilemoonProvider) Extract(url string) ([]Stream, error) {
    // decrypt payload
    // parse m3u8
    return nil, nil
}
```

---

# Provider Registry

```go
var providers = []Provider{
    FilemoonProvider{},
    HianimeProvider{},
    WixmpProvider{},
}
```

---

# Provider Resolution

```go
func ResolveProvider(url string) Provider {
    for _, p := range providers {
        if p.CanHandle(url) {
            return p
        }
    }

    return nil
}
```

---

# Generic Extraction Flow

```go
provider := ResolveProvider(url)

streams, err := provider.Extract(url)
```

No giant switch statements.

---

# Why Provider Isolation Matters

Providers constantly:

- change domains
- alter encryption
- modify payload formats
- break extraction logic

With isolated providers:

```text
only one provider module changes
```

instead of rewriting the entire application.

---

# Parallel Extraction

Go excels at concurrent provider extraction.

Example:

```go
var wg sync.WaitGroup

results := make(chan []Stream)

for _, source := range sources {

    wg.Add(1)

    go func(src string) {
        defer wg.Done()

        provider := ResolveProvider(src)

        if provider == nil {
            return
        }

        streams, err := provider.Extract(src)

        if err == nil {
            results <- streams
        }

    }(source)
}

wg.Wait()
close(results)
```

---

# Stream Prioritization

Not all providers are equally reliable.

Example:

```go
var ProviderPriority = map[string]int{
    "hianime":  1,
    "filemoon": 2,
    "wixmp":    3,
}
```

Sort streams before display.

---

# Encoding vs Encryption

## Encoding

Encoding is NOT security.

Example:

```text
hello
↓
aGVsbG8=
```

This is Base64.

Purpose:

- binary transport
- avoiding special characters

---

## Encryption

Encryption hides data using a key.

AllAnime commonly uses:

```text
AES-256-CTR
```

---

# AES Extraction Pipeline

```text
API Response
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
```

---

# AES Decryption in Go

```go
block, err := aes.NewCipher(key)
if err != nil {
    panic(err)
}

stream := cipher.NewCTR(block, iv)
stream.XORKeyStream(dst, ciphertext)
```

---

# Obfuscation

Some providers additionally remap characters.

Example:

```text
79 -> A
7a -> B
08 -> 0
```

This is NOT encryption.

It is anti-scraping obfuscation.

Port this logic separately.

---

# m3u8 Streaming

Most providers return:

```text
master.m3u8
```

This is an HLS playlist.

It contains multiple qualities.

You should:

1. Fetch playlist
2. Parse variants
3. Select quality
4. Pass final URL to mpv

---

# Recommended m3u8 Library

```text
github.com/grafov/m3u8
```

---

# Referrer Handling

Many providers require:

```http
Referer: https://allmanga.to
```

Without it:
- playback fails
- 403 errors occur

---

# Passing Referrer to mpv

```go
exec.Command(
    "mpv",
    "--referrer="+stream.Referer,
    stream.URL,
).Start()
```

---

# mpv Integration

Simplest method:

```go
exec.Command("mpv", streamURL).Start()
```

Advanced method:
- mpv IPC

This enables:
- pause
- seek
- subtitles
- next episode

---

# Cloudflare and Anti-Bot Handling

## IMPORTANT

Do NOT attempt to bypass CAPTCHAs or defeat access controls.

Instead:
- behave like a legitimate browser
- minimize suspicious traffic
- cache aggressively

---

# Proper Browser Headers

```go
req.Header.Set(
    "User-Agent",
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64)",
)

req.Header.Set("Referer", "https://allmanga.to")
req.Header.Set("Origin", "https://allmanga.to")
```

---

# Reuse HTTP Client

Bad:

```go
http.Get(...)
```

Good:

```go
client := &http.Client{}
```

Reuse the same client globally.

---

# Enable Cookie Jar

```go
jar, _ := cookiejar.New(nil)

client := &http.Client{
    Jar: jar,
}
```

This preserves sessions and cookies.

---

# Add Rate Limiting

Avoid:
- request spam
- infinite retries
- aggressive scraping

Recommended:

```text
1–3 requests per second
```

---

# Caching Strategy

Caching is CRITICAL.

---

# Permanent Cache

Store:

```text
MAL ID -> AllAnime ID
```

---

# Short-Term Cache

Store:
- episode lists
- stream extraction
- subtitles

Example TTLs:

```text
Episodes: 1 day
Streams: 30 minutes
```

---

# Recommended Storage

- SQLite
- Redis
- BoltDB

SQLite is usually enough for a TUI app.

---

# When JavaScript Challenges Appear

Some providers require JS execution.

Legitimate solution:
- chromedp
- Rod

Recommended flow:

```text
Normal HTTP Client
        ↓
If challenge appears
        ↓
Launch headless browser once
        ↓
Obtain session cookies
        ↓
Reuse cookies in HTTP client
```

---

# What To Avoid

## Avoid Tight Coupling

Do NOT spread provider logic across the app.

---

## Avoid Using AllAnime as Main Database

Use Jikan for metadata.

---

## Avoid Infinite Retries

This increases blocks.

---

## Avoid CAPTCHA Solvers

They are fragile and may violate service terms.

---

## Avoid Hardcoding Provider Logic

Providers constantly change.

---

# Recommended Go Libraries

## TUI

```text
github.com/charmbracelet/bubbletea
```

## Styling

```text
github.com/charmbracelet/lipgloss
```

## m3u8

```text
github.com/grafov/m3u8
```

## mpv IPC

```text
github.com/iamscottxu/go-mpvjsonipc
```

## Browser Automation

```text
github.com/chromedp/chromedp
```

or

```text
github.com/go-rod/rod
```

---

# Final Recommendations

Build your architecture around these principles:

## Metadata Layer
Use:
- Jikan

## Provider Index Layer
Use:
- AllAnime GraphQL

## Stream Layer
Use:
- modular provider extractors

## Playback Layer
Use:
- mpv

---

# Most Important Takeaway

The real complexity is NOT the API itself.

The real complexity is:

- provider extraction
- AES decryption
- m3u8 parsing
- referrer handling
- provider instability

If your provider architecture is modular and isolated:

your application becomes:
- maintainable
- scalable
- resilient
- easy to debug
- easy to extend
