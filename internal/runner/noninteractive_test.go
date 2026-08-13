package runner

import (
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/deagy/lana/internal/approval"
	"github.com/deagy/lana/internal/output"
	"github.com/deagy/lana/internal/provider"
	"github.com/deagy/lana/internal/session"
	"github.com/deagy/lana/internal/storage"
	"github.com/deagy/lana/internal/tools"
)

// mockReader implements provider.Reader for testing.
type mockReader struct {
	events []provider.Event
	idx    int
	closed bool
}

func (m *mockReader) NextEvent(ctx context.Context) (provider.Event, error) {
	if m.idx >= len(m.events) {
		return nil, io.EOF
	}
	e := m.events[m.idx]
	m.idx++
	return e, nil
}

func (m *mockReader) Close() error {
	m.closed = true
	return nil
}

// mockClient implements provider.Client for testing.
type mockClient struct {
	events []provider.Event
}

func (m *mockClient) Chat(ctx context.Context, req *provider.Request) (provider.Reader, error) {
	return &mockReader{events: m.events}, nil
}

func (m *mockClient) Name() string                                             { return "mock" }
func (m *mockClient) Model() string                                            { return "test" }
func (m *mockClient) SupportedModels(ctx context.Context) ([]provider.ModelInfo, error) {
	return nil, nil
}

func TestNonInteractiveRunnerSimpleMessage(t *testing.T) {
	// Create test session store
	store, _ := storage.NewFileStore(t.TempDir())
	defer store.Close()

	ctx := context.Background()
	sessionID, _ := store.Create(ctx, session.CreateOpts{
		Model:    "test",
		Provider: "mock",
	})

	// Create mock provider with simple response
	events := []provider.Event{
		&provider.MessageStartEvent{Role: "assistant"},
		&provider.MessageDeltaEvent{Content: "Hello"},
		&provider.MessageDeltaEvent{Content: " world"},
		&provider.MessageEndEvent{StopReason: "stop"},
	}
	client := &mockClient{events: events}

	// Create registry (empty for this test)
	registry := tools.NewRegistry()

	// Create formatter
	formatter := output.NewFormatter("plain")

	// Create policy (auto-approve all)
	policy := approval.NewStaticPolicy(approval.FullAutoMode)

	// Create runner
	runner := NewNonInteractiveRunner(
		sessionID,
		store,
		client,
		registry,
		policy,
		formatter,
		10,
	)

	// Run
	err := runner.Run(ctx, "Hello there")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify session was saved
	sess, _ := store.Get(ctx, sessionID)
	if len(sess.Transcript) < 2 {
		t.Errorf("transcript should have at least 2 messages, got %d", len(sess.Transcript))
	}

	// Check user message
	if sess.Transcript[0].Role != "user" {
		t.Errorf("first message should be user, got %s", sess.Transcript[0].Role)
	}

	// Check assistant message
	if sess.Transcript[1].Role != "assistant" {
		t.Errorf("second message should be assistant, got %s", sess.Transcript[1].Role)
	}

	if sess.Transcript[1].Content != "Hello world" {
		t.Errorf("assistant content mismatch: got %q", sess.Transcript[1].Content)
	}
}

func TestNonInteractiveRunnerWithToolCall(t *testing.T) {
	// Create test session store
	store, _ := storage.NewFileStore(t.TempDir())
	defer store.Close()

	ctx := context.Background()
	sessionID, _ := store.Create(ctx, session.CreateOpts{
		Model:    "test",
		Provider: "mock",
	})

	// Create mock provider with tool call
	toolInput, _ := json.Marshal(map[string]string{"path": "file.txt"})
	events := []provider.Event{
		&provider.MessageStartEvent{Role: "assistant"},
		&provider.MessageDeltaEvent{Content: "I'll read"},
		&provider.ToolCallEvent{
			ID:    "call_123",
			Name:  "read_file",
			Input: toolInput,
		},
		&provider.MessageEndEvent{StopReason: "tool_use"},
	}
	client := &mockClient{events: events}

	// Create empty registry (tool will fail but that's OK for this test)
	registry := tools.NewRegistry()

	// Create formatter
	formatter := output.NewFormatter("plain")

	// Create policy
	policy := approval.NewStaticPolicy(approval.FullAutoMode)

	// Create runner
	runner := NewNonInteractiveRunner(
		sessionID,
		store,
		client,
		registry,
		policy,
		formatter,
		10,
	)

	// Run (will fail on tool execution, but that's expected)
	runner.Run(ctx, "Read file.txt")

	// Verify transcript was saved
	sess, _ := store.Get(ctx, sessionID)
	if len(sess.Transcript) < 1 {
		t.Fatalf("transcript should have messages, got %d", len(sess.Transcript))
	}
}

func TestToProviderMessages(t *testing.T) {
	msgs := []session.Message{
		{
			Role:      "user",
			Content:   "Hello",
			Timestamp: time.Now(),
		},
		{
			Role:      "assistant",
			Content:   "Hi there",
			Timestamp: time.Now(),
		},
	}

	result := toProviderMessages(msgs)

	if len(result) != 2 {
		t.Errorf("expected 2 messages, got %d", len(result))
	}

	if result[0].Role != "user" {
		t.Errorf("first role should be user, got %s", result[0].Role)
	}

	if result[0].Content != "Hello" {
		t.Errorf("first content mismatch: got %q", result[0].Content)
	}
}
