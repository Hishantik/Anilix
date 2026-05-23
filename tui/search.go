package tui

import (
	"context"
	"fmt"
	"image/color"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hishantik/anilix/aniskip"
	"github.com/hishantik/anilix/config"
	allanime "github.com/hishantik/anilix/provider/allanime"
	"github.com/hishantik/anilix/provider/anilist"
	"github.com/hishantik/anilix/provider/jikan"
	"github.com/hishantik/anilix/source"
	"github.com/hishantik/anilix/player"
	"github.com/hishantik/anilix/tui/style"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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
	loading   spinner.Model
	progress  progress.Model

	searchList  list.Model
	episodeList list.Model

	allanimeClient   *allanime.AllanimeClient
	jikanClient      *jikan.JikanClient
	anilistClient    *anilist.Client
	allanimeProvider *allanime.AllanimeProvider

	prevState     tuiState
	confirmSelect int // 0 = yes, 1 = no

	settingsState *SettingsState

	selectedResult *SelectionResult
	lastQuery      string

	width     int
	height    int
	listWidth int

	progressPercent   float64
	progressStart     time.Time

	prevSearchListIndex   int
	prefetchCancel        context.CancelFunc
	debounceTimer         *time.Timer
	episodeTitlesCache    map[int][]string
	episodeTitlesCacheMu  sync.Mutex
	episodeMetadataCache  map[string]*jikan.Episode
	metadataFetchChan     chan int
	episodeMetadataChan   chan int
	episodeDebounceTimer  *time.Timer
	prevEpisodeListIndex  int
}

