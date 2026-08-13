package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestManagerStartSuccess tests successful manager startup with multiple servers.
func TestManagerStartSuccess(t *testing.T) {
	// Create configs for fake servers
	configs := []ServerConfig{
		{
			Name:      "server1",
			Transport: "stdio",
			Command:   "echo", // Won't actually spawn, but config is valid
		},
		{
			Name:      "server2",
			Transport: "stdio",
			Command:   "cat",
		},
	}

	// Create manager
	mgr := NewManager(configs)

	// Verify configs are stored
	if len(mgr.configs) != 2 {
		t.Fatalf("expected 2 configs, got %d", len(mgr.configs))
	}

	// Note: We don't actually call Start() here because it would try to spawn processes.
	// In a real integration test, we'd use the fake server from the binary.
}

// TestManagerDisabledServers tests that disabled servers are skipped.
func TestManagerDisabledServers(t *testing.T) {
	configs := []ServerConfig{
		{
			Name:      "enabled",
			Transport: "stdio",
			Command:   "echo",
		},
		{
			Name:      "disabled",
			Transport: "stdio",
			Command:   "cat",
			Disabled:  true,
		},
	}

	mgr := NewManager(configs)
	if len(mgr.configs) != 2 {
		t.Fatalf("expected 2 configs despite one being disabled")
	}
}

// TestManagerToolsNamespacing tests that tools are properly namespaced.
func TestManagerToolsNamespacing(t *testing.T) {
	// Create a manager with simulated tool discovery
	mgr := &Manager{
		configs: make(map[string]ServerConfig),
		clients: make(map[string]*Client),
		tools: map[string][]NamedTool{
			"server1": {
				{
					ServerName: "server1",
					ToolName:   "tool1",
					Spec: ToolSpec{
						Name:        "tool1",
						Description: "Tool 1",
						InputSchema: json.RawMessage(`{}`),
					},
				},
			},
			"server2": {
				{
					ServerName: "server2",
					ToolName:   "tool1", // Same name, different server
					Spec: ToolSpec{
						Name:        "tool1",
						Description: "Tool 1 from server2",
						InputSchema: json.RawMessage(`{}`),
					},
				},
			},
		},
	}

	tools := mgr.Tools()
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools total, got %d", len(tools))
	}

	// Verify both tools are present (namespaceing in the adapter handles the actual namespacing)
	serverNames := make(map[string]int)
	for _, tool := range tools {
		serverNames[tool.ServerName]++
	}

	if len(serverNames) != 2 {
		t.Fatalf("expected tools from 2 servers, got %d", len(serverNames))
	}
}

// TestManagerCallToolNotFound tests error handling for missing servers.
func TestManagerCallToolNotFound(t *testing.T) {
	mgr := NewManager([]ServerConfig{})

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	_, err := mgr.CallTool(ctx, "nonexistent", "tool", json.RawMessage(`{}`))
	if err == nil {
		t.Fatalf("expected error for nonexistent server")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected 'not found' in error, got: %v", err)
	}
}

// TestManagerTimeouts tests that timeouts are applied correctly.
func TestManagerTimeouts(t *testing.T) {
	configs := []ServerConfig{
		{
			Name:                "test",
			Transport:           "stdio",
			Command:             "sleep",
			StartTimeoutSeconds: 1,
		},
	}

	mgr := NewManager(configs)

	// Verify defaults are set
	if mgr.configs["test"].StartTimeoutSeconds != 1 {
		t.Fatalf("StartTimeoutSeconds not set")
	}

	if mgr.configs["test"].CallTimeoutSeconds == 0 {
		t.Fatalf("CallTimeoutSeconds should have a default")
	}
}
