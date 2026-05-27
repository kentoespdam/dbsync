package tui

import (
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/dbsync/internal/mysql"
	"github.com/user/dbsync/internal/storage"
)

type mappingEditorModel struct {
	conn      storage.Connection
	masterKey []byte
	store     *storage.DB
	tableName string

	loading bool
	spinner spinner.Model
	err     error

	sourceCols []mysql.Column
	destCols   []mysql.Column
	mappings   []storage.Mapping
	dirty      bool

	filteredMappings []storage.Mapping
	filterText       string
	warningsOnly     bool

	table  table.Model
	width  int
	height int

	filtering   bool
	filterInput textinput.Model

	editForm *mappingEditFormModel
}

func newMappingEditorModel(conn storage.Connection, masterKey []byte, store *storage.DB, tableName string) mappingEditorModel {
	columns := []table.Column{
		{Title: "ST", Width: 4},
		{Title: "DEST COLUMN", Width: 20},
		{Title: "SOURCE COLUMN", Width: 20},
		{Title: "DEFAULT", Width: 15},
	}

	t := table.New(table.WithColumns(columns), table.WithFocused(true))
	s := table.DefaultStyles()
	s.Header = s.Header.Bold(true)
	t.SetStyles(s)

	ti := textinput.New()
	ti.Placeholder = "Filter destination column..."
	ti.CharLimit = 50
	ti.Width = 30

	return mappingEditorModel{
		conn:        conn,
		masterKey:   masterKey,
		store:       store,
		tableName:   tableName,
		loading:     true,
		spinner:     spinner.New(spinner.WithSpinner(spinner.Dot)),
		table:       t,
		filterInput: ti,
	}
}

func (m mappingEditorModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.loadData)
}

type mappingDataLoadedMsg struct {
	srcCols  []mysql.Column
	dstCols  []mysql.Column
	mappings []storage.Mapping
	isNew    bool
}

