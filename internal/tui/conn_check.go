package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/dbsync/internal/crypto"
	"github.com/user/dbsync/internal/mysql"
	"github.com/user/dbsync/internal/redact"
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

// testAll runs in a tea.Cmd goroutine. It must NOT mutate m (value receiver);
// instead it returns testDoneMsg carrying the results, which Update applies.
func (m connTestModel) testAll() tea.Msg {
	return testDoneMsg{
		srcStatus: probe(m.conn.SourcePassword, m.masterKey, mysql.Config{
			Host: m.conn.SourceHost, Port: m.conn.SourcePort,
			User: m.conn.SourceUser, DBName: m.conn.SourceDB,
		}),
		dstStatus: probe(m.conn.DestPassword, m.masterKey, mysql.Config{
			Host: m.conn.DestHost, Port: m.conn.DestPort,
			User: m.conn.DestUser, DBName: m.conn.DestDB,
		}),
	}
}

// probe decrypts pw, opens a MySQL pool with cfg, and returns a user-visible
// status string. Errors are redacted (quoted values stripped) before display.
func probe(encPw string, masterKey []byte, cfg mysql.Config) string {
	pw, err := crypto.Decrypt(encPw, masterKey)
	if err != nil {
		return fmt.Sprintf("✗ Decryption failed: %s", redact.Error(err))
	}
	cfg.Password = string(pw)
	if _, err := mysql.Open(cfg); err != nil {
		return fmt.Sprintf("✗ %s", redact.Error(err))
	}
	return "✓ OK"
}

type testDoneMsg struct {
	srcStatus string
	dstStatus string
}

func (m connTestModel) Update(msg tea.Msg) (connTestModel, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case testDoneMsg:
		m.loading = false
		m.srcStatus = msg.srcStatus
		m.dstStatus = msg.dstStatus
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
