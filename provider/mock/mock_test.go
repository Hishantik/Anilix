package mock

import (
	"strings"
	"testing"

	"github.com/anilix/anilix/source"
)

func TestMockName(t *testing.T) {
	m := &Mock{}
	if m.Name() != Name {
		t.Errorf("Name() = %v, want %v", m.Name(), Name)
	}
}

func TestMockID(t *testing.T) {
	m := &Mock{}
	if m.ID() != ID {
		t.Errorf("ID() = %v, want %v", m.ID(), ID)
	}
}

func TestSearch(t *testing.T) {
	m := &Mock{}

	tests := []struct {
		name     string
		query    string
		wantLen  int
		wantName string
	}{
		{"exact match", "one piece", 1, "One Piece"},
		{"partial match", "naruto", 1, "Naruto"},
		{"no match", "xyz", 0, ""},
		{"case insensitive", "ATTACK ON TITAN", 1, "Attack on Titan"},
		{"substring", "one", 1, "One Piece"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := m.Search(tt.query)
			if err != nil {
				t.Fatalf("Search() error = %v", err)
			}
			if len(results) != tt.wantLen {
				t.Errorf("Search() returned %d results, want %d", len(results), tt.wantLen)
			}
			if tt.wantLen > 0 && results[0].Name != tt.wantName {
				t.Errorf("Search() first result = %v, want %v", results[0].Name, tt.wantName)
			}
		})
	}
}

func TestSeasonsOf(t *testing.T) {
	m := &Mock{}

	// Attack on Titan has multiple seasons
	anime := &source.Anime{Name: "Attack on Titan", Source: m}
	seasons, err := m.SeasonsOf(anime)
	if err != nil {
		t.Fatalf("SeasonsOf() error = %v", err)
	}

	if len(seasons) != 4 {
		t.Errorf("SeasonsOf() returned %d seasons, want 4", len(seasons))
	}

	// Check season numbers
	for i, s := range seasons {
		if s.Number != i+1 {
			t.Errorf("Season %d number = %v, want %v", i, s.Number, i+1)
		}
	}

	// One Piece has single season
	anime2 := &source.Anime{Name: "One Piece", Source: m}
	seasons2, err := m.SeasonsOf(anime2)
	if err != nil {
		t.Fatalf("SeasonsOf() error = %v", err)
	}
	if len(seasons2) != 1 {
		t.Errorf("SeasonsOf() for One Piece = %d seasons, want 1", len(seasons2))
	}
}

func TestEpisodesOf(t *testing.T) {
	m := &Mock{}

	anime := &source.Anime{Name: "One Piece", Source: m}
	episodes, err := m.EpisodesOf(anime, 1)
	if err != nil {
		t.Fatalf("EpisodesOf() error = %v", err)
	}

	if len(episodes) != 5 {
		t.Errorf("EpisodesOf() returned %d episodes, want 5", len(episodes))
	}

	for i, ep := range episodes {
		if ep.Number != float64(i+1) {
			t.Errorf("Episode %d number = %v, want %v", i, ep.Number, float64(i+1))
		}
		if ep.Season != 1 {
			t.Errorf("Episode %d season = %v, want 1", i, ep.Season)
		}
		if ep.Anime != anime {
			t.Errorf("Episode %d Anime is not set", i)
		}
	}
}

func TestEpisodesOfMultiSeason(t *testing.T) {
	m := &Mock{}

	anime := &source.Anime{Name: "Attack on Titan", Source: m}

	// Season 1
	ep1, err := m.EpisodesOf(anime, 1)
	if err != nil {
		t.Fatalf("EpisodesOf() error = %v", err)
	}
	if len(ep1) != 12 {
		t.Errorf("Season 1 has %d episodes, want 12", len(ep1))
	}

	// Season 4
	ep4, err := m.EpisodesOf(anime, 4)
	if err != nil {
		t.Fatalf("EpisodesOf() error = %v", err)
	}
	if len(ep4) != 16 {
		t.Errorf("Season 4 has %d episodes, want 16", len(ep4))
	}
}

func TestStreamsOf(t *testing.T) {
	m := &Mock{}

	episode := &source.Episode{
		Number: 1,
		Season: 1,
		Anime:  &source.Anime{Name: "One Piece", Source: m},
	}

	streams, err := m.StreamsOf(episode)
	if err != nil {
		t.Fatalf("StreamsOf() error = %v", err)
	}

	if len(streams) != 3 {
		t.Errorf("StreamsOf() returned %d streams, want 3", len(streams))
	}

	qualities := []string{"1080p", "720p", "480p"}
	for i, s := range streams {
		if s.Quality != qualities[i] {
			t.Errorf("Stream %d quality = %v, want %v", i, s.Quality, qualities[i])
		}
		if !strings.Contains(s.URL, ".m3u8") {
			t.Errorf("Stream %d URL does not contain .m3u8: %v", i, s.URL)
		}
	}
}