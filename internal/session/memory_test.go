package session

import (
	"context"
	"testing"
	"time"
)

func TestMemoryStoreCreate(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	ctx := context.Background()
	id, err := store.Create(ctx, CreateOpts{
		Model:    "gpt-4",
		Provider: "openai",
		Title:    "Test Session",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if id == "" {
		t.Error("expected non-empty session ID")
	}

	// Verify the session exists
	session, err := store.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if session.Title != "Test Session" {
		t.Errorf("expected title 'Test Session', got %s", session.Title)
	}

	if session.Model != "gpt-4" {
		t.Errorf("expected model 'gpt-4', got %s", session.Model)
	}

	if session.Provider != "openai" {
		t.Errorf("expected provider 'openai', got %s", session.Provider)
	}
}

func TestMemoryStoreAppendMessage(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	ctx := context.Background()
	sessionID, _ := store.Create(ctx, CreateOpts{
		Model:    "gpt-4",
		Provider: "openai",
	})

	msg := &Message{
		Role:    "user",
		Content: "Hello",
	}

	if err := store.AppendMessage(ctx, sessionID, msg); err != nil {
		t.Fatalf("AppendMessage failed: %v", err)
	}

	session, err := store.Get(ctx, sessionID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if len(session.Transcript) != 1 {
		t.Errorf("expected 1 message, got %d", len(session.Transcript))
	}

	if session.Transcript[0].Role != "user" {
		t.Errorf("expected role 'user', got %s", session.Transcript[0].Role)
	}

	if session.Transcript[0].Content != "Hello" {
		t.Errorf("expected content 'Hello', got %s", session.Transcript[0].Content)
	}

	if session.Transcript[0].Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}

func TestMemoryStoreList(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	ctx := context.Background()

	// Create multiple sessions
	_, _ = store.Create(ctx, CreateOpts{
		Model:    "gpt-4",
		Provider: "openai",
		Title:    "Session 1",
	})

	_, _ = store.Create(ctx, CreateOpts{
		Model:    "gpt-3.5",
		Provider: "openai",
		Title:    "Session 2",
	})

	sessions, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(sessions))
	}

	titles := make(map[string]bool)
	for _, s := range sessions {
		titles[s.Title] = true
	}

	if !titles["Session 1"] || !titles["Session 2"] {
		t.Error("expected both sessions in list")
	}
}

func TestMemoryStoreDelete(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	ctx := context.Background()
	id, _ := store.Create(ctx, CreateOpts{
		Model:    "gpt-4",
		Provider: "openai",
	})

	if err := store.Delete(ctx, id); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err := store.Get(ctx, id)
	if err == nil {
		t.Error("expected Get to fail after delete")
	}
}

func TestMemoryStoreUpdatesTimestamp(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	ctx := context.Background()
	id, _ := store.Create(ctx, CreateOpts{
		Model:    "gpt-4",
		Provider: "openai",
	})

	session, _ := store.Get(ctx, id)
	originalUpdated := session.UpdatedAt

	// Add a message
	time.Sleep(10 * time.Millisecond) // Ensure time difference
	msg := &Message{Role: "user", Content: "Hello"}
	store.AppendMessage(ctx, id, msg)

	session, _ = store.Get(ctx, id)
	if !session.UpdatedAt.After(originalUpdated) {
		t.Error("expected UpdatedAt to increase after appending message")
	}
}
