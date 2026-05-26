package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/user/dbsync/internal/crypto"
	"github.com/user/dbsync/internal/mysql"
	"github.com/user/dbsync/internal/storage"
)

type connFormModel struct {
	inputs       []textinput.Model
	focused      int
	store        *storage.DB
	masterKey    []byte
	isEdit       bool
	connectionID int64
	loading      bool
	spinner      spinner.Model
	loadingMsg   string
	showConfirm  bool
	success      bool
	err          error
}

const (
	fName = iota
	fSrcHost
	fSrcPort
	fSrcUser
	fSrcPass
	fSrcDB
	fDstHost
	fDstPort
	fDstUser
	fDstPass
	fDstDB
)

func newConnFormModel(db *storage.DB, masterKey []byte, editConn *storage.Connection) connFormModel {
	inputs := make([]textinput.Model, 11)
	for i := range inputs {
		inputs[i] = textinput.New()
	}

	inputs[fName].Placeholder = "Connection Name"
	inputs[fSrcHost].Placeholder = "Source Host"
	inputs[fSrcPort].Placeholder = "3306"
	inputs[fSrcUser].Placeholder = "root"
	inputs[fSrcPass].Placeholder = "password"
	inputs[fSrcPass].EchoMode = textinput.EchoPassword
	inputs[fSrcDB].Placeholder = "source_db"

	inputs[fDstHost].Placeholder = "Dest Host"
	inputs[fDstPort].Placeholder = "3306"
	inputs[fDstUser].Placeholder = "root"
	inputs[fDstPass].Placeholder = "password"
	inputs[fDstPass].EchoMode = textinput.EchoPassword
	inputs[fDstDB].Placeholder = "dest_db"

	m := connFormModel{
		inputs:    inputs,
		store:     db,
		masterKey: masterKey,
		spinner:   spinner.New(spinner.WithSpinner(spinner.Dot)),
	}

	if editConn != nil {
		m.isEdit = true
		m.connectionID = editConn.ID
		m.inputs[fName].SetValue(editConn.Name)
		m.inputs[fSrcHost].SetValue(editConn.SourceHost)
		m.inputs[fSrcPort].SetValue(strconv.Itoa(editConn.SourcePort))
		m.inputs[fSrcUser].SetValue(editConn.SourceUser)
		m.inputs[fSrcPass].Placeholder = "(leave empty to keep unchanged)"
		m.inputs[fSrcDB].SetValue(editConn.SourceDB)
		m.inputs[fDstHost].SetValue(editConn.DestHost)
		m.inputs[fDstPort].SetValue(strconv.Itoa(editConn.DestPort))
		m.inputs[fDstUser].SetValue(editConn.DestUser)
		m.inputs[fDstPass].Placeholder = "(leave empty to keep unchanged)"
		m.inputs[fDstDB].SetValue(editConn.DestDB)
	} else {
		m.inputs[fSrcPort].SetValue("3306")
		m.inputs[fDstPort].SetValue("3306")
	}

	m.inputs[m.focused].Focus()
	return m
}

func (m connFormModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m connFormModel) Update(msg tea.Msg) (connFormModel, tea.Cmd) {
	if m.loading {
		switch msg := msg.(type) {
		case spinner.TickMsg:
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		case testResult:
			if msg.err != nil {
				m.loading = false
				m.err = msg.err
				m.showConfirm = true
				return m, nil
			}
			m.loadingMsg = msg.nextMsg
			if msg.nextCmd != nil {
				return m, msg.nextCmd
			}
			// Both OK, save
			return m, m.save(false)
		}
	}

	if m.showConfirm {
		if msg, ok := msg.(tea.KeyMsg); ok {
			switch strings.ToLower(msg.String()) {
			case "y":
				m.showConfirm = false
				return m, m.save(true)
			case "n", "esc":
				m.showConfirm = false
				m.err = nil
				return m, nil
			}
		}
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "enter":
			if m.focused == len(m.inputs)-1 && msg.String() == "enter" {
				return m, m.validateAndTest()
			}
			m.inputs[m.focused].Blur()
			m.focused = (m.focused + 1) % len(m.inputs)
			m.inputs[m.focused].Focus()
			return m, nil

		case "shift+tab":
			m.inputs[m.focused].Blur()
			m.focused = (m.focused - 1 + len(m.inputs)) % len(m.inputs)
			m.inputs[m.focused].Focus()
			return m, nil

		case "esc":
			return m, nil // handled by app.go
		}
	}

	var cmds []tea.Cmd
	for i := range m.inputs {
		var cmd tea.Cmd
		m.inputs[i], cmd = m.inputs[i].Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m connFormModel) validateAndTest() tea.Cmd {
	// Simple validation
	if m.inputs[fName].Value() == "" {
		m.err = fmt.Errorf("name is required")
		return nil
	}
	m.err = nil
	m.loading = true
	m.loadingMsg = "Testing source connection..."
	return tea.Batch(m.spinner.Tick, m.testSource)
}

