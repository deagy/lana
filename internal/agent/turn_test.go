package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/deagy/lana/internal/provider"
	"github.com/deagy/lana/internal/tools"
)

func TestTurnRunnerFeedsToolResultIntoNextProviderRequest(t *testing.T) {
	callData, err := json.Marshal(tools.Call{ID: "call-1", Name: "read_file", Arguments: json.RawMessage(`{"path":"README.md"}`)})
	if err != nil {
		t.Fatal(err)
	}
	toolCall := provider.Event{SchemaVersion: provider.EventSchemaVersion, ID: "event-1", Type: provider.EventToolCall, At: now(), Data: callData}
	end := provider.Event{SchemaVersion: provider.EventSchemaVersion, ID: "event-2", Type: provider.EventMessageEnd, At: now()}
	client := &provider.StaticClient{Streams: []provider.Stream{provider.NewSliceStream(toolCall, end), provider.NewSliceStream(end)}}
	runner := TurnRunner{
		Provider:   client,
		Authorizer: tools.AllowAll{},
		Executor: tools.ExecutorFunc(func(_ context.Context, call tools.Call) (tools.Result, error) {
			return tools.JSONResult(call, tools.ReadFileOutput{Path: "README.md", Content: "ok"})
		}),
	}
	result, err := runner.Run(context.Background(), provider.Request{Messages: []provider.Message{{Role: provider.RoleUser, Content: "read it"}}})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Completed || result.Rounds != 2 || len(result.ToolResults) != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if len(client.Requests) != 2 || len(client.Requests[1].Messages) != 2 {
		t.Fatalf("tool result was not added to next request: %#v", client.Requests)
	}
	if client.Requests[1].Messages[1].Role != provider.RoleTool {
		t.Fatal("expected tool message")
	}
}

func TestTurnRunnerAuthorizationFailureReturnsToolResult(t *testing.T) {
	data, err := json.Marshal(tools.Call{ID: "call-1", Name: "exec", Arguments: json.RawMessage(`{"command":"false"}`)})
	if err != nil {
		t.Fatal(err)
	}
	toolCall := provider.Event{SchemaVersion: provider.EventSchemaVersion, Type: provider.EventToolCall, At: now(), Data: data}
	end := provider.Event{SchemaVersion: provider.EventSchemaVersion, Type: provider.EventMessageEnd, At: now()}
	runner := TurnRunner{Provider: &provider.StaticClient{Streams: []provider.Stream{provider.NewSliceStream(toolCall, end), provider.NewSliceStream(end)}}, Authorizer: tools.AuthorizerFunc(func(context.Context, tools.Call) error { return errors.New("TOKEN=topsecret") }), Executor: tools.ExecutorFunc(func(context.Context, tools.Call) (tools.Result, error) {
		t.Fatal("executor must not run")
		return tools.Result{}, nil
	})}
	result, err := runner.Run(context.Background(), provider.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.ToolResults) != 1 || !result.ToolResults[0].IsError || result.ToolResults[0].ErrorCode != "authorization_denied" {
		t.Fatalf("unexpected results: %#v", result.ToolResults)
	}
	if string(result.ToolResults[0].Content) == `"TOKEN=topsecret"` {
		t.Fatal("secret leaked into tool result")
	}
}

