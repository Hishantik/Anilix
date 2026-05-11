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

func TestEpisodesOf(t *testing.T) {
	m := &Mock{}

	anime := &source.Anime{Name: "One Piece", Source: m}
	episodes, err := m.EpisodesOf(anime)
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
		if ep.Anime != anime {
			t.Errorf("Episode %d Anime is not set", i)
		}
	}
}

func TestStreamsOf(t *testing.T) {
	m := &Mock{}

	episode := &source.Episode{
		Number: 1,
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