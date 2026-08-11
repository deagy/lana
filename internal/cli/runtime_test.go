package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/deagy/lana/internal/agent"
	"github.com/deagy/lana/internal/policy"
	"github.com/deagy/lana/internal/provider"
	"github.com/deagy/lana/internal/session"
	"github.com/deagy/lana/internal/testkit"
	"github.com/deagy/lana/internal/tools"
)

func event(t *testing.T, eventType provider.EventType, data any) provider.Event {
	t.Helper()
	e, err := provider.NewEvent(eventType, data)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestRuntimeStreamsAndRetainsAssistantMessage(t *testing.T) {
	runner := TurnExecutorFunc(func(ctx context.Context, request provider.Request, sink EventSink) (agent.TurnResult, error) {
		if len(request.Messages) != 1 || request.Messages[0].Content != "hello" {
			t.Fatalf("request=%#v", request)
		}
		if err := sink.Emit(ctx, event(t, provider.EventTextDelta, map[string]string{"text": "hi"})); err != nil {
			return agent.TurnResult{}, err
		}
		return agent.TurnResult{}, nil
	})
	r := NewRuntime(Options{Executor: runner, NewSessionID: func() string { return "test" }})
	var output bytes.Buffer
	if _, err := r.Send(context.Background(), "hello", PlainRenderer{Stdout: &output}); err != nil {
		t.Fatal(err)
	}
	if output.String() != "hi" || len(r.messages) != 2 || r.messages[1].Content != "hi" {
		t.Fatalf("output=%q messages=%#v", output.String(), r.messages)
	}
}

func TestRuntimePersistsOnlyRedactedProviderEventPayloads(t *testing.T) {
	const secret = "do-not-show"
	store, err := session.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	runtime := NewRuntime(Options{
		Sessions:     store,
		NewSessionID: func() string { return "redaction" },
		Executor: TurnExecutorFunc(func(ctx context.Context, _ provider.Request, sink EventSink) (agent.TurnResult, error) {
			return agent.TurnResult{}, sink.Emit(ctx, provider.Event{
				SchemaVersion: provider.EventSchemaVersion,
				Type:          provider.EventToolCall,
				At:            time.Now(),
				Data: json.RawMessage(`{
					"command":"curl -H 'Authorization: Bearer do-not-show' https://user:do-not-show@example.test/?api-key=do-not-show",
					"headers":{"X-API-Key":"do-not-show","Cookie":"session=do-not-show"},
					"metadata":{"refresh-token":"do-not-show"}
				}`),
			})
		}),
	})
	if _, err := runtime.Send(context.Background(), "run it", nil); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load(context.Background(), "redaction")
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(loaded)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), secret) {
		t.Fatalf("session leaked credential: %s", encoded)
	}
}

