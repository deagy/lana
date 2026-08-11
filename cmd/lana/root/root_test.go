package root

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/deagy/lana/internal/agent"
	"github.com/deagy/lana/internal/app"
	"github.com/deagy/lana/internal/cli"
	"github.com/deagy/lana/internal/provider"
	"github.com/deagy/lana/internal/session"
	"github.com/deagy/lana/internal/testkit"
	"github.com/deagy/lana/internal/tools"
)

func TestNewRootCommand(t *testing.T) {
	cmd := NewRootCommand()
	if cmd == nil {
		t.Fatal("NewRootCommand returned nil")
	}
	if cmd.Use != "lana" {
		t.Errorf("expected Use to be 'lana', got %q", cmd.Use)
	}
}

func TestRootCommandHasSubcommands(t *testing.T) {
	cmd := NewRootCommand()
	expectedSubcommands := []string{"agents", "exec", "file", "knowledge", "sdlc", "system"}

	cmdMap := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		cmdMap[sub.Name()] = true
	}

	for _, expected := range expectedSubcommands {
		if !cmdMap[expected] {
			t.Errorf("expected subcommand %q not found", expected)
		}
	}
	for _, unsupported := range []string{"dispatch", "shell", "git", "goal", "mcp", "plan", "plugin", "skill"} {
		if cmdMap[unsupported] {
			t.Errorf("unsupported compatibility command %q is publicly reachable", unsupported)
		}
	}
}

func TestBashCompletionGenerationHasNoPersistentShortFlagCollision(t *testing.T) {
	cmd := NewRootCommand()
	configFlag := cmd.PersistentFlags().Lookup("config")
	if configFlag == nil {
		t.Fatal("missing persistent --config flag")
	}
	if configFlag.Shorthand != "" {
		t.Fatalf("persistent --config must not reserve -%s; subcommands use -c", configFlag.Shorthand)
	}

	var output bytes.Buffer
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("bash completion generation panicked: %v", recovered)
		}
	}()
	if err := cmd.GenBashCompletion(&output); err != nil {
		t.Fatalf("generate bash completion: %v", err)
	}
	if output.Len() == 0 {
		t.Fatal("generated bash completion is empty")
	}
}

func TestRootHelpDoesNotAdvertiseUnsupportedCompatibilityCommands(t *testing.T) {
	cmd := NewRootCommand()
	var output bytes.Buffer
	cmd.SetOut(&output)
	if err := cmd.Help(); err != nil {
		t.Fatalf("root help: %v", err)
	}
	for _, unsupported := range []string{"dispatch", "shell", "git", "goal", "mcp", "plan", "plugin", "skill"} {
		if strings.Contains(output.String(), unsupported) {
			t.Errorf("root help advertises unsupported compatibility command %q:\n%s", unsupported, output.String())
		}
	}
}

func TestSDLCExposesOnlyReadOnlyInspectionCommands(t *testing.T) {
	cmd := NewRootCommand()
	sdlc, _, err := cmd.Find([]string{"sdlc"})
	if err != nil || sdlc == nil {
		t.Fatalf("sdlc command: %v", err)
	}
	children := make(map[string]bool)
	for _, child := range sdlc.Commands() {
		children[child.Name()] = true
	}
	for _, expected := range []string{"status", "list-runs", "show-run", "read-plan", "read-record"} {
		if !children[expected] {
			t.Errorf("missing read-only SDLC command %q", expected)
		}
	}
	for _, mutation := range []string{"init", "write-plan", "write-record", "review-gate", "approve-gate", "reject-gate", "request-changes", "invalidate-gates"} {
		if children[mutation] {
			t.Errorf("lifecycle mutation command %q is publicly reachable", mutation)
		}
	}
}

func TestExecUsesPublishedSessionFlagAndKeepsResumeHidden(t *testing.T) {
	store := testSessionStore(t)
	if _, err := store.Create(context.Background(), "existing", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Append(context.Background(), "existing", "message.user", provider.Message{Role: "user", Content: "earlier prompt"}); err != nil {
		t.Fatal(err)
	}
	script := &testkit.Script{Result: agent.TurnResult{Completed: true}}
	cmd := NewRootCommandWithOptions(Options{Executor: scriptedExecutor(script), Sessions: store})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"exec", "--session", "existing", "next prompt"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("exec --session: %v", err)
	}
	requests := script.Requests()
	if len(requests) != 1 || len(requests[0].Messages) != 2 || requests[0].Messages[0].Content != "earlier prompt" {
		t.Fatalf("resumed request = %#v", requests)
	}
	execCmd, _, err := cmd.Find([]string{"exec"})
	if err != nil {
		t.Fatalf("exec command: %v", err)
	}
	if flag := execCmd.Flags().Lookup("session"); flag == nil || flag.Hidden {
		t.Fatal("published --session flag is not visible")
	}
	if flag := execCmd.Flags().Lookup("resume"); flag == nil || !flag.Hidden {
		t.Fatal("legacy --resume flag must remain available only as a hidden compatibility alias")
	}
}

func TestDefaultTerminalRequiresInputAndOutputTTY(t *testing.T) {
	reader, _, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	if defaultTerminal(reader, os.Stdout) {
		t.Fatal("pipe input must not select TUI")
	}
}

