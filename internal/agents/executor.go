package agents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// TurnExecutor bridges the agent queue to Lana's turn runner. It receives a
// task's structured input, constructs a provider request, and records the
// resulting events as the task's result. The executor never interprets task
// input as shell text or configures providers directly.
type TurnExecutor struct {
	Provider Provider
}

// Provider is the minimum interface needed to run a turn. Embedding
// applications supply a real provider; tests inject a stub.
type Provider interface {
	Stream(context.Context, Request) (Stream, error)
}

// Request mirrors the provider.Request shape used by the turn runner.
type Request struct {
	Messages []Message
}

// Message mirrors the provider.Message shape.
type Message struct {
	Role string
	Name string
	Data json.RawMessage
}

// Stream mirrors the provider.Stream shape.
type Stream interface {
	Recv(context.Context) (Event, error)
	Close() error
}

// Event mirrors the provider.Event shape.
type Event struct {
	Type string
	Data json.RawMessage
}

// NewTurnExecutor creates an executor that runs tasks through the turn runner.
func NewTurnExecutor(provider Provider) *TurnExecutor {
	return &TurnExecutor{Provider: provider}
}

// Execute runs a single task through the turn runner. The task's input is
// passed as the first message's data. The result contains the collected
// events from the turn.
func (e *TurnExecutor) Execute(ctx context.Context, task Task) (Result, error) {
	if e.Provider == nil {
		return Result{}, errors.New("agent provider is required")
	}

	// Construct the request from task input
	request := Request{}
	if len(task.Input) > 0 {
		request.Messages = append(request.Messages, Message{
			Role: "user",
			Data: task.Input,
		})
	}

	// Run the turn
	stream, err := e.Provider.Stream(ctx, request)
	if err != nil {
		return Result{}, fmt.Errorf("start provider stream: %w", err)
	}
	defer stream.Close()

	var events []Event
	for {
		event, err := stream.Recv(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				task.CancelRequested = true
				return Result{Output: marshalEvents(events)}, nil
			}
			return Result{}, fmt.Errorf("receive event: %w", err)
		}
		events = append(events, event)
	}
}

func marshalEvents(events []Event) json.RawMessage {
	if len(events) == 0 {
		return json.RawMessage(`[]`)
	}
	data, _ := json.Marshal(events)
	return data
}
