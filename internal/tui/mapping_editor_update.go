package tui

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kentoespdam/dbsync/internal/storage"
)

func (m mappingEditorModel) Update(msg tea.Msg) (mappingEditorModel, tea.Cmd) {
	if m.editForm != nil {
		var cmd tea.Cmd
		form, cmd := m.editForm.Update(msg)
		m.editForm = &form
		if m.editForm.done {
			if !m.editForm.canceled {
				m.dirty = true
				for i, mp := range m.mappings {
					if mp.DestColumn == m.editForm.mapping.DestColumn {
						m.mappings[i] = m.editForm.mapping
						break
					}
				}
				m.applyFilter()
			}
			m.editForm = nil
		}
		return m, cmd
	}

	if m.filtering {
		return m.updateFiltering(msg)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.table.SetWidth(m.width)
		m.table.SetHeight(m.height - 12)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case mappingDataLoadedMsg:
		m.loading = false
		m.sourceCols, m.destCols, m.mappings = msg.srcCols, msg.dstCols, msg.mappings
		m.dirty = msg.isNew
		m.applyFilter()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m mappingEditorModel) updateFiltering(msg tea.Msg) (mappingEditorModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter", "esc":
			m.filtering = false
			m.filterText = m.filterInput.Value()
			m.applyFilter()
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.filterInput, cmd = m.filterInput.Update(msg)
	m.filterText = m.filterInput.Value()
	m.applyFilter()
	return m, cmd
}

func (m mappingEditorModel) handleKey(msg tea.KeyMsg) (mappingEditorModel, tea.Cmd) {
	if m.loading {
		return m, nil
	}

	switch msg.String() {
	case "/":
		m.filtering = true
		m.filterInput.Focus()
		return m, nil
	case "s":
		return m, m.save
	case "w":
		m.warningsOnly = !m.warningsOnly
		m.applyFilter()
	case "N":
		m.nextWarning()
	case "e":
		m.editSelected()
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *mappingEditorModel) nextWarning() {
	curr := m.table.Cursor()
	for i := 1; i <= len(m.filteredMappings); i++ {
		idx := (curr + i) % len(m.filteredMappings)
		mp := m.filteredMappings[idx]
		dc := m.findDestCol(mp.DestColumn)
		icon, _ := m.mappingStatus(mp, dc)
		if icon == "⚠" {
			m.table.SetCursor(idx)
			break
		}
	}
}

func (m *mappingEditorModel) editSelected() {
	idx := m.table.Cursor()
	if idx >= 0 && idx < len(m.filteredMappings) {
		mp := m.filteredMappings[idx]
		dc := m.findDestCol(mp.DestColumn)
		form := newMappingEditFormModel(mp, dc, m.sourceCols)
		m.editForm = &form
	}
}

func (m mappingEditorModel) save() tea.Msg {
	// Validation
	unresolved := 0
	for _, mp := range m.mappings {
		dc := m.findDestCol(mp.DestColumn)
		icon, _ := m.mappingStatus(mp, dc)
		if icon == "⚠" {
			unresolved++
		}
	}
	if unresolved > 0 {
		return fmt.Errorf("cannot save: %d NOT NULL columns unresolved", unresolved)
	}

	for _, mp := range m.mappings {
		if mp.ValueMap.Valid {
			dc := m.findDestCol(mp.DestColumn)
			if err := storage.ValidateMapping(mp, dc); err != nil {
				return fmt.Errorf("validate mapping %s: %v", mp.DestColumn, err)
			}
		}
	}

	ctx := context.Background()
	_ = m.store.Mappings().DeleteByTable(ctx, m.conn.ID, m.tableName)
	err := m.store.Mappings().BulkInsert(ctx, m.mappings)
	if err != nil {
		return err
	}
	return successMsg{message: fmt.Sprintf("✓ Saved %d mappings", len(m.mappings))}
}
