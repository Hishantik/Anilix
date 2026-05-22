package style

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

func New() lipgloss.Style {
	return lipgloss.NewStyle()
}

func NewColored(foreground, background color.Color) lipgloss.Style {
	return New().Foreground(foreground).Background(background)
}

func Fg(c color.Color) func(string) string {
	s := NewColored(c, nil)
	return func(str string) string { return s.Render(str) }
}

func Bg(c color.Color) func(string) string {
	s := NewColored(nil, c)
	return func(str string) string { return s.Render(str) }
}

func Truncate(max int) func(string) string {
	s := New().MaxWidth(max)
	return func(str string) string { return s.Render(str) }
}

func Faint(str string) string {
	return New().Faint(true).Render(str)
}

func Bold(str string) string {
	return New().Bold(true).Render(str)
}

func Italic(str string) string {
	return New().Italic(true).Render(str)
}

func Underline(str string) string {
	return New().Underline(true).Render(str)
}
