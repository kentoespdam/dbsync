package tui

import (
	"database/sql"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/user/dbsync/internal/mysql"
	"github.com/user/dbsync/internal/storage"
)

type columnItem struct {
	name string
}

func (i columnItem) Title() string       { return i.name }
func (i columnItem) Description() string { return "" }
func (i columnItem) FilterValue() string { return i.name }

type simpleDelegate struct{}

func (d simpleDelegate) Height() int                             { return 1 }
func (d simpleDelegate) Spacing() int                            { return 0 }
func (d simpleDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }
func (d simpleDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(columnItem)
	if !ok {
		return
	}
	str := i.name
	style := lipgloss.NewStyle().PaddingLeft(2)
	if index == m.Index() {
		style = style.Foreground(lipgloss.Color("170")).Bold(true)
		str = "> " + str
	} else {
		str = "  " + str
	}
	fmt.Fprint(w, style.Render(str))
}

type mappingEditFormModel struct {
	mapping    storage.Mapping
	sourceCols []mysql.Column
	destCols   []mysql.Column
	isNew      bool

	destList   list.Model
	sourceList list.Model
	input      textinput.Model
	focused    int // 0: destList (if isNew), 1: sourceList, 2: input
	done       bool
	canceled   bool
	width, height int
}

func newMappingEditFormModel(m storage.Mapping, sourceCols []mysql.Column, destCols []mysql.Column, isNew bool) mappingEditFormModel {
	var srcItems []list.Item
	srcItems = append(srcItems, columnItem{name: "(none / use default)"})
	for _, c := range sourceCols {
		srcItems = append(srcItems, columnItem{name: c.Name})
	}

	sl := list.New(srcItems, simpleDelegate{}, 40, 10)
	sl.Title = "Source Column"
	sl.SetShowHelp(false)
	sl.SetShowStatusBar(false)
	sl.SetFilteringEnabled(true)

	// Set initial source selection
	if m.SourceColumn.Valid {
		for i, item := range srcItems {
			if item.(columnItem).name == m.SourceColumn.String {
				sl.Select(i)
				break
			}
		}
	} else {
		sl.Select(0)
	}

	ti := textinput.New()
	ti.Placeholder = "Default Value (e.g. 42, 'literal', NOW())"
	if m.DefaultValue.Valid {
		ti.SetValue(m.DefaultValue.String)
	}

	form := mappingEditFormModel{
		mapping:    m,
		sourceCols: sourceCols,
		destCols:   destCols,
		isNew:      isNew,
		sourceList: sl,
		input:      ti,
		focused:    1,
	}

	if isNew {
		var dstItems []list.Item
		for _, c := range destCols {
			dstItems = append(dstItems, columnItem{name: c.Name})
		}
		dl := list.New(dstItems, simpleDelegate{}, 40, 10)
		dl.Title = "Select Destination Column"
		dl.SetShowHelp(false)
		dl.SetShowStatusBar(false)
		dl.SetFilteringEnabled(true)
		form.destList = dl
		form.focused = 0
	}

	if form.focused == 2 {
		form.input.Focus()
	}

	return form
}

func (m mappingEditFormModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m mappingEditFormModel) Update(msg tea.Msg) (mappingEditFormModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.sourceList.SetWidth(msg.Width - 10)
		if m.isNew {
			m.destList.SetWidth(msg.Width - 10)
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			if m.isNew {
				m.focused = (m.focused + 1) % 3
			} else {
				m.focused = (m.focused%2 + 1) % 2
				if m.focused == 0 {
					m.focused = 1 // skip destList
				}
			}

			if m.focused == 2 {
				m.input.Focus()
			} else {
				m.input.Blur()
			}
			return m, nil

		case "enter":
			if m.focused == 0 && m.destList.FilterState() == list.Filtering {
				break
			}
			if m.focused == 1 && m.sourceList.FilterState() == list.Filtering {
				break
			}

			// Apply
			if m.isNew {
				sel := m.destList.SelectedItem().(columnItem)
				m.mapping.DestColumn = sel.name
			}

			sel := m.sourceList.SelectedItem().(columnItem)
			if sel.name == "(none / use default)" {
				m.mapping.SourceColumn = sql.NullString{Valid: false}
			} else {
				m.mapping.SourceColumn = sql.NullString{String: sel.name, Valid: true}
			}
			val := m.input.Value()
			if val == "" {
				m.mapping.DefaultValue = sql.NullString{Valid: false}
			} else {
				m.mapping.DefaultValue = sql.NullString{String: val, Valid: true}
			}
			m.done = true
			return m, nil

		case "esc":
			if m.isNew && m.destList.FilterState() == list.Filtering {
				break
			}
			if m.sourceList.FilterState() == list.Filtering {
				break
			}
			m.canceled = true
			m.done = true
			return m, nil
		}
	}

	var cmd tea.Cmd
	if m.focused == 0 {
		m.destList, cmd = m.destList.Update(msg)
	} else if m.focused == 1 {
		m.sourceList, cmd = m.sourceList.Update(msg)
	} else {
		m.input, cmd = m.input.Update(msg)
	}
	return m, cmd
}

func (m mappingEditFormModel) View() string {
	s := strings.Builder{}
	title := "Editing mapping for: " + m.mapping.DestColumn
	if m.isNew {
		title = "Adding new mapping"
	}
	s.WriteString(titleStyle.Render(title) + "\n\n")

	if m.isNew {
		listStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1)
		if m.focused == 0 {
			listStyle = listStyle.BorderForeground(lipgloss.Color("62"))
		}
		s.WriteString("Dest Column:\n")
		s.WriteString(listStyle.Render(m.destList.View()))
		s.WriteString("\n\n")
	}

	listStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1)
	if m.focused == 1 {
		listStyle = listStyle.BorderForeground(lipgloss.Color("62"))
	}
	s.WriteString("Source Column:\n")
	s.WriteString(listStyle.Render(m.sourceList.View()))
	s.WriteString("\n\n")

	inputStyle := lipgloss.NewStyle()
	if m.focused == 2 {
		inputStyle = inputStyle.Foreground(lipgloss.Color("62"))
	}
	s.WriteString("Default Value: " + m.input.View() + "\n")

	s.WriteString("\n" + helpStyle.Render("tab: switch field • enter: apply • esc: cancel"))
	return s.String()
}
