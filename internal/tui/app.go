package tui

import (
	"context"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/dbsync/internal/storage"
)

type screen int

const (
	screenPasswordPrompt screen = iota
	screenMain
	screenConnList
	screenConnForm
	screenConnTest
	screenConnPicker
	screenTablePicker
	screenMappingEditor
	screenMappingEditForm
	screenRunSync
	screenHistory
	screenCheckpoints
)

type model struct {
	current       screen
	history       []screen // stack untuk back-navigation
	masterKey     []byte
	store         *storage.DB
	width, height int
	err           error

	// child models per screen
	pwdPrompt   passwordPromptModel
	mainMenu    mainMenuModel
	connList    connListModel
	connForm    connFormModel
	connTest    connTestModel
	connPicker  connPickerModel
	tablePicker tablePickerModel
	mappingEditor mappingEditorModel
	mappingForm   mappingEditFormModel
	runSync       runScreenModel
	historyViewer historyModel
	checkpointViewer checkpointsModel

	selectedConn  *storage.Connection
	selectedTable string
	flow          string // "mapping" or "sync"

	// Delete confirmation state
	showDeleteConfirm bool
	connToDelete      *storage.Connection

	showDiscardConfirm bool
}

func New(db *storage.DB) model {
	return model{
		current:   screenPasswordPrompt,
		store:     db,
		pwdPrompt: newPasswordPromptModel(db),
	}
}

