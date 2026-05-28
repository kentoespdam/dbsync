package tui

import (
	"context"
	"fmt"
	"strconv"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kentoespdam/dbsync/internal/storage"
)

type checkpointsModel struct {
	store       *storage.DB
	table       table.Model
	checkpoints []storage.Checkpoint
	conns       map[int64]storage.Connection
	width       int
	height      int
	confirmDel  bool
	selectedCP  *storage.Checkpoint
}

func newCheckpointsModel(store *storage.DB) checkpointsModel {
	columns := []table.Column{
		{Title: "CONNECTION", Width: 20},
		{Title: "TABLE", Width: 20},
		{Title: "STATUS", Width: 12},
		{Title: "BATCH", Width: 10},
		{Title: "UPDATED", Width: 20},
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

	return checkpointsModel{
		store: store,
		table: t,
		conns: make(map[int64]storage.Connection),
	}
}

func (m checkpointsModel) Init() tea.Cmd {
	return m.refresh()
}

func (m checkpointsModel) refresh() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		
		conns, _ := m.store.Connections().List(ctx)
		connMap := make(map[int64]storage.Connection)
		for _, c := range conns {
			connMap[c.ID] = c
		}

		checkpoints, err := m.store.Checkpoints().ListActive(ctx)
		if err != nil {
			return errorMsg(err)
		}

		return checkpointsLoadedMsg{checkpoints: checkpoints, conns: connMap}
	}
}

type checkpointsLoadedMsg struct {
	checkpoints []storage.Checkpoint
	conns       map[int64]storage.Connection
}

func (m checkpointsModel) Update(msg tea.Msg) (checkpointsModel, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.table.SetWidth(msg.Width)
		m.table.SetHeight(msg.Height - 6)

	case checkpointsLoadedMsg:
		m.checkpoints = msg.checkpoints
		m.conns = msg.conns
		rows := make([]table.Row, len(m.checkpoints))
		for i, c := range m.checkpoints {
			connName := strconv.FormatInt(c.ConnectionID, 10)
			if conn, ok := m.conns[c.ConnectionID]; ok {
				connName = conn.Name
			}
			rows[i] = table.Row{
				connName,
				c.TableName,
				c.Status,
				strconv.Itoa(c.LastBatchCompleted),
				c.UpdatedAt.Format("2006-01-02 15:04"),
			}
		}
		m.table.SetRows(rows)

	case tea.KeyMsg:
		if m.confirmDel {
			switch msg.String() {
			case "y", "Y":
				if m.selectedCP != nil {
					cp := m.selectedCP
					return m, func() tea.Cmd {
						_ = m.store.Checkpoints().Delete(context.Background(), cp.ConnectionID, cp.TableName)
						return m.refresh()
					}()
				}
				m.confirmDel = false
				m.selectedCP = nil
			case "n", "N", "esc":
				m.confirmDel = false
				m.selectedCP = nil
			}
			return m, nil
		}

		switch msg.String() {
		case "x":
			idx := m.table.Cursor()
			if idx >= 0 && idx < len(m.checkpoints) {
				m.selectedCP = &m.checkpoints[idx]
				m.confirmDel = true
			}
		case "r":
			// Resume: handle in app.go by returning the choice
			idx := m.table.Cursor()
			if idx >= 0 && idx < len(m.checkpoints) {
				return m, func() tea.Msg {
					return resumeCheckpointMsg{cp: m.checkpoints[idx]}
				}
			}
		}
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

type resumeCheckpointMsg struct {
	cp storage.Checkpoint
}

func (m checkpointsModel) View() string {
	if m.confirmDel && m.selectedCP != nil {
		return fmt.Sprintf("\nReset checkpoint for %s / %s?\n\n(y/N)", 
			m.conns[m.selectedCP.ConnectionID].Name, m.selectedCP.TableName)
	}

	header := lipgloss.NewStyle().Bold(true).Padding(0, 1).Render("Active Checkpoints")
	
	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		m.table.View(),
		"\n ↑/↓: navigate • [r] resume • [x] reset • q/esc: back",
	)
}
