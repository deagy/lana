package session

import (
	"context"
	"encoding/json"
	"time"
)

// Store manages persistent session storage.
type Store interface {
	// Create creates a new session and returns its ID.
	Create(ctx context.Context, opts CreateOpts) (string, error)

	// Get retrieves a session by ID.
	Get(ctx context.Context, id string) (*Session, error)

	// List returns all session IDs.
	List(ctx context.Context) ([]SessionMetadata, error)

	// AppendMessage appends a message to the session's transcript.
	AppendMessage(ctx context.Context, sessionID string, msg *Message) error

	// Save persists the session state.
	Save(ctx context.Context, sessionID string, state *Session) error

	// Delete removes a session.
	Delete(ctx context.Context, id string) error

	// Close closes the store.
	Close() error
}

// Session represents an active or archived conversation.
type Session struct {
	ID           string    `json:"id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Model        string    `json:"model"`
	Provider     string    `json:"provider"`
	Title        string    `json:"title,omitempty"`
	Workspace    string    `json:"workspace,omitempty"`
	Transcript   []Message `json:"transcript"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// SessionMetadata is metadata for a session without the full transcript.
type SessionMetadata struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Provider  string    `json:"provider"`
	Model     string    `json:"model"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Messages  int       `json:"message_count"`
}

// Message represents a single message in the session transcript.
type Message struct {
	Role      string          `json:"role"` // "user", "assistant", "system"
	Content   string          `json:"content"`
	Timestamp time.Time       `json:"timestamp"`
	ToolCalls []ToolCall      `json:"tool_calls,omitempty"`
	Metadata  json.RawMessage `json:"metadata,omitempty"`
}

// ToolCall represents a tool invocation in the transcript.
type ToolCall struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Input    json.RawMessage `json:"input"`
	Status   string          `json:"status"` // "pending", "approved", "denied", "completed", "failed"
	Result   string          `json:"result,omitempty"`
	Error    string          `json:"error,omitempty"`
	DeniedBy string          `json:"denied_by,omitempty"`
}

// CreateOpts contains options for creating a new session.
type CreateOpts struct {
	Model     string
	Provider  string
	Title     string
	Workspace string
	Metadata  map[string]string
}
