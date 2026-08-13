package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// SimpleDiff represents a unified diff.
type SimpleDiff struct {
	Filename string
	Added    int
	Removed  int
	Lines    []DiffLine
}

// DiffLine represents a single line in a diff.
type DiffLine struct {
	Type    string // "add", "remove", "context"
	Content string
}

// RenderDiff renders a diff as colored text.
func RenderDiff(diff SimpleDiff, width int) string {
	var lines []string

	// Header
	header := diff.Filename
	lines = append(lines, lipgloss.NewStyle().Bold(true).Render(header))

	// Stats
	stats := ""
	if diff.Added > 0 {
		stats += lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render(
			"+"+strings.Repeat("█", min(diff.Added, 20)),
		) + " "
	}
	if diff.Removed > 0 {
		stats += lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render(
			"-" + strings.Repeat("█", min(diff.Removed, 20)),
		)
	}
	if stats != "" {
		lines = append(lines, stats)
	}

	// Diff lines
	for _, line := range diff.Lines {
		switch line.Type {
		case "add":
			styled := lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render("+ " + line.Content)
			lines = append(lines, styled)

		case "remove":
			styled := lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render("- " + line.Content)
			lines = append(lines, styled)

		case "context":
			styled := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("  " + line.Content)
			lines = append(lines, styled)
		}
	}

	content := strings.Join(lines, "\n")

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		Width(width)

	return style.Render(content)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
