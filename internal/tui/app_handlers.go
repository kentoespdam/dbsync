package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
)

func (m model) handleDeleteConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		if m.connToDelete != nil {
			_ = m.store.Connections().Delete(context.Background(), m.connToDelete.ID)
			m.showDeleteConfirm, m.connToDelete = false, nil
			return m, m.connList.refresh()
		}
	case "n", "N", "esc":
		m.showDeleteConfirm, m.connToDelete = false, nil
	}
	return m, nil
}

func (m model) handleDiscardConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.showDiscardConfirm = false
		m.mappingEditor.dirty = false
		m.popHistory()
	case "n", "N", "esc":
		m.showDiscardConfirm = false
	}
	return m, nil
}

func (m model) delegateUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.current {
	case screenPasswordPrompt:
		return m.updatePwdPrompt(msg)
	case screenMain:
		return m.updateMain(msg)
	case screenConnList:
		return m.updateConnList(msg)
	case screenConnForm:
		m.connForm, cmd = m.connForm.Update(msg)
		return m, cmd
	case screenConnTest:
		return m.updateConnTest(msg)
	case screenConnPicker:
		return m.updateConnPicker(msg)
	case screenTablePicker:
		return m.updateTablePicker(msg)
	case screenMappingEditor:
		m.mappingEditor, cmd = m.mappingEditor.Update(msg)
		return m, cmd
	case screenRunSync:
		m.runSync, cmd = m.runSync.Update(msg)
		return m, cmd
	case screenHistory:
		m.historyViewer, cmd = m.historyViewer.Update(msg)
		return m, cmd
	case screenCheckpoints:
		return m.updateCheckpoints(msg)
	}
	return m, nil
}

func (m model) updatePwdPrompt(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.pwdPrompt, cmd = m.pwdPrompt.Update(msg)
	if m.pwdPrompt.success {
		m.masterKey = m.pwdPrompt.masterKey
		m.current = screenMain
		m.mainMenu = newMainMenuModel()
		m.mainMenu.list.SetSize(m.width, m.height)
		return m, tea.Batch(cmd, m.mainMenu.Init())
	}
	return m, cmd
}

func (m model) updateMain(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.mainMenu, cmd = m.mainMenu.Update(msg)
	if m.mainMenu.choice == "" { return m, cmd }
	
	choice := m.mainMenu.choice
	m.mainMenu.choice = ""
	
	switch choice {
	case "Connections":
		m.pushHistory(m.current)
		m.current = screenConnList
		m.connList = newConnListModel(m.store)
		m.connList.table.SetWidth(m.width)
		m.connList.table.SetHeight(m.height - 5)
		return m, tea.Batch(cmd, m.connList.Init())
	case "Tables & Mappings", "Sync":
		m.flow = "mapping"
		if choice == "Sync" { m.flow = "sync" }
		m.pushHistory(m.current)
		m.current = screenConnPicker
		m.connPicker = newConnPickerModel(m.store)
		m.connPicker.list.SetSize(m.width, m.height)
		return m, tea.Batch(cmd, m.connPicker.Init())
	case "History":
		m.pushHistory(m.current)
		m.current = screenHistory
		m.historyViewer = newHistoryModel(m.store, nil)
		return m, tea.Batch(cmd, m.historyViewer.Init())
	case "Checkpoints":
		m.pushHistory(m.current)
		m.current = screenCheckpoints
		m.checkpointViewer = newCheckpointsModel(m.store)
		return m, tea.Batch(cmd, m.checkpointViewer.Init())
	case "Quit":
		return m, tea.Quit
	}
	return m, cmd
}
