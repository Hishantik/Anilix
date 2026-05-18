package style

import "github.com/charmbracelet/lipgloss"

var (
	Title      = NewColored("#9d4edd", "#f4f4f6").Padding(0, 1).Render
	ErrorTitle = NewColored("#9d4edd", "#f4f4f6").Padding(0, 1).Render
	SubTitle   = NewColored("#f4f4f6", "#9d4edd").Padding(0, 1).Render
	DubTitle   = NewColored("#f4f4f6", "#9d4edd").Padding(0, 1).Render
)

func Tag(foreground, background lipgloss.Color) func(string) string {
	s := NewColored(foreground, background).Padding(0, 1)
	return func(str string) string { return s.Render(str) }
}