func TestRenderersAreProtocolSafe(t *testing.T) {
	text := event(t, provider.EventTextDelta, map[string]string{"delta": "ok"})
	var plain bytes.Buffer
	if err := (PlainRenderer{Stdout: &plain}).Emit(context.Background(), text); err != nil {
		t.Fatal(err)
	}
	if plain.String() != "ok" {
		t.Fatal(plain.String())
	}
	var lines bytes.Buffer
	if err := (JSONLRenderer{Writer: &lines}).Emit(context.Background(), text); err != nil {
		t.Fatal(err)
	}
	var decoded provider.Event
	if err := json.Unmarshal(bytes.TrimSpace(lines.Bytes()), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Type != provider.EventTextDelta || strings.Contains(lines.String(), "\\x1b") {
		t.Fatalf("line=%q", lines.String())
	}
}

func TestEventTextStringAndMap(t *testing.T) {
	e := provider.Event{SchemaVersion: provider.EventSchemaVersion, Type: provider.EventTextDelta, At: time.Now(), Data: []byte(`"one"`)}
	if EventText(e) != "one" {
		t.Fatal(EventText(e))
	}
}

func TestApprovalBrokerWaitsForDecision(t *testing.T) {
	broker := NewApprovalBroker()
	done := make(chan error, 1)
	go func() {
		done <- broker.Authorize(context.Background(), tools.Call{ID: "1", Name: "read_file", Arguments: []byte(`{}`)})
	}()
	request := <-broker.Requests()
	if request.Call.Name != "read_file" {
		t.Fatal(request.Call)
	}
	request.Allow()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

type testAuthorizationController struct {
	mode policy.Mode
	err  error
}

func (c *testAuthorizationController) Mode() policy.Mode { return c.mode }
func (c *testAuthorizationController) SetMode(mode policy.Mode) error {
	if c.err != nil {
		return c.err
	}
	c.mode = mode
	return nil
}

type controlledAuthorizer struct{ *testAuthorizationController }

func (controlledAuthorizer) Authorize(context.Context, tools.Call) error { return nil }

func TestRuntimePermissionModeUsesInjectedAuthorizerController(t *testing.T) {
	authorizer := controlledAuthorizer{&testAuthorizationController{mode: policy.ModeWorkspaceWrite}}
	runtime := NewRuntime(Options{Executor: Kernel{Authorizer: authorizer}})
	if err := runtime.SetPermissionMode(policy.ModeWorkspaceReadOnly); err != nil {
		t.Fatal(err)
	}
	if authorizer.mode != policy.ModeWorkspaceReadOnly || runtime.Permissions != "workspace-read-only" {
		t.Fatalf("controller=%s runtime=%q", authorizer.mode, runtime.Permissions)
	}
	if err := runtime.SetPermissionMode(policy.Mode("ask")); err == nil {
		t.Fatal("undocumented mode was accepted")
	}
}

func TestRuntimePermissionModeRejectsMetadataOnlyRuntime(t *testing.T) {
	runtime := NewRuntime(Options{Permissions: "ask"})
	if runtime.Permissions != "" {
		t.Fatalf("undocumented initial mode retained: %q", runtime.Permissions)
	}
	if err := runtime.SetPermissionMode(policy.ModeUnrestricted); err == nil || !strings.Contains(err.Error(), "mode-aware authorizer") {
		t.Fatalf("metadata-only runtime mode change = %v", err)
	}
}

func TestRuntimeApprovalBrokerIdentityMatchesKernelAuthorizer(t *testing.T) {
	broker := NewApprovalBroker()
	runtime := NewRuntime(Options{Executor: Kernel{Authorizer: broker}})
	if !runtime.UsesApprovalBroker(broker) {
		t.Fatal("runtime did not expose its broker authorization path")
	}
	if runtime.UsesApprovalBroker(NewApprovalBroker()) {
		t.Fatal("runtime accepted a different displayed broker")
	}
}

func TestApprovalPreviewRedactsAndSummarizesPatch(t *testing.T) {
	preview := (&ApprovalRequest{Call: tools.Call{ID: "1", Name: "write_file", Arguments: json.RawMessage(`{"path":"a.txt","patch":"--- a.txt\n+++ a.txt\n-old\n+new","token":"do-not-show"}`)}}).Preview()
	if !strings.Contains(preview.Scope, `path="a.txt"`) || !strings.Contains(preview.DiffSummary, "1 addition(s), 1 removal(s)") {
		t.Fatalf("preview = %#v", preview)
	}
	if strings.Contains(preview.Arguments, "do-not-show") || strings.Contains(preview.Arguments, "-old") || !strings.Contains(preview.Arguments, "[REDACTED]") {
		t.Fatalf("unsafe arguments = %s", preview.Arguments)
	}
}

func TestApprovalPreviewRedactsCommandLineSecretFlags(t *testing.T) {
	preview := (&ApprovalRequest{Call: tools.Call{ID: "1", Name: "exec", Arguments: json.RawMessage(`{"command":"curl --token do-not-show https://example.test"}`)}}).Preview()
	if strings.Contains(preview.Arguments, "do-not-show") || !strings.Contains(preview.Arguments, "[REDACTED]") {
		t.Fatalf("unsafe command preview = %s", preview.Arguments)
	}
}

func TestApprovalPreviewRedactsHTTPHeadersAndURICredentials(t *testing.T) {
	const secret = "do-not-show"
	preview := (&ApprovalRequest{Call: tools.Call{ID: "1", Name: "exec", Arguments: json.RawMessage(`{
		"command":"curl -H 'X-API-Key: do-not-show' -H 'Cookie: session=do-not-show' https://user:do-not-show@example.test/?api-key=do-not-show",
		"headers":{"X-API-Key":"do-not-show","Authorization":"Bearer do-not-show","Cookie":"session=do-not-show"},
		"metadata":{"access-token":"do-not-show"}
	}`)}}).Preview()
	if strings.Contains(preview.Arguments, secret) || strings.Contains(preview.Scope, secret) || strings.Contains(preview.DiffSummary, secret) {
		t.Fatalf("unsafe approval preview = %#v", preview)
	}
}

func TestJSONLRendererRedactsCommandHeaderAndMetadataPayloads(t *testing.T) {
	const secret = "do-not-show"
	event := provider.Event{
		SchemaVersion: provider.EventSchemaVersion,
		Type:          provider.EventToolCall,
		At:            time.Now(),
		Data: json.RawMessage(`{
			"command":"curl -H 'X-API-Key: do-not-show' https://api.example.test/?refresh-token=do-not-show",
			"headers":{"X-API-Key":"do-not-show","Cookie":"session=do-not-show"},
			"metadata":{"access-token":"do-not-show"}
		}`),
	}
	var output bytes.Buffer
	if err := (JSONLRenderer{Writer: &output}).Emit(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), secret) {
		t.Fatalf("renderer leaked credential: %s", output.String())
	}
}

func TestRuntimeRejectsUnavailableExecutorBeforeStarting(t *testing.T) {
	runtime := NewRuntime(Options{})
	if err := runtime.Ready(); err == nil || !strings.Contains(err.Error(), "configure") {
		t.Fatalf("ready error = %v", err)
	}
}

func TestRuntimeSerializesConcurrentTurns(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	runner := TurnExecutorFunc(func(context.Context, provider.Request, EventSink) (agent.TurnResult, error) {
		if calls.Add(1) == 1 {
			close(entered)
		}
		<-release
		return agent.TurnResult{Completed: true}, nil
	})
	runtime := NewRuntime(Options{Executor: runner})
	var group sync.WaitGroup
	group.Add(2)
	for _, prompt := range []string{"one", "two"} {
		go func(prompt string) { defer group.Done(); _, _ = runtime.Send(context.Background(), prompt, nil) }(prompt)
	}
	<-entered
	time.Sleep(25 * time.Millisecond)
	if calls.Load() != 1 {
		t.Fatalf("concurrent turns entered executor: %d", calls.Load())
	}
	close(release)
	group.Wait()
	if calls.Load() != 2 || len(runtime.messages) != 2 {
		t.Fatalf("calls=%d messages=%#v", calls.Load(), runtime.messages)
	}
}

func TestRuntimePersistsCancelledOutcome(t *testing.T) {
	store, err := session.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	runner := TurnExecutorFunc(func(ctx context.Context, _ provider.Request, _ EventSink) (agent.TurnResult, error) {
		close(started)
		<-ctx.Done()
		return agent.TurnResult{}, ctx.Err()
	})
	runtime := NewRuntime(Options{Executor: runner, Sessions: store, NewSessionID: func() string { return "cancelled" }})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, sendErr := runtime.Send(ctx, "wait", nil); done <- sendErr }()
	<-started
	cancel()
	if sendErr := <-done; !errors.Is(sendErr, context.Canceled) {
		t.Fatalf("send error = %v", sendErr)
	}
	loaded, err := store.Load(context.Background(), "cancelled")
	if err != nil {
		t.Fatal(err)
	}
	var outcome TurnOutcome
	for _, record := range loaded.Records {
		if record.Kind == "turn.outcome" {
			if err := json.Unmarshal(record.Data, &outcome); err != nil {
				t.Fatal(err)
			}
		}
	}
	if outcome.Status != "cancelled" {
		t.Fatalf("outcome=%#v", outcome)
	}
}

