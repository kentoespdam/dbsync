package tui

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/dbsync/internal/engine"
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
	autoResume bool

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
type promptResumeMsg struct{ cp storage.Checkpoint }
type startSyncMsg struct{ resume bool }

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

	if m.autoResume {
		return func() tea.Msg { return startSyncMsg{resume: true} }
	}

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
