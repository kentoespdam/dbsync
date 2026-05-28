package tui

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("36")) // Cyan
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))            // Red
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))           // Green
	helpStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	borderStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder())

	// Badges & Status
	mappedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))  // Green
	unmappedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240")) // Grey
	warningStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("208")) // Orange
	infoStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("44"))  // Cyan

	filterFocusStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("170"))
	filterBlurStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240"))
)
