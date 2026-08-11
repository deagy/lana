// Package session implements durable, schema-versioned, append-only JSONL
// storage for conversations and agent turn events.
package session

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/deagy/lana/internal/provider"
)

const SchemaVersion = 1

const (
	KindCreated = "session.created"
	KindForked  = "session.forked"
)

type Record struct {
	SchemaVersion int             `json:"schema_version"`
	SessionID     string          `json:"session_id"`
	Sequence      int64           `json:"sequence"`
	At            time.Time       `json:"at"`
	Kind          string          `json:"kind"`
	Data          json.RawMessage `json:"data,omitempty"`
}

type Session struct {
	ID        string            `json:"id"`
	CreatedAt time.Time         `json:"created_at"`
	ParentID  string            `json:"parent_id,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Records   []Record          `json:"records"`
	Recovered bool              `json:"recovered,omitempty"`
}

type Summary struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	ParentID  string    `json:"parent_id,omitempty"`
	Records   int       `json:"records"`
	Recovered bool      `json:"recovered,omitempty"`
}

type Store struct {
	root        string
	mu          sync.Mutex
	now         func() time.Time
	syncFile    func(*os.File) error
	writeRecord func(io.Writer, Record) error
}

func NewStore(root string) (*Store, error) {
	if strings.TrimSpace(root) == "" {
		return nil, errors.New("session store root is required")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("create session store: %w", err)
	}
	return &Store{
		root:        root,
		now:         func() time.Time { return time.Now().UTC() },
		syncFile:    func(file *os.File) error { return file.Sync() },
		writeRecord: writeRecord,
	}, nil
}

func (s *Store) Root() string { return s.root }

func (s *Store) Create(ctx context.Context, id string, metadata map[string]string) (Session, error) {
	if err := ctx.Err(); err != nil {
		return Session{}, err
	}
	if err := validID(id); err != nil {
		return Session{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	created := s.timestamp()
	payload, err := json.Marshal(struct {
		Metadata map[string]string `json:"metadata,omitempty"`
	}{Metadata: provider.RedactMetadata(metadata)})
	if err != nil {
		return Session{}, fmt.Errorf("marshal session metadata: %w", err)
	}
	record := Record{SchemaVersion: SchemaVersion, SessionID: id, Sequence: 1, At: created, Kind: KindCreated, Data: payload}
	file, err := os.OpenFile(s.path(id), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return Session{}, fmt.Errorf("create session %q: %w", id, err)
	}
	defer file.Close()
	if err := s.write(file, record); err != nil {
		return Session{}, err
	}
	if err := s.sync(file); err != nil {
		return Session{}, fmt.Errorf("sync session %q: %w", id, err)
	}
	return Session{ID: id, CreatedAt: created, Metadata: provider.RedactMetadata(metadata), Records: []Record{record}}, nil
}

// Append serializes one payload as a new immutable line. The sequence number is
// assigned by the store, never trusted from the caller.
func (s *Store) Append(ctx context.Context, id, kind string, data any) (Record, error) {
	if err := ctx.Err(); err != nil {
		return Record{}, err
	}
	if err := validID(id); err != nil {
		return Record{}, err
	}
	if strings.TrimSpace(kind) == "" {
		return Record{}, errors.New("session record kind is required")
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return Record{}, fmt.Errorf("marshal session record data: %w", err)
	}
	payload = provider.RedactJSON(payload)
	s.mu.Lock()
	defer s.mu.Unlock()
	loaded, err := s.loadLocked(id)
	if err != nil {
		return Record{}, err
	}
	if err := ctx.Err(); err != nil {
		return Record{}, err
	}
	record := Record{SchemaVersion: SchemaVersion, SessionID: id, Sequence: int64(len(loaded.Records) + 1), At: s.timestamp(), Kind: kind, Data: payload}
	file, err := os.OpenFile(s.path(id), os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return Record{}, fmt.Errorf("open session %q for append: %w", id, err)
	}
	defer file.Close()
	if err := s.write(file, record); err != nil {
		return Record{}, err
	}
	if err := s.sync(file); err != nil {
		return Record{}, fmt.Errorf("sync session %q: %w", id, err)
	}
	return record, nil
}

func (s *Store) Load(ctx context.Context, id string) (Session, error) {
	if err := ctx.Err(); err != nil {
		return Session{}, err
	}
	if err := validID(id); err != nil {
		return Session{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked(id)
}

// Recover removes only an incomplete final JSONL line. A malformed line that
// ends in a newline is corruption and is rejected rather than discarded. It
// never edits valid records, preserving append-only history following a crash.
func (s *Store) Recover(ctx context.Context, id string) (Session, error) {
	if err := ctx.Err(); err != nil {
		return Session{}, err
	}
	if err := validID(id); err != nil {
		return Session{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	loaded, err := s.loadLocked(id)
	if err != nil {
		return Session{}, err
	}
	if !loaded.Recovered {
		return loaded, nil
	}
	file, err := os.OpenFile(s.path(id), os.O_RDWR, 0o600)
	if err != nil {
		return Session{}, fmt.Errorf("open session %q for recovery: %w", id, err)
	}
	defer file.Close()
	validBytes, err := encodedRecords(loaded.Records)
	if err != nil {
		return Session{}, err
	}
	if err := file.Truncate(0); err != nil {
		return Session{}, fmt.Errorf("truncate session %q: %w", id, err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return Session{}, err
	}
	if _, err := file.Write(validBytes); err != nil {
		return Session{}, fmt.Errorf("rewrite recovered session %q: %w", id, err)
	}
	if err := s.sync(file); err != nil {
		return Session{}, fmt.Errorf("sync recovered session %q: %w", id, err)
	}
	loaded.Recovered = false
	return loaded, nil
}

func (s *Store) List(ctx context.Context) ([]Summary, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	result := make([]Summary, 0, len(entries))
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".jsonl")
		if validID(id) != nil {
			continue
		}
		loaded, err := s.loadLocked(id)
		if err != nil {
			return nil, err
		}
		if len(loaded.Records) == 0 {
			continue
		}
		result = append(result, Summary{ID: id, CreatedAt: loaded.CreatedAt, UpdatedAt: loaded.Records[len(loaded.Records)-1].At, ParentID: loaded.ParentID, Records: len(loaded.Records), Recovered: loaded.Recovered})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.Before(result[j].CreatedAt) })
	return result, nil
}

// Fork creates an independent append-only child containing a copy of the
// parent's history and a provenance record. Future writes never affect parent.
func (s *Store) Fork(ctx context.Context, parentID, childID string) (Session, error) {
	if err := ctx.Err(); err != nil {
		return Session{}, err
	}
	if err := validID(parentID); err != nil {
		return Session{}, err
	}
	if err := validID(childID); err != nil {
		return Session{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	parent, err := s.loadLocked(parentID)
	if err != nil {
		return Session{}, err
	}
	if parent.Recovered {
		return Session{}, fmt.Errorf("cannot fork unrecovered session %q", parentID)
	}
	childMeta := cloneMetadata(parent.Metadata)
	childMeta["forked_from"] = parentID
	childMeta = provider.RedactMetadata(childMeta)
	created := s.timestamp()
	data, err := json.Marshal(struct {
		Metadata map[string]string `json:"metadata,omitempty"`
		ParentID string            `json:"parent_id"`
	}{provider.RedactMetadata(childMeta), parentID})
	if err != nil {
		return Session{}, err
	}
	records := []Record{{SchemaVersion: SchemaVersion, SessionID: childID, Sequence: 1, At: created, Kind: KindForked, Data: data}}
	for _, original := range parent.Records {
		copy := original
		copy.SessionID = childID
		copy.Sequence = int64(len(records) + 1)
		copy.Data = provider.RedactJSON(copy.Data)
		records = append(records, copy)
	}
	file, err := os.OpenFile(s.path(childID), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return Session{}, fmt.Errorf("create fork %q: %w", childID, err)
	}
	defer file.Close()
	for _, record := range records {
		if err := s.write(file, record); err != nil {
			return Session{}, err
		}
	}
	if err := s.sync(file); err != nil {
		return Session{}, fmt.Errorf("sync fork %q: %w", childID, err)
	}
	return Session{ID: childID, CreatedAt: created, ParentID: parentID, Metadata: childMeta, Records: records}, nil
}

func (s *Store) loadLocked(id string) (Session, error) {
	contents, err := os.ReadFile(s.path(id))
	if err != nil {
		return Session{}, fmt.Errorf("open session %q: %w", id, err)
	}
	var records []Record
	lines := bytes.Split(contents, []byte{'\n'})
	recovered := false
	for index, lineData := range lines {
		if len(lineData) == 0 && index == len(lines)-1 {
			continue
		}
		var record Record
		if err := json.Unmarshal(lineData, &record); err != nil {
			// A malformed final line is the expected signature of a torn append.
			if index == len(lines)-1 && len(contents) > 0 && contents[len(contents)-1] != '\n' {
				recovered = true
				break
			}
			return Session{}, fmt.Errorf("decode session %q line %d: %w", id, index+1, err)
		}
		if err := validateRecord(id, int64(len(records)+1), record); err != nil {
			return Session{}, fmt.Errorf("validate session %q line %d: %w", id, index+1, err)
		}
		records = append(records, record)
	}
	if len(records) == 0 {
		return Session{}, fmt.Errorf("session %q has no valid records", id)
	}
	if records[0].Kind != KindCreated && records[0].Kind != KindForked {
		return Session{}, fmt.Errorf("session %q has no creation record", id)
	}
	session := Session{ID: id, CreatedAt: records[0].At, Records: records, Recovered: recovered}
	var metadata struct {
		Metadata map[string]string `json:"metadata"`
		ParentID string            `json:"parent_id"`
	}
	if err := json.Unmarshal(records[0].Data, &metadata); err == nil {
		session.Metadata, session.ParentID = metadata.Metadata, metadata.ParentID
	}
	if session.Metadata == nil {
		session.Metadata = map[string]string{}
	}
	if session.ParentID == "" {
		session.ParentID = session.Metadata["forked_from"]
	}
	return session, nil
}

func (s *Store) path(id string) string { return filepath.Join(s.root, id+".jsonl") }

func validID(id string) error {
	if id == "" || len(id) > 128 {
		return errors.New("invalid session id")
	}
	for _, char := range id {
		if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '-' || char == '_') {
			return errors.New("invalid session id")
		}
	}
	return nil
}

func validateRecord(id string, expectedSequence int64, record Record) error {
	if record.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported schema version %d", record.SchemaVersion)
	}
	if record.SessionID != id {
		return errors.New("session id does not match file")
	}
	if record.Sequence != expectedSequence {
		return fmt.Errorf("expected sequence %d, got %d", expectedSequence, record.Sequence)
	}
	if record.Kind == "" || record.At.IsZero() {
		return errors.New("record kind and timestamp are required")
	}
	return nil
}

func writeRecord(writer io.Writer, record Record) error {
	encoded, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal session record: %w", err)
	}
	if _, err := writer.Write(append(encoded, '\n')); err != nil {
		return fmt.Errorf("write session record: %w", err)
	}
	return nil
}

func encodedRecords(records []Record) ([]byte, error) {
	var output strings.Builder
	for _, record := range records {
		encoded, err := json.Marshal(record)
		if err != nil {
			return nil, err
		}
		output.Write(encoded)
		output.WriteByte('\n')
	}
	return []byte(output.String()), nil
}

func cloneMetadata(metadata map[string]string) map[string]string {
	copy := make(map[string]string, len(metadata))
	for key, value := range metadata {
		copy[key] = value
	}
	return copy
}

func (s *Store) timestamp() time.Time {
	if s.now == nil {
		return time.Now().UTC()
	}
	return s.now().UTC()
}

func (s *Store) sync(file *os.File) error {
	if s.syncFile == nil {
		return file.Sync()
	}
	return s.syncFile(file)
}

func (s *Store) write(writer io.Writer, record Record) error {
	if s.writeRecord == nil {
		return writeRecord(writer, record)
	}
	return s.writeRecord(writer, record)
}
