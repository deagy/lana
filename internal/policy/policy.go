// Package policy contains the enforcement decisions shared by file and process
// operations. It deliberately does not claim to provide an OS sandbox.
package policy

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Mode describes the filesystem policy selected for an operation.
type Mode string

const (
	ModeUnrestricted      Mode = "unrestricted"
	ModeWorkspaceWrite    Mode = "workspace-write"
	ModeWorkspaceReadOnly Mode = "workspace-read-only"
)

// ModeController is the narrow control surface an authorization implementation
// may expose to an interactive client. Implementations must apply a selected
// mode to future authorization decisions; it is not presentation metadata.
type ModeController interface {
	Mode() Mode
	SetMode(Mode) error
}

// Modes returns the policy modes documented by the public CLI contract.
// The returned slice is a copy and may be used directly in help or validation
// messages.
func Modes() []Mode {
	return []Mode{ModeUnrestricted, ModeWorkspaceWrite, ModeWorkspaceReadOnly}
}

// ParseMode validates a public policy-mode spelling.
func ParseMode(value string) (Mode, error) {
	mode := Mode(strings.TrimSpace(value))
	if !validMode(mode) {
		return "", fmt.Errorf("invalid policy mode %q (expected unrestricted, workspace-write, or workspace-read-only)", value)
	}
	return mode, nil
}

// Operation is the action being authorized.
type Operation string

const (
	OperationRead    Operation = "read"
	OperationWrite   Operation = "write"
	OperationDelete  Operation = "delete"
	OperationExecute Operation = "execute"
	OperationSearch  Operation = "search"
	OperationInfo    Operation = "info"
)

// Risk is an operation's inherent impact category.
type Risk string

const (
	RiskLow    Risk = "low"
	RiskMedium Risk = "medium"
	RiskHigh   Risk = "high"
)

// Decision is the result of a policy evaluation.
type Decision string

const (
	DecisionAllow           Decision = "allow"
	DecisionRequireApproval Decision = "require-approval"
	DecisionDeny            Decision = "deny"
	DecisionUnenforceable   Decision = "unenforceable"
)

var (
	// ErrDenied means the selected policy prohibits the request.
	ErrDenied = errors.New("policy denied operation")
	// ErrApprovalRequired means a high-risk request needs explicit approval.
	ErrApprovalRequired = errors.New("policy approval required")
	// ErrUnenforceable means Lana cannot enforce the requested boundary safely.
	ErrUnenforceable = errors.New("policy boundary cannot be enforced")
)

// Options constructs a Policy. Workspace must identify an existing directory.
type Options struct {
	Mode      Mode
	Workspace string
}

// Evaluation describes a policy decision and the canonical path it evaluated.
type Evaluation struct {
	Decision      Decision
	Risk          Risk
	CanonicalPath string
	Reason        string
}

// Policy enforces workspace containment for filesystem operations.
// Canonicalization prevents known symlink escapes, but cannot eliminate races
// between validation and a subsequent filesystem operation.
type Policy struct {
	mu        sync.RWMutex
	mode      Mode
	workspace string
}

// New constructs a Policy with a canonical workspace root.
func New(options Options) (*Policy, error) {
	if !validMode(options.Mode) {
		return nil, fmt.Errorf("invalid policy mode %q", options.Mode)
	}
	if options.Workspace == "" {
		return nil, fmt.Errorf("workspace must not be empty")
	}
	root, err := filepath.Abs(options.Workspace)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace %q: %w", options.Workspace, err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("canonicalize workspace %q: %w", options.Workspace, err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("stat workspace %q: %w", root, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("workspace %q is not a directory", root)
	}
	return &Policy{mode: options.Mode, workspace: root}, nil
}

func validMode(mode Mode) bool {
	switch mode {
	case ModeUnrestricted, ModeWorkspaceWrite, ModeWorkspaceReadOnly:
		return true
	default:
		return false
	}
}

// Mode returns the configured mode.
func (p *Policy) Mode() Mode {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.mode
}

// SetMode selects a documented policy mode for future evaluations. Callers
// should only expose this through an authorization implementation that uses
// the policy for the affected tool calls.
func (p *Policy) SetMode(mode Mode) error {
	if !validMode(mode) {
		return fmt.Errorf("invalid policy mode %q", mode)
	}
	p.mu.Lock()
	p.mode = mode
	p.mu.Unlock()
	return nil
}

// Workspace returns the canonical workspace root, without a trailing separator.
func (p *Policy) Workspace() string { return p.workspace }

