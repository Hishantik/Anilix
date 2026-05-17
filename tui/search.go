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
	"github.com/anilix/anilix/tui/style"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	paddingStyle       = lipgloss.NewStyle().Padding(1, 2)
	subDubPaddingStyle = lipgloss.NewStyle().Padding(0, 1)
)

type animeItem struct {
	anime *source.Anime
}

func (i animeItem) Title() string       { return i.anime.Name }
func (i animeItem) Description() string { return "" }
func (i animeItem) FilterValue() string { return i.anime.Name }

type episodeItem struct {
	number string
	title  string
}

func (i episodeItem) Title() string       { return fmt.Sprintf("Episode %s", i.number) }
func (i episodeItem) Description() string { return i.title }
func (i episodeItem) FilterValue() string { return i.number }

type SearchModel struct {
	state tuiState

	searchState  *SearchState
	episodeState *EpisodeState

	textInput textinput.Model
	help      help.Model
	keymap    keymap

	searchList  list.Model
	episodeList list.Model

	allanimeClient   *allanime.AllanimeClient
	jikanClient      *jikan.JikanClient
	allanimeProvider *allanime.AllanimeProvider

	selectedResult *SelectionResult
	lastQuery      string

	width     int
	height    int
	listWidth int

	prevSearchListIndex int
}

func NewSearchModel() *SearchModel {
	ti := textinput.New()
	ti.Placeholder = "Search anime..."
	ti.Focus()
	ti.Prompt = "> "
	ti.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#F2BB05"))
	ti.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#E2294F"))
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#3D348B"))

	allanimeProvider := allanime.NewAllanimeProvider()
	allanimeProvider.SetTranslation("sub")

	help := help.New()
	help.Styles.ShortKey = lipgloss.NewStyle().Foreground(lipgloss.Color("#F2BB05"))
	help.Styles.ShortDesc = lipgloss.NewStyle().Foreground(lipgloss.Color("#B6C2D9"))
	help.Styles.FullKey = lipgloss.NewStyle().Foreground(lipgloss.Color("#F2BB05"))
	help.Styles.FullDesc = lipgloss.NewStyle().Foreground(lipgloss.Color("#B6C2D9"))

	km := newKeymap()

	searchList := makeList("Search Results", km)
	episodeList := makeList("Episodes", km)

	return &SearchModel{
		state:            searchState,
		searchState:      NewSearchState(),
		episodeState:     NewEpisodeState(),
		textInput:        ti,
		help:             help,
		keymap:           km,
		searchList:       searchList,
		episodeList:      episodeList,
		allanimeClient:   allanime.NewAllanimeClient(),
		jikanClient:      jikan.NewClient("https://api.jikan.moe/v4"),
		allanimeProvider: allanimeProvider,
		lastQuery:        "",
	}
}

func (m *SearchModel) Init() tea.Cmd {
	return nil
}

