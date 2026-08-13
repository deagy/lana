package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ComposerPane handles multi-line message input.
type ComposerPane struct {
	textarea   textarea.Model
	width      int
	height     int
	lastWidth  int
	lastHeight int
	focused    bool
	theme      Theme
}

// NewComposerPane creates a new composer pane.
func NewComposerPane() *ComposerPane {
	theme := DefaultTheme()
	ta := textarea.New()
	ta.Placeholder = "Type your message... (Ctrl+D to submit, Shift+Tab to navigate)"
	ta.SetWidth(80)
	ta.SetHeight(3)
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle().Foreground(theme.FocusColor)

	return &ComposerPane{
		textarea:   ta,
		height:     3,
		lastWidth:  80,
		lastHeight: 3,
		focused:    false,
		theme:      theme,
	}
}

// SetFocus sets focus state and only focuses textarea if transitioning to focused.
func (cp *ComposerPane) SetFocus(focused bool) {
	if focused && !cp.focused {
		cp.textarea.Focus()
	} else if !focused && cp.focused {
		cp.textarea.Blur()
	}
	cp.focused = focused
}

// Update handles input events for the composer.
func (cp *ComposerPane) Update(msg tea.Msg) {
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
	// Only update textarea size if it actually changed
	if width != cp.lastWidth || height != cp.lastHeight {
		cp.width = width
		cp.height = height
		cp.lastWidth = width
		cp.lastHeight = height
		cp.textarea.SetWidth(width - 2)
		cp.textarea.SetHeight(height)
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		Width(width).
		Height(height)

	content := cp.textarea.View()
	return style.Render(content)
}
