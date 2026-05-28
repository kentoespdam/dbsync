package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/user/dbsync/internal/engine"
)

func (m runScreenModel) Update(msg tea.Msg) (runScreenModel, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.viewport.Width, m.viewport.Height = msg.Width, msg.Height-10
	case promptResumeMsg:
		m.status = "prompt_resume"
	case startSyncMsg:
		m.status, m.startTime = "running", time.Now()
		return m, m.startSync(msg.resume)
	case syncStartedMsg:
		return m.handleSyncStarted(msg)
	case estimationMsg:
		m.estimated = int(msg)
	case eventMsg:
		return m.handleEvent(msg)
	case eventChannelClosedMsg:
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	case progress.FrameMsg:
		newModel, cmd := m.progress.Update(msg)
		m.progress = newModel.(progress.Model)
		return m, cmd
	}
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m runScreenModel) handleSyncStarted(msg syncStartedMsg) (runScreenModel, tea.Cmd) {
	m.ctx, m.cancel, m.engine, m.eventChan, m.logPath = msg.ctx, msg.cancel, msg.engine, msg.eventChan, msg.logPath
	m.status = "running"
	return m, waitForEvent(m.eventChan)
}

func (m runScreenModel) handleKey(msg tea.KeyMsg) (runScreenModel, tea.Cmd) {
	switch m.status {
	case "prompt_resume":
		if msg.String() == "r" { return m, func() tea.Msg { return startSyncMsg{resume: true} } }
		if msg.String() == "f" { return m, func() tea.Msg { return startSyncMsg{resume: false} } }
	case "running":
		if msg.String() == "c" { m.status = "confirm_cancel" }
	case "confirm_cancel":
		if msg.String() == "y" || msg.String() == "Y" {
			if m.cancel != nil { m.cancel() }
			m.status = "running"
		} else if msg.String() == "n" || msg.String() == "N" || msg.String() == "esc" {
			m.status = "running"
		}
	}
	return m, nil
}

func (m runScreenModel) handleEvent(msg eventMsg) (runScreenModel, tea.Cmd) {
	switch ev := msg.(type) {
	case engine.ProgressEvent:
		m.batches, m.totalRows = ev.Batch, ev.RowsDone
		m.log(fmt.Sprintf("↳ batch %d: %d rows upserted", ev.Batch, ev.RowsDone))
	case engine.BatchErrorEvent:
		m.log(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(fmt.Sprintf("⚠ batch %d: %v", ev.Batch, ev.Err)))
	case engine.RowErrorEvent:
		m.log(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(fmt.Sprintf("⚠ row pk=%v: %v", ev.PK, ev.Err)))
	case engine.DoneEvent:
		return m.handleDone(ev)
	}
	return m, waitForEvent(m.eventChan)
}

func (m *runScreenModel) log(s string) {
	m.logs = append(m.logs, s)
	m.viewport.SetContent(strings.Join(m.logs, "\n"))
	m.viewport.GotoBottom()
}
