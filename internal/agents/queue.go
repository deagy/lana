package agents

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

const (
	TaskSchemaVersion    = 1
	defaultLeaseDuration = 30 * time.Second
)

type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
	StatusBlocked   Status = "blocked"
)

func (s Status) terminal() bool {
	return s == StatusCompleted || s == StatusFailed || s == StatusCancelled || s == StatusBlocked
}

// Task is a local work item. Input and Result are JSON values, never command
// lines. Metadata is for small queryable values such as a session reference.
type Task struct {
	SchemaVersion   int               `json:"schema_version"`
	ID              string            `json:"id"`
	Role            string            `json:"role"`
	Input           json.RawMessage   `json:"input"`
	Metadata        map[string]string `json:"metadata,omitempty"`
	DependsOn       []string          `json:"depends_on,omitempty"`
	Status          Status            `json:"status"`
	Result          json.RawMessage   `json:"result,omitempty"`
	ResultMetadata  map[string]string `json:"result_metadata,omitempty"`
	Error           string            `json:"error,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
	StartedAt       time.Time         `json:"started_at,omitempty"`
	CompletedAt     time.Time         `json:"completed_at,omitempty"`
	CancelRequested bool              `json:"cancel_requested,omitempty"`
	LeaseOwner      string            `json:"lease_owner,omitempty"`
	LeaseExpiresAt  time.Time         `json:"lease_expires_at,omitempty"`
}

// TaskRequest contains caller-controlled registration data.
type TaskRequest struct {
	ID        string
	Role      string
	Input     json.RawMessage
	Metadata  map[string]string
	DependsOn []string
}

// Result is returned by an Executor and persisted on the associated Task.
type Result struct {
	Output   json.RawMessage
	Metadata map[string]string
}

// Executor is the only execution boundary. A Queue does not interpret task
// input and has no process, network, provider, or credential implementation.
// Implementations can bridge to Lana's turn/session runtime by receiving a
// task's structured input and recording their own session reference in Result.
type Executor interface {
	Execute(context.Context, Task) (Result, error)
}
type ExecutorFunc func(context.Context, Task) (Result, error)

func (f ExecutorFunc) Execute(ctx context.Context, task Task) (Result, error) { return f(ctx, task) }

// Queue coordinates dependency-aware task execution over a local Store.
type Queue struct {
	Registry Registry
	Store    Store
	Executor Executor
	Now      func() time.Time
	// LeaseDuration bounds how long another worker waits after this worker stops
	// renewing its claim. Zero uses a safe default.
	LeaseDuration time.Duration
	// WorkerID identifies a worker process in durable claims. Zero generates an
	// opaque per-Run value; it is exposed for deterministic integrations/tests.
	WorkerID string

	mu      sync.Mutex
	running map[string]context.CancelFunc
}

func (q *Queue) Register(ctx context.Context, request TaskRequest) (Task, error) {
	if q.Store == nil {
		return Task{}, errors.New("agent task store is required")
	}
	if !validID(request.ID) {
		return Task{}, fmt.Errorf("invalid agent task ID %q", request.ID)
	}
	if !q.Registry.Has(request.Role) {
		return Task{}, fmt.Errorf("%w: %s", ErrRoleNotFound, request.Role)
	}
	input := append(json.RawMessage(nil), request.Input...)
	if len(input) == 0 || !json.Valid(input) {
		return Task{}, errors.New("agent task input must be valid JSON")
	}
	if err := validateMetadata(request.Metadata); err != nil {
		return Task{}, err
	}
	seen := map[string]bool{}
	dependencies := append([]string(nil), request.DependsOn...)
	for _, id := range dependencies {
		if !validID(id) || id == request.ID {
			return Task{}, fmt.Errorf("invalid dependency %q", id)
		}
		if seen[id] {
			return Task{}, fmt.Errorf("duplicate dependency %q", id)
		}
		seen[id] = true
		if _, err := q.Store.Get(ctx, id); err != nil {
			return Task{}, fmt.Errorf("dependency %q: %w", id, err)
		}
	}
	task := Task{SchemaVersion: TaskSchemaVersion, ID: request.ID, Role: request.Role, Input: input, Metadata: cloneStrings(request.Metadata), DependsOn: dependencies, Status: StatusQueued, CreatedAt: q.now()}
	if err := q.Store.Create(ctx, task); err != nil {
		return Task{}, err
	}
	return cloneTask(task), nil
}

// Cancel atomically cancels a task that is not running, or records a durable
// cancellation request for a running task. A local executor is also signalled
// immediately when this Queue owns the corresponding lease.
func (q *Queue) Cancel(ctx context.Context, id string) (Task, error) {
	if q.Store == nil {
		return Task{}, errors.New("agent task store is required")
	}
	task, err := q.Store.Cancel(ctx, id, q.now())
	if err != nil {
		return Task{}, err
	}
	if task.Status == StatusRunning {
		q.mu.Lock()
		cancel := q.running[id]
		q.mu.Unlock()
		if cancel != nil {
			cancel()
		}
	}
	return task, nil
}

// Run executes queued work with at most maxConcurrency active executors in
// this process. Claims are lease-bound, so concurrent processes cannot execute
// the same queued task. A recovered, expired claim may be retried; executors
// must therefore make any external effect idempotent.
func (q *Queue) Run(ctx context.Context, maxConcurrency int) error {
	if q.Store == nil {
		return errors.New("agent task store is required")
	}
	if q.Executor == nil {
		return errors.New("agent task executor is required")
	}
	if maxConcurrency < 1 {
		return errors.New("agent task concurrency must be positive")
	}
	workerID, err := q.workerID()
	if err != nil {
		return err
	}
	lease := q.leaseDuration()
	q.mu.Lock()
	if q.running == nil {
		q.running = make(map[string]context.CancelFunc)
	}
	q.mu.Unlock()

	type completion struct {
		id     string
		result Result
		err    error
	}
	done := make(chan completion, maxConcurrency)
	running := 0
	for {
		if _, err := q.Store.Recover(ctx, q.now()); err != nil {
			return err
		}
		if err := q.markBlocked(ctx); err != nil {
			return err
		}
		tasks, err := q.Store.List(ctx)
		if err != nil {
			return err
		}
		byID := indexTasks(tasks)
		for running < maxConcurrency {
			next, ready := readyTask(tasks, byID)
			if !ready {
				break
			}
			claimed, ok, err := q.Store.Claim(ctx, next.ID, workerID, q.now(), lease)
			if err != nil {
				return err
			}
			if !ok {
				tasks, err = q.Store.List(ctx)
				if err != nil {
					return err
				}
				byID = indexTasks(tasks)
				continue
			}
			next = claimed
			taskCtx, cancel := context.WithCancel(ctx)
			q.mu.Lock()
			q.running[next.ID] = cancel
			q.mu.Unlock()
			// A cancellation can race with Claim before the local cancel function
			// was registered. Inspect once after registration; later calls see it.
			current, getErr := q.Store.Get(ctx, next.ID)
			if getErr != nil {
				cancel()
				return getErr
			}
			if current.CancelRequested {
				cancel()
			}
			running++
			go func(task Task) {
				result, runErr := q.execute(taskCtx, cancel, task, workerID, lease)
				done <- completion{id: task.ID, result: result, err: runErr}
			}(next)
			tasks, err = q.Store.List(ctx)
			if err != nil {
				return err
			}
			byID = indexTasks(tasks)
		}
		if running == 0 {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return nil
		}
		select {
		case completed := <-done:
			running--
			q.mu.Lock()
			delete(q.running, completed.id)
			q.mu.Unlock()
			if err := q.finish(ctx, completed.id, workerID, completed.result, completed.err); err != nil {
				return err
			}
		case <-ctx.Done():
			q.cancelRunning()
			// Executors must observe cancellation. Keep collecting them so their
			// terminal status is persisted before returning the context error.
			for running > 0 {
				completed := <-done
				running--
				q.mu.Lock()
				delete(q.running, completed.id)
				q.mu.Unlock()
				if err := q.finish(context.Background(), completed.id, workerID, completed.result, context.Canceled); err != nil {
					return err
				}
			}
			return ctx.Err()
		}
	}
}

func (q *Queue) finish(ctx context.Context, id, workerID string, result Result, executeErr error) error {
	if executeErr == nil && len(result.Output) > 0 && !json.Valid(result.Output) {
		executeErr = fmt.Errorf("agent task %q result must be valid JSON", id)
		result.Output = nil
	}
	if executeErr == nil {
		if err := validateMetadata(result.Metadata); err != nil {
			executeErr = err
			result.Metadata = nil
		}
	}
	failure := ""
	if executeErr != nil && !errors.Is(executeErr, context.Canceled) && !errors.Is(executeErr, context.DeadlineExceeded) {
		failure = executeErr.Error()
	}
	_, _, err := q.Store.Complete(ctx, id, workerID, result, failure, errors.Is(executeErr, context.Canceled) || errors.Is(executeErr, context.DeadlineExceeded), q.now())
	return err
}

func (q *Queue) markBlocked(ctx context.Context) error {
	tasks, err := q.Store.List(ctx)
	if err != nil {
		return err
	}
	byID := indexTasks(tasks)
	for _, task := range tasks {
		if task.Status != StatusQueued {
			continue
		}
		for _, dependency := range task.DependsOn {
			dep, exists := byID[dependency]
			if !exists || (dep.Status.terminal() && dep.Status != StatusCompleted) {
				if _, _, err := q.Store.Block(ctx, task.ID, fmt.Sprintf("dependency %q did not complete", dependency), q.now()); err != nil {
					return err
				}
				break
			}
		}
	}
	return nil
}

func readyTask(tasks []Task, byID map[string]Task) (Task, bool) {
	for _, task := range tasks {
		if task.Status != StatusQueued {
			continue
		}
		ready := true
		for _, dependency := range task.DependsOn {
			if byID[dependency].Status != StatusCompleted {
				ready = false
				break
			}
		}
		if ready {
			return task, true
		}
	}
	return Task{}, false
}

func indexTasks(tasks []Task) map[string]Task {
	indexed := make(map[string]Task, len(tasks))
	for _, task := range tasks {
		indexed[task.ID] = task
	}
	return indexed
}

func (q *Queue) execute(ctx context.Context, cancel context.CancelFunc, task Task, workerID string, lease time.Duration) (Result, error) {
	stopRenewal := make(chan struct{})
	doneRenewal := make(chan struct{})
	go func() {
		defer close(doneRenewal)
		ticker := time.NewTicker(renewalInterval(lease))
		defer ticker.Stop()
		for {
			select {
			case <-stopRenewal:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				renewed, owned, err := q.Store.Renew(context.Background(), task.ID, workerID, q.now(), lease)
				if err != nil || !owned || renewed.CancelRequested {
					cancel()
					return
				}
			}
		}
	}()
	result, err := q.Executor.Execute(ctx, cloneTask(task))
	close(stopRenewal)
	<-doneRenewal
	return result, err
}

func (q *Queue) cancelRunning() {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, cancel := range q.running {
		cancel()
	}
}

func (q *Queue) leaseDuration() time.Duration {
	if q.LeaseDuration > 0 {
		return q.LeaseDuration
	}
	return defaultLeaseDuration
}

func renewalInterval(lease time.Duration) time.Duration {
	interval := lease / 3
	if interval < 10*time.Millisecond {
		return 10 * time.Millisecond
	}
	return interval
}

func (q *Queue) workerID() (string, error) {
	if q.WorkerID != "" {
		return q.WorkerID, nil
	}
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate agent worker ID: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

func (q *Queue) now() time.Time {
	if q.Now != nil {
		return q.Now().UTC()
	}
	return time.Now().UTC()
}

func validateMetadata(metadata map[string]string) error {
	for key := range metadata {
		if key == "" || len(key) > 128 {
			return fmt.Errorf("invalid agent task metadata key %q", key)
		}
	}
	return nil
}

func cloneStrings(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func cloneTask(task Task) Task {
	task.Input = append(json.RawMessage(nil), task.Input...)
	task.Result = append(json.RawMessage(nil), task.Result...)
	task.Metadata = cloneStrings(task.Metadata)
	task.ResultMetadata = cloneStrings(task.ResultMetadata)
	task.DependsOn = append([]string(nil), task.DependsOn...)
	return task
}

// SortTasks is useful to presenters needing deterministic task output.
func SortTasks(tasks []Task) {
	sort.SliceStable(tasks, func(i, j int) bool { return tasks[i].CreatedAt.Before(tasks[j].CreatedAt) })
}
