package tools

import (
	"fmt"
	"sync"
)

// Registry manages available tools.
type Registry struct {
	tools map[string]Tool
	mu    sync.RWMutex
}

// NewRegistry creates a new tool registry.
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// Register adds a tool to the registry.
func (r *Registry) Register(tool Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := tool.Name()
	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("tool already registered: %s", name)
	}

	r.tools[name] = tool
	return nil
}

// Get retrieves a tool by name.
func (r *Registry) Get(name string) (Tool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tool, ok := r.tools[name]
	if !ok {
		return nil, fmt.Errorf("tool not found: %s", name)
	}

	return tool, nil
}

// List returns all registered tools.
func (r *Registry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []Tool
	for _, tool := range r.tools {
		result = append(result, tool)
	}

	return result
}

// Execute runs a tool by name (deprecated, use Executor instead).
func (r *Registry) Execute(name string, input interface{}) (string, error) {
	return "", fmt.Errorf("use execution.Executor for tool execution")
}

// Schemas returns JSON schemas for all tools.
func (r *Registry) Schemas() []map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []map[string]interface{}
	for _, tool := range r.tools {
		result = append(result, map[string]interface{}{
			"name":        tool.Name(),
			"description": tool.Description(),
			"schema":      tool.InputSchema(),
		})
	}

	return result
}