func NewSearchModel() *SearchModel {
	ti := textinput.New()
	ti.Placeholder = "Type to search..."
	ti.Focus()
	ti.Prompt = "> "
	ts := textinput.DefaultStyles(true)
	ts.Focused.Prompt = lipgloss.NewStyle().Foreground(lipgloss.Color("#9d4edd"))
	ts.Focused.Text = lipgloss.NewStyle().Foreground(lipgloss.Color("#f4f4f6"))
	ts.Focused.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color("#f4f4f6"))
	ti.SetStyles(ts)

	allanimeProvider := allanime.NewAllanimeProvider()
	allanimeProvider.SetTranslation("sub")

	h := help.New()
	hs := help.DefaultStyles(true)
	hs.ShortKey = lipgloss.NewStyle().Foreground(lipgloss.Color("#9d4edd"))
	hs.ShortDesc = lipgloss.NewStyle().Foreground(lipgloss.Color("#f4f4f6"))
	hs.FullKey = lipgloss.NewStyle().Foreground(lipgloss.Color("#9d4edd"))
	hs.FullDesc = lipgloss.NewStyle().Foreground(lipgloss.Color("#f4f4f6"))
	h.Styles = hs

	km := newKeymap()

	s := spinner.New()
	s.Spinner = spinner.Points
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#9d4edd"))

	p := progress.New(progress.WithColors(lipgloss.Color("#9d4edd"), lipgloss.Color("#c084fc")), progress.WithScaled(true))
	p.SetWidth(40)

	searchList := makeList("Search Results", km)
	episodeList := makeList("Episodes", km)

	return &SearchModel{
		state:            searchState,
		searchState:      NewSearchState(),
		episodeState:     NewEpisodeState(),
		settingsState: &SettingsState{
			Quality: func() string {
				q := config.GetString("quality")
				if q == "" {
					return "auto"
				}
				return q
			}(),
			AniskipEnabled: config.GetBool("aniskip.enabled"),
		},
		textInput:        ti,
		help:             h,
		keymap:           km,
		loading:          s,
		progress:         p,
		searchList:       searchList,
		episodeList:      episodeList,
		allanimeClient:   allanime.NewAllanimeClient(),
		jikanClient:      jikan.NewClient("https://api.jikan.moe/v4"),
		anilistClient:    anilist.NewClient(),
		allanimeProvider: allanimeProvider,
		lastQuery:        "",
		episodeTitlesCache: make(map[int][]string),
		episodeMetadataCache: make(map[string]*jikan.Episode),
		metadataFetchChan: make(chan int, 1),
		episodeMetadataChan: make(chan int, 1),
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

	case spinner.TickMsg:
		m.loading, _ = m.loading.Update(msg)

	case progressTickMsg:
		if m.searchState.Loading || m.episodeState.Loading || m.episodeState.Playing {
			elapsed := time.Since(m.progressStart).Seconds()
			m.progressPercent = math.Min(0.9, elapsed/5.0)
		}
		cmds = append(cmds, tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
			return progressTickMsg{}
		}))

	case tea.KeyMsg:
		if m.state == searchState && m.textInput.Focused() {
			switch {
			case key.Matches(msg, m.keymap.Quit):
				m.prevState = m.state
				m.state = confirmQuitState
				return m, nil
			case key.Matches(msg, m.keymap.Back):
				m.textInput.Blur()
				return m, nil
			case key.Matches(msg, m.keymap.Select):
				query := m.textInput.Value()
				if len(query) >= 2 {
					m.lastQuery = query
					m.searchState.Query = query
					m.searchState.Loading = true
					m.progressPercent = 0
					m.progressStart = time.Now()
					m.textInput.Blur()
					cmds = append(cmds, m.doSearch(query))
					cmds = append(cmds, tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
						return progressTickMsg{}
					}))
					return m, tea.Batch(cmds...)
				}
				return m, nil
			case msg.String() == "up", msg.String() == "down":
				m.textInput.Blur()
				// pass through to list handling below
			case key.Matches(msg, m.keymap.Toggle):
				if m.searchState.TranslationType == "sub" {
					m.searchState.TranslationType = "dub"
				} else {
					m.searchState.TranslationType = "sub"
				}
				return m, nil
			case key.Matches(msg, m.keymap.Settings):
				m.prevState = m.state
				m.state = settingsState
				return m, nil
			default:
				var cmd tea.Cmd
				m.textInput, cmd = m.textInput.Update(msg)
				cmds = append(cmds, cmd)
				return m, tea.Batch(cmds...)
			}
		}

		if m.state == episodesState && m.textInput.Focused() {
			switch {
			case key.Matches(msg, m.keymap.Quit):
				m.prevState = m.state
				m.state = confirmQuitState
				return m, nil
			case key.Matches(msg, m.keymap.Back):
				m.textInput.Blur()
				return m, nil
			case key.Matches(msg, m.keymap.Select):
				query := m.textInput.Value()
				if query != "" {
					m.selectEpisodeByNumber(query)
				}
				m.textInput.Blur()
				return m, nil
			case msg.String() == "up", msg.String() == "down":
				m.textInput.Blur()
				// pass through to list handling below
			default:
				var cmd tea.Cmd
				m.textInput, cmd = m.textInput.Update(msg)
				cmds = append(cmds, cmd)
				m.filterEpisodesByNumber(m.textInput.Value())
				return m, tea.Batch(cmds...)
			}
		}

		// Handle settingsState keys
		if m.state == settingsState {
			switch msg.String() {
			case "up", "k":
				if m.settingsState.Cursor > 0 {
					m.settingsState.Cursor--
				}
			case "down", "j":
				if m.settingsState.Cursor < 1 {
					m.settingsState.Cursor++
				}
			case "left", "h", "right", "l":
				if m.settingsState.Cursor == 0 {
					// Quality cycle
					qualities := []string{"best", "1080p", "720p", "480p", "360p", "auto"}
					idx := 0
					for i, q := range qualities {
						if q == m.settingsState.Quality {
							idx = i
							break
						}
					}
					if msg.String() == "left" || msg.String() == "h" {
						idx = (idx - 1 + len(qualities)) % len(qualities)
					} else {
						idx = (idx + 1) % len(qualities)
					}
					m.settingsState.Quality = qualities[idx]
					config.Set("quality", m.settingsState.Quality)
				} else {
					// AniSkip toggle
					m.settingsState.AniskipEnabled = !m.settingsState.AniskipEnabled
					config.Set("aniskip.enabled", m.settingsState.AniskipEnabled)
				}
			case "esc":
				m.state = m.prevState
				return m, nil
			}
			return m, nil
		}

		// Handle confirmQuitState keys first
		if m.state == confirmQuitState {
			switch msg.String() {
			case "left", "h":
				m.confirmSelect = 0
			case "right", "l":
				m.confirmSelect = 1
			case "enter":
				if m.confirmSelect == 0 {
					return m, tea.Quit
				}
				m.state = m.prevState
				return m, nil
			case "y":
				return m, tea.Quit
			case "n", "esc":
				m.state = m.prevState
				return m, nil
			}
			return m, nil
		}

		switch {
		case key.Matches(msg, m.keymap.Quit):
			m.prevState = m.state
			m.confirmSelect = 1 // default to "No"
			m.state = confirmQuitState
			return m, nil

		case key.Matches(msg, m.keymap.Back):
			if m.state == episodesState {
				m.state = searchState
				m.episodeState = NewEpisodeState()
				m.textInput.Placeholder = "Type to search..."
				return m, nil
			}

		case key.Matches(msg, m.keymap.Search):
			if m.state == episodesState {
				m.state = searchState
				m.episodeState = NewEpisodeState()
			}
			m.textInput.Focus()
			m.textInput.SetValue("")
			return m, nil

		case key.Matches(msg, m.keymap.Toggle):
			if m.searchState.TranslationType == "sub" {
				m.searchState.TranslationType = "dub"
			} else {
				m.searchState.TranslationType = "sub"
			}

			if m.state == episodesState && m.episodeState.AnimeID != "" {
				m.episodeState.Loading = true
				m.progressPercent = 0
				m.progressStart = time.Now()
				selectedAnime := m.searchState.Results[m.searchState.Selected]
				if selectedAnime != nil {
					cmds = append(cmds, m.fetchEpisodes(m.episodeState.AnimeID, selectedAnime.MALID))
					cmds = append(cmds, tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
						return progressTickMsg{}
					}))
				}
			} else if m.lastQuery != "" {
				m.searchState.Loading = true
				m.progressPercent = 0
				m.progressStart = time.Now()
				cmds = append(cmds, m.doSearch(m.lastQuery))
				cmds = append(cmds, tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
					return progressTickMsg{}
				}))
			}

		case key.Matches(msg, m.keymap.Settings):
			m.prevState = m.state
			m.state = settingsState
			return m, nil

		case key.Matches(msg, m.keymap.Select):
			if m.state == searchState {
				m.searchState.Selected = m.searchList.Index()
				if len(m.searchState.Results) > 0 && m.searchState.Selected < len(m.searchState.Results) {
					anime := m.searchState.Results[m.searchState.Selected]
					if anime.AllAnimeID != "" {
						m.state = episodesState
						m.textInput.Placeholder = "Search episode..."
						m.episodeState.AnimeID = anime.AllAnimeID
						m.episodeState.Loading = true
						m.progressPercent = 0
						m.progressStart = time.Now()
						cmds = append(cmds, m.fetchEpisodes(anime.AllAnimeID, anime.MALID))
						cmds = append(cmds, tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
							return progressTickMsg{}
						}))
					}
				}
			} else if m.state == episodesState {
				m.episodeState.Selected = m.episodeList.Index()
				if len(m.episodeState.Episodes) > 0 && m.episodeState.Selected < len(m.episodeState.Episodes) {
					selectedAnime := m.searchState.Results[m.searchState.Selected]
					selectedEpisode := m.episodeState.Episodes[m.episodeState.Selected]
					m.episodeState.Playing = true
					m.progressPercent = 0
					m.progressStart = time.Now()
					cmds = append(cmds, m.playEpisode(selectedAnime.AllAnimeID, selectedEpisode, selectedAnime.Name, selectedAnime.MALID))
					cmds = append(cmds, tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
						return progressTickMsg{}
					}))
					return m, tea.Batch(cmds...)
				}
			}
		}

		if m.state == searchState && !m.textInput.Focused() && len(m.searchState.Results) > 0 {
			var cmd tea.Cmd
			m.searchList, cmd = m.searchList.Update(msg)
			cmds = append(cmds, cmd)
			if m.searchList.Index() != m.prevSearchListIndex {
				m.prevSearchListIndex = m.searchList.Index()
				m.searchState.Selected = m.searchList.Index()

				if m.debounceTimer != nil {
					m.debounceTimer.Stop()
				}

				idx := m.searchList.Index()
				m.debounceTimer = time.AfterFunc(150*time.Millisecond, func() {
					select {
					case m.metadataFetchChan <- idx:
					default:
					}
				})

				if m.prefetchCancel != nil {
					m.prefetchCancel()
				}
				ctx, cancel := context.WithCancel(context.Background())
				m.prefetchCancel = cancel
				go m.prefetchMetadata(ctx, m.searchList.Index()+1, m.searchList.Index()+6)
			}
		} else if m.state == episodesState && !m.textInput.Focused() && len(m.episodeState.Episodes) > 0 {
			// Auto-focus search bar when typing numbers
			if msg.String() >= "0" && msg.String() <= "9" {
				m.textInput.Focus()
				m.textInput.SetValue("")
				var cmd tea.Cmd
				m.textInput, cmd = m.textInput.Update(msg)
				cmds = append(cmds, cmd)
				m.filterEpisodesByNumber(m.textInput.Value())
				return m, tea.Batch(cmds...)
			}
			var cmd tea.Cmd
			m.episodeList, cmd = m.episodeList.Update(msg)
			cmds = append(cmds, cmd)
			if m.episodeList.Index() != m.prevEpisodeListIndex {
				m.prevEpisodeListIndex = m.episodeList.Index()
				m.episodeState.Selected = m.episodeList.Index()

				if m.episodeDebounceTimer != nil {
					m.episodeDebounceTimer.Stop()
				}

				idx := m.episodeList.Index()
				m.episodeDebounceTimer = time.AfterFunc(150*time.Millisecond, func() {
					select {
					case m.episodeMetadataChan <- idx:
					default:
					}
				})
			} else {
				m.episodeState.Selected = m.episodeList.Index()
			}
		}

	case SearchResultsMsg:
		m.searchState.Results = msg.Results
		m.searchState.Loading = false
		m.progressPercent = 1.0
		m.searchState.Selected = 0
		m.textInput.Blur()
		m.episodeTitlesCacheMu.Lock()
		m.episodeTitlesCache = make(map[int][]string)
		m.episodeTitlesCacheMu.Unlock()
		if m.prefetchCancel != nil {
			m.prefetchCancel()
		}
		m.progress = progress.New(progress.WithColors(lipgloss.Color("#9d4edd"), lipgloss.Color("#c084fc")), progress.WithScaled(true))
		m.progress.SetWidth(m.width)
		if len(msg.Results) > 0 {
			ctx, cancel := context.WithCancel(context.Background())
			m.prefetchCancel = cancel
			go m.prefetchMetadata(ctx, 0, 10)
			m.searchState.MetadataLoading = true
			m.loading = spinner.New()
			m.loading.Spinner = spinner.Points
			m.loading.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#9d4edd"))
			cmds = append(cmds, m.fetchMetadata())
			cmds = append(cmds, func() tea.Msg {
				select {
				case idx := <-m.metadataFetchChan:
					return MetadataFetchTriggered{Index: idx}
				case <-time.After(30 * time.Second):
					return MetadataFetchTriggered{Index: -1}
				}
			})
		}
		m.updateSearchList()

	case MetadataFetchTriggered:
		if msg.Index >= 0 && msg.Index == m.searchState.Selected {
			m.searchState.MetadataLoading = true
			m.loading = spinner.New()
			m.loading.Spinner = spinner.Points
			m.loading.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#9d4edd"))
			cmds = append(cmds, m.fetchMetadata())
		}
		if msg.Index >= 0 {
			cmds = append(cmds, func() tea.Msg {
				select {
				case idx := <-m.metadataFetchChan:
					return MetadataFetchTriggered{Index: idx}
				case <-time.After(30 * time.Second):
					return MetadataFetchTriggered{Index: -1}
				}
			})
		}

	case MetadataLoadedMsg:
		if msg.Index == m.searchState.Selected {
			m.searchState.Metadata = msg.Metadata
			m.searchState.MetadataLoading = false
		}

	case EpisodesLoadedMsg:
		m.episodeState.Episodes = msg.Episodes
		m.episodeState.EpisodeTitles = msg.EpisodeTitles
		m.episodeState.Loading = false
		m.progressPercent = 1.0
		m.progress = progress.New(progress.WithColors(lipgloss.Color("#9d4edd"), lipgloss.Color("#c084fc")), progress.WithScaled(true))
		m.progress.SetWidth(m.width)
		if msg.Error != nil {
			m.episodeState.Err = msg.Error
		}
		m.updateEpisodeList()
		if len(msg.Episodes) > 0 {
			m.episodeState.MetadataLoading = true
			cmds = append(cmds, m.fetchEpisodeMetadata())
			cmds = append(cmds, func() tea.Msg {
				select {
				case idx := <-m.episodeMetadataChan:
					return EpisodeMetadataFetchTriggered{Index: idx}
				case <-time.After(30 * time.Second):
					return EpisodeMetadataFetchTriggered{Index: -1}
				}
			})
		}

	case EpisodeMetadataFetchTriggered:
		if msg.Index >= 0 && msg.Index == m.episodeState.Selected {
			m.episodeState.MetadataLoading = true
			cmds = append(cmds, m.fetchEpisodeMetadata())
		}
		if msg.Index >= 0 {
			cmds = append(cmds, func() tea.Msg {
				select {
				case idx := <-m.episodeMetadataChan:
					return EpisodeMetadataFetchTriggered{Index: idx}
				case <-time.After(30 * time.Second):
					return EpisodeMetadataFetchTriggered{Index: -1}
				}
			})
		}

	case EpisodeMetadataLoadedMsg:
		if msg.CacheKey != "" && msg.RawEpisode != nil {
			m.episodeMetadataCache[msg.CacheKey] = msg.RawEpisode
		}
		if msg.Index == m.episodeState.Selected {
			m.episodeState.EpisodeMetadata = msg.Metadata
			m.episodeState.MetadataLoading = false
		}

	case PlayStreamMsg:
		m.episodeState.Playing = false
		m.progressPercent = 1.0
		m.progress = progress.New(progress.WithColors(lipgloss.Color("#9d4edd"), lipgloss.Color("#c084fc")), progress.WithScaled(true))
		m.progress.SetWidth(m.width)

	case TUIErrorMsg:
		m.episodeState.Playing = false
		m.progressPercent = 1.0
		m.progress = progress.New(progress.WithColors(lipgloss.Color("#9d4edd"), lipgloss.Color("#c084fc")), progress.WithScaled(true))
		m.progress.SetWidth(m.width)
		m.episodeState.Err = msg.Err
	}

	if m.searchState.MetadataLoading || m.episodeState.MetadataLoading {
		cmds = append(cmds, tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
			return spinner.TickMsg{Time: t}
		}))
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
	// Reserve space for: title(1) + margin(1) + searchBar(3) + margin(1) + progressBar(1) + helpBar(3) + padding(2)
	usedHeight := 12
	listHeight := styledHeight - usedHeight
	if listHeight < 1 {
		listHeight = 1
	}

	m.searchList.SetSize(m.listWidth, listHeight)
	m.episodeList.SetSize(m.listWidth, listHeight)
	m.textInput.SetWidth(styledWidth / 2)
	m.help.SetWidth(styledWidth)
	m.progress.SetWidth(m.width)
}

