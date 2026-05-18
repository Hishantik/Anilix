package Allanime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/anilix/anilix/source"
)

const (
	BaseURL    = "https://api.allanime.day/api"
	Referer    = "https://allmanga.to"
	UserAgent  = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
	RateLimit  = 3 // requests per second
	MaxRetries = 3
)

type AllanimeClient struct {
	http    *http.Client
	baseURL string
}

func NewAllanimeClient() *AllanimeClient {
	return &AllanimeClient{
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: BaseURL,
	}
}

// Search shows via AllAnime GraphQL
func (c *AllanimeClient) SearchShows(ctx context.Context, query string, limit int, page int, translationType string) ([]ShowNode, error) {
	// Use full query matching ani-cli
	searchQuery := `query( $search: SearchInput $limit: Int $page: Int $translationType: VaildTranslationTypeEnumType $countryOrigin: VaildCountryOriginEnumType ) { shows( search: $search limit: $limit page: $page translationType: $translationType countryOrigin: $countryOrigin ) { edges { _id name malId aniListId thumbnail availableEpisodes __typename } }}`

	variables := map[string]interface{}{
		"search": map[string]interface{}{
			"allowAdult":     false,
			"allowUnknown":   false,
			"query":         query,
		},
		"limit":          limit,
		"page":           page,
		"translationType": translationType,
		"countryOrigin":  "ALL",
	}

	resp, err := c.doGraphQL(ctx, searchQuery, variables)
	if err != nil {
		return nil, err
	}

	var result ShowsResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal shows response: %w", err)
	}

	shows := make([]ShowNode, 0, len(result.Shows.Edges))
	for _, edge := range result.Shows.Edges {
		shows = append(shows, edge)
	}
	return shows, nil
}

// Get episode list for a show
func (c *AllanimeClient) GetShowEpisodes(ctx context.Context, showID string, translationType string) (map[string][]string, error) {
	query := `query($showId: String!) { show(_id: $showId) { _id name availableEpisodesDetail } }`

	variables := map[string]interface{}{
		"showId": showID,
	}

	resp, err := c.doGraphQL(ctx, query, variables)
	if err != nil {
		return nil, err
	}

	var result ShowEpisodesRaw
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal show response: %w", err)
	}

	// Parse availableEpisodesDetail which comes as a map
	episodes := make(map[string][]string)
	detail := result.Show.AvailableEpisodesDetail

	if sub, ok := detail["sub"].([]interface{}); ok {
		list := make([]string, len(sub))
		for i, v := range sub {
			list[i] = fmt.Sprintf("%v", v)
		}
		episodes["sub"] = list
	}
	if dub, ok := detail["dub"].([]interface{}); ok {
		list := make([]string, len(dub))
		for i, v := range dub {
			list[i] = fmt.Sprintf("%v", v)
		}
		episodes["dub"] = list
	}

	return episodes, nil
}

type ShowEpisodesRaw struct {
	Show struct {
		ID                      string                 `json:"_id"`
		Name                    string                 `json:"name"`
		AvailableEpisodesDetail  map[string]interface{} `json:"availableEpisodesDetail"`
	} `json:"show"`
}

