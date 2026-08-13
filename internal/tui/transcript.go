package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// TranscriptPane displays the conversation history.
type TranscriptPane struct {
	messages     []TranscriptMessage
	scrollOffset int
	width        int
	height       int
}

type TranscriptMessage struct {
	Role    string
	Content string
	IsUser  bool
}

// NewTranscriptPane creates a new transcript pane.
func NewTranscriptPane() *TranscriptPane {
	return &TranscriptPane{
		messages: []TranscriptMessage{},
	}
}

// StartMessage starts a new message in the transcript.
func (tp *TranscriptPane) StartMessage(role string) {
	msg := TranscriptMessage{
		Role:    role,
		Content: "",
		IsUser:  role == "user",
	}
	tp.messages = append(tp.messages, msg)
	tp.ScrollToEnd()
}

// AppendContent appends content to the last message.
func (tp *TranscriptPane) AppendContent(content string) {
	if len(tp.messages) > 0 {
		tp.messages[len(tp.messages)-1].Content += content
	}
}

// AppendToolCall appends a tool call to the transcript.
func (tp *TranscriptPane) AppendToolCall(name string, input string) {
	msg := TranscriptMessage{
		Role:    "system",
		Content: fmt.Sprintf("🔧 %s\n%s", name, input),
		IsUser:  false,
	}
	tp.messages = append(tp.messages, msg)
	tp.ScrollToEnd()
}

// AppendToolResult appends a tool result to the transcript.
func (tp *TranscriptPane) AppendToolResult(name string, output string, err string) {
	var content string
	if err != "" {
		content = fmt.Sprintf("🔧 %s error:\n%s", name, err)
	} else {
		if len(output) > 200 {
			output = output[:197] + "..."
		}
		content = fmt.Sprintf("🔧 %s result:\n%s", name, output)
	}

	msg := TranscriptMessage{
		Role:    "system",
		Content: content,
		IsUser:  false,
	}
	tp.messages = append(tp.messages, msg)
	tp.ScrollToEnd()
}

// GetLastMessage returns the content of the last message.
func (tp *TranscriptPane) GetLastMessage() string {
	if len(tp.messages) > 0 {
		return tp.messages[len(tp.messages)-1].Content
	}
	return ""
}

// ScrollUp scrolls the transcript up.
func (tp *TranscriptPane) ScrollUp() {
	if tp.scrollOffset > 0 {
		tp.scrollOffset--
	}
}

// ScrollDown scrolls the transcript down.
func (tp *TranscriptPane) ScrollDown() {
	maxScroll := len(tp.messages) - 1
	if tp.scrollOffset < maxScroll {
		tp.scrollOffset++
	}
}

// ScrollToStart scrolls to the beginning.
func (tp *TranscriptPane) ScrollToStart() {
	tp.scrollOffset = 0
}

// ScrollToEnd scrolls to the end.
func (tp *TranscriptPane) ScrollToEnd() {
	maxScroll := len(tp.messages) - 1
	tp.scrollOffset = maxScroll
}

// Render renders the transcript pane.
func (tp *TranscriptPane) Render(width, height int) string {
	tp.width = width
	tp.height = height

	if len(tp.messages) == 0 {
		return tp.renderEmpty()
	}

	var lines []string
	for _, msg := range tp.messages {
		lines = append(lines, tp.renderMessage(msg, width-2)...)
	}

	// Apply scroll offset
	if len(lines) > height {
		startIdx := len(lines) - height
		if startIdx < 0 {
			startIdx = 0
		}
		lines = lines[startIdx:]
	}

	content := strings.Join(lines, "\n")

	// Apply border
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		Width(width).
		Height(height)

	return style.Render(content)
}

func (tp *TranscriptPane) renderMessage(msg TranscriptMessage, width int) []string {
	var lines []string

	// Role header
	roleColor := lipgloss.Color("6")
	if msg.IsUser {
		roleColor = lipgloss.Color("4")
	}

	roleStyle := lipgloss.NewStyle().Foreground(roleColor).Bold(true)
	roleHeader := roleStyle.Render(strings.ToUpper(msg.Role))
	lines = append(lines, roleHeader)

	// Content with word wrap
	wrapped := wrapText(msg.Content, width)
	lines = append(lines, wrapped...)

	// Spacing
	lines = append(lines, "")

	return lines
}

func (tp *TranscriptPane) renderEmpty() string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2).
		Width(tp.width).
		Height(tp.height).
		Foreground(lipgloss.Color("8"))

	return style.Render("Start a conversation by typing in the composer below.\nPress ? for help.")
}

// Helper function to wrap text
func wrapText(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}

	var lines []string
	for _, line := range strings.Split(text, "\n") {
		for len(line) > width {
			lines = append(lines, line[:width])
			line = line[width:]
		}
		if len(line) > 0 {
			lines = append(lines, line)
		}
	}

	return lines
}
