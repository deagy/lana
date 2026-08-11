// Package tui implements Lana's interactive Bubble Tea terminal interface.
package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/deagy/lana/internal/cli"
	"github.com/deagy/lana/internal/policy"
	"github.com/deagy/lana/internal/provider"
	"github.com/deagy/lana/internal/tools"
)

// Options are deliberately presentation-only. The runtime is injected and can
// be backed by a real agent kernel or by a deterministic test fake.
type Options struct {
	Runtime       *cli.Runtime
	Approvals     *cli.ApprovalBroker
	Context       context.Context
	Color         bool
	Input         io.Reader
	Output        io.Writer
	InitialPrompt string
}

type eventMsg provider.Event
type turnDoneMsg struct{ err error }
type submitMsg struct{}
type approvalMsg struct{ request *cli.ApprovalRequest }

type line struct {
	prefix string
	text   string
}

// Model is kept exportable so tests can drive Update without requiring a PTY.
type Model struct {
	runtime    *cli.Runtime
	composer   textarea.Model
	transcript []line
	history    []string
	historyAt  int
	width      int
	height     int
	active     bool
	color      bool
	cancel     context.CancelFunc
	events     chan provider.Event
	done       chan error
	approvals  *cli.ApprovalBroker
	pending    *cli.ApprovalRequest
	ctx        context.Context
}

var (
	accentStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	mutedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	errorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	toolStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
)

func New(opts Options) Model {
	composer := textarea.New()
	composer.Placeholder = "Ask Lana…  (Ctrl+Enter to send, /help for commands)"
	composer.Focus()
	composer.SetHeight(3)
	composer.ShowLineNumbers = false
	composer.Prompt = "❯ "
	composer.CharLimit = 0
	if !opts.Color {
		composer.Prompt = "> "
	}
	composer.SetValue(opts.InitialPrompt)
	if opts.Context == nil {
		opts.Context = context.Background()
	}
	m := Model{runtime: opts.Runtime, approvals: opts.Approvals, ctx: opts.Context, composer: composer, color: opts.Color, historyAt: -1}
	m.append("system", "Ready. Type /help for commands.")
	return m
}

func (m Model) Init() tea.Cmd {
	commands := []tea.Cmd{textarea.Blink}
	if m.approvals != nil {
		commands = append(commands, waitForApproval(m.ctx, m.approvals))
	}
	if strings.TrimSpace(m.composer.Value()) != "" {
		commands = append(commands, func() tea.Msg { return submitMsg{} })
	}
	return tea.Batch(commands...)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case approvalMsg:
		if typed.request == nil {
			m.append("error", "approval request was unavailable")
			return m, nil
		}
		m.pending = typed.request
		preview := typed.request.Preview()
		m.append("tool", fmt.Sprintf("Approval needed for %s.\nScope: %s\nArguments: %s\nChange: %s\nPress y to allow or n to deny.", preview.Tool, preview.Scope, preview.Arguments, preview.DiffSummary))
		return m, waitForApproval(m.ctx, m.approvals)
	case submitMsg:
		return m.submit()
	case tea.WindowSizeMsg:
		m.width, m.height = typed.Width, typed.Height
		m.composer.SetWidth(max(20, typed.Width-4))
		return m, nil
	case tea.KeyMsg:
		if typed.String() == "ctrl+c" {
			if m.pending != nil {
				m.pending.Deny()
				m.pending = nil
			}
			if m.cancel != nil {
				m.cancel()
			}
			if m.active {
				m.append("system", "Cancelling current turn…")
				return m, nil
			}
			return m, tea.Quit
		}
		if m.pending != nil {
			switch typed.String() {
			case "y", "Y":
				m.pending.Allow()
				m.append("system", "tool approved")
				m.pending = nil
				return m, nil
			case "n", "N":
				m.pending.Deny()
				m.append("system", "tool denied")
				m.pending = nil
				return m, nil
			}
		}
		switch typed.String() {
		case "ctrl+enter", "alt+enter", "ctrl+s":
			return m.submit()
		case "up":
			if m.composer.Value() == "" && len(m.history) > 0 {
				if m.historyAt < 0 {
					m.historyAt = len(m.history) - 1
				} else if m.historyAt > 0 {
					m.historyAt--
				}
				m.composer.SetValue(m.history[m.historyAt])
				return m, nil
			}
		case "down":
			if m.historyAt >= 0 {
				if m.historyAt < len(m.history)-1 {
					m.historyAt++
					m.composer.SetValue(m.history[m.historyAt])
				} else {
					m.historyAt = -1
					m.composer.Reset()
				}
				return m, nil
			}
		}
	case eventMsg:
		m.addEvent(provider.Event(typed))
		return m, waitForEvent(m.events, m.done)
	case turnDoneMsg:
		m.active = false
		m.cancel = nil
		if typed.err != nil && typed.err != context.Canceled {
			m.append("error", typed.err.Error())
		} else if typed.err == context.Canceled {
			m.append("system", "Turn cancelled.")
		}
		return m, nil
	}
	if !m.active {
		var command tea.Cmd
		m.composer, command = m.composer.Update(msg)
		return m, command
	}
	return m, nil
}

