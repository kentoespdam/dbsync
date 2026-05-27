package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/user/dbsync/internal/crypto"
	"github.com/user/dbsync/internal/engine"
	"github.com/user/dbsync/internal/logger"
	"github.com/user/dbsync/internal/mysql"
)

func (m runScreenModel) handleDone(ev engine.DoneEvent) (runScreenModel, tea.Cmd) {
	m.status, m.err, m.totalRows = ev.Status, ev.Err, ev.TotalRows
	m.tableResults = append(m.tableResults, tableResult{
		table: m.currentTable, status: ev.Status, rows: ev.TotalRows, err: ev.Err,
	})
	
	if ev.Status == "completed" && m.tableIdx < len(m.tables)-1 {
		m.tableIdx++
		m.currentTable = m.tables[m.tableIdx]
		m.log(lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render(fmt.Sprintf("✓ Table %s done.", m.tables[m.tableIdx-1])))
		m.batches, m.totalRows, m.estimated, m.status = 0, 0, 0, "running"
		return m, m.checkCheckpoint()
	}
	return m, nil
}

func (m runScreenModel) startSync(resume bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithCancel(context.Background())
		if !resume {
			_ = m.store.Checkpoints().Delete(context.Background(), m.conn.ID, m.currentTable)
		}
		l, err := logger.New(m.conn.Name, m.currentTable)
		if err != nil { cancel(); return errorMsg(err) }
		
		eng := engine.New(m.store, m.masterKey, l)
		opts := engine.Options{ConnectionID: m.conn.ID, TableName: m.currentTable, DryRun: m.dryRun}
		ch, err := eng.Run(ctx, opts)
		if err != nil { cancel(); return errorMsg(err) }
		
		return syncStartedMsg{ctx: ctx, cancel: cancel, engine: eng, eventChan: ch, logPath: l.Path()}
	}
}

func (m runScreenModel) estimateRows() tea.Cmd {
	return func() tea.Msg {
		srcPass, err := crypto.Decrypt(m.conn.SourcePassword, m.masterKey)
		if err != nil { return nil }
		db, err := mysql.Open(mysql.Config{
			Host: m.conn.SourceHost, Port: m.conn.SourcePort, User: m.conn.SourceUser,
			Password: string(srcPass), DBName: m.conn.SourceDB,
		})
		if err != nil { return nil }
		defer db.Close()
		count, _ := mysql.CountRows(context.Background(), db.DB(), m.conn.SourceDB, m.currentTable)
		return estimationMsg(count)
	}
}

func waitForEvent(ch <-chan engine.Event) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok { return eventChannelClosedMsg{} }
		return eventMsg(ev)
	}
}

type syncStartedMsg struct {
	ctx       context.Context
	cancel    context.CancelFunc
	engine    *engine.Engine
	eventChan <-chan engine.Event
	logPath   string
}
