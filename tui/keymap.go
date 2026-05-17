package tui

import "github.com/charmbracelet/bubbles/key"

type keymap struct {
	Up       key.Binding
	Down     key.Binding
	Select   key.Binding
	Back     key.Binding
	Quit     key.Binding
	Toggle   key.Binding
	Search   key.Binding
}

func newKeymap() keymap {
	return keymap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Select: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "quit"),
		),
		Toggle: key.NewBinding(
			key.WithKeys("ctrl+t"),
			key.WithHelp("ctrl+t", "toggle sub/dub"),
		),
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search"),
		),
	}
}

func (k keymap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Select, k.Back, k.Quit}
}

func (k keymap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Select},
		{k.Back, k.Quit, k.Toggle},
	}
}