func waitForApproval(ctx context.Context, approvals *cli.ApprovalBroker) tea.Cmd {
	return func() tea.Msg {
		request, err := approvals.Wait(ctx)
		if err != nil {
			return turnDoneMsg{err: err}
		}
		return approvalMsg{request: request}
	}
}

func (m Model) submit() (tea.Model, tea.Cmd) {
	prompt := strings.TrimSpace(m.composer.Value())
	if prompt == "" || m.active {
		return m, nil
	}
	if strings.HasPrefix(prompt, "/") {
		m.composer.Reset()
		return m.command(prompt)
	}
	m.history = append(m.history, prompt)
	m.historyAt = -1
	m.composer.Reset()
	m.append("you", prompt)
	if m.runtime == nil {
		m.append("error", "No conversational runtime is configured.")
		return m, nil
	}
	m.active = true
	ctx, cancel := context.WithCancel(m.ctx)
	m.cancel = cancel
	m.events = make(chan provider.Event, 16)
	m.done = make(chan error, 1)
	go func() {
		_, err := m.runtime.Send(ctx, prompt, cli.EventSinkFunc(func(_ context.Context, event provider.Event) error {
			select {
			case m.events <- event:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}))
		m.done <- err
		close(m.events)
	}()
	return m, waitForEvent(m.events, m.done)
}

func waitForEvent(events <-chan provider.Event, done <-chan error) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-events
		if ok {
			return eventMsg(event)
		}
		return turnDoneMsg{err: <-done}
	}
}

func (m Model) command(input string) (tea.Model, tea.Cmd) {
	parts := strings.Fields(input)
	name := strings.TrimPrefix(parts[0], "/")
	argument := ""
	if len(parts) > 1 {
		argument = strings.Join(parts[1:], " ")
	}
	switch name {
	case "help":
		m.append("system", "/help  /status  /model [name]  /permissions [unrestricted|workspace-write|workspace-read-only]  /resume <id>  /new  /quit")
	case "status":
		if m.runtime == nil {
			m.append("system", "runtime: unavailable")
			return m, nil
		}
		m.append("system", fmt.Sprintf("session: %s  model: %s  permissions: %s", m.runtime.SessionID, valueOr(m.runtime.Model, "default"), valueOr(m.runtime.Permissions, "default")))
	case "model":
		if m.runtime == nil {
			m.append("error", "runtime unavailable")
		} else if argument == "" {
			m.append("system", "model: "+valueOr(m.runtime.Model, "default"))
		} else {
			m.runtime.Model = argument
			m.append("system", "model set to "+argument)
		}
	case "permissions":
		if m.runtime == nil {
			m.append("error", "runtime unavailable")
		} else if argument == "" {
			m.append("system", "permissions: "+valueOr(m.runtime.Permissions, "unconfigured"))
		} else if err := m.runtime.SetPermissionMode(policy.Mode(argument)); err != nil {
			m.append("error", err.Error())
		} else {
			m.append("system", "permissions set to "+m.runtime.Permissions)
		}
	case "resume":
		if argument == "" {
			m.append("error", "usage: /resume <session-id>")
		} else if m.runtime == nil {
			m.append("error", "runtime unavailable")
		} else if err := m.runtime.Resume(context.Background(), argument); err != nil {
			m.append("error", err.Error())
		} else {
			m.append("system", "resumed "+argument)
		}
	case "new":
		if m.runtime == nil {
			m.append("error", "runtime unavailable")
		} else if err := m.runtime.New(context.Background()); err != nil {
			m.append("error", err.Error())
		} else {
			m.transcript = nil
			m.append("system", "started "+m.runtime.SessionID)
		}
	case "quit", "exit":
		return m, tea.Quit
	default:
		m.append("error", "unknown command: /"+name+" (try /help)")
	}
	return m, nil
}