// Get episode sources (provider references)
func (c *AllanimeClient) GetEpisodeSources(ctx context.Context, showID, episodeString, translationType string) ([]SourceUrl, error) {
	variables := map[string]interface{}{
		"showId":          showID,
		"episodeString":   episodeString,
		"translationType": translationType,
	}

	resp, err := c.doGraphQL(ctx, queryEpisodeSources, variables)
	if err != nil {
		return nil, err
	}

	// Try to parse response as either encrypted (tobeparsed) or direct sourceUrls
	// Persisted query returns: {"_m":"b7","tobeparsed":"..."}
	// Regular GraphQL returns: {"data":{"episode":{"sourceUrls":[...]}}}
	var rawResp map[string]interface{}
	if err := json.Unmarshal(resp, &rawResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check for persisted query format - direct _m and tobeparsed (no data wrapper)
	if toBeParsed, ok := rawResp["tobeparsed"].(string); ok && toBeParsed != "" {
		return decodeToBeParsed(toBeParsed)
	}

	// Check for GraphQL wrapper format
	if data, ok := rawResp["data"].(map[string]interface{}); ok {
		// Try lowercase 'tobeparsed'
		if toBeParsed, ok := data["tobeparsed"].(string); ok && toBeParsed != "" {
			return decodeToBeParsed(toBeParsed)
		}
		// Check for regular episode format with sourceUrls
		if episode, ok := data["episode"].(map[string]interface{}); ok {
			if sourceUrls, ok := episode["sourceUrls"].([]interface{}); ok {
				return parseSourceUrlsFromInterface(sourceUrls), nil
			}
		}
	}

	// Fallback: try direct EpisodeResponse parsing
	var result EpisodeResponse
	if err := json.Unmarshal(resp, &result); err == nil {
		return result.Episode.SourceUrls, nil
	}

	return nil, fmt.Errorf("no valid source URLs found in response")
}

func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// parseSourceUrlsFromInterface converts interface{} slice to SourceUrl slice
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
			if priority, ok := m["priority"].(float64); ok {
				src.Priority = int(priority)
			}
			result = append(result, src)
		}
	}
	return result
}

func (c *AllanimeClient) doGraphQL(ctx context.Context, query string, variables map[string]interface{}) ([]byte, error) {
	reqBody, err := json.Marshal(GraphQLRequest{
		Query:     query,
		Variables: variables,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Try curl directly since Go HTTP client gets blocked by Cloudflare
	return c.doGraphQLCurl(ctx, reqBody)
}

func (c *AllanimeClient) doGraphQLHTTP(ctx context.Context, reqBody []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Referer", "https://allmanga.to/")
	req.Header.Set("Origin", "https://allmanga.to")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("non-200 status: %d, body: %s", resp.StatusCode, string(body))
	}

	var result GraphQLResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("GraphQL error: %s", result.Errors[0].Message)
	}

	return result.Data, nil
}

func (c *AllanimeClient) doGraphQLCurl(ctx context.Context, reqBody []byte) ([]byte, error) {
	// Use Firefox user-agent like ani-cli
	agent := "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:109.0) Gecko/20100101 Firefox/121.0"

	// Try persisted query first (ani-cli approach) - ONLY for episode sources
	persistedResp, err := c.doGraphQLCurlPersisted(ctx, reqBody, agent)
	if err == nil && len(persistedResp) > 0 {
		var persisted GraphQLResponse
		if json.Unmarshal(persistedResp, &persisted) == nil && len(persisted.Errors) == 0 {
			return persisted.Data, nil
		}
	}
	// Persisted failed or not applicable - fall through to regular POST

	// Fallback to regular GraphQL POST
	cmd := exec.CommandContext(ctx, "curl", "-s", "-X", "POST", c.baseURL,
		"-H", "Content-Type: application/json",
		"-H", "User-Agent: "+agent,
		"-H", "Referer: https://allmanga.to",
		"-H", "Origin: https://allmanga.to",
		"-d", "@-")

	cmd.Stdin = bytes.NewReader(reqBody)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("curl failed: %w", err)
	}

	var result GraphQLResponse
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse curl response: %w", err)
	}

	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("GraphQL error: %s", result.Errors[0].Message)
	}

	return result.Data, nil
}

