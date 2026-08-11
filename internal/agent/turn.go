// Package agent coordinates a single cancellation-aware model turn.
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/deagy/lana/internal/provider"
	"github.com/deagy/lana/internal/tools"
)

const DefaultMaxToolRounds = 32

type EventSink interface {
	Emit(context.Context, provider.Event) error
}
type EventSinkFunc func(context.Context, provider.Event) error

func (f EventSinkFunc) Emit(ctx context.Context, event provider.Event) error { return f(ctx, event) }

// TurnRunner owns no provider credentials or authorization policy. Both are
// injected behind interfaces so terminal UIs, CLIs, and tests can choose their
// own provider and approval behavior.
type TurnRunner struct {
	Provider      provider.Client
	Authorizer    tools.Authorizer
	Executor      tools.Executor
	Sink          EventSink
	MaxToolRounds int
}

type TurnResult struct {
	Events      []provider.Event
	ToolResults []tools.Result
	Rounds      int
	Completed   bool
}

// Run streams model events and feeds every requested tool result back into the
// next provider request. It returns promptly when ctx is cancelled, including
// while a tool or a provider is blocked.
func (r TurnRunner) Run(ctx context.Context, request provider.Request) (TurnResult, error) {
	if r.Provider == nil {
		return TurnResult{}, errors.New("agent provider is required")
	}
	if r.Authorizer == nil {
		return TurnResult{}, errors.New("agent tool authorizer is required")
	}
	if r.Executor == nil {
		return TurnResult{}, errors.New("agent tool executor is required")
	}
	maxRounds := r.MaxToolRounds
	if maxRounds == 0 {
		maxRounds = DefaultMaxToolRounds
	}
	if maxRounds < 1 {
		return TurnResult{}, errors.New("agent max tool rounds must be positive")
	}

	result := TurnResult{}
	for round := 0; round < maxRounds; round++ {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		stream, err := r.Provider.Stream(ctx, request)
		if err != nil {
			return result, fmt.Errorf("start provider stream: %w", err)
		}
		toolRequested := false
		streamDone := false
		for !streamDone {
			event, receiveErr := stream.Recv(ctx)
			if receiveErr != nil {
				_ = stream.Close()
				if errors.Is(receiveErr, io.EOF) {
					break
				}
				return result, fmt.Errorf("receive provider event: %w", receiveErr)
			}
			if err := event.Validate(); err != nil {
				_ = stream.Close()
				return result, err
			}
			result.Events = append(result.Events, event)
			if r.Sink != nil {
				if err := r.Sink.Emit(ctx, event); err != nil {
					_ = stream.Close()
					return result, fmt.Errorf("emit agent event: %w", err)
				}
			}
			switch event.Type {
			case provider.EventToolCall:
				call, err := decodeCall(event)
				if err != nil {
					_ = stream.Close()
					return result, err
				}
				toolRequested = true
				toolResult, rejected := r.runTool(ctx, call)
				result.ToolResults = append(result.ToolResults, toolResult)
				if rejected {
					event, err := rejectedToolResultEvent(toolResult.At)
					if err != nil {
						_ = stream.Close()
						return result, fmt.Errorf("create rejected tool result event: %w", err)
					}
					result.Events = append(result.Events, event)
					if r.Sink != nil {
						if err := r.Sink.Emit(ctx, event); err != nil {
							_ = stream.Close()
							return result, fmt.Errorf("emit agent event: %w", err)
						}
					}
				}
				encoded, err := json.Marshal(toolResult)
				if err != nil {
					_ = stream.Close()
					return result, fmt.Errorf("marshal tool result: %w", err)
				}
				request.Messages = append(request.Messages, provider.Message{Role: provider.RoleTool, Name: call.Name, ToolCallID: call.ID, Data: encoded})
			case provider.EventMessageEnd:
				streamDone = true
			}
		}
		_ = stream.Close()
		result.Rounds = round + 1
		if !toolRequested {
			result.Completed = true
			return result, nil
		}
	}
	return result, fmt.Errorf("agent turn exceeded %d tool rounds", maxRounds)
}

func decodeCall(event provider.Event) (tools.Call, error) {
	var call tools.Call
	if err := json.Unmarshal(event.Data, &call); err != nil {
		return tools.Call{}, fmt.Errorf("decode tool call: %w", err)
	}
	if err := call.Validate(); err != nil {
		return tools.Call{}, fmt.Errorf("invalid tool call: %w", err)
	}
	return call, nil
}

func (r TurnRunner) runTool(ctx context.Context, call tools.Call) (tools.Result, bool) {
	if err := r.Authorizer.Authorize(ctx, call); err != nil {
		return tools.ErrorResult(call, "authorization_denied", err), false
	}
	toolResult, err := r.Executor.Execute(ctx, call)
	if err != nil {
		return tools.ErrorResult(call, "execution_failed", err), false
	}
	if toolResult.SchemaVersion == 0 {
		toolResult.SchemaVersion = tools.ResultSchemaVersion
	}
	if toolResult.CallID == "" {
		toolResult.CallID = call.ID
	}
	if toolResult.Name == "" {
		toolResult.Name = call.Name
	}
	// Validate before sanitizing. RedactJSON intentionally turns malformed JSON
	// into a safe JSON placeholder, which is correct for rendering but must not
	// let an executor's malformed result become provider-visible.
	if err := toolResult.Validate(); err != nil {
		return tools.ErrorResult(call, "invalid_tool_result", errors.New("tool result rejected")), true
	}
	toolResult = tools.SanitizeResult(toolResult)
	if err := toolResult.Validate(); err != nil {
		return tools.ErrorResult(call, "invalid_tool_result", errors.New("tool result rejected")), true
	}
	return toolResult, false
}

func rejectedToolResultEvent(at time.Time) (provider.Event, error) {
	return provider.NewEventAt(at, provider.EventError, struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}{
		Code:    "invalid_tool_result",
		Message: "tool result rejected",
	})
}
