package style

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

var (
	Title      = NewColored(lipgloss.Color("#9d4edd"), lipgloss.Color("#f4f4f6")).Padding(0, 1).Render
	ErrorTitle = NewColored(lipgloss.Color("#9d4edd"), lipgloss.Color("#f4f4f6")).Padding(0, 1).Render
	SubTitle   = NewColored(lipgloss.Color("#f4f4f6"), lipgloss.Color("#9d4edd")).Padding(0, 1).Render
	DubTitle   = NewColored(lipgloss.Color("#f4f4f6"), lipgloss.Color("#9d4edd")).Padding(0, 1).Render
)

func Tag(foreground, background color.Color) func(string) string {
	s := NewColored(foreground, background).Padding(0, 1)
	return func(str string) string { return s.Render(str) }
}
