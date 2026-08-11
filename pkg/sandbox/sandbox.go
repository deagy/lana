// Package sandbox preserves the legacy sandbox API while delegating all
// decisions to the central policy package.
package sandbox

import (
	"github.com/deagy/lana/internal/policy"
)

// Mode represents a sandbox execution mode.
type Mode = policy.Mode

const (
	ModeUnrestricted      = policy.ModeUnrestricted
	ModeWorkspaceWrite    = policy.ModeWorkspaceWrite
	ModeWorkspaceReadOnly = policy.ModeWorkspaceReadOnly
)

// Sandbox is a compatibility adapter around policy.Policy.
type Sandbox struct {
	policy *policy.Policy
	err    error
}

// New creates a Sandbox with the given mode and workspace root. Construction
// errors are preserved and returned by all validation calls for compatibility
// with the old no-error constructor.
func New(mode Mode, root string) *Sandbox {
	p, err := policy.New(policy.Options{Mode: mode, Workspace: root})
	return &Sandbox{policy: p, err: err}
}

// Mode returns the current sandbox mode.
func (s *Sandbox) Mode() Mode {
	if s.policy == nil {
		return ""
	}
	return s.policy.Mode()
}

// Root returns the canonical workspace root.
func (s *Sandbox) Root() string {
	if s.policy == nil {
		return ""
	}
	return s.policy.Workspace()
}

// ValidatePath checks workspace containment for a path.
func (s *Sandbox) ValidatePath(path string) error {
	if s.err != nil {
		return s.err
	}
	_, err := s.policy.Enforce(policy.OperationRead, path, false)
	return err
}

// AllowedWrite checks whether a write operation is allowed for the given path.
func (s *Sandbox) AllowedWrite(path string) bool {
	if s.err != nil {
		return false
	}
	_, err := s.policy.Enforce(policy.OperationWrite, path, false)
	return err == nil
}

// AllowedRead checks whether a read operation is allowed for the given path.
func (s *Sandbox) AllowedRead(path string) bool {
	return s.ValidatePath(path) == nil
}
