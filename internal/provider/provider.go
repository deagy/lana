package provider

import (
	"context"
	"encoding/json"
	"io"
)

// Client is the versioned interface for AI provider interactions.
type Client interface {
	// Chat initiates a streaming chat completion request.
	// The response should be read from the returned Reader until EOF.
	Chat(ctx context.Context, req *Request) (Reader, error)

	// Name returns the provider's name (e.g., "openai", "ollama").
	Name() string

	// Model returns the currently configured model identifier.
	Model() string

	// SupportedModels returns the list of available models for this provider.
	// Returns nil if discovery is not supported.
	SupportedModels(ctx context.Context) ([]ModelInfo, error)
}

// Request represents a chat completion request.
type Request struct {
	Messages    []Message `json:"messages"`
	Model       string    `json:"model"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature *float32  `json:"temperature,omitempty"`
	Tools       []Tool    `json:"tools,omitempty"`
	ToolChoice  *string   `json:"tool_choice,omitempty"`
}

// Message represents a single message in the conversation.
type Message struct {
	Role    string      `json:"role"` // "user", "assistant", "system"
	Content string      `json:"content"`
	ToolID  string      `json:"tool_call_id,omitempty"` // For tool results
	Name    string      `json:"name,omitempty"`         // For tool results
	ToolUse []ToolUse   `json:"tool_use,omitempty"`     // For assistant tool calls
}

// ToolUse represents a tool call made by the assistant.
type ToolUse struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// Tool represents a tool definition that the provider can call.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"` // JSON Schema
}

// Reader provides streaming events from a chat completion.
type Reader interface {
	io.Closer

	// NextEvent reads the next event. Returns nil, io.EOF at end of stream.
	// Returns nil, <error> if there was an error; should still call Close().
	NextEvent(ctx context.Context) (Event, error)
}

// Event represents a streaming event from the provider.
type Event interface {
	// Type returns the event type: "message.start", "message.delta", "tool.call", "message.end", "error"
	Type() string
}

// MessageStartEvent signals the start of a message.
type MessageStartEvent struct {
	Role string
}

func (e *MessageStartEvent) Type() string { return "message.start" }

// MessageDeltaEvent contains a chunk of message text.
type MessageDeltaEvent struct {
	Content string
}

func (e *MessageDeltaEvent) Type() string { return "message.delta" }

// ToolCallEvent contains a tool invocation.
type ToolCallEvent struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

func (e *ToolCallEvent) Type() string { return "tool.call" }

// MessageEndEvent signals the end of a message.
type MessageEndEvent struct {
	StopReason string // "stop", "tool_use", etc.
}

func (e *MessageEndEvent) Type() string { return "message.end" }

// ErrorEvent signals an error during streaming.
type ErrorEvent struct {
	Message string
	Code    string
	Err     error
}

func (e *ErrorEvent) Type() string { return "error" }

// ModelInfo describes an available model.
type ModelInfo struct {
	ID           string
	Name         string
	Organization string
	Description  string
	MaxTokens    int
	Capabilities []string // e.g., "chat", "tool_use", "vision"
}
