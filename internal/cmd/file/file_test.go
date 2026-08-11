package file

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deagy/lana/internal/app"
	"github.com/deagy/lana/pkg/config"
	"github.com/spf13/cobra"
)

func executeFile(t *testing.T, workspace string, args ...string) (string, error) {
	t.Helper()
	root := &cobra.Command{Use: "lana", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().String("workspace", workspace, "")
	root.AddCommand(NewCommand())
	var out, errOut bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs(append([]string{"file"}, args...))
	err := root.Execute()
	return out.String() + errOut.String(), err
}

func TestFileReadRejectsSymlinkEscape(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	outsideFile := filepath.Join(outside, "secret")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsideFile, filepath.Join(root, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	_, err := executeFile(t, root, "read", "link")
	if err == nil || !strings.Contains(err.Error(), "policy denied") {
		t.Fatalf("error = %v", err)
	}
}

func TestFileWriteRejectsNewPathBelowSymlinkEscape(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	_, err := executeFile(t, root, "write", "escape/new", "--content", "unsafe")
	if err == nil || !strings.Contains(err.Error(), "policy denied") {
		t.Fatalf("error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(outside, "new")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unexpected outside write: %v", err)
	}
}

func TestFileWorkspaceWriteMutationIsRejectedBeforeUse(t *testing.T) {
	root := t.TempDir()
	_, err := executeFile(t, root, "write", "new-file", "--content", "unsafe")
	if err == nil || !strings.Contains(err.Error(), "descriptor-relative no-follow") {
		t.Fatalf("expected explicit mutation enforcement error, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "new-file")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("rejected write changed workspace: %v", err)
	}
}

func TestFileReadIsBounded(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "large"), []byte("12345"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := executeFile(t, root, "read", "large", "--max-bytes", "4")
	if err == nil || !errors.Is(err, ErrReadLimit) {
		t.Fatalf("error = %v", err)
	}
}

func TestFileDeleteRequiresExplicitApproval(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "delete-me")
	if err := os.WriteFile(target, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := executeFile(t, root, "delete", "delete-me", "--sandbox", "unrestricted")
	if err == nil || !strings.Contains(err.Error(), "approval required") {
		t.Fatalf("error = %v", err)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("file should remain: %v", err)
	}
	if _, err := executeFile(t, root, "delete", "delete-me", "--approve", "--sandbox", "unrestricted"); err != nil {
		t.Fatalf("approved delete: %v", err)
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("file should be deleted: %v", err)
	}
}

func TestFileCopyIsBoundedAndContained(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "source"), []byte("12345"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := executeFile(t, root, "copy", "source", "destination", "--max-bytes", "4", "--sandbox", "unrestricted")
	if err == nil || !errors.Is(err, ErrReadLimit) {
		t.Fatalf("error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "destination")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("destination should not exist: %v", err)
	}
}

func TestFileMoveRequiresApprovalForSourceDeletion(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "source"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := executeFile(t, root, "move", "source", "destination", "--sandbox", "unrestricted")
	if err == nil || !strings.Contains(err.Error(), "approval required") {
		t.Fatalf("error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "source")); err != nil {
		t.Fatalf("source should remain: %v", err)
	}
	if _, err := executeFile(t, root, "move", "source", "destination", "--approve", "--sandbox", "unrestricted"); err != nil {
		t.Fatalf("approved move: %v", err)
	}
}

func TestFileDeletePreservesInternalSymlinkTarget(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	if err := os.WriteFile(target, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := executeFile(t, root, "delete", "link", "--approve", "--sandbox", "unrestricted"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("target was deleted: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(root, "link")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("link should be deleted: %v", err)
	}
}

func TestFileUsesAppWorkspaceAndSandboxWithFlagOverride(t *testing.T) {
	workspace := t.TempDir()
	application := fileTestApplication(t, workspace, "exec:\n  sandbox: unrestricted\n")
	cmd := NewCommand()
	cmd.SetContext(app.WithContext(context.Background(), application))
	cmd.SetArgs([]string{"write", "configured.txt", "--content", "ok"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(workspace, "configured.txt")); err != nil || string(data) != "ok" {
		t.Fatalf("app workspace write: data=%q err=%v", data, err)
	}

	cmd = NewCommand()
	cmd.SetContext(app.WithContext(context.Background(), application))
	cmd.SetArgs([]string{"write", "blocked.txt", "--content", "no", "--sandbox", "workspace-write"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "cannot be enforced") {
		t.Fatalf("sandbox flag should override config, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspace, "blocked.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("blocked write modified workspace: %v", err)
	}
}

func fileTestApplication(t *testing.T, workspace, contents string) *app.Application {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	application, err := app.New(app.Options{Config: config.ResolveOptions{Workspace: workspace, WorkingDirectory: workspace, ConfigPath: configPath, UserConfigPath: filepath.Join(t.TempDir(), "missing.yaml")}})
	if err != nil {
		t.Fatal(err)
	}
	return application
}
