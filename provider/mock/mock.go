package mock

import (
	"fmt"
	"strings"

	"github.com/anilix/anilix/source"
)

const (
	Name = "mock"
	ID   = "mock"
)

type Mock struct{}

func (m *Mock) Name() string { return Name }

func (m *Mock) ID() string { return ID }

func (m *Mock) Search(query string) ([]*source.Anime, error) {
	query = strings.ToLower(strings.TrimSpace(query))

	animes := []*source.Anime{
		{
			Name:   "One Piece",
			URL:    "https://allmanga.to/anime/one-piece",
			Cover:  "https://placehold.co/300x400/1a1a2e/FFF?text=One+Piece",
			Year:   1999,
			Genres: []string{"Action", "Adventure", "Comedy", "Fantasy"},
			Status: "airing",
			Source: m,
		},
		{
			Name:   "Naruto",
			URL:    "https://allmanga.to/anime/naruto",
			Cover:  "https://placehold.co/300x400/ff6b35/FFF?text=Naruto",
			Year:   2002,
			Genres: []string{"Action", "Adventure", "Martial Arts"},
			Status: "finished",
			Source: m,
		},
		{
			Name:   "Attack on Titan",
			URL:    "https://allmanga.to/anime/attack-on-titan",
			Cover:  "https://placehold.co/300x400/8b0000/FFF?text=Attack+on+Titan",
			Year:   2013,
			Genres: []string{"Action", "Drama", "Fantasy", "Horror"},
			Status: "finished",
			Source: m,
		},
	}

	var results []*source.Anime
	for _, anime := range animes {
		if strings.Contains(strings.ToLower(anime.Name), query) {
			results = append(results, anime)
		}
	}

	return results, nil
}

func (m *Mock) SeasonsOf(anime *source.Anime) ([]source.Season, error) {
	switch anime.Name {
	case "One Piece":
		return []source.Season{{Number: 1, Name: "Season 1"}}, nil
	case "Naruto":
		return []source.Season{{Number: 1, Name: "Season 1"}}, nil
	case "Attack on Titan":
		return []source.Season{
			{Number: 1, Name: "Season 1"},
			{Number: 2, Name: "Season 2"},
			{Number: 3, Name: "Season 3"},
			{Number: 4, Name: "Season 4 - Final"},
		}, nil
	default:
		return nil, fmt.Errorf("unknown anime: %s", anime.Name)
	}
}

func (m *Mock) EpisodesOf(anime *source.Anime, season int) ([]*source.Episode, error) {
	switch anime.Name {
	case "One Piece":
		return onePieceEpisodes(anime, season), nil
	case "Naruto":
		return narutoEpisodes(anime, season), nil
	case "Attack on Titan":
		eps, err := attackOnTitanEpisodes(anime, season)
		return eps, err
	default:
		return nil, fmt.Errorf("unknown anime: %s", anime.Name)
	}
}

func (m *Mock) StreamsOf(episode *source.Episode) ([]*source.Stream, error) {
	anime := episode.Anime
	epNum := int(episode.Number)

	animeID := strings.ToLower(strings.ReplaceAll(anime.Name, " ", "-"))
	epURL := fmt.Sprintf("https://allanime.day/episode/%s/s%d/ep%03d", animeID, episode.Season, epNum)

	return []*source.Stream{
		{
			Quality:  "1080p",
			URL:      epURL + "/1080p.m3u8",
			Provider: "allanime",
		},
		{
			Quality:  "720p",
			URL:      epURL + "/720p.m3u8",
			Provider: "allanime",
		},
		{
			Quality:  "480p",
			URL:      epURL + "/480p.m3u8",
			Provider: "allanime",
		},
	}, nil
}

func onePieceEpisodes(anime *source.Anime, season int) []*source.Episode {
	titles := []string{
		"I'm Luffy! The Man Who's Gonna Be King of the Pirates!",
		"The Straw Hats Appear! The Great Mission to Save Romance!",
		"The Great Race! Chef Sola Fights Luffy at Sunset!",
		"The Pirate Zoro! I Want To Be a Great Swordsman!",
		"Nami Appears! The Straw Hats Get Into a Bind!",
	}
	return createEpisodes(anime, season, 5, titles)
}

func narutoEpisodes(anime *source.Anime, season int) []*source.Episode {
	titles := []string{
		"Enter: Naruto Uzumaki!",
		"My Name is Konohamaru!",
		"Sasuke Uchiha!",
		"Lost and Found... The End of the Road!",
	}
	return createEpisodes(anime, season, 4, titles)
}

func attackOnTitanEpisodes(anime *source.Anime, season int) ([]*source.Episode, error) {
	seasonData := map[int]struct {
		count  int
		titles []string
	}{
		1: {12, []string{
			"To You, 2000 Years From Now",
			"That Day",
			"A Dim Light Within the Walls",
			"The Evening of the Deciding Battle",
			"First Move",
			"The World She Knew",
			"Intothe Abyss",
			"Realm of the Algernon",
			"Snake",
			"Adrift",
			"Charge",
			"Seize Freedom",
		}},
		2: {12, []string{
			"I'm Sure",
			"Young",
			"Delicate",
			"Support",
			"Shifting Strategy",
			"Warriors",
			"Closers",
			"The Perfect Duo",
			"Blood-Sea",
			"The Unknown",
			"Wide",
			"Visions",
		}},
		3: {12, []string{
			"Pain",
			"Victory",
			"Battle",
			"Monday",
			"The Reason",
			"Self-Determination",
			"Hero",
			"The Perfect Duo",
			"Blood-Sea",
			"The Unknown",
			"Wide",
			"Visions",
		}},
		4: {16, []string{
			"The Distant Light",
			"Devour",
			"Answer",
			"Hope",
			"Delimeters",
			"Accelerant",
			"Sole Salvation",
			"The Founders",
			"Believing",
			"Fang",
			"Guiding",
			"Steady",
			"Towards",
			"Rumbling",
			"I Saw",
			"Uniting",
		}},
	}

	data, ok := seasonData[season]
	if !ok {
		return nil, fmt.Errorf("unknown season: %d", season)
	}

	return createEpisodes(anime, season, data.count, data.titles), nil
}

func createEpisodes(anime *source.Anime, season, count int, titles []string) []*source.Episode {
	episodes := make([]*source.Episode, count)
	for i := 0; i < count; i++ {
		episodes[i] = &source.Episode{
			Number: float64(i + 1),
			Title:  titles[i],
			URL:    fmt.Sprintf("https://allmanga.to/anime/%s/season/%d/episode/%d", anime.Name, season, i+1),
			Season: season,
			Anime:  anime,
		}
	}
	return episodes
}