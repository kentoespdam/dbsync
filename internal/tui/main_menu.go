package tui

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

type item string

func (i item) FilterValue() string { return string(i) }
func (i item) Title() string       { return string(i) }
func (i item) Description() string {
	switch string(i) {
	case "Connections":
		return "Manage MySQL source and destination connections"
	case "Tables & Mappings":
		return "Configure table and column sync mappings"
	case "Sync":
		return "Run synchronization jobs"
	case "History":
		return "View sync history and logs"
	case "Checkpoints":
		return "View and resume interrupted syncs"
	case "Quit":
		return "Exit the application"
	default:
		return ""
	}
}

type mainMenuModel struct {
	list   list.Model
	choice string
	inited bool
}

func newMainMenuModel() mainMenuModel {
	items := []list.Item{
		item("Connections"),
		item("Tables & Mappings"),
		item("Sync"),
		item("History"),
		item("Checkpoints"),
		item("Quit"),
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "dbsync - Main Menu"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)

	return mainMenuModel{
		list:   l,
		inited: true,
	}
}

func (m mainMenuModel) Init() tea.Cmd {
	return nil
}

func (m mainMenuModel) Update(msg tea.Msg) (mainMenuModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height)
	case tea.KeyMsg:
		if msg.String() == "enter" {
			if i, ok := m.list.SelectedItem().(item); ok {
				m.choice = string(i)
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m mainMenuModel) View() string {
	return m.list.View()
}
