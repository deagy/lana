package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/deagy/lana/internal/session"
	"github.com/google/uuid"
)

// FileStore persists sessions to disk as JSON files.
type FileStore struct {
	basePath string
	mu       sync.RWMutex
}

// NewFileStore creates a new file-based session store.
func NewFileStore(basePath string) (*FileStore, error) {
	// Create directory if it doesn't exist
	if err := os.MkdirAll(basePath, 0700); err != nil {
		return nil, fmt.Errorf("create directory: %w", err)
	}

	return &FileStore{
		basePath: basePath,
	}, nil
}

// Create implements session.Store.
func (fs *FileStore) Create(ctx context.Context, opts session.CreateOpts) (string, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	id := uuid.New().String()
	now := time.Now()

	sess := &session.Session{
		ID:         id,
		CreatedAt:  now,
		UpdatedAt:  now,
		Model:      opts.Model,
		Provider:   opts.Provider,
		Title:      opts.Title,
		Workspace:  opts.Workspace,
		Transcript: []session.Message{},
		Metadata:   opts.Metadata,
	}

	if sess.Title == "" {
		sess.Title = "Chat Session"
	}

	if err := fs.writeSession(sess); err != nil {
		return "", err
	}

	return id, nil
}

// Get implements session.Store.
func (fs *FileStore) Get(ctx context.Context, id string) (*session.Session, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	path := fs.sessionPath(id)
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("session not found: %s", id)
		}
		return nil, fmt.Errorf("open session: %w", err)
	}
	defer file.Close()

	var sess session.Session
	if err := json.NewDecoder(file).Decode(&sess); err != nil {
		return nil, fmt.Errorf("decode session: %w", err)
	}

	return &sess, nil
}

// List implements session.Store.
func (fs *FileStore) List(ctx context.Context) ([]session.SessionMetadata, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	entries, err := os.ReadDir(fs.basePath)
	if err != nil {
		return nil, fmt.Errorf("read directory: %w", err)
	}

	var result []session.SessionMetadata
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join(fs.basePath, entry.Name())
		file, err := os.Open(path)
		if err != nil {
			continue
		}

		var sess session.Session
		if err := json.NewDecoder(file).Decode(&sess); err != nil {
			file.Close()
			continue
		}
		file.Close()

		result = append(result, session.SessionMetadata{
			ID:        sess.ID,
			Title:     sess.Title,
			Provider:  sess.Provider,
			Model:     sess.Model,
			CreatedAt: sess.CreatedAt,
			UpdatedAt: sess.UpdatedAt,
			Messages:  len(sess.Transcript),
		})
	}

	// Sort by updated time, newest first
	sort.Slice(result, func(i, j int) bool {
		return result[i].UpdatedAt.After(result[j].UpdatedAt)
	})

	return result, nil
}

// AppendMessage implements session.Store.
func (fs *FileStore) AppendMessage(ctx context.Context, sessionID string, msg *session.Message) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	sess, err := fs.getSessionUnlocked(sessionID)
	if err != nil {
		return err
	}

	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}

	sess.Transcript = append(sess.Transcript, *msg)
	sess.UpdatedAt = time.Now()

	return fs.writeSession(sess)
}

// Save implements session.Store.
func (fs *FileStore) Save(ctx context.Context, sessionID string, state *session.Session) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if _, err := fs.getSessionUnlocked(sessionID); err != nil {
		return err
	}

	state.UpdatedAt = time.Now()
	return fs.writeSession(state)
}

// Delete implements session.Store.
func (fs *FileStore) Delete(ctx context.Context, id string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	path := fs.sessionPath(id)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete session: %w", err)
	}

	return nil
}

// Close implements session.Store.
func (fs *FileStore) Close() error {
	return nil
}

// Export exports a session as formatted text.
func (fs *FileStore) Export(ctx context.Context, sessionID string, format string) (string, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	sess, err := fs.getSessionUnlocked(sessionID)
	if err != nil {
		return "", err
	}

	switch format {
	case "markdown":
		return fs.exportMarkdown(sess), nil
	case "json":
		data, err := json.MarshalIndent(sess, "", "  ")
		return string(data), err
	default:
		return "", fmt.Errorf("unsupported format: %s", format)
	}
}

func (fs *FileStore) exportMarkdown(sess *session.Session) string {
	var buf strings.Builder

	fmt.Fprintf(&buf, "# %s\n\n", sess.Title)
	fmt.Fprintf(&buf, "**Provider:** %s  \n", sess.Provider)
	fmt.Fprintf(&buf, "**Model:** %s  \n", sess.Model)
	fmt.Fprintf(&buf, "**Created:** %s  \n", sess.CreatedAt.Format(time.RFC3339))
	fmt.Fprintf(&buf, "**Updated:** %s  \n\n", sess.UpdatedAt.Format(time.RFC3339))

	for _, msg := range sess.Transcript {
		if msg.Role == "user" {
			fmt.Fprintf(&buf, "**You:** %s\n\n", msg.Content)
		} else {
			fmt.Fprintf(&buf, "**%s:** %s\n\n", titleCase(msg.Role), msg.Content)
		}
	}

	return buf.String()
}

func (fs *FileStore) sessionPath(id string) string {
	return filepath.Join(fs.basePath, id+".json")
}

func (fs *FileStore) writeSession(sess *session.Session) error {
	path := fs.sessionPath(sess.ID)
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer file.Close()

	if err := json.NewEncoder(file).Encode(sess); err != nil {
		return fmt.Errorf("encode session: %w", err)
	}

	return nil
}

func (fs *FileStore) getSessionUnlocked(id string) (*session.Session, error) {
	path := fs.sessionPath(id)
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("session not found: %s", id)
		}
		return nil, fmt.Errorf("open session: %w", err)
	}
	defer file.Close()

	var sess session.Session
	if err := json.NewDecoder(file).Decode(&sess); err != nil {
		return nil, fmt.Errorf("decode session: %w", err)
	}

	return &sess, nil
}

func titleCase(s string) string {
	if len(s) == 0 {
		return s
	}
	return string(s[0]-32) + s[1:]
}
