package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	allanime "github.com/anilix/anilix/provider/allanime"
	"github.com/anilix/anilix/provider/jikan"
	"github.com/anilix/anilix/source"
	"github.com/anilix/anilix/player"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SearchModel is the main model for anime search TUI
type SearchModel struct {
	// Search state
	searchState *SearchState
	episodeState *EpisodeState

	// UI components
	textInput   textinput.Model
	viewport    viewport.Model

	// API clients
	allanimeClient *allanime.AllanimeClient
	jikanClient    *jikan.JikanClient

	// Mode: "search" or "episodes"
	mode string

	// Result
	selectedResult *SelectionResult

	// Debounce
	lastQuery string

	// Window dimensions
	width  int
	height int
}

// NewSearchModel creates a new search model
func NewSearchModel() *SearchModel {
	ti := textinput.New()
	ti.Placeholder = "Search anime..."
	ti.Focus()
	ti.Prompt = "> "

	vp := viewport.New(60, 20)

	return &SearchModel{
		searchState:    NewSearchState(),
		episodeState:   NewEpisodeState(),
		textInput:      ti,
		viewport:       vp,
		allanimeClient: allanime.NewAllanimeClient(),
		jikanClient:    jikan.NewClient("https://api.jikan.moe/v4"),
		mode:           "search",
		lastQuery:      "",
	}
}

// Init initializes the model
func (m *SearchModel) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (m *SearchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle navigation keys first (before textinput)
		switch msg.String() {
		case "ctrl+c", "esc":
			if m.mode == "episodes" {
				// Go back to search mode
				m.mode = "search"
				m.textInput.Placeholder = "Search anime..."
				m.episodeState = NewEpisodeState()
				m.textInput.Focus() // Re-focus text input for search
				m.updateViewport()
				cmds = append(cmds, func() tea.Msg { return tea.WindowSizeMsg{Width: m.width, Height: m.height} })
				return m, tea.Batch(cmds...)
			}
			return m, tea.Quit

		case "up", "k":
			if m.mode == "episodes" {
				if len(m.episodeState.Episodes) > 0 && m.episodeState.Selected > 0 {
					m.episodeState.Selected--
					m.updateEpisodesViewport()
					cmds = append(cmds, func() tea.Msg { return tea.WindowSizeMsg{Width: m.width, Height: m.height} })
				}
			} else {
				if len(m.searchState.Results) > 0 && m.searchState.Selected > 0 {
					m.searchState.Selected--
					cmds = append(cmds, m.fetchMetadata())
					m.updateViewport()
				}
			}
			return m, tea.Batch(cmds...)

		case "down", "j":
			if m.mode == "episodes" {
				if m.episodeState.Selected < len(m.episodeState.Episodes)-1 {
					m.episodeState.Selected++
					m.updateEpisodesViewport()
					cmds = append(cmds, func() tea.Msg { return tea.WindowSizeMsg{Width: m.width, Height: m.height} })
				}
			} else {
				if m.searchState.Selected < len(m.searchState.Results)-1 {
					m.searchState.Selected++
					cmds = append(cmds, m.fetchMetadata())
					m.updateViewport()
				}
			}
			return m, tea.Batch(cmds...)

		case "enter":
			fmt.Printf("DEBUG: Enter pressed, mode=%s\n", m.mode)
			if m.mode == "search" {
				// Switch to episode mode
				if len(m.searchState.Results) > 0 && m.searchState.Selected < len(m.searchState.Results) {
					anime := m.searchState.Results[m.searchState.Selected]
					if anime.AllAnimeID != "" {
						m.mode = "episodes"
						m.episodeState.AnimeID = anime.AllAnimeID
						m.textInput.Placeholder = "Select episode..."
						m.textInput.Reset()
						m.textInput.Blur() // Lose focus so Enter key isn't captured
						cmds = append(cmds, m.fetchEpisodes(anime.AllAnimeID, anime.MALID))
						return m, tea.Batch(cmds...)
					}
				}
			} else if m.mode == "episodes" {
			// Play the selected episode
			if len(m.episodeState.Episodes) > 0 && m.episodeState.Selected < len(m.episodeState.Episodes) {
				selectedAnime := m.searchState.Results[m.searchState.Selected]
				selectedEpisode := m.episodeState.Episodes[m.episodeState.Selected]

				// Fetch sources and play
				cmds = append(cmds, m.playEpisode(selectedAnime.AllAnimeID, selectedEpisode, selectedAnime.Name))
				return m, tea.Batch(cmds...)
			}
			return m, nil
		}
			return m, nil
		}

		// Pass other keys (characters) to textinput
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		cmds = append(cmds, cmd)

		// Trigger search if query changed
		query := m.textInput.Value()
		if query != m.lastQuery && len(query) >= 2 {
			m.lastQuery = query
			m.searchState.Query = query
			cmds = append(cmds, m.doSearch(query))
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateViewport()

	case SearchResultsMsg:
		m.searchState.Results = msg.Results
		m.searchState.Loading = false
		m.searchState.Selected = 0
		if len(msg.Results) > 0 {
			cmds = append(cmds, m.fetchMetadata())
		}
		m.updateViewport()

	case MetadataLoadedMsg:
		m.searchState.Metadata = msg.Metadata
		m.searchState.MetadataLoading = false
		// Force redraw
		cmds = append(cmds, func() tea.Msg { return tea.WindowSizeMsg{Width: m.width, Height: m.height} })

	case EpisodesLoadedMsg:
		m.episodeState.Episodes = msg.Episodes
		m.episodeState.EpisodeTitles = msg.EpisodeTitles
		m.episodeState.Loading = false
		if msg.Error != nil {
			m.episodeState.Err = msg.Error
		}
		m.updateEpisodesViewport()

	case TUIErrorMsg:
		// Show error in right panel
		m.episodeState.Err = msg.Err

	case textinput.Model:
		// This is handled in tea.KeyMsg case above
	}
	m.viewport, _ = m.viewport.Update(msg)

	return m, tea.Batch(cmds...)
}

