package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

func (m model) View() string {
	var view string
	if m.showDeleteConfirm && m.connToDelete != nil {
		view = fmt.Sprintf("\nDelete connection '%s'?\n\n(y/N)", m.connToDelete.Name)
	} else if m.showDiscardConfirm {
		view = "\nDiscard unsaved mapping changes?\n\n(y/N)"
	} else {
		view = m.renderScreen()
	}

	if m.toastMsg != "" {
		toast := lipgloss.Place(m.width, 3, lipgloss.Center, lipgloss.Center, m.renderToast())
		return view + "\n" + toast
	}

	return view
}

func (m model) renderScreen() string {
	switch m.current {
	case screenPasswordPrompt: return m.pwdPrompt.View()
	case screenMain:           return m.mainMenu.View()
	case screenConnList:       return m.connList.View()
	case screenConnForm:       return m.connForm.View()
	case screenConnTest:       return m.connTest.View()
	case screenConnPicker:     return m.connPicker.View()
	case screenTablePicker:    return m.tablePicker.View()
	case screenMappingEditor:  return m.mappingEditor.View()
	case screenRunSync:        return m.runSync.View()
	case screenHistory:        return m.historyViewer.View()
	case screenCheckpoints:    return m.checkpointViewer.View()
	default:                   return "Unknown screen"
	}
}