func (m *SearchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.resize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keymap.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.keymap.Back):
			if m.state == episodesState {
				m.state = searchState
				m.episodeState = NewEpisodeState()
				m.textInput.Focus()
				return m, nil
			}

		case key.Matches(msg, m.keymap.Toggle):
			if m.searchState.TranslationType == "sub" {
				m.searchState.TranslationType = "dub"
			} else {
				m.searchState.TranslationType = "sub"
			}

			if m.state == episodesState && m.episodeState.AnimeID != "" {
				selectedAnime := m.searchState.Results[m.searchState.Selected]
				if selectedAnime != nil {
					cmds = append(cmds, m.fetchEpisodes(m.episodeState.AnimeID, selectedAnime.MALID))
				}
			} else if m.lastQuery != "" {
				cmds = append(cmds, m.doSearch(m.lastQuery))
			}
			return m, tea.Batch(cmds...)

		case key.Matches(msg, m.keymap.Select):
			if m.state == searchState {
				m.searchState.Selected = m.searchList.Index()
				if len(m.searchState.Results) > 0 && m.searchState.Selected < len(m.searchState.Results) {
					anime := m.searchState.Results[m.searchState.Selected]
					if anime.AllAnimeID != "" {
						m.state = episodesState
						m.episodeState.AnimeID = anime.AllAnimeID
						m.textInput.Blur()
						cmds = append(cmds, m.fetchEpisodes(anime.AllAnimeID, anime.MALID))
						return m, tea.Batch(cmds...)
					}
				}
			} else if m.state == episodesState {
				m.episodeState.Selected = m.episodeList.Index()
				if len(m.episodeState.Episodes) > 0 && m.episodeState.Selected < len(m.episodeState.Episodes) {
					selectedAnime := m.searchState.Results[m.searchState.Selected]
					selectedEpisode := m.episodeState.Episodes[m.episodeState.Selected]
					cmds = append(cmds, m.playEpisode(selectedAnime.AllAnimeID, selectedEpisode, selectedAnime.Name))
					return m, tea.Batch(cmds...)
				}
			}
		}

		if m.state == searchState {
			var cmd tea.Cmd
			m.textInput, cmd = m.textInput.Update(msg)
			cmds = append(cmds, cmd)

			query := m.textInput.Value()
			if query != m.lastQuery && len(query) >= 2 {
				m.lastQuery = query
				m.searchState.Query = query
				cmds = append(cmds, m.doSearch(query))
			}
		}

		if m.state == searchState && len(m.searchState.Results) > 0 {
			var cmd tea.Cmd
			m.searchList, cmd = m.searchList.Update(msg)
			cmds = append(cmds, cmd)
			if m.searchList.Index() != m.prevSearchListIndex {
				m.prevSearchListIndex = m.searchList.Index()
				m.searchState.Selected = m.searchList.Index()
				cmds = append(cmds, m.fetchMetadata())
			}
		} else if m.state == episodesState && len(m.episodeState.Episodes) > 0 {
			var cmd tea.Cmd
			m.episodeList, cmd = m.episodeList.Update(msg)
			cmds = append(cmds, cmd)
			m.episodeState.Selected = m.episodeList.Index()
		}

	case SearchResultsMsg:
		m.searchState.Results = msg.Results
		m.searchState.Loading = false
		m.searchState.Selected = 0
		if len(msg.Results) > 0 {
			cmds = append(cmds, m.fetchMetadata())
		}
		m.updateSearchList()

	case MetadataLoadedMsg:
		m.searchState.Metadata = msg.Metadata
		m.searchState.MetadataLoading = false

	case EpisodesLoadedMsg:
		m.episodeState.Episodes = msg.Episodes
		m.episodeState.EpisodeTitles = msg.EpisodeTitles
		m.episodeState.Loading = false
		if msg.Error != nil {
			m.episodeState.Err = msg.Error
		}
		m.updateEpisodeList()

	case TUIErrorMsg:
		m.episodeState.Err = msg.Err
	}

	return m, tea.Batch(cmds...)
}

func (m *SearchModel) resize(width, height int) {
	m.width = width
	m.height = height

	x, y := paddingStyle.GetFrameSize()
	styledWidth := width - x
	styledHeight := height - y

	m.listWidth = styledWidth / 2
	if m.listWidth < 20 {
		m.listWidth = 20
	}
	listHeight := styledHeight - 6
	if listHeight < 1 {
		listHeight = 1
	}

	m.searchList.SetSize(m.listWidth, listHeight)
	m.episodeList.SetSize(m.listWidth, listHeight)
	m.textInput.Width = styledWidth / 2
	m.help.Width = styledWidth
}

func (m *SearchModel) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	var content string

	switch m.state {
	case searchState:
		content = m.viewSearchState()
	case episodesState:
		content = m.viewEpisodesState()
	}

	return m.renderContent(content)
}

func (m *SearchModel) viewSearchState() string {
	panelWidth := m.listWidth

	leftPanel := m.renderLeftPanel(panelWidth)
	rightPanel := m.renderRightPanel(panelWidth)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
}

func (m *SearchModel) viewEpisodesState() string {
	panelWidth := m.listWidth

	leftPanel := m.renderEpisodeLeftPanel(panelWidth)
	rightPanel := m.renderEpisodeRightPanel(panelWidth)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
}

func (m *SearchModel) renderLeftPanel(width int) string {
	var lines []string

	switchText := renderTextSwitch(m.searchState.TranslationType)
	lines = append(lines, subDubPaddingStyle.Render(switchText))
	lines = append(lines, m.textInput.View())
	lines = append(lines, "")
	lines = append(lines, m.searchList.View())

	return strings.Join(lines, "\n")
}

func (m *SearchModel) renderRightPanel(width int) string {
	if m.searchState.MetadataLoading {
		return paddingStyle.Render(style.New().Foreground(lipgloss.Color("#7D1128")).Render("Loading metadata..."))
	}

	if m.searchState.Metadata == nil {
		return paddingStyle.Render(style.New().Foreground(lipgloss.Color("#7D1128")).Render("Select an anime"))
	}

	return paddingStyle.Render(strings.Join(m.renderMetadata(), "\n"))
}

func (m *SearchModel) renderEpisodeLeftPanel(width int) string {
	var lines []string

	switchText := renderTextSwitch(m.searchState.TranslationType)
	lines = append(lines, subDubPaddingStyle.Render(switchText))
	lines = append(lines, "")
	lines = append(lines, m.episodeList.View())

	return strings.Join(lines, "\n")
}

