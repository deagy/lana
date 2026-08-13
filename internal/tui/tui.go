package tui

import (
	"context"
	"io"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/deagy/lana/internal/approval"
	"github.com/deagy/lana/internal/provider"
	"github.com/deagy/lana/internal/session"
)

// Model represents the main TUI state.
type Model struct {
	// Core state
	sessionID      string
	sessionStore   session.Store
	providerClient provider.Client
	approvalPolicy approval.Policy

	// UI components
	transcript *TranscriptPane
	composer   *ComposerPane
	sidebar    *SidebarPane
	statusBar  *StatusBar

	// Layout
	width  int
	height int

	// Interaction
	focusedPane string // "transcript", "composer", "sidebar"
	mode        string // "normal", "help"

	// Streaming state
	streaming    bool
	pendingInput string
}

// New creates a new TUI model.
func New(sessionID string, store session.Store, client provider.Client, policy approval.Policy) (*Model, error) {
	// Load session
	ctx := context.Background()
	sess, err := store.Get(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	return &Model{
		sessionID:      sessionID,
		sessionStore:   store,
		providerClient: client,
		approvalPolicy: policy,
		transcript:     NewTranscriptPane(),
		composer:       NewComposerPane(),
		sidebar:        NewSidebarPane(sess),
		statusBar:      NewStatusBar(client.Name(), client.Model(), sessionID),
		focusedPane:    "composer",
		mode:           "normal",
		streaming:      false,
	}, nil
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.layout()
		return m, nil

	case ChatResponseEvent:
		return m.handleChatResponse(msg)

	case ChatErrorEvent:
		m.statusBar.SetError(msg.Error.Error())
		m.streaming = false
		return m, nil

	case ChatDoneEvent:
		m.streaming = false
		m.statusBar.ClearError()
		return m, nil
	}

	return m, nil
}

// View implements tea.Model.
func (m *Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	if m.mode == "help" {
		return m.viewHelp()
	}

	// Calculate pane heights
	sidebarWidth := 20
	composerHeight := 3
	statusHeight := 1
	availableHeight := m.height - composerHeight - statusHeight

	// Side-by-side layout if width allows
	if m.width > 100 {
		transcriptWidth := m.width - sidebarWidth - 2
		transcriptHeight := availableHeight

		left := m.transcript.Render(transcriptWidth, transcriptHeight)
		right := m.sidebar.Render(sidebarWidth, transcriptHeight)
		composer := m.composer.Render(m.width, composerHeight)
		status := m.statusBar.Render(m.width)

		return lipgloss.JoinVertical(lipgloss.Top,
			lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right),
			composer,
			status,
		)
	}

	// Stacked layout for narrow terminals
	transcript := m.transcript.Render(m.width, availableHeight-1)
	composer := m.composer.Render(m.width, composerHeight)
	status := m.statusBar.Render(m.width)

	return lipgloss.JoinVertical(lipgloss.Top,
		transcript,
		composer,
		status,
	)
}

func (m *Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global keys
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit

	case "ctrl+h", "?":
		if m.mode == "normal" {
			m.mode = "help"
		} else {
			m.mode = "normal"
		}
		return m, nil

	case "tab":
		m.focusedPane = m.nextPane()
		return m, nil

	case "shift+tab":
		m.focusedPane = m.prevPane()
		return m, nil
	}

	// Mode-specific handling
	if m.mode == "help" {
		if msg.String() == "q" || msg.String() == "esc" {
			m.mode = "normal"
		}
		return m, nil
	}

	// Pane-specific handling
	switch m.focusedPane {
	case "composer":
		// Composer handles most input
		if msg.String() == "enter" && !msg.Alt {
			// Send message
			input := m.composer.Value()
			if input != "" {
				m.composer.Reset()
				return m, m.sendMessage(input)
			}
		} else {
			m.composer.Update(msg)
		}

	case "transcript":
		// Navigation in transcript
		switch msg.String() {
		case "j", "down":
			m.transcript.ScrollDown()
		case "k", "up":
			m.transcript.ScrollUp()
		case "g":
			m.transcript.ScrollToStart()
		case "G":
			m.transcript.ScrollToEnd()
		}

	case "sidebar":
		// Sidebar selection
		m.sidebar.Update(msg)
	}

	return m, nil
}

