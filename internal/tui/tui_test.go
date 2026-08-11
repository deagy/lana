package tui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/deagy/lana/internal/agent"
	"github.com/deagy/lana/internal/cli"
	"github.com/deagy/lana/internal/policy"
	"github.com/deagy/lana/internal/provider"
	"github.com/deagy/lana/internal/tools"
)

type modeControl struct {
	mode policy.Mode
	err  error
}

func (c *modeControl) Mode() policy.Mode { return c.mode }
func (c *modeControl) SetMode(mode policy.Mode) error {
	if c.err != nil {
		return c.err
	}
	c.mode = mode
	return nil
}

type controlledExecutor struct{ control cli.AuthorizationController }

func (controlledExecutor) Run(context.Context, provider.Request, cli.EventSink) (agent.TurnResult, error) {
	return agent.TurnResult{}, nil
}
func (e controlledExecutor) AuthorizationController() cli.AuthorizationController { return e.control }

func TestSlashCommandsUpdateRuntime(t *testing.T) {
	control := &modeControl{mode: policy.ModeWorkspaceWrite}
	r := cli.NewRuntime(cli.Options{Executor: controlledExecutor{control: control}, NewSessionID: func() string { return "test" }})
	m := New(Options{Runtime: r, Color: false})
	updated, _ := m.command("/model test-model")
	m = updated.(Model)
	if r.Model != "test-model" {
		t.Fatalf("model = %q", r.Model)
	}
	updated, _ = m.command("/permissions workspace-read-only")
	m = updated.(Model)
	if r.Permissions != "workspace-read-only" || control.mode != policy.ModeWorkspaceReadOnly {
		t.Fatalf("permissions = %q", r.Permissions)
	}
	updated, _ = m.command("/permissions read-only")
	m = updated.(Model)
	if r.Permissions != "workspace-read-only" || !strings.Contains(m.transcript[len(m.transcript)-1].text, "invalid policy mode") {
		t.Fatalf("invalid mode changed permissions or lacked diagnostic: %#v", m.transcript[len(m.transcript)-1])
	}
	updated, command := m.command("/quit")
	if updated == nil || command == nil {
		t.Fatal("quit command missing")
	}
}

func TestToolAndTextEventsAppearInTranscript(t *testing.T) {
	m := New(Options{Color: false})
	m.addEvent(provider.Event{Type: provider.EventTextDelta, Data: []byte(`{"text":"hello"}`)})
	m.addEvent(provider.Event{Type: provider.EventToolCall, Data: []byte(`{"id":"1","name":"read_file","arguments":{}}`)})
	if len(m.transcript) != 3 || m.transcript[1].text != "hello" || m.transcript[2].prefix != "tool" {
		t.Fatalf("transcript=%#v", m.transcript)
	}
}

func TestCtrlCCancelsActiveTurn(t *testing.T) {
	m := New(Options{Color: false})
	cancelled := false
	m.active = true
	m.cancel = func() { cancelled = true }
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = updated.(Model)
	if !cancelled || !m.active {
		t.Fatalf("cancelled=%v active=%v", cancelled, m.active)
	}
}

func TestApprovalSurfaceRespondsToKey(t *testing.T) {
	broker := cli.NewApprovalBroker()
	m := New(Options{Approvals: broker, Color: false})
	request := &cli.ApprovalRequest{}
	updated, _ := m.Update(approvalMsg{request: request})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = updated.(Model)
	if m.pending != nil {
		t.Fatal("approval was not cleared")
	}
}

