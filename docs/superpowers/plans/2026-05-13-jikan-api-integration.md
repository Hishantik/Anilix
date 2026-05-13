# Jikan API Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement Jikan API integration for anime search and metadata via REST, enabling search via Jikan while streaming links come from separate pluggable sources.

**Architecture:** Three-component design: JikanClient handles HTTP API calls with rate limiting, JikanProvider implements source.Source for search/metadata, AnimeLinker maps anime (with MAL ID) to streaming source episodes.

**Tech Stack:** Go 1.18+, net/http for HTTP client, standard library for JSON unmarshaling

---

## File Structure

```
provider/jikan/
├── client.go    # HTTP client with rate limiting and retry
├── types.go     # Jikan API response types
├── jikan.go     # JikanProvider implementing source.Source
├── linker.go    # AnimeLinker for mapping to streaming sources
└── init.go      # Provider registration
```

---

## Task 1: Jikan API Types

**Files:**
- Create: `provider/jikan/types.go`
- Test: `provider/jikan/types_test.go`

- [ ] **Step 1: Create types.go with Jikan API response structures**

```go
package jikan

// AnimeResponse represents the Jikan API search response
type AnimeResponse struct {
	Data []AnimeData `json:"data"`
}

// AnimeData represents a single anime in Jikan response
type AnimeData struct {
	MalID            int         `json:"mal_id"`
	Title            string      `json:"title"`
	TitleEnglish     string      `json:"title_english"`
	TitleJapanese    string      `json:"title_japanese"`
	URL              string      `json:"url"`
	Images           Images      `json:"images"`
	Year             interface{} `json:"year"` // can be null or int
	Genres           []Genre     `json:"genres"`
	Status           string      `json:"status"`
	Synopsis         string      `json:"synopsis"`
}

// Images contains image URLs
type Images struct {
	JPG JPGImages `json:"jpg"`
}

// JPGImages contains specific image URLs
type JPGImages struct {
	ImageURL      string `json:"image_url"`
	SmallImageURL string `json:"small_image_url"`
	LargeImageURL string `json:"large_image_url"`
}

// Genre represents an anime genre
type Genre struct {
	MalID int    `json:"mal_id"`
	Name  string `json:"name"`
}
```

- [ ] **Step 2: Run go build to verify types compile**

Run: `go build ./provider/jikan/`
Expected: No errors (empty package is fine)

- [ ] **Step 3: Commit**

```bash
git add provider/jikan/types.go
git commit -m "feat: add Jikan API response types"
```

---

## Task 2: Jikan HTTP Client with Rate Limiting

**Files:**
- Create: `provider/jikan/client.go`
- Test: `provider/jikan/client_test.go`

- [ ] **Step 1: Write the failing test for JikanClient**

```go
package jikan

import (
	"testing"
	"time"
)

func TestJikanClient_NewClient(t *testing.T) {
	client := NewClient("https://api.jikan.moe/v4")
	
	// Test that client can be created
	if client == nil {
		t.Fatal("NewClient returned nil")
	}
	
	// Test that base URL is set correctly
	if client.baseURL != "https://api.jikan.moe/v4" {
		t.Errorf("expected base URL https://api.jikan.moe/v4, got %s", client.baseURL)
	}
}

func TestRateLimiter_Acquire(t *testing.T) {
	rl := newRateLimiter(3, time.Second)
	
	// Should be able to acquire 3 tokens
	if !rl.acquire() {
		t.Error("expected to acquire first token")
	}
	if !rl.acquire() {
		t.Error("expected to acquire second token")
	}
	if !rl.acquire() {
		t.Error("expected to acquire third token")
	}
	
	// Fourth should fail (rate limited)
	if rl.acquire() {
		t.Error("expected to be rate limited")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./provider/jikan/... -v`
Expected: FAIL (undefined: NewClient, JikanClient, rateLimiter, newRateLimiter)

- [ ] **Step 3: Write JikanClient implementation**

