# AllAnime Integration Fixes

This document details the fixes made to the AllAnime provider implementation to match ani-cli's behavior.

## Date: 2026-05-14

## Problem Summary

The AllAnime integration was not working correctly. Three main issues were identified:

1. **Episode list queries were not returning data** - Search worked but episode lists returned empty
2. **Persisted queries were being attempted for non-episode queries** - Caused failures for episode list queries
3. **Stream source parsing was failing** - Decrypted content wasn't being parsed correctly

## Changes Made

### 1. Fixed Persisted Query Handling

**File:** `provider/allanime/client.go`

**Problem:** The `doGraphQLCurlPersisted()` function was being called for ALL queries, including episode list queries. This caused failures because:
- Persisted queries only work with a specific hash for episode sources
- Episode list queries don't have an `episodeString` parameter

**Fix:** Added a check to only use persisted queries when `episodeString` is present:

```go
// Check for episodeString - only episode queries should use persisted
if _, hasEpisodeString := req.Variables["episodeString"]; !hasEpisodeString {
    return nil, fmt.Errorf("not an episode query (no episodeString)")
}
```

**Reference:** ani-cli only uses persisted queries for episode sources (line 273-284 of ani-cli script):
```shell
# Persisted query hash for episode
query_hash="d405d0edd690624b66baba3068e0edc3ac90f1597d898a1ec8db4e5c43c00fec"
```

### 2. Fixed Response Format Detection

**File:** `provider/allanime/client.go`

**Problem:** The `GetEpisodeSources()` function was expecting `{"data":{"episode":{"sourceUrls":[...]}}}` format, but persisted queries return `{"_m":"b7","tobeparsed":"..."}`.

**Fix:** Updated to handle both response formats:

```go
// Check for persisted query format - direct _m and tobeparsed (no data wrapper)
if toBeParsed, ok := rawResp["tobeparsed"].(string); ok && toBeParsed != "" {
    return decodeToBeParsed(toBeParsed)
}

// Check for GraphQL wrapper format
if data, ok := rawResp["data"].(map[string]interface{}); ok {
    if toBeParsed, ok := data["tobeparsed"].(string); ok && toBeParsed != "" {
        return decodeToBeParsed(toBeParsed)
    }
    // ... handle regular episode format
}
```

### 3. Fixed URL Parsing from Decrypted Content

**File:** `provider/allanime/decoder.go`

**Problem:** The `parseSourceUrls()` function was looking for `"--url"` pattern, but the actual decrypted content contains `"sourceUrl":"url"` without the double dash prefix (or single dash in some cases).

**Fix:** Updated the regex patterns to handle both formats:

```go
urlRe := regexp.MustCompile(`"sourceUrl"\s*:\s*"([^"]*)"`)
nameRe := regexp.MustCompile(`"sourceName"\s*:\s*"([^"]*)"`)

urlMatches := urlRe.FindAllStringSubmatch(jsonStr, -1)
nameMatches := nameRe.FindAllStringSubmatch(jsonStr, -1)