func (m *SearchModel) View() tea.View {
	if m.width == 0 || m.height == 0 {
		return tea.NewView("Loading...")
	}

	var content string

	switch m.state {
	case searchState:
		content = m.viewSearchState()
	case episodesState:
		content = m.viewEpisodesState()
	case confirmQuitState:
		content = m.viewConfirmQuit()
	case settingsState:
		content = m.viewSettings()
	}

	v := tea.NewView(m.renderContent(content))
	v.AltScreen = true
	return v
}

func gradientText(text string, colors []string) string {
	if len(text) == 0 || len(colors) == 0 {
		return text
	}
	var result strings.Builder
	for i, ch := range text {
		colorIdx := i * (len(colors) - 1) / (len(text) - 1)
		if colorIdx >= len(colors) {
			colorIdx = len(colors) - 1
		}
		result.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colors[colorIdx])).Render(string(ch)))
	}
	return result.String()
}

func (m *SearchModel) viewSearchState() string {
	panelWidth := m.listWidth

	gradientColors := []string{"#9d4edd", "#a855f7", "#b366ff", "#c084fc"}
	title := gradientText("ANILIX", gradientColors)
	titleLine := lipgloss.NewStyle().PaddingLeft(2).MarginBottom(1).Render(title)
	gradientBorderColors := []color.Color{lipgloss.Color("#9d4edd"), lipgloss.Color("#a855f7"), lipgloss.Color("#b366ff"), lipgloss.Color("#c084fc")}
	searchBar := lipgloss.NewStyle().Width(panelWidth / 2).Border(lipgloss.RoundedBorder()).BorderForegroundBlend(gradientBorderColors...).Padding(0, 1).MarginBottom(1).Render(m.textInput.View())
	leftPanel := lipgloss.NewStyle().Width(panelWidth).MaxWidth(panelWidth).Render(m.renderLeftPanel(panelWidth))
	rightPanelContent := m.renderRightPanel(panelWidth)
	rightPanel := lipgloss.NewStyle().Width(panelWidth).MaxWidth(panelWidth).Render(rightPanelContent)
	if m.searchState.Metadata != nil && !m.searchState.MetadataLoading {
		rightPanel = lipgloss.NewStyle().Width(panelWidth-6).MaxWidth(panelWidth-6).Border(lipgloss.RoundedBorder()).BorderForegroundBlend(gradientBorderColors...).Padding(0, 1).Margin(1).Render(rightPanelContent)
	}

	return titleLine + "\n" + searchBar + "\n" + lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
}

