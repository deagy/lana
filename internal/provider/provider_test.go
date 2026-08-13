package provider

import (
	"context"
	"io"
	"testing"
)

func TestMockProvider(t *testing.T) {
	provider := NewMockProvider("mock", "mock-model-1")

	if provider.Name() != "mock" {
		t.Errorf("expected name 'mock', got %s", provider.Name())
	}

	if provider.Model() != "mock-model-1" {
		t.Errorf("expected model 'mock-model-1', got %s", provider.Model())
	}
}

func TestMockProviderWithEvents(t *testing.T) {
	provider := NewMockProvider("mock", "mock-model-1")

	events := []Event{
		&MessageStartEvent{Role: "assistant"},
		&MessageDeltaEvent{Content: "Hello "},
		&MessageDeltaEvent{Content: "world"},
		&MessageEndEvent{StopReason: "stop"},
	}

	provider.SetEvents(events)

	ctx := context.Background()
	reader, err := provider.Chat(ctx, &Request{
		Messages: []Message{{Role: "user", Content: "Hello"}},
		Model:    "mock-model-1",
	})
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}
	defer reader.Close()

	// Read all events
	var receivedEvents []Event
	for {
		event, err := reader.NextEvent(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("NextEvent failed: %v", err)
		}
		receivedEvents = append(receivedEvents, event)
	}

	if len(receivedEvents) != len(events) {
		t.Errorf("expected %d events, got %d", len(events), len(receivedEvents))
	}

	if start, ok := receivedEvents[0].(*MessageStartEvent); !ok || start.Role != "assistant" {
		t.Error("first event should be MessageStartEvent with role assistant")
	}

	if delta, ok := receivedEvents[1].(*MessageDeltaEvent); !ok || delta.Content != "Hello " {
		t.Error("second event should be MessageDeltaEvent with content 'Hello '")
	}
}

func TestMockProviderSupportedModels(t *testing.T) {
	provider := NewMockProvider("mock", "mock-model-1")

	models, err := provider.SupportedModels(context.Background())
	if err != nil {
		t.Fatalf("SupportedModels failed: %v", err)
	}

	if len(models) != 1 {
		t.Errorf("expected 1 model, got %d", len(models))
	}

	if models[0].ID != "mock-model-1" {
		t.Errorf("expected model ID 'mock-model-1', got %s", models[0].ID)
	}
}
