package session

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MemoryStore is an in-memory session store for development and testing.
type MemoryStore struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

// NewMemoryStore creates a new in-memory session store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		sessions: make(map[string]*Session),
	}
}

// Create implements Store.
func (ms *MemoryStore) Create(ctx context.Context, opts CreateOpts) (string, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	id := uuid.New().String()
	now := time.Now()

	session := &Session{
		ID:         id,
		CreatedAt:  now,
		UpdatedAt:  now,
		Model:      opts.Model,
		Provider:   opts.Provider,
		Title:      opts.Title,
		Workspace:  opts.Workspace,
		Transcript: []Message{},
		Metadata:   opts.Metadata,
	}

	if session.Title == "" {
		session.Title = "Untitled"
	}

	ms.sessions[id] = session
	return id, nil
}

// Get implements Store.
func (ms *MemoryStore) Get(ctx context.Context, id string) (*Session, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	session, ok := ms.sessions[id]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", id)
	}

	return session, nil
}

// List implements Store.
func (ms *MemoryStore) List(ctx context.Context) ([]SessionMetadata, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	var result []SessionMetadata
	for _, session := range ms.sessions {
		result = append(result, SessionMetadata{
			ID:        session.ID,
			Title:     session.Title,
			Provider:  session.Provider,
			Model:     session.Model,
			CreatedAt: session.CreatedAt,
			UpdatedAt: session.UpdatedAt,
			Messages:  len(session.Transcript),
		})
	}

	return result, nil
}

// AppendMessage implements Store.
func (ms *MemoryStore) AppendMessage(ctx context.Context, sessionID string, msg *Message) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	session, ok := ms.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}

	session.Transcript = append(session.Transcript, *msg)
	session.UpdatedAt = time.Now()

	return nil
}

// Save implements Store.
func (ms *MemoryStore) Save(ctx context.Context, sessionID string, state *Session) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if _, ok := ms.sessions[sessionID]; !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	state.UpdatedAt = time.Now()
	ms.sessions[sessionID] = state

	return nil
}

// Delete implements Store.
func (ms *MemoryStore) Delete(ctx context.Context, id string) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	delete(ms.sessions, id)
	return nil
}

// Close implements Store.
func (ms *MemoryStore) Close() error {
	return nil
}
