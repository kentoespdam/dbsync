package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kentoespdam/dbsync/internal/storage"
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
	screenRunSync
	screenHistory
	screenCheckpoints
)

type model struct {
	current       screen
	history       []screen // stack for back-navigation
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
	toastMsg string
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

func (m *model) pushHistory(s screen) {
	m.history = append(m.history, s)
}

func (m *model) popHistory() {
	if len(m.history) > 0 {
		m.current = m.history[len(m.history)-1]
		m.history = m.history[:len(m.history)-1]
	}
}

type successMsg struct {
	message string
}

type errorMsg error