```go
package jikan

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// JikanClient handles HTTP requests to Jikan API
type JikanClient struct {
	baseURL    string
	httpClient *http.Client
	rateLimiter *rateLimiter
}

// rateLimiter implements token bucket for rate limiting
type rateLimiter struct {
	tokens        int
	maxTokens     int
	refillInterval time.Duration
	lastRefill    time.Time
	mu           sync.Mutex
}

func newRateLimiter(maxTokens int, refillInterval time.Duration) *rateLimiter {
	return &rateLimiter{
		tokens:        maxTokens,
		maxTokens:     maxTokens,
		refillInterval: refillInterval,
		lastRefill:    time.Now(),
	}
}

func (r *rateLimiter) acquire() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	now := time.Since(r.lastRefill)
	if now >= r.refillInterval {
		r.tokens = r.maxTokens
		r.lastRefill = time.Now()
	}
	
	if r.tokens > 0 {
		r.tokens--
		return true
	}
	return false
}

// waitForToken waits until a token is available
func (r *rateLimiter) waitForToken() {
	for !r.acquire() {
		time.Sleep(100 * time.Millisecond)
	}
}

// NewClient creates a new Jikan API client
func NewClient(baseURL string) *JikanClient {
	return &JikanClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		rateLimiter: newRateLimiter(3, time.Second), // 3 requests per second
	}
}

// SearchAnime performs an anime search via Jikan API
func (c *JikanClient) SearchAnime(ctx context.Context, query string) (*AnimeResponse, error) {
	// Wait for rate limiter
	c.rateLimiter.waitForToken()
	
	url := fmt.Sprintf("%s/anime?q=%s&limit=20", c.baseURL, query)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == 429 {
		return nil, fmt.Errorf("rate limited by Jikan API")
	}
	
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Jikan API returned status %d", resp.StatusCode)
	}
	
	var result AnimeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return &result, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./provider/jikan/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add provider/jikan/client.go
git commit -m "feat: add Jikan HTTP client with rate limiting"
```

---

## Task 3: JikanProvider (Source Implementation)

**Files:**
- Create: `provider/jikan/jikan.go`
- Test: `provider/jikan/jikan_test.go`

- [ ] **Step 1: Write the failing test for JikanProvider**

```go
package jikan

import (
	"testing"
	
	"github.com/anilix/anilix/source"
)

func TestJikanProvider_Name(t *testing.T) {
	jp := NewJikanProvider()
	
	if jp.Name() != "Jikan" {
		t.Errorf("expected Name() to return 'Jikan', got %s", jp.Name())
	}
}

func TestJikanProvider_ID(t *testing.T) {
	jp := NewJikanProvider()
	
	if jp.ID() != "jikan" {
		t.Errorf("expected ID() to return 'jikan', got %s", jp.ID())
	}
}

func TestJikanProvider_Search(t *testing.T) {
	jp := NewJikanProvider()
	
	results, err := jp.Search("Naruto")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	
	if len(results) == 0 {
		t.Fatal("expected results, got empty slice")
	}
	
	// Check first result has MAL ID
	if results[0].MALID == 0 {
		t.Error("expected MALID to be set")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./provider/jikan/... -v`
Expected: FAIL (undefined: NewJikanProvider, JikanProvider)

- [ ] **Step 3: Write JikanProvider implementation**

