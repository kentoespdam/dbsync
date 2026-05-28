package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func (m runScreenModel) View() string {
	header := m.renderHeader()
	var body string
	switch m.status {
	case "prompt_resume": body = m.renderPrompt("Previous run was interrupted.\n\n[r] Resume from last checkpoint\n[f] Fresh start\n[esc] Cancel", "62")
	case "confirm_cancel": body = m.renderPrompt("Cancel sync? Progress will be saved as checkpoint.\n\n[y] Yes, cancel\n[n] No, continue", "9")
	case "running", "completed", "failed", "interrupted":
		body = m.renderMain()
		if m.status != "running" {
			body = lipgloss.JoinVertical(lipgloss.Left, body, m.renderSummary(), "\n[esc] Back")
		} else {
			body = lipgloss.JoinVertical(lipgloss.Left, body, "\n[c] cancel  [esc] back when done")
		}
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, body)
}

func (m runScreenModel) renderHeader() string {
	h := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62")).
		Render(fmt.Sprintf("Sync [%d/%d]: %s / %s", m.tableIdx+1, len(m.tables), m.conn.Name, m.currentTable))
	if m.dryRun { h += lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render(" [DRY RUN]") }
	return h
}

func (m runScreenModel) renderPrompt(text, color string) string {
	p := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(color)).Padding(1, 2).Render(text)
	return lipgloss.Place(m.width, m.height-2, lipgloss.Center, lipgloss.Center, p)
}

func (m runScreenModel) renderMain() string {
	percent := 0.0
	if m.estimated > 0 { percent = float64(m.totalRows) / float64(m.estimated) }
	
	eta := "Calculating..."
	if m.totalRows > 0 {
		elapsed := time.Since(m.startTime)
		rowsPerSec := float64(m.totalRows) / elapsed.Seconds()
		if rowsPerSec > 0.1 && m.estimated > m.totalRows {
			remaining := float64(m.estimated-m.totalRows) / rowsPerSec
			eta = time.Duration(remaining * float64(time.Second)).Round(time.Second).String()
		}
	}

	prog := fmt.Sprintf("Batch %d | Rows %d | ETA %s\n%s", m.batches, m.totalRows, eta, m.progress.ViewAs(percent))
	return lipgloss.JoinVertical(lipgloss.Left, prog, lipgloss.NewStyle().Border(lipgloss.NormalBorder(), true, false, true, false).Width(m.width).Render(m.viewport.View()))
}

func (m runScreenModel) renderSummary() string {
	style := lipgloss.NewStyle().Bold(true).Padding(1)
	if len(m.tables) > 1 {
		success, partial, failed := 0, 0, 0
		for _, r := range m.tableResults {
			switch r.status {
			case "completed": success++
			case "interrupted": partial++
			case "failed": failed++
			}
		}
		summary := style.Render(fmt.Sprintf("Summary: %d success, %d partial, %d failed", success, partial, failed))
		var details []string
		for i, r := range m.tableResults {
			icon := "✗"
			if r.status == "completed" { icon = "✓" }
			if r.status == "interrupted" { icon = "⚠" }
			details = append(details, fmt.Sprintf("  %s %-20s %d rows", icon, r.table, r.rows))
			if i >= 4 && len(m.tableResults) > 6 {
				details = append(details, fmt.Sprintf("  ... and %d more", len(m.tableResults)-i-1))
				break
			}
		}
		return summary + "\n" + strings.Join(details, "\n")
	}
	switch m.status {
	case "completed": return style.Foreground(lipgloss.Color("10")).Render(fmt.Sprintf("✓ Done. %d rows in %v.", m.totalRows, time.Since(m.startTime).Round(time.Second)))
	case "failed": return style.Foreground(lipgloss.Color("9")).Render(fmt.Sprintf("✗ Failed: %v", m.err))
	case "interrupted": return style.Foreground(lipgloss.Color("11")).Render("⚠ Interrupted. Progress saved to checkpoint.")
	}
	return ""
}
