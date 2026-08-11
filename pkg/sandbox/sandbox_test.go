package sandbox

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSandboxValidatePath_Unrestricted(t *testing.T) {
	s := New(ModeUnrestricted, t.TempDir())
	if err := s.ValidatePath("/etc/passwd"); err != nil {
		t.Fatalf("unrestricted path: %v", err)
	}
}

func TestSandboxValidatePath_WithinWorkspace(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "file.txt")
	s := New(ModeWorkspaceWrite, root)
	if err := s.ValidatePath(path); err != nil {
		t.Fatalf("workspace path: %v", err)
	}
}

func TestSandboxValidatePath_OutsideWorkspace(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	s := New(ModeWorkspaceWrite, root)
	if err := s.ValidatePath(filepath.Join(outside, "file.txt")); err == nil {
		t.Fatal("expected outside path denial")
	}
}

func TestSandboxValidatePath_SymlinkEscape(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	s := New(ModeWorkspaceWrite, root)
	if err := s.ValidatePath(filepath.Join(root, "escape", "file.txt")); err == nil {
		t.Fatal("expected symlink escape denial")
	}
}

func TestSandboxAllowedWrite(t *testing.T) {
	root := t.TempDir()
	s := New(ModeWorkspaceWrite, root)
	if s.AllowedWrite(filepath.Join(root, "file.txt")) {
		t.Error("expected write to be rejected until no-follow mutation support exists")
	}
	if s.AllowedWrite(filepath.Join(t.TempDir(), "file.txt")) {
		t.Error("expected outside write denied")
	}
}

func TestSandboxAllowedRead(t *testing.T) {
	root := t.TempDir()
	s := New(ModeWorkspaceReadOnly, root)
	if !s.AllowedRead(filepath.Join(root, "file.txt")) {
		t.Error("expected read allowed")
	}
	if s.AllowedWrite(filepath.Join(root, "file.txt")) {
		t.Error("expected readonly write denied")
	}
}

func TestSandboxRoot(t *testing.T) {
	s := New(ModeUnrestricted, t.TempDir())
	if !filepath.IsAbs(s.Root()) {
		t.Errorf("expected absolute root, got %s", s.Root())
	}
}

func TestNewUnsupportedMode(t *testing.T) {
	s := New(Mode("unknown"), t.TempDir())
	if err := s.ValidatePath("file"); err == nil {
		t.Error("expected unsupported mode error")
	}
}
