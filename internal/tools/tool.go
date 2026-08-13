package tools

import (
	"context"
	"encoding/json"
)

// Tool defines an executable tool that an agent can invoke.
type Tool interface {
	// Name returns the tool's unique identifier.
	Name() string

	// Description returns a description suitable for the AI provider.
	Description() string

	// InputSchema returns the JSON Schema for this tool's input.
	InputSchema() json.RawMessage

	// Execute runs the tool with the given input. Results should be sanitized.
	Execute(ctx context.Context, input json.RawMessage) (string, error)
}

// Executor is a pluggable handler for a specific tool.
type Executor interface {
	// Execute runs the tool and returns a result string and error.
	Execute(ctx context.Context, input json.RawMessage) (string, error)
}

// Definition is a complete tool definition.
type Definition struct {
	NameVal        string
	DescriptionVal string
	SchemaVal      json.RawMessage
	ExecutorVal    Executor
	RiskLevel      RiskLevel
	RequiresApproval bool
}

func (d *Definition) Name() string {
	return d.NameVal
}

func (d *Definition) Description() string {
	return d.DescriptionVal
}

func (d *Definition) InputSchema() json.RawMessage {
	return d.SchemaVal
}

func (d *Definition) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	return d.ExecutorVal.Execute(ctx, input)
}

// RiskLevel categorizes the risk of executing a tool.
type RiskLevel int

const (
	RiskLevelLow RiskLevel = iota
	RiskLevelMedium
	RiskLevelHigh
)

// String returns a human-readable name for the risk level.
func (r RiskLevel) String() string {
	switch r {
	case RiskLevelLow:
		return "low"
	case RiskLevelMedium:
		return "medium"
	case RiskLevelHigh:
		return "high"
	default:
		return "unknown"
	}
}