func (m *SearchModel) viewEpisodesState() string {
	panelWidth := m.listWidth

	gradientColors := []string{"#9d4edd", "#a855f7", "#b366ff", "#c084fc"}
	title := gradientText("ANILIX", gradientColors)
	titleLine := lipgloss.NewStyle().PaddingLeft(2).MarginBottom(1).Render(title)
	gradientBorderColors := []color.Color{lipgloss.Color("#9d4edd"), lipgloss.Color("#a855f7"), lipgloss.Color("#b366ff"), lipgloss.Color("#c084fc")}
	searchBar := lipgloss.NewStyle().Width(panelWidth / 2).Border(lipgloss.RoundedBorder()).BorderForegroundBlend(gradientBorderColors...).Padding(0, 1).MarginBottom(1).Render(m.textInput.View())
	leftPanel := lipgloss.NewStyle().Width(panelWidth).MaxWidth(panelWidth).Render(m.renderEpisodeLeftPanel(panelWidth))
	rightPanelContent := m.renderEpisodeRightPanel(panelWidth)
	rightPanel := lipgloss.NewStyle().Width(panelWidth).MaxWidth(panelWidth).Render(rightPanelContent)
	if m.episodeState.EpisodeMetadata != nil && !m.episodeState.MetadataLoading {
		rightPanel = lipgloss.NewStyle().Width(panelWidth-6).MaxWidth(panelWidth-6).Border(lipgloss.RoundedBorder()).BorderForegroundBlend(gradientBorderColors...).Padding(0, 1).Margin(1).Render(rightPanelContent)
	}

	return titleLine + "\n" + searchBar + "\n" + lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
}

func (m *SearchModel) viewConfirmQuit() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#f4f4f6")).
		Align(lipgloss.Center)

	promptStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#f4f4f6")).
		Align(lipgloss.Center).
		MarginTop(1)

	selectedBg := lipgloss.Color("#9d4edd")
	dimBg := lipgloss.Color("#555555")

	btnYesStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#f4f4f6")).
		Background(selectedBg).
		Padding(0, 2).
		MarginRight(1)

	btnNoStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#f4f4f6")).
		Background(selectedBg).
		Padding(0, 2).
		MarginLeft(1)

	if m.confirmSelect == 0 {
		btnNoStyle = btnNoStyle.Background(dimBg)
	} else {
		btnYesStyle = btnYesStyle.Background(dimBg)
	}

	boxWidth := 40
	title := titleStyle.Width(boxWidth).Render("Quit Anilix?")
	prompt := promptStyle.Width(boxWidth).Render("Are you sure you want to quit?")
	buttons := lipgloss.NewStyle().MarginTop(1).Render(
		lipgloss.JoinHorizontal(lipgloss.Center, btnYesStyle.Render("Yes"), btnNoStyle.Render("No")),
	)

	popup := lipgloss.JoinVertical(lipgloss.Center, title, prompt, buttons)

	popupBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#9d4edd")).
		Padding(1, 3).
		Render(popup)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, popupBox)
}

func (m *SearchModel) viewSettings() string {
	selectedBg := lipgloss.Color("#9d4edd")
	dimBg := lipgloss.Color("#555555")
	fg := lipgloss.Color("#f4f4f6")

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(fg).Align(lipgloss.Center)

	labelStyle := lipgloss.NewStyle().Foreground(fg).Width(14)

	selectedStyle := lipgloss.NewStyle().
		Bold(true).Foreground(fg).Background(selectedBg).Padding(0, 1)

	unselectedStyle := lipgloss.NewStyle().
		Foreground(fg).Background(dimBg).Padding(0, 1)

	qualityVal := m.settingsState.Quality
	aniskipVal := "OFF"
	if m.settingsState.AniskipEnabled {
		aniskipVal = "ON"
	}

	// Quality row
	qualityStyle := unselectedStyle
	if m.settingsState.Cursor == 0 {
		qualityStyle = selectedStyle
	}
	qualityRow := lipgloss.JoinHorizontal(lipgloss.Center,
		labelStyle.Render("Quality:"),
		qualityStyle.Render(qualityVal),
	)

	// AniSkip row
	aniskipStyle := unselectedStyle
	if m.settingsState.Cursor == 1 {
		aniskipStyle = selectedStyle
	}
	aniskipRow := lipgloss.JoinHorizontal(lipgloss.Center,
		labelStyle.Render("AniSkip:"),
		aniskipStyle.Render(aniskipVal),
	)

	boxWidth := 40
	title := titleStyle.Width(boxWidth).Render("Settings")

	content := lipgloss.JoinVertical(lipgloss.Left,
		qualityRow,
		lipgloss.NewStyle().MarginTop(1).Render(aniskipRow),
	)

	popup := lipgloss.JoinVertical(lipgloss.Center, title, lipgloss.NewStyle().MarginTop(1).Render(content))

	popupBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#9d4edd")).
		Padding(1, 3).
		Render(popup)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, popupBox)
}

func (m *SearchModel) renderLeftPanel(width int) string {
	var lines []string

	switchText := renderTextSwitch(m.searchState.TranslationType)
	qualityText := style.Faint(" | ") + style.SubTitle(m.settingsState.Quality)
	headerLine := subDubPaddingStyle.Render(switchText + qualityText)
	lines = append(lines, lipgloss.NewStyle().MaxWidth(width).Render(headerLine))
	lines = append(lines, "")
	lines = append(lines, "")
	lines = append(lines, m.searchList.View())

	return strings.Join(lines, "\n")
}

func (m *SearchModel) renderRightPanel(width int) string {
	if m.searchState.MetadataLoading {
		msg := lipgloss.JoinHorizontal(lipgloss.Center, m.loading.View(), " Loading metadata...")
		return lipgloss.NewStyle().Width(m.listWidth).AlignHorizontal(lipgloss.Center).Padding(1, 2).Render(msg)
	}

	if m.searchState.Metadata == nil {
		return lipgloss.NewStyle().Width(m.listWidth).Padding(1, 2).Render("")
	}

	lines := m.renderMetadata(width)
	// Reserve space for: title(1) + margin(1) + searchBar(3) + margin(1) + progressBar(1) + helpBar(3) + padding(2)
	maxLines := m.height - 12
	if maxLines < 1 {
		maxLines = 1
	}
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	return paddingStyle.Render(strings.Join(lines, "\n"))
}

func (m *SearchModel) renderEpisodeLeftPanel(width int) string {
	var lines []string

	switchText := renderTextSwitch(m.searchState.TranslationType)
	qualityText := style.Faint(" | ") + style.SubTitle(m.settingsState.Quality)
	headerLine := subDubPaddingStyle.Render(switchText + qualityText)
	lines = append(lines, lipgloss.NewStyle().MaxWidth(width).Render(headerLine))
	lines = append(lines, "")
	lines = append(lines, "")
	lines = append(lines, "")
	lines = append(lines, m.episodeList.View())

	return strings.Join(lines, "\n")
}

