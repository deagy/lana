package impl

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/deagy/lana/internal/policy"
	"github.com/deagy/lana/internal/tools"
)

// ShellTool provides shell command execution.
type ShellTool struct {
	workspace string
}

// NewShellTool creates a new shell tool.
func NewShellTool(workspace string) *ShellTool {
	return &ShellTool{
		workspace: workspace,
	}
}

// Exec creates an exec tool for running commands.
func (st *ShellTool) Exec() tools.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": {
				"type": "string",
				"description": "Shell command to execute"
			},
			"cwd": {
				"type": "string",
				"description": "Working directory (relative to workspace)"
			}
		},
		"required": ["command"]
	}`)

	return &tools.Definition{
		NameVal:        "exec",
		DescriptionVal: "Execute a shell command in the workspace",
		SchemaVal:      schema,
		RiskLevel:      tools.RiskLevelHigh,
		ExecutorVal:    &shellExecExecutor{workspace: st.workspace},
	}
}

type shellExecExecutor struct {
	workspace string
}

func (e *shellExecExecutor) Execute(_ interface{}, input json.RawMessage) (string, error) {
	var req struct {
		Command string `json:"command"`
		Cwd     string `json:"cwd"`
	}

	if err := json.Unmarshal(input, &req); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	// Check for high-risk patterns
	if policy.IsHighRisk(req.Command) {
		return "", fmt.Errorf("high-risk command requires approval: %s", req.Command)
	}

	// Determine working directory
	cwd := e.workspace
	if req.Cwd != "" {
		absPath, err := policy.ResolveWorkspacePath(e.workspace, req.Cwd)
		if err != nil {
			return "", fmt.Errorf("invalid cwd: %w", err)
		}
		cwd = absPath
	}

	// Execute command
	cmd := exec.Command("sh", "-c", req.Command)
	cmd.Dir = cwd
	cmd.Env = os.Environ()

	// Run and capture output
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Still return output even on error
		return fmt.Sprintf("Exit error: %v\n%s", err, string(output)), nil
	}

	return string(output), nil
}
