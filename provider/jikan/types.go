package jikan

// AnimeResponse represents the Jikan API search response
type AnimeResponse struct {
	Data []AnimeData `json:"data"`
}

// AnimeData represents a single anime in Jikan response
type AnimeData struct {
	MalID            int         `json:"mal_id"`
	Title            string      `json:"title"`
	TitleEnglish     string      `json:"title_english"`
	TitleJapanese    string      `json:"title_japanese"`
	URL              string      `json:"url"`
	Images           Images      `json:"images"`
	Year             interface{} `json:"year"` // can be null or int
	Genres           []Genre     `json:"genres"`
	Status           string      `json:"status"`
	Synopsis         string      `json:"synopsis"`
}

// Images contains image URLs
type Images struct {
	JPG JPGImages `json:"jpg"`
}

// JPGImages contains specific image URLs
type JPGImages struct {
	ImageURL      string `json:"image_url"`
	SmallImageURL string `json:"small_image_url"`
	LargeImageURL string `json:"large_image_url"`
}

// Genre represents an anime genre
type Genre struct {
	MalID int    `json:"mal_id"`
	Name  string `json:"name"`
}