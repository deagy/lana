package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ComposerPane handles multi-line message input.
type ComposerPane struct {
	textarea textarea.Model
	width    int
	height   int
}

// NewComposerPane creates a new composer pane.
func NewComposerPane() *ComposerPane {
	ta := textarea.New()
	ta.Placeholder = "Type your message... (Ctrl+D to submit, Shift+Tab to navigate)"
	ta.SetWidth(80)
	ta.SetHeight(3)
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle().Foreground(lipgloss.Color("69"))

	return &ComposerPane{
		textarea: ta,
		height:   3,
	}
}

// Update handles input events for the composer.
func (cp *ComposerPane) Update(msg tea.Msg) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+d" {
			// Clear on ctrl+d instead of submit
			// Submit is handled in main model
		}
	}

	cp.textarea.Focus()
	_, _ = cp.textarea.Update(msg)
}

// Value returns the current input value.
func (cp *ComposerPane) Value() string {
	return strings.TrimSpace(cp.textarea.Value())
}

// Reset clears the composer.
func (cp *ComposerPane) Reset() {
	cp.textarea.Reset()
}

// Render renders the composer pane.
func (cp *ComposerPane) Render(width, height int) string {
	cp.width = width
	cp.textarea.SetWidth(width - 2)
	cp.textarea.SetHeight(height)

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		Width(width).
		Height(height)

	content := cp.textarea.View()
	return style.Render(content)
}
