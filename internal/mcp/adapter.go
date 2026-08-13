package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/deagy/lana/internal/tools"
)

// RegisterTools discovers MCP tools and registers them in the provided tool registry.
// Each tool is namespaced as "mcp__<server>__<tool>" to avoid collisions.
func RegisterTools(registry *tools.Registry, mgr *Manager) error {
	namedTools := mgr.Tools()

	for _, namedTool := range namedTools {
		// Create a namespaced tool name
		toolName := fmt.Sprintf("mcp__%s__%s", namedTool.ServerName, namedTool.ToolName)

		// Parse the risk level from the server config
		riskLevel := tools.RiskLevelMedium
		cfg, ok := mgr.configs[namedTool.ServerName]
		if ok {
			switch cfg.RiskLevel {
			case "low":
				riskLevel = tools.RiskLevelLow
			case "medium":
				riskLevel = tools.RiskLevelMedium
			case "high":
				riskLevel = tools.RiskLevelHigh
			}
		}

		// Create a tool definition that wraps the MCP tool
		toolDef := &tools.Definition{
			NameVal:        toolName,
			DescriptionVal: namedTool.Spec.Description,
			SchemaVal:      namedTool.Spec.InputSchema,
			RiskLevel:      riskLevel,
			ExecutorVal: &mcpToolExecutor{
				mgr:        mgr,
				serverName: namedTool.ServerName,
				toolName:   namedTool.ToolName,
			},
		}

		// Register the tool
		if err := registry.Register(toolDef); err != nil {
			return fmt.Errorf("register MCP tool %s: %w", toolName, err)
		}
	}

	return nil
}

// mcpToolExecutor implements tools.SimpleExecutor for MCP tools.
type mcpToolExecutor struct {
	mgr        *Manager
	serverName string
	toolName   string
}

// Execute implements tools.SimpleExecutor.
func (e *mcpToolExecutor) Execute(input interface{}, data json.RawMessage) (string, error) {
	// Use context with no timeout here; the manager will apply its own timeout
	ctx := context.Background()
	return e.mgr.CallTool(ctx, e.serverName, e.toolName, data)
}
