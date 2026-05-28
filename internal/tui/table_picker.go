package tui

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/user/dbsync/internal/crypto"
	"github.com/user/dbsync/internal/mysql"
	"github.com/user/dbsync/internal/storage"
)

type tableItem struct {
	name     string
	isMapped bool
	hasPK    bool
}

func (i tableItem) Title()       string { return i.name }
func (i tableItem) Description() string { return "" }
func (i tableItem) FilterValue() string { return i.name }

type tableItemDelegate struct {
	list.DefaultDelegate
}

func newTableItemDelegate() tableItemDelegate {
	d := list.NewDefaultDelegate()
	return tableItemDelegate{d}
}

func (d tableItemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(tableItem)
	if !ok {
		return
	}

	icon := unmappedStyle.Render("○")
	if i.isMapped {
		icon = mappedStyle.Render("✓")
	}

	title := i.name
	var description string
	if !i.hasPK {
		description = warningStyle.Render(" [no-pk]")
	}

	if index == m.Index() {
		title = lipgloss.NewStyle().Foreground(lipgloss.Color("170")).Bold(true).Render(title)
	}

	fmt.Fprintf(w, " %s  %s%s", icon, title, description)
}

func (d tableItemDelegate) Height() int                             { return 1 }
func (d tableItemDelegate) Spacing() int                            { return 0 }
func (d tableItemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }

type tablePickerModel struct {
	list         list.Model
	allItems     []list.Item
	filterInput  textinput.Model
	unmappedOnly bool
	focused      int // 0: filter, 1: list

	conn      storage.Connection
	masterKey []byte
	store     *storage.DB
	loading   bool
	spinner   spinner.Model
	err       error
	choice    string
	syncAll   bool
	inited    bool
	width     int
	height    int
}

func newTablePickerModel(conn storage.Connection, masterKey []byte, store *storage.DB, flow string) tablePickerModel {
	l := list.New(nil, newTableItemDelegate(), 0, 0)
	l.Title = "Tables: " + conn.Name
	if flow == "sync" {
		l.Title = "Select Table to Sync: " + conn.Name
	}
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.SetShowTitle(false)

	ti := textinput.New()
	ti.Placeholder = "Filter tables..."
	ti.Focus()
	ti.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("170"))
	ti.TextStyle = ti.PromptStyle

	return tablePickerModel{
		list:        l,
		filterInput: ti,
		conn:        conn,
		masterKey:   masterKey,
		store:       store,
		loading:     true,
		spinner:     spinner.New(spinner.WithSpinner(spinner.Dot)),
		inited:      true,
	}
}

func (m tablePickerModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.loadTables)
}

func (m tablePickerModel) loadTables() tea.Msg {
	srcPass, err := crypto.Decrypt(m.conn.SourcePassword, m.masterKey)
	if err != nil {
		return err
	}
	pool, err := mysql.Open(mysql.Config{
		Host:     m.conn.SourceHost,
		Port:     m.conn.SourcePort,
		User:     m.conn.SourceUser,
		Password: string(srcPass),
		DBName:   m.conn.SourceDB,
	})
	if err != nil {
		return err
	}
	defer pool.Close()

	ctx := context.Background()
	tableNames, err := mysql.ListTables(ctx, pool.DB(), m.conn.SourceDB)
	if err != nil {
		return err
	}

	var items []list.Item
	if m.list.Title == "Select Table to Sync: "+m.conn.Name {
		items = append(items, tableItem{name: "--- SYNC ALL MAPPED TABLES ---", isMapped: true, hasPK: true})
	}
	for _, name := range tableNames {
		isMapped, _ := m.store.Mappings().Exists(ctx, m.conn.ID, name)
		pk, _ := mysql.DetectPK(ctx, pool.DB(), m.conn.SourceDB, name)
		items = append(items, tableItem{
			name:     name,
			isMapped: isMapped,
			hasPK:    len(pk) > 0,
		})
	}

	return tablesLoadedMsg{items}
}

type tablesLoadedMsg struct {
	items []list.Item
}

