package tui

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/user/dbsync/internal/config"
	"github.com/user/dbsync/internal/crypto"
	"github.com/user/dbsync/internal/storage"
)

type passwordPromptModel struct {
	textInput    textinput.Model
	confirmInput textinput.Model
	isFirstRun   bool
	isConfirming bool
	salt         []byte
	masterKey    []byte
	success      bool
	err          error
	store        *storage.DB
}

func newPasswordPromptModel(store *storage.DB) passwordPromptModel {
	ti := textinput.New()
	ti.Placeholder = "Master Password"
	ti.EchoMode = textinput.EchoPassword
	ti.Focus()

	ci := textinput.New()
	ci.Placeholder = "Confirm Master Password"
	ci.EchoMode = textinput.EchoPassword

	m := passwordPromptModel{
		textInput:    ti,
		confirmInput: ci,
		store:        store,
	}

	saltPath, err := config.SaltPath()
	if err == nil {
		if _, err := os.Stat(saltPath); os.IsNotExist(err) {
			m.isFirstRun = true
		} else {
			m.salt, _ = os.ReadFile(saltPath)
		}
	}

	return m
}

func (m passwordPromptModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m passwordPromptModel) Update(msg tea.Msg) (passwordPromptModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if m.isFirstRun && !m.isConfirming {
				m.isConfirming = true
				m.confirmInput.Focus()
				return m, nil
			}

			// Validate
			password := m.textInput.Value()
			if m.isFirstRun {
				if password != m.confirmInput.Value() {
					m.err = errors.New("passwords do not match")
					m.textInput.Reset()
					m.confirmInput.Reset()
					m.isConfirming = false
					m.textInput.Focus()
					return m, nil
				}
				// Generate salt
				m.salt = make([]byte, crypto.MinSaltSize)
				if _, err := io.ReadFull(rand.Reader, m.salt); err != nil {
					m.err = err
					return m, nil
				}
				saltPath, _ := config.SaltPath()
				_ = os.MkdirAll(filepath.Dir(saltPath), 0700)
				_ = os.WriteFile(saltPath, m.salt, 0600)
			}

			key, err := crypto.DeriveKey(password, m.salt)
			if err != nil {
				m.err = err
				return m, nil
			}

			// Verify if there are existing connections
			if !m.isFirstRun && m.store != nil {
				conns, _ := m.store.Connections().List(context.Background())
				if len(conns) > 0 {
					// Try decrypt source password of first connection
					_, err := crypto.Decrypt(conns[0].SourcePassword, key)
					if err != nil {
						m.err = errors.New("wrong master password")
						m.textInput.Reset()
						return m, nil
					}
				}
			}

			m.masterKey = key
			m.success = true
			return m, nil
		}
	}

	if m.isFirstRun && m.isConfirming {
		m.confirmInput, cmd = m.confirmInput.Update(msg)
	} else {
		m.textInput, cmd = m.textInput.Update(msg)
	}

	return m, cmd
}

func (m passwordPromptModel) View() string {
	s := titleStyle.Render("Enter Master Password") + "\n\n"
	if m.isFirstRun {
		s = titleStyle.Render("Create Master Password (first run)") + "\n\n"
	}

	if m.isFirstRun && m.isConfirming {
		s += m.confirmInput.View()
	} else {
		s += m.textInput.View()
	}

	if m.err != nil {
		s += "\n\n" + errorStyle.Render("Error: "+m.err.Error())
	}

	return s
}
