package tui

import "github.com/charmbracelet/lipgloss"

// Theme defines centralized color scheme for TUI.
type Theme struct {
	// Message roles
	UserColor      lipgloss.Color
	AssistantColor lipgloss.Color
	SystemColor    lipgloss.Color

	// States
	ErrorColor     lipgloss.Color
	SuccessColor   lipgloss.Color
	WarningColor   lipgloss.Color
	StreamingColor lipgloss.Color

	// UI elements
	BorderColor    lipgloss.Color
	FocusColor     lipgloss.Color
	StatusBGColor  lipgloss.Color
	StatusFGColor  lipgloss.Color
	MutedColor     lipgloss.Color
}

// DefaultTheme returns the default theme.
func DefaultTheme() Theme {
	return Theme{
		UserColor:      lipgloss.Color("4"),       // Blue
		AssistantColor: lipgloss.Color("6"),       // Cyan
		SystemColor:    lipgloss.Color("8"),       // Gray
		ErrorColor:     lipgloss.Color("1"),       // Red
		SuccessColor:   lipgloss.Color("2"),       // Green
		WarningColor:   lipgloss.Color("3"),       // Yellow
		StreamingColor: lipgloss.Color("5"),       // Magenta
		BorderColor:    lipgloss.Color("8"),       // Gray
		FocusColor:     lipgloss.Color("69"),      // Light blue
		StatusBGColor:  lipgloss.Color("8"),       // Dark gray
		StatusFGColor:  lipgloss.Color("15"),      // White
		MutedColor:     lipgloss.Color("8"),       // Gray
	}
}