func TestRuntimeForUsesInjectedKernelDependencies(t *testing.T) {
	store, err := session.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := runtimeFor(&cobra.Command{}, Options{
		Provider:   &provider.StaticClient{},
		Authorizer: tools.AllowAll{},
		ToolExecutor: tools.ExecutorFunc(func(_ context.Context, call tools.Call) (tools.Result, error) {
			return tools.JSONResult(call, map[string]bool{"ok": true})
		}),
		Sessions: store,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.Ready(); err != nil {
		t.Fatalf("injected runtime is not ready: %v", err)
	}
}

func TestRootWiresResolvedApplicationAndSeparatesFlags(t *testing.T) {
	rootDir := t.TempDir()
	workspace := filepath.Join(rootDir, "workspace")
	if err := os.MkdirAll(workspace, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(rootDir, "settings.yaml")
	if err := os.WriteFile(configPath, []byte("logging:\n  level: debug\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := NewRootCommand()
	var received *app.Application
	cmd.AddCommand(&cobra.Command{
		Use: "probe",
		RunE: func(command *cobra.Command, _ []string) error {
			var ok bool
			received, ok = app.FromContext(command.Context())
			if !ok {
				t.Fatal("application was not attached to command context")
			}
			return nil
		},
	})
	cmd.SetArgs([]string{"--workspace", workspace, "--config", configPath, "probe"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("root command failed: %v", err)
	}
	if received == nil {
		t.Fatal("probe did not receive application")
	}
	if got := received.Config().Workspace(); got != workspace {
		t.Errorf("workspace: got %q want %q", got, workspace)
	}
	if got := received.Config().Logging().Level; got != "debug" {
		t.Errorf("config path was not applied: got log level %q", got)
	}
}

func TestRootPlainModeUsesNoTTYFallbackAndKeepsProtocolsSeparate(t *testing.T) {
	events, err := testkit.LoadEvents("terminal", "provider-events.json")
	if err != nil {
		t.Fatal(err)
	}
	script := &testkit.Script{Events: events, Result: agent.TurnResult{Completed: true}}
	var stdout, stderr bytes.Buffer
	cmd := NewRootCommandWithOptions(Options{
		Executor: scriptedExecutor(script),
		Sessions: testSessionStore(t),
		IsTerminal: func(_ io.Reader, _ io.Writer) bool {
			return false
		},
	})
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"describe", "the", "fixture"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "fixture answer\n" {
		t.Fatalf("plain stdout = %q", stdout.String())
	}
	if stderr.String() != "tool: read_file\n" {
		t.Fatalf("plain stderr = %q", stderr.String())
	}
	requests := script.Requests()
	if len(requests) != 1 || len(requests[0].Messages) != 1 || requests[0].Messages[0].Content != "describe the fixture" {
		t.Fatalf("plain request = %#v", requests)
	}
}

func TestExecJSONLEmitsFixtureEventsInProviderOrder(t *testing.T) {
	events, err := testkit.LoadEvents("terminal", "provider-events.json")
	if err != nil {
		t.Fatal(err)
	}
	script := &testkit.Script{Events: events, Result: agent.TurnResult{Completed: true}}
	var stdout, stderr bytes.Buffer
	cmd := NewRootCommandWithOptions(Options{Executor: scriptedExecutor(script), Sessions: testSessionStore(t)})
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"exec", "--jsonl", "render", "events"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if len(lines) != len(events) {
		t.Fatalf("JSONL lines = %d, want %d: %q", len(lines), len(events), stdout.String())
	}
	for i, line := range lines {
		var got provider.Event
		if err := json.Unmarshal([]byte(line), &got); err != nil {
			t.Fatalf("line %d is not JSONL: %v", i, err)
		}
		if got.ID != events[i].ID || got.Type != events[i].Type || !got.At.Equal(events[i].At) {
			t.Fatalf("event %d = %#v, want %#v", i, got, events[i])
		}
	}
	if stderr.Len() != 0 || strings.Contains(stdout.String(), "\x1b[") {
		t.Fatalf("JSONL protocol polluted: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestRootNoTTYReadsPromptFromInput(t *testing.T) {
	script := &testkit.Script{Events: []provider.Event{{SchemaVersion: provider.EventSchemaVersion, Type: provider.EventTextDelta, At: fixtureTime(), Data: []byte(`{"text":"stdin answer"}`)}}, Result: agent.TurnResult{Completed: true}}
	var stdout bytes.Buffer
	terminalChecks := 0
	cmd := NewRootCommandWithOptions(Options{
		Executor: scriptedExecutor(script), Sessions: testSessionStore(t),
		IsTerminal: func(_ io.Reader, _ io.Writer) bool { terminalChecks++; return false },
	})
	cmd.SetIn(strings.NewReader("prompt from stdin\n"))
	cmd.SetOut(&stdout)
	cmd.SetArgs(nil)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	requests := script.Requests()
	if terminalChecks != 1 || len(requests) != 1 || requests[0].Messages[0].Content != "prompt from stdin" || stdout.String() != "stdin answer\n" {
		t.Fatalf("terminal checks=%d requests=%#v stdout=%q", terminalChecks, requests, stdout.String())
	}
}

func scriptedExecutor(script *testkit.Script) cli.TurnExecutor {
	return cli.TurnExecutorFunc(func(ctx context.Context, request provider.Request, sink cli.EventSink) (agent.TurnResult, error) {
		return script.Run(ctx, request, sink.Emit)
	})
}

func testSessionStore(t *testing.T) *session.Store {
	t.Helper()
	store, err := session.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func fixtureTime() time.Time { return time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC) }
