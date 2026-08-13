package providers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/deagy/lana/internal/provider"
)

func TestOpenAICompatibleClient_Chat(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check auth header
		if r.Header.Get("Authorization") != "Bearer test-key" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Return a streaming response
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)

		// Send start event
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"role\":\"assistant\"}}]}\n\n"))
		flusher.Flush()

		// Send content chunks
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"Hello \"}}]}\n\n"))
		flusher.Flush()
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"world\"}}]}\n\n"))
		flusher.Flush()

		// Send finish
		w.Write([]byte("data: {\"choices\":[{\"finish_reason\":\"stop\"}]}\n\n"))
		flusher.Flush()
		w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	client := NewOpenAICompatibleClient(server.URL, "test-key", "gpt-4")

	req := &provider.Request{
		Messages: []provider.Message{
			{Role: "user", Content: "Hello"},
		},
		Model: "gpt-4",
	}

	ctx := context.Background()
	reader, err := client.Chat(ctx, req)
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}
	defer reader.Close()

	// Read events
	var events []provider.Event
	for {
		event, err := reader.NextEvent(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("NextEvent failed: %v", err)
		}
		events = append(events, event)
	}

	if len(events) < 3 {
		t.Errorf("expected at least 3 events, got %d", len(events))
	}

	// Check first event is MessageStart
	if start, ok := events[0].(*provider.MessageStartEvent); !ok || start.Role != "assistant" {
		t.Error("first event should be MessageStartEvent")
	}

	// Check middle events are MessageDelta
	var foundHello, foundWorld bool
	for _, e := range events {
		if delta, ok := e.(*provider.MessageDeltaEvent); ok {
			if delta.Content == "Hello " {
				foundHello = true
			}
			if delta.Content == "world" {
				foundWorld = true
			}
		}
	}
	if !foundHello || !foundWorld {
		t.Error("expected MessageDelta events with 'Hello ' and 'world'")
	}

	// Check last event is MessageEnd
	if len(events) > 0 {
		if end, ok := events[len(events)-1].(*provider.MessageEndEvent); !ok || end.StopReason != "stop" {
			t.Error("last event should be MessageEndEvent with stop reason")
		}
	}
}

func TestOpenAICompatibleClient_Name(t *testing.T) {
	client := NewOpenAICompatibleClient("https://api.openai.com/v1", "key", "gpt-4")
	if client.Name() != "openai-compat" {
		t.Errorf("expected name 'openai-compat', got %s", client.Name())
	}
}

func TestOpenAICompatibleClient_Model(t *testing.T) {
	client := NewOpenAICompatibleClient("https://api.openai.com/v1", "key", "gpt-4")
	if client.Model() != "gpt-4" {
		t.Errorf("expected model 'gpt-4', got %s", client.Model())
	}
}

func TestOpenAICompatibleClient_CustomHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Custom-Header") != "test-value" {
			http.Error(w, "Missing custom header", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"role\":\"assistant\"}}]}\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	client := NewOpenAICompatibleClient(server.URL, "key", "gpt-4")
	client.SetHeader("X-Custom-Header", "test-value")

	req := &provider.Request{
		Messages: []provider.Message{{Role: "user", Content: "test"}},
		Model:    "gpt-4",
	}

	_, err := client.Chat(context.Background(), req)
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}
}

func TestOpenAICompatibleClient_InvalidAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer valid-key" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	client := NewOpenAICompatibleClient(server.URL, "invalid-key", "gpt-4")

	req := &provider.Request{
		Messages: []provider.Message{{Role: "user", Content: "test"}},
		Model:    "gpt-4",
	}

	_, err := client.Chat(context.Background(), req)
	if err == nil {
		t.Error("expected error for invalid API key")
	}

	if !strings.Contains(err.Error(), "http 401") {
		t.Errorf("expected 401 error, got: %v", err)
	}
}

func TestOpenAICompatibleClient_ToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)

		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"role\":\"assistant\"}}]}\n\n"))
		flusher.Flush()

		// Tool call
		w.Write([]byte(`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_123","type":"function","function":{"name":"get_weather","arguments":"{\"location\":\"NYC\"}"}}]}}]}`))
		w.Write([]byte("\n\n"))
		flusher.Flush()

		w.Write([]byte("data: {\"choices\":[{\"finish_reason\":\"tool_calls\"}]}\n\n"))
		flusher.Flush()
		w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	client := NewOpenAICompatibleClient(server.URL, "key", "gpt-4")

	req := &provider.Request{
		Messages: []provider.Message{{Role: "user", Content: "What's the weather?"}},
		Model:    "gpt-4",
	}

	ctx := context.Background()
	reader, err := client.Chat(ctx, req)
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}
	defer reader.Close()

	var toolCalls []*provider.ToolCallEvent
	for {
		event, err := reader.NextEvent(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("NextEvent failed: %v", err)
		}
		if tc, ok := event.(*provider.ToolCallEvent); ok {
			toolCalls = append(toolCalls, tc)
		}
	}

	if len(toolCalls) == 0 {
		t.Error("expected at least one tool call")
	}

	if len(toolCalls) > 0 {
		if toolCalls[0].Name != "get_weather" {
			t.Errorf("expected tool name 'get_weather', got %s", toolCalls[0].Name)
		}
		if !strings.Contains(string(toolCalls[0].Input), "NYC") {
			t.Error("expected tool input to contain 'NYC'")
		}
	}
}

func TestOpenAICompatibleClient_EmptyAPIKey(t *testing.T) {
	// Some endpoints (like local) don't require auth
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); auth != "" && !strings.HasPrefix(auth, "Bearer") {
			http.Error(w, "Invalid auth", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"role\":\"assistant\"}}]}\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	client := NewOpenAICompatibleClient(server.URL, "", "gpt-4")

	req := &provider.Request{
		Messages: []provider.Message{{Role: "user", Content: "test"}},
		Model:    "gpt-4",
	}

	_, err := client.Chat(context.Background(), req)
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}
}

func TestOpenAICompatibleClient_EndpointNormalization(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			http.Error(w, "Wrong path", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	// Test with trailing slash
	client := NewOpenAICompatibleClient(server.URL+"/", "key", "gpt-4")

	req := &provider.Request{
		Messages: []provider.Message{{Role: "user", Content: "test"}},
		Model:    "gpt-4",
	}

	_, err := client.Chat(context.Background(), req)
	if err != nil {
		t.Fatalf("Chat with trailing slash failed: %v", err)
	}
}
