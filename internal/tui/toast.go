package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type toastMsg struct {
	message string
	timeout time.Duration
}

type clearToastMsg struct{}

func showToast(msg string, timeout time.Duration) tea.Cmd {
	return func() tea.Msg {
		return toastMsg{message: msg, timeout: timeout}
	}
}

func (m model) renderToast() string {
	if m.toastMsg == "" {
		return ""
	}

	bgColor := "62" // Purple-ish
	if strings.HasPrefix(m.toastMsg, "Error:") {
		bgColor = "9" // Red
	}

	style := lipgloss.NewStyle().
		Padding(0, 1).
		Background(lipgloss.Color(bgColor)).
		Foreground(lipgloss.Color("255")).
		Bold(true).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("255"))

	return style.Render(m.toastMsg)
}