// doSearch performs the AllAnime search
func (m *SearchModel) doSearch(query string) tea.Cmd {
	return func() tea.Msg {
		m.searchState.Loading = true

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		shows, err := m.allanimeClient.SearchShows(ctx, query, 20, 1, "sub")
		if err != nil {
			return SearchErrorMsg{Err: err}
		}

		results := make([]*source.Anime, 0, len(shows))
		for _, show := range shows {
			anime := m.allanimeClient.MapToAnime(&show)
			results = append(results, anime)
		}

		return SearchResultsMsg{Results: results}
	}
}

// fetchMetadata fetches Jikan metadata for the selected anime
func (m *SearchModel) fetchMetadata() tea.Cmd {
	if m.searchState.Selected >= len(m.searchState.Results) {
		m.searchState.Metadata = nil
		m.searchState.MetadataLoading = false
		return nil
	}

	anime := m.searchState.Results[m.searchState.Selected]

	if anime.MALID == 0 {
		m.searchState.Metadata = nil
		m.searchState.MetadataLoading = false
		return nil // No MAL ID, skip metadata
	}

	m.searchState.MetadataLoading = true

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		data, err := m.jikanClient.GetAnime(ctx, anime.MALID)
		if err != nil {
			// On error, still set loading to false
			return MetadataLoadedMsg{Metadata: nil}
		}

		genres := make([]string, 0, len(data.Genres))
		for _, g := range data.Genres {
			genres = append(genres, g.Name)
		}

		year := 0
		if y, ok := data.Year.(float64); ok {
			year = int(y)
		}

		metadata := &MetadataPanel{
			Title:        data.Title,
			TitleEnglish: data.TitleEnglish,
			Cover:        data.Images.JPG.LargeImageURL,
			Year:         year,
			Type:         data.Type,
			Status:       data.Status,
			Episodes:     data.Episodes,
			Score:        data.Score,
			Rank:         data.Rank,
			Popularity:   data.Popularity,
			Genres:       genres,
			Synopsis:     truncateSynopsis(data.Synopsis, 500),
		}

		return MetadataLoadedMsg{Metadata: metadata}
	}
}

// fetchEpisodes fetches episode list for the selected anime
func (m *SearchModel) fetchEpisodes(showID string, malID int) tea.Cmd {
	m.episodeState.Loading = true

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Get episodes from AllAnime
		episodes, err := m.allanimeClient.GetShowEpisodes(ctx, showID, "sub")
		if err != nil {
			m.episodeState.Err = err
			m.episodeState.Loading = false
			return EpisodesLoadedMsg{Episodes: nil, EpisodeTitles: nil, Error: err}
		}

		// Get sub episodes
		subEpisodes := episodes["sub"]
		m.episodeState.Episodes = subEpisodes
		m.episodeState.Selected = 0

		// Also fetch episode titles from Jikan if we have MAL ID
		var episodeTitles []string
		if malID > 0 {
			jikanEpisodes, err := m.jikanClient.GetEpisodes(ctx, malID)
			if err == nil && len(jikanEpisodes) > 0 {
				episodeTitles = make([]string, len(jikanEpisodes))
				for i, ep := range jikanEpisodes {
					episodeTitles[i] = ep.Title
				}
			}
		}
		m.episodeState.EpisodeTitles = episodeTitles
		m.episodeState.Loading = false

		return EpisodesLoadedMsg{Episodes: subEpisodes, EpisodeTitles: episodeTitles, Error: nil}
	}
}


