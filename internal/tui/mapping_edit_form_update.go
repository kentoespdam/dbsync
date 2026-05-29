package tui

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kentoespdam/dbsync/internal/storage"
)

func (m mappingEditFormModel) Update(msg tea.Msg) (mappingEditFormModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "shift+tab":
			maxFocus := 1
			if m.hasValueMap { maxFocus = 2 }
			m.focused = (m.focused + 1) % (maxFocus + 1)
			if m.focused == 1 && !m.isBool && !m.isEnum { m.input.Focus() } else { m.input.Blur() }
			if m.focused != 2 {
				if m.valueMapEditing > 0 { m.valueMapInput.Blur() }
				m.valueMapEditing = 0
				m.valueMapEditIdx = -1
			}
			return m, nil
		case "enter":
			if m.focused == 0 && m.sourceList.FilterState() == list.Filtering { break }
			if m.focused == 2 && m.valueMapEditing == 1 {
				val := m.valueMapInput.Value()
				if val != "" {
					m.valueMapEditing = 2
					m.valueMapCursor = 0
				}
				return m, nil
			}
			if m.focused == 2 && m.valueMapEditing == 2 {
				dst := m.valueMapDestHint[m.valueMapCursor]
				if m.valueMapEditIdx >= 0 {
					m.valueMapPairs[m.valueMapEditIdx] = ValueMapPair{
						Source: m.valueMapInput.Value(), Destination: dst,
					}
					m.valueMapEditIdx = -1
				} else {
					m.valueMapPairs = append(m.valueMapPairs, ValueMapPair{
						Source: m.valueMapInput.Value(), Destination: dst,
					})
				}
				m.valueMapInput.SetValue("")
				m.valueMapEditing = 0
				return m, nil
			}
			return m.handleApply()
		case "esc":
			if m.focused == 0 && m.sourceList.FilterState() == list.Filtering {
				m.sourceList.ResetFilter()
				return m, nil
			}
			if m.focused == 2 && m.valueMapEditing > 0 {
				m.valueMapEditing = 0
				m.valueMapInput.SetValue("")
				m.valueMapEditIdx = -1
				return m, nil
			}
			m.canceled, m.done = true, true
			return m, nil
		case "up":
			if m.focused == 2 && m.valueMapEditing == 2 {
				if m.valueMapCursor > 0 { m.valueMapCursor-- }
				return m, nil
			}
			if m.focused == 2 && m.valueMapEditing == 0 && len(m.valueMapPairs) > 0 {
				if m.valueMapCursor > 0 { m.valueMapCursor-- }
				return m, nil
			}
		case "down":
			if m.focused == 2 && m.valueMapEditing == 2 {
				if m.valueMapCursor < len(m.valueMapDestHint)-1 { m.valueMapCursor++ }
				return m, nil
			}
			if m.focused == 2 && m.valueMapEditing == 0 && len(m.valueMapPairs) > 0 {
				if m.valueMapCursor < len(m.valueMapPairs)-1 { m.valueMapCursor++ }
				return m, nil
			}
		case "a":
			if m.focused == 2 && m.valueMapEditing == 0 {
				m.valueMapInput.Placeholder = "Source value..."
				m.valueMapEditing = 1
				m.valueMapInput.Focus()
				return m, nil
			}
		case "e":
			if m.focused == 2 && m.valueMapEditing == 0 && len(m.valueMapPairs) > 0 && m.valueMapCursor < len(m.valueMapPairs) {
				m.valueMapInput.SetValue(m.valueMapPairs[m.valueMapCursor].Source)
				m.valueMapEditIdx = m.valueMapCursor
				m.valueMapInput.Placeholder = "Source value..."
				m.valueMapEditing = 1
				m.valueMapInput.Focus()
				return m, nil
			}
		case "x":
			if m.focused == 2 && m.valueMapEditing == 0 && len(m.valueMapPairs) > 0 {
				if m.valueMapCursor < len(m.valueMapPairs) {
					m.valueMapPairs = append(m.valueMapPairs[:m.valueMapCursor], m.valueMapPairs[m.valueMapCursor+1:]...)
					if m.valueMapCursor >= len(m.valueMapPairs) && m.valueMapCursor > 0 {
						m.valueMapCursor--
					}
				}
				return m, nil
			}
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
	} else if m.focused == 1 && m.isEnum {
		m.enumList, cmd = m.enumList.Update(msg)
	} else if m.focused == 1 && !m.isBool {
		m.input, cmd = m.input.Update(msg)
	} else if m.focused == 2 && m.valueMapEditing == 1 {
		m.valueMapInput, cmd = m.valueMapInput.Update(msg)
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

	if m.hasValueMap && len(m.valueMapPairs) > 0 {
		vmap := make(map[string]string)
		for _, p := range m.valueMapPairs {
			vmap[p.Source] = p.Destination
		}
		data, err := json.Marshal(vmap)
		if err != nil {
			m.errorMsg = fmt.Sprintf("value_map: %v", err)
			return m, nil
		}
		m.mapping.ValueMap = sql.NullString{String: string(data), Valid: true}

		if err := storage.ValidateMapping(m.mapping, m.destCol); err != nil {
			m.errorMsg = err.Error()
			return m, nil
		}
	} else {
		m.mapping.ValueMap = sql.NullString{Valid: false}
	}

	if !m.destCol.IsNullable && !m.mapping.SourceColumn.Valid && !m.mapping.DefaultValue.Valid && !m.mapping.ValueMap.Valid {
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
