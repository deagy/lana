package impl

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/deagy/lana/internal/policy"
	"github.com/deagy/lana/internal/tools"
)

// FileTool provides file reading and writing operations.
type FileTool struct {
	workspace string
}

// NewFileTool creates a new file tool.
func NewFileTool(workspace string) *FileTool {
	return &FileTool{
		workspace: workspace,
	}
}

// Read creates a read_file tool.
func (ft *FileTool) Read() tools.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {
				"type": "string",
				"description": "Path to file to read (relative to workspace)"
			}
		},
		"required": ["path"]
	}`)

	return &tools.Definition{
		NameVal:        "read_file",
		DescriptionVal: "Read a file from the workspace",
		SchemaVal:      schema,
		RiskLevel:      tools.RiskLevelLow,
		ExecutorVal:    &fileReadExecutor{workspace: ft.workspace},
	}
}

// Write creates a write_file tool.
func (ft *FileTool) Write() tools.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {
				"type": "string",
				"description": "Path to file to write (relative to workspace)"
			},
			"content": {
				"type": "string",
				"description": "Content to write to the file"
			}
		},
		"required": ["path", "content"]
	}`)

	return &tools.Definition{
		NameVal:        "write_file",
		DescriptionVal: "Write content to a file in the workspace",
		SchemaVal:      schema,
		RiskLevel:      tools.RiskLevelMedium,
		ExecutorVal:    &fileWriteExecutor{workspace: ft.workspace},
	}
}

type fileReadExecutor struct {
	workspace string
}

func (e *fileReadExecutor) Execute(_ interface{}, input json.RawMessage) (string, error) {
	var req struct {
		Path string `json:"path"`
	}

	if err := json.Unmarshal(input, &req); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	// Validate path
	absPath, err := policy.ResolveWorkspacePath(e.workspace, req.Path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	// Read file
	content, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}

	return string(content), nil
}

type fileWriteExecutor struct {
	workspace string
}

func (e *fileWriteExecutor) Execute(_ interface{}, input json.RawMessage) (string, error) {
	var req struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}

	if err := json.Unmarshal(input, &req); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	// Validate path
	absPath, err := policy.ResolveWorkspacePath(e.workspace, req.Path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	// Create parent directories if needed
	if dir := filepath.Dir(absPath); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", fmt.Errorf("create directories: %w", err)
		}
	}

	// Write file
	if err := os.WriteFile(absPath, []byte(req.Content), 0644); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}

	return fmt.Sprintf("Written %d bytes to %s", len(req.Content), req.Path), nil
}

// List creates a list_files tool.
func (ft *FileTool) List() tools.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {
				"type": "string",
				"description": "Directory path (relative to workspace)"
			},
			"recursive": {
				"type": "boolean",
				"description": "Include files in subdirectories"
			}
		},
		"required": ["path"]
	}`)

	return &tools.Definition{
		NameVal:        "list_files",
		DescriptionVal: "List files in a workspace directory",
		SchemaVal:      schema,
		RiskLevel:      tools.RiskLevelLow,
		ExecutorVal:    &fileListExecutor{workspace: ft.workspace},
	}
}

type fileListExecutor struct {
	workspace string
}

func (e *fileListExecutor) Execute(_ interface{}, input json.RawMessage) (string, error) {
	var req struct {
		Path      string `json:"path"`
		Recursive bool   `json:"recursive"`
	}

	if err := json.Unmarshal(input, &req); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	// Validate path
	absPath, err := policy.ResolveWorkspacePath(e.workspace, req.Path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	// List files
	var result []string
	if req.Recursive {
		filepath.Walk(absPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			rel, _ := filepath.Rel(absPath, path)
			if rel != "." {
				if info.IsDir() {
					result = append(result, rel+"/")
				} else {
					result = append(result, rel)
				}
			}
			return nil
		})
	} else {
		entries, err := os.ReadDir(absPath)
		if err != nil {
			return "", fmt.Errorf("read directory: %w", err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				result = append(result, entry.Name()+"/")
			} else {
				result = append(result, entry.Name())
			}
		}
	}

	return strings.Join(result, "\n"), nil
}
