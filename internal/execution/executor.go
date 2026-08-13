package execution

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/deagy/lana/internal/approval"
	"github.com/deagy/lana/internal/tools"
)

// Executor runs tools with approval checks and error handling.
type Executor struct {
	registry       *tools.Registry
	approvalPolicy approval.Policy
	approvalBroker approval.Broker
}

// NewExecutor creates a new tool executor.
func NewExecutor(registry *tools.Registry, policy approval.Policy, broker approval.Broker) *Executor {
	return &Executor{
		registry:       registry,
		approvalPolicy: policy,
		approvalBroker: broker,
	}
}

// Result represents the result of executing a tool.
type Result struct {
	ID       string
	ToolName string
	Input    json.RawMessage
	Output   string
	Error    string
	Approved bool
}

// Execute runs a tool with approval checks.
func (e *Executor) Execute(ctx context.Context, id, toolName string, input json.RawMessage) (*Result, error) {
	result := &Result{
		ID:       id,
		ToolName: toolName,
		Input:    input,
	}

	// Get the tool
	tool, err := e.registry.Get(toolName)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}

	// Get tool definition for risk level
	def, ok := tool.(*tools.Definition)
	if !ok {
		result.Error = "tool is not a definition"
		return result, fmt.Errorf("invalid tool type")
	}

	// Check if approval is needed
	needsApproval := e.approvalPolicy.ShouldApprove(ctx, toolName, def.RiskLevel)

	if needsApproval && e.approvalBroker != nil {
		// Request approval
		approved, err := e.approvalBroker.Request(ctx, toolName, fmt.Sprintf("Execute %s", toolName))
		if err != nil {
			result.Error = fmt.Sprintf("approval error: %v", err)
			return result, err
		}

		result.Approved = approved
		if !approved {
			result.Error = "tool execution denied by user"
			return result, fmt.Errorf("denied")
		}
	} else {
		result.Approved = !needsApproval
	}

	// Execute the tool
	output, err := tool.Execute(ctx, input)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}

	result.Output = output
	return result, nil
}
