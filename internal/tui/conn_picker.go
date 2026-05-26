package tui

import (
	"context"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/dbsync/internal/storage"
)

type connItem struct {
	conn storage.Connection
}

func (i connItem) Title() string       { return i.conn.Name }
func (i connItem) Description() string { return i.conn.SourceHost + " -> " + i.conn.DestHost }
func (i connItem) FilterValue() string { return i.conn.Name }

type connPickerModel struct {
	list   list.Model
	store  *storage.DB
	err    error
	choice *storage.Connection
}

func newConnPickerModel(store *storage.DB) connPickerModel {
	l := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Select Connection"
	l.SetShowStatusBar(false)

	return connPickerModel{
		list:  l,
		store: store,
	}
}

func (m connPickerModel) Init() tea.Cmd {
	return m.loadConns
}

func (m connPickerModel) loadConns() tea.Msg {
	conns, err := m.store.Connections().List(context.Background())
	if err != nil {
		return err
	}
	var items []list.Item
	for _, c := range conns {
		items = append(items, connItem{conn: c})
	}
	return connsLoadedMsg{items}
}

type connsLoadedMsg struct {
	items []list.Item
}

func (m connPickerModel) Update(msg tea.Msg) (connPickerModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height-2)

	case connsLoadedMsg:
		m.list.SetItems(msg.items)
		return m, nil

	case error:
		m.err = msg
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if i, ok := m.list.SelectedItem().(connItem); ok {
				m.choice = &i.conn
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m connPickerModel) View() string {
	if m.err != nil {
		return errorStyle.Render("Error: " + m.err.Error())
	}
	return m.list.View()
}
