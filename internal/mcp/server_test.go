package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/deagy/lana/internal/tools/impl"
)

func TestServerInitialize(t *testing.T) {
	registry, err := impl.InitializeRegistry(".")
	if err != nil {
		t.Fatalf("Initialize registry: %v", err)
	}

	server := NewServer(registry)
	req := &Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion": "2024-11-05", "capabilities": {}, "clientInfo": {"name": "test", "version": "1.0"}}`),
	}

	resp, err := server.HandleRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("HandleRequest: %v", err)
	}

	if resp.ID != 1 {
		t.Errorf("Response ID = %d, want 1", resp.ID)
	}

	if resp.Error != nil {
		t.Errorf("Response error = %v, want nil", resp.Error)
	}

	var result InitializeResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Unmarshal result: %v", err)
	}

	if result.ServerInfo.Name != "Lana" {
		t.Errorf("Server name = %q, want Lana", result.ServerInfo.Name)
	}
}

func TestServerListTools(t *testing.T) {
	registry, err := impl.InitializeRegistry(".")
	if err != nil {
		t.Fatalf("Initialize registry: %v", err)
	}

	server := NewServer(registry)
	req := &Request{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/list",
		Params:  json.RawMessage(`{}`),
	}

	resp, err := server.HandleRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("HandleRequest: %v", err)
	}

	if resp.Error != nil {
		t.Errorf("Response error = %v, want nil", resp.Error)
	}

	var result ListToolsResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Unmarshal result: %v", err)
	}

	if len(result.Tools) == 0 {
		t.Errorf("No tools returned")
	}

	// Check for known tools
	toolNames := make(map[string]bool)
	for _, tool := range result.Tools {
		toolNames[tool.Name] = true
	}

	expectedTools := []string{"read_file", "write_file", "exec"}
	for _, expected := range expectedTools {
		if !toolNames[expected] {
			t.Errorf("Tool %q not found", expected)
		}
	}
}

func TestServerCallTool(t *testing.T) {
	// Note: Tool execution is tested indirectly through HTTP and stdio tests
	// Direct execution with workspace validation is already tested in tool_impl tests
	// This test verifies that the server properly handles tool/call requests and responses

	registry, err := impl.InitializeRegistry(".")
	if err != nil {
		t.Fatalf("Initialize registry: %v", err)
	}

	server := NewServer(registry)

	// Call tools/list first to get available tools
	listReq := &Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
		Params:  json.RawMessage(`{}`),
	}

	resp, err := server.HandleRequest(context.Background(), listReq)
	if err != nil {
		t.Fatalf("HandleRequest: %v", err)
	}

	var listResult ListToolsResult
	if err := json.Unmarshal(resp.Result, &listResult); err != nil {
		t.Fatalf("Unmarshal list result: %v", err)
	}

	if len(listResult.Tools) == 0 {
		t.Fatalf("No tools available for testing")
	}

	// Test that tools/call response structure is correct
	// (actual execution is validated in tool_impl tests)
	callReq := &Request{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name": "nonexistent", "arguments": {}}`),
	}

	callResp, err := server.HandleRequest(context.Background(), callReq)
	if err != nil {
		t.Fatalf("HandleRequest: %v", err)
	}

	// Should return an error for nonexistent tool
	if callResp.Error == nil {
		t.Errorf("Expected error for nonexistent tool")
	}
}

func TestServerToolNotFound(t *testing.T) {
	registry, err := impl.InitializeRegistry(".")
	if err != nil {
		t.Fatalf("Initialize registry: %v", err)
	}

	server := NewServer(registry)
	req := &Request{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name": "nonexistent_tool", "arguments": {}}`),
	}

	resp, err := server.HandleRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("HandleRequest: %v", err)
	}

	if resp.Error == nil {
		t.Errorf("Expected error for nonexistent tool, got nil")
	}
}

func TestServerInvalidMethod(t *testing.T) {
	registry, err := impl.InitializeRegistry(".")
	if err != nil {
		t.Fatalf("Initialize registry: %v", err)
	}

	server := NewServer(registry)
	req := &Request{
		JSONRPC: "2.0",
		ID:      5,
		Method:  "invalid/method",
		Params:  json.RawMessage(`{}`),
	}

	resp, err := server.HandleRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("HandleRequest: %v", err)
	}

	if resp.Error == nil {
		t.Errorf("Expected error for invalid method, got nil")
	}

	if resp.Error.Code != -32601 {
		t.Errorf("Error code = %d, want -32601", resp.Error.Code)
	}
}
