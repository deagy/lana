package agents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
	"time"
)

const taskStoreVersion = 1

// Store is the persistence boundary for local task metadata. Its mutating
// methods are conditional: a worker can only renew or finish the lease it
// claimed. Implementations must return copies so callers cannot mutate stored
// task state.
type Store interface {
	Create(context.Context, Task) error
	Get(context.Context, string) (Task, error)
	List(context.Context) ([]Task, error)
	Cancel(context.Context, string, time.Time) (Task, error)
	Claim(context.Context, string, string, time.Time, time.Duration) (Task, bool, error)
	Renew(context.Context, string, string, time.Time, time.Duration) (Task, bool, error)
	Complete(context.Context, string, string, Result, string, bool, time.Time) (Task, bool, error)
	Block(context.Context, string, string, time.Time) (Task, bool, error)
	Recover(context.Context, time.Time) ([]Task, error)
}

// FileStore stores task metadata in one local JSON document. All operations
// acquire an advisory lock file in addition to the in-process mutex, so
// independently started lana processes cannot lose updates or double-claim a
// task. The operating system releases an advisory lock if its process dies.
type FileStore struct {
	path     string
	lockPath string
	mu       sync.Mutex
}

type fileState struct {
	Version int    `json:"version"`
	Tasks   []Task `json:"tasks"`
}

func NewFileStore(path string) (*FileStore, error) {
	if path == "" {
		return nil, errors.New("agent task store path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create agent task store directory: %w", err)
	}
	return &FileStore{path: path, lockPath: path + ".lock"}, nil
}

func (s *FileStore) Create(ctx context.Context, task Task) error {
	return s.withState(ctx, func(state *fileState) error {
		for _, existing := range state.Tasks {
			if existing.ID == task.ID {
				return fmt.Errorf("agent task %q already exists", task.ID)
			}
		}
		state.Tasks = append(state.Tasks, cloneTask(task))
		return nil
	})
}

func (s *FileStore) Get(ctx context.Context, id string) (Task, error) {
	var found Task
	err := s.withRead(ctx, func(state fileState) error {
		for _, task := range state.Tasks {
			if task.ID == id {
				found = cloneTask(task)
				return nil
			}
		}
		return fmt.Errorf("agent task %q not found", id)
	})
	return found, err
}

func (s *FileStore) List(ctx context.Context) ([]Task, error) {
	var tasks []Task
	err := s.withRead(ctx, func(state fileState) error {
		tasks = make([]Task, len(state.Tasks))
		for i, task := range state.Tasks {
			tasks[i] = cloneTask(task)
		}
		sort.SliceStable(tasks, func(i, j int) bool { return tasks[i].CreatedAt.Before(tasks[j].CreatedAt) })
		return nil
	})
	return tasks, err
}

// Cancel atomically either cancels a task that has not started or records a
// durable cancellation request for its lease owner. The owner observes that
// request through Renew and Complete always honours it.
func (s *FileStore) Cancel(ctx context.Context, id string, now time.Time) (Task, error) {
	var changed Task
	err := s.withState(ctx, func(state *fileState) error {
		for i := range state.Tasks {
			task := &state.Tasks[i]
			if task.ID != id {
				continue
			}
			switch task.Status {
			case StatusQueued, StatusBlocked:
				task.Status, task.Error, task.CompletedAt = StatusCancelled, "cancelled", now
			case StatusRunning:
				task.CancelRequested = true
			default:
				return fmt.Errorf("agent task %q cannot be cancelled from status %s", id, task.Status)
			}
			changed = cloneTask(*task)
			return nil
		}
		return fmt.Errorf("agent task %q not found", id)
	})
	return changed, err
}

// Claim performs the only queued-to-running transition. It succeeds for one
// caller only and binds the task to owner until its renewable lease expires.
func (s *FileStore) Claim(ctx context.Context, id, owner string, now time.Time, lease time.Duration) (Task, bool, error) {
	if owner == "" || lease <= 0 {
		return Task{}, false, errors.New("agent task claim owner and lease are required")
	}
	var claimed Task
	ok := false
	err := s.withState(ctx, func(state *fileState) error {
		for i := range state.Tasks {
			task := &state.Tasks[i]
			if task.ID != id {
				continue
			}
			if task.Status != StatusQueued {
				return nil
			}
			task.Status, task.StartedAt, task.Error = StatusRunning, now, ""
			task.CancelRequested = false
			task.LeaseOwner, task.LeaseExpiresAt = owner, now.Add(lease)
			claimed, ok = cloneTask(*task), true
			return nil
		}
		return fmt.Errorf("agent task %q not found", id)
	})
	return claimed, ok, err
}

// Renew extends an active claim. It returns false when a different process has
// recovered or claimed the task, or the caller renewed after its lease expired.
func (s *FileStore) Renew(ctx context.Context, id, owner string, now time.Time, lease time.Duration) (Task, bool, error) {
	if owner == "" || lease <= 0 {
		return Task{}, false, errors.New("agent task claim owner and lease are required")
	}
	var renewed Task
	ok := false
	err := s.withState(ctx, func(state *fileState) error {
		for i := range state.Tasks {
			task := &state.Tasks[i]
			if task.ID != id {
				continue
			}
			if task.Status != StatusRunning || task.LeaseOwner != owner || !task.LeaseExpiresAt.After(now) {
				return nil
			}
			task.LeaseExpiresAt = now.Add(lease)
			renewed, ok = cloneTask(*task), true
			return nil
		}
		return fmt.Errorf("agent task %q not found", id)
	})
	return renewed, ok, err
}

