package agents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func newTestQueue(t *testing.T, executor Executor) *Queue {
	t.Helper()
	store, err := NewFileStore(t.TempDir() + "/tasks.json")
	if err != nil {
		t.Fatal(err)
	}
	return &Queue{Registry: DefaultRegistry(), Store: store, Executor: executor}
}

func registerTestTask(t *testing.T, queue *Queue, id string, dependencies ...string) Task {
	t.Helper()
	task, err := queue.Register(context.Background(), TaskRequest{ID: id, Role: "planner", Input: json.RawMessage(`{"objective":"test"}`), DependsOn: dependencies, Metadata: map[string]string{"session_id": "local-session"}})
	if err != nil {
		t.Fatalf("register %s: %v", id, err)
	}
	return task
}

func TestRegistryListsAndDescribesLocalRoles(t *testing.T) {
	registry := DefaultRegistry()
	roles := registry.List()
	if len(roles) != 3 || roles[0].ID != "implementer" || roles[1].ID != "planner" || roles[2].ID != "reviewer" {
		t.Fatalf("roles = %#v", roles)
	}
	role, ok := registry.Describe("planner")
	if !ok || role.Name != "Planner" {
		t.Fatalf("planner = %#v, %t", role, ok)
	}
}

func TestQueueHonorsDependenciesAndConcurrencyBound(t *testing.T) {
	var active, maximum atomic.Int32
	startedFirst := make(chan struct{})
	releaseFirst := make(chan struct{})
	queue := newTestQueue(t, ExecutorFunc(func(_ context.Context, task Task) (Result, error) {
		current := active.Add(1)
		for {
			seen := maximum.Load()
			if current <= seen || maximum.CompareAndSwap(seen, current) {
				break
			}
		}
		defer active.Add(-1)
		if task.ID == "first" {
			close(startedFirst)
			<-releaseFirst
		}
		return Result{Output: json.RawMessage(`{"ok":true}`), Metadata: map[string]string{"session_id": task.ID}}, nil
	}))
	registerTestTask(t, queue, "first")
	registerTestTask(t, queue, "second")
	registerTestTask(t, queue, "dependent", "first")

	done := make(chan error, 1)
	go func() { done <- queue.Run(context.Background(), 2) }()
	<-startedFirst
	// The independent second task may run with first, but the dependent cannot
	// start until first is released. A concurrency maximum above two would be a
	// direct violation of the queue bound.
	if got := maximum.Load(); got > 2 {
		t.Fatalf("maximum active tasks = %d", got)
	}
	close(releaseFirst)
	if err := <-done; err != nil {
		t.Fatalf("run: %v", err)
	}
	for _, id := range []string{"first", "second", "dependent"} {
		task, err := queue.Store.Get(context.Background(), id)
		if err != nil || task.Status != StatusCompleted {
			t.Fatalf("%s status = %s, %v", id, task.Status, err)
		}
	}
	if got := maximum.Load(); got > 2 {
		t.Fatalf("maximum active tasks = %d", got)
	}
}

func TestQueueBlocksDependentTaskAfterFailedDependency(t *testing.T) {
	queue := newTestQueue(t, ExecutorFunc(func(_ context.Context, task Task) (Result, error) {
		if task.ID == "fails" {
			return Result{}, errors.New("expected failure")
		}
		return Result{}, nil
	}))
	registerTestTask(t, queue, "fails")
	registerTestTask(t, queue, "blocked", "fails")
	if err := queue.Run(context.Background(), 1); err != nil {
		t.Fatalf("run: %v", err)
	}
	failed, _ := queue.Store.Get(context.Background(), "fails")
	blocked, _ := queue.Store.Get(context.Background(), "blocked")
	if failed.Status != StatusFailed || blocked.Status != StatusBlocked {
		t.Fatalf("statuses = %s, %s", failed.Status, blocked.Status)
	}
}

func TestQueueCancellationSignalsRunningExecutor(t *testing.T) {
	started := make(chan struct{})
	queue := newTestQueue(t, ExecutorFunc(func(ctx context.Context, _ Task) (Result, error) {
		close(started)
		<-ctx.Done()
		return Result{}, ctx.Err()
	}))
	registerTestTask(t, queue, "cancel-me")
	done := make(chan error, 1)
	go func() { done <- queue.Run(context.Background(), 1) }()
	<-started
	if _, err := queue.Cancel(context.Background(), "cancel-me"); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("queue did not finish after cancellation")
	}
	task, err := queue.Store.Get(context.Background(), "cancel-me")
	if err != nil || task.Status != StatusCancelled {
		t.Fatalf("task = %#v, %v", task, err)
	}
}

