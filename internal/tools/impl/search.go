package impl

import (
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/deagy/lana/internal/tools"
)

// SearchTool provides file searching capabilities.
type SearchTool struct {
	workspace string
}

// NewSearchTool creates a new search tool.
func NewSearchTool(workspace string) *SearchTool {
	return &SearchTool{
		workspace: workspace,
	}
}

// Search creates a search tool using ripgrep or grep.
func (st *SearchTool) Search() tools.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"pattern": {
				"type": "string",
				"description": "Search pattern (regex)"
			},
			"file_pattern": {
				"type": "string",
				"description": "File pattern to search in (glob, e.g. '*.go')"
			},
			"context": {
				"type": "integer",
				"description": "Lines of context to show (default 2)"
			}
		},
		"required": ["pattern"]
	}`)

	return &tools.Definition{
		NameVal:        "search",
		DescriptionVal: "Search files in workspace (using ripgrep or grep)",
		SchemaVal:      schema,
		RiskLevel:      tools.RiskLevelLow,
		ExecutorVal:    &searchExecutor{workspace: st.workspace},
	}
}

type searchExecutor struct {
	workspace string
}

func (e *searchExecutor) Execute(_ interface{}, input json.RawMessage) (string, error) {
	var req struct {
		Pattern     string `json:"pattern"`
		FilePattern string `json:"file_pattern"`
		Context     int    `json:"context"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	if req.Pattern == "" {
		return "", fmt.Errorf("search pattern required")
	}

	if req.Context == 0 {
		req.Context = 2
	}

	// Try ripgrep first (faster), fallback to grep
	return e.search(req.Pattern, req.FilePattern, req.Context)
}

func (e *searchExecutor) search(pattern, filePattern string, context int) (string, error) {
	// Try ripgrep first
	result, err := e.searchWithRipgrep(pattern, filePattern, context)
	if err == nil {
		return result, nil
	}

	// Fallback to grep
	return e.searchWithGrep(pattern, filePattern, context)
}

func (e *searchExecutor) searchWithRipgrep(pattern, filePattern string, context int) (string, error) {
	args := []string{
		"--color", "never",
		"--context", fmt.Sprintf("%d", context),
		"--max-count", "50", // Limit results
		pattern,
	}

	if filePattern != "" {
		args = append(args, "-g", filePattern)
	}

	cmd := exec.Command("rg", args...)
	cmd.Dir = e.workspace

	output, err := cmd.CombinedOutput()
	if err != nil {
		// No matches is not an error for ripgrep (exit code 1)
		if cmd.ProcessState.ExitCode() == 1 {
			return "No matches found", nil
		}
		return "", fmt.Errorf("ripgrep error: %w", err)
	}

	return string(output), nil
}

func (e *searchExecutor) searchWithGrep(pattern, filePattern string, context int) (string, error) {
	args := []string{
		"-r",
		"-n",
		fmt.Sprintf("-A%d", context),
		fmt.Sprintf("-B%d", context),
		"--max-count=50",
		pattern,
		e.workspace,
	}

	if filePattern != "" {
		// grep doesn't have a clean glob filter, just add it to the command
		// This is a simplified approach
		args = append(args, "--include="+filePattern)
	}

	cmd := exec.Command("grep", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		// Exit code 1 means no matches
		if cmd.ProcessState != nil && cmd.ProcessState.ExitCode() == 1 {
			return "No matches found", nil
		}
		return "", fmt.Errorf("grep error: %w", err)
	}

	return string(output), nil
}
