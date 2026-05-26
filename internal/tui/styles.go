package tui

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("36")) // Cyan
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))            // Red
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))           // Green
	helpStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	borderStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder())
)