func (m *SearchModel) renderEpisodeRightPanel(width int) string {
	if m.episodeState.MetadataLoading {
		msg := lipgloss.JoinHorizontal(lipgloss.Center, m.loading.View(), " Loading episode info...")
		return lipgloss.NewStyle().Width(m.listWidth).AlignHorizontal(lipgloss.Center).Padding(1, 2).Render(msg)
	}

	if len(m.episodeState.Episodes) == 0 {
		return lipgloss.NewStyle().Width(m.listWidth).Padding(1, 2).Render(style.Faint("No episodes"))
	}

	if m.episodeState.Selected >= len(m.episodeState.Episodes) {
		return lipgloss.NewStyle().Width(m.listWidth).Padding(1, 2).Render(style.Faint("Select an episode"))
	}

	meta := m.episodeState.EpisodeMetadata
	if meta == nil {
		if m.episodeState.Selected >= len(m.episodeState.Episodes) {
			return lipgloss.NewStyle().Width(m.listWidth).Padding(1, 2).Render(style.Faint("Select an episode"))
		}
		selectedEp := m.episodeState.Episodes[m.episodeState.Selected]
		var selectedTitle string
		if len(m.episodeState.EpisodeTitles) > m.episodeState.Selected {
			selectedTitle = m.episodeState.EpisodeTitles[m.episodeState.Selected]
		}
		titleStyle := style.New().Foreground(lipgloss.Color("#9d4edd")).Bold(true).Width(m.listWidth - paddingStyle.GetHorizontalFrameSize())
		if selectedTitle != "" {
			return paddingStyle.Render(titleStyle.Render(fmt.Sprintf("Episode %s: %s", selectedEp, selectedTitle)))
		}
		return paddingStyle.Render(titleStyle.Render(fmt.Sprintf("Episode %s", selectedEp)))
	}

	lines := m.renderEpisodeMetadata(width)
	// Reserve space for: title(1) + margin(1) + searchBar(3) + margin(1) + progressBar(1) + helpBar(3) + padding(2)
	maxLines := m.height - 12
	if maxLines < 1 {
		maxLines = 1
	}
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	return paddingStyle.Render(strings.Join(lines, "\n"))
}

func (m *SearchModel) renderEpisodeMetadata(width int) []string {
	meta := m.episodeState.EpisodeMetadata
	var lines []string

	lines = append(lines, style.New().Foreground(lipgloss.Color("#9d4edd")).Bold(true).Render(meta.Title))

	if meta.TitleJapanese != "" {
		lines = append(lines, style.New().Foreground(lipgloss.Color("#9d4edd")).Italic(true).Render(meta.TitleJapanese))
	}

	var infoParts []string
	if meta.Aired != "" {
		aired := meta.Aired
		if len(aired) > 10 {
			aired = aired[:10]
		}
		infoParts = append(infoParts, fmt.Sprintf("Aired: %s", aired))
	}
	if meta.Duration > 0 {
		infoParts = append(infoParts, fmt.Sprintf("Duration: %dm", meta.Duration/60))
	}
	if meta.Score > 0 {
		infoParts = append(infoParts, fmt.Sprintf("Score: %.2f", meta.Score))
	}
	if len(infoParts) > 0 {
		lines = append(lines, style.New().Foreground(lipgloss.Color("#9d4edd")).Render(strings.Join(infoParts, " | ")))
	}

	var tags []string
	if meta.Filler {
		tags = append(tags, style.New().Foreground(lipgloss.Color("#e74c3c")).Render("Filler"))
	}
	if meta.Recap {
		tags = append(tags, style.New().Foreground(lipgloss.Color("#f39c12")).Render("Recap"))
	}
	if len(tags) > 0 {
		lines = append(lines, strings.Join(tags, "  "))
	}

	lines = append(lines, "")
	lines = append(lines, style.New().Foreground(lipgloss.Color("#9d4edd")).Bold(true).Render("Synopsis:"))
	synopsis := stripHTML(meta.Synopsis)
	if width < 60 {
		maxLines := m.height / 2
		charWidth := width - paddingStyle.GetHorizontalFrameSize()
		if charWidth < 1 {
			charWidth = 1
		}
		maxChars := maxLines * charWidth
		if maxChars > 0 && len(synopsis) > maxChars {
			synopsis = synopsis[:maxChars] + "..."
		}
	}
	synopsisStyle := style.New().Foreground(lipgloss.Color("#f4f4f6")).Width(width - paddingStyle.GetHorizontalFrameSize())
	lines = append(lines, synopsisStyle.Render(synopsis))

	return lines
}

func (m *SearchModel) renderMetadata(width int) []string {
	meta := m.searchState.Metadata
	var lines []string

	lines = append(lines, style.New().Foreground(lipgloss.Color("#9d4edd")).Bold(true).Render(meta.Title))

	if meta.TitleEnglish != "" && meta.TitleEnglish != meta.Title {
		lines = append(lines, style.New().Foreground(lipgloss.Color("#9d4edd")).Italic(true).Render(meta.TitleEnglish))
	}

	if meta.TitleNative != "" {
		lines = append(lines, style.New().Foreground(lipgloss.Color("#9d4edd")).Italic(true).Render(meta.TitleNative))
	}

	lines = append(lines, style.New().Foreground(lipgloss.Color("#9d4edd")).Render(fmt.Sprintf("Type: %s | Year: %d | Episodes: %d", meta.Type, meta.Year, meta.Episodes)))
	lines = append(lines, style.New().Foreground(lipgloss.Color("#9d4edd")).Render(fmt.Sprintf("Status: %s", meta.Status)))
	lines = append(lines, style.New().Foreground(lipgloss.Color("#9d4edd")).Render(fmt.Sprintf("Score: %.2f | Rank: #%d", meta.Score, meta.Rank)))

	if len(meta.Genres) > 0 {
		lines = append(lines, style.New().Foreground(lipgloss.Color("#9d4edd")).Render(fmt.Sprintf("Genres: %s", strings.Join(meta.Genres, ", "))))
	}

	lines = append(lines, "")
	lines = append(lines, style.New().Foreground(lipgloss.Color("#9d4edd")).Bold(true).Render("Synopsis:"))
	synopsis := stripHTML(meta.Synopsis)
	if width < 60 {
		maxLines := m.height / 2
		charWidth := width - paddingStyle.GetHorizontalFrameSize()
		if charWidth < 1 {
			charWidth = 1
		}
		maxChars := maxLines * charWidth
		if maxChars > 0 && len(synopsis) > maxChars {
			synopsis = synopsis[:maxChars] + "..."
		}
	}
	synopsisStyle := style.New().Foreground(lipgloss.Color("#f4f4f6")).Width(width - paddingStyle.GetHorizontalFrameSize())
	lines = append(lines, synopsisStyle.Render(synopsis))

	return lines
}

