package tui

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func recentSearchesPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "~"
	}
	return filepath.Join(home, ".anilix", "recent.json")
}

func loadRecentSearches() []string {
	path := recentSearchesPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var searches []string
	if err := json.Unmarshal(data, &searches); err != nil {
		return nil
	}
	return searches
}

func saveRecentSearch(query string) {
	query = strings.TrimSpace(query)
	if query == "" {
		return
	}

	searches := loadRecentSearches()

	// Deduplicate (case-insensitive)
	queryLower := strings.ToLower(query)
	filtered := make([]string, 0, len(searches))
	for _, s := range searches {
		if strings.ToLower(s) != queryLower {
			filtered = append(filtered, s)
		}
	}

	// Prepend new query, cap at 10
	searches = append([]string{query}, filtered...)
	if len(searches) > 10 {
		searches = searches[:10]
	}

	path := recentSearchesPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("[anilix] failed to create config dir: %v\n", err)
		return
	}

	data, err := json.MarshalIndent(searches, "", "  ")
	if err != nil {
		log.Printf("[anilix] failed to marshal recent searches: %v\n", err)
		return
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		log.Printf("[anilix] failed to save recent searches: %v\n", err)
	}
}