func TestRegisterRejectsUnknownRoleAndMissingDependency(t *testing.T) {
	queue := newTestQueue(t, nil)
	if _, err := queue.Register(context.Background(), TaskRequest{ID: "unknown", Role: "missing", Input: json.RawMessage(`{}`)}); !errors.Is(err, ErrRoleNotFound) {
		t.Fatalf("unknown role error = %v", err)
	}
	if _, err := queue.Register(context.Background(), TaskRequest{ID: "missing-dependency", Role: "planner", Input: json.RawMessage(`{}`), DependsOn: []string{"absent"}}); err == nil {
		t.Fatal("missing dependency was accepted")
	}
}

func TestQueueConcurrentWorkersClaimTaskOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.json")
	storeOne, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	storeTwo, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	release := make(chan struct{})
	var executions atomic.Int32
	executor := ExecutorFunc(func(_ context.Context, _ Task) (Result, error) {
		if executions.Add(1) == 1 {
			close(started)
		}
		<-release
		return Result{Output: json.RawMessage(`{"ok":true}`)}, nil
	})
	queueOne := &Queue{Registry: DefaultRegistry(), Store: storeOne, Executor: executor, WorkerID: "worker-one", LeaseDuration: time.Second}
	queueTwo := &Queue{Registry: DefaultRegistry(), Store: storeTwo, Executor: executor, WorkerID: "worker-two", LeaseDuration: time.Second}
	registerTestTask(t, queueOne, "only-once")

	doneOne := make(chan error, 1)
	go func() { doneOne <- queueOne.Run(context.Background(), 1) }()
	<-started
	if err := queueTwo.Run(context.Background(), 1); err != nil {
		t.Fatalf("second worker run: %v", err)
	}
	if got := executions.Load(); got != 1 {
		t.Fatalf("executions before release = %d, want 1", got)
	}
	close(release)
	if err := <-doneOne; err != nil {
		t.Fatalf("first worker run: %v", err)
	}
	task, err := storeOne.Get(context.Background(), "only-once")
	if err != nil || task.Status != StatusCompleted {
		t.Fatalf("task = %#v, %v", task, err)
	}
}

func TestFileStoreConcurrentSubmitAndCancelAreNotLost(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.json")
	const taskCount = 24
	var wg sync.WaitGroup
	errs := make(chan error, taskCount)
	for i := 0; i < taskCount; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			store, err := NewFileStore(path)
			if err != nil {
				errs <- err
				return
			}
			queue := &Queue{Registry: DefaultRegistry(), Store: store}
			_, err = queue.Register(context.Background(), TaskRequest{ID: fmt.Sprintf("task-%02d", i), Role: "planner", Input: json.RawMessage(`{"objective":"test"}`)})
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	store, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := store.List(context.Background())
	if err != nil || len(tasks) != taskCount {
		t.Fatalf("tasks = %d, %v; want %d", len(tasks), err, taskCount)
	}

	claimed, ok, err := store.Claim(context.Background(), "task-00", "stopped-worker", time.Now().UTC(), time.Minute)
	if err != nil || !ok || claimed.Status != StatusRunning {
		t.Fatalf("claim = %#v, %t, %v", claimed, ok, err)
	}
	cancelErrs := make(chan error, 8)
	for i := 0; i < cap(cancelErrs); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			otherStore, err := NewFileStore(path)
			if err == nil {
				_, err = (&Queue{Store: otherStore}).Cancel(context.Background(), "task-00")
			}
			cancelErrs <- err
		}()
	}
	wg.Wait()
	close(cancelErrs)
	for err := range cancelErrs {
		if err != nil {
			t.Fatalf("concurrent cancellation: %v", err)
		}
	}
	cancelled, err := store.Get(context.Background(), "task-00")
	if err != nil || !cancelled.CancelRequested || cancelled.Status != StatusRunning {
		t.Fatalf("durable cancellation = %#v, %v", cancelled, err)
	}
	completed, ok, err := store.Complete(context.Background(), "task-00", "stopped-worker", Result{Output: json.RawMessage(`{"ok":true}`)}, "", false, time.Now().UTC())
	if err != nil || !ok || completed.Status != StatusCancelled {
		t.Fatalf("completion must honour cancellation = %#v, %t, %v", completed, ok, err)
	}
}

