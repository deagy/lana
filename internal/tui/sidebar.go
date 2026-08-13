package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/deagy/lana/internal/session"
)

// SidebarPane displays session info and navigation.
type SidebarPane struct {
	currentSession *session.Session
	recentSessions []session.SessionMetadata
	selectedIdx    int
	width          int
	height         int
}

// NewSidebarPane creates a new sidebar pane.
func NewSidebarPane(sess *session.Session) *SidebarPane {
	return &SidebarPane{
		currentSession: sess,
		recentSessions: []session.SessionMetadata{},
		selectedIdx:    0,
	}
}

// Update handles input events for the sidebar.
func (sp *SidebarPane) Update(msg tea.Msg) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return
	}

	switch keyMsg.String() {
	case "j", "down":
		if sp.selectedIdx < len(sp.recentSessions)-1 {
			sp.selectedIdx++
		}
	case "k", "up":
		if sp.selectedIdx > 0 {
			sp.selectedIdx--
		}
	}
}

// SetSessions sets the list of recent sessions to display.
func (sp *SidebarPane) SetSessions(sessions []session.SessionMetadata) {
	sp.recentSessions = sessions
}

// Render renders the sidebar pane.
func (sp *SidebarPane) Render(width, height int) string {
	sp.width = width
	sp.height = height

	var content []string

	// Current session info
	content = append(content, lipgloss.NewStyle().Bold(true).Render("Current Session"))
	content = append(content, fmt.Sprintf("  %s", sp.currentSession.Title))
	content = append(content, fmt.Sprintf("  Model: %s", sp.currentSession.Model))
	content = append(content, "")

	// Recent sessions
	content = append(content, lipgloss.NewStyle().Bold(true).Render("Recent Sessions"))
	for i, sess := range sp.recentSessions {
		marker := " "
		if i == sp.selectedIdx {
			marker = ">"
		}

		line := fmt.Sprintf("%s %s", marker, sess.Title)
		if len(line) > width-2 {
			line = line[:width-4] + ".."
		}
		content = append(content, line)
	}

	text := strings.Join(content, "\n")

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		Width(width).
		Height(height).
		Foreground(lipgloss.Color("8"))

	return style.Render(text)
}
