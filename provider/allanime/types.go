package Allanime

import "encoding/json"

// GraphQL request/response types

type GraphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables,omitempty"`
}

type GraphQLResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []GraphQLError  `json:"errors,omitempty"`
}

type GraphQLError struct {
	Message string `json:"message"`
}

// Search response types

type ShowsResponse struct {
	Shows ShowConnection `json:"shows"`
}

type ShowConnection struct {
	Edges []ShowNode `json:"edges"`
}

type ShowNode struct {
	ID                 string              `json:"_id"`
	Name               string              `json:"name"`
	Thumbnail          string              `json:"thumbnail"`
	AvailableEpisodes  AvailableEpisodes  `json:"availableEpisodes"`
	MalID              string              `json:"malId"`
	AniListID          string              `json:"aniListId"`
	Type               string              `json:"type"`
	Season             string              `json:"season"`
}

type AvailableEpisodes struct {
	Sub int `json:"sub"`
	Dub int `json:"dub"`
	Raw int `json:"raw"`
}

// Episode list response

type ShowResponse struct {
	Show ShowDetail `json:"show"`
}

type ShowDetail struct {
	ID                      string                  `json:"_id"`
	Name                    string                  `json:"name"`
	AvailableEpisodesDetail AvailableEpisodesDetail `json:"availableEpisodesDetail"`
	Thumbnail               string                  `json:"thumbnail"`
}

type AvailableEpisodesDetail struct {
	Sub []string `json:"sub"`
	Dub []string `json:"dub"`
}

// Episode source response

type EpisodeResponse struct {
	Episode EpisodeDetail `json:"episode"`
}

type EpisodeDetail struct {
	SourceUrls []SourceUrl `json:"sourceUrls"`
}

type SourceUrl struct {
	SourceName string `json:"sourceName"`
	SourceUrl  string `json:"url"`
	Priority   int    `json:"priority"`
}

// For parsing sourceUrls from AllAnime's encrypted format
type EncryptedSourceUrls struct {
	URL      string `json:"url"`
	Source   string `json:"source"`
	SourceID string `json:"sourceId"`
	Type     string `json:"type"`
}

type any = interface{}