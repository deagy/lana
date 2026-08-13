package providers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deagy/lana/internal/provider"
)

func TestOllamaClient_Chat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		// Send streaming responses as JSONL
		responses := []map[string]interface{}{
			{
				"model": "llama2",
				"message": map[string]string{
					"role":    "assistant",
					"content": "Hello ",
				},
				"done": false,
			},
			{
				"model": "llama2",
				"message": map[string]string{
					"role":    "assistant",
					"content": "world",
				},
				"done": false,
			},
			{
				"model":          "llama2",
				"done":           true,
				"total_duration": 1000000000,
				"eval_count":     10,
			},
		}

		flusher := w.(http.Flusher)
		for _, resp := range responses {
			data, _ := json.Marshal(resp)
			w.Write(data)
			w.Write([]byte("\n"))
			flusher.Flush()
		}
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL, "llama2")

	req := &provider.Request{
		Messages: []provider.Message{
			{Role: "user", Content: "Hello"},
		},
		Model: "llama2",
	}

	ctx := context.Background()
	reader, err := client.Chat(ctx, req)
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}
	defer reader.Close()

	// Read all events
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
		t.Error("first event should be MessageStartEvent with role assistant")
	}

	// Check for content
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
		t.Error("expected content 'Hello ' and 'world'")
	}

	// Check for end event
	if len(events) > 0 {
		if end, ok := events[len(events)-1].(*provider.MessageEndEvent); !ok || end.StopReason != "stop" {
			t.Error("last event should be MessageEndEvent")
		}
	}
}

func TestOllamaClient_Name(t *testing.T) {
	client := NewOllamaClient("http://localhost:11434", "llama2")
	if client.Name() != "ollama" {
		t.Errorf("expected name 'ollama', got %s", client.Name())
	}
}

func TestOllamaClient_Model(t *testing.T) {
	client := NewOllamaClient("http://localhost:11434", "llama2")
	if client.Model() != "llama2" {
		t.Errorf("expected model 'llama2', got %s", client.Model())
	}
}

func TestOllamaClient_SupportedModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"models": []map[string]string{
				{"name": "llama2"},
				{"name": "mistral"},
				{"name": "neural-chat"},
			},
		})
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL, "llama2")
	models, err := client.SupportedModels(context.Background())
	if err != nil {
		t.Fatalf("SupportedModels failed: %v", err)
	}

	if len(models) != 3 {
		t.Errorf("expected 3 models, got %d", len(models))
	}

	if models[0].ID != "llama2" {
		t.Errorf("expected first model 'llama2', got %s", models[0].ID)
	}
}

func TestOllamaClient_IsAvailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"models":[]}`))
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL, "llama2")
	available := client.IsAvailable(context.Background())
	if !available {
		t.Error("expected Ollama to be available")
	}
}

func TestOllamaClient_NotAvailable(t *testing.T) {
	// Use an unreachable address
	client := NewOllamaClient("http://localhost:9999", "llama2")
	available := client.IsAvailable(context.Background())
	if available {
		t.Error("expected Ollama to not be available")
	}
}

func TestOllamaClient_EndpointNormalization(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"model":"llama2","message":{"role":"assistant","content":"test"},"done":true}`))
		w.Write([]byte("\n"))
	}))
	defer server.Close()

	// Test with trailing slash
	client := NewOllamaClient(server.URL+"/", "llama2")

	req := &provider.Request{
		Messages: []provider.Message{{Role: "user", Content: "test"}},
		Model:    "llama2",
	}

	_, err := client.Chat(context.Background(), req)
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}
}
