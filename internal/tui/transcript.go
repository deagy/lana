package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// TranscriptPane displays the conversation history.
type TranscriptPane struct {
	messages      []TranscriptMessage
	scrollOffset  int
	width         int
	height        int
	cachedLines   []string
	cachedWidth   int
	messageCount  int // Track message count to detect changes
	theme         Theme
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
		theme:    DefaultTheme(),
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
	tp.invalidateCache()
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
	tp.invalidateCache()
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
	tp.invalidateCache()
	tp.ScrollToEnd()
}

// GetLastMessage returns the content of the last message.
func (tp *TranscriptPane) GetLastMessage() string {
	if len(tp.messages) > 0 {
		return tp.messages[len(tp.messages)-1].Content
	}
	return ""
}

// ScrollUp scrolls the transcript up by one line.
func (tp *TranscriptPane) ScrollUp() {
	if tp.scrollOffset > 0 {
		tp.scrollOffset--
	}
}

// ScrollDown scrolls the transcript down by one line.
func (tp *TranscriptPane) ScrollDown() {
	tp.scrollOffset++
	// Will be bounded in Render() once we know cache size
}

// ScrollToStart scrolls to the beginning.
func (tp *TranscriptPane) ScrollToStart() {
	tp.scrollOffset = 0
}

// ScrollToEnd scrolls to the end (set to max so viewport shows last N lines).
func (tp *TranscriptPane) ScrollToEnd() {
	tp.scrollOffset = 999999 // Large number; will be bounded in Render()
}

// invalidateCache marks the cache as stale.
func (tp *TranscriptPane) invalidateCache() {
	tp.messageCount = len(tp.messages)
	tp.cachedLines = nil
	tp.cachedWidth = 0
}

// rebuildLinesIfNeeded rebuilds the line cache if width changed or messages changed.
func (tp *TranscriptPane) rebuildLinesIfNeeded(width int) {
	if tp.cachedLines != nil && tp.cachedWidth == width && tp.messageCount == len(tp.messages) {
		return // Cache is still valid
	}

	tp.cachedLines = nil
	for _, msg := range tp.messages {
		tp.cachedLines = append(tp.cachedLines, tp.renderMessage(msg, width-2)...)
	}
	tp.cachedWidth = width
	tp.messageCount = len(tp.messages)
}

// Render renders the transcript pane.
func (tp *TranscriptPane) Render(width, height int) string {
	tp.width = width
	tp.height = height

	if len(tp.messages) == 0 {
		return tp.renderEmpty()
	}

	// Build line cache if needed
	tp.rebuildLinesIfNeeded(width)

	// Bound scroll offset to valid range
	maxScroll := len(tp.cachedLines) - height
	if maxScroll < 0 {
		maxScroll = 0
	}
	if tp.scrollOffset > maxScroll {
		tp.scrollOffset = maxScroll
	}
	if tp.scrollOffset < 0 {
		tp.scrollOffset = 0
	}

	// Get visible lines
	endIdx := tp.scrollOffset + height
	if endIdx > len(tp.cachedLines) {
		endIdx = len(tp.cachedLines)
	}
	visibleLines := tp.cachedLines[tp.scrollOffset:endIdx]

	content := strings.Join(visibleLines, "\n")

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

	// Role header - use theme colors
	roleColor := tp.theme.AssistantColor
	if msg.IsUser {
		roleColor = tp.theme.UserColor
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
