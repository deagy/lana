package policy

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestEnforceCanonicalWorkspaceContainment(t *testing.T) {
	root := t.TempDir()
	p, err := New(Options{Mode: ModeWorkspaceWrite, Workspace: root})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Enforce(OperationWrite, filepath.Join(root, "new", "file"), false); !errors.Is(err, ErrUnenforceable) {
		t.Fatalf("contained mutation error: %v", err)
	}
	if _, err := p.Enforce(OperationRead, filepath.Join(root, "..", "outside"), false); !errors.Is(err, ErrDenied) {
		t.Fatalf("traversal error = %v, want denied", err)
	}
}

func TestEnforceRejectsExistingAndNewSymlinkEscape(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	p, err := New(Options{Mode: ModeWorkspaceWrite, Workspace: root})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(root, "escape"), filepath.Join(root, "escape", "new-file")} {
		if _, err := p.Enforce(OperationWrite, path, false); !errors.Is(err, ErrDenied) {
			t.Errorf("%s error = %v, want denied", path, err)
		}
	}
}

func TestEvaluateRiskAndApproval(t *testing.T) {
	p, err := New(Options{Mode: ModeUnrestricted, Workspace: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	evaluation, err := p.Evaluate(OperationDelete, filepath.Join(p.Workspace(), "file"), false)
	if err != nil {
		t.Fatal(err)
	}
	if evaluation.Risk != RiskHigh || evaluation.Decision != DecisionRequireApproval {
		t.Fatalf("evaluation = %+v", evaluation)
	}
	if _, err := p.Enforce(OperationDelete, filepath.Join(p.Workspace(), "file"), false); !errors.Is(err, ErrApprovalRequired) {
		t.Fatalf("delete error = %v", err)
	}
	if _, err := p.Enforce(OperationDelete, filepath.Join(p.Workspace(), "file"), true); err != nil {
		t.Fatalf("approved delete: %v", err)
	}
}

func TestWorkspaceMutationIsExplicitlyUnenforceable(t *testing.T) {
	p, err := New(Options{Mode: ModeWorkspaceWrite, Workspace: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Enforce(OperationWrite, filepath.Join(p.Workspace(), "file"), false); !errors.Is(err, ErrUnenforceable) {
		t.Fatalf("write error = %v", err)
	}
}

func TestReadOnlyAndExecutionDecisions(t *testing.T) {
	p, err := New(Options{Mode: ModeWorkspaceReadOnly, Workspace: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(p.Workspace(), "file")
	if _, err := p.Enforce(OperationWrite, path, false); !errors.Is(err, ErrDenied) {
		t.Fatalf("write error = %v", err)
	}
	if _, err := p.Enforce(OperationExecute, p.Workspace(), false); !errors.Is(err, ErrUnenforceable) {
		t.Fatalf("execute error = %v", err)
	}
}

func TestUnrestrictedExecuteIsAllowed(t *testing.T) {
	p, err := New(Options{Mode: ModeUnrestricted, Workspace: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	evaluation, err := p.Enforce(OperationExecute, "/", false)
	if err != nil || evaluation.Decision != DecisionAllow || evaluation.Risk != RiskHigh {
		t.Fatalf("evaluation=%+v error=%v", evaluation, err)
	}
}

func TestParseModeAndPolicyControllerAcceptOnlyDocumentedModes(t *testing.T) {
	p, err := New(Options{Mode: ModeWorkspaceReadOnly, Workspace: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range Modes() {
		parsed, err := ParseMode(string(mode))
		if err != nil || parsed != mode {
			t.Fatalf("parse %q = %q, %v", mode, parsed, err)
		}
	}
	if _, err := ParseMode("read-only"); err == nil {
		t.Fatal("undocumented alias was accepted")
	}
	if err := p.SetMode(ModeUnrestricted); err != nil || p.Mode() != ModeUnrestricted {
		t.Fatalf("set mode: %v, %q", err, p.Mode())
	}
	if err := p.SetMode(Mode("ask")); err == nil || p.Mode() != ModeUnrestricted {
		t.Fatalf("invalid mode mutated policy: %v, %q", err, p.Mode())
	}
}
