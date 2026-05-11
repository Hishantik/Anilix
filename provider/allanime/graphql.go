package allanime

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/anilix/anilix/source"
)

const (
	APIBaseURL = "https://api.allanime.day/api"
	RefererURL = "https://allmanga.to"
	APIKey     = "Xot36i3lK3:v1"
)

var httpClient = &http.Client{}

func searchAnime(query string) (*SearchResponse, error) {
	body := map[string]interface{}{
		"query": `query( $search: SearchInput $limit: Int $page: Int $translationType: VaildTranslationTypeEnumType $countryOrigin: VaildCountryOriginEnumType ) { shows( search: $search limit: $limit page: $page translationType: $translationType countryOrigin: $countryOrigin ) { edges { _id name availableEpisodes thumbnail __typename } }}`,
		"variables": map[string]interface{}{
			"search": map[string]interface{}{
				"allowAdult":    false,
				"allowUnknown":  false,
				"query":         query,
			},
			"limit":           20,
			"page":            1,
			"translationType": "sub",
			"countryOrigin":   "ALL",
		},
	}

	resp, err := postGraphQL(body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func getShow(showID string) (*ShowResponse, error) {
	body := map[string]interface{}{
		"query":     `query ($showId: String!) { show( _id: $showId ) { _id name thumbnail availableEpisodesDetail availableEpisodes }}`,
		"variables": map[string]interface{}{"showId": showID},
	}

	resp, err := postGraphQL(body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result ShowResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func getEpisodeStreams(showID, episodeString string) (*EpisodeResponse, error) {
	body := map[string]interface{}{
		"query": `query ($showId: String!, $translationType: VaildTranslationTypeEnumType!, $episodeString: String!) { episode( showId: $showId translationType: $translationType episodeString: $episodeString ) { episodeString sourceUrls }}`,
		"variables": map[string]interface{}{
			"showId":          showID,
			"translationType": "sub",
			"episodeString":   episodeString,
		},
	}

	resp, err := postGraphQL(body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result EpisodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func postGraphQL(body map[string]interface{}) (*http.Response, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", APIBaseURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Referer", RefererURL)
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; anilix/1.0)")

	return httpClient.Do(req)
}

// Response types

type SearchResponse struct {
	Data struct {
		Shows struct {
			Edges []ShowEdge `json:"edges"`
		} `json:"shows"`
	} `json:"data"`
}

type ShowEdge struct {
	ID                string `json:"_id"`
	Name              string `json:"name"`
	AvailableEpisodes int    `json:"availableEpisodes"`
	Thumbnail         string `json:"thumbnail"`
	Type              string `json:"__typename"`
}

type ShowResponse struct {
	Data struct {
		Show Show `json:"show"`
	} `json:"data"`
}

type Show struct {
	ID                      string                 `json:"_id"`
	Name                    string                 `json:"name"`
	Thumbnail               string                 `json:"thumbnail"`
	AvailableEpisodesDetail map[string][]any       `json:"availableEpisodesDetail"`
	AvailableEpisodes       map[string]int         `json:"availableEpisodes"`
}

type EpisodeResponse struct {
	Data struct {
		Episode Episode `json:"episode"`
	} `json:"data"`
}

type Episode struct {
	EpisodeString string      `json:"episodeString"`
	SourceUrls    []SourceUrl `json:"sourceUrls"`
}

type SourceUrl struct {
	SourceName string `json:"sourceName"`
	SourceUrl  string `json:"sourceUrl"`
	Type       string `json:"type"`
}

// Decode encrypted source URLs (tobeparsed)
func decodeTobeparsed(encoded string) ([]SourceUrl, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("base64 decode failed: %w", err)
	}

	plaintext, err := decryptAES(data)
	if err != nil {
		return nil, err
	}

	var urls []SourceUrl
	if err := json.Unmarshal(plaintext, &urls); err != nil {
		return nil, fmt.Errorf("json parse failed: %w", err)
	}

	return urls, nil
}

// DecryptAES performs AES-256-CTR decryption
func decryptAES(data []byte) ([]byte, error) {
	key := sha256Hash(APIKey)

	if len(data) < 29 { // 1 byte flag + 12 byte IV + 16 byte tag minimum
		return nil, fmt.Errorf("data too short: need at least 29 bytes, got %d", len(data))
	}

	// IV is at offset 1, 12 bytes
	iv := data[1:13]
	ctr := make([]byte, 16)
	copy(ctr, iv)
	ctr[15] = 2 // Counter byte

	// Ciphertext starts at offset 13, ends before the 16-byte tag
	ciphertext := data[13 : len(data)-16]

	return aesCTRDecrypt(key, ctr, ciphertext)
}

func sha256Hash(input string) []byte {
	h := sha256.Sum256([]byte(input))
	return h[:]
}

func aesCTRDecrypt(key, ctr, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	stream := cipher.NewCTR(block, ctr)
	plaintext := make([]byte, len(ciphertext))
	stream.XORKeyStream(plaintext, ciphertext)

	return plaintext, nil
}

// ExtractStreamUrls extracts m3u8 URLs from SourceUrls
func ExtractStreamUrls(urls []SourceUrl) []source.Stream {
	var streams []source.Stream

	for _, url := range urls {
		if strings.Contains(url.SourceUrl, "m3u8") || strings.Contains(url.SourceUrl, "mp4") {
			streams = append(streams, source.Stream{
				Quality:  "best",
				URL:      cleanUrl(url.SourceUrl),
				Provider: url.SourceName,
			})
		}
	}

	return streams
}

func cleanUrl(url string) string {
	url = strings.ReplaceAll(url, "\\/", "/")
	url = strings.ReplaceAll(url, "\\", "")
	return strings.TrimSpace(url)
}

// ReadAll reads entire body
func readBody(resp *http.Response) ([]byte, error) {
	return io.ReadAll(resp.Body)
}