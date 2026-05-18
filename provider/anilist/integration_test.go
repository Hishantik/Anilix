package anilist

import (
	"context"
	"testing"
	"time"
)

func TestIntegration_AniListGetAnime(t *testing.T) {
	client := NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	data, err := client.GetAnime(ctx, 1)
	if err != nil {
		t.Fatalf("GetAnime failed: %v", err)
	}

	if data.ID != 1 {
		t.Errorf("expected ID 1, got %d", data.ID)
	}
	if data.Title.Romaji != "Cowboy Bebop" {
		t.Errorf("expected title 'Cowboy Bebop', got '%s'", data.Title.Romaji)
	}
	if data.Title.English != "Cowboy Bebop" {
		t.Errorf("expected english title 'Cowboy Bebop', got '%s'", data.Title.English)
	}
	if data.Format != "TV" {
		t.Errorf("expected format 'TV', got '%s'", data.Format)
	}
	if data.Episodes != 26 {
		t.Errorf("expected 26 episodes, got %d", data.Episodes)
	}
	if data.AverageScore == 0 {
		t.Error("expected non-zero averageScore")
	}
	if len(data.Genres) == 0 {
		t.Error("expected non-empty genres")
	}
	if data.Description == "" {
		t.Error("expected non-empty description")
	}
}

func TestIntegration_AniListGetAnimeBatch(t *testing.T) {
	client := NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ids := []int{1, 21, 1535}
	results, err := client.GetAnimeBatch(ctx, ids)
	if err != nil {
		t.Fatalf("GetAnimeBatch failed: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}

	if data, ok := results[1]; !ok || data.Title.Romaji != "Cowboy Bebop" {
		t.Errorf("expected Cowboy Bebop for ID 1")
	}
	if data, ok := results[21]; !ok || data.Title.Romaji != "ONE PIECE" {
		t.Errorf("expected ONE PIECE for ID 21")
	}
	if data, ok := results[1535]; !ok || data.Title.Romaji != "DEATH NOTE" {
		t.Errorf("expected DEATH NOTE for ID 1535")
	}
}

func TestIntegration_AniListSearch(t *testing.T) {
	client := NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	results, err := client.SearchAnime(ctx, "death note", 5)
	if err != nil {
		t.Fatalf("SearchAnime failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected at least 1 result")
	}

	found := false
	for _, r := range results {
		if r.ID == 1535 {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Death Note (ID 1535) in search results")
	}
}

func TestIntegration_AniListCache(t *testing.T) {
	client := NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := client.GetAnime(ctx, 1)
	if err != nil {
		t.Fatalf("first GetAnime failed: %v", err)
	}

	if !client.IsCached(1) {
		t.Error("expected ID 1 to be cached")
	}

	cached, err := client.GetAnime(ctx, 1)
	if err != nil {
		t.Fatalf("cached GetAnime failed: %v", err)
	}
	if cached.Title.Romaji != "Cowboy Bebop" {
		t.Errorf("cached data mismatch")
	}
}