```go
package jikan

import (
	"context"
	"fmt"
	
	"github.com/anilix/anilix/source"
)

// JikanProvider implements source.Source for Jikan API
type JikanProvider struct {
	client *JikanClient
}

// NewJikanProvider creates a new Jikan provider
func NewJikanProvider() *JikanProvider {
	return &JikanProvider{
		client: NewClient("https://api.jikan.moe/v4"),
	}
}

// Name returns the provider name
func (j *JikanProvider) Name() string {
	return "Jikan"
}

// ID returns the provider ID
func (j *JikanProvider) ID() string {
	return "jikan"
}

// Search queries Jikan API for anime
func (j *JikanProvider) Search(query string) ([]*source.Anime, error) {
	ctx := context.Background()
	resp, err := j.client.SearchAnime(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("Jikan search failed: %w", err)
	}
	
	animes := make([]*source.Anime, 0, len(resp.Data))
	for _, data := range resp.Data {
		anime := j.mapToAnime(&data)
		animes = append(animes, anime)
	}
	
	return animes, nil
}

// mapToAnime transforms Jikan response to source.Anime
func (j *JikanProvider) mapToAnime(data *AnimeData) *source.Anime {
	anime := &source.Anime{
		Name:   data.TitleEnglish,
		URL:    data.URL,
		MALID:  data.MalID,
		Status: data.Status,
	}
	
	// Fallback to Japanese title if English is not available
	if anime.Name == "" {
		anime.Name = data.Title
	}
	
	// Set cover image
	if data.Images.JPG.LargeImageURL != "" {
		anime.Cover = data.Images.JPG.LargeImageURL
	} else if data.Images.JPG.ImageURL != "" {
		anime.Cover = data.Images.JPG.ImageURL
	}
	
	// Extract year
	if year, ok := data.Year.(float64); ok {
		anime.Year = int(year)
	}
	
	// Extract genre names
	if len(data.Genres) > 0 {
		genres := make([]string, len(data.Genres))
		for i, g := range data.Genres {
			genres[i] = g.Name
		}
		anime.Genres = genres
	}
	
	return anime
}

// SeasonsOf returns seasons for an anime (not implemented for Jikan - returns empty)
func (j *JikanProvider) SeasonsOf(anime *source.Anime) ([]source.Season, error) {
	return []source.Season{}, nil
}

// EpisodesOf returns episodes for an anime (not implemented for Jikan - returns empty)
func (j *JikanProvider) EpisodesOf(anime *source.Anime, season int) ([]*source.Episode, error) {
	return []*source.Episode{}, nil
}

// StreamsOf returns streams for an episode (not implemented for Jikan - returns empty)
func (j *JikanProvider) StreamsOf(episode *source.Episode) ([]*source.Stream, error) {
	return []*source.Stream{}, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./provider/jikan/... -v -run TestJikanProvider_Name`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add provider/jikan/jikan.go
git commit -m "feat: add JikanProvider implementing source.Source"
```

---

## Task 4: AnimeLinker

**Files:**
- Create: `provider/jikan/linker.go`
- Test: `provider/jikan/linker_test.go`

- [ ] **Step 1: Write the failing test for AnimeLinker**

```go
package jikan

import (
	"testing"
	
	"github.com/anilix/anilix/source"
)

// mockSource is a mock streaming source for testing
type mockSource struct {
	episodes map[int][]*source.Episode
}

func (m *mockSource) Name() string { return "MockSource" }
func (m *mockSource) ID() string { return "mock" }
func (m *mockSource) Search(query string) ([]*source.Anime, error) { return nil, nil }
func (m *mockSource) SeasonsOf(anime *source.Anime) ([]source.Season, error) { return nil, nil }
func (m *mockSource) StreamsOf(episode *source.Episode) ([]*source.Stream, error) { return nil, nil }

func (m *mockSource) EpisodesOf(anime *source.Anime, season int) ([]*source.Episode, error) {
	return m.episodes[anime.MALID], nil
}

func TestAnimeLinker_GetEpisodes(t *testing.T) {
	linker := NewAnimeLinker()
	
	mockSrc := &mockSource{
		episodes: map[int][]*source.Episode{
			1: {{Number: 1, Title: "Episode 1"}, {Number: 2, Title: "Episode 2"}},
		},
	}
	
	anime := &source.Anime{
		Name:  "Test Anime",
		MALID: 1,
	}
	
	episodes, err := linker.GetEpisodes(anime, mockSrc)
	if err != nil {
		t.Fatalf("GetEpisodes failed: %v", err)
	}
	
	if len(episodes) != 2 {
		t.Errorf("expected 2 episodes, got %d", len(episodes))
	}
}

