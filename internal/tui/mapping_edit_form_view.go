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

	// Value Map section (only for ENUM dest columns)
	if m.hasValueMap {
		vmStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
		if m.focused == 2 { vmStyle = vmStyle.BorderForeground(lipgloss.Color("170")) }
		s.WriteString("\nValue Map:\n")

		var vmContent strings.Builder
		if len(m.valueMapPairs) == 0 {
			vmContent.WriteString(infoStyle.Render(" passthrough (no mapping)") + "\n")
		} else {
			for i, p := range m.valueMapPairs {
				prefix := "  "
				if m.focused == 2 && m.valueMapEditing == 0 && i == m.valueMapCursor {
					prefix = "> "
				}
				line := fmt.Sprintf("%s%s → %s", prefix, p.Source, p.Destination)
				if m.focused == 2 && m.valueMapEditing == 0 && i == m.valueMapCursor {
					line = lipgloss.NewStyle().Foreground(lipgloss.Color("170")).Render(line)
				}
				vmContent.WriteString(line + "\n")
			}
		}

		if m.focused == 2 && m.valueMapEditing > 0 {
			if m.valueMapEditing == 1 {
				vmContent.WriteString("\n" + m.valueMapInput.View() + "\n")
			} else {
				for i, v := range m.valueMapDestHint {
					suffix := ""
					if i == m.valueMapCursor && m.valueMapEditing == 2 {
						suffix = " ←"
					}
					vmContent.WriteString(fmt.Sprintf("  %s%s\n", v, lipgloss.NewStyle().Foreground(lipgloss.Color("170")).Render(suffix)))
				}
				vmContent.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(" tab: switch • ↑↓: choose") + "\n")
			}
		} else if m.focused == 2 {
			vmContent.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(" a: add  e: edit  x: remove  ↑↓: browse") + "\n")
		}

		s.WriteString(vmStyle.Render(strings.TrimRight(vmContent.String(), "\n")) + "\n")
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