// playEpisode fetches sources and plays the selected episode
func (m *SearchModel) playEpisode(showID, episodeNum, animeTitle string) tea.Cmd {
	return func() tea.Msg {
		// Use AllanimeProvider which handles decoding and extraction properly
		allanimeProvider := allanime.NewAllanimeProvider()

		// Create a mock episode with the anime info
		episodeNumFloat, _ := strconv.ParseFloat(episodeNum, 64)
		episode := &source.Episode{
			Number: episodeNumFloat,
			Anime: &source.Anime{
				AllAnimeID: showID,
				Name:       animeTitle,
			},
		}

		// Get streams using the provider (handles decoding, extraction properly)
		streams, err := allanimeProvider.StreamsOf(episode)
		if err != nil {
			return TUIErrorMsg{Err: fmt.Errorf("failed to get streams: %w", err)}
		}

		if len(streams) == 0 {
			return TUIErrorMsg{Err: fmt.Errorf("no streams found")}
		}

		// Try each stream until one works (like ani-cli does)
		playStream := tryPlayStream(streams, animeTitle, episodeNum)
		if playStream == nil {
			return TUIErrorMsg{Err: fmt.Errorf("no playable stream found")}
		}

		return nil
	}
}

// tryPlayStream tries each stream in order until one plays successfully
func tryPlayStream(streams []*source.Stream, animeTitle, episodeNum string) *source.Stream {
	p := player.Mpv
	opts := player.Options{
		Title: fmt.Sprintf("%s - Episode %s", animeTitle, episodeNum),
	}

	// Sort streams by priority (like ani-cli does - wixmp, hianime, filemoon, others)
	sorted := sortStreamsByPriority(streams)

	for _, s := range sorted {
		// Fix relative URLs
		url := s.URL
		if strings.HasPrefix(url, "//") {
			url = "https:" + url
		}

		opts.Referrer = s.Referer

		// Try to launch
		if err := p.Launch(url, opts); err == nil {
			return s
		}
	}

	return nil
}

// sortStreamsByPriority sorts streams similar to ani-cli: wixmp > hianime > filemoon > others
func sortStreamsByPriority(streams []*source.Stream) []*source.Stream {
	priority := map[string]int{
		"wixmp":        1,
		"hianime":      2,
		"filemoon":     3,
		"vidstreaming": 4,
		"vid-mp4":      4,
		"mp4upload":    5,
		"streamsb":     6,
		"ok":           7,
		"streamwish":   8,
		"default":      9,
	}

	// Sort by priority
	sorted := make([]*source.Stream, len(streams))
	copy(sorted, streams)

	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			pI := priority[sorted[i].Provider]
			if pI == 0 {
				pI = 10
			}
			pJ := priority[sorted[j].Provider]
			if pJ == 0 {
				pJ = 10
			}
			if pI > pJ {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	return sorted
}


// updateViewport updates the viewport content
func (m *SearchModel) updateViewport() {
	if m.mode == "episodes" {
		m.updateEpisodesViewport()
		return
	}

	if len(m.searchState.Results) == 0 {
		m.viewport.SetContent("Start typing to search anime...")
		return
	}

	var b strings.Builder
	for i, anime := range m.searchState.Results {
		prefix := "  "
		if i == m.searchState.Selected {
			prefix = ">"
		}

		title := anime.Name
		if len(title) > 50 {
			title = title[:50] + "..."
		}

		b.WriteString(fmt.Sprintf("%s %s\n", prefix, title))
	}

	m.viewport.SetContent(b.String())
}

// updateEpisodesViewport updates the viewport for episode selection
func (m *SearchModel) updateEpisodesViewport() {
	if len(m.episodeState.Episodes) == 0 {
		m.viewport.SetContent("Loading episodes...")
		return
	}

	var b strings.Builder
	for i, ep := range m.episodeState.Episodes {
		prefix := "  "
		if i == m.episodeState.Selected {
			prefix = ">"
		}

		// Show title if available, otherwise just episode number
		if len(m.episodeState.EpisodeTitles) > i && m.episodeState.EpisodeTitles[i] != "" {
			title := m.episodeState.EpisodeTitles[i]
			if len(title) > 40 {
				title = title[:40] + "..."
			}
			b.WriteString(fmt.Sprintf("%s Ep %s: %s\n", prefix, ep, title))
		} else {
			b.WriteString(fmt.Sprintf("%s Episode %s\n", prefix, ep))
		}
	}

	m.viewport.SetContent(b.String())
}

// View renders the UI
func (m *SearchModel) View() string {
	// Calculate widths
	panelWidth := m.width / 2
	if panelWidth < 40 {
		panelWidth = 40
	}

	// Left side - search input and results list
	leftPanel := m.renderLeftPanelFixed(panelWidth)

	// Right side - depends on mode
	var rightPanel string
	if m.mode == "episodes" {
		rightPanel = m.renderEpisodesRightPanel(panelWidth)
	} else {
		rightPanel = m.renderRightPanelFixed(panelWidth)
	}

	// Combine with fixed widths
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftPanel,
		rightPanel,
	)
}