for i, urlMatch := range urlMatches {
    url := urlMatch[1]

    // Handle hex-encoded provider IDs (start with --)
    if strings.HasPrefix(url, "--") {
        decoded := decodeHexProviderID(url[2:])
        // ...
    }
}
```

### 4. Added Interface-based Source URL Parser

**File:** `provider/allanime/client.go`

**Problem:** Needed to handle source URLs when they come as `[]interface{}` from JSON parsing.

**Fix:** Added `parseSourceUrlsFromInterface()` function:

```go
func parseSourceUrlsFromInterface(urls []interface{}) []SourceUrl {
    result := make([]SourceUrl, 0, len(urls))
    for _, u := range urls {
        if m, ok := u.(map[string]interface{}); ok {
            src := SourceUrl{}
            if name, ok := m["sourceName"].(string); ok {
                src.SourceName = name
            }
            if url, ok := m["url"].(string); ok {
                src.SourceUrl = url
            }
            // ...
        }
    }
    return result
}
```

## API Response Formats

### Search Query Response
```json
{
  "data": {
    "shows": {
      "edges": [
        {
          "_id": "vkD8H5e7HsG2jctw9",
          "name": "Boruto: Naruto Next Generations",
          "malId": "34566",
          "availableEpisodes": { "sub": 293, "dub": 293 }
        }
      ]
    }
  }
}
```

### Episode List Query Response
```json
{
  "data": {
    "show": {
      "_id": "vkD8H5e7HsG2jctw9",
      "name": "Boruto: Naruto Next Generations",
      "availableEpisodesDetail": {
        "sub": ["293", "292", ... "1"],
        "dub": ["293", "292", ... "1"],
        "raw": []
      }
    }
  }
}
```

### Episode Sources Response (Persisted Query - Encrypted)
```json
{
  "_m": "b7",
  "tobeparsed": "AZPrT752E+d4J+pHj55Ft7Sv6alsKMMRvEdLX0d8ETgw2sVjGdt6BK2d2SfewIo6k..."
}
```

### Episode Sources Response (Regular GraphQL - Decrypted)
```json
{
  "episode": {
    "episodeString": "1",
    "sourceUrls": [
      {
        "sourceUrl": "//vidstreaming.io/load.php?id=OTc2MzI=",
        "sourceName": "Vid-mp4",
        "priority": 4,
        "type": "iframe"
      },
      {
        "sourceUrl": "https://tools.fast4speed.rsvp/media/videos/...",
        "sourceName": "Yt-mp4",
        "priority": 7.9,
        "type": "player"
      }
    ]
  }
}
```

## Decryption Process

The `tobeparsed` field contains Base64-encoded AES-256-CTR encrypted data.

### Steps:
1. **Base64 Decode** the `tobeparsed` string
2. **Extract IV** - bytes 1-12 of decoded data
3. **Create CTR counter** - IV + `00000002` (16 bytes total)
4. **Extract ciphertext** - bytes 13 to (length - 16)
5. **AES-256-CTR Decrypt** using:
   - Key: SHA256("Xot36i3lK3:v1") = `a254aa27c410f297bd04ba33a0c0df7ff4e706bf3ae27271c6703f84e750f552`
   - CTR counter as IV

### Shell Equivalent (from ani-cli):
```bash
printf '%s' "$tobeparsed" | base64 -d >"$tmp"
iv="$(dd if="$tmp" bs=1 skip=1 count=12 | od -A n -t x1 | tr -d ' \n')"
ctr="${iv}00000002"
plain="$(dd if="$tmp" bs=1 skip=13 count="$ct_len" | openssl enc -d -aes-256-ctr -K "$allanime_key" -iv "$ctr")"
```

## Testing

All integration tests now pass:

```
=== RUN   TestIntegration_GetEpisodesByShowID
    Episode counts - Sub: 293, Dub: 293
    Found 8 sources
    - Provider: Vid-mp4, URL: //vidstreaming.io/load.php?id=...
    - Provider: Yt-mp4, URL: https://tools.fast4speed.rsvp/...
    - Provider: Ss-Hls, URL: https://streamsb.net/...
    - Provider: Ok, URL: https://ok.ru/...
    - Provider: Mp4, URL: https://mp4upload.com/...
--- PASS

=== RUN   TestIntegration_AllanimeSearch
    Found 20 results
    Episode counts - Sub: 293, Dub: 293
    Found 7 sources
--- PASS

=== RUN   TestIntegration_GetEpisodeSources
    Found 6 sources
--- PASS

=== RUN   TestIntegration_Provider
    Found 3 results (Cowboy Bebop series)
--- PASS
```

## Files Modified

1. `provider/allanime/client.go` - Fixed persisted query handling and response parsing
2. `provider/allanime/decoder.go` - Fixed URL parsing from decrypted content
3. `provider/allanime/integration_test.go` - Added direct episode source test

## References

- [ani-cli GitHub](https://github.com/pystardust/ani-cli)
- AllAnime API: `https://api.allanime.day/api`
- Persisted query hash: `d405d0edd690624b66baba3068e0edc3ac90f1597d898a1ec8db4e5c43c00fec`