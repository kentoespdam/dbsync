package tui

import (
	"context"
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/table"
	"github.com/user/dbsync/internal/storage"
)

type connListModel struct {
	table  table.Model
	store  *storage.DB
	conns  []storage.Connection
	err    error
	inited bool
}

func newConnListModel(db *storage.DB) connListModel {
	columns := []table.Column{
		{Title: "NAME", Width: 15},
		{Title: "SOURCE", Width: 20},
		{Title: "DEST", Width: 20},
		{Title: "UPDATED", Width: 20},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.Bold(true)
	t.SetStyles(s)

	return connListModel{
		table:  t,
		store:  db,
		inited: true,
	}
}

func (m connListModel) Init() tea.Cmd {
	return m.refresh()
}

func (m connListModel) refresh() tea.Cmd {
	return func() tea.Msg {
		conns, err := m.store.Connections().List(context.Background())
		if err != nil {
			return err
		}
		return conns
	}
}

func (m connListModel) Update(msg tea.Msg) (connListModel, tea.Cmd) {
	switch msg := msg.(type) {
	case []storage.Connection:
		m.conns = msg
		rows := make([]table.Row, len(m.conns))
		for i, c := range m.conns {
			rows[i] = table.Row{
				c.Name,
				fmt.Sprintf("%s:%d", c.SourceHost, c.SourcePort),
				fmt.Sprintf("%s:%d", c.DestHost, c.DestPort),
				c.UpdatedAt.Format("2006-01-02 15:04"),
			}
		}
		m.table.SetRows(rows)
		return m, nil

	case error:
		m.err = msg
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			return m, m.refresh()
		case "n":
			// Will be handled in app.go by checking state? 
			// Or we signal app.go via a message.
		}
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m connListModel) View() string {
	if m.err != nil {
		return errorStyle.Render("Error: " + m.err.Error())
	}

	return fmt.Sprintf(
		"%s\n\n%s",
		m.table.View(),
		helpStyle.Render("n: new • enter: edit • t: test • d: delete • r: reload"),
	)
}
