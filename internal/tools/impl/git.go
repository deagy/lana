package impl

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/deagy/lana/internal/policy"
	"github.com/deagy/lana/internal/tools"
)

// GitTool provides Git operations.
type GitTool struct {
	workspace string
}

// NewGitTool creates a new Git tool.
func NewGitTool(workspace string) *GitTool {
	return &GitTool{
		workspace: workspace,
	}
}

// Status creates a git_status tool.
func (gt *GitTool) Status() tools.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"porcelain": {
				"type": "boolean",
				"description": "Use porcelain format (machine-readable)"
			}
		}
	}`)

	return &tools.Definition{
		NameVal:        "git_status",
		DescriptionVal: "Show git working directory status",
		SchemaVal:      schema,
		RiskLevel:      tools.RiskLevelLow,
		ExecutorVal:    &gitStatusExecutor{workspace: gt.workspace},
	}
}

// Diff creates a git_diff tool.
func (gt *GitTool) Diff() tools.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"file": {
				"type": "string",
				"description": "Specific file to diff (optional)"
			},
			"staged": {
				"type": "boolean",
				"description": "Show staged changes only"
			}
		}
	}`)

	return &tools.Definition{
		NameVal:        "git_diff",
		DescriptionVal: "Show git diff for changes",
		SchemaVal:      schema,
		RiskLevel:      tools.RiskLevelLow,
		ExecutorVal:    &gitDiffExecutor{workspace: gt.workspace},
	}
}

// Commit creates a git_commit tool.
func (gt *GitTool) Commit() tools.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"message": {
				"type": "string",
				"description": "Commit message"
			},
			"all": {
				"type": "boolean",
				"description": "Stage all changes before committing"
			}
		},
		"required": ["message"]
	}`)

	return &tools.Definition{
		NameVal:        "git_commit",
		DescriptionVal: "Create a git commit",
		SchemaVal:      schema,
		RiskLevel:      tools.RiskLevelMedium,
		ExecutorVal:    &gitCommitExecutor{workspace: gt.workspace},
	}
}

// Branch creates a git_branch tool.
func (gt *GitTool) Branch() tools.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {
				"type": "string",
				"enum": ["list", "create", "delete", "current"],
				"description": "Branch action to perform"
			},
			"name": {
				"type": "string",
				"description": "Branch name (for create/delete)"
			}
		},
		"required": ["action"]
	}`)

	return &tools.Definition{
		NameVal:        "git_branch",
		DescriptionVal: "Manage git branches",
		SchemaVal:      schema,
		RiskLevel:      tools.RiskLevelMedium,
		ExecutorVal:    &gitBranchExecutor{workspace: gt.workspace},
	}
}

type gitStatusExecutor struct {
	workspace string
}

func (e *gitStatusExecutor) Execute(_ interface{}, input json.RawMessage) (string, error) {
	var req struct {
		Porcelain bool `json:"porcelain"`
	}
	json.Unmarshal(input, &req)

	cmd := exec.Command("git", "status")
	if req.Porcelain {
		cmd = exec.Command("git", "status", "--porcelain")
	}
	cmd.Dir = e.workspace

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git status failed: %w", err)
	}

	return string(output), nil
}

type gitDiffExecutor struct {
	workspace string
}

func (e *gitDiffExecutor) Execute(_ interface{}, input json.RawMessage) (string, error) {
	var req struct {
		File   string `json:"file"`
		Staged bool   `json:"staged"`
	}
	json.Unmarshal(input, &req)

	args := []string{"diff"}
	if req.Staged {
		args = append(args, "--staged")
	}
	if req.File != "" {
		// Validate file path
		if _, err := policy.ResolveWorkspacePath(e.workspace, req.File); err != nil {
			return "", fmt.Errorf("invalid file: %w", err)
		}
		args = append(args, req.File)
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = e.workspace

	output, err := cmd.CombinedOutput()
	if err != nil && len(output) == 0 {
		return "", fmt.Errorf("git diff failed: %w", err)
	}

	return string(output), nil
}

type gitCommitExecutor struct {
	workspace string
}

func (e *gitCommitExecutor) Execute(_ interface{}, input json.RawMessage) (string, error) {
	var req struct {
		Message string `json:"message"`
		All     bool   `json:"all"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	if req.Message == "" {
		return "", fmt.Errorf("commit message required")
	}

	args := []string{"commit"}
	if req.All {
		args = append(args, "-a")
	}
	args = append(args, "-m", req.Message)

	cmd := exec.Command("git", args...)
	cmd.Dir = e.workspace

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git commit failed: %w", err)
	}

	return string(output), nil
}

type gitBranchExecutor struct {
	workspace string
}

func (e *gitBranchExecutor) Execute(_ interface{}, input json.RawMessage) (string, error) {
	var req struct {
		Action string `json:"action"`
		Name   string `json:"name"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	switch req.Action {
	case "list":
		cmd := exec.Command("git", "branch", "-a")
		cmd.Dir = e.workspace
		output, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("git branch list failed: %w", err)
		}
		return string(output), nil

	case "current":
		cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
		cmd.Dir = e.workspace
		output, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("git branch current failed: %w", err)
		}
		return strings.TrimSpace(string(output)), nil

	case "create":
		if req.Name == "" {
			return "", fmt.Errorf("branch name required for create")
		}
		cmd := exec.Command("git", "checkout", "-b", req.Name)
		cmd.Dir = e.workspace
		output, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("git branch create failed: %w", err)
		}
		return string(output), nil

	case "delete":
		if req.Name == "" {
			return "", fmt.Errorf("branch name required for delete")
		}
		cmd := exec.Command("git", "branch", "-d", req.Name)
		cmd.Dir = e.workspace
		output, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("git branch delete failed: %w", err)
		}
		return string(output), nil

	default:
		return "", fmt.Errorf("unknown git branch action: %s", req.Action)
	}
}
