package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// StatusBar displays status information at the bottom of the TUI.
type StatusBar struct {
	provider  string
	model     string
	sessionID string
	streaming bool
	error     string
	width     int
	theme     Theme
}

// NewStatusBar creates a new status bar.
func NewStatusBar(provider, model, sessionID string) *StatusBar {
	return &StatusBar{
		provider:  provider,
		model:     model,
		sessionID: sessionID[:8], // Truncate to 8 chars
		streaming: false,
		theme:     DefaultTheme(),
	}
}

// SetStreaming sets the streaming state.
func (sb *StatusBar) SetStreaming(streaming bool) {
	sb.streaming = streaming
}

// SetError sets an error message.
func (sb *StatusBar) SetError(errMsg string) {
	sb.error = errMsg
}

// ClearError clears any error message.
func (sb *StatusBar) ClearError() {
	sb.error = ""
}

// Render renders the status bar.
func (sb *StatusBar) Render(width int) string {
	sb.width = width

	// Build status line
	var parts []string

	// Provider and model
	parts = append(parts, fmt.Sprintf("%s/%s", sb.provider, sb.model))

	// Streaming indicator
	if sb.streaming {
		parts = append(parts, "● streaming")
	}

	// Session ID
	parts = append(parts, fmt.Sprintf("#%s", sb.sessionID))

	// Error
	if sb.error != "" {
		errorStyle := lipgloss.NewStyle().Foreground(sb.theme.ErrorColor)
		parts = append(parts, errorStyle.Render(fmt.Sprintf("⚠ %s", sb.error)))
	}

	// Help text
	parts = append(parts, "? help")

	statusText := strings.Join(parts, " │ ")

	// Truncate if too long
	if len(statusText) > width-2 {
		statusText = statusText[:width-5] + "..."
	}

	// Pad to width
	statusText = fmt.Sprintf(" %-*s ", width-2, statusText)

	// Style
	style := lipgloss.NewStyle().
		Background(sb.theme.StatusBGColor).
		Foreground(sb.theme.StatusFGColor).
		Width(width)

	return style.Render(statusText)
}