func TestQueueRestartRecoversExpiredLeaseAndCancellation(t *testing.T) {
	store, err := NewFileStore(filepath.Join(t.TempDir(), "tasks.json"))
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	initial := &Queue{Registry: DefaultRegistry(), Store: store, Now: func() time.Time { return base }}
	registerTestTask(t, initial, "retry-after-restart")
	if _, ok, err := store.Claim(context.Background(), "retry-after-restart", "dead-worker", base, time.Second); err != nil || !ok {
		t.Fatalf("initial claim = %t, %v", ok, err)
	}
	var ran atomic.Int32
	restarted := &Queue{
		Registry: DefaultRegistry(), Store: store, WorkerID: "replacement-worker", Now: func() time.Time { return base.Add(2 * time.Second) },
		Executor: ExecutorFunc(func(context.Context, Task) (Result, error) {
			ran.Add(1)
			return Result{Output: json.RawMessage(`{"ok":true}`)}, nil
		}),
	}
	if err := restarted.Run(context.Background(), 1); err != nil {
		t.Fatalf("restart run: %v", err)
	}
	if got := ran.Load(); got != 1 {
		t.Fatalf("restarted executions = %d, want 1", got)
	}
	task, err := store.Get(context.Background(), "retry-after-restart")
	if err != nil || task.Status != StatusCompleted {
		t.Fatalf("recovered task = %#v, %v", task, err)
	}

	registerTestTask(t, initial, "cancel-after-restart")
	if _, ok, err := store.Claim(context.Background(), "cancel-after-restart", "dead-worker", base, time.Second); err != nil || !ok {
		t.Fatalf("initial claim = %t, %v", ok, err)
	}
	if _, err := initial.Cancel(context.Background(), "cancel-after-restart"); err != nil {
		t.Fatalf("request cancellation: %v", err)
	}
	if _, err := store.Recover(context.Background(), base.Add(2*time.Second)); err != nil {
		t.Fatalf("recover cancellation: %v", err)
	}
	task, err = store.Get(context.Background(), "cancel-after-restart")
	if err != nil || task.Status != StatusCancelled || !task.CancelRequested {
		t.Fatalf("recovered cancellation = %#v, %v", task, err)
	}
}

// TestFileStoreProcessHelper is executed in child test binaries so this test
// exercises the advisory lock across OS processes rather than just goroutines.
func TestFileStoreProcessHelper(t *testing.T) {
	if os.Getenv("LANA_AGENT_STORE_HELPER") != "1" {
		return
	}
	store, err := NewFileStore(os.Getenv("LANA_AGENT_STORE_PATH"))
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("LANA_AGENT_STORE_MODE") == "claim" {
		_, ok, err := store.Claim(context.Background(), "process-claim", os.Getenv("LANA_AGENT_STORE_OWNER"), time.Now().UTC(), time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprint(os.Stdout, ok)
		return
	}
	for i := 0; i < 8; i++ {
		id := fmt.Sprintf("%s-%d", os.Getenv("LANA_AGENT_STORE_OWNER"), i)
		if err := store.Create(context.Background(), Task{SchemaVersion: TaskSchemaVersion, ID: id, Role: "planner", Input: json.RawMessage(`{}`), Status: StatusQueued, CreatedAt: time.Now().UTC()}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFileStoreCoordinatesSeparateProcesses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.json")
	baseEnv := append(os.Environ(), "LANA_AGENT_STORE_HELPER=1", "LANA_AGENT_STORE_PATH="+path)
	commands := make([]*exec.Cmd, 3)
	for i := range commands {
		commands[i] = exec.Command(os.Args[0], "-test.run=^TestFileStoreProcessHelper$")
		commands[i].Env = append(baseEnv, "LANA_AGENT_STORE_OWNER=process-"+fmt.Sprint(i))
		if err := commands[i].Start(); err != nil {
			t.Fatal(err)
		}
	}
	for _, command := range commands {
		if err := command.Wait(); err != nil {
			t.Fatalf("helper process: %v", err)
		}
	}
	store, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := store.List(context.Background())
	if err != nil || len(tasks) != 24 {
		t.Fatalf("tasks after processes = %d, %v", len(tasks), err)
	}
	if err := store.Create(context.Background(), Task{SchemaVersion: TaskSchemaVersion, ID: "process-claim", Role: "planner", Input: json.RawMessage(`{}`), Status: StatusQueued, CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	claimCommands := make([]*exec.Cmd, 2)
	outputs := make([][]byte, len(claimCommands))
	for i := range claimCommands {
		claimCommands[i] = exec.Command(os.Args[0], "-test.run=^TestFileStoreProcessHelper$")
		claimCommands[i].Env = append(baseEnv, "LANA_AGENT_STORE_MODE=claim", "LANA_AGENT_STORE_OWNER=claimer-"+fmt.Sprint(i))
	}
	var wg sync.WaitGroup
	for i, command := range claimCommands {
		wg.Add(1)
		go func(i int, command *exec.Cmd) {
			defer wg.Done()
			output, err := command.CombinedOutput()
			if err != nil {
				t.Errorf("claim helper: %v: %s", err, output)
			}
			outputs[i] = output
		}(i, command)
	}
	wg.Wait()
	claims := 0
	for _, output := range outputs {
		if strings.Contains(string(output), "true") {
			claims++
		}
	}
	if claims != 1 {
		t.Fatalf("process claims = %d, want 1 (%q, %q)", claims, outputs[0], outputs[1])
	}
}