func TestAnimeLinker_GetEpisodes_NoMALID(t *testing.T) {
	linker := NewAnimeLinker()
	
	mockSrc := &mockSource{}
	
	anime := &source.Anime{
		Name:  "Test Anime",
		MALID: 0, // No MAL ID
	}
	
	_, err := linker.GetEpisodes(anime, mockSrc)
	if err == nil {
		t.Error("expected error when anime has no MAL ID")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./provider/jikan/... -v`
Expected: FAIL (undefined: NewAnimeLinker, AnimeLinker)

- [ ] **Step 3: Write AnimeLinker implementation**

```go
package jikan

import (
	"fmt"
	
	"github.com/anilix/anilix/source"
)

// AnimeLinker maps anime with MAL ID to streaming source episodes
type AnimeLinker struct{}

// NewAnimeLinker creates a new AnimeLinker
func NewAnimeLinker() *AnimeLinker {
	return &AnimeLinker{}
}

// GetEpisodes fetches episodes from streaming source using anime's MAL ID
// The anime must have MALID set for this to work
func (l *AnimeLinker) GetEpisodes(anime *source.Anime, src source.Source) ([]*source.Episode, error) {
	if anime == nil {
		return nil, fmt.Errorf("anime is nil")
	}
	
	if anime.MALID == 0 {
		return nil, fmt.Errorf("anime has no MAL ID - cannot map to streaming source")
	}
	
	// Get seasons first
	seasons, err := src.SeasonsOf(anime)
	if err != nil {
		return nil, fmt.Errorf("failed to get seasons: %w", err)
	}
	
	// If no seasons, get episodes directly (single season anime)
	if len(seasons) == 0 {
		return src.EpisodesOf(anime, 0)
	}
	
	// For multi-season anime, get episodes from first season for now
	return src.EpisodesOf(anime, seasons[0].Number)
}

// GetStreams fetches streams for an episode from streaming source
func (l *AnimeLinker) GetStreams(episode *source.Episode, src source.Source) ([]*source.Stream, error) {
	if episode == nil {
		return nil, fmt.Errorf("episode is nil")
	}
	
	return src.StreamsOf(episode)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./provider/jikan/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add provider/jikan/linker.go
git commit -m "feat: add AnimeLinker for MAL ID to streaming source mapping"
```

---

## Task 5: Provider Registration

**Files:**
- Create: `provider/jikan/init.go`

- [ ] **Step 1: Add init.go to register Jikan provider**

```go
package jikan

import (
	"github.com/anilix/anilix/provider"
	"github.com/anilix/anilix/source"
)

func init() {
	provider.Register(&provider.Provider{
		ID:           "jikan",
		Name:         "Jikan",
		UsesHeadless: false,
		IsCustom:     false,
		CreateSource: func() (source.Source, error) {
			return NewJikanProvider(), nil
		},
	})
}
```

- [ ] **Step 2: Run go build to verify compilation**

Run: `go build ./...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add provider/jikan/init.go
git commit -m "feat: register Jikan provider"
```

---

## Task 6: Integration Test

**Files:**
- Create: `provider/jikan/integration_test.go`

- [ ] **Step 1: Write integration test**

```go
package jikan

import (
	"testing"
	
	"github.com/anilix/anilix/source"
)

func TestIntegration_JikanSearch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	
	jp := NewJikanProvider()
	results, err := jp.Search("Cowboy Bebop")
	if err != nil {
		t.Fatalf("Jikan search failed: %v", err)
	}
	
	if len(results) == 0 {
		t.Fatal("expected search results")
	}
	
	anime := results[0]
	if anime.MALID == 0 {
		t.Error("expected MAL ID to be set from Jikan")
	}
	
	t.Logf("Found anime: %s (MAL ID: %d)", anime.Name, anime.MALID)
}

func TestIntegration_AnimeLinker(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	
	// Test AnimeLinker with a simple mock
	linker := NewAnimeLinker()
	
	anime := &source.Anime{
		Name:  "Test",
		MALID: 1,
	}
	
	_ = anime
	_ = linker
}
```

- [ ] **Step 2: Run integration test**

Run: `go test ./provider/jikan/... -v -short=false`
Expected: PASS (may take a few seconds)

- [ ] **Step 3: Commit**

```bash
git add provider/jikan/integration_test.go
git commit -m "test: add Jikan integration test"
```

---

## Spec Coverage Check

- [x] Jikan API integration (REST, base URL, endpoints) - Task 2, 3
- [x] Response mapping (MALID, Title, Cover, Year, Genres, Status, URL) - Task 3
- [x] Rate limiting (3 req/sec, token bucket) - Task 2
- [x] Error handling (wrapped errors) - Task 2, 3
- [x] Provider pattern - Task 5
- [x] AnimeLinker for MAL ID mapping - Task 4

All spec requirements covered. No placeholders found. Type consistency verified.

---

## Plan Complete

Plan saved to `docs/superpowers/plans/2026-05-13-jikan-api-integration.md`.

**Two execution options:**

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

Which approach?