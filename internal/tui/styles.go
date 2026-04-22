package tui

import "github.com/charmbracelet/lipgloss"

var (
	Orange = lipgloss.Color("#FF8C00")
	Grey   = lipgloss.Color("#3C3C3C")
	White  = lipgloss.Color("#FFFFFF")

	HeaderStyle = lipgloss.NewStyle().
			Foreground(White).
			Background(Orange).
			Padding(0, 1).
			Bold(true)

	SelectedStyle = lipgloss.NewStyle().
			Foreground(Orange).
			Bold(true)

	UnselectedStyle = lipgloss.NewStyle().
			Foreground(White)

	PaneStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(Grey).
			Padding(1)

	ActivePaneStyle = lipgloss.NewStyle().
			Border(lipgloss.ThickBorder()).
			BorderForeground(Orange).
			Padding(1)

	TitleStyle = lipgloss.NewStyle().
			Foreground(Orange).
			Bold(true).
			MarginBottom(1)
)
