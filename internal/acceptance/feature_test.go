// Package acceptance executes deterministic regression checks named by the
// checked-in Gherkin feature inventory. It uses no networked BDD runtime.
package acceptance

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/deagy/lana/internal/agents"
	knowledgecmd "github.com/deagy/lana/internal/cmd/knowledge"
	"github.com/deagy/lana/internal/knowledge"
)

// TestAcceptanceFeature ensures every SCN tag in the checked-in specification
// has one executable, deterministic assertion.
func TestAcceptanceFeature(t *testing.T) {
	path := featurePath(t)
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) && os.Getenv("LANA_NONGIT_RELEASE_REGRESSION") != "" {
		// The non-Git release fixture intentionally copies only source and
		// testdata. Its own test verifies the release behavior, while the
		// documentation-backed acceptance inventory runs in this repository.
		t.Skip("non-Git release fixture does not package repository documentation")
	}
	scenarios, err := parseFeature(path)
	if err != nil {
		t.Fatal(err)
	}
	runners := map[string]func(*testing.T){
		"SCN-AGT-001": acceptanceConcurrentClaimAndUpdate,
		"SCN-AGT-002": acceptanceCancellationRecovery,
		"SCN-KNO-001": acceptanceNoFollowRejection,
		"SCN-KNO-002": acceptanceTerminalSafeOutput,
	}
	if len(scenarios) != len(runners) {
		t.Fatalf("feature has %d scenarios, harness has %d", len(scenarios), len(runners))
	}
	for id, scenario := range scenarios {
		run, ok := runners[id]
		if !ok {
			t.Fatalf("%s (%s) has no acceptance runner", id, scenario.name)
		}
		t.Run(id+"/"+scenario.name, run)
	}
}

