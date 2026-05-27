package tui

import (
	"database/sql"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

func (m mappingEditFormModel) Update(msg tea.Msg) (mappingEditFormModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "shift+tab":
			m.focused = (m.focused + 1) % 2
			if m.focused == 1 && !m.isBool && !m.isEnum { m.input.Focus() } else { m.input.Blur() }
			return m, nil
		case "enter":
			if m.focused == 0 && m.sourceList.FilterState() == list.Filtering { break }
			return m.handleApply()
		case "esc":
			if m.focused == 0 && m.sourceList.FilterState() == list.Filtering {
				m.sourceList.ResetFilter()
				return m, nil
			}
			m.canceled, m.done = true, true
			return m, nil
		case " ":
			if m.focused == 1 && m.isBool {
				m.boolVal = (m.boolVal + 1) % 3
				return m, nil
			}
		}
	}

	var cmd tea.Cmd
	if m.focused == 0 {
		m.sourceList, cmd = m.sourceList.Update(msg)
	} else if m.isEnum {
		m.enumList, cmd = m.enumList.Update(msg)
	} else if !m.isBool {
		m.input, cmd = m.input.Update(msg)
	}
	return m, cmd
}

func (m mappingEditFormModel) handleApply() (mappingEditFormModel, tea.Cmd) {
	sel := m.sourceList.SelectedItem().(columnItem)
	if sel.name == "(none)" {
		m.mapping.SourceColumn = sql.NullString{Valid: false}
	} else {
		m.mapping.SourceColumn = sql.NullString{String: sel.name, Valid: true}
	}

	m.applyDefaultValue()

	if !m.destCol.IsNullable && !m.mapping.SourceColumn.Valid && !m.mapping.DefaultValue.Valid {
		m.errorMsg = "NOT NULL column needs source or default"
		return m, nil
	}

	m.done = true
	return m, nil
}

func (m *mappingEditFormModel) applyDefaultValue() {
	if m.isBool {
		switch m.boolVal {
		case 1: m.mapping.DefaultValue = sql.NullString{String: "true", Valid: true}
		case 2: m.mapping.DefaultValue = sql.NullString{String: "false", Valid: true}
		default: m.mapping.DefaultValue = sql.NullString{Valid: false}
		}
	} else if m.isEnum {
		sel := m.enumList.SelectedItem().(columnItem)
		if sel.name == "(empty)" { m.mapping.DefaultValue = sql.NullString{Valid: false} } else {
			m.mapping.DefaultValue = sql.NullString{String: sel.name, Valid: true}
		}
	} else {
		val := m.input.Value()
		if val == "" { m.mapping.DefaultValue = sql.NullString{Valid: false} } else {
			m.mapping.DefaultValue = sql.NullString{String: val, Valid: true}
		}
	}
}