// Complete conditionally persists an executor outcome. A cancellation request
// always wins, even if the executor returned success after it was requested.
func (s *FileStore) Complete(ctx context.Context, id, owner string, result Result, failure string, cancelled bool, now time.Time) (Task, bool, error) {
	var completed Task
	ok := false
	err := s.withState(ctx, func(state *fileState) error {
		for i := range state.Tasks {
			task := &state.Tasks[i]
			if task.ID != id {
				continue
			}
			if task.Status != StatusRunning || task.LeaseOwner != owner {
				return nil
			}
			task.Result = append(json.RawMessage(nil), result.Output...)
			task.ResultMetadata = cloneStrings(result.Metadata)
			task.CompletedAt = now
			task.LeaseOwner, task.LeaseExpiresAt = "", time.Time{}
			switch {
			case task.CancelRequested || cancelled:
				task.Status, task.Error = StatusCancelled, "cancelled"
			case failure != "":
				task.Status, task.Error = StatusFailed, failure
			default:
				task.Status, task.Error = StatusCompleted, ""
			}
			completed, ok = cloneTask(*task), true
			return nil
		}
		return fmt.Errorf("agent task %q not found", id)
	})
	return completed, ok, err
}

// Block marks a queued task blocked only if it has not been claimed or
// cancelled since the caller inspected its dependencies.
func (s *FileStore) Block(ctx context.Context, id, reason string, now time.Time) (Task, bool, error) {
	var blocked Task
	ok := false
	err := s.withState(ctx, func(state *fileState) error {
		for i := range state.Tasks {
			task := &state.Tasks[i]
			if task.ID != id {
				continue
			}
			if task.Status != StatusQueued {
				return nil
			}
			task.Status, task.Error, task.CompletedAt = StatusBlocked, reason, now
			blocked, ok = cloneTask(*task), true
			return nil
		}
		return fmt.Errorf("agent task %q not found", id)
	})
	return blocked, ok, err
}

// Recover resolves state left by a stopped worker. An expired claim returns to
// queued state for another worker; an expired cancellation request becomes a
// terminal cancellation. Pre-lease running records are treated as expired.
func (s *FileStore) Recover(ctx context.Context, now time.Time) ([]Task, error) {
	var recovered []Task
	err := s.withState(ctx, func(state *fileState) error {
		for i := range state.Tasks {
			task := &state.Tasks[i]
			if task.Status != StatusRunning || (!task.LeaseExpiresAt.IsZero() && task.LeaseExpiresAt.After(now)) {
				continue
			}
			task.LeaseOwner, task.LeaseExpiresAt = "", time.Time{}
			if task.CancelRequested {
				task.Status, task.Error, task.CompletedAt = StatusCancelled, "cancelled", now
			} else {
				task.Status, task.Error = StatusQueued, ""
			}
			recovered = append(recovered, cloneTask(*task))
		}
		return nil
	})
	return recovered, err
}

func (s *FileStore) withRead(ctx context.Context, use func(fileState) error) error {
	return s.withLocked(ctx, func() error {
		state, err := s.readLocked()
		if err != nil {
			return err
		}
		return use(state)
	})
}

func (s *FileStore) withState(ctx context.Context, change func(*fileState) error) error {
	return s.withLocked(ctx, func() error {
		state, err := s.readLocked()
		if err != nil {
			return err
		}
		if err := change(&state); err != nil {
			return err
		}
		return s.writeLocked(state)
	})
}

func (s *FileStore) withLocked(ctx context.Context, use func() error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	lock, err := s.lock(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
		_ = lock.Close()
	}()
	return use()
}

func (s *FileStore) lock(ctx context.Context) (*os.File, error) {
	lock, err := os.OpenFile(s.lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open agent task store lock: %w", err)
	}
	for {
		err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return lock, nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) {
			_ = lock.Close()
			return nil, fmt.Errorf("lock agent task store: %w", err)
		}
		select {
		case <-ctx.Done():
			_ = lock.Close()
			return nil, ctx.Err()
		case <-time.After(5 * time.Millisecond):
		}
	}
}

func (s *FileStore) readLocked() (fileState, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return fileState{Version: taskStoreVersion, Tasks: []Task{}}, nil
	}
	if err != nil {
		return fileState{}, fmt.Errorf("read agent task store: %w", err)
	}
	var state fileState
	if err := json.Unmarshal(data, &state); err != nil {
		return fileState{}, fmt.Errorf("parse agent task store: %w", err)
	}
	if state.Version != taskStoreVersion {
		return fileState{}, fmt.Errorf("unsupported agent task store version %d", state.Version)
	}
	if state.Tasks == nil {
		state.Tasks = []Task{}
	}
	return state, nil
}

func (s *FileStore) writeLocked(state fileState) error {
	state.Version = taskStoreVersion
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal agent task store: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(s.path), ".agents-*.tmp")
	if err != nil {
		return fmt.Errorf("create agent task store temp file: %w", err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("set agent task store permissions: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write agent task store: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync agent task store: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close agent task store: %w", err)
	}
	if err := os.Rename(temporaryName, s.path); err != nil {
		return fmt.Errorf("replace agent task store: %w", err)
	}
	directory, err := os.Open(filepath.Dir(s.path))
	if err != nil {
		return fmt.Errorf("open agent task store directory: %w", err)
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return fmt.Errorf("sync agent task store directory: %w", err)
	}
	return nil
}