func (m *Model) handleChatResponse(msg ChatResponseEvent) (tea.Model, tea.Cmd) {
	switch e := msg.Event.(type) {
	case *provider.MessageStartEvent:
		m.transcript.StartMessage(e.Role)

	case *provider.MessageDeltaEvent:
		m.transcript.AppendContent(e.Content)

	case *provider.MessageEndEvent:
		// Message complete
		ctx := context.Background()
		_ = m.sessionStore.AppendMessage(ctx, m.sessionID, &session.Message{
			Role:    "assistant",
			Content: m.transcript.GetLastMessage(),
		})

	case *provider.ToolCallEvent:
		m.transcript.AppendToolCall(e.Name, string(e.Input))

	case *provider.ErrorEvent:
		m.statusBar.SetError(e.Message)

	default:
		// Unknown event type
	}

	return m, nil
}

func (m *Model) sendMessage(input string) tea.Cmd {
	return func() tea.Msg {
		// Save user message
		ctx := context.Background()
		_ = m.sessionStore.AppendMessage(ctx, m.sessionID, &session.Message{
			Role:    "user",
			Content: input,
		})

		// Get updated transcript
		sess, _ := m.sessionStore.Get(ctx, m.sessionID)
		msgs := make([]provider.Message, len(sess.Transcript))
		for i, msg := range sess.Transcript {
			msgs[i] = provider.Message{
				Role:    msg.Role,
				Content: msg.Content,
			}
		}

		// Send to provider
		m.streaming = true
		m.statusBar.SetStreaming(true)

		reader, err := m.providerClient.Chat(ctx, &provider.Request{
			Messages: msgs,
			Model:    m.providerClient.Model(),
		})

		if err != nil {
			return ChatErrorEvent{Error: err}
		}

		// Stream events (simplified for Phase 3)
		// TODO: Implement streaming via channels or commands in Phase 4+
		go func() {
			for {
				_, err := reader.NextEvent(ctx)
				if err == io.EOF {
					// Use a command to send done event
					return
				}
				if err != nil {
					// Error event already sent
					return
				}
				// Send event back to update loop
				// This would need a channel-based architecture
			}
		}()

		return nil
	}
}

func (m *Model) layout() {
	// Layout will be handled in View()
}

func (m *Model) nextPane() string {
	panes := []string{"composer", "transcript", "sidebar"}
	for i, p := range panes {
		if p == m.focusedPane {
			return panes[(i+1)%len(panes)]
		}
	}
	return "composer"
}

func (m *Model) prevPane() string {
	panes := []string{"composer", "transcript", "sidebar"}
	for i, p := range panes {
		if p == m.focusedPane {
			if i == 0 {
				return panes[len(panes)-1]
			}
			return panes[i-1]
		}
	}
	return "composer"
}

func (m *Model) viewHelp() string {
	help := `
╭─ Lana Help ────────────────────────────╮
│                                        │
│ Global:                                │
│   Ctrl+C     - Quit                    │
│   Ctrl+H, ?  - Toggle help             │
│   Tab        - Next pane               │
│   Shift+Tab  - Previous pane           │
│                                        │
│ Composer:                              │
│   Enter      - Send message            │
│   Ctrl+U     - Clear line              │
│                                        │
│ Transcript:                            │
│   j, Down    - Scroll down             │
│   k, Up      - Scroll up               │
│   g          - Go to start             │
│   G          - Go to end               │
│                                        │
│ Press Q or Esc to close help           │
│                                        │
╰────────────────────────────────────────╯
`
	return help
}

// Custom tea events for streaming

type ChatResponseEvent struct {
	Event provider.Event
}

type ChatErrorEvent struct {
	Error error
}

type ChatDoneEvent struct{}
