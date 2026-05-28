package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.handleWindowSize(msg)
	case tea.KeyMsg:
		return m.handleKey(msg)
	case successMsg:
		return m.handleSuccess(msg)
	case error, errorMsg:
		return m.handleError(msg)
	case toastMsg:
		m.toastMsg = msg.message
		return m, tea.Tick(msg.timeout, func(t time.Time) tea.Msg { return clearToastMsg{} })
	case clearToastMsg:
		m.toastMsg = ""
		return m, nil
	}
	return m.delegateUpdate(msg)
}

func (m model) handleWindowSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.width, m.height = msg.Width, msg.Height
	if m.mainMenu.inited { m.mainMenu.list.SetSize(msg.Width, msg.Height) }
	if m.connList.inited {
		m.connList.table.SetWidth(msg.Width)
		m.connList.table.SetHeight(msg.Height - 5)
	}
	if m.connPicker.inited { m.connPicker.list.SetSize(msg.Width, msg.Height) }
	if m.tablePicker.inited {
		m.tablePicker, _ = m.tablePicker.Update(msg)
	}
	
	m.mappingEditor.width, m.mappingEditor.height = msg.Width, msg.Height
	m.runSync.width, m.runSync.height = msg.Width, msg.Height
	m.historyViewer.width, m.historyViewer.height = msg.Width, msg.Height
	m.checkpointViewer.width, m.checkpointViewer.height = msg.Width, msg.Height

	// Delegate to active child so its own Update sees the WindowSizeMsg
	// (e.g. runScreenModel resizes its viewport). bd-7h9.
	return m.delegateUpdate(msg)
}

func (m model) handleSuccess(msg successMsg) (tea.Model, tea.Cmd) {
	m.toastMsg = msg.message
	tick := tea.Tick(3*time.Second, func(t time.Time) tea.Msg { return clearToastMsg{} })
	
	if m.current == screenMappingEditor {
		m.popHistory()
	} else if len(m.history) > 0 {
		m.popHistory()
		if m.current == screenConnList {
			return m, tea.Batch(tick, m.connList.refresh())
		}
	}
	return m, tick
}

func (m model) handleError(msg any) (tea.Model, tea.Cmd) {
	var errStr string
	if e, ok := msg.(error); ok { errStr = e.Error() }
	if e, ok := msg.(errorMsg); ok { errStr = e.Error() }
	
	m.toastMsg = "Error: " + errStr
	return m, tea.Tick(5*time.Second, func(t time.Time) tea.Msg { return clearToastMsg{} })
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.showDeleteConfirm { return m.handleDeleteConfirm(msg) }
	if m.showDiscardConfirm { return m.handleDiscardConfirm(msg) }

	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "q", "esc":
		if m.current == screenMain || m.current == screenPasswordPrompt {
			return m, tea.Quit
		}
		if m.current == screenMappingEditor && m.mappingEditor.dirty {
			m.showDiscardConfirm = true
			return m, nil
		}
		m.popHistory()
		if m.current == screenConnList {
			return m, m.connList.refresh()
		}
		return m, nil
	}
	return m.delegateUpdate(msg)
}