func TestApprovalSurfaceDisplaysSanitizedArgumentsScopeAndChangeSummary(t *testing.T) {
	m := New(Options{Color: false})
	request := &cli.ApprovalRequest{Call: tools.Call{
		ID:        "call-1",
		Name:      "write_file",
		Arguments: []byte(`{"path":"secrets.env","content":"TOKEN=topsecret","api_key":"also-secret"}`),
	}}
	updated, _ := m.Update(approvalMsg{request: request})
	m = updated.(Model)
	view := m.transcript[len(m.transcript)-1].text
	for _, want := range []string{"Scope: path=\"secrets.env\"", "Arguments:", "Change: replacement content supplied"} {
		if !strings.Contains(view, want) {
			t.Fatalf("approval view missing %q: %s", want, view)
		}
	}
	if strings.Contains(view, "topsecret") || strings.Contains(view, "also-secret") || !strings.Contains(view, "[REDACTED]") {
		t.Fatalf("approval view leaked a secret: %s", view)
	}
}

func TestCtrlCCancelsPendingApproval(t *testing.T) {
	broker := cli.NewApprovalBroker()
	approvalDone := make(chan error, 1)
	go func() {
		approvalDone <- broker.Authorize(context.Background(), tools.Call{ID: "1", Name: "exec", Arguments: []byte(`{}`)})
	}()
	request := <-broker.Requests()
	m := New(Options{Approvals: broker, Color: false})
	m.active = true
	cancelled := false
	m.cancel = func() { cancelled = true }
	m.pending = request
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = updated.(Model)
	if !cancelled || m.pending != nil {
		t.Fatalf("cancelled=%v pending=%#v", cancelled, m.pending)
	}
	if err := <-approvalDone; err == nil || !errors.Is(err, context.Canceled) && err.Error() != "tool call denied by user" {
		t.Fatalf("approval result = %v", err)
	}
}

func TestApprovalKeysDenyAndAcceptTheBrokerRequest(t *testing.T) {
	for name, key := range map[string]struct {
		key  rune
		deny bool
	}{
		"deny":   {key: 'n', deny: true},
		"accept": {key: 'y'},
	} {
		t.Run(name, func(t *testing.T) {
			broker := cli.NewApprovalBroker()
			decision := make(chan error, 1)
			go func() {
				decision <- broker.Authorize(context.Background(), tools.Call{ID: "call-1", Name: "read_file", Arguments: []byte(`{"path":"README.md"}`)})
			}()
			request := <-broker.Requests()
			m := New(Options{Approvals: broker, Color: false})
			updated, _ := m.Update(approvalMsg{request: request})
			m = updated.(Model)
			updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{key.key}})
			m = updated.(Model)
			if m.pending != nil {
				t.Fatal("approval remains pending")
			}
			select {
			case err := <-decision:
				if key.deny && (err == nil || !strings.Contains(err.Error(), "denied")) {
					t.Fatalf("deny error = %v", err)
				}
				if !key.deny && err != nil {
					t.Fatalf("allow error = %v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("approval decision did not reach broker")
			}
		})
	}
}

func TestNarrowNoColorViewUsesPlainState(t *testing.T) {
	runtime := cli.NewRuntime(cli.Options{SessionID: "narrow-session", Model: "small", Permissions: "ask"})
	m := New(Options{Runtime: runtime, Color: false})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 8, Height: 4})
	m = updated.(Model)
	m.append("lana", "A response that must remain readable in a narrow terminal.")
	view := m.View()
	if m.composer.Width() < 18 {
		t.Fatalf("composer width = %d, want readable minimum", m.composer.Width())
	}
	if strings.Contains(view, "\x1b[") || !strings.Contains(view, "lana: ") || !strings.Contains(view, "> ") || !strings.Contains(view, "narrow-session") {
		t.Fatalf("unexpected plain narrow view: %q", view)
	}
}

func TestRunRejectsMismatchedApprovalBroker(t *testing.T) {
	authorizer := cli.NewApprovalBroker()
	displayed := cli.NewApprovalBroker()
	runtime := cli.NewRuntime(cli.Options{Executor: cli.Kernel{Authorizer: authorizer}})
	err := Run(context.Background(), Options{Runtime: runtime, Approvals: displayed, Color: false})
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("mismatched broker error = %v", err)
	}
}