func (m *Model) addEvent(event provider.Event) {
	switch event.Type {
	case provider.EventTextDelta:
		text := cli.EventText(event)
		if text == "" {
			return
		}
		if len(m.transcript) > 0 && m.transcript[len(m.transcript)-1].prefix == "lana" {
			m.transcript[len(m.transcript)-1].text += text
		} else {
			m.append("lana", text)
		}
	case provider.EventToolCall:
		var call tools.Call
		if json.Unmarshal(event.Data, &call) == nil {
			m.append("tool", "approval/tool activity: "+call.Name)
		} else {
			m.append("tool", "tool requested")
		}
	case provider.EventError:
		m.append("error", valueOr(cli.EventText(event), "provider error"))
	}
}

func (m *Model) append(prefix, text string) {
	m.transcript = append(m.transcript, line{prefix: prefix, text: text})
}

func (m Model) View() string {
	width := max(20, m.width)
	lines := make([]string, 0, len(m.transcript)+4)
	for _, entry := range m.transcript {
		label := entry.prefix + ": "
		switch entry.prefix {
		case "lana":
			label = accentStyle.Render(label)
		case "tool":
			label = toolStyle.Render(label)
		case "error":
			label = errorStyle.Render(label)
		default:
			label = mutedStyle.Render(label)
		}
		if !m.color {
			label = entry.prefix + ": "
		}
		lines = append(lines, lipgloss.NewStyle().Width(width-2).Render(label+entry.text))
	}
	status := fmt.Sprintf("%s  model:%s  permissions:%s  %s", valueOr(session(m.runtime), "new"), valueOr(model(m.runtime), "default"), valueOr(perms(m.runtime), "default"), activity(m.active))
	if m.color {
		status = mutedStyle.Render(status)
	}
	return strings.Join(lines, "\n") + "\n\n" + m.composer.View() + "\n" + status
}

func Run(ctx context.Context, opts Options) error {
	if opts.Runtime == nil {
		return fmt.Errorf("conversational runtime is required")
	}
	if err := validateApprovalPath(opts.Runtime, opts.Approvals); err != nil {
		return err
	}
	if err := opts.Runtime.Ready(); err != nil {
		return err
	}
	opts.Context = ctx
	m := New(opts)
	programOptions := []tea.ProgramOption{tea.WithContext(ctx), tea.WithInput(opts.Input), tea.WithOutput(opts.Output)}
	_, err := tea.NewProgram(m, programOptions...).Run()
	return err
}

func validateApprovalPath(runtime *cli.Runtime, displayed *cli.ApprovalBroker) error {
	configured, usesBroker := runtime.ApprovalBroker()
	if usesBroker && configured != displayed {
		return fmt.Errorf("interactive approval broker does not match the runtime authorization path")
	}
	if displayed != nil && !runtime.UsesApprovalBroker(displayed) {
		return fmt.Errorf("interactive approval broker is not the runtime authorization path")
	}
	return nil
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
func session(r *cli.Runtime) string {
	if r == nil {
		return ""
	}
	return r.SessionID
}
func model(r *cli.Runtime) string {
	if r == nil {
		return ""
	}
	return r.Model
}
func perms(r *cli.Runtime) string {
	if r == nil {
		return ""
	}
	return valueOr(r.Permissions, "unconfigured")
}
func activity(active bool) string {
	if active {
		return "streaming (Ctrl-C cancels)"
	}
	return "ready"
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