func (m model) Init() tea.Cmd {
	return m.pwdPrompt.Init()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Propagate to child models that need it
		m.mainMenu.list.SetSize(msg.Width, msg.Height)
		m.connList.table.SetWidth(msg.Width)
		m.connList.table.SetHeight(msg.Height - 5) // leave room for help text
		m.connPicker.list.SetSize(msg.Width, msg.Height)
		m.tablePicker.list.SetSize(msg.Width, msg.Height)
		m.mappingEditor.width = msg.Width
		m.mappingEditor.height = msg.Height
		m.mappingForm.width = msg.Width
		m.mappingForm.height = msg.Height
		m.runSync.width = msg.Width
		m.runSync.height = msg.Height
		m.historyViewer.width = msg.Width
		m.historyViewer.height = msg.Height
		m.checkpointViewer.width = msg.Width
		m.checkpointViewer.height = msg.Height


	case tea.KeyMsg:
		if m.showDeleteConfirm {
			switch msg.String() {
			case "y", "Y":
				if m.connToDelete != nil {
					_ = m.store.Connections().Delete(context.Background(), m.connToDelete.ID)
					m.showDeleteConfirm = false
					m.connToDelete = nil
					return m, m.connList.refresh()
				}
			case "n", "N", "esc":
				m.showDeleteConfirm = false
				m.connToDelete = nil
				return m, nil
			}
			return m, nil
		}

		if m.showDiscardConfirm {
			switch msg.String() {
			case "y", "Y":
				m.showDiscardConfirm = false
				m.mappingEditor.dirty = false
				// Pop history
				if len(m.history) > 0 {
					m.current = m.history[len(m.history)-1]
					m.history = m.history[:len(m.history)-1]
				}
				return m, nil
			case "n", "N", "esc":
				m.showDiscardConfirm = false
				return m, nil
			}
			return m, nil
		}

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

			// Pop history
			if len(m.history) > 0 {
				m.current = m.history[len(m.history)-1]
				m.history = m.history[:len(m.history)-1]
				// Refresh list if we go back to it
				if m.current == screenConnList {
					cmds = append(cmds, m.connList.refresh())
				}
				return m, tea.Batch(cmds...)
			}
		}

	case successMsg:
		// Form saved successfully
		if len(m.history) > 0 {
			m.current = m.history[len(m.history)-1]
			m.history = m.history[:len(m.history)-1]
			if m.current == screenConnList {
				cmds = append(cmds, m.connList.refresh())
			}
		}
		return m, tea.Batch(cmds...)
	}

	// Delegate to current screen
	switch m.current {
	case screenPasswordPrompt:
		m.pwdPrompt, cmd = m.pwdPrompt.Update(msg)
		cmds = append(cmds, cmd)
		if m.pwdPrompt.success {
			m.masterKey = m.pwdPrompt.masterKey
			m.current = screenMain
			m.mainMenu = newMainMenuModel()
			cmds = append(cmds, m.mainMenu.Init())
		}

	case screenMain:
		m.mainMenu, cmd = m.mainMenu.Update(msg)
		cmds = append(cmds, cmd)
		if m.mainMenu.choice != "" {
			switch m.mainMenu.choice {
			case "Connections":
				m.pushHistory(m.current)
				m.current = screenConnList
				m.connList = newConnListModel(m.store)
				cmds = append(cmds, m.connList.Init())
			case "Tables & Mappings":
				m.flow = "mapping"
				m.pushHistory(m.current)
				m.current = screenConnPicker
				m.connPicker = newConnPickerModel(m.store)
				cmds = append(cmds, m.connPicker.Init())
			case "Sync":
				m.flow = "sync"
				m.pushHistory(m.current)
				m.current = screenConnPicker
				m.connPicker = newConnPickerModel(m.store)
				cmds = append(cmds, m.connPicker.Init())
			case "History":
				m.pushHistory(m.current)
				m.current = screenHistory
				m.historyViewer = newHistoryModel(m.store, nil)
				cmds = append(cmds, m.historyViewer.Init())
			case "Checkpoints":
				m.pushHistory(m.current)
				m.current = screenCheckpoints
				m.checkpointViewer = newCheckpointsModel(m.store)
				cmds = append(cmds, m.checkpointViewer.Init())
			case "Quit":
				return m, tea.Quit
			}
			m.mainMenu.choice = ""
		}

	case screenConnList:
		m.connList, cmd = m.connList.Update(msg)
		cmds = append(cmds, cmd)

		// Handle actions from list
		if kmsg, ok := msg.(tea.KeyMsg); ok {
			switch kmsg.String() {
			case "n":
				m.pushHistory(m.current)
				m.current = screenConnForm
				m.connForm = newConnFormModel(m.store, m.masterKey, nil)
				cmds = append(cmds, m.connForm.Init())
			case "enter":
				selected := m.connList.table.SelectedRow()
				if selected != nil {
					// Find connection by name (simple way)
					var found *storage.Connection
					for _, c := range m.connList.conns {
						if c.Name == selected[0] {
							found = &c
							break
						}
					}
					if found != nil {
						m.pushHistory(m.current)
						m.current = screenConnForm
						m.connForm = newConnFormModel(m.store, m.masterKey, found)
						cmds = append(cmds, m.connForm.Init())
					}
				}
			case "t":
				selected := m.connList.table.SelectedRow()
				if selected != nil {
					var found *storage.Connection
					for _, c := range m.connList.conns {
						if c.Name == selected[0] {
							found = &c
							break
						}
					}
					if found != nil {
						m.pushHistory(m.current)
						m.current = screenConnTest
						m.connTest = newConnTestModel(*found, m.masterKey)
						cmds = append(cmds, m.connTest.Init())
					}
				}
			case "d":
				selected := m.connList.table.SelectedRow()
				if selected != nil {
					for _, c := range m.connList.conns {
						if c.Name == selected[0] {
							m.connToDelete = &c
							m.showDeleteConfirm = true
							break
						}
					}
				}
			}
		}

	case screenConnForm:
		m.connForm, cmd = m.connForm.Update(msg)
		cmds = append(cmds, cmd)

	case screenConnTest:
		m.connTest, cmd = m.connTest.Update(msg)
		cmds = append(cmds, cmd)
		if _, ok := msg.(tea.KeyMsg); ok && !m.connTest.loading {
			// Return to list on any key
			if len(m.history) > 0 {
				m.current = m.history[len(m.history)-1]
				m.history = m.history[:len(m.history)-1]
			}
		}

	case screenConnPicker:
		m.connPicker, cmd = m.connPicker.Update(msg)
		cmds = append(cmds, cmd)
		if m.connPicker.choice != nil {
			m.selectedConn = m.connPicker.choice
			m.connPicker.choice = nil
			m.pushHistory(m.current)
			m.current = screenTablePicker
			m.tablePicker = newTablePickerModel(*m.selectedConn, m.masterKey, m.store, m.flow)
			cmds = append(cmds, m.tablePicker.Init())
		}

	case screenTablePicker:
		m.tablePicker, cmd = m.tablePicker.Update(msg)
		cmds = append(cmds, cmd)
		if m.tablePicker.choice != "" {
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
				cmds = append(cmds, m.runSync.Init())
			} else {
				m.current = screenMappingEditor
				m.mappingEditor = newMappingEditorModel(*m.selectedConn, m.masterKey, m.store, m.selectedTable)
				cmds = append(cmds, m.mappingEditor.Init())
			}
		}

	case screenMappingEditor:
		m.mappingEditor, cmd = m.mappingEditor.Update(msg)
		cmds = append(cmds, cmd)

		if m.mappingEditor.selectedMapping != nil {
			m.pushHistory(m.current)
			m.current = screenMappingEditForm
			m.mappingForm = newMappingEditFormModel(*m.mappingEditor.selectedMapping, m.mappingEditor.sourceCols, m.mappingEditor.destCols, false)
			cmds = append(cmds, m.mappingForm.Init())
			m.mappingEditor.selectedMapping = nil
		} else if m.mappingEditor.addMapping {
			m.pushHistory(m.current)
			m.current = screenMappingEditForm
			m.mappingForm = newMappingEditFormModel(storage.Mapping{
				ConnectionID: m.selectedConn.ID,
				TableName:    m.selectedTable,
			}, m.mappingEditor.sourceCols, m.mappingEditor.destCols, true)
			cmds = append(cmds, m.mappingForm.Init())
			m.mappingEditor.addMapping = false
		}

	case screenMappingEditForm:
		m.mappingForm, cmd = m.mappingForm.Update(msg)
		cmds = append(cmds, cmd)
		if m.mappingForm.done {
			if !m.mappingForm.canceled {
				if m.mappingForm.isNew {
					m.mappingEditor.mappings = append(m.mappingEditor.mappings, m.mappingForm.mapping)
				} else {
					// Update existing in editor's slice
					for i, mp := range m.mappingEditor.mappings {
						if mp.DestColumn == m.mappingForm.mapping.DestColumn {
							m.mappingEditor.mappings[i] = m.mappingForm.mapping
							break
						}
					}
				}
				m.mappingEditor.dirty = true
				m.mappingEditor.refreshTable()
				m.mappingEditor.recomputeWarnings()
			}
			m.mappingForm.done = false
			// Pop history
			if len(m.history) > 0 {
				m.current = m.history[len(m.history)-1]
				m.history = m.history[:len(m.history)-1]
			}
		}

	case screenRunSync:
		m.runSync, cmd = m.runSync.Update(msg)
		cmds = append(cmds, cmd)

	case screenHistory:
		m.historyViewer, cmd = m.historyViewer.Update(msg)
		cmds = append(cmds, cmd)

	case screenCheckpoints:
		m.checkpointViewer, cmd = m.checkpointViewer.Update(msg)
		cmds = append(cmds, cmd)

		if resumeMsg, ok := msg.(resumeCheckpointMsg); ok {
			// Find connection
			var conn storage.Connection
			c, err := m.store.Connections().GetByID(context.Background(), resumeMsg.cp.ConnectionID)
			if err == nil {
				conn = c
				m.selectedConn = &conn
				m.selectedTable = resumeMsg.cp.TableName
				m.flow = "sync"
				m.pushHistory(m.current)
				m.current = screenRunSync
				m.runSync = newRunScreenModel(m.store, conn, []string{m.selectedTable}, m.masterKey, false)
				cmds = append(cmds, m.runSync.Init())
			}
		}
	}

	return m, tea.Batch(cmds...)
}

type successMsg struct{}
type errorMsg error

func (m model) View() string {
	if m.showDeleteConfirm && m.connToDelete != nil {
		return "\nDelete connection '" + m.connToDelete.Name + "'?\n\n(y/N)"
	}

	if m.showDiscardConfirm {
		return "\nDiscard unsaved mapping changes?\n\n(y/N)"
	}

	switch m.current {
	case screenPasswordPrompt:
		return m.pwdPrompt.View()
	case screenMain:
		return m.mainMenu.View()
	case screenConnList:
		return m.connList.View()
	case screenConnForm:
		return m.connForm.View()
	case screenConnTest:
		return m.connTest.View()
	case screenConnPicker:
		return m.connPicker.View()
	case screenTablePicker:
		return m.tablePicker.View()
	case screenMappingEditor:
		return m.mappingEditor.View()
	case screenMappingEditForm:
		return m.mappingForm.View()
	case screenRunSync:
		return m.runSync.View()
	case screenHistory:
		return m.historyViewer.View()
	case screenCheckpoints:
		return m.checkpointViewer.View()
	default:
		return "Unknown screen"
	}
}

func (m *model) pushHistory(s screen) {
	m.history = append(m.history, s)
}
