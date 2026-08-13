package agent

import (
	"encoding/json"

	"github.com/deagy/lana/internal/provider"
)

// Event represents any event in the agent pipeline.
type Event interface {
	EventType() string
}

// StreamEvent wraps a provider event.
type StreamEvent struct {
	ProviderEvent provider.Event
}

func (e *StreamEvent) EventType() string {
	return "stream"
}

// ToolCallStartEvent signals a tool call is beginning.
type ToolCallStartEvent struct {
	ID       string
	ToolName string
	Input    json.RawMessage
}

func (e *ToolCallStartEvent) EventType() string {
	return "tool.start"
}

// ToolCallResultEvent signals tool execution completion.
type ToolCallResultEvent struct {
	ID       string
	ToolName string
	Output   string
	Error    string
	Approved bool
}

func (e *ToolCallResultEvent) EventType() string {
	return "tool.result"
}

// ErrorEvent signals an error.
type ErrorEvent struct {
	Error string
}

func (e *ErrorEvent) EventType() string {
	return "error"
}

// DoneEvent signals turn completion.
type DoneEvent struct{}

func (e *DoneEvent) EventType() string {
	return "done"
}

// EventPipeline manages event streaming.
type EventPipeline struct {
	events chan Event
}

// NewEventPipeline creates a new event pipeline.
func NewEventPipeline(bufferSize int) *EventPipeline {
	return &EventPipeline{
		events: make(chan Event, bufferSize),
	}
}

// Send sends an event to the pipeline.
func (ep *EventPipeline) Send(event Event) {
	select {
	case ep.events <- event:
	default:
		// Pipeline full, drop event (should not happen in normal operation)
	}
}

// Receive receives an event from the pipeline.
func (ep *EventPipeline) Receive() <-chan Event {
	return ep.events
}

// Close closes the pipeline.
func (ep *EventPipeline) Close() {
	close(ep.events)
}