func (m *SearchModel) renderContent(content string) string {
	if m.height <= 0 {
		return ""
	}

	h := strings.Count(content, "\n") + 1

	var helpView string
	if m.state == confirmQuitState {
		confirmHelp := confirmKeymap{m.keymap.ConfirmYes, m.keymap.ConfirmNo}
		helpView = m.help.View(confirmHelp)
	} else if m.state == settingsState {
		sHelp := settingsKeymap{
			Up:    key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
			Down:  key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
			Left:  key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "decrease")),
			Right: key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "increase")),
			Close: key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close")),
		}
		helpView = m.help.View(sHelp)
	} else {
		helpView = m.help.View(m.keymap)
	}

	remaining := m.height - h - 2
	if remaining < 0 {
		remaining = 0
	}

	var progressBar string
	if m.searchState.Loading || m.episodeState.Loading || m.episodeState.Playing {
		progressBar = lipgloss.NewStyle().Align(lipgloss.Center).Width(m.width).Render(m.progress.ViewAs(m.progressPercent)) + "\n"
	}

	helpBox := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForegroundBlend(lipgloss.Color("#9d4edd"), lipgloss.Color("#a855f7"), lipgloss.Color("#b366ff"), lipgloss.Color("#c084fc")).Padding(0, 2).Render(helpView)
	centeredHelp := lipgloss.Place(m.width, lipgloss.Height(helpBox), lipgloss.Center, lipgloss.Top, helpBox)
	return content + strings.Repeat("\n", remaining) + progressBar + centeredHelp
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

func (m *SearchModel) filterEpisodesByNumber(query string) {
	if query == "" {
		m.updateEpisodeList()
		return
	}
	items := make([]list.Item, 0)
	for i, ep := range m.episodeState.Episodes {
		if strings.Contains(ep, query) {
			title := ""
			if len(m.episodeState.EpisodeTitles) > i {
				title = m.episodeState.EpisodeTitles[i]
			}
			items = append(items, episodeItem{number: ep, title: title})
		}
	}
	cmd := m.episodeList.SetItems(items)
	if cmd != nil {
		cmd()
	}
	if len(items) > 0 {
		m.episodeList.Select(0)
	}
}

func (m *SearchModel) selectEpisodeByNumber(query string) {
	for i, ep := range m.episodeState.Episodes {
		if ep == query || strings.Contains(ep, query) {
			m.episodeState.Selected = i
			m.episodeList.Select(i)
			return
		}
	}
}

func (m *SearchModel) doSearch(query string) tea.Cmd {
	translationType := m.searchState.TranslationType
	if translationType == "" {
		translationType = "sub"
	}
	allanimeClient := m.allanimeClient

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		shows, err := allanimeClient.SearchShows(ctx, query, 20, 1, translationType)
		if err != nil {
			return SearchErrorMsg{Err: err}
		}

		results := make([]*source.Anime, 0, len(shows))
		for _, show := range shows {
			anime := allanimeClient.MapToAnime(&show)
			results = append(results, anime)
		}

		// Sort by relevance: exact match > prefix match > contains match
		queryLower := strings.ToLower(query)
		sort.SliceStable(results, func(i, j int) bool {
			nameI := strings.ToLower(results[i].Name)
			nameJ := strings.ToLower(results[j].Name)
			scoreI := matchScore(nameI, queryLower)
			scoreJ := matchScore(nameJ, queryLower)
			return scoreI > scoreJ
		})

		return SearchResultsMsg{Results: results}
	}
}

func (m *SearchModel) fetchMetadata() tea.Cmd {
	if len(m.searchState.Results) == 0 || m.searchState.Selected >= len(m.searchState.Results) {
		m.searchState.Metadata = nil
		m.searchState.MetadataLoading = false
		return nil
	}

	anime := m.searchState.Results[m.searchState.Selected]

	hasMalID := anime.MALID > 0
	hasAniListID := anime.AniListID > 0

	if !hasMalID && !hasAniListID {
		m.searchState.Metadata = nil
		m.searchState.MetadataLoading = false
		return nil
	}

	if hasMalID && m.jikanClient.IsCached(anime.MALID) && (!hasAniListID || m.anilistClient.IsCached(anime.AniListID)) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var jikanData *jikan.AnimeData
		var anilistData *anilist.MediaData

		if hasMalID {
			data, err := m.jikanClient.GetAnime(ctx, anime.MALID)
			if err == nil {
				jikanData = data
			}
		}
		if hasAniListID {
			data, err := m.anilistClient.GetAnime(ctx, anime.AniListID)
			if err == nil {
				anilistData = data
			}
		}

		if jikanData != nil || anilistData != nil {
			m.searchState.Metadata = mergeMetadata(jikanData, anilistData)
			m.searchState.MetadataLoading = false
			return nil
		}
	}

	m.searchState.MetadataLoading = true
	m.loading = spinner.New()
	m.loading.Spinner = spinner.Points
	m.loading.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#9d4edd"))
	idx := m.searchState.Selected

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var jikanData *jikan.AnimeData
		var anilistData *anilist.MediaData

		var wg sync.WaitGroup

		if hasMalID {
			wg.Add(1)
			go func() {
				defer wg.Done()
				data, err := m.jikanClient.GetAnime(ctx, anime.MALID)
				if err == nil {
					jikanData = data
				}
			}()
		}

		if hasAniListID {
			wg.Add(1)
			go func() {
				defer wg.Done()
				data, err := m.anilistClient.GetAnime(ctx, anime.AniListID)
				if err == nil {
					anilistData = data
				}
			}()
		}

		wg.Wait()

		if jikanData == nil && anilistData == nil {
			return MetadataLoadedMsg{Metadata: nil, Index: idx}
		}

		return MetadataLoadedMsg{Metadata: mergeMetadata(jikanData, anilistData), Index: idx}
	}
}

func mergeMetadata(jikanData *jikan.AnimeData, anilistData *anilist.MediaData) *MetadataPanel {
	panel := &MetadataPanel{}

	var sources []string
	if jikanData != nil {
		sources = append(sources, "Jikan")
	}
	if anilistData != nil {
		sources = append(sources, "AniList")
	}
	if len(sources) > 1 {
		panel.Source = "Jikan + AniList"
	} else if len(sources) == 1 {
		panel.Source = sources[0]
	}

	if anilistData != nil {
		panel.Title = anilistData.Title.Romaji
		panel.TitleEnglish = anilistData.Title.English
		panel.TitleNative = anilistData.Title.Native
		panel.Cover = anilistData.CoverImage.Large
		if panel.Cover == "" {
			panel.Cover = anilistData.CoverImage.Medium
		}
		if anilistData.Format != "" {
			panel.Type = anilistData.Format
		} else {
			panel.Type = anilistData.Type
		}
		panel.Status = anilistData.Status
		panel.Episodes = anilistData.Episodes
		panel.Score = float64(anilistData.AverageScore) / 10
		panel.Popularity = anilistData.Popularity
		panel.Genres = anilistData.Genres
		panel.Synopsis = anilistData.Description
		if anilistData.StartDate.Year != 0 {
			panel.Year = anilistData.StartDate.Year
		} else if anilistData.SeasonYear != 0 {
			panel.Year = anilistData.SeasonYear
		}
	}

	if jikanData != nil {
		if panel.Title == "" {
			if jikanData.TitleEnglish != "" {
				panel.Title = jikanData.TitleEnglish
			} else {
				panel.Title = jikanData.Title
			}
		}
		if panel.TitleEnglish == "" && jikanData.TitleEnglish != "" {
			panel.TitleEnglish = jikanData.TitleEnglish
		}
		if panel.Cover == "" {
			panel.Cover = jikanData.Images.JPG.LargeImageURL
			if panel.Cover == "" {
				panel.Cover = jikanData.Images.JPG.ImageURL
			}
		}
		if panel.Type == "" {
			panel.Type = jikanData.Type
		}
		if panel.Status == "" {
			panel.Status = jikanData.Status
		}
		if panel.Episodes == 0 {
			panel.Episodes = jikanData.Episodes
		}
		if panel.Score == 0 {
			panel.Score = jikanData.Score
		}
		if panel.Rank == 0 {
			panel.Rank = jikanData.Rank
		}
		if panel.Popularity == 0 {
			panel.Popularity = jikanData.Popularity
		}
		if panel.Year == 0 {
			if y, ok := jikanData.Year.(float64); ok {
				panel.Year = int(y)
			}
		}
		if len(panel.Genres) == 0 {
			genres := make([]string, 0, len(jikanData.Genres))
			for _, g := range jikanData.Genres {
				genres = append(genres, g.Name)
			}
			panel.Genres = genres
		}
		if panel.Synopsis == "" {
			panel.Synopsis = jikanData.Synopsis
		}
	}

	return panel
}