func (m tablePickerModel) Update(msg tea.Msg) (tablePickerModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.resize(msg.Width, msg.Height)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tablesLoadedMsg:
		m.loading = false
		m.allItems = msg.items
		m.applyFilter()
		m.resize(m.width, m.height) // statsLine may toggle → recompute list height
		return m, nil

	case error:
		m.err = msg
		m.loading = false
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			m.focused = (m.focused + 1) % 2
			if m.focused == 0 {
				m.filterInput.Focus()
			} else {
				m.filterInput.Blur()
			}
			return m, nil

		case "enter":
			if m.focused == 1 {
				if i, ok := m.list.SelectedItem().(tableItem); ok {
					if i.name == "--- SYNC ALL MAPPED TABLES ---" {
						m.syncAll = true
						m.choice = "*"
					} else {
						m.choice = i.name
					}
				}
				return m, nil
			}

		case "u":
			if m.focused == 1 {
				m.unmappedOnly = !m.unmappedOnly
				m.applyFilter()
				m.resize(m.width, m.height) // statsLine toggled → recompute
				return m, nil
			}

		case "r":
			if m.focused == 1 {
				m.loading = true
				return m, m.loadTables
			}

		case "esc":
			m.choice = ""
			return m, nil
		}
	}

	if m.focused == 0 {
		var cmd tea.Cmd
		prevHadStats := m.statsLineActive()
		m.filterInput, cmd = m.filterInput.Update(msg)
		m.applyFilter()
		if m.statsLineActive() != prevHadStats {
			m.resize(m.width, m.height) // statsLine toggled by typing/clearing filter
		}
		return m, cmd
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *tablePickerModel) applyFilter() {
	var filtered []list.Item
	text := strings.ToLower(m.filterInput.Value())

	for _, item := range m.allItems {
		ti := item.(tableItem)
		matchesText := text == "" || strings.Contains(strings.ToLower(ti.name), text)
		matchesUnmapped := !m.unmappedOnly || !ti.isMapped

		// Special case for sync all mapped tables item - keep it if it matches text
		if ti.name == "--- SYNC ALL MAPPED TABLES ---" {
			if matchesText {
				filtered = append(filtered, item)
			}
			continue
		}

		if matchesText && matchesUnmapped {
			filtered = append(filtered, item)
		}
	}
	m.list.SetItems(filtered)
}

// statsLineActive returns true when the secondary stats line (filter/unmapped-only)
// should be rendered. The stats line adds 1 line to chrome height.
func (m tablePickerModel) statsLineActive() bool {
	return m.filterInput.Value() != "" || m.unmappedOnly
}

// renderChrome builds the non-scrollable parts of the view: everything above
// the list (top) and everything below it (bottom). Pure function of model
// state — safe to measure with lipgloss.Height for layout calculation.
func (m tablePickerModel) renderChrome() (top, bottom string) {
	total, mapped, unmapped, noPK := 0, 0, 0, 0
	for _, item := range m.allItems {
		ti := item.(tableItem)
		if ti.name == "--- SYNC ALL MAPPED TABLES ---" {
			continue
		}
		total++
		if ti.isMapped {
			mapped++
		} else {
			unmapped++
		}
		if !ti.hasPK {
			noPK++
		}
	}

	title := titleStyle.Render(m.list.Title)
	stats1 := fmt.Sprintf("%d tables • %d mapped • %d unmapped • %d no-pk", total, mapped, unmapped, noPK)

	statsSegment := title + "\n" + stats1
	if m.statsLineActive() {
		var parts []string
		if m.filterInput.Value() != "" {
			parts = append(parts, fmt.Sprintf("Filter: '%s'", m.filterInput.Value()))
		}
		if m.unmappedOnly {
			parts = append(parts, "unmapped-only")
		}
		parts = append(parts, fmt.Sprintf("showing %d", len(m.list.Items())))
		statsSegment += "\n" + strings.Join(parts, " • ")
	}

	filterStyle := filterBlurStyle
	if m.focused == 0 {
		filterStyle = filterFocusStyle
	}
	filterSegment := filterStyle.Render(m.filterInput.View())

	helpLine1 := "tab focus • ↑↓ nav • enter open • u unmapped-only • r reload"
	helpLine2 := "esc back"
	helpSegment := helpStyle.Render(helpLine1 + "\n" + helpLine2)

	// One blank line between stats and filter for breathing room.
	top = statsSegment + "\n" + filterSegment
	bottom = helpSegment
	return top, bottom
}

// resize is the single source of truth for sizing the inner list. It
// measures actual rendered chrome height (header + filter + help) and
// gives the list the remaining vertical space. Guarantees a minimum
// usable list area of 3 rows even on very short terminals.
//
// Pointer receiver: mutates list and filterInput dimensions in place.
func (m *tablePickerModel) resize(width, height int) {
	m.width = width
	m.height = height
	m.filterInput.Width = width - 4 // border + padding
	top, bottom := m.renderChrome()
	// +1 for the blank line JoinVertical implies between top and list,
	// and another +1 between list and bottom. Keep this in sync with View().
	chromeH := lipgloss.Height(top) + lipgloss.Height(bottom) + 2
	listH := height - chromeH
	const minListH = 3
	if listH < minListH {
		listH = minListH
	}
	m.list.SetSize(width, listH)
}

func (m tablePickerModel) View() string {
	if m.err != nil {
		return errorStyle.Render("Error: " + m.err.Error())
	}
	if m.loading {
		return "\n " + m.spinner.View() + " Loading tables..."
	}

	top, bottom := m.renderChrome()
	listSegment := m.list.View()

	// Single JoinVertical, blank lines explicit via "\n" prefix on inner segments.
	// This matches the +2 accounted for in resize().
	return lipgloss.JoinVertical(lipgloss.Left,
		top,
		"\n"+listSegment,
		"\n"+bottom,
	)
}
