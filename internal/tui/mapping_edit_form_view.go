package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m mappingEditFormModel) View() string {
	var s strings.Builder
	title := fmt.Sprintf("Edit Mapping: %s", m.destCol.Name)
	s.WriteString(titleStyle.Render(title) + "\n")
	s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(m.destCol.ColumnType) + "\n\n")

	// Source List
	srcStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	if m.focused == 0 { srcStyle = srcStyle.BorderForeground(lipgloss.Color("170")) }
	s.WriteString("Source Column:\n")
	s.WriteString(srcStyle.Render(m.sourceList.View()) + "\n\n")

	// Default Widget
	defStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	if m.focused == 1 { defStyle = defStyle.BorderForeground(lipgloss.Color("170")) }
	s.WriteString("Default Value:\n")

	if m.isBool {
		s.WriteString(defStyle.Render(m.renderBoolOptions()) + "\n")
		if m.focused == 1 { s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(" (space to toggle)") + "\n") }
	} else if m.isEnum {
		s.WriteString(defStyle.Render(m.enumList.View()) + "\n")
	} else {
		s.WriteString(defStyle.Render(m.input.View()) + "\n")
	}

	if m.errorMsg != "" {
		s.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(m.errorMsg) + "\n")
	}
	s.WriteString("\n" + helpStyle.Render("tab: switch • enter: save • esc: cancel"))
	return s.String()
}

func (m mappingEditFormModel) renderBoolOptions() string {
	options := []string{"(empty)", "true", "false"}
	var optStr []string
	for i, o := range options {
		prefix := "( ) "
		if m.boolVal == i { prefix = "(•) " }
		optStr = append(optStr, prefix+o)
	}
	return strings.Join(optStr, "\n")
}
