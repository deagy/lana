package tools

import (
	"context"
	"encoding/json"
	"testing"
)

// BenchmarkRegistryGet measures registry lookup performance
func BenchmarkRegistryGet(b *testing.B) {
	registry := NewRegistry()

	// Register test tools
	for i := 0; i < 20; i++ {
		name := "tool_" + string(rune('a'+i%26))
		registry.Register(&mockTool{name: name})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		registry.Get("tool_a")
	}
}

// BenchmarkRegistryList measures registry listing performance
func BenchmarkRegistryList(b *testing.B) {
	registry := NewRegistry()

	// Register test tools
	for i := 0; i < 100; i++ {
		name := "tool_" + string(rune(i%26+'a'))
		registry.Register(&mockTool{name: name})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		registry.List()
	}
}

// BenchmarkRegistrySchemas measures schema generation
func BenchmarkRegistrySchemas(b *testing.B) {
	registry := NewRegistry()

	// Register test tools
	for i := 0; i < 50; i++ {
		name := "tool_" + string(rune(i%26+'a'))
		registry.Register(&mockTool{name: name})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		registry.Schemas()
	}
}

// Mock tool for benchmarking
type mockTool struct {
	name string
}

func (m *mockTool) Name() string                 { return m.name }
func (m *mockTool) Description() string          { return "test tool" }
func (m *mockTool) InputSchema() json.RawMessage { return json.RawMessage(`{}`) }
func (m *mockTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	return "test", nil
}
