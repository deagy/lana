package dispatch

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDispatchRunRequiresTask(t *testing.T) {
	cmd := NewCommand()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"run", "--role", "implementer"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error without --task")
	}
	if !strings.Contains(errOut.String(), "--task is required") {
		t.Fatalf("expected '--task is required' error, got: %q", errOut.String())
	}
}

func TestDispatchRunRegistersTask(t *testing.T) {
	dir := t.TempDir()
	cmd := NewCommand()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"run", "--role", "implementer", "--task", "implement feature", "--workdir", dir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "Task registered:") {
		t.Fatalf("expected task registered output, got: %q", out.String())
	}
	if !strings.Contains(out.String(), "Role: implementer") {
		t.Fatalf("expected role output, got: %q", out.String())
	}
	if !strings.Contains(out.String(), "Status: queued") {
		t.Fatalf("expected status output, got: %q", out.String())
	}
}

func TestDispatchRunWithDependencies(t *testing.T) {
	dir := t.TempDir()
	cmd := NewCommand()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"run", "--role", "reviewer", "--task", "review code", "--workdir", dir, "--depends-on", "task-1,task-2"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "Task registered:") {
		t.Fatalf("expected task registered output, got: %q", out.String())
	}
}

func TestDispatchStatusEmpty(t *testing.T) {
	dir := t.TempDir()
	cmd := NewCommand()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"status", "--workdir", dir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "No dispatch tasks found.") {
		t.Fatalf("expected empty status, got: %q", out.String())
	}
}

func TestDispatchStatusShowsTasks(t *testing.T) {
	dir := t.TempDir()
	// First register a task
	cmd := NewCommand()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"run", "--role", "planner", "--task", "plan feature", "--workdir", dir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Then check status
	cmd2 := NewCommand()
	var out2, errOut2 bytes.Buffer
	cmd2.SetOut(&out2)
	cmd2.SetErr(&errOut2)
	cmd2.SetArgs([]string{"status", "--workdir", dir})
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out2.String(), "Dispatch Status") {
		t.Fatalf("expected dispatch status, got: %q", out2.String())
	}
}

func TestDispatchStatusJSON(t *testing.T) {
	dir := t.TempDir()
	// First register a task
	cmd := NewCommand()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"run", "--role", "planner", "--task", "plan feature", "--workdir", dir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Then check status in JSON
	cmd2 := NewCommand()
	var out2, errOut2 bytes.Buffer
	cmd2.SetOut(&out2)
	cmd2.SetErr(&errOut2)
	cmd2.SetArgs([]string{"status", "--workdir", dir, "--json"})
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var tasks []json.RawMessage
	if err := json.Unmarshal(out2.Bytes(), &tasks); err != nil {
		t.Fatalf("expected valid JSON, got: %q", out2.String())
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
}

func TestDispatchHistoryEmpty(t *testing.T) {
	dir := t.TempDir()
	cmd := NewCommand()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"history", "--workdir", dir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "No dispatch history.") {
		t.Fatalf("expected empty history, got: %q", out.String())
	}
}

func TestDispatchHistoryShowsTasks(t *testing.T) {
	dir := t.TempDir()
	// Register multiple tasks
	for i := 0; i < 3; i++ {
		cmd := NewCommand()
		var out, errOut bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&errOut)
		cmd.SetArgs([]string{"run", "--role", "implementer", "--task", "implement feature", "--workdir", dir})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	// Check history
	cmd2 := NewCommand()
	var out2, errOut2 bytes.Buffer
	cmd2.SetOut(&out2)
	cmd2.SetErr(&errOut2)
	cmd2.SetArgs([]string{"history", "--workdir", dir})
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out2.String(), "Dispatch History") {
		t.Fatalf("expected dispatch history, got: %q", out2.String())
	}
}

func TestDispatchHistoryLimit(t *testing.T) {
	dir := t.TempDir()
	// Register multiple tasks
	for i := 0; i < 5; i++ {
		cmd := NewCommand()
		var out, errOut bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&errOut)
		cmd.SetArgs([]string{"run", "--role", "implementer", "--task", "implement feature", "--workdir", dir})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	// Check history with limit
	cmd2 := NewCommand()
	var out2, errOut2 bytes.Buffer
	cmd2.SetOut(&out2)
	cmd2.SetErr(&errOut2)
	cmd2.SetArgs([]string{"history", "--workdir", dir, "--limit", "2"})
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out2.String(), "last 2") {
		t.Fatalf("expected limit in output, got: %q", out2.String())
	}
}

func TestDispatchRunDefaultRole(t *testing.T) {
	dir := t.TempDir()
	cmd := NewCommand()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"run", "--task", "implement feature", "--workdir", dir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "Role: implementer") {
		t.Fatalf("expected default role, got: %q", out.String())
	}
}

func TestDispatchRunCreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	workDir := filepath.Join(dir, "work")
	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatal(err)
	}
	cmd := NewCommand()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"run", "--role", "planner", "--task", "plan", "--workdir", workDir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Check that .lana/agents directory was created
	agentsDir := filepath.Join(workDir, ".lana", "agents")
	if _, err := os.Stat(agentsDir); os.IsNotExist(err) {
		t.Fatalf("expected .lana/agents directory to be created")
	}
}
