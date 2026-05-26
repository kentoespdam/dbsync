package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/user/dbsync/internal/crypto"
	"github.com/user/dbsync/internal/engine"
	"github.com/user/dbsync/internal/logger"
	"github.com/user/dbsync/internal/mysql"
	"github.com/user/dbsync/internal/storage"
)

type runScreenModel struct {
	store      *storage.DB
	conn       storage.Connection
	tables     []string
	tableIdx   int
	currentTable string
	tableResults []tableResult
	masterKey  []byte
	dryRun     bool

	// Engine state
	ctx        context.Context
	cancel     context.CancelFunc
	engine     *engine.Engine
	eventChan  <-chan engine.Event
	
	// UI state
	progress   progress.Model
	viewport   viewport.Model
	logs       []string
	
	totalRows  int
	estimated  int
	batches    int
	startTime  time.Time
	
	status     string // "running", "completed", "failed", "interrupted", "confirm_cancel", "prompt_resume"
	err        error
	logPath    string
	
	width      int
	height     int
	ready      bool
}

type tableResult struct {
	table  string
	status string
	rows   int
	err    error
}

type eventMsg engine.Event
type eventChannelClosedMsg struct{}
type estimationMsg int

func newRunScreenModel(store *storage.DB, conn storage.Connection, tables []string, key []byte, dryRun bool) runScreenModel {
	p := progress.New(progress.WithGradient("#5A56E0", "#EE6FF8"), progress.WithWidth(40))
	v := viewport.New(0, 0)
	
	return runScreenModel{
		store:     store,
		conn:      conn,
		tables:    tables,
		masterKey: key,
		dryRun:    dryRun,
		progress:  p,
		viewport:  v,
		status:    "starting",
	}
}

func (m runScreenModel) Init() tea.Cmd {
	if len(m.tables) == 0 {
		return nil
	}
	m.currentTable = m.tables[m.tableIdx]
	return m.checkCheckpoint()
}

func (m runScreenModel) checkCheckpoint() tea.Cmd {
	return func() tea.Msg {
		cp, err := m.store.Checkpoints().Get(context.Background(), m.conn.ID, m.currentTable)
		if err == nil && (cp.Status == "interrupted" || cp.Status == "failed") {
			return promptResumeMsg{cp}
		}
		return startSyncMsg{resume: false}
	}
}

type promptResumeMsg struct{ cp storage.Checkpoint }
type startSyncMsg struct{ resume bool }

func (m runScreenModel) startSync(resume bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithCancel(context.Background())
		
		// 1. Setup logger
		l, err := logger.New(m.conn.Name, m.currentTable)
		if err != nil {
			cancel()
			return errorMsg(err)
		}
		
		eng := engine.New(m.store, m.masterKey, l)
		
		// 2. Start estimation async
		estimateCmd := m.estimateRows()
		
		// 3. Run engine
		opts := engine.Options{
			ConnectionID: m.conn.ID,
			TableName:    m.currentTable,
			DryRun:       m.dryRun,
		}
		
		ch, err := eng.Run(ctx, opts)
		if err != nil {
			cancel()
			return errorMsg(err)
		}
		
		return tea.Batch(
			estimateCmd,
			func() tea.Msg {
				return syncStartedMsg{
					ctx:       ctx,
					cancel:    cancel,
					engine:    eng,
					eventChan: ch,
					logPath:   l.Path(),
				}
			},
		)()
	}
}

func (m runScreenModel) estimateRows() tea.Cmd {
	return func() tea.Msg {
		// Decrypt source password
		srcPass, err := crypto.Decrypt(m.conn.SourcePassword, m.masterKey)
		if err != nil {
			return nil // Ignore estimation errors
		}
		
		db, err := mysql.Open(mysql.Config{
			Host:     m.conn.SourceHost,
			Port:     m.conn.SourcePort,
			User:     m.conn.SourceUser,
			Password: string(srcPass),
			DBName:   m.conn.SourceDB,
		})
		if err != nil {
			return nil
		}
		defer db.Close()
		
		count, err := mysql.CountRows(context.Background(), db.DB(), m.conn.SourceDB, m.currentTable)
		if err != nil {
			return nil
		}
		return estimationMsg(count)
	}
}

