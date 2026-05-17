package style

import "github.com/charmbracelet/lipgloss"

var (
	Title      = NewColored("#F2BB05", "#3D348B").Padding(0, 1).Render
	ErrorTitle = NewColored("#E2294F", "#3D348B").Padding(0, 1).Render
	SubTitle   = NewColored("#EAEAEA", "#3D348B").Padding(0, 1).Render
	DubTitle   = NewColored("#EAEAEA", "#F2BB05").Padding(0, 1).Render
)

func Tag(foreground, background lipgloss.Color) func(string) string {
	s := NewColored(foreground, background).Padding(0, 1)
	return func(str string) string { return s.Render(str) }
}
