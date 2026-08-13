package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ToolCall represents a tool call in the UI.
type ToolCall struct {
	ID       string
	Name     string
	Input    string
	Output   string
	Status   string // "pending", "approved", "running", "complete", "error"
	Error    string
	Approved bool
}

// RenderToolCall renders a tool call as a card.
func RenderToolCall(tc ToolCall, width int) string {
	var lines []string

	// Header
	statusIcon := "⏳"
	statusColor := lipgloss.Color("3") // Yellow
	if tc.Status == "complete" {
		statusIcon = "✓"
		statusColor = lipgloss.Color("2") // Green
	} else if tc.Status == "error" {
		statusIcon = "✗"
		statusColor = lipgloss.Color("1") // Red
	}

	header := fmt.Sprintf("%s %s", statusIcon, tc.Name)
	headerStyle := lipgloss.NewStyle().Foreground(statusColor).Bold(true)
	lines = append(lines, headerStyle.Render(header))

	// Input (if present)
	if tc.Input != "" {
		inputPreview := tc.Input
		if len(inputPreview) > 50 {
			inputPreview = inputPreview[:47] + "..."
		}
		inputStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		lines = append(lines, inputStyle.Render(fmt.Sprintf("  Input: %s", inputPreview)))
	}

	// Output (if present)
	if tc.Output != "" {
		outputPreview := tc.Output
		if len(outputPreview) > 100 {
			outputPreview = outputPreview[:97] + "..."
		}
		lines = append(lines, fmt.Sprintf("  Output: %s", outputPreview))
	}

	// Error (if present)
	if tc.Error != "" {
		errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
		lines = append(lines, errorStyle.Render(fmt.Sprintf("  Error: %s", tc.Error)))
	}

	// Status line
	statusText := tc.Status
	if tc.Status == "approved" {
		statusText = "approved by user"
	}
	statusLineStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	lines = append(lines, statusLineStyle.Render(fmt.Sprintf("  Status: %s", statusText)))

	content := strings.Join(lines, "\n")

	// Apply border
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		Width(width)

	return style.Render(content)
}

// ApprovalPrompt renders an approval prompt.
func RenderApprovalPrompt(toolName string, riskLevel string) string {
	var lines []string

	lines = append(lines, "")
	lines = append(lines, lipgloss.NewStyle().Bold(true).Render("⚠  Approval Required"))
	lines = append(lines, fmt.Sprintf("Tool: %s (Risk: %s)", toolName, riskLevel))
	lines = append(lines, "")
	lines = append(lines, "Press 'y' to approve or 'n' to deny")
	lines = append(lines, "")

	content := strings.Join(lines, "\n")
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2).
		BorderForeground(lipgloss.Color("1"))

	return style.Render(content)
}