func TestRuntimeResumeForkRetainsConversationAndKeepsParentIndependent(t *testing.T) {
	store, err := session.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	events, err := testkit.LoadEvents("terminal", "provider-events.json")
	if err != nil {
		t.Fatal(err)
	}
	first := &testkit.Script{Events: events, Result: agent.TurnResult{Completed: true}}
	parent := NewRuntime(Options{
		Executor: TurnExecutorFunc(func(ctx context.Context, request provider.Request, sink EventSink) (agent.TurnResult, error) {
			return first.Run(ctx, request, sink.Emit)
		}),
		Sessions:     store,
		NewSessionID: func() string { return "parent" },
	})
	if _, err := parent.Send(context.Background(), "first prompt", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Fork(context.Background(), "parent", "child"); err != nil {
		t.Fatal(err)
	}

	childScript := &testkit.Script{Events: []provider.Event{event(t, provider.EventTextDelta, map[string]string{"text": "child answer"})}, Result: agent.TurnResult{Completed: true}}
	child := NewRuntime(Options{
		Executor: TurnExecutorFunc(func(ctx context.Context, request provider.Request, sink EventSink) (agent.TurnResult, error) {
			return childScript.Run(ctx, request, sink.Emit)
		}),
		Sessions: store,
	})
	if err := child.Resume(context.Background(), "child"); err != nil {
		t.Fatal(err)
	}
	if _, err := child.Send(context.Background(), "continued prompt", nil); err != nil {
		t.Fatal(err)
	}

	requests := childScript.Requests()
	if len(requests) != 1 {
		t.Fatalf("child requests = %#v", requests)
	}
	want := []provider.Message{
		{Role: provider.RoleUser, Content: "first prompt"},
		{Role: provider.RoleAssistant, Content: "fixture answer"},
		{Role: provider.RoleUser, Content: "continued prompt"},
	}
	if !sameMessages(requests[0].Messages, want) {
		t.Fatalf("resumed messages = %#v, want %#v", requests[0].Messages, want)
	}
	parentSession, err := store.Load(context.Background(), "parent")
	if err != nil {
		t.Fatal(err)
	}
	childSession, err := store.Load(context.Background(), "child")
	if err != nil {
		t.Fatal(err)
	}
	if childSession.ParentID != "parent" || len(childSession.Records) <= len(parentSession.Records) {
		t.Fatalf("fork provenance/history lost: parent=%#v child=%#v", parentSession, childSession)
	}
	for _, record := range parentSession.Records {
		if bytes.Contains(record.Data, []byte("continued prompt")) {
			t.Fatalf("child turn leaked into parent record %#v", record)
		}
	}
}

func TestRuntimeCanContinueAfterCancelledTurn(t *testing.T) {
	started := make(chan struct{})
	var calls atomic.Int32
	var requests []provider.Request
	var requestsMu sync.Mutex
	runner := TurnExecutorFunc(func(ctx context.Context, request provider.Request, sink EventSink) (agent.TurnResult, error) {
		requestsMu.Lock()
		requests = append(requests, request)
		requestsMu.Unlock()
		if calls.Add(1) == 1 {
			close(started)
			<-ctx.Done()
			return agent.TurnResult{}, ctx.Err()
		}
		if err := sink.Emit(ctx, event(t, provider.EventTextDelta, map[string]string{"text": "recovered"})); err != nil {
			return agent.TurnResult{}, err
		}
		return agent.TurnResult{Completed: true}, nil
	})
	runtime := NewRuntime(Options{Executor: runner})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := runtime.Send(ctx, "cancel me", nil); done <- err }()
	<-started
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled turn error = %v", err)
	}
	if _, err := runtime.Send(context.Background(), "continue", nil); err != nil {
		t.Fatal(err)
	}
	requestsMu.Lock()
	defer requestsMu.Unlock()
	if len(requests) != 2 || !sameMessages(requests[1].Messages, []provider.Message{
		{Role: provider.RoleUser, Content: "cancel me"},
		{Role: provider.RoleUser, Content: "continue"},
	}) {
		t.Fatalf("post-cancellation request = %#v", requests)
	}
}

