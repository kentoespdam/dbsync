package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/dbsync/internal/storage"
)

func (m model) updateConnList(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.connList, cmd = m.connList.Update(msg)
	
	kmsg, ok := msg.(tea.KeyMsg)
	if !ok { return m, cmd }
	
	switch kmsg.String() {
	case "n":
		m.pushHistory(m.current)
		m.current = screenConnForm
		m.connForm = newConnFormModel(m.store, m.masterKey, nil)
		return m, tea.Batch(cmd, m.connForm.Init())
	case "enter", "t", "d":
		selected := m.connList.table.SelectedRow()
		if selected == nil { return m, cmd }
		var found *storage.Connection
		for _, c := range m.connList.conns {
			if c.Name == selected[0] { found = &c; break }
		}
		if found == nil { return m, cmd }
		return m.handleConnAction(kmsg.String(), found, cmd)
	}
	return m, cmd
}

func (m model) handleConnAction(action string, conn *storage.Connection, baseCmd tea.Cmd) (tea.Model, tea.Cmd) {
	m.pushHistory(m.current)
	switch action {
	case "enter":
		m.current = screenConnForm
		m.connForm = newConnFormModel(m.store, m.masterKey, conn)
		return m, tea.Batch(baseCmd, m.connForm.Init())
	case "t":
		m.current = screenConnTest
		m.connTest = newConnTestModel(*conn, m.masterKey)
		return m, tea.Batch(baseCmd, m.connTest.Init())
	case "d":
		m.history = m.history[:len(m.history)-1] // don't push history for delete confirm
		m.connToDelete = conn
		m.showDeleteConfirm = true
	}
	return m, baseCmd
}

func (m model) updateConnTest(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.connTest, cmd = m.connTest.Update(msg)
	if _, ok := msg.(tea.KeyMsg); ok && !m.connTest.loading {
		m.popHistory()
	}
	return m, cmd
}

func (m model) updateConnPicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.connPicker, cmd = m.connPicker.Update(msg)
	if m.connPicker.choice != nil {
		m.selectedConn = m.connPicker.choice
		m.connPicker.choice = nil
		m.pushHistory(m.current)
		m.current = screenTablePicker
		m.tablePicker = newTablePickerModel(*m.selectedConn, m.masterKey, m.store, m.flow)
		m.tablePicker.list.SetSize(m.width, m.height)
		return m, tea.Batch(cmd, m.tablePicker.Init())
	}
	return m, cmd
}

func (m model) updateTablePicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.tablePicker, cmd = m.tablePicker.Update(msg)
	if m.tablePicker.choice == "" { return m, cmd }
	
	m.selectedTable = m.tablePicker.choice
	m.tablePicker.choice = ""
	m.pushHistory(m.current)
	
	if m.flow == "sync" {
		m.current = screenRunSync
		var tables []string
		if m.tablePicker.syncAll {
			tables, _ = m.store.Mappings().ListDistinctTables(context.Background(), m.selectedConn.ID)
		} else {
			tables = []string{m.selectedTable}
		}
		m.runSync = newRunScreenModel(m.store, *m.selectedConn, tables, m.masterKey, false)
		return m, tea.Batch(cmd, m.runSync.Init())
	}
	
	m.current = screenMappingEditor
	m.mappingEditor = newMappingEditorModel(*m.selectedConn, m.masterKey, m.store, m.selectedTable)
	return m, tea.Batch(cmd, m.mappingEditor.Init())
}

func (m model) updateCheckpoints(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.checkpointViewer, cmd = m.checkpointViewer.Update(msg)
	
	if resumeMsg, ok := msg.(resumeCheckpointMsg); ok {
		conn, err := m.store.Connections().GetByID(context.Background(), resumeMsg.cp.ConnectionID)
		if err == nil {
			m.selectedConn = &conn
			m.selectedTable = resumeMsg.cp.TableName
			m.flow = "sync"
			m.pushHistory(m.current)
			m.current = screenRunSync
			m.runSync = newRunScreenModel(m.store, conn, []string{m.selectedTable}, m.masterKey, false)
			m.runSync.autoResume = true
			return m, tea.Batch(cmd, m.runSync.Init())
		}
	}
	return m, cmd
}