// Evaluate categorizes and decides a request. approve is an explicit caller
// approval for operations whose default decision is require-approval.
func (p *Policy) Evaluate(operation Operation, path string, approve bool) (Evaluation, error) {
	evaluation := Evaluation{Risk: riskFor(operation)}
	mode := p.Mode()
	if !validOperation(operation) {
		return evaluation, fmt.Errorf("%w: unknown operation %q", ErrDenied, operation)
	}

	if operation == OperationExecute && mode != ModeUnrestricted {
		evaluation.Decision = DecisionUnenforceable
		evaluation.Reason = "workspace execution is not an OS-enforced sandbox; use unrestricted only when that authority is intended"
		return evaluation, nil
	}

	if mode != ModeUnrestricted {
		canonical, err := p.CanonicalPath(path)
		if err != nil {
			return evaluation, err
		}
		evaluation.CanonicalPath = canonical
		if !within(p.workspace, canonical) {
			evaluation.Decision = DecisionDeny
			evaluation.Reason = "path resolves outside the workspace"
			return evaluation, nil
		}
	} else if path != "" {
		canonical, err := p.CanonicalPath(path)
		if err == nil {
			evaluation.CanonicalPath = canonical
		}
	}

	if mode == ModeWorkspaceReadOnly && isMutating(operation) {
		evaluation.Decision = DecisionDeny
		evaluation.Reason = "workspace-read-only does not permit mutation"
		return evaluation, nil
	}
	if mode == ModeWorkspaceWrite && isMutating(operation) {
		// Canonicalizing before os.Open/os.Remove is not sufficient to contain a
		// mutation: an attacker can swap a path component for a symlink between
		// validation and use. This package does not yet use descriptor-relative,
		// no-follow syscalls, so rejecting is safer than claiming containment.
		evaluation.Decision = DecisionUnenforceable
		evaluation.Reason = "workspace mutation requires descriptor-relative no-follow filesystem support; use unrestricted only with explicit authority"
		return evaluation, nil
	}
	if operation == OperationDelete && !approve {
		evaluation.Decision = DecisionRequireApproval
		evaluation.Reason = "delete is high-risk and requires explicit approval"
		return evaluation, nil
	}
	evaluation.Decision = DecisionAllow
	return evaluation, nil
}

// Enforce returns a stable, categorized error for a rejected decision.
func (p *Policy) Enforce(operation Operation, path string, approve bool) (Evaluation, error) {
	evaluation, err := p.Evaluate(operation, path, approve)
	if err != nil {
		return evaluation, err
	}
	switch evaluation.Decision {
	case DecisionAllow:
		return evaluation, nil
	case DecisionRequireApproval:
		return evaluation, fmt.Errorf("%w: %s", ErrApprovalRequired, evaluation.Reason)
	case DecisionUnenforceable:
		return evaluation, fmt.Errorf("%w: %s", ErrUnenforceable, evaluation.Reason)
	default:
		return evaluation, fmt.Errorf("%w: %s", ErrDenied, evaluation.Reason)
	}
}

// CanonicalPath resolves a path while also resolving the nearest existing
// parent. That makes validation work for new write destinations and detects
// symlink escapes in existing ancestors.
func (p *Policy) CanonicalPath(path string) (string, error) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(p.workspace, path)
	}
	return canonicalPath(path)
}

func canonicalPath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path must not be empty")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve path %q: %w", path, err)
	}
	abs = filepath.Clean(abs)
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("canonicalize path %q: %w", path, err)
	}

	ancestor := abs
	var missing []string
	for {
		if _, err := os.Lstat(ancestor); err == nil {
			break
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("inspect path %q: %w", path, err)
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			return "", fmt.Errorf("no existing ancestor for path %q", path)
		}
		missing = append(missing, filepath.Base(ancestor))
		ancestor = parent
	}
	resolved, err := filepath.EvalSymlinks(ancestor)
	if err != nil {
		return "", fmt.Errorf("canonicalize parent of %q: %w", path, err)
	}
	for i := len(missing) - 1; i >= 0; i-- {
		resolved = filepath.Join(resolved, missing[i])
	}
	return resolved, nil
}

func within(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func validOperation(operation Operation) bool {
	switch operation {
	case OperationRead, OperationWrite, OperationDelete, OperationExecute, OperationSearch, OperationInfo:
		return true
	default:
		return false
	}
}

func isMutating(operation Operation) bool {
	return operation == OperationWrite || operation == OperationDelete
}

func riskFor(operation Operation) Risk {
	switch operation {
	case OperationRead, OperationSearch, OperationInfo:
		return RiskLow
	case OperationWrite:
		return RiskMedium
	case OperationDelete, OperationExecute:
		return RiskHigh
	default:
		return RiskHigh
	}
}