func TestRuntimePersistsAndReplaysToolResults(t *testing.T) {
	store, err := session.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	call := tools.Call{ID: "call-1", Name: "read_file", Arguments: json.RawMessage(`{"path":"README.md"}`)}
	toolResult, err := tools.JSONResultAt(time.Date(2026, 8, 10, 3, 4, 5, 0, time.UTC), call, map[string]string{"content": "persisted"})
	if err != nil {
		t.Fatal(err)
	}
	first := NewRuntime(Options{
		Sessions: store, NewSessionID: func() string { return "tool-history" },
		Executor: TurnExecutorFunc(func(ctx context.Context, _ provider.Request, sink EventSink) (agent.TurnResult, error) {
			if err := sink.Emit(ctx, event(t, provider.EventToolCall, call)); err != nil {
				return agent.TurnResult{}, err
			}
			return agent.TurnResult{ToolResults: []tools.Result{toolResult}, Completed: true}, nil
		}),
	})
	if _, err := first.Send(context.Background(), "read it", nil); err != nil {
		t.Fatal(err)
	}
	var resumedRequest provider.Request
	resumed := NewRuntime(Options{
		Sessions: store,
		Executor: TurnExecutorFunc(func(_ context.Context, request provider.Request, _ EventSink) (agent.TurnResult, error) {
			resumedRequest = request
			return agent.TurnResult{Completed: true}, nil
		}),
	})
	if err := resumed.Resume(context.Background(), "tool-history"); err != nil {
		t.Fatal(err)
	}
	if _, err := resumed.Send(context.Background(), "continue", nil); err != nil {
		t.Fatal(err)
	}
	if len(resumedRequest.Messages) != 3 || resumedRequest.Messages[1].Role != provider.RoleTool || resumedRequest.Messages[1].ToolCallID != "call-1" || !bytes.Contains(resumedRequest.Messages[1].Data, []byte("persisted")) {
		t.Fatalf("tool history missing from resumed request: %#v", resumedRequest.Messages)
	}
	loaded, err := store.Load(context.Background(), "tool-history")
	if err != nil {
		t.Fatal(err)
	}
	if !hasRecordKind(loaded, "tool.result") {
		t.Fatalf("tool result not persisted: %#v", loaded.Records)
	}
}