func (m *SearchModel) prefetchMetadata(ctx context.Context, start, end int) {
	results := m.searchState.Results
	if end > len(results) {
		end = len(results)
	}
	anilistClient := m.anilistClient
	jikanClient := m.jikanClient

	anilistIDs := make([]int, 0)
	anilistIndexMap := make(map[int]int)
	for i := start; i < end; i++ {
		select {
		case <-ctx.Done():
			return
		default:
		}
		anime := results[i]
		if anime.AniListID > 0 && !anilistClient.IsCached(anime.AniListID) {
			anilistIDs = append(anilistIDs, anime.AniListID)
			anilistIndexMap[anime.AniListID] = i
		}
	}

	if len(anilistIDs) > 0 {
		batchSize := 10
		for i := 0; i < len(anilistIDs); i += batchSize {
			select {
			case <-ctx.Done():
				return
			default:
			}
			batchEnd := i + batchSize
			if batchEnd > len(anilistIDs) {
				batchEnd = len(anilistIDs)
			}
			batch := anilistIDs[i:batchEnd]
			batchCtx, batchCancel := context.WithTimeout(ctx, 10*time.Second)
			_, _ = anilistClient.GetAnimeBatch(batchCtx, batch)
			batchCancel()
		}
	}

	malIDs := make([]int, 0)
	for i := start; i < end; i++ {
		select {
		case <-ctx.Done():
			return
		default:
		}
		anime := results[i]
		if anime.MALID > 0 && !jikanClient.IsCached(anime.MALID) {
			malIDs = append(malIDs, anime.MALID)
		}
	}

	for _, malID := range malIDs {
		select {
		case <-ctx.Done():
			return
		default:
		}
		dataCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, _ = jikanClient.GetAnime(dataCtx, malID)
		cancel()
		time.Sleep(350 * time.Millisecond)
	}

	newTitles := make(map[int][]string)
	for i := start; i < end; i++ {
		select {
		case <-ctx.Done():
			return
		default:
		}
		anime := results[i]
		if anime.MALID == 0 {
			continue
		}
		dataCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		episodes, err := jikanClient.GetEpisodes(dataCtx, anime.MALID)
		cancel()
		if err == nil && len(episodes) > 0 {
			titles := make([]string, len(episodes))
			for j, ep := range episodes {
				titles[j] = ep.Title
			}
			newTitles[i] = titles
		}
		time.Sleep(350 * time.Millisecond)
	}

	if len(newTitles) > 0 {
		m.episodeTitlesCacheMu.Lock()
		for i, titles := range newTitles {
			m.episodeTitlesCache[i] = titles
		}
		m.episodeTitlesCacheMu.Unlock()
	}

	m.metadataFetchChan <- -1
}

func (m *SearchModel) fetchEpisodes(showID string, malID int) tea.Cmd {
	translationType := m.searchState.TranslationType
	if translationType == "" {
		translationType = "sub"
	}
	searchSelected := m.searchState.Selected
	allanimeClient := m.allanimeClient
	jikanClient := m.jikanClient

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		episodes, err := allanimeClient.GetShowEpisodes(ctx, showID, translationType)
		if err != nil {
			return EpisodesLoadedMsg{Episodes: nil, EpisodeTitles: nil, Error: err}
		}

		epList, ok := episodes[translationType]
		if !ok {
			epList, ok = episodes["sub"]
			if !ok {
				return EpisodesLoadedMsg{Episodes: nil, EpisodeTitles: nil, Error: fmt.Errorf("no episodes found for %s", translationType)}
			}
		}

		m.episodeTitlesCacheMu.Lock()
		var episodeTitles []string
		if titles, ok := m.episodeTitlesCache[searchSelected]; ok {
			episodeTitles = titles
		}
		m.episodeTitlesCacheMu.Unlock()
		if len(episodeTitles) == 0 && malID > 0 {
			jikanEpisodes, err := jikanClient.GetEpisodes(ctx, malID)
			if err == nil && len(jikanEpisodes) > 0 {
				episodeTitles = make([]string, len(jikanEpisodes))
				for i, ep := range jikanEpisodes {
					episodeTitles[i] = ep.Title
				}
			}
		}

		return EpisodesLoadedMsg{Episodes: epList, EpisodeTitles: episodeTitles, Error: nil}
	}
}

func (m *SearchModel) fetchEpisodeMetadata() tea.Cmd {
	if len(m.episodeState.Episodes) == 0 || m.episodeState.Selected >= len(m.episodeState.Episodes) {
		m.episodeState.EpisodeMetadata = nil
		m.episodeState.MetadataLoading = false
		return nil
	}

	if len(m.searchState.Results) == 0 || m.searchState.Selected >= len(m.searchState.Results) {
		m.episodeState.EpisodeMetadata = nil
		m.episodeState.MetadataLoading = false
		return nil
	}

	anime := m.searchState.Results[m.searchState.Selected]
	if anime == nil || anime.MALID == 0 {
		m.episodeState.EpisodeMetadata = nil
		m.episodeState.MetadataLoading = false
		return nil
	}

	epNumStr := m.episodeState.Episodes[m.episodeState.Selected]
	cacheKey := fmt.Sprintf("%d-%s", anime.MALID, epNumStr)

	if cached, ok := m.episodeMetadataCache[cacheKey]; ok {
		m.episodeState.EpisodeMetadata = buildEpisodeMetadataPanel(cached)
		m.episodeState.MetadataLoading = false
		return nil
	}

	epNum, err := strconv.ParseFloat(epNumStr, 64)
	if err != nil {
		m.episodeState.EpisodeMetadata = nil
		m.episodeState.MetadataLoading = false
		return nil
	}

	idx := m.episodeState.Selected
	malID := anime.MALID
	jikanClient := m.jikanClient

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		episode, err := jikanClient.GetEpisode(ctx, malID, int(epNum))
		if err != nil {
			return EpisodeMetadataLoadedMsg{Metadata: nil, Index: idx}
		}

		return EpisodeMetadataLoadedMsg{Metadata: buildEpisodeMetadataPanel(episode), Index: idx, CacheKey: cacheKey, RawEpisode: episode}
	}
}