// doGraphQLCurlPersisted uses persisted queries like ani-cli
func (c *AllanimeClient) doGraphQLCurlPersisted(ctx context.Context, reqBody []byte, agent string) ([]byte, error) {
	// Parse the request to extract variables
	var req struct {
		Variables map[string]interface{} `json:"variables"`
		Query     string                 `json:"query"`
	}
	if err := json.Unmarshal(reqBody, &req); err != nil {
		return nil, fmt.Errorf("failed to parse request: %w", err)
	}

	// Only use persisted query for episode sources
	if req.Query == "" || req.Variables == nil {
		return nil, fmt.Errorf("no query or variables")
	}

	// Check if this is an episode query (only persisted queries work for episode sources)
	if _, hasShowId := req.Variables["showId"]; !hasShowId {
		return nil, fmt.Errorf("not an episode query")
	}

	// Key: Only use persisted query for episode sources (requires episodeString)
	if _, hasEpisodeString := req.Variables["episodeString"]; !hasEpisodeString {
		return nil, fmt.Errorf("not an episode query (no episodeString)")
	}

	// Persisted query hash for episode (from ani-cli)
	queryHash := "d405d0edd690624b66baba3068e0edc3ac90f1597d898a1ec8db4e5c43c00fec"

	// Build URL with persisted query
	showId := req.Variables["showId"].(string)
	translationType := "sub"
	if tt, ok := req.Variables["translationType"].(string); ok {
		translationType = tt
	}
	episodeString := "1"
	if ep, ok := req.Variables["episodeString"].(string); ok {
		episodeString = ep
	}

	encodedVars := fmt.Sprintf(`{"showId":"%s","translationType":"%s","episodeString":"%s"}`,
		showId, translationType, episodeString)
	encodedVars = strings.ReplaceAll(encodedVars, `"`, "%22")
	encodedVars = strings.ReplaceAll(encodedVars, `:`, "%3A")
	encodedVars = strings.ReplaceAll(encodedVars, `{`, "%7B")
	encodedVars = strings.ReplaceAll(encodedVars, `}`, "%7D")
	encodedVars = strings.ReplaceAll(encodedVars, `,`, "%2C")

	encodedExt := `{"persistedQuery":{"version":1,"sha256Hash":"` + queryHash + `"}}`
	encodedExt = strings.ReplaceAll(encodedExt, `"`, "%22")
	encodedExt = strings.ReplaceAll(encodedExt, `:`, "%3A")
	encodedExt = strings.ReplaceAll(encodedExt, `{`, "%7B")
	encodedExt = strings.ReplaceAll(encodedExt, `}`, "%7D")
	encodedExt = strings.ReplaceAll(encodedExt, `,`, "%2C")
	encodedExt = strings.ReplaceAll(encodedExt, ` `, "%20")

	apiURL := c.baseURL + "?variables=" + encodedVars + "&extensions=" + encodedExt

	// Try with different referer (ani-cli uses youtu-chan.com first)
	referers := []string{
		"https://youtu-chan.com",
		"https://allmanga.to",
	}

	for _, referer := range referers {
		cmd := exec.CommandContext(ctx, "curl", "-s",
			"-H", "User-Agent: "+agent,
			"-H", "Referer: "+referer,
			"-H", "Origin: https://youtu-chan.com",
			apiURL)

		output, err := cmd.Output()
		if err != nil {
			continue
		}

		// Check if we got a valid response
		var check map[string]interface{}
		if json.Unmarshal(output, &check) == nil {
			if data, ok := check["data"].(map[string]interface{}); ok && data != nil {
				return output, nil
			}
			// Check for CAPTCHA
			if errs, ok := check["errors"].([]interface{}); ok && len(errs) > 0 {
				for _, e := range errs {
					if errMap, ok := e.(map[string]interface{}); ok {
						if msg, ok := errMap["message"].(string); ok {
							if strings.Contains(msg, "CAPTCHA") {
								continue // try next referer
							}
						}
					}
				}
			}
		}
	}

	return nil, fmt.Errorf("persisted query failed")
}

// Map AllAnime show to source.Anime
func (c *AllanimeClient) MapToAnime(show *ShowNode) *source.Anime {
	anime := &source.Anime{
		Name:         show.Name,
		AllAnimeID:   show.ID,
		URL:          fmt.Sprintf("https://allmanga.to/show/%s", show.ID),
		EpisodeCount: show.AvailableEpisodes.Sub,
	}

	// Parse malId from string to int
	if show.MalID != "" {
		if malID, err := strconv.Atoi(show.MalID); err == nil {
			anime.MALID = malID
		}
	}

	// Parse aniListId from string to int
	if show.AniListID != "" {
		if aniListID, err := strconv.Atoi(show.AniListID); err == nil {
			anime.AniListID = aniListID
		}
	}

	// Handle thumbnail - AllAnime returns various formats
	if show.Thumbnail != "" {
		// Extract the actual URL from the thumbnail field
		anime.Cover = show.Thumbnail
	}

	return anime
}
