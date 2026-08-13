package impl

import (
	"github.com/deagy/lana/internal/tools"
)

// InitializeRegistry creates and populates a tool registry with all available tools.
func InitializeRegistry(workspace string) (*tools.Registry, error) {
	registry := tools.NewRegistry()

	// File tools
	fileTool := NewFileTool(workspace)
	registry.Register(fileTool.Read())
	registry.Register(fileTool.Write())
	registry.Register(fileTool.List())

	// Shell tool
	shellTool := NewShellTool(workspace)
	registry.Register(shellTool.Exec())

	// Git tools
	gitTool := NewGitTool(workspace)
	registry.Register(gitTool.Status())
	registry.Register(gitTool.Diff())
	registry.Register(gitTool.Commit())
	registry.Register(gitTool.Branch())

	// Search tool
	searchTool := NewSearchTool(workspace)
	registry.Register(searchTool.Search())

	return registry, nil
}