func (m *SearchModel) renderEpisodeRightPanel(width int) string {
	if len(m.episodeState.Episodes) > 0 && m.episodeState.Selected < len(m.episodeState.Episodes) {
		selectedEp := m.episodeState.Episodes[m.episodeState.Selected]
		var selectedTitle string
		if len(m.episodeState.EpisodeTitles) > m.episodeState.Selected {
			selectedTitle = m.episodeState.EpisodeTitles[m.episodeState.Selected]
		}
		if selectedTitle != "" {
			return paddingStyle.Render(style.New().Foreground(lipgloss.Color("#FF2C55")).Bold(true).Render(fmt.Sprintf("Episode %s: %s", selectedEp, selectedTitle)))
		}
		return paddingStyle.Render(style.New().Foreground(lipgloss.Color("#FF2C55")).Bold(true).Render(fmt.Sprintf("Episode %s", selectedEp)))
	}
	return paddingStyle.Render(style.Faint("No episodes"))
}

func (m *SearchModel) renderMetadata() []string {
	meta := m.searchState.Metadata
	var lines []string

	lines = append(lines, style.New().Foreground(lipgloss.Color("#FF2C55")).Bold(true).Render(meta.Title))

	if meta.TitleEnglish != "" && meta.TitleEnglish != meta.Title {
		lines = append(lines, style.New().Foreground(lipgloss.Color("#C41E3D")).Italic(true).Render(meta.TitleEnglish))
	}

	lines = append(lines, style.New().Foreground(lipgloss.Color("#E2294F")).Render(fmt.Sprintf("Type: %s | Year: %d | Episodes: %d", meta.Type, meta.Year, meta.Episodes)))
	lines = append(lines, style.New().Foreground(lipgloss.Color("#E2294F")).Render(fmt.Sprintf("Status: %s", meta.Status)))
	lines = append(lines, style.New().Foreground(lipgloss.Color("#E2294F")).Render(fmt.Sprintf("Score: %.2f | Rank: #%d", meta.Score, meta.Rank)))

	if len(meta.Genres) > 0 {
		lines = append(lines, style.New().Foreground(lipgloss.Color("#C41E3D")).Render(fmt.Sprintf("Genres: %s", strings.Join(meta.Genres, ", "))))
	}

	lines = append(lines, "")
	lines = append(lines, style.New().Foreground(lipgloss.Color("#FF2C55")).Bold(true).Render("Synopsis:"))
	lines = append(lines, style.Faint(truncateSynopsis(meta.Synopsis, 300)))

	return lines
}

func (m *SearchModel) renderContent(content string) string {
	if m.height <= 0 {
		return ""
	}

	h := strings.Count(content, "\n") + 1
	helpView := m.help.View(m.keymap)
	remaining := m.height - h - 2
	if remaining < 0 {
		remaining = 0
	}
	return content + strings.Repeat("\n", remaining) + helpView
}

func (m *SearchModel) updateSearchList() {
	items := make([]list.Item, 0, len(m.searchState.Results))
	for _, anime := range m.searchState.Results {
		items = append(items, animeItem{anime: anime})
	}
	cmd := m.searchList.SetItems(items)
	if cmd != nil {
		cmd()
	}
	m.searchList.Select(m.searchState.Selected)
}

func (m *SearchModel) updateEpisodeList() {
	items := make([]list.Item, 0, len(m.episodeState.Episodes))
	for i, ep := range m.episodeState.Episodes {
		title := ""
		if len(m.episodeState.EpisodeTitles) > i {
			title = m.episodeState.EpisodeTitles[i]
		}
		items = append(items, episodeItem{number: ep, title: title})
	}
	cmd := m.episodeList.SetItems(items)
	if cmd != nil {
		cmd()
	}
	m.episodeList.Select(m.episodeState.Selected)
}