func acceptanceConcurrentClaimAndUpdate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.json")
	initial, err := agents.NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	if err := initial.Create(context.Background(), agents.Task{
		SchemaVersion: agents.TaskSchemaVersion, ID: "shared-task", Role: "planner",
		Input: json.RawMessage(`{"objective":"local"}`), Status: agents.StatusQueued, CreatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	type claim struct {
		owner string
		ok    bool
		err   error
	}
	claims := make(chan claim, 2)
	start := make(chan struct{})
	var wait sync.WaitGroup
	for _, owner := range []string{"worker-a", "worker-b"} {
		wait.Add(1)
		go func(owner string) {
			defer wait.Done()
			store, err := agents.NewFileStore(path)
			if err != nil {
				claims <- claim{owner: owner, err: err}
				return
			}
			<-start
			_, ok, err := store.Claim(context.Background(), "shared-task", owner, now, time.Minute)
			claims <- claim{owner: owner, ok: ok, err: err}
		}(owner)
	}
	close(start)
	wait.Wait()
	close(claims)

	var winner string
	for result := range claims {
		if result.err != nil {
			t.Fatalf("%s claim: %v", result.owner, result.err)
		}
		if result.ok {
			if winner != "" {
				t.Fatalf("multiple workers claimed the task: %s and %s", winner, result.owner)
			}
			winner = result.owner
		}
	}
	if winner == "" {
		t.Fatal("no worker claimed the queued task")
	}

	completed, ok, err := initial.Complete(context.Background(), "shared-task", winner, agents.Result{Output: json.RawMessage(`{"done":true}`)}, "", false, now.Add(time.Second))
	if err != nil || !ok || completed.Status != agents.StatusCompleted {
		t.Fatalf("winner completion = %#v, ok=%t, err=%v", completed, ok, err)
	}
	loser := "worker-a"
	if loser == winner {
		loser = "worker-b"
	}
	if _, ok, err := initial.Complete(context.Background(), "shared-task", loser, agents.Result{}, "", false, now.Add(2*time.Second)); err != nil || ok {
		t.Fatalf("non-owner completion changed task: ok=%t err=%v", ok, err)
	}
}

func acceptanceCancellationRecovery(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.json")
	initial, err := agents.NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	if err := initial.Create(context.Background(), agents.Task{
		SchemaVersion: agents.TaskSchemaVersion, ID: "stopped-task", Role: "planner",
		Input: json.RawMessage(`{}`), Status: agents.StatusQueued, CreatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := initial.Claim(context.Background(), "stopped-task", "stopped-worker", now, time.Second); err != nil || !ok {
		t.Fatalf("claim = ok=%t err=%v", ok, err)
	}
	if task, err := initial.Cancel(context.Background(), "stopped-task", now.Add(time.Millisecond)); err != nil || !task.CancelRequested || task.Status != agents.StatusRunning {
		t.Fatalf("durable cancellation = %#v, err=%v", task, err)
	}

	restarted, err := agents.NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.Recover(context.Background(), now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	task, err := restarted.Get(context.Background(), "stopped-task")
	if err != nil || task.Status != agents.StatusCancelled || !task.CancelRequested {
		t.Fatalf("recovered task = %#v, err=%v", task, err)
	}
	if _, ok, err := restarted.Complete(context.Background(), "stopped-task", "stopped-worker", agents.Result{Output: json.RawMessage(`{"late":"success"}`)}, "", false, now.Add(3*time.Second)); err != nil || ok {
		t.Fatalf("late completion replaced cancellation: ok=%t err=%v", ok, err)
	}
}

func acceptanceNoFollowRejection(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(t.TempDir(), "target-index.json")
	if err := os.WriteFile(target, []byte(`{"version":2,"documents":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, "index.json")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	store, err := knowledge.New(knowledge.Options{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ListDocuments(""); err == nil {
		t.Fatal("knowledge store followed a symlinked index")
	}
}

func acceptanceTerminalSafeOutput(t *testing.T) {
	root := t.TempDir()
	storePath := filepath.Join(root, "store")
	source := filepath.Join(root, "notes.txt")
	const raw = "visible\x1b[31m text \u202Ereverse"
	if err := os.WriteFile(source, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	if output, err := runKnowledge(storePath, "ingest", source); err != nil {
		t.Fatalf("ingest: %v\n%s", err, output)
	}
	output, err := runKnowledge(storePath, "search", "visible")
	if err != nil {
		t.Fatalf("search: %v\n%s", err, output)
	}
	if strings.Contains(output, "\x1b") || strings.Contains(output, "\u202e") {
		t.Fatalf("human output contains raw terminal character: %q", output)
	}
	if !strings.Contains(output, `\x1B`) || !strings.Contains(output, `\u202E`) {
		t.Fatalf("human output did not render terminal characters visibly: %q", output)
	}
}

func runKnowledge(storePath string, args ...string) (string, error) {
	command := knowledgecmd.NewCommand()
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs(append([]string{"--store", storePath}, args...))
	err := command.Execute()
	return output.String(), err
}

type featureScenario struct{ name string }

func parseFeature(path string) (map[string]featureScenario, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read feature: %w", err)
	}
	result := map[string]featureScenario{}
	var pending []string
	for lineNumber, line := range strings.Split(string(contents), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "@") {
			pending = strings.Fields(line)
			continue
		}
		if !strings.HasPrefix(line, "Scenario:") {
			continue
		}
		name := strings.TrimSpace(strings.TrimPrefix(line, "Scenario:"))
		if name == "" {
			return nil, fmt.Errorf("feature line %d: scenario name is empty", lineNumber+1)
		}
		for _, tag := range pending {
			id := strings.TrimPrefix(tag, "@")
			if !strings.HasPrefix(id, "SCN-") {
				continue
			}
			if _, exists := result[id]; exists {
				return nil, fmt.Errorf("feature line %d: duplicate scenario tag %s", lineNumber+1, id)
			}
			result[id] = featureScenario{name: name}
		}
		pending = nil
	}
	if len(result) == 0 {
		return nil, errors.New("feature contains no SCN-* scenario tags")
	}
	return result, nil
}

func featurePath(t *testing.T) string {
	return filepath.Join(repositoryRoot(t), "docs", "acceptance-scenarios.feature")
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate acceptance source")
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}
