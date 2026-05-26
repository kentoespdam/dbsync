package tui

import (
	"context"
	"fmt"
	"strconv"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/user/dbsync/internal/storage"
)

type historyModel struct {
	store    *storage.DB
	conn     *storage.Connection // if nil, show all
	table    table.Model
	records  []storage.HistoryRecord
	conns    map[int64]storage.Connection
	width    int
	height   int
}

func newHistoryModel(store *storage.DB, conn *storage.Connection) historyModel {
	columns := []table.Column{
		{Title: "STARTED", Width: 20},
		{Title: "DURATION", Width: 10},
		{Title: "TABLE", Width: 15},
		{Title: "ROWS", Width: 10},
		{Title: "STATUS", Width: 12},
		{Title: "ERROR", Width: 30},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		Bold(true)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(true)
	t.SetStyles(s)

	return historyModel{
		store: store,
		conn:  conn,
		table: t,
		conns: make(map[int64]storage.Connection),
	}
}

func (m historyModel) Init() tea.Cmd {
	return m.refresh()
}

func (m historyModel) refresh() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		
		// Load connections for names
		conns, _ := m.store.Connections().List(ctx)
		connMap := make(map[int64]storage.Connection)
		for _, c := range conns {
			connMap[c.ID] = c
		}

		var records []storage.HistoryRecord
		var err error
		if m.conn != nil {
			records, err = m.store.History().ListByConnection(ctx, m.conn.ID, 100)
		} else {
			records, err = m.store.History().ListAll(ctx, 100)
		}
		
		if err != nil {
			return errorMsg(err)
		}

		return historyLoadedMsg{records: records, conns: connMap}
	}
}

type historyLoadedMsg struct {
	records []storage.HistoryRecord
	conns   map[int64]storage.Connection
}

func (m historyModel) Update(msg tea.Msg) (historyModel, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.table.SetWidth(msg.Width)
		m.table.SetHeight(msg.Height - 6)

	case historyLoadedMsg:
		m.records = msg.records
		m.conns = msg.conns
		rows := make([]table.Row, len(m.records))
		for i, r := range m.records {
			duration := "-"
			if r.DurationSeconds.Valid {
				duration = fmt.Sprintf("%ds", r.DurationSeconds.Int64)
			}
			rows[i] = table.Row{
				r.StartedAt.Format("2006-01-02 15:04"),
				duration,
				r.TableName,
				strconv.FormatInt(r.TotalRows.Int64, 10),
				r.Status,
				r.ErrorSummary.String,
			}
		}
		m.table.SetRows(rows)
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m historyModel) View() string {
	title := "Sync History"
	if m.conn != nil {
		title = fmt.Sprintf("Sync History: %s", m.conn.Name)
	}

	header := lipgloss.NewStyle().Bold(true).Padding(0, 1).Render(title)
	
	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		m.table.View(),
		"\n ↑/↓: navigate • q/esc: back",
	)
}