func TestRuntimeResumeAndForkExplicitlyRecoverTornSession(t *testing.T) {
	store, err := session.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(context.Background(), "torn", nil); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(store.Root(), "torn.jsonl")
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(`{"schema_version":`); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	runtime := NewRuntime(Options{Sessions: store, Executor: TurnExecutorFunc(func(context.Context, provider.Request, EventSink) (agent.TurnResult, error) {
		return agent.TurnResult{}, nil
	})})
	if err := runtime.Resume(context.Background(), "torn"); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load(context.Background(), "torn")
	if err != nil || loaded.Recovered {
		t.Fatalf("resume did not explicitly recover: %#v err=%v", loaded, err)
	}
	if _, err := runtime.Fork(context.Background(), "torn", "torn-child"); err != nil {
		t.Fatalf("fork after explicit recovery: %v", err)
	}
}

func TestRecoveredSessionCanResumeAndFork(t *testing.T) {
	store, err := session.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := os.ReadFile(testkit.FixturePath("terminal", "recovery-session.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	// A torn append has no trailing newline. Keep the fixture readable in the
	// repository while writing the exact crash signature expected by Store.
	fixture = bytes.TrimSuffix(fixture, []byte("\n"))
	if err := os.WriteFile(filepath.Join(store.Root(), "recovery.jsonl"), fixture, 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load(context.Background(), "recovery")
	if err != nil || !loaded.Recovered {
		t.Fatalf("torn fixture was not detected: session=%#v err=%v", loaded, err)
	}
	if _, err := store.Fork(context.Background(), "recovery", "must-not-fork"); err == nil {
		t.Fatal("unrecovered session must not fork")
	}
	if _, err := store.Recover(context.Background(), "recovery"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Fork(context.Background(), "recovery", "recovered-child"); err != nil {
		t.Fatal(err)
	}
	script := &testkit.Script{Events: []provider.Event{event(t, provider.EventTextDelta, map[string]string{"text": "after recovery"})}, Result: agent.TurnResult{Completed: true}}
	runtime := NewRuntime(Options{
		Executor: TurnExecutorFunc(func(ctx context.Context, request provider.Request, sink EventSink) (agent.TurnResult, error) {
			return script.Run(ctx, request, sink.Emit)
		}),
		Sessions: store,
	})
	if err := runtime.Resume(context.Background(), "recovered-child"); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.Send(context.Background(), "continue safely", nil); err != nil {
		t.Fatal(err)
	}
	requests := script.Requests()
	if len(requests) != 1 || !sameMessages(requests[0].Messages, []provider.Message{
		{Role: provider.RoleUser, Content: "recovered prompt"},
		{Role: provider.RoleUser, Content: "continue safely"},
	}) {
		t.Fatalf("recovered resume request = %#v", requests)
	}
}

func sameMessages(got, want []provider.Message) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i].Role != want[i].Role || got[i].Content != want[i].Content {
			return false
		}
	}
	return true
}

func hasRecordKind(loaded session.Session, kind string) bool {
	for _, record := range loaded.Records {
		if record.Kind == kind {
			return true
		}
	}
	return false
}