type testResult struct {
	err     error
	nextMsg string
	nextCmd tea.Cmd
}

func (m connFormModel) testSource() tea.Msg {
	cfg := m.getConfig(true)
	// If edit and password empty, we can't test unless we decrypt existing
	if m.isEdit && m.inputs[fSrcPass].Value() == "" {
		// Skip test for now or load existing
		return testResult{nextMsg: "Testing destination connection...", nextCmd: m.testDest}
	}

	_, err := mysql.Open(cfg)
	if err != nil {
		return testResult{err: fmt.Errorf("source connection failed: %v", err)}
	}
	return testResult{nextMsg: "Testing destination connection...", nextCmd: m.testDest}
}

func (m connFormModel) testDest() tea.Msg {
	cfg := m.getConfig(false)
	if m.isEdit && m.inputs[fDstPass].Value() == "" {
		return testResult{}
	}

	_, err := mysql.Open(cfg)
	if err != nil {
		return testResult{err: fmt.Errorf("destination connection failed: %v", err)}
	}
	return testResult{}
}

func (m connFormModel) getConfig(source bool) mysql.Config {
	prefix := fSrcHost
	if !source {
		prefix = fDstHost
	}

	port, _ := strconv.Atoi(m.inputs[prefix+1].Value())
	return mysql.Config{
		Host:     m.inputs[prefix].Value(),
		Port:     port,
		User:     m.inputs[prefix+2].Value(),
		Password: m.inputs[prefix+3].Value(),
		DBName:   m.inputs[prefix+4].Value(),
	}
}

func (m connFormModel) save(ignoreErrors bool) tea.Cmd {
	return func() tea.Msg {
		c := storage.Connection{
			ID:         m.connectionID,
			Name:       m.inputs[fName].Value(),
			SourceHost: m.inputs[fSrcHost].Value(),
			SourceDB:   m.inputs[fSrcDB].Value(),
			SourceUser: m.inputs[fSrcUser].Value(),
			DestHost:   m.inputs[fDstHost].Value(),
			DestDB:     m.inputs[fDstDB].Value(),
			DestUser:   m.inputs[fDstUser].Value(),
		}
		c.SourcePort, _ = strconv.Atoi(m.inputs[fSrcPort].Value())
		c.DestPort, _ = strconv.Atoi(m.inputs[fDstPort].Value())

		// Handle passwords
		var existing storage.Connection
		if m.isEdit {
			var err error
			existing, err = m.store.Connections().GetByID(context.Background(), m.connectionID)
			if err != nil {
				return err
			}
			c.SourcePassword = existing.SourcePassword
			c.DestPassword = existing.DestPassword
		}

		if m.inputs[fSrcPass].Value() != "" {
			enc, err := crypto.Encrypt([]byte(m.inputs[fSrcPass].Value()), m.masterKey)
			if err != nil {
				return err
			}
			c.SourcePassword = enc
		}
		if m.inputs[fDstPass].Value() != "" {
			enc, err := crypto.Encrypt([]byte(m.inputs[fDstPass].Value()), m.masterKey)
			if err != nil {
				return err
			}
			c.DestPassword = enc
		}

		var err error
		if m.isEdit {
			err = m.store.Connections().Update(context.Background(), c)
		} else {
			_, err = m.store.Connections().Insert(context.Background(), c)
		}

		if err != nil {
			return err
		}
		return successMsg{}
	}
}

type successMsg struct{}

func (m connFormModel) View() string {
	if m.loading {
		return fmt.Sprintf("\n %s %s", m.spinner.View(), m.loadingMsg)
	}

	if m.showConfirm {
		return fmt.Sprintf("\n%s: %v\n\nSave anyway? (y/N)", errorStyle.Render("Error"), m.err)
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("Connection Details") + "\n\n")

	labels := []string{
		"Name", "Src Host", "Src Port", "Src User", "Src Pass", "Src DB",
		"Dst Host", "Dst Port", "Dst User", "Dst Pass", "Dst DB",
	}

	for i, input := range m.inputs {
		b.WriteString(fmt.Sprintf("%-10s %s\n", labels[i], input.View()))
	}

	if m.err != nil {
		b.WriteString("\n" + errorStyle.Render(m.err.Error()) + "\n")
	}

	b.WriteString("\n" + helpStyle.Render("tab/shift+tab: navigate • enter: next/submit • esc: cancel"))
	return b.String()
}
