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
)

type model struct {
	current       screen
	history       []screen // stack untuk back-navigation
	masterKey     []byte
	store         *storage.DB
	width, height int
	err           error

	// child models per screen
	pwdPrompt passwordPromptModel
	mainMenu  mainMenuModel
	connList  connListModel
	connForm  connFormModel
	connTest  connTestModel

	// Delete confirmation state
	showDeleteConfirm bool
	connToDelete      *storage.Connection
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

		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "q", "esc":
			if m.current == screenMain || m.current == screenPasswordPrompt {
				return m, tea.Quit
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
	}

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	if m.showDeleteConfirm && m.connToDelete != nil {
		return "\nDelete connection '" + m.connToDelete.Name + "'?\n\n(y/N)"
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
	default:
		return "Unknown screen"
	}
}

func (m *model) pushHistory(s screen) {
	m.history = append(m.history, s)
}
