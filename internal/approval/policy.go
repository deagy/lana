package approval

import (
	"context"
	"github.com/deagy/lana/internal/tools"
)

// Policy determines whether and when a tool call requires approval.
type Policy interface {
	// ShouldApprove returns true if the tool call requires explicit approval.
	ShouldApprove(ctx context.Context, toolName string, riskLevel tools.RiskLevel) bool
}

// Mode represents the approval mode for the session.
type Mode string

const (
	// AskMode requires approval for all high-risk operations.
	AskMode Mode = "ask"

	// AutoEditMode auto-approves file edits but asks for shell execution.
	AutoEditMode Mode = "auto-edit"

	// FullAutoMode auto-approves all operations.
	FullAutoMode Mode = "full-auto"
)

// StaticPolicy is a simple policy based on approval mode.
type StaticPolicy struct {
	mode Mode
}

// NewStaticPolicy creates a new static approval policy.
func NewStaticPolicy(mode Mode) *StaticPolicy {
	return &StaticPolicy{mode: mode}
}

// ShouldApprove implements Policy.
func (p *StaticPolicy) ShouldApprove(ctx context.Context, toolName string, riskLevel tools.RiskLevel) bool {
	switch p.mode {
	case FullAutoMode:
		return false
	case AutoEditMode:
		// Ask for shell/system operations, auto-approve file edits
		if toolName == "exec" || toolName == "shell" {
			return true
		}
		return riskLevel == tools.RiskLevelHigh
	case AskMode:
		fallthrough
	default:
		// Ask for medium and high risk, auto-approve low risk
		return riskLevel >= tools.RiskLevelMedium
	}
}

// Broker handles approval requests interactively.
type Broker interface {
	// Request asks the user for approval of a tool call.
	// Returns true if approved, false if denied.
	Request(ctx context.Context, toolName string, description string) (bool, error)
}

// NullBroker is a no-op broker that always denies.
type NullBroker struct{}

func (nb *NullBroker) Request(ctx context.Context, toolName string, description string) (bool, error) {
	return false, nil
}