type syncStartedMsg struct {
	ctx       context.Context
	cancel    context.CancelFunc
	engine    *engine.Engine
	eventChan <-chan engine.Event
	logPath   string
}

func waitForEvent(ch <-chan engine.Event) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return eventChannelClosedMsg{}
		}
		return eventMsg(ev)
	}
}

func (m runScreenModel) Update(msg tea.Msg) (runScreenModel, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 10 // Leave room for header and progress
		m.ready = true

	case promptResumeMsg:
		m.status = "prompt_resume"
		return m, nil

	case startSyncMsg:
		m.status = "running"
		m.startTime = time.Now()
		return m, m.startSync(msg.resume)

	case syncStartedMsg:
		m.ctx = msg.ctx
		m.cancel = msg.cancel
		m.engine = msg.engine
		m.eventChan = msg.eventChan
		m.logPath = msg.logPath
		m.status = "running"
		return m, waitForEvent(m.eventChan)

	case estimationMsg:
		m.estimated = int(msg)
		return m, nil

	case eventMsg:
		switch ev := msg.(type) {
		case engine.ProgressEvent:
			m.batches = ev.Batch
			m.totalRows = ev.RowsDone
			m.logs = append(m.logs, fmt.Sprintf("↳ batch %d: %d rows upserted", ev.Batch, ev.RowsDone))
			m.viewport.SetContent(strings.Join(m.logs, "\n"))
			m.viewport.GotoBottom()
			
		case engine.BatchErrorEvent:
			m.logs = append(m.logs, lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(fmt.Sprintf("⚠ batch %d: %v", ev.Batch, ev.Err)))
			m.viewport.SetContent(strings.Join(m.logs, "\n"))
			m.viewport.GotoBottom()

		case engine.RowErrorEvent:
			m.logs = append(m.logs, lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(fmt.Sprintf("⚠ row pk=%v: %v", ev.PK, ev.Err)))
			m.viewport.SetContent(strings.Join(m.logs, "\n"))
			m.viewport.GotoBottom()

		case engine.DoneEvent:
			m.status = ev.Status
			m.err = ev.Err
			m.totalRows = ev.TotalRows
			
			m.tableResults = append(m.tableResults, tableResult{
				table:  m.currentTable,
				status: ev.Status,
				rows:   ev.TotalRows,
				err:    ev.Err,
			})
			
			if ev.Status == "completed" && m.tableIdx < len(m.tables)-1 {
				m.tableIdx++
				m.currentTable = m.tables[m.tableIdx]
				m.logs = append(m.logs, lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render(fmt.Sprintf("✓ Table %s done.", m.tables[m.tableIdx-1])))
				m.viewport.SetContent(strings.Join(m.logs, "\n"))
				m.viewport.GotoBottom()
				
				// Reset per-table state
				m.batches = 0
				m.totalRows = 0
				m.estimated = 0
				m.status = "running"
				
				return m, m.checkCheckpoint()
			}
			
			return m, nil
		}
		return m, waitForEvent(m.eventChan)

	case eventChannelClosedMsg:
		return m, nil

	case tea.KeyMsg:
		switch m.status {
		case "prompt_resume":
			switch msg.String() {
			case "r":
				return m, func() tea.Msg { return startSyncMsg{resume: true} }
			case "f":
				return m, func() tea.Msg { return startSyncMsg{resume: false} }
			case "esc":
				// Handle back in app.go
			}
		case "running":
			switch msg.String() {
			case "c":
				m.status = "confirm_cancel"
			}
		case "confirm_cancel":
			switch msg.String() {
			case "y", "Y":
				if m.cancel != nil {
					m.cancel()
				}
				m.status = "running"
			case "n", "N", "esc":
				m.status = "running"
			}
		}

	case progress.FrameMsg:
		newModel, cmd := m.progress.Update(msg)
		m.progress = newModel.(progress.Model)
		return m, cmd
	}

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m runScreenModel) View() string {
	if !m.ready {
		return "Initializing..."
	}

	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("62")).
		Render(fmt.Sprintf("Sync [%d/%d]: %s / %s", m.tableIdx+1, len(m.tables), m.conn.Name, m.currentTable))

	if m.dryRun {
		header += lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render(" [DRY RUN]")
	}

	var body string
	switch m.status {
	case "prompt_resume":
		body = "\nPrevious run was interrupted.\n\n[r] Resume from last checkpoint\n[f] Fresh start\n[esc] Cancel"
	case "confirm_cancel":
		body = "\nCancel sync? Progress will be saved as checkpoint.\n\n[y] Yes, cancel\n[n] No, continue"
	case "running", "completed", "failed", "interrupted":
		percent := 0.0
		if m.estimated > 0 {
			percent = float64(m.totalRows) / float64(m.estimated)
		}
		
		eta := "Calculating..."
		if m.totalRows > 0 {
			elapsed := time.Since(m.startTime)
			rowsPerSec := float64(m.totalRows) / elapsed.Seconds()
			if rowsPerSec > 0 && m.estimated > m.totalRows {
				remaining := float64(m.estimated-m.totalRows) / rowsPerSec
				eta = time.Duration(remaining * float64(time.Second)).Round(time.Second).String()
			}
		}

		prog := fmt.Sprintf("Batch %d | Rows %d | ETA %s\n%s", m.batches, m.totalRows, eta, m.progress.ViewAs(percent))
		
		body = lipgloss.JoinVertical(lipgloss.Left,
			prog,
			lipgloss.NewStyle().
				Border(lipgloss.NormalBorder(), true, false, true, false).
				Width(m.width).
				Render(m.viewport.View()),
		)

		if m.status != "running" {
			summaryStyle := lipgloss.NewStyle().Bold(true).Padding(1)
			var summary string
			if len(m.tables) > 1 {
				success, partial, failed := 0, 0, 0
				for _, r := range m.tableResults {
					switch r.status {
					case "completed":
						success++
					case "interrupted":
						partial++
					case "failed":
						failed++
					}
				}
				summary = summaryStyle.Render(fmt.Sprintf("Summary: %d success, %d partial, %d failed", success, partial, failed))
				var details []string
				for _, r := range m.tableResults {
					icon := "✗"
					if r.status == "completed" { icon = "✓" }
					if r.status == "interrupted" { icon = "⚠" }
					details = append(details, fmt.Sprintf("  %s %-20s %d rows", icon, r.table, r.rows))
				}
				summary += "\n" + strings.Join(details, "\n")
			} else {
				switch m.status {
				case "completed":
					summary = summaryStyle.Foreground(lipgloss.Color("10")).Render(fmt.Sprintf("✓ Done. %d rows in %v.", m.totalRows, time.Since(m.startTime).Round(time.Second)))
				case "failed":
					summary = summaryStyle.Foreground(lipgloss.Color("9")).Render(fmt.Sprintf("✗ Failed: %v", m.err))
					if m.logPath != "" {
						summary += fmt.Sprintf("\nSee log: %s", m.logPath)
					}
				case "interrupted":
					summary = summaryStyle.Foreground(lipgloss.Color("11")).Render("⚠ Interrupted. Progress saved to checkpoint.")
				}
			}
			body = lipgloss.JoinVertical(lipgloss.Left, body, summary, "\n[esc] Back")
		} else {
			body = lipgloss.JoinVertical(lipgloss.Left, body, "\n[c] cancel  [esc] back when done")
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, body)
}
