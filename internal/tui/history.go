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
	store      *storage.DB
	conn       *storage.Connection // if nil, show all
	table      table.Model
	records    []storage.HistoryRecord
	conns      map[int64]storage.Connection
	width      int
	height     int
	showDetail bool
	selected   *storage.HistoryRecord
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

	if m.showDetail {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "esc", "q", "enter":
				m.showDetail = false
				m.selected = nil
				return m, nil
			}
		}
		return m, nil
	}

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
			status := r.Status
			switch status {
			case "completed":
				status = "✓ " + status
			case "failed":
				status = "✗ " + status
			case "interrupted":
				status = "⚠ " + status
			}
			rows[i] = table.Row{
				r.StartedAt.Format("2006-01-02 15:04"),
				duration,
				r.TableName,
				strconv.FormatInt(r.TotalRows.Int64, 10),
				status,
				r.ErrorSummary.String,
			}
		}
		m.table.SetRows(rows)

	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			idx := m.table.Cursor()
			if idx >= 0 && idx < len(m.records) {
				m.selected = &m.records[idx]
				m.showDetail = true
				return m, nil
			}
		}
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m historyModel) View() string {
	if m.showDetail && m.selected != nil {
		r := m.selected
		titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62")).Underline(true).MarginBottom(1)
		labelStyle := lipgloss.NewStyle().Bold(true).Width(15)

		connName := "Unknown"
		if c, ok := m.conns[r.ConnectionID]; ok {
			connName = c.Name
		}

		statusColor := "15"
		switch r.Status {
		case "completed": statusColor = "10"
		case "failed": statusColor = "9"
		case "interrupted": statusColor = "11"
		}
		statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(statusColor)).Bold(true)

		content := lipgloss.JoinVertical(lipgloss.Left,
			titleStyle.Render("Sync Detail"),
			fmt.Sprintf("%s %s", labelStyle.Render("Connection:"), connName),
			fmt.Sprintf("%s %s", labelStyle.Render("Table:"), r.TableName),
			fmt.Sprintf("%s %s", labelStyle.Render("Status:"), statusStyle.Render(r.Status)),
			fmt.Sprintf("%s %s", labelStyle.Render("Started:"), r.StartedAt.Format("2006-01-02 15:04:05")),
			fmt.Sprintf("%s %s", labelStyle.Render("Finished:"), r.FinishedAt.Time.Format("2006-01-02 15:04:05")),
			fmt.Sprintf("%s %ds", labelStyle.Render("Duration:"), r.DurationSeconds.Int64),
			fmt.Sprintf("%s %d", labelStyle.Render("Total Rows:"), r.TotalRows.Int64),
			fmt.Sprintf("%s %d", labelStyle.Render("Success:"), r.SuccessRows.Int64),
			fmt.Sprintf("%s %d", labelStyle.Render("Failed:"), r.FailedRows.Int64),
		)

		if r.ErrorSummary.Valid && r.ErrorSummary.String != "" {
			content = lipgloss.JoinVertical(lipgloss.Left, content,
				fmt.Sprintf("\n%s\n%s", lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9")).Render("Error:"), r.ErrorSummary.String),
			)
		}

		logPath := fmt.Sprintf("logs/%s/%s.jsonl", connName, r.TableName)
		content = lipgloss.JoinVertical(lipgloss.Left, content,
			fmt.Sprintf("\n%s %s", labelStyle.Render("Log Path:"), logPath),
		)

		detailBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(1, 2).
			Width(m.width - 4).
			Render(content)

		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, detailBox)
	}

	title := "Sync History"
	if m.conn != nil {
		title = fmt.Sprintf("Sync History: %s", m.conn.Name)
	}

	header := lipgloss.NewStyle().Bold(true).Padding(0, 1).Render(title)

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		m.table.View(),
		"\n ↑/↓: navigate • enter: detail • q/esc: back",
	)
}