func (m *SearchModel) doSearch(query string) tea.Cmd {
	return func() tea.Msg {
		m.searchState.Loading = true

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		translationType := m.searchState.TranslationType
		if translationType == "" {
			translationType = "sub"
		}

		shows, err := m.allanimeClient.SearchShows(ctx, query, 20, 1, translationType)
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
		return nil
	}

	m.searchState.MetadataLoading = true

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		data, err := m.jikanClient.GetAnime(ctx, anime.MALID)
		if err != nil {
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

func (m *SearchModel) fetchEpisodes(showID string, malID int) tea.Cmd {
	m.episodeState.Loading = true

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		translationType := m.searchState.TranslationType
		if translationType == "" {
			translationType = "sub"
		}
		episodes, err := m.allanimeClient.GetShowEpisodes(ctx, showID, translationType)
		if err != nil {
			m.episodeState.Err = err
			m.episodeState.Loading = false
			return EpisodesLoadedMsg{Episodes: nil, EpisodeTitles: nil, Error: err}
		}

		epList, ok := episodes[translationType]
		if !ok {
			epList, ok = episodes["sub"]
			if !ok {
				m.episodeState.Err = fmt.Errorf("no episodes found for %s", translationType)
				m.episodeState.Loading = false
				return EpisodesLoadedMsg{Episodes: nil, EpisodeTitles: nil, Error: m.episodeState.Err}
			}
		}
		m.episodeState.Episodes = epList
		m.episodeState.Selected = 0

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

		return EpisodesLoadedMsg{Episodes: epList, EpisodeTitles: episodeTitles, Error: nil}
	}
}

func (m *SearchModel) playEpisode(showID, episodeNum, animeTitle string) tea.Cmd {
	return func() tea.Msg {
		translationType := m.searchState.TranslationType
		if translationType == "" {
			translationType = "sub"
		}
		m.allanimeProvider.SetTranslation(translationType)

		episodeNumFloat, _ := strconv.ParseFloat(episodeNum, 64)
		episode := &source.Episode{
			Number: episodeNumFloat,
			Anime: &source.Anime{
				AllAnimeID: showID,
				Name:       animeTitle,
			},
		}

		streams, err := m.allanimeProvider.StreamsOf(episode)
		if err != nil {
			return TUIErrorMsg{Err: fmt.Errorf("failed to get streams: %w", err)}
		}

		if len(streams) == 0 {
			return TUIErrorMsg{Err: fmt.Errorf("no streams found")}
		}

		playStream := tryPlayStream(streams, animeTitle, episodeNum)
		if playStream == nil {
			return TUIErrorMsg{Err: fmt.Errorf("no playable stream found")}
		}

		return nil
	}
}

func tryPlayStream(streams []*source.Stream, animeTitle, episodeNum string) *source.Stream {
	p := player.Mpv
	opts := player.Options{
		Title: fmt.Sprintf("%s - Episode %s", animeTitle, episodeNum),
	}

	for _, s := range streams {
		url := s.URL
		if strings.HasPrefix(url, "//") {
			url = "https:" + url
		}

		opts.Referrer = s.Referer

		if err := p.Launch(url, opts); err == nil {
			return s
		}
	}

	return nil
}

func makeList(title string, km keymap) list.Model {
	delegate := list.NewDefaultDelegate()

	delegate.Styles.SelectedTitle = lipgloss.NewStyle().
		Border(lipgloss.ThickBorder(), false, false, false, true).
		BorderForeground(lipgloss.Color("#F2BB05")).
		Foreground(lipgloss.Color("#F2BB05")).
		Padding(0, 0, 0, 1)

	delegate.Styles.SelectedDesc = lipgloss.NewStyle().
		Border(lipgloss.ThickBorder(), false, false, false, true).
		BorderForeground(lipgloss.Color("#F2BB05")).
		Foreground(lipgloss.Color("#B6C2D9")).
		Padding(0, 0, 0, 1)

	delegate.Styles.NormalTitle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#B6C2D9")).
		Padding(0, 0, 0, 1)

	delegate.Styles.NormalDesc = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#3D348B")).
		Padding(0, 0, 0, 1)

	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Title = title
	l.Styles.Title = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F2BB05")).
		Background(lipgloss.Color("#3D348B")).
		Padding(0, 1).
		Bold(true)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.KeyMap = list.KeyMap{
		CursorUp:   km.Up,
		CursorDown: km.Down,
	}

	return l
}

func renderTextSwitch(translationType string) string {
	var subPart, dubPart string
	if translationType == "sub" {
		subPart = style.SubTitle("SUB")
		dubPart = style.Faint("DUB")
	} else {
		subPart = style.Faint("SUB")
		dubPart = style.DubTitle("DUB")
	}

	separator := style.Faint(" / ")
	inner := lipgloss.JoinHorizontal(lipgloss.Top, subPart, separator, dubPart)

	return inner
}

func truncateSynopsis(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	lastSpace := strings.LastIndex(s[:maxLen], " ")
	if lastSpace == -1 {
		return s[:maxLen] + "..."
	}
	return s[:lastSpace] + "..."
}

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

type TUIErrorMsg struct {
	Err error
}

type SearchErrorMsg struct {
	Err error
}

func RunSearch() (*SelectionResult, error) {
	model := NewSearchModel()
	p := tea.NewProgram(model, tea.WithAltScreen())

	err := p.Start()
	if err != nil {
		return nil, err
	}

	return model.selectedResult, nil
}

func (m *SearchModel) GetSelectedAnime() *source.Anime {
	if m.searchState.Selected < len(m.searchState.Results) {
		return m.searchState.Results[m.searchState.Selected]
	}
	return nil
}
