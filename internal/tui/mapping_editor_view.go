package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/user/dbsync/internal/mysql"
	"github.com/user/dbsync/internal/storage"
)

func (m mappingEditorModel) View() string {
	if m.err != nil {
		return errorStyle.Render("Error: " + m.err.Error())
	}
	if m.loading {
		return "\n " + m.spinner.View() + " Loading mapping data..."
	}

	header := m.renderHeader()
	table := m.table.View()
	
	footer := m.renderFooter()
	if m.filtering {
		footer = "\n" + m.filterInput.View() + "\n" + footer
	}

	base := lipgloss.JoinVertical(lipgloss.Left, header, table, footer)

	if m.editForm != nil {
		modal := lipgloss.NewStyle().
			Width(60).
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1).
			Render(m.editForm.View())

		return lipgloss.Place(m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			modal,
			lipgloss.WithWhitespaceChars(" "),
			lipgloss.WithWhitespaceForeground(lipgloss.Color("240")),
		)
	}

	return base
}

func (m mappingEditorModel) renderHeader() string {
	title := titleStyle.Render(fmt.Sprintf("Mapping: %s", m.tableName))
	srcDst := fmt.Sprintf("%s.%s  →  %s.%s", m.conn.SourceDB, m.tableName, m.conn.DestDB, m.tableName)
	
	// Stats
	total := len(m.mappings)
	mapped, def, unresolved := 0, 0, 0
	for _, mp := range m.mappings {
		dc := m.findDestCol(mp.DestColumn)
		icon, _ := m.mappingStatus(mp, dc)
		switch icon {
		case "✓": mapped++
		case "●": def++
		case "⚠": unresolved++
		}
	}
	stats := fmt.Sprintf("%d cols • %d mapped • %d default • %d ⚠ unresolved", total, mapped, def, unresolved)
	
	return fmt.Sprintf("%s\n%s\n%s\n", title, srcDst, stats)
}

func (m mappingEditorModel) renderFooter() string {
	// 2-line context help
	line1 := "e edit  n add-extra-dest  d delete  / filter  w warnings-only"
	line2 := "N next-warning   r reset   s save   esc back"
	
	help := helpStyle.Render(line1 + "\n" + line2)
	if m.dirty {
		help = lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Render("UNSAVED CHANGES • ") + help
	}
	
	// Selection info
	idx := m.table.Cursor()
	selection := ""
	if idx >= 0 && idx < len(m.filteredMappings) {
		mp := m.filteredMappings[idx]
		dc := m.findDestCol(mp.DestColumn)
		selection = fmt.Sprintf("\nSelected: %s\nType: %s%s  •  Status: %s", 
			mp.DestColumn, dc.ColumnType, ifThen(dc.IsNullable, "", " NOT NULL"), m.statusText(mp, dc))
	}

	return selection + "\n\n" + help
}

func (m mappingEditorModel) mappingStatus(mp storage.Mapping, dc mysql.Column) (string, lipgloss.Style) {
	if mp.SourceColumn.Valid {
		return "✓", lipgloss.NewStyle().Foreground(lipgloss.Color("10")) // Green
	}
	if mp.DefaultValue.Valid {
		return "●", lipgloss.NewStyle().Foreground(lipgloss.Color("44")) // Cyan
	}
	if !dc.IsNullable {
		return "⚠", lipgloss.NewStyle().Foreground(lipgloss.Color("208")) // Orange
	}
	return "○", lipgloss.NewStyle().Foreground(lipgloss.Color("240")) // Gray
}

func (m mappingEditorModel) statusText(mp storage.Mapping, dc mysql.Column) string {
	icon, _ := m.mappingStatus(mp, dc)
	switch icon {
	case "✓": return "mapped via source"
	case "●": return "default-only"
	case "⚠": return "broken (needs source or default)"
	case "○": return "skipped (nullable)"
	}
	return ""
}

func ifThen(cond bool, a, b string) string {
	if cond { return a }
	return b
}
