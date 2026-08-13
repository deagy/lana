package agent

import (
	"context"
	"io"

	"github.com/deagy/lana/internal/execution"
	"github.com/deagy/lana/internal/provider"
	"github.com/deagy/lana/internal/session"
)

// Turn represents a single exchange with the provider.
type Turn struct {
	Session   *session.Session
	Pipeline  *EventPipeline
	Executor  *execution.Executor
	Provider  provider.Client
}

// NewTurn creates a new turn.
func NewTurn(sess *session.Session, executor *execution.Executor, client provider.Client) *Turn {
	return &Turn{
		Session:  sess,
		Pipeline: NewEventPipeline(100),
		Executor: executor,
		Provider: client,
	}
}

// Run executes a single turn.
func (t *Turn) Run(ctx context.Context, userMessage string) error {
	defer t.Pipeline.Close()

	// Add user message to session
	userMsg := &session.Message{
		Role:    "user",
		Content: userMessage,
	}
	t.Session.Transcript = append(t.Session.Transcript, *userMsg)

	// Build request
	msgs := make([]provider.Message, len(t.Session.Transcript))
	for i, msg := range t.Session.Transcript {
		msgs[i] = provider.Message{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	req := &provider.Request{
		Messages: msgs,
		Model:    t.Session.Model,
	}

	// Stream from provider
	go t.streamFromProvider(ctx, req)

	// Process events
	return t.processEvents(ctx)
}

func (t *Turn) streamFromProvider(ctx context.Context, req *provider.Request) {
	defer close(t.Pipeline.events)

	reader, err := t.Provider.Chat(ctx, req)
	if err != nil {
		t.Pipeline.Send(&ErrorEvent{Error: err.Error()})
		return
	}
	defer reader.Close()

	for {
		event, err := reader.NextEvent(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Pipeline.Send(&ErrorEvent{Error: err.Error()})
			break
		}

		t.Pipeline.Send(&StreamEvent{ProviderEvent: event})
	}

	t.Pipeline.Send(&DoneEvent{})
}

func (t *Turn) processEvents(ctx context.Context) error {
	var currentMessage string
	var toolCalls []provider.ToolCallEvent

	for event := range t.Pipeline.Receive() {
		switch e := event.(type) {
		case *StreamEvent:
			t.handleStreamEvent(ctx, e.ProviderEvent, &currentMessage, &toolCalls)

		case *ErrorEvent:
			// Error occurred
			return nil // Error already logged
		}
	}

	// Save assistant message if any content
	if currentMessage != "" || len(toolCalls) > 0 {
		assistantMsg := &session.Message{
			Role:    "assistant",
			Content: currentMessage,
		}
		t.Session.Transcript = append(t.Session.Transcript, *assistantMsg)
	}

	return nil
}

func (t *Turn) handleStreamEvent(ctx context.Context, event provider.Event, currentMsg *string, toolCalls *[]provider.ToolCallEvent) {
	switch e := event.(type) {
	case *provider.MessageStartEvent:
		// Start of message
		t.Pipeline.Send(&StreamEvent{ProviderEvent: event})

	case *provider.MessageDeltaEvent:
		*currentMsg += e.Content
		t.Pipeline.Send(&StreamEvent{ProviderEvent: event})

	case *provider.ToolCallEvent:
		*toolCalls = append(*toolCalls, *e)
		t.Pipeline.Send(&ToolCallStartEvent{
			ID:       e.ID,
			ToolName: e.Name,
			Input:    e.Input,
		})

		// Execute tool
		result, err := t.Executor.Execute(ctx, e.ID, e.Name, e.Input)
		if err != nil {
			t.Pipeline.Send(&ToolCallResultEvent{
				ID:       e.ID,
				ToolName: e.Name,
				Error:    err.Error(),
			})
		} else {
			t.Pipeline.Send(&ToolCallResultEvent{
				ID:       e.ID,
				ToolName: e.Name,
				Output:   result.Output,
				Approved: result.Approved,
			})
		}

	case *provider.MessageEndEvent:
		t.Pipeline.Send(&StreamEvent{ProviderEvent: event})

	case *provider.ErrorEvent:
		t.Pipeline.Send(&ErrorEvent{Error: e.Message})
	}
}