func buildEpisodeMetadataPanel(ep *jikan.Episode) *EpisodeMetadataPanel {
	return &EpisodeMetadataPanel{
		Title:         ep.Title,
		TitleJapanese: ep.TitleJapanese,
		Aired:         ep.Aired,
		Score:         ep.Score,
		Filler:        ep.Filler,
		Recap:         ep.Recap,
		Synopsis:      ep.Synopsis,
		Duration:      ep.Duration,
	}
}

func (m *SearchModel) playEpisode(showID, episodeNum, animeTitle string, malID int) tea.Cmd {
	translationType := m.searchState.TranslationType
	if translationType == "" {
		translationType = "sub"
	}
	allanimeProvider := m.allanimeProvider
	aniskipEnabled := m.settingsState.AniskipEnabled
	quality := m.settingsState.Quality

	return func() tea.Msg {
		allanimeProvider.SetTranslation(translationType)

		episodeNumFloat, _ := strconv.ParseFloat(episodeNum, 64)
		episode := &source.Episode{
			Number: episodeNumFloat,
			Anime: &source.Anime{
				AllAnimeID: showID,
				Name:       animeTitle,
			},
		}

		streams, err := allanimeProvider.StreamsOf(episode)
		if err != nil {
			return TUIErrorMsg{Err: fmt.Errorf("failed to get streams: %w", err)}
		}

		if len(streams) == 0 {
			return TUIErrorMsg{Err: fmt.Errorf("no streams found")}
		}

		// Fetch AniSkip skip times if enabled and MAL ID is available
		var skipTimes []aniskip.SkipInterval
		if aniskipEnabled && malID > 0 {
			epNum, _ := strconv.Atoi(episodeNum)
			if times, err := aniskip.GetSkipTimes(malID, epNum); err == nil {
				skipTimes = times
			}
		}

		playStream := tryPlayStream(streams, animeTitle, episodeNum, skipTimes, quality)
		if playStream == nil {
			return TUIErrorMsg{Err: fmt.Errorf("no playable stream found")}
		}

		return PlayStreamMsg{}
	}
}

func filterByQuality(streams []*source.Stream, quality string) []*source.Stream {
	if quality == "" || quality == "auto" {
		return streams
	}

	// "best" returns all streams, let the player handle quality selection
	if quality == "best" {
		return streams
	}

	target := parseQualityNum(quality)

	// Exact match
	var exact []*source.Stream
	for _, s := range streams {
		if s.Quality == quality {
			exact = append(exact, s)
		}
	}
	if len(exact) > 0 {
		return exact
	}

	// Closest match (prefer lower)
	var closest []*source.Stream
	bestDiff := int(^uint(0) >> 1) // max int
	for _, s := range streams {
		q := parseQualityNum(s.Quality)
		diff := q - target
		if diff < 0 {
			diff = -diff
		}
		if diff < bestDiff {
			bestDiff = diff
			closest = []*source.Stream{s}
		} else if diff == bestDiff {
			closest = append(closest, s)
		}
	}
	return closest
}

func parseQualityNum(q string) int {
	q = strings.TrimSuffix(q, "p")
	n, _ := strconv.Atoi(q)
	if n == 0 {
		return 9999 // "auto" or unknown = highest
	}
	return n
}

func tryPlayStream(streams []*source.Stream, animeTitle, episodeNum string, skipTimes []aniskip.SkipInterval, quality string) *source.Stream {
	d := &player.Detector{}

	// Filter by quality if not "auto"
	filtered := filterByQuality(streams, quality)
	if len(filtered) == 0 {
		filtered = streams
	}

	// On Android, sort streams: no-referrer first (more likely to work)
	ordered := make([]*source.Stream, len(filtered))
	copy(ordered, filtered)
	if player.IsAndroid() {
		sort.SliceStable(ordered, func(i, j int) bool {
			return !ordered[i].NeedsReferrer && ordered[j].NeedsReferrer
		})
	}

	// Convert aniskip intervals to player skip intervals
	var playerSkips []player.SkipInterval
	for _, st := range skipTimes {
		playerSkips = append(playerSkips, player.SkipInterval{
			Start: st.StartTime,
			End:   st.EndTime,
			Type:  st.Type,
		})
	}

	for _, s := range ordered {
		url := s.URL
		if strings.HasPrefix(url, "//") {
			url = "https:" + url
		}

		p := d.PreferredForReferrer(s.NeedsReferrer)
		opts := player.Options{
			Title:     fmt.Sprintf("%s - Episode %s", animeTitle, episodeNum),
			Referrer:  s.Referer,
			SkipTimes: playerSkips,
		}
		for _, sub := range s.Subtitles {
			opts.Subtitles = append(opts.Subtitles, sub.URL)
		}

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
		BorderForeground(lipgloss.Color("#9d4edd")).
		Foreground(lipgloss.Color("#9d4edd")).
		Padding(0, 0, 0, 1)

	delegate.Styles.SelectedDesc = lipgloss.NewStyle().
		Border(lipgloss.ThickBorder(), false, false, false, true).
		BorderForeground(lipgloss.Color("#9d4edd")).
		Foreground(lipgloss.Color("#f4f4f6")).
		Padding(0, 0, 0, 1)

	delegate.Styles.NormalTitle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#f4f4f6")).
		Padding(0, 0, 0, 1)

	delegate.Styles.NormalDesc = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9d4edd")).
		Padding(0, 0, 0, 1)

	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Title = title
	l.Styles.Title = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9d4edd")).
		Background(lipgloss.Color("#f4f4f6")).
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

var htmlTagRe = regexp.MustCompile(`<[^>]*>`)

func stripHTML(s string) string {
	s = strings.ReplaceAll(s, "<br>", "\n")
	s = strings.ReplaceAll(s, "<br/>", "\n")
	s = strings.ReplaceAll(s, "<br />", "\n")
	return htmlTagRe.ReplaceAllString(s, "")
}

func matchScore(name, query string) int {
	if name == query {
		return 4 // exact match
	}
	if strings.HasPrefix(name, query) {
		return 3 // prefix match
	}
	if strings.Contains(name, query) {
		return 2 // contains match
	}
	// Check if all query words appear in name
	queryWords := strings.Fields(query)
	if len(queryWords) > 1 {
		allMatch := true
		for _, w := range queryWords {
			if !strings.Contains(name, w) {
				allMatch = false
				break
			}
		}
		if allMatch {
			return 1
		}
	}
	return 0
}

type MetadataFetchTriggered struct {
	Index int
}

type SearchResultsMsg struct {
	Results []*source.Anime
}

type MetadataLoadedMsg struct {
	Metadata *MetadataPanel
	Index    int
}

type EpisodesLoadedMsg struct {
	Episodes      []string
	EpisodeTitles []string
	Error         error
}

type TUIErrorMsg struct {
	Err error
}

type PlayStreamMsg struct{}

type SearchErrorMsg struct {
	Err error
}

type progressTickMsg struct{}

func RunSearch() (*SelectionResult, error) {
	model := NewSearchModel()
	p := tea.NewProgram(model)

	_, err := p.Run()
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
