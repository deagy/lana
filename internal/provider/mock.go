package provider

import (
	"context"
	"io"
	"sync"
)

// MockProvider is a deterministic provider for testing.
type MockProvider struct {
	name   string
	model  string
	events []Event
	mu     sync.Mutex
	closed bool
}

// NewMockProvider creates a new mock provider.
func NewMockProvider(name, model string) *MockProvider {
	return &MockProvider{
		name:  name,
		model: model,
	}
}

// Chat returns a mock reader with predefined events.
func (m *MockProvider) Chat(ctx context.Context, req *Request) (Reader, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return &mockReader{
		events: m.events,
		index:  0,
	}, nil
}

// Name implements Client.
func (m *MockProvider) Name() string {
	return m.name
}

// Model implements Client.
func (m *MockProvider) Model() string {
	return m.model
}

// SupportedModels implements Client.
func (m *MockProvider) SupportedModels(ctx context.Context) ([]ModelInfo, error) {
	return []ModelInfo{
		{
			ID:   m.model,
			Name: m.model,
		},
	}, nil
}

// SetEvents sets the events to be returned by Chat.
func (m *MockProvider) SetEvents(events []Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = events
}

type mockReader struct {
	events []Event
	index  int
	mu     sync.Mutex
}

// NextEvent implements Reader.
func (mr *mockReader) NextEvent(ctx context.Context) (Event, error) {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	if mr.index >= len(mr.events) {
		return nil, io.EOF
	}

	event := mr.events[mr.index]
	mr.index++
	return event, nil
}

// Close implements Reader.
func (mr *mockReader) Close() error {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	mr.index = len(mr.events)
	return nil
}
