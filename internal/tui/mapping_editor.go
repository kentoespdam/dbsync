package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/user/dbsync/internal/crypto"
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

	table  table.Model
	width  int
	height int

	warnings []string

	selectedMapping *storage.Mapping
	addMapping      bool
}

func newMappingEditorModel(conn storage.Connection, masterKey []byte, store *storage.DB, tableName string) mappingEditorModel {
	columns := []table.Column{
		{Title: "DEST COLUMN", Width: 20},
		{Title: "SOURCE COLUMN", Width: 20},
		{Title: "DEFAULT", Width: 15},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.Bold(true)
	t.SetStyles(s)

	return mappingEditorModel{
		conn:      conn,
		masterKey: masterKey,
		store:     store,
		tableName: tableName,
		loading:   true,
		spinner:   spinner.New(spinner.WithSpinner(spinner.Dot)),
		table:     t,
	}
}

func (m mappingEditorModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.loadData)
}

func (m mappingEditorModel) loadData() tea.Msg {
	srcPass, err := crypto.Decrypt(m.conn.SourcePassword, m.masterKey)
	if err != nil {
		return err
	}
	dstPass, err := crypto.Decrypt(m.conn.DestPassword, m.masterKey)
	if err != nil {
		return err
	}

	srcPool, err := mysql.Open(mysql.Config{
		Host: m.conn.SourceHost, Port: m.conn.SourcePort,
		User: m.conn.SourceUser, Password: string(srcPass), DBName: m.conn.SourceDB,
	})
	if err != nil {
		return err
	}
	defer srcPool.Close()

	dstPool, err := mysql.Open(mysql.Config{
		Host: m.conn.DestHost, Port: m.conn.DestPort,
		User: m.conn.DestUser, Password: string(dstPass), DBName: m.conn.DestDB,
	})
	if err != nil {
		return err
	}
	defer dstPool.Close()

	ctx := context.Background()
	srcCols, err := mysql.DescribeColumns(ctx, srcPool.DB(), m.conn.SourceDB, m.tableName)
	if err != nil {
		return err
	}
	dstCols, err := mysql.DescribeColumns(ctx, dstPool.DB(), m.conn.DestDB, m.tableName)
	if err != nil {
		return err
	}

	mappings, err := m.store.Mappings().ListByTable(ctx, m.conn.ID, m.tableName)
	if err != nil {
		return err
	}

	isNew := len(mappings) == 0
	if isNew {
		auto := storage.AutoMap(m.conn.ID, m.tableName, srcCols, dstCols)
		mappings = auto.Mappings
	}

	return mappingDataLoadedMsg{
		srcCols:  srcCols,
		dstCols:  dstCols,
		mappings: mappings,
		isNew:    isNew,
	}
}

type mappingDataLoadedMsg struct {
	srcCols  []mysql.Column
	dstCols  []mysql.Column
	mappings []storage.Mapping
	isNew    bool
}

func (m mappingEditorModel) Update(msg tea.Msg) (mappingEditorModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.table.SetWidth(m.width * 2 / 3)
		m.table.SetHeight(m.height - 10)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case mappingDataLoadedMsg:
		m.loading = false
		m.sourceCols = msg.srcCols
		m.destCols = msg.dstCols
		m.mappings = msg.mappings
		if msg.isNew {
			m.dirty = true
		}
		m.refreshTable()
		m.recomputeWarnings()
		return m, nil

	case error:
		m.err = msg
		m.loading = false
		return m, nil

	case tea.KeyMsg:
		if m.loading {
			return m, nil
		}
		switch msg.String() {
		case "s":
			return m, m.save
		case "r":
			auto := storage.AutoMap(m.conn.ID, m.tableName, m.sourceCols, m.destCols)
			m.mappings = auto.Mappings
			m.dirty = true
			m.refreshTable()
			m.recomputeWarnings()
			return m, nil
		case "e":
			idx := m.table.Cursor()
			if idx >= 0 && idx < len(m.mappings) {
				m.selectedMapping = &m.mappings[idx]
			}
		case "n":
			m.addMapping = true
		case "d":
			idx := m.table.Cursor()
			if idx >= 0 && idx < len(m.mappings) {
				m.mappings = append(m.mappings[:idx], m.mappings[idx+1:]...)
				m.dirty = true
				m.refreshTable()
				m.recomputeWarnings()
			}
		}
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *mappingEditorModel) refreshTable() {
	rows := make([]table.Row, len(m.mappings))
	for i, mp := range m.mappings {
		src := "-"
		if mp.SourceColumn.Valid {
			src = mp.SourceColumn.String
		}
		def := "-"
		if mp.DefaultValue.Valid {
			def = mp.DefaultValue.String
		}
		rows[i] = table.Row{mp.DestColumn, src, def}
	}
	m.table.SetRows(rows)
}

func (m *mappingEditorModel) recomputeWarnings() {
	m.warnings = nil
	mappedDest := make(map[string]storage.Mapping)
	for _, mp := range m.mappings {
		mappedDest[mp.DestColumn] = mp
	}

	for _, dc := range m.destCols {
		mp, ok := mappedDest[dc.Name]
		if !ok {
			if !dc.IsNullable {
				m.warnings = append(m.warnings, fmt.Sprintf("⚠ %s: NOT NULL, no mapping", dc.Name))
			}
			continue
		}

		if !mp.SourceColumn.Valid && !mp.DefaultValue.Valid {
			if !dc.IsNullable {
				m.warnings = append(m.warnings, fmt.Sprintf("⚠ %s: NOT NULL, no source/default", dc.Name))
			}
		}
	}
}

func (m mappingEditorModel) save() tea.Msg {
	ctx := context.Background()
	// Delete old
	err := m.store.Mappings().DeleteByTable(ctx, m.conn.ID, m.tableName)
	if err != nil {
		return err
	}
	// Insert new
	err = m.store.Mappings().BulkInsert(ctx, m.mappings)
	if err != nil {
		return err
	}
	return successMsg{}
}

func (m mappingEditorModel) View() string {
	if m.err != nil {
		return errorStyle.Render("Error: " + m.err.Error())
	}
	if m.loading {
		return "\n " + m.spinner.View() + " Loading mapping data..."
	}

	leftPanel := m.table.View()
	
	rightPanel := "DETAIL & WARNINGS\n\n"
	if len(m.warnings) > 0 {
		rightPanel += lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Render(strings.Join(m.warnings, "\n"))
	} else {
		rightPanel += "✓ No warnings"
	}

	// Get current selected mapping details
	idx := m.table.Cursor()
	if idx >= 0 && idx < len(m.mappings) {
		mp := m.mappings[idx]
		rightPanel += "\n\nSelected: " + mp.DestColumn
		// find dest col info
		for _, dc := range m.destCols {
			if dc.Name == mp.DestColumn {
				rightPanel += fmt.Sprintf("\nType: %s", dc.DataType)
				if !dc.IsNullable {
					rightPanel += " (NOT NULL)"
				}
				break
			}
		}
	}

	main := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(m.width*2/3).Render(leftPanel),
		lipgloss.NewStyle().PaddingLeft(2).Width(m.width/3).Render(rightPanel),
	)

	help := "e: edit • n: add • d: delete • s: save • r: reset • esc: back"
	if m.dirty {
		help = lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Render("UNSAVED CHANGES • ") + help
	}

	return fmt.Sprintf("%s\n\n%s", main, helpStyle.Render(help))
}
