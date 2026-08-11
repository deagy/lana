package agents

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	localagents "github.com/deagy/lana/internal/agents"
)

func testCommand(t *testing.T, store localagents.Store, args ...string) (*bytes.Buffer, error) {
	t.Helper()
	command := NewCommand(Options{Store: func(_ *cobra.Command) (localagents.Store, error) { return store, nil }})
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetArgs(args)
	return &output, command.Execute()
}

func TestRolesAndTaskLifecycleCommands(t *testing.T) {
	store, err := localagents.NewFileStore(filepath.Join(t.TempDir(), "tasks.json"))
	if err != nil {
		t.Fatal(err)
	}
	roles, err := testCommand(t, store, "roles")
	if err != nil || !strings.Contains(roles.String(), "planner") {
		t.Fatalf("roles = %q, %v", roles.String(), err)
	}
	submitted, err := testCommand(t, store, "submit", "--id", "plan-task", "--role", "planner", "--input", `{"objective":"plan"}`, "--metadata", "session_id=s-1")
	if err != nil || !strings.Contains(submitted.String(), `"status": "queued"`) {
		t.Fatalf("submit = %q, %v", submitted.String(), err)
	}
	listed, err := testCommand(t, store, "list", "--status", "queued")
	if err != nil || !strings.Contains(listed.String(), "plan-task\tqueued\tplanner") {
		t.Fatalf("list = %q, %v", listed.String(), err)
	}
	cancelled, err := testCommand(t, store, "cancel", "plan-task")
	if err != nil || !strings.Contains(cancelled.String(), `"status": "cancelled"`) {
		t.Fatalf("cancel = %q, %v", cancelled.String(), err)
	}
	shown, err := testCommand(t, store, "show", "plan-task")
	if err != nil || !strings.Contains(shown.String(), "status: cancelled") {
		t.Fatalf("show = %q, %v", shown.String(), err)
	}
}

func TestWorkRefusesToExecuteWithoutConfiguredWorker(t *testing.T) {
	store, err := localagents.NewFileStore(filepath.Join(t.TempDir(), "tasks.json"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = testCommand(t, store, "work")
	if err == nil || !strings.Contains(err.Error(), "standalone CLI") {
		t.Fatalf("work error = %v", err)
	}
}
