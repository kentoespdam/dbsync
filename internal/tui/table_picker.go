package tui

import (
	"context"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/dbsync/internal/crypto"
	"github.com/user/dbsync/internal/mysql"
	"github.com/user/dbsync/internal/storage"
)

type tableItem struct {
	name     string
	isMapped bool
	hasPK    bool
}

func (i tableItem) Title() string       { return i.name }
func (i tableItem) Description() string {
	var badges []string
	if i.isMapped {
		badges = append(badges, "[mapped]")
	}
	if !i.hasPK {
		badges = append(badges, "[no-pk]")
	}
	return strings.Join(badges, " ")
}
func (i tableItem) FilterValue() string { return i.name }

type tablePickerModel struct {
	list      list.Model
	conn      storage.Connection
	masterKey []byte
	store     *storage.DB
	loading   bool
	spinner   spinner.Model
	err       error
	choice    string
	syncAll   bool
}

func newTablePickerModel(conn storage.Connection, masterKey []byte, store *storage.DB, flow string) tablePickerModel {
	l := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Select Table: " + conn.Name
	if flow == "sync" {
		l.Title = "Select Table to Sync: " + conn.Name
	}
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)

	return tablePickerModel{
		list:      l,
		conn:      conn,
		masterKey: masterKey,
		store:     store,
		loading:   true,
		spinner:   spinner.New(spinner.WithSpinner(spinner.Dot)),
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
		m.list.SetSize(msg.Width, msg.Height-2)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tablesLoadedMsg:
		m.loading = false
		m.list.SetItems(msg.items)
		return m, nil

	case error:
		m.err = msg
		m.loading = false
		return m, nil

	case tea.KeyMsg:
		if m.list.FilterState() == list.Filtering {
			break
		}

		switch msg.String() {
		case "enter":
			if i, ok := m.list.SelectedItem().(tableItem); ok {
				if i.name == "--- SYNC ALL MAPPED TABLES ---" {
					m.syncAll = true
					m.choice = "*"
				} else {
					m.choice = i.name
				}
			}
		case "r":
			m.loading = true
			return m, m.loadTables
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m tablePickerModel) View() string {
	if m.err != nil {
		return errorStyle.Render("Error: " + m.err.Error())
	}
	if m.loading {
		return "\n " + m.spinner.View() + " Loading tables..."
	}
	return m.list.View()
}
