package tui

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kentoespdam/dbsync/internal/mysql"
	"github.com/kentoespdam/dbsync/internal/storage"
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
	if !ok { return }
	str := "  " + i.name
	style := lipgloss.NewStyle().PaddingLeft(2)
	if index == m.Index() {
		style = style.Foreground(lipgloss.Color("170")).Bold(true)
		str = "> " + i.name
	}
	fmt.Fprint(w, style.Render(str))
}

type ValueMapPair struct {
	Source      string
	Destination string
}

type mappingEditFormModel struct {
	mapping    storage.Mapping
	destCol    mysql.Column
	sourceCols []mysql.Column

	sourceList list.Model
	enumList   list.Model
	input      textinput.Model

	focused int // 0: sourceList, 1: defaultWidget, 2: valueMap
	done    bool
	canceled bool

	isBool     bool
	isEnum     bool
	boolVal    int // 0: empty, 1: true, 2: false
	hasValueMap bool

	valueMapPairs   []ValueMapPair
	valueMapCursor  int
	valueMapEditing int // 0: none, 1: source, 2: dest
	valueMapEditIdx int // -1 = new, >=0 = replace pair index
	valueMapInput   textinput.Model
	valueMapDestHint []string

	errorMsg string
}

func newMappingEditFormModel(m storage.Mapping, destCol mysql.Column, sourceCols []mysql.Column) mappingEditFormModel {
	var srcItems []list.Item
	srcItems = append(srcItems, columnItem{name: "(none)"})
	for _, c := range sourceCols {
		srcItems = append(srcItems, columnItem{name: c.Name})
	}

	sl := list.New(srcItems, simpleDelegate{}, 30, 8)
	sl.Title = "Source Column"
	sl.SetShowHelp(false)
	sl.SetShowStatusBar(false)
	sl.SetFilteringEnabled(true)

	if m.SourceColumn.Valid {
		for i, item := range srcItems {
			if item.(columnItem).name == m.SourceColumn.String { sl.Select(i); break }
		}
	} else {
		sl.Select(0)
	}

	form := mappingEditFormModel{mapping: m, destCol: destCol, sourceCols: sourceCols, sourceList: sl, focused: 0}
	form.initDefaultWidget(m)
	form.initValueMap(m, destCol)
	return form
}

func (m *mappingEditFormModel) initDefaultWidget(mrg storage.Mapping) {
	if m.destCol.IsBool() {
		m.isBool = true
		if mrg.DefaultValue.Valid {
			switch strings.ToLower(mrg.DefaultValue.String) {
			case "true", "1": m.boolVal = 1
			case "false", "0": m.boolVal = 2
			}
		}
	} else if enums := m.destCol.EnumValues(); len(enums) > 0 {
		m.isEnum = true
		var enumItems []list.Item
		enumItems = append(enumItems, columnItem{name: "(empty)"})
		for _, e := range enums { enumItems = append(enumItems, columnItem{name: e}) }
		el := list.New(enumItems, simpleDelegate{}, 30, 5)
		el.Title = "Default Value (ENUM)"
		el.SetShowHelp(false)
		el.SetShowStatusBar(false)
		m.enumList = el
		if mrg.DefaultValue.Valid {
			for i, item := range enumItems {
				if item.(columnItem).name == mrg.DefaultValue.String { el.Select(i); break }
			}
		} else { el.Select(0) }
	} else {
		ti := textinput.New()
		ti.Placeholder = "Default value..."
		if mrg.DefaultValue.Valid { ti.SetValue(mrg.DefaultValue.String) }
		m.input = ti
	}
}

func (m *mappingEditFormModel) initValueMap(mrg storage.Mapping, destCol mysql.Column) {
	if enums := destCol.EnumValues(); len(enums) > 0 {
		m.hasValueMap = true
		m.valueMapDestHint = enums
		m.valueMapInput = textinput.New()
		m.valueMapInput.Placeholder = "Source value..."
		m.valueMapInput.Width = 20
		m.valueMapEditIdx = -1

		if mrg.ValueMap.Valid {
			var vmap map[string]string
			if err := json.Unmarshal([]byte(mrg.ValueMap.String), &vmap); err == nil {
				for src, dst := range vmap {
					m.valueMapPairs = append(m.valueMapPairs, ValueMapPair{Source: src, Destination: dst})
				}
			}
		}
	}
}

func (m mappingEditFormModel) Init() tea.Cmd {
	return textinput.Blink
}
