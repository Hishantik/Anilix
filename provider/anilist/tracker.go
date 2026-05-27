package anilist

import (
	"context"
	"encoding/json"
	"fmt"
)

// GetViewer returns the authenticated user's username and user ID.
func (c *Client) GetViewer(ctx context.Context) (string, int, error) {
	if c.token == "" {
		return "", 0, fmt.Errorf("not authenticated")
	}

	query := `{ Viewer { id name } }`
	resp, err := c.doGraphQL(ctx, query, nil)
	if err != nil {
		return "", 0, fmt.Errorf("viewer query failed: %w", err)
	}

	var result ViewerResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", 0, fmt.Errorf("failed to decode viewer response: %w", err)
	}

	return result.Viewer.Name, result.Viewer.ID, nil
}

// SaveMediaListEntry creates or updates a list entry for the given anime.
// status should be one of: CURRENT, PLANNING, COMPLETED, DROPPED, PAUSED, REPEATING.
func (c *Client) SaveMediaListEntry(ctx context.Context, mediaID int, status string, progress int) error {
	if c.token == "" {
		return fmt.Errorf("not authenticated")
	}

	c.rateLimiter.waitForToken()

	mutation := `mutation ($mediaId: Int, $status: MediaListStatus, $progress: Int) {
		SaveMediaListEntry(mediaId: $mediaId, status: $status, progress: $progress) {
			id status progress
		}
	}`

	variables := map[string]interface{}{
		"mediaId":  mediaID,
		"status":   status,
		"progress": progress,
	}

	resp, err := c.doGraphQL(ctx, mutation, variables)
	if err != nil {
		return fmt.Errorf("save media list entry failed: %w", err)
	}

	var result SaveMediaListResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return fmt.Errorf("failed to decode save response: %w", err)
	}

	return nil
}

// GetMediaListStatus returns the current list status for the given anime, or nil if not in list.
func (c *Client) GetMediaListStatus(ctx context.Context, mediaID int) (*MediaListEntry, error) {
	if c.token == "" {
		return nil, nil
	}

	c.rateLimiter.waitForToken()

	query := `query ($mediaId: Int) {
		MediaList(mediaId: $mediaId, type: ANIME) {
			id status progress
		}
	}`
	variables := map[string]interface{}{"mediaId": mediaID}

	resp, err := c.doGraphQL(ctx, query, variables)
	if err != nil {
		return nil, fmt.Errorf("media list query failed: %w", err)
	}

	var result MediaListResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to decode list response: %w", err)
	}

	return result.MediaList, nil
}