func TestTurnRunnerSanitizesCustomExecutorResultBeforeProviderRequest(t *testing.T) {
	call := tools.Call{ID: "call-1", Name: "read_file", Arguments: json.RawMessage(`{"path":"README.md"}`)}
	callData, err := json.Marshal(call)
	if err != nil {
		t.Fatal(err)
	}
	toolCall := provider.Event{SchemaVersion: provider.EventSchemaVersion, ID: "event-1", Type: provider.EventToolCall, At: now(), Data: callData}
	end := provider.Event{SchemaVersion: provider.EventSchemaVersion, ID: "event-2", Type: provider.EventMessageEnd, At: now()}
	client := &provider.StaticClient{Streams: []provider.Stream{provider.NewSliceStream(toolCall, end), provider.NewSliceStream(end)}}
	runner := TurnRunner{
		Provider:   client,
		Authorizer: tools.AllowAll{},
		Executor: tools.ExecutorFunc(func(context.Context, tools.Call) (tools.Result, error) {
			return tools.Result{
				Content:   json.RawMessage(`{"contents":"ok","api_token":"topsecret"}`),
				IsError:   true,
				ErrorCode: " Invalid code! ",
				At:        time.Date(2026, 8, 10, 3, 4, 5, 0, time.FixedZone("MST", -7*60*60)),
			}, nil
		}),
	}

	result, err := runner.Run(context.Background(), provider.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.ToolResults) != 1 || result.ToolResults[0].Validate() != nil {
		t.Fatalf("unexpected tool result: %#v", result.ToolResults)
	}
	toolResult := result.ToolResults[0]
	if toolResult.CallID != call.ID || toolResult.Name != call.Name || toolResult.SchemaVersion != tools.ResultSchemaVersion || toolResult.At.Location() != time.UTC || !toolResult.IsError || toolResult.ErrorCode != "tool_failed" {
		t.Fatalf("custom result was not normalized: %#v", toolResult)
	}
	if strings.Contains(string(toolResult.Content), "topsecret") || !strings.Contains(string(toolResult.Content), "[REDACTED]") {
		t.Fatalf("secret was not redacted: %s", toolResult.Content)
	}
	if len(client.Requests) != 2 || strings.Contains(string(client.Requests[1].Messages[0].Data), "topsecret") {
		t.Fatalf("secret leaked into provider request: %#v", client.Requests)
	}
}

func TestTurnRunnerRejectsInvalidCustomExecutorResultWithoutLeakingPayload(t *testing.T) {
	call := tools.Call{ID: "call-1", Name: "read_file", Arguments: json.RawMessage(`{"path":"README.md"}`)}
	callData, err := json.Marshal(call)
	if err != nil {
		t.Fatal(err)
	}
	toolCall := provider.Event{SchemaVersion: provider.EventSchemaVersion, ID: "event-1", Type: provider.EventToolCall, At: now(), Data: callData}
	end := provider.Event{SchemaVersion: provider.EventSchemaVersion, ID: "event-2", Type: provider.EventMessageEnd, At: now()}
	var emitted []provider.Event
	client := &provider.StaticClient{Streams: []provider.Stream{provider.NewSliceStream(toolCall, end), provider.NewSliceStream(end)}}
	runner := TurnRunner{
		Provider:   client,
		Authorizer: tools.AllowAll{},
		Executor: tools.ExecutorFunc(func(context.Context, tools.Call) (tools.Result, error) {
			return tools.Result{Content: json.RawMessage(`{"api_token":"topsecret"`), At: now()}, nil
		}),
		Sink: EventSinkFunc(func(_ context.Context, event provider.Event) error {
			emitted = append(emitted, event)
			return nil
		}),
	}

	result, err := runner.Run(context.Background(), provider.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.ToolResults) != 1 {
		t.Fatalf("unexpected tool results: %#v", result.ToolResults)
	}
	toolResult := result.ToolResults[0]
	if !toolResult.IsError || toolResult.ErrorCode != "invalid_tool_result" || toolResult.Validate() != nil {
		t.Fatalf("invalid result was not replaced with a safe error: %#v", toolResult)
	}
	if strings.Contains(string(toolResult.Content), "topsecret") {
		t.Fatalf("secret leaked into rejected result: %s", toolResult.Content)
	}
	if len(client.Requests) != 2 || strings.Contains(string(client.Requests[1].Messages[0].Data), "topsecret") {
		t.Fatalf("secret leaked into provider request: %#v", client.Requests)
	}
	if len(result.Events) != 4 || result.Events[1].Type != provider.EventError || result.Events[1].ErrorCode != "invalid_tool_result" || strings.Contains(string(result.Events[1].Data), "topsecret") {
		t.Fatalf("rejected result did not produce a safe structured event: %#v", result.Events)
	}
	if len(emitted) != 4 || emitted[1].Type != provider.EventError {
		t.Fatalf("rejected result event was not emitted: %#v", emitted)
	}
}

func TestTurnRunnerHonorsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runner := TurnRunner{Provider: &provider.StaticClient{}, Authorizer: tools.AllowAll{}, Executor: tools.ExecutorFunc(func(context.Context, tools.Call) (tools.Result, error) { return tools.Result{}, nil })}
	_, err := runner.Run(ctx, provider.Request{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want canceled", err)
	}
}