// renderEpisodesRightPanel renders the right panel for episode selection
func (m *SearchModel) renderEpisodesRightPanel(width int) string {
	if m.episodeState.Loading {
		return loadingStyle.Width(width).Render("Loading episodes...")
	}

	if len(m.episodeState.Episodes) == 0 {
		return metadataPanelStyle.Width(width).Render("No episodes found")
	}

	selectedEp := m.episodeState.Episodes[m.episodeState.Selected]
	selectedTitle := ""
	if len(m.episodeState.EpisodeTitles) > m.episodeState.Selected {
		selectedTitle = m.episodeState.EpisodeTitles[m.episodeState.Selected]
	}

	var content string
	if selectedTitle != "" {
		content = fmt.Sprintf("Episode: %s\nTitle: %s\n\nUse ↑/↓ to navigate\nPress Enter to play", selectedEp, selectedTitle)
	} else {
		content = fmt.Sprintf("Episode: %s\n\nUse ↑/↓ to navigate\nPress Enter to play", selectedEp)
	}

	return metadataPanelStyle.Width(width).Render(content)
}

func (m *SearchModel) renderLeftPanelFixed(width int) string {
	inputStyle := lipgloss.NewStyle().
		Width(width).
		Foreground(lipgloss.Color("86"))

	inputView := inputStyle.Render(m.textInput.View())
	resultsView := m.viewport.View()

	return lipgloss.JoinVertical(
		lipgloss.Left,
		inputView,
		resultsView,
	)
}

func (m *SearchModel) renderRightPanelFixed(width int) string {
	if m.searchState.MetadataLoading {
		return loadingStyle.Width(width).Render("Loading metadata...")
	}

	if m.searchState.Metadata == nil {
		return metadataPanelStyle.Width(width).Render("Select an anime")
	}

	meta := m.searchState.Metadata

	var b strings.Builder

	b.WriteString(titleStyle.Render(meta.Title) + "\n")

	if meta.TitleEnglish != "" && meta.TitleEnglish != meta.Title {
		b.WriteString(italicStyle.Render(meta.TitleEnglish) + "\n")
	}

	b.WriteString(fmt.Sprintf("Type: %s | Year: %d | Episodes: %d\n", meta.Type, meta.Year, meta.Episodes))
	b.WriteString(fmt.Sprintf("Status: %s\n", meta.Status))
	b.WriteString(fmt.Sprintf("Score: %.2f | Rank: #%d\n", meta.Score, meta.Rank))

	if len(meta.Genres) > 0 {
		b.WriteString(fmt.Sprintf("Genres: %s\n", strings.Join(meta.Genres, ", ")))
	}

	b.WriteString("\nSynopsis:\n")
	b.WriteString(synopsisStyle.Render(meta.Synopsis))

	return metadataPanelStyle.Width(width).Render(b.String())
}

func (m *SearchModel) renderLeftPanel() string {
	return m.renderLeftPanelFixed(40)
}

func (m *SearchModel) renderRightPanel() string {
	return m.renderRightPanelFixed(40)
}

// Message types
type SearchResultsMsg struct {
	Results []*source.Anime
}

type MetadataLoadedMsg struct {
	Metadata *MetadataPanel
}

type EpisodesLoadedMsg struct {
	Episodes      []string
	EpisodeTitles []string
	Error         error
}

// TUIErrorMsg is used for various TUI errors
type TUIErrorMsg struct {
	Err error
}

type SearchErrorMsg struct {
	Err error
}

// Helper functions
func truncateSynopsis(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	// Find last space before maxLen
	lastSpace := strings.LastIndex(s[:maxLen], " ")
	if lastSpace == -1 {
		return s[:maxLen] + "..."
	}
	return s[:lastSpace] + "..."
}

// Styles
var (
	metadataPanelStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("240")).
				Padding(1, 2)

	loadingStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(1, 2)

	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")).
			Bold(true)

	italicStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Italic(true)

	synopsisStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245"))
)

// RunSearch launches the interactive anime search and returns selected anime and episode
func RunSearch() (*SelectionResult, error) {
	model := NewSearchModel()
	p := tea.NewProgram(model, tea.WithAltScreen())

	err := p.Start()
	if err != nil {
		return nil, err
	}

	return model.selectedResult, nil
}

// GetSelectedAnime returns the currently selected anime
func (m *SearchModel) GetSelectedAnime() *source.Anime {
	if m.searchState.Selected < len(m.searchState.Results) {
		return m.searchState.Results[m.searchState.Selected]
	}
	return nil
}