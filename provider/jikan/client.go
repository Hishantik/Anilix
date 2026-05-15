package jikan

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type JikanClient struct {
	baseURL     string
	httpClient  *http.Client
	rateLimiter *rateLimiter
	cache       map[int]*AnimeData
	cacheMu     sync.RWMutex
}

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

func (r *rateLimiter) waitForToken() {
	for !r.acquire() {
		time.Sleep(100 * time.Millisecond)
	}
}

func NewClient(baseURL string) *JikanClient {
	return &JikanClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		rateLimiter: newRateLimiter(3, time.Second),
		cache:       make(map[int]*AnimeData),
	}
}

func (c *JikanClient) SearchAnime(ctx context.Context, query string) (*AnimeResponse, error) {
	c.rateLimiter.waitForToken()

	url := fmt.Sprintf("%s/anime?q=%s&limit=20", c.baseURL, url.QueryEscape(query))

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

// GetAnime fetches full anime details by MAL ID (with caching)
func (c *JikanClient) GetAnime(ctx context.Context, malID int) (*AnimeData, error) {
	// Check cache first
	c.cacheMu.RLock()
	if cached, ok := c.cache[malID]; ok {
		c.cacheMu.RUnlock()
		return cached, nil
	}
	c.cacheMu.RUnlock()

	c.rateLimiter.waitForToken()

	url := fmt.Sprintf("%s/anime/%d", c.baseURL, malID)

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

	var result AnimeSingleResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Cache the result
	c.cacheMu.Lock()
	c.cache[malID] = &result.Data
	c.cacheMu.Unlock()

	return &result.Data, nil
}

// GetEpisodes fetches episode list for an anime by MAL ID
func (c *JikanClient) GetEpisodes(ctx context.Context, malID int) ([]Episode, error) {
	c.rateLimiter.waitForToken()

	url := fmt.Sprintf("%s/anime/%d/episodes", c.baseURL, malID)

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

	var result EpisodesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Data, nil
}