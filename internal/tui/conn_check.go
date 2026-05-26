package tui

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/user/dbsync/internal/crypto"
	"github.com/user/dbsync/internal/mysql"
	"github.com/user/dbsync/internal/storage"
)

type connTestModel struct {
	conn      storage.Connection
	masterKey []byte
	srcStatus string
	dstStatus string
	loading   bool
	spinner   spinner.Model
}

func newConnTestModel(c storage.Connection, masterKey []byte) connTestModel {
	return connTestModel{
		conn:      c,
		masterKey: masterKey,
		loading:   true,
		spinner:   spinner.New(spinner.WithSpinner(spinner.Dot)),
	}
}

func (m connTestModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.testAll)
}

func (m connTestModel) testAll() tea.Msg {
	// Test Source
	srcPass, err := crypto.Decrypt(m.conn.SourcePassword, m.masterKey)
	if err != nil {
		m.srcStatus = "✗ Decryption failed"
	} else {
		_, err = mysql.Open(mysql.Config{
			Host:     m.conn.SourceHost,
			Port:     m.conn.SourcePort,
			User:     m.conn.SourceUser,
			Password: string(srcPass),
			DBName:   m.conn.SourceDB,
		})
		if err != nil {
			m.srcStatus = fmt.Sprintf("✗ %v", err)
		} else {
			m.srcStatus = "✓ OK"
		}
	}

	// Test Dest
	dstPass, err := crypto.Decrypt(m.conn.DestPassword, m.masterKey)
	if err != nil {
		m.dstStatus = "✗ Decryption failed"
	} else {
		_, err = mysql.Open(mysql.Config{
			Host:     m.conn.DestHost,
			Port:     m.conn.DestPort,
			User:     m.conn.DestUser,
			Password: string(dstPass),
			DBName:   m.conn.DestDB,
		})
		if err != nil {
			m.dstStatus = fmt.Sprintf("✗ %v", err)
		} else {
			m.dstStatus = "✓ OK"
		}
	}

	return testDoneMsg{}
}

type testDoneMsg struct{}

func (m connTestModel) Update(msg tea.Msg) (connTestModel, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case testDoneMsg:
		m.loading = false
		return m, nil
	}
	return m, nil
}

func (m connTestModel) View() string {
	s := fmt.Sprintf("Testing Connection: %s\n\n", m.conn.Name)
	if m.loading {
		s += m.spinner.View() + " Testing..."
	} else {
		s += fmt.Sprintf("Source: %s\n", m.srcStatus)
		s += fmt.Sprintf("Dest:   %s\n", m.dstStatus)
		s += "\nPress any key to return"
	}
	return s
}
